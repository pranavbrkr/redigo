# Redigo

Redigo is a Redis-compatible, in-memory key–value store implemented from
scratch in Go. It's intentionally small and educational while providing a
faithful subset of Redis behaviour over the RESP2 protocol so standard
clients like `redis-cli` work unchanged.

Key characteristics
- Implements a selected set of Redis semantics: `SET`, `GET`, `DEL`, `EXISTS`,
	`EXPIRE`, `EXPIREAT`, `TTL`, `INFO`, and `COMMAND`.
- RESP2 protocol parsing and encoding live under `protocol/resp`.
- In-memory storage with accurate, Redis-style TTL semantics and both
	immediate and lazy eviction; background expiration is handled by a
	reaper goroutine in `store/reaper.go`.
- Durable persistence via an append-only file (AOF) under `internal/aof`.
	AOF replay restores state on startup and fsync policy is configurable
	(`always`, `everysec`, `never`) to balance durability vs performance.
- Concurrent server design with explicit synchronization and graceful
	shutdown handling.
- Companion CLI (`cmd/redigo-cli`) with interactive REPL, one-shot mode,
	piping/script mode, and Redis-style output formatting.

Why this layout
- `protocol/resp` isolates all wire-format concerns (decoder/encoder/types).
- `server/` contains command routing, request lifecycle, and command
	registration.
- `store/` owns the in-memory model, TTL bookkeeping, and the reaper.
- `internal/aof` provides AOF append + replay; tests ensure replay format
	remains stable.

Requirements
- Go 1.25+
- Windows is supported (no WSL required) but standard Go tooling works across
	platforms.

Quick start (development)
```powershell
go run ./cmd/redigo
# with AOF enabled
go run ./cmd/redigo -aof-enabled=true -aof-path data/appendonly.aof
```

Build
```powershell
go build ./cmd/redigo
go build ./cmd/redigo-cli
```

Testing
```powershell
go test ./...
# Run a package test: go test ./store -run TestName
```

Developer conventions and guidance for contributors / AI agents
- Wire-level changes: update `protocol/resp` (decoder/encoder) and add
	codec tests before touching `server` logic.
- Commands: register handlers in `server/` and add end-to-end tests that
	exercise RESP requests through the server to the `store`.
- Persistence: any state-changing command must produce AOF lines compatible
	with `internal/aof` replay. If you modify serialization, update the
	AOF tests under `internal/aof`.
- Expiry: `store` is the sole owner of TTL behavior. Use its APIs rather than
	duplicating expiration logic elsewhere.

Testing patterns
- Tests live next to packages (`*_test.go`) and frequently assert timing/
	TTL and AOF replay behaviour. Prefer deterministic tests over long sleeps.

Scope & non-goals
- Redigo is intentionally not a full Redis replacement. It omits lists,
	sets, hashes, pub/sub, clustering and scripting.

Notes
- The repo includes `cmd/redigo` (server) and `cmd/redigo-cli` (client).
- The AOF file path used by default is `data/appendonly.aof` — tests may
	create temporary AOF files to isolate persistence behavior.

If you'd like I can add short examples showing how `SET`/`EXPIRE`/`GET`
interact, or extract command registration and AOF append examples from
specific files to include inline in this `README.md`.