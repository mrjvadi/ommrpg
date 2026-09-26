// world-service owns worlds. It persists nothing but each world's seed and
// serves generated chunks, species and spawn points.
package main

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/fs"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/mrjvadi/ommrpg/backend/pkg/apperr"
	"github.com/mrjvadi/ommrpg/backend/pkg/bus"
	"github.com/mrjvadi/ommrpg/backend/pkg/config"
	c "github.com/mrjvadi/ommrpg/backend/pkg/contracts"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/bestiary"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/world"
	"github.com/mrjvadi/ommrpg/backend/pkg/lru"
	"github.com/mrjvadi/ommrpg/backend/pkg/store"
	"github.com/mrjvadi/ommrpg/backend/pkg/svc"
)

//go:embed migrations/*.sql
var migrations embed.FS

func main() {
	s := svc.New("world")
	db, err := store.Postgres(s.Ctx, config.String("POSTGRES_DSN", "postgres://ommrpg:ommrpg@localhost:5432/world?sslmode=disable"))
	if err != nil {
		s.Fatal("postgres", err)
	}
	sub, _ := fs.Sub(migrations, "migrations")
	if err := store.Migrate(s.Ctx, db, sub, "world"); err != nil {
		s.Fatal("migrate", err)
	}
	rdb, err := store.Dragonfly(s.Ctx, config.String("DRAGONFLY_URL", "redis://localhost:6379/0"))
	if err != nil {
		s.Fatal("dragonfly", err)
	}
	a := &app{db: db, rdb: rdb, chunks: lru.New[string, []byte](2048, 0), worlds: lru.New[int, c.WorldInfo](64, time.Minute)}
	if err := a.ensureDefault(s.Ctx, config.Uint64("WORLD_SEED", 0), config.Int("WORLD_SIZE_CHUNKS", 64)); err != nil {
		s.Fatal("default world", err)
	}
	b, err := bus.Connect(s.Cfg.NATSURL, "world", s.Log)
	if err != nil {
		s.Fatal("nats", err)
	}
	defer b.Close()
	for _, err := range []error{
		bus.Handle(b, c.WorldList, a.list),
		bus.Handle(b, c.WorldGet, a.get),
		bus.Handle(b, c.WorldDefault, a.defaultWorld),
		bus.Handle(b, c.WorldSpecies, a.species),
		bus.Handle(b, c.WorldChunk, a.chunkRaw),
	} {
		if err != nil {
			s.Fatal("subscribe", err)
		}
	}
	s.Ready()
	s.Wait()
}

type app struct {
	db     *pgxpool.Pool
	rdb    *redis.Client
	chunks *lru.Cache[string, []byte]
	worlds *lru.Cache[int, c.WorldInfo]
}

func (a *app) ensureDefault(ctx context.Context, seedV uint64, size int) error {
	var n int
	if err := a.db.QueryRow(ctx, `SELECT count(*) FROM worlds WHERE is_default`).Scan(&n); err != nil || n > 0 {
		return err
	}
	if seedV == 0 {
		var buf [8]byte
		_, _ = rand.Read(buf[:])
		seedV = binary.LittleEndian.Uint64(buf[:]) >> 1
	}
	_, err := a.db.Exec(ctx, `INSERT INTO worlds (seed, size_chunks, name, is_default) VALUES ($1::numeric, $2, $3, true)
		ON CONFLICT DO NOTHING`, strconv.FormatUint(seedV, 10), size, world.WorldName(seedV))
	return err
}

func info(p world.Params) c.WorldInfo {
	x, y := p.Spawn()
	return c.WorldInfo{Params: p, SpawnX: x, SpawnY: y}
}

func (a *app) load(ctx context.Context, id int) (c.WorldInfo, error) {
	if w, ok := a.worlds.Get(id); ok {
		return w, nil
	}
	var p world.Params
	var seedS string
	err := a.db.QueryRow(ctx, `SELECT id, seed::text, size_chunks, name FROM worlds WHERE id=$1`, id).Scan(&p.ID, &seedS, &p.SizeChunks, &p.Name)
	if err != nil {
		return c.WorldInfo{}, apperr.New(apperr.NotFound, "world %d not found", id)
	}
	p.Seed, _ = strconv.ParseUint(seedS, 10, 64)
	w := info(p)
	a.worlds.Put(id, w)
	return w, nil
}

func (a *app) list(ctx context.Context, _ struct{}) (c.WorldListResp, error) {
	rows, err := a.db.Query(ctx, `SELECT id FROM worlds ORDER BY id`)
	if err != nil {
		return c.WorldListResp{}, err
	}
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return c.WorldListResp{}, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	out := c.WorldListResp{}
	for _, id := range ids {
		w, err := a.load(ctx, id)
		if err != nil {
			return out, err
		}
		out.Worlds = append(out.Worlds, w)
	}
	return out, nil
}

func (a *app) get(ctx context.Context, req c.WorldReq) (c.WorldInfo, error) {
	return a.load(ctx, req.WorldID)
}

func (a *app) defaultWorld(ctx context.Context, _ struct{}) (c.WorldInfo, error) {
	var id int
	if err := a.db.QueryRow(ctx, `SELECT id FROM worlds WHERE is_default`).Scan(&id); err != nil {
		return c.WorldInfo{}, apperr.New(apperr.NotFound, "no default world")
	}
	return a.load(ctx, id)
}

func (a *app) species(ctx context.Context, req c.WorldReq) (c.SpeciesResp, error) {
	w, err := a.load(ctx, req.WorldID)
	if err != nil {
		return c.SpeciesResp{}, err
	}
	return c.SpeciesResp{Species: bestiary.ForWorld(w.Seed)}, nil
}

// chunkRaw returns the chunk JSON; generation results are cached in memory
// and in Dragonfly so other replicas never regenerate hot chunks.
func (a *app) chunkRaw(ctx context.Context, req c.ChunkReq) (json.RawMessage, error) {
	w, err := a.load(ctx, req.WorldID)
	if err != nil {
		return nil, err
	}
	if req.CX < 0 || req.CY < 0 || req.CX >= w.SizeChunks || req.CY >= w.SizeChunks {
		return nil, apperr.New(apperr.NotFound, "chunk out of world")
	}
	key := fmt.Sprintf("chunk:%d:%d:%d", req.WorldID, req.CX, req.CY)
	if v, ok := a.chunks.Get(key); ok {
		return v, nil
	}
	if v, err := a.rdb.Get(ctx, key).Bytes(); err == nil {
		a.chunks.Put(key, v)
		return v, nil
	}
	ch := c.Chunk{
		WorldID:  req.WorldID,
		Terrain:  w.ChunkTerrain(req.CX, req.CY),
		Monsters: bestiary.ChunkSpawns(w.Params, req.CX, req.CY),
	}
	if e, ok := w.EntranceIn(req.CX, req.CY); ok {
		ch.Entrance = &e
	}
	raw, err := json.Marshal(ch)
	if err != nil {
		return nil, err
	}
	a.chunks.Put(key, raw)
	_ = a.rdb.Set(ctx, key, raw, 6*time.Hour).Err()
	return raw, nil
}
