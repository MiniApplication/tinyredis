package memdb

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hsn0918/tinyredis/pkg/RESP"
	"github.com/hsn0918/tinyredis/pkg/logger"
)

func RegisterHashCommands() {
	RegisterCommand("hdel", hDelHash)
	RegisterCommand("hexists", hExistsHash)
	RegisterCommand("hget", hGetHash)
	RegisterCommand("hgetall", hGetAllHash)
	RegisterCommand("hincrby", hIncrByHash)
	RegisterCommand("hincrbyfloat", hIncrByFloatHash)
	RegisterCommand("hkeys", hKeysHash)
	RegisterCommand("hlen", hLenHash)
	RegisterCommand("hmget", hMGetHash)
	RegisterCommand("hset", hSetHash)
	RegisterCommand("hsetnx", hSetNxHash)
	RegisterCommand("hvals", hValsHash)
	RegisterCommand("hstrlen", hStrLenHash)
	RegisterCommand("hrandfield", hRandFieldHash)
}

func hRandFieldHash(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "hrandfield" {
		logger.Error("hRandFieldHash Function: cmdName is not hrandfield")
		return RESP.MakeErrorData("server error")
	}

	if len(cmd) != 2 && len(cmd) != 3 && len(cmd) != 4 {
		return RESP.MakeErrorData("wrong number of arguments for 'hrandfield' command")
	}
	key := string(cmd[1])
	count := 1
	withValues := false
	var err error
	if len(cmd) >= 3 {
		count, err = strconv.Atoi(string(cmd[2]))
		if err != nil {
			return RESP.MakeErrorData("err: count value must be integer")
		}
	}
	if len(cmd) == 4 {
		if strings.ToLower(string(cmd[3])) == "withvalues" {
			withValues = true
		} else {
			return RESP.MakeErrorData(fmt.Sprintf("command option error, not support %s option", string(cmd[3])))
		}
	}
	if !m.CheckTTL(key) {
		return RESP.MakeEmptyArrayData()
	}
	m.locks.RLock(key)
	defer m.locks.RUnLock(key)
	tem, ok := m.db.Get(key)
	if !ok {
		return RESP.MakeEmptyArrayData()
	}
	hash, ok := tem.(*Hash)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	res := make([]RESP.RedisData, 0)
	if withValues {
		fields := hash.RandomWithValue(count)
		for _, v := range fields {
			res = append(res, RESP.MakeBulkData(v))
		}
	} else {
		fields := hash.Random(count)
		for _, v := range fields {
			res = append(res, RESP.MakeBulkData([]byte(v)))
		}
	}
	return RESP.MakeArrayData(res)
}

func hStrLenHash(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "hstrlen" {
		logger.Error("hStrLenHash Function: cmdName is not hstrlen")
		return RESP.MakeErrorData("server error")
	}
	if len(cmd) != 3 {
		return RESP.MakeErrorData("wrong number of arguments for 'hstrlen' command")
	}
	key := string(cmd[1])
	field := string(cmd[2])

	if !m.CheckTTL(key) {
		return RESP.MakeIntData(0)
	}

	m.locks.RLock(key)
	defer m.locks.RUnLock(key)
	tem, ok := m.db.Get(key)
	if !ok {
		return RESP.MakeIntData(0)
	}
	hash, ok := tem.(*Hash)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	res := hash.StrLen(field)

	return RESP.MakeIntData(int64(res))
}

func hSetNxHash(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "hsetnx" {
		logger.Error("hSetNxHash Function: cmdName is not hsetnx")
		return RESP.MakeErrorData("server error")
	}
	if len(cmd) != 4 {
		return RESP.MakeErrorData("wrong number of arguments for 'hsetnx' command")
	}
	key := string(cmd[1])
	field := string(cmd[2])
	value := cmd[3]
	m.CheckTTL(key)

	m.locks.Lock(key)
	defer m.locks.UnLock(key)
	var hash *Hash
	tem, ok := m.db.Get(key)
	if !ok {
		hash = NewHash()
		m.db.Set(key, hash)
	} else {
		hash, ok = tem.(*Hash)
		if !ok {
			return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
	}

	if hash.Exist(field) {
		return RESP.MakeIntData(0)
	}

	hash.Set(field, value)
	return RESP.MakeIntData(1)

}

func hIncrByFloatHash(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "hincrbyfloat" {
		logger.Error("hIncrByFloatHash Function: cmdName is not hincrbyfloat")
		return RESP.MakeErrorData("server error")
	}

	if len(cmd) != 4 {
		return RESP.MakeErrorData("wrong number of arguments for 'hincrbyfloat' command")
	}

	var hash *Hash
	key, field := string(cmd[1]), string(cmd[2])
	incr, err := strconv.ParseFloat(string(cmd[3]), 64)
	if err != nil {
		return RESP.MakeErrorData("incr value must be a float")
	}
	m.CheckTTL(key)

	m.locks.Lock(key)
	defer m.locks.UnLock(key)

	tem, ok := m.db.Get(key)
	if !ok {
		hash = NewHash()
		m.db.Set(key, hash)
	} else {
		hash, ok = tem.(*Hash)
		if !ok {
			return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
	}

	res, ok := hash.IncrByFloat(field, incr)
	if !ok {
		return RESP.MakeErrorData("value is not a float")
	}

	return RESP.MakeBulkData([]byte(strconv.FormatFloat(res, 'f', -1, 64)))
}

func hIncrByHash(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "hincrby" {
		logger.Error("hIncrByHash Function: cmdName is not hincrby")
		return RESP.MakeErrorData("server error")
	}
	if len(cmd) != 4 {
		return RESP.MakeErrorData("wrong number of arguments for 'hincrby' command")
	}
	var incr int
	var err error
	var hash *Hash
	key := string(cmd[1])
	field := string(cmd[2])
	incr, err = strconv.Atoi(string(cmd[3]))
	if err != nil {
		return RESP.MakeErrorData("incr value must be an integer")
	}
	m.CheckTTL(key)
	m.locks.Lock(key)
	defer m.locks.UnLock(key)
	tem, ok := m.db.Get(key)
	if !ok {
		hash = NewHash()
		m.db.Set(key, hash)
	} else {
		hash, ok = tem.(*Hash)
		if !ok {
			return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
	}
	res, ok := hash.IncrBy(field, incr)
	if !ok {
		return RESP.MakeErrorData("value is not an integer")
	}
	return RESP.MakeIntData(int64(res))
}

func hValsHash(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "hvals" {
		logger.Error("hValsHash Function: cmdName is not hvals")
		return RESP.MakeErrorData("server error")
	}
	if len(cmd) != 2 {
		return RESP.MakeErrorData("wrong number of arguments for 'hvals' command")
	}
	key := string(cmd[1])
	if !m.CheckTTL(key) {
		return RESP.MakeEmptyArrayData()
	}
	m.locks.RLock(key)
	defer m.locks.RUnLock(key)
	tem, ok := m.db.Get(key)
	if !ok {
		return RESP.MakeEmptyArrayData()
	}
	hash, ok := tem.(*Hash)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	vals := hash.Values()
	res := make([]RESP.RedisData, 0, len(vals))
	for _, val := range vals {
		res = append(res, RESP.MakeBulkData(val))
	}
	return RESP.MakeArrayData(res)
}

func hKeysHash(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "hkeys" {
		logger.Error("hKeysHash Function: cmdName is not hkeys")
		return RESP.MakeErrorData("server error")
	}
	if len(cmd) != 2 {
		return RESP.MakeErrorData("wrong number of arguments for 'hkeys' command")
	}
	key := string(cmd[1])
	if !m.CheckTTL(key) {
		return RESP.MakeEmptyArrayData()
	}
	m.locks.RLock(key)
	defer m.locks.RUnLock(key)
	tem, ok := m.db.Get(key)
	if !ok {
		return RESP.MakeEmptyArrayData()
	}
	hash, ok := tem.(*Hash)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	fields := hash.Keys()
	res := make([]RESP.RedisData, 0, len(fields))
	for _, v := range fields {
		res = append(res, RESP.MakeBulkData([]byte(v)))
	}
	return RESP.MakeArrayData(res)
}
func hLenHash(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "hlen" {
		logger.Error("hLenHash Function: cmdName is not hlen")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) != 2 {
		return RESP.MakeErrorData("wrong number of arguments for 'hlen' command")
	}
	key := string(cmd[1])
	if !m.CheckTTL(key) {
		return RESP.MakeIntData(0)
	}
	m.locks.RLock(key)
	defer m.locks.RUnLock(key)

	tem, ok := m.db.Get(key)
	if !ok {
		return RESP.MakeIntData(0)
	}
	hash, ok := tem.(*Hash)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	res := hash.Len()
	return RESP.MakeIntData(int64(res))
}
func hSetHash(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "hset" {
		logger.Error("hSetHash Function: cmdName is not hset")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) < 4 || len(cmd)&1 == 1 {
		return RESP.MakeErrorData("wrong number of arguments for 'hset' command")
	}
	key := string(cmd[1])
	m.CheckTTL(key)

	m.locks.Lock(key)
	defer m.locks.UnLock(key)
	var hash *Hash
	tem, ok := m.db.Get(key)
	if !ok {
		hash = NewHash()
		m.db.Set(key, hash)
	} else {
		hash, ok = tem.(*Hash)
		if !ok {
			return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
	}
	// 优化：批量设置
	addedCount := 0
	for i := 2; i < len(cmd); i += 2 {
		field := string(cmd[i])
		value := cmd[i+1]
		if _, exists := hash.table[field]; !exists {
			addedCount++
		}
		hash.Set(field, value)
	}
	return RESP.MakeIntData(int64(addedCount))
}
func hGetHash(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "hget" {
		logger.Error("hGetHash Function: command name is not hget")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) != 3 {
		return RESP.MakeErrorData("wrong number of arguments for 'hget' command")
	}
	key := string(cmd[1])
	if !m.CheckTTL(key) {
		return RESP.MakeNullBulkData()
	}
	m.locks.RLock(key)
	defer m.locks.RUnLock(key)
	tem, ok := m.db.Get(key)
	if !ok {
		return RESP.MakeNullBulkData()
	}
	hash, ok := tem.(*Hash)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	res := hash.Get(string(cmd[2]))
	if len(res) == 0 {
		return RESP.MakeBulkData(nil)
	}
	return RESP.MakeBulkData(res)
}
func hMGetHash(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "hmget" {
		logger.Error("hMGetHash Function: cmdName is not hmget")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) < 3 {
		return RESP.MakeErrorData("wrong number of arguments for 'hmget' command")
	}
	key := string(cmd[1])
	if !m.CheckTTL(key) {
		// 优化：直接返回null元素数组
		res := make([]RESP.RedisData, len(cmd)-2)
		for i := range res {
			res[i] = RESP.MakeBulkData(nil)
		}
		return RESP.MakeArrayData(res)
	}
	m.locks.RLock(key)
	defer m.locks.RUnLock(key)
	tem, ok := m.db.Get(key)
	if !ok {
		res := make([]RESP.RedisData, len(cmd)-2)
		for i := range res {
			res[i] = RESP.MakeBulkData(nil)
		}
		return RESP.MakeArrayData(res)
	}
	hash, ok := tem.(*Hash)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	// 优化：预分配精确大小
	res := make([]RESP.RedisData, len(cmd)-2)
	for i := 2; i < len(cmd); i++ {
		field := string(cmd[i])
		data := hash.Get(field)
		if len(data) == 0 {
			res[i-2] = RESP.MakeBulkData(nil)
		} else {
			res[i-2] = RESP.MakeBulkData(data)
		}
	}
	return RESP.MakeArrayData(res)
}
func hDelHash(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "hdel" {
		logger.Error("hDelHash Function: cmdName is not hdel")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) < 3 {
		return RESP.MakeErrorData("wrong number of arguments for 'hdel' command")
	}
	key := string(cmd[1])
	if !m.CheckTTL(key) {
		return RESP.MakeIntData(0)
	}
	m.locks.Lock(key)
	defer m.locks.UnLock(key)
	tem, ok := m.db.Get(key)
	if !ok {
		return RESP.MakeIntData(0)
	}
	hash, ok := tem.(*Hash)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	defer func() {
		if hash.IsEmpty() {
			m.db.Delete(key)
			m.DelTTL(key)
		}
	}()
	res := 0
	for i := 2; i < len(cmd); i++ {
		res += hash.Del(string(cmd[i]))
	}

	return RESP.MakeIntData(int64(res))
}
func hExistsHash(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "hexists" {
		logger.Error("hExistsHash Function: cmdName is not hexists")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) != 3 {
		return RESP.MakeErrorData("wrong number of arguments for 'hexists' command")
	}
	key := string(cmd[1])
	if !m.CheckTTL(key) {
		return RESP.MakeIntData(0)
	}
	m.locks.RLock(key)
	defer m.locks.RUnLock(key)
	tem, ok := m.db.Get(key)
	if !ok {
		return RESP.MakeIntData(0)
	}
	hash, ok := tem.(*Hash)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	if hash.Exist(string(cmd[2])) {
		return RESP.MakeIntData(1)
	}
	return RESP.MakeIntData(0)
}
func hGetAllHash(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "hgetall" {
		logger.Error("hGetAllHash Function: cmdName is not hgetall")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) != 2 {
		return RESP.MakeErrorData("wrong number of arguments for 'hgetall' command")
	}
	key := string(cmd[1])
	if !m.CheckTTL(key) {
		return RESP.MakeEmptyArrayData()
	}
	m.locks.RLock(key)
	defer m.locks.RUnLock(key)
	tem, ok := m.db.Get(key)
	if !ok {
		return RESP.MakeEmptyArrayData()
	}
	hash, ok := tem.(*Hash)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	table := hash.Table()

	res := make([]RESP.RedisData, 0, len(table)*2)
	for k, v := range table {
		res = append(res, RESP.MakeBulkData([]byte(k)), RESP.MakeBulkData(v))
	}
	return RESP.MakeArrayData(res)
}
