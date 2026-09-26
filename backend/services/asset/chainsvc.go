package main

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/mrjvadi/ommrpg/backend/pkg/apperr"
	"github.com/mrjvadi/ommrpg/backend/pkg/chain"
	c "github.com/mrjvadi/ommrpg/backend/pkg/contracts"
)

func (a *app) ensureGenesis(ctx context.Context) error {
	var n int
	if err := a.db.QueryRow(ctx, `SELECT count(*) FROM chain_blocks`).Scan(&n); err != nil || n > 0 {
		return err
	}
	h := chain.Header{Height: 0, PrevHash: chain.GenesisPrev, MerkleRoot: chain.MerkleRoot(nil), TxCount: 0, Time: time.Now().UTC().Truncate(time.Microsecond)}
	hash := h.Hash()
	_, err := a.db.Exec(ctx, `INSERT INTO chain_blocks (height, prev_hash, merkle_root, hash, tx_count, signature, created_at) VALUES (0,$1,$2,$3,0,$4,$5) ON CONFLICT DO NOTHING`,
		h.PrevHash, h.MerkleRoot, hash, a.signer.Sign(hash), h.Time)
	return err
}

// blockProducer seals pending transactions into signed blocks. A
// transaction-scoped advisory lock makes exactly one replica the producer.
func (a *app) blockProducer(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := a.sealBlock(ctx); err != nil {
				a.log.Warn("seal block", "err", err)
			}
		}
	}
}

func (a *app) sealBlock(ctx context.Context) error {
	return pgx.BeginFunc(ctx, a.db, func(tx pgx.Tx) error {
		var got bool
		if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(hashtext('omm-chain-producer'))`).Scan(&got); err != nil || !got {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT id, hash FROM chain_txs WHERE block_height IS NULL ORDER BY id LIMIT 1000 FOR UPDATE`)
		if err != nil {
			return err
		}
		var ids []int64
		var hashes []string
		for rows.Next() {
			var id int64
			var h string
			if err := rows.Scan(&id, &h); err != nil {
				rows.Close()
				return err
			}
			ids, hashes = append(ids, id), append(hashes, h)
		}
		rows.Close()
		if len(ids) == 0 {
			return nil
		}
		var prevH int64
		var prevHash string
		if err := tx.QueryRow(ctx, `SELECT height, hash FROM chain_blocks ORDER BY height DESC LIMIT 1`).Scan(&prevH, &prevHash); err != nil {
			return err
		}
		h := chain.Header{Height: prevH + 1, PrevHash: prevHash, MerkleRoot: chain.MerkleRoot(hashes), TxCount: len(ids), Time: time.Now().UTC().Truncate(time.Microsecond)}
		hash := h.Hash()
		if _, err := tx.Exec(ctx, `INSERT INTO chain_blocks (height, prev_hash, merkle_root, hash, tx_count, signature, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			h.Height, h.PrevHash, h.MerkleRoot, hash, h.TxCount, a.signer.Sign(hash), h.Time); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE chain_txs SET block_height=$1 WHERE id = ANY($2)`, h.Height, ids)
		return err
	})
}

const blockCols = `height, prev_hash, merkle_root, hash, tx_count, signature, created_at`

func scanBlock(r pgx.Row) (c.Block, error) {
	var b c.Block
	err := r.Scan(&b.Height, &b.PrevHash, &b.MerkleRoot, &b.Hash, &b.TxCount, &b.Signature, &b.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return b, apperr.New(apperr.NotFound, "block not found")
	}
	return b, err
}

func (a *app) chainInfo(ctx context.Context, _ struct{}) (c.ChainInfoResp, error) {
	out := c.ChainInfoResp{Name: "OMM Chain", PublicKey: a.signer.PublicKey()}
	_ = a.db.QueryRow(ctx, `SELECT COALESCE(max(height),0) FROM chain_blocks`).Scan(&out.Height)
	_ = a.db.QueryRow(ctx, `SELECT count(*) FILTER (WHERE block_height IS NULL), count(*) FROM chain_txs`).Scan(&out.Pending, &out.Txs)
	return out, nil
}

func (a *app) blocks(ctx context.Context, req c.BlocksReq) (c.BlocksResp, error) {
	limit := req.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	before := req.Before
	if before <= 0 {
		before = 1 << 62
	}
	rows, err := a.db.Query(ctx, `SELECT `+blockCols+` FROM chain_blocks WHERE height < $1 ORDER BY height DESC LIMIT $2`, before, limit)
	if err != nil {
		return c.BlocksResp{}, err
	}
	defer rows.Close()
	out := c.BlocksResp{Blocks: []c.Block{}}
	for rows.Next() {
		b, err := scanBlock(rows)
		if err != nil {
			return out, err
		}
		out.Blocks = append(out.Blocks, b)
	}
	return out, rows.Err()
}

func (a *app) block(ctx context.Context, req c.BlocksReq) (c.Block, error) {
	b, err := scanBlock(a.db.QueryRow(ctx, `SELECT `+blockCols+` FROM chain_blocks WHERE height=$1`, req.Height))
	if err != nil {
		return b, err
	}
	b.Txs, err = a.txs(ctx, `WHERE block_height=$1 ORDER BY id`, req.Height)
	return b, err
}

// proof returns a Merkle inclusion proof for a transaction, verified here
// too (the client can re-verify with the public key and the proof).
func (a *app) proof(ctx context.Context, req c.ProofReq) (c.ProofResp, error) {
	txs, err := a.txs(ctx, `WHERE hash=$1`, req.TxHash)
	if err != nil {
		return c.ProofResp{}, err
	}
	if len(txs) == 0 {
		return c.ProofResp{}, apperr.New(apperr.NotFound, "transaction not found")
	}
	t := txs[0]
	out := c.ProofResp{Tx: t, PublicKey: a.signer.PublicKey()}
	if t.BlockHeight == nil {
		return out, nil // not sealed yet
	}
	blk, err := a.block(ctx, c.BlocksReq{Height: *t.BlockHeight})
	if err != nil {
		return out, err
	}
	var hashes []string
	idx := -1
	for i, x := range blk.Txs {
		hashes = append(hashes, x.Hash)
		if x.Hash == t.Hash {
			idx = i
		}
	}
	steps := chain.Proof(hashes, idx)
	for _, s := range steps {
		out.Proof = append(out.Proof, c.ProofStep{Hash: s.Hash, Left: s.Left})
	}
	hdr := chain.Header{Height: blk.Height, PrevHash: blk.PrevHash, MerkleRoot: blk.MerkleRoot, TxCount: blk.TxCount, Time: blk.CreatedAt}
	out.Valid = chain.VerifyProof(t.Hash, steps, blk.MerkleRoot) && hdr.Hash() == blk.Hash && chain.VerifyBlock(out.PublicKey, blk.Hash, blk.Signature)
	blk.Txs = nil
	out.Block = blk
	return out, nil
}
