# BitSwarm — Agent Guidance

Module: `bitswarm/server`

## Architecture

Layered per-resource packages under `internal/`: **handler → service → repository**. `service` depends on the resource's `Repository` interface, never a concrete DB type, so it can be tested with a fake. See `internal/health/` for the worked pattern — copy it for every new resource.

```
cmd/server/main.go        composition root
internal/
  config/                 env-based config
  logger/                 winston-style logger on log/slog
  jsonutil/               encoding/json wrapper
  db/                     database/sql + SQLite, migrations/ (embedded)
  http/                   Fiber router + middleware
  <resource>/             model.go, repository.go, service.go, handler.go + tests
```

## Allowed dependencies

| Purpose | Dependency |
| --- | --- |
| HTTP routing | `github.com/gofiber/fiber/v2` |
| SQLite driver | `modernc.org/sqlite` (pure Go, via `database/sql`) |
| BitTorrent engine | `github.com/anacrolix/torrent` (with its own dependencies, plus `golang.org/x/time/rate` and `github.com/anacrolix/dht/v2` for its config types). Used only in `internal/engine`. |
| HTML parsing for schedules | `github.com/andybalholm/cascadia`, `golang.org/x/net/html` |
| Hot reload | `air` (dev tool) |

No logging library, no JSON library beyond `encoding/json`, no ORM, no assertion library, no config library. Ask before adding anything outside this table.

## Adding a new resource

1. Copy an existing resource package (e.g. `internal/health/`) to `internal/<resource>/`.
2. Rename types/receivers, update `model.go`, `repository.go` SQL, `service.go` logic, `handler.go` routes.
3. Register routes in `internal/http/router.go`.
4. Add an `NNNN_name.sql` file under `internal/db/migrations/` (embedded, applied in order on startup; write them idempotent with `IF NOT EXISTS`).
5. Write `*_test.go` for every file touched (one test file per source file, table-driven, stdlib `testing` only).
6. Run the validation checklist below.

## Run

```sh
make run          # air hot-reload dev server
make build         # go build ./cmd/server
make test          # go test ./... -race
```

## Validation checklist (run before considering any change done)

```sh
go build ./...
go vet ./...
go test ./...
```

## Config

Env vars: `PORT` (default 8080), `LOG_LEVEL`, `LOG_FORMAT`, `DB_PATH` (SQLite file, default `data/swarm.db`; the engine keeps its state in `engine/` next to it), `SWARM_DOWNLOADS` (default save folder, default `./downloads`), `SWARM_DEMO=1` (simulated engine with sample data), `SWARM_ALLOW_COMMANDS=1`, `SWARM_UPDATE_FEED`.

## Engine

`internal/torrents` talks to the BitTorrent client only through the `Engine` interface (`engine.go`). `internal/engine` implements it with anacrolix/torrent. Without an engine (demo mode) the service runs its simulation. Keep `torrents.Service.syncReal` declarative: a torrent is loaded in the engine exactly when its status is downloading, seeding, checking or rechecking.
