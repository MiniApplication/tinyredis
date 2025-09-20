# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview
TinyRedis is a high-performance, Redis-compatible in-memory cache server written in Go. It implements the Redis Serialization Protocol (RESP) and supports core Redis commands with persistence and TTL functionality.

## Common Development Commands

### Build and Run
```bash
# Build the binary
make build

# Run the server (default: 0.0.0.0:6379)
./tiny-redis

# Run with custom configuration
./tiny-redis -H 0.0.0.0 -p 6379 -l info -s 1024
```

### Testing
```bash
# Run all tests
make test

# Run tests for a specific package
go test ./pkg/memdb/...

# Run tests with coverage
go test -cover ./...

# Run a single test
go test -run TestSetCommand ./pkg/memdb/
```

### Docker Operations
```bash
# Build Docker image
make docker-build

# Run in Docker
make docker-run

# Clean build artifacts
make clean
```

## Core Architecture

### Module Organization
The codebase follows a clean architecture with clear separation of concerns:

- **pkg/RESP/**: Redis protocol implementation. Core interface is `RedisData` with implementations for all RESP types.
- **pkg/memdb/**: The heart of the database - concurrent sharded map with automatic red-black tree conversion for collision chains (threshold: 8 items).
- **pkg/server/**: TCP server handling connections with one goroutine per client.
- **pkg/config/**: Configuration management supporting both CLI args and config files.

### Key Design Patterns

1. **Sharded Concurrency**: The database uses 1024 shards (configurable) to minimize lock contention. Each shard has its own RWMutex.

2. **Command Registration**: Commands are registered via `MemDb.RegisterCommand()` in `pkg/memdb/command_*.go` files. Each command implements:
   ```go
   func(db *MemDb, cmd [][]byte) RESP.RedisData
   ```

3. **TTL Management**: Uses a min-heap (`TTLManager`) with periodic cleanup. TTL operations are handled separately from the main data operations.

4. **AOF Persistence**: Asynchronous write-ahead logging for durability without blocking operations.

### Adding New Commands

1. Create a new file `pkg/memdb/command_<category>.go`
2. Implement the command function following the signature above
3. Register in `RegisterCommands()` function
4. Add corresponding tests in `pkg/memdb/command_<category>_test.go`

### Performance Considerations

- The database automatically converts collision chains from linked lists to red-black trees when they exceed 8 items
- TTL cleanup runs periodically (100ms intervals) to prevent memory leaks
- AOF writes are buffered and asynchronous to avoid blocking operations
- Each connection is handled in a separate goroutine for maximum concurrency

### Testing Approach

- Unit tests use the testify framework for assertions
- Integration tests verify command behavior against the full stack
- Performance can be validated using `redis-benchmark` against the running server
- Mock Redis clients can be used for protocol compatibility testing