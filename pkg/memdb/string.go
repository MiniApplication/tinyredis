package memdb

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hsn0918/tinyredis/pkg/RESP"
	"github.com/hsn0918/tinyredis/pkg/logger"
)

func RegisterStringCommands() {
	RegisterCommand("set", setString)
	RegisterCommand("get", getString)
	RegisterCommand("setrange", setRangeString)
	RegisterCommand("getrange", getRangeString)
	RegisterCommand("mset", mSetString)
	RegisterCommand("mget", mGetString)
	RegisterCommand("setex", setExString)
	RegisterCommand("setnx", setNxString)
	RegisterCommand("strlen", strLenString)
	RegisterCommand("incr", incrString)
	RegisterCommand("incrby", incrByString)
	RegisterCommand("decr", decrString)
	RegisterCommand("decrby", decrByString)
	RegisterCommand("incrbyfloat", incrByFloatString)
	RegisterCommand("append", appendString)
}

func setString(m *MemDb, cmd [][]byte) RESP.RedisData {
	cmdName := string(cmd[0])
	if strings.ToLower(cmdName) != "set" {
		logger.Error("setString Function: is not set")
		return RESP.MakeErrorData("server error ")

	}
	if len(cmd) < 3 {
		return RESP.MakeErrorData("error: commands is invalid")
	}
	// if key is expired,delete key
	m.CheckTTL(string(cmd[1]))
	var err error
	// nx 表示是否使用 SET 命令的 nx 选项，只在键不存在时设置键值对
	// xx 表示是否使用 SET 命令的 xx 选项，只在键已经存在时设置键值对
	// get 表示是否使用 SET 命令的 get 选项，如果为 true，需要返回原始键的值
	// ex 表示是否使用 SET 命令的 ex 选项，如果为 true，需要设置键的过期时间
	// keepttl 表示是否使用 SET 命令的 keepttl 选项，如果为 true，需要保持键的原有 TTL 不变
	var nx, xx, get, ex, keepttl bool
	var exval int64
	for i := 3; i < len(cmd); i++ {
		switch strings.ToLower(string(cmd[i])) {
		case "nx":
			nx = true
		case "xx":
			xx = true
		case "get":
			get = true
		case "keepttl":
			keepttl = true
		case "ex":
			ex = true
			i++
			if i >= len(cmd) {
				return RESP.MakeErrorData("error: commands is invalid")
			}
			exval, err = strconv.ParseInt(string(cmd[i]), 10, 64)
			if err != nil {
				return RESP.MakeErrorData(fmt.Sprintf("error: commands is invalid, %s is not interger", string(cmd[i])))
			}
		default:
			return RESP.MakeErrorData("Error unsupported option: " + string(cmd[i]))
		}
	}
	if (nx && xx) || (ex && keepttl) {
		return RESP.MakeErrorData("error: commands is invalid")
	}
	m.locks.Lock(string(cmd[1]))
	defer m.locks.UnLock(string(cmd[1]))
	var res RESP.RedisData

	oldVal, oldOk := m.db.Get(string(cmd[1]))
	// check is string
	var oldTypeVal []byte
	var typeOk bool
	if oldOk {
		oldTypeVal, typeOk = oldVal.([]byte)
		if !typeOk {
			return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
	}
	if nx || xx {
		if nx {
			if oldOk {
				res = RESP.MakeNullBulkData()
			} else {
				m.db.Set(string(cmd[1]), cmd[2])
				res = RESP.MakeStringData("OK")
			}
		} else {
			if oldOk {
				m.db.Set(string(cmd[1]), cmd[2])
				res = RESP.MakeStringData("OK")
			} else {
				res = RESP.MakeNullBulkData()
			}
		}
	} else {
		m.db.Set(string(cmd[1]), cmd[2])
		res = RESP.MakeStringData("OK")
	}
	if get {
		if !oldOk {
			res = RESP.MakeNullBulkData()
		} else {
			res = RESP.MakeBulkData(oldTypeVal)
		}
	}
	if !keepttl {
		m.DelTTL(string(cmd[1]))
	}
	if ex {
		m.SetTTL(string(cmd[1]), exval+time.Now().Unix())
	}
	return res
}

func getString(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "get" {
		logger.Error("getString Function: cmdName is not get")
		return RESP.MakeErrorData("server error")
	}
	if len(cmd) != 2 {
		return RESP.MakeErrorData("error: commands is invalid")
	}
	key := string(cmd[1])
	if !m.CheckTTL(key) {
		return RESP.MakeNullBulkData()
	}
	m.locks.RLock(key)
	defer m.locks.RUnLock(key)
	val, ok := m.db.Get(key)
	if !ok {
		return RESP.MakeNullBulkData()
	}
	byteVal, ok := val.([]byte)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	return RESP.MakeBulkData(byteVal)
}

func setRangeString(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "setrange" {
		logger.Error("setRangeString Function: cmdName is not setrange")
		return RESP.MakeErrorData("server error")
	}
	if len(cmd) != 4 {
		return RESP.MakeErrorData("error: commands is invalid")
	}

	offset, err := strconv.Atoi(string(cmd[2]))
	if err != nil || offset < 0 {
		return RESP.MakeErrorData("error: offset is not a integer or less than 0")
	}
	var oldVal []byte
	key := string(cmd[1])
	m.CheckTTL(key)
	m.locks.Lock(key)
	defer m.locks.UnLock(key)
	val, ok := m.db.Get(key)
	if !ok {
		oldVal = make([]byte, 0)
	} else {
		oldVal, ok = val.([]byte)
		if !ok {
			return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
	}
	requiredLen := offset + len(cmd[3])
	if requiredLen < len(oldVal) {
		requiredLen = len(oldVal)
	}
	newVal := make([]byte, requiredLen)
	copy(newVal, oldVal)
	copy(newVal[offset:], cmd[3])
	m.db.Set(key, newVal)
	return RESP.MakeIntData(int64(len(newVal)))
}

func getRangeString(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "getrange" {
		logger.Error("getRangeString Function: cmdName is not getrange")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) != 4 {
		return RESP.MakeErrorData("error: commands is not invalid")
	}
	key := string(cmd[1])
	if !m.CheckTTL(key) {
		return RESP.MakeNullBulkData()
	}
	m.locks.RLock(key)
	defer m.locks.RUnLock(key)
	val, ok := m.db.Get(key)
	if !ok {
		return RESP.MakeNullBulkData()
	}
	byteVal, ok := val.([]byte)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	start, err := strconv.Atoi(string(cmd[2]))
	if err != nil {
		return RESP.MakeErrorData("error: commands is invalid")
	}
	end, err := strconv.Atoi(string(cmd[3]))
	if err != nil {
		return RESP.MakeErrorData("error: commands is invalid")
	}
	length := len(byteVal)
	if length == 0 {
		return RESP.MakeBulkData([]byte{})
	}
	if start < 0 {
		start = length + start
	}
	if end < 0 {
		end = length + end
	}
	if start < 0 {
		start = 0
	}
	if start > length {
		return RESP.MakeBulkData([]byte{})
	}
	if end >= length {
		end = length - 1
	}
	if end < start {
		return RESP.MakeBulkData([]byte{})
	}
	return RESP.MakeBulkData(byteVal[start : end+1])
}

func mSetString(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "mset" {
		logger.Error("mSetString Function: cmdName is not mset")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) < 3 || len(cmd)&1 != 1 {
		return RESP.MakeErrorData("error; commands is invalid")
	}
	keys := make([]string, 0)
	vals := make([][]byte, 0)
	for i := 1; i < len(cmd); i += 2 {
		keys = append(keys, string(cmd[i]))
		vals = append(vals, cmd[i+1])
	}
	m.locks.LockMulti(keys)
	defer m.locks.UnLockMulti(keys)
	for i := 0; i < len(keys); i++ {
		m.DelTTL(keys[i])
		m.db.Set(keys[i], vals[i])
	}
	return RESP.MakeStringData("OK")
}

func mGetString(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "mget" {
		logger.Error("mGetString Function: cmdName is not mget")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) < 2 {
		return RESP.MakeErrorData("error: commands is invalid")
	}
	res := make([]RESP.RedisData, 0)
	for i := 1; i < len(cmd); i++ {
		key := string(cmd[i])
		if !m.CheckTTL(key) {
			res = append(res, RESP.MakeNullBulkData())
			continue
		}
		m.locks.RLock(key)
		val, ok := m.db.Get(key)
		m.locks.RUnLock(key)
		if !ok {
			res = append(res, RESP.MakeNullBulkData())
		} else {
			byteVal, ok := val.([]byte)
			if !ok {
				res = append(res, RESP.MakeNullBulkData())
			} else {
				res = append(res, RESP.MakeBulkData(byteVal))
			}
		}

	}
	return RESP.MakeArrayData(res)
}

func setExString(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "setex" {
		logger.Error("setExString Function: cmdName is not setEx")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) != 4 {
		return RESP.MakeErrorData("error; commands is invalid")
	}
	ex, err := strconv.ParseInt(string(cmd[2]), 10, 64)
	if err != nil {
		return RESP.MakeErrorData(fmt.Sprintf("error: %s is not a integer", string(cmd[2])))
	}
	ttl := time.Now().Unix() + ex
	key := string(cmd[1])
	val := cmd[3]
	m.locks.Lock(key)
	defer m.locks.UnLock(key)
	m.db.Set(key, val)
	m.SetTTL(key, ttl)
	return RESP.MakeStringData("OK")
}

func setNxString(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "setnx" {
		logger.Error("setNxString Function: commands is invalid")
		return RESP.MakeErrorData("Server")
	}
	if len(cmd) != 3 {
		return RESP.MakeErrorData("error: commands is in valid")
	}
	key := string(cmd[1])
	val := cmd[2]
	m.CheckTTL(key)
	m.locks.Lock(key)
	defer m.locks.UnLock(key)
	res := m.db.SetIfNotExist(key, val)
	return RESP.MakeIntData(int64(res))
}

func strLenString(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "strlen" {
		logger.Error("strLenString Function: cmdName is not strlen")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) != 2 {
		return RESP.MakeErrorData("error: commands is invalid")
	}
	key := string(cmd[1])
	m.CheckTTL(key)
	m.locks.RLock(key)
	defer m.locks.RUnLock(key)
	val, ok := m.db.Get(key)
	if !ok {
		return RESP.MakeNullBulkData()
	}
	byteVal, ok := val.([]byte)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of val")
	}
	return RESP.MakeIntData(int64(len(byteVal)))
}

func incrString(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "incr" {
		logger.Error("incrString Function: cmdName is not incr")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) != 2 {
		return RESP.MakeErrorData("error: commands is invalid")
	}
	key := string(cmd[1])
	m.CheckTTL(key)
	m.locks.Lock(key)
	defer m.locks.UnLock(key)
	val, ok := m.db.Get(key)
	if !ok {
		m.db.Set(key, []byte("1"))
		return RESP.MakeIntData(1)
	}
	typeVal, ok := val.([]byte)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	intVal, err := strconv.ParseInt(string(typeVal), 10, 64)
	if err != nil {
		return RESP.MakeErrorData("value is not an integer")
	}
	intVal++
	m.db.Set(key, []byte(strconv.FormatInt(intVal, 10)))
	return RESP.MakeIntData(intVal)
}

func incrByString(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "incrby" {
		logger.Error("incrByString Funcction: cmdName is not incrby")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) != 3 {
		return RESP.MakeErrorData("error: commands is invalid")
	}
	key := string(cmd[1])
	inc, err := strconv.ParseInt(string(cmd[2]), 10, 64)
	if err != nil {
		return RESP.MakeErrorData("commands invalid: increment value is not an integer")
	}
	m.CheckTTL(key)

	m.locks.Lock(key)
	defer m.locks.UnLock(key)
	val, ok := m.db.Get(key)
	if !ok {
		m.db.Set(key, []byte(strconv.FormatInt(inc, 10)))
		return RESP.MakeIntData(inc)
	}
	typeVal, ok := val.([]byte)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	intVal, err := strconv.ParseInt(string(typeVal), 10, 64)
	if err != nil {
		return RESP.MakeErrorData("value is not an integer")
	}
	intVal += inc
	m.db.Set(key, []byte(strconv.FormatInt(intVal, 10)))
	return RESP.MakeIntData(intVal)
}

func decrString(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "decr" {
		logger.Error("decrString Function: cmdName is not decr")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) != 2 {
		return RESP.MakeErrorData("error: commands is invalid")
	}
	key := string(cmd[1])
	m.CheckTTL(key)

	m.locks.Lock(key)
	defer m.locks.UnLock(key)
	val, ok := m.db.Get(key)
	if !ok {
		m.db.Set(key, []byte("-1"))
		return RESP.MakeIntData(-1)
	}
	typeVal, ok := val.([]byte)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	intVal, err := strconv.ParseInt(string(typeVal), 10, 64)
	if err != nil {
		return RESP.MakeErrorData("value is not an integer")
	}
	intVal--
	m.db.Set(key, []byte(strconv.FormatInt(intVal, 10)))
	return RESP.MakeIntData(intVal)
}

func decrByString(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "decrby" {
		logger.Error("decrByString Function: cmdName is not decrby")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) != 3 {
		return RESP.MakeErrorData("error: commands is invalid")
	}
	key := string(cmd[1])
	dec, err := strconv.ParseInt(string(cmd[2]), 10, 64)
	if err != nil {
		return RESP.MakeErrorData("commands invalid: increment value is not an integer")
	}
	m.CheckTTL(key)

	m.locks.Lock(key)
	defer m.locks.UnLock(key)
	val, ok := m.db.Get(key)
	if !ok {
		m.db.Set(key, []byte(strconv.FormatInt(-dec, 10)))
		return RESP.MakeIntData(-dec)
	}
	typeVal, ok := val.([]byte)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	intVal, err := strconv.ParseInt(string(typeVal), 10, 64)
	if err != nil {
		return RESP.MakeErrorData("value is not an integer")
	}
	intVal -= dec
	m.db.Set(key, []byte(strconv.FormatInt(intVal, 10)))
	return RESP.MakeIntData(intVal)
}

func incrByFloatString(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "incrbyfloat" {
		logger.Error("incrByFloatString Function: cmdName is not incrbyfloat")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) != 3 {
		return RESP.MakeErrorData("error: commands is invalid")
	}

	key := string(cmd[1])
	inc, err := strconv.ParseFloat(string(cmd[2]), 64)
	if err != nil {
		return RESP.MakeErrorData("commands invalid: increment value is not an float")
	}

	m.CheckTTL(key)

	m.locks.Lock(key)
	defer m.locks.UnLock(key)

	val, ok := m.db.Get(key)
	if !ok {
		m.db.Set(key, []byte(strconv.FormatFloat(inc, 'f', -1, 64)))
		return RESP.MakeBulkData([]byte(strconv.FormatFloat(inc, 'f', -1, 64)))
	}
	typeVal, ok := val.([]byte)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	floatVal, err := strconv.ParseFloat(string(typeVal), 64)
	if err != nil {
		return RESP.MakeErrorData("value is not an float")
	}
	floatVal += inc
	m.db.Set(key, []byte(strconv.FormatFloat(floatVal, 'f', -1, 64)))
	return RESP.MakeBulkData([]byte(strconv.FormatFloat(floatVal, 'f', -1, 64)))
}

func appendString(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "append" {
		logger.Error("appendString Function: cmdName is not append")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) != 3 {
		return RESP.MakeErrorData("error: commands is invalid")
	}
	key := string(cmd[1])
	val := cmd[2]
	m.CheckTTL(key)

	m.locks.Lock(key)
	defer m.locks.UnLock(key)
	oldVal, ok := m.db.Get(key)
	if !ok {
		m.db.Set(key, val)
		return RESP.MakeIntData(int64(len(val)))
	}
	typeVal, ok := oldVal.([]byte)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	newVal := append(typeVal, val...)
	m.db.Set(key, newVal)
	return RESP.MakeIntData(int64(len(newVal)))
}
