# redigo

A Redis-compatible in-memory key–value store written in Go (TCP + RESP2).

## Goals
- Speak RESP2 over TCP (works with `redis-cli`)
- Core KV commands (GET/SET/DEL/EXISTS)
- TTL expiration (EXPIRE/TTL, SET EX)
- Optional persistence via AOF replay
- Simple, readable, interview-friendly design

## Non-goals
- Full Redis feature parity (lists/hashes/cluster/lua/etc.)

## Requirements
- Go 1.25+

## Running
```bash
go run ./cmd/redigo
