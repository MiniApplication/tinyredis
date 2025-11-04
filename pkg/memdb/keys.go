package memdb

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hsn0918/tinyredis/pkg/RESP"
	"github.com/hsn0918/tinyredis/pkg/logger"
	"github.com/hsn0918/tinyredis/pkg/util"
)

// RegisterKeyCommand
// Register command
func RegisterKeyCommand() {
	RegisterCommand("ping", pingKeys)
	RegisterCommand("del", delKey)
	RegisterCommand("exists", existsKey)
	RegisterCommand("keys", keysKey)
	RegisterCommand("expire", expireKey)
	RegisterCommand("persist", persistKey)
	RegisterCommand("ttl", ttlKey)
	RegisterCommand("type", typeKey)
	RegisterCommand("rename", renameKey)
}

// pingKeys
// if ping return pong
func pingKeys(m *MemDb, cmd [][]byte) RESP.RedisData {
	cmdName := string(cmd[0])
	if strings.ToLower(cmdName) != "ping" {
		logger.Error("pingKeys Function: cmdName is not ping")
		return RESP.MakeErrorData("server error")
	}
	if len(cmd) > 2 {
		return RESP.MakeErrorData("error: command args number is invalid")
	}
	if len(cmd) == 1 {
		return RESP.MakeStringData("PONG")
	}
	return RESP.MakeBulkData(cmd[1])
}
func delKey(m *MemDb, cmd [][]byte) RESP.RedisData {
	cmdName := string(cmd[0])
	if strings.ToLower(cmdName) != "del" {
		logger.Error("delKey Function: cmdName is not del")
		return RESP.MakeErrorData("Protocol error: command is not del")
	}
	dKeyCount := 0
	for _, key := range cmd[1:] {
		k := string(key)
		if !m.CheckTTL(k) {
			// key expired, treat as non-existent for deletion count
			continue
		}
		m.locks.Lock(k)
		dKeyCount += m.db.Delete(k)
		m.ttl.RemoveTTL(k)
		m.locks.UnLock(k)
	}
	return RESP.MakeIntData(int64(dKeyCount))
}
func existsKey(m *MemDb, cmd [][]byte) RESP.RedisData {
	cmdName := string(cmd[0])
	if strings.ToLower(cmdName) != "exists" || len(cmd) < 2 {
		logger.Error("existsKey Function: cmdName is not exists")
		return RESP.MakeErrorData("protocol error: command is not exists")
	}
	eKeyCount := 0
	for _, keyByte := range cmd[1:] {
		key := string(keyByte)
		if m.CheckTTL(key) {
			m.locks.RLock(key)
			if _, ok := m.db.Get(key); ok {
				eKeyCount++
			}
			m.locks.RUnLock(key)
		}
	}

	return RESP.MakeIntData(int64(eKeyCount))
}
func keysKey(m *MemDb, cmd [][]byte) RESP.RedisData {
	if len(cmd) != 2 {
		logger.Error("keysKey Function: cmd length is not 2")
		return RESP.MakeErrorData(fmt.Sprintf("error: keys function requires exactly 2 arguments, got %d", len(cmd)))
	}
	if strings.ToLower(string(cmd[0])) != "keys" {
		logger.Error("keysKey Function: cmdName is not 'keys'")
		return RESP.MakeErrorData(fmt.Sprintf("error: keys function got invalid command %s", string(cmd[0])))
	}

	res := make([]RESP.RedisData, 0)
	allKeys := m.db.Keys()
	pattern := string(cmd[1])

	for _, key := range allKeys {
		if m.CheckTTL(key) {
			if util.PatternMatch(pattern, key) {
				res = append(res, RESP.MakeBulkData([]byte(key)))
			}
		}
	}
	return RESP.MakeArrayData(res)
}
func expireKey(m *MemDb, cmd [][]byte) RESP.RedisData {
	cmdName := string(cmd[0])
	if strings.ToLower(cmdName) != "expire" || len(cmd) < 3 || len(cmd) > 4 {
		logger.Error("expireKey Function: cmdName is not expire or command args number is invalid")
		return RESP.MakeErrorData("error: cmdName is not expire or command args number is invalid")
	}

	v, err := strconv.ParseInt(string(cmd[2]), 10, 64)
	if err != nil {
		logger.Errorf("expireKey Function: cmd[2] %s is not int", string(cmd[2]))
		return RESP.MakeErrorData(fmt.Sprintf("error: %s is not int", string(cmd[2])))
	}
	ttl := time.Now().Unix() + v
	var opt string
	if len(cmd) == 4 {
		opt = strings.ToLower(string(cmd[3]))
	}
	key := string(cmd[1])
	if !m.CheckTTL(key) {
		return RESP.MakeIntData(int64(0))
	}

	m.locks.Lock(key)
	defer m.locks.UnLock(key)
	var res int
	currentExpire, hasTTL := m.ttl.ExpireAt(key)
	switch opt {
	case "nx":
		if !hasTTL {
			res = m.SetTTL(key, ttl)
		}
	case "xx":
		if hasTTL {
			res = m.SetTTL(key, ttl)
		}
	case "gt":
		if hasTTL && ttl > currentExpire {
			res = m.SetTTL(key, ttl)
		}
	case "lt":
		if hasTTL && ttl < currentExpire {
			res = m.SetTTL(key, ttl)
		}
	default:
		if opt != "" {
			logger.Errorf("expireKey Function: opt %s is not nx, xx, gt or lt", opt)
			return RESP.MakeErrorData(fmt.Sprintf("error: unsupport %s, except nx, xx, gt, lt", opt))
		}
		res = m.SetTTL(key, ttl)
	}
	return RESP.MakeIntData(int64(res))
}
func persistKey(m *MemDb, cmd [][]byte) RESP.RedisData {
	cmdName := string(cmd[0])
	if strings.ToLower(cmdName) != "persist" || len(cmd) != 2 {
		logger.Error("persistKey Function: cmdName is not persist or command args number is invalid")
		return RESP.MakeErrorData("error: cmdName is not persist or command args number is invalid")
	}
	key := string(cmd[1])
	if !m.CheckTTL(key) {
		return RESP.MakeIntData(int64(0))
	}
	m.locks.Lock(key)
	defer m.locks.UnLock(key)
	res := m.DelTTL(key)
	return RESP.MakeIntData(int64(res))
}
func ttlKey(m *MemDb, cmd [][]byte) RESP.RedisData {
	cmdName := string(cmd[0])
	if strings.ToLower(cmdName) != "ttl" || len(cmd) != 2 {
		logger.Error("ttlKey Function: cmdName is not ttl or command args number is invalid")
		return RESP.MakeErrorData("error: cmdName is not ttl or command args number is invalid")
	}
	key := string(cmd[1])
	m.locks.RLock(key)
	defer m.locks.RUnLock(key)
	if _, ok := m.db.Get(key); !ok {
		return RESP.MakeIntData(int64(-2))
	}

	// 获取 TTL 信息
	expireAt, exists := m.ttl.ExpireAt(key)
	if !exists {
		return RESP.MakeIntData(int64(-1))
	}
	now := time.Now().Unix()
	return RESP.MakeIntData(expireAt - now)
}

func typeKey(m *MemDb, cmd [][]byte) RESP.RedisData {
	cmdName := string(cmd[0])
	if strings.ToLower(cmdName) != "type" || len(cmd) != 2 {
		logger.Error("typeKey Function: cmdName is not type or command args number is invalid")
		return RESP.MakeErrorData("error: cmdName is not type or command args number is invalid")
	}
	key := string(cmd[1])
	if !m.CheckTTL(key) {
		return RESP.MakeBulkData([]byte("none"))
	}
	m.locks.RLock(key)
	defer m.locks.RUnLock(key)
	v, ok := m.db.Get(key)
	if !ok {
		return RESP.MakeStringData("none")
	}
	switch v.(type) {
	case []byte:
		return RESP.MakeStringData("string")
	case *List:
		return RESP.MakeStringData("list")
	case *Set:
		return RESP.MakeStringData("set")
	case *Hash:
		return RESP.MakeStringData("hash")
	case *ZSet:
		return RESP.MakeStringData("zset")

	default:
		logger.Error("typeKey Function: type func error, not in string|list|set|hash")
	}
	return RESP.MakeErrorData("unknown error: server error")
}
func renameKey(m *MemDb, cmd [][]byte) RESP.RedisData {
	cmdName := string(cmd[0])
	if strings.ToLower(cmdName) != "rename" || len(cmd) != 3 {
		logger.Error("renameKey Function: cmdName is not rename or command args number is not invalid")
		return RESP.MakeErrorData("error: cmdName is not rename or command args number is not invalid")
	}
	oldName, newName := string(cmd[1]), string(cmd[2])
	if !m.CheckTTL(oldName) {
		return RESP.MakeErrorData(fmt.Sprintf("error: %s not exist", oldName))
	}
	// 需要写入，使用写锁保护两个键，避免并发冲突
	m.locks.LockMulti([]string{oldName, newName})
	defer m.locks.UnLockMulti([]string{oldName, newName})
	oldValue, ok := m.db.Get(oldName)
	if !ok {
		return RESP.MakeErrorData(fmt.Sprintf("error: %s not exist", oldName))
	}
	m.db.Delete(oldName)
	m.ttl.RemoveTTL(oldName)
	m.db.Delete(newName)
	m.ttl.RemoveTTL(newName)
	m.db.Set(newName, oldValue)
	return RESP.MakeStringData("OK")
}
