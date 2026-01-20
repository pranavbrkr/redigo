# Eedigo

A Redis-compatible in-memory key–value store written in Go.

This project implements a small subset of Redis server functionality
and speaks the RESP2 protocol over TCP, allowing it to work with
the standard `redis-cli` client.

## Scope
- TCP server compatible with `redis-cli`
- RESP2 protocol support
- In-memory key–value store
- TTL expiration (EXPIRE / TTL / SET EX)
- Append-only file (AOF) persistence

## Non-goals
- Full Redis feature parity
- Lists, hashes, pub/sub, clustering, or Lua scripting

## Requirements
- Go 1.25+
- Windows (no WSL required)

## Running (Windows)
```powershell
go run ./cmd/redigo
go run ./cmd/redigo -aof-enabled=true -aof-path data/appendonly.aof