# Redigo

Small, Redis-compatible in-memory key/value store implemented in Go.

What this project is
- A Redis-inspired in-memory key/value store written in Go, implementing a
  practical subset of Redis functionality with real-world persistence semantics.
- Focuses on correctness, durability tradeoffs, and background maintenance
  (TTL reaping, AOF rewrite) rather than feature parity.
- Designed as a systems learning project that mirrors how production data
  stores handle persistence, crashes, and compaction.

Major components
- `cmd/redigo` — server entrypoint and flags; run the server here.
- `cmd/redigo-cli` — small companion CLI for interactive use and scripting.
- `protocol/resp` — RESP2 encoder/decoder and protocol types.
- `server` — command registration, request lifecycle, and network handling.
- `store` — in-memory key/value storage, TTL bookkeeping, and reaper.
- `internal/aof` — append-only file persistence and replay implementation.

Persistence & durability model

Redigo supports Redis-style append-only file (AOF) persistence with configurable
durability guarantees:

- `appendfsync=always`
  - Flushes and fsyncs on every write.
  - Strongest durability; lowest throughput.

- `appendfsync=everysec`
  - Buffers writes and fsyncs approximately once per second.
  - Good balance between durability and performance.

- `appendfsync=never`
  - Relies on OS buffering; fastest but least durable.

Crash safety
- AOF replay tolerates truncated final entries (common after crashes).
- Partial writes at the end of the file are safely ignored during recovery.

Background rewrite (BGREWRITEAOF)
- Redigo supports non-blocking AOF compaction via `BGREWRITEAOF`.
- The server:
  1. Takes a point-in-time snapshot of in-memory state.
  2. Rewrites a compact AOF in the background.
  3. Buffers concurrent writes as tail operations.
  4. Atomically swaps the rewritten file and appends buffered tail operations.
- Clients continue to operate normally during the rewrite.

Supported commands (subset)

- Connection / utility: `PING`, `ECHO`, `INFO`, `COMMAND`
- Key/value: `SET`, `GET`, `DEL`, `EXISTS`
- Expiration: `EXPIRE`, `EXPIREAT`, `TTL`
- Persistence: `BGREWRITEAOF`

The command set is intentionally limited to keep the implementation focused
and easy to reason about.

Non-goals

- Full Redis command or data type compatibility.
- Clustering, replication, or high availability.
- Lua scripting, transactions, or pub/sub.

Redigo is intentionally scoped to emphasize persistence mechanics,
crash recovery, and background maintenance rather than feature breadth.

Dependencies & requirements
- Go: 1.25.0 (see `go.mod`).
- No external services required; runs locally on supported OSes including Windows.

Quick start — run the server
PowerShell (Windows):
```powershell
# run in-place (development)
go run ./cmd/redigo

# run with AOF persistence enabled (writes to data/appendonly.aof)
go run ./cmd/redigo -aof-enabled=true -aof-path data/appendonly.aof
```

Shell (Linux / macOS):
```bash
go run ./cmd/redigo
go run ./cmd/redigo -aof-enabled=true -aof-path data/appendonly.aof
```

Build the binaries
```powershell
go build -o bin/redigo ./cmd/redigo
go build -o bin/redigo-cli ./cmd/redigo-cli
```

How to run the server

- Build a binary (optional):
  ```powershell
  go build -o bin/redigo ./cmd/redigo
  ```

- Run the built binary:
  ```powershell
  # default (no AOF)
  .\bin\redigo

  # enable AOF persistence
  .\bin\redigo -aof-enabled=true -aof-path data/appendonly.aof
  ```

- Or run in-place (development):
  ```powershell
  go run ./cmd/redigo
  go run ./cmd/redigo -aof-enabled=true -aof-path data/appendonly.aof
  ```

- Common flags
  - `-aof-enabled` (bool): enable append-only persistence (default: false).
  - `-aof-path` (string): path to the AOF file (default: `data/appendonly.aof`).
  See `cmd/redigo/main.go` for all flags and defaults.

How to interact with the server

- Using the bundled client (`redigo-cli`):
  1. Build the CLI:
     ```powershell
     go build -o bin/redigo-cli ./cmd/redigo-cli
     ```
  2. Use it to send single commands:
     ```powershell
     .\bin\redigo-cli set mykey "hello"
     .\bin\redigo-cli get mykey
     .\bin\redigo-cli expire mykey 30
     .\bin\redigo-cli ttl mykey
     ```
  3. `redigo-cli` supports interactive and scripting modes — check `cmd/redigo-cli/main.go` for usage details.

- Using other RESP2-compatible clients
  - `redis-cli` (official Redis client) works against this server — point it at the server host/port.
  - `memurai-cli` (on Windows) and other RESP2-compatible tools also work.

Notes
- AOF default path: `data/appendonly.aof` (repo root).
- This project is intentionally minimal and focuses on a small set of commands and clear implementation rather than full Redis feature parity.

Enjoy!
