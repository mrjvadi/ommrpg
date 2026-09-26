# Development

## Requirements

* Go 1.26+
* Godot 4.4.1 (+ Web export templates to build the web client)
* Docker + Compose, **or** local PostgreSQL 16, Dragonfly (or Redis),
  NATS 2.11+ with JetStream, Centrifugo v6, MongoDB 7 (or FerretDB)

## Everything with Docker

```
cd deploy
cp .env.example .env        # set secrets; DEV_LOGIN=true for local testing
godot --headless --path ../client --export-release "Web" ../client/build/web/index.html
docker compose up -d --build
open http://localhost:8088
```

## Services as local processes

Start the infrastructure yourself (for example `docker compose up -d
postgres dragonfly nats centrifugo mongodb` with the ports published, or
native binaries), then:

```
# databases: identity world character item asset admin
ADMIN_BOOTSTRAP_PASSWORD=choose-one ./scripts/run-local.sh   # builds and starts all 13 services, logs in .data/logs
./scripts/run-local.sh stop
```

The admin panel is then at http://localhost:8095/admin/ (user `owner`).

For a local Centrifugo, point its RPC proxy at the gateway:
`CENTRIFUGO_RPC_PROXY_ENDPOINT=http://localhost:8080/centrifugo/rpc centrifugo -c deploy/centrifugo/config.json`.

## Tests

```
cd backend
go test ./...                       # unit tests (generators, rules, auth, sprites)
go run ./tools/bot                  # end-to-end: login, create, walk, fight, loot, enhance, dungeon
go run ./tools/bot -n 20 -kills 5 -dungeon=false   # load / concurrency
go run ./tools/bot -scenario market # NFT mint, TON and gold sales, withdrawal, chain proof (TON_MODE=mock)
```

Client tests: see [CLIENT.md](CLIENT.md#testing).

## Useful tools

| tool | |
|---|---|
| `tools/worldmap` | render a world seed to PNG |
| `tools/spriterender` | render a seeded LPC character sheet |
| `tools/lpcimport` | regenerate `assets/lpc/catalog.json` from the LPC repo |
| `tools/bot` | headless player / E2E test / load generator |
| `tools/devserver` | serve the web build + proxy API/WS on one origin |

## Configuration (environment)

| variable | used by | default |
|---|---|---|
| `NATS_URL` | all | `nats://localhost:4222` |
| `POSTGRES_DSN` | identity, world, character, item, asset, admin | local db per service |
| `DRAGONFLY_URL` | gateway, world, presence, combat, dungeon | `redis://localhost:6379/0` |
| `MONGO_URI`, `MONGO_DB` | history | `mongodb://localhost:27017`, `ommrpg_history` |
| `CENTRIFUGO_API_URL`, `CENTRIFUGO_API_KEY` | publishers | `http://localhost:8000`, `dev-api-key` |
| `TELEGRAM_BOT_TOKEN` | gateway | (required in production) |
| `DEV_LOGIN` | gateway | `false` |
| `SESSION_SECRET`, `CENTRIFUGO_TOKEN_SECRET`, `CENTRIFUGO_PROXY_SECRET` | gateway | dev values |
| `GAME_SECRET` | item, combat, dungeon | dev value, **change it** (seeds audited rolls) |
| `WORLD_SEED`, `WORLD_SIZE_CHUNKS` | world | random, `64` |
| `SPRITE_CATALOG`, `SPRITE_CACHE_DIR`, `SPRITE_REMOTE` | sprite | repo catalog, `.data/sprite-cache`, `default` |
| `CHAIN_SIGNING_KEY`, `CHAIN_BLOCK_INTERVAL` | asset | `GAME_SECRET`, `3s` |
| `TON_MODE`, `TON_WALLET_SEED`, `TON_WALLET_VERSION`, `TON_CONFIG_URL` | ton | `mock`, -, `v4r2`, public config |
| `ADMIN_SECRET`, `ADMIN_BOOTSTRAP_USER`, `ADMIN_BOOTSTRAP_PASSWORD`, `ADMIN_WS_URL` | admin | dev value, `owner`, random (logged), `/connection/websocket` |
| `HEALTH_ADDR` | all | `:8081` (`/healthz`, `/readyz`) |
