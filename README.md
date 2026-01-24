# Redigo

Small, Redis-compatible in-memory key/value store implemented in Go.

What this project is
- A learning-focused Redis-like server that supports a compact, useful
  subset of Redis commands over the RESP2 protocol.
- Intended for exploration, experimentation, and small personal projects
  where you want a lightweight in-memory store with optional AOF persistence.

Major components
- `cmd/redigo` — server entrypoint and flags; run the server here.
- `cmd/redigo-cli` — small companion CLI for interactive use and scripting.
- `protocol/resp` — RESP2 encoder/decoder and protocol types.
- `server` — command registration, request lifecycle, and network handling.
- `store` — in-memory key/value storage, TTL bookkeeping, and reaper.
- `internal/aof` — append-only file persistence and replay implementation.

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
