package server

import "strings"

// IsWriteCommand determines if a command mutates the database state.
func IsWriteCommand(cmd [][]byte) bool {
	if len(cmd) == 0 {
		return false
	}

	switch strings.ToUpper(string(cmd[0])) {
	case "SET", "SETNX", "SETEX", "PSETEX", "DEL", "INCR", "DECR", "APPEND", "MSET", "MSETNX":
		return true
	case "HSET", "HSETNX", "HDEL", "HINCRBY", "HINCRBYFLOAT", "HMSET":
		return true
	case "LPUSH", "RPUSH", "LPOP", "RPOP", "LREM", "LSET", "LTRIM":
		return true
	case "SADD", "SREM", "SPOP", "SMOVE":
		return true
	case "ZADD", "ZREM", "ZINCRBY", "ZPOPMAX", "ZPOPMIN":
		return true
	case "EXPIRE", "EXPIREAT", "PERSIST", "RENAME", "RENAMENX", "FLUSHDB", "FLUSHALL":
		return true
	case "MULTI", "EXEC", "DISCARD", "WATCH", "UNWATCH":
		return true
	default:
		return false
	}
}
