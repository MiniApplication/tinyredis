package memdb

import (
	"math"
	"strconv"
	"strings"

	"github.com/hsn0918/tinyredis/pkg/RESP"
	"github.com/hsn0918/tinyredis/pkg/logger"
)

func RegisterSetCommands() {
	RegisterCommand("sadd", sAddSet)
	RegisterCommand("scard", sCardSet)
	RegisterCommand("sdiff", sDiffSet)
	RegisterCommand("sdiffstore", sDiffStoreSet)
	RegisterCommand("sinter", sInterSet)
	RegisterCommand("sinterstore", sInterStoreSet)
	RegisterCommand("sismember", sIsMemberSet)
	RegisterCommand("smembers", sMembersSet)
	RegisterCommand("smove", sMoveSet)
	RegisterCommand("spop", sPopSet)
	RegisterCommand("srandmember", sRandMemberSet)
	RegisterCommand("srem", sRemSet)
	RegisterCommand("sunion", sUnionSet)
	RegisterCommand("sunionstore", sUnionStoreSet)
	//RegisterCommand("sscan", sScanSet)
}

// sScanSet
//func sScanSet(m *MemDb, cmd [][]byte) RESP.RedisData {
//	if strings.ToLower(string(cmd[0])) != "sscan" {
//		logger.Error("sScanSet Function: cmdName is not sscan")
//		return RESP.MakeErrorData("Server error")
//	}
//	if len(cmd) < 3 {
//		return RESP.MakeErrorData("wrong number of arguments for 'sscan' command")
//	}
//	key := string(cmd[1])
//	cursor, err := strconv.Atoi(string(cmd[2]))
//	if err != nil {
//		return RESP.MakeErrorData("cursor value must be an integer")
//	}
//	if !m.CheckTTL(key) {
//		return RESP.MakeArrayData([]RESP.RedisData{RESP.MakeIntData(0), RESP.MakeEmptyArrayData()})
//	}
//
//	m.locks.RLock(key)
//	defer m.locks.RUnLock(key)
//	tem, ok := m.db.Get(key)
//	if !ok {
//		return RESP.MakeArrayData([]RESP.RedisData{RESP.MakeIntData(0), RESP.MakeEmptyArrayData()})
//	}
//	sets, ok := tem.(*Set)
//	if !ok {
//		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
//	}
//
//	members := sets.Members()
//
//	start := 0
//	if cursor > 0 {
//		start = cursor
//	}
//
//	var end int
//	if len(members) < start+10 {
//		end = len(members)
//	} else {
//		end = start + 10
//	}
//
//	data := make([]RESP.RedisData, end-start)
//	for i := start; i < end; i++ {
//		data[i-start] = RESP.MakeBulkData([]byte(members[i]))
//	}
//
//	nextCursor := end
//	if end == len(members) {
//		nextCursor = 0
//	}
//
//	return RESP.MakeArrayData([]RESP.RedisData{RESP.MakeIntData(int64(nextCursor)), RESP.MakeArrayData(data)})
//}

func sUnionStoreSet(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "sunionstore" {
		logger.Error("sUnionStoreSet Function: cmdName is not sunionstore")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) < 3 {
		return RESP.MakeErrorData("wrong number of arguments for 'sunionstore' command")
	}
	// first check if the destination type is a set. if not, return error immediately.
	desKey := string(cmd[1])
	m.CheckTTL(desKey)
	tem, ok := m.db.Get(desKey)
	if ok {
		_, ok = tem.(*Set)
		if !ok {
			return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
	}
	// second, get the union set
	keys := make([]string, 0)
	for i := 2; i < len(cmd); i++ {
		key := string(cmd[i])
		if m.CheckTTL(key) {
			keys = append(keys, string(cmd[i]))
		}
	}
	if len(keys) == 0 {
		return RESP.MakeIntData(0)
	}
	m.locks.RLockMulti(keys)
	sets := make([]*Set, 0)
	for _, key := range keys {
		tem, ok := m.db.Get(key)
		if !ok {
			continue
		}
		set, ok := tem.(*Set)
		if !ok {
			m.locks.RUnLockMulti(keys)
			return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
		sets = append(sets, set)
	}
	if len(sets) == 0 {
		m.locks.RUnLockMulti(keys)
		return RESP.MakeEmptyArrayData()
	}

	resSet := sets[0].Union(sets[1:]...)
	m.locks.RUnLockMulti(keys)
	// third, set the destination key
	m.CheckTTL(desKey)
	m.locks.Lock(desKey)
	m.locks.UnLock(desKey)
	tem, ok = m.db.Get(desKey)
	if ok {
		_, ok = tem.(*Set)
		if !ok {
			return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
	}
	if resSet.Len() != 0 {
		m.db.Set(desKey, resSet)
	}
	return RESP.MakeIntData(int64(resSet.Len()))

}

func sUnionSet(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "sunion" {
		logger.Error("sUnionSet Function: cmdName is not sunion")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) < 2 {
		return RESP.MakeErrorData("wrong number of arguments for 'sunion' command")
	}

	keys := make([]string, 0)

	for i := 1; i < len(cmd); i++ {
		key := string(cmd[i])
		if m.CheckTTL(key) {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return RESP.MakeEmptyArrayData()
	}

	m.locks.RLockMulti(keys)
	defer m.locks.RUnLockMulti(keys)
	sets := make([]*Set, 0)
	for _, key := range keys {
		tem, ok := m.db.Get(key)
		if !ok {
			continue
		}
		set, ok := tem.(*Set)
		if !ok {
			return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
		sets = append(sets, set)
	}
	if len(sets) == 0 {
		return RESP.MakeEmptyArrayData()
	}

	resSet := sets[0].Union(sets[1:]...)
	res := make([]RESP.RedisData, 0, resSet.Len())
	for _, member := range resSet.Members() {
		res = append(res, RESP.MakeBulkData([]byte(member)))
	}
	return RESP.MakeArrayData(res)

}

func sRemSet(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "srem" {
		logger.Error("sRemSet Function: cmdName is not srem")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) < 3 {
		return RESP.MakeErrorData("wrong number of arguments for 'srem' command")
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

	set, ok := tem.(*Set)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	defer func() {
		if set.Len() == 0 {
			m.db.Delete(key)
			m.DelTTL(key)
		}
	}()

	res := 0
	for i := 2; i < len(cmd); i++ {
		member := string(cmd[i])
		res += set.Remove(member)
	}

	return RESP.MakeIntData(int64(res))
}

func sRandMemberSet(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "srandmember" {
		logger.Error("sRandMemberSet Function: cmdName is not srandmember")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) != 2 && len(cmd) != 3 {
		return RESP.MakeErrorData("wrong number of arguments for 'srandmember' command")
	}
	var count int
	var err error
	if len(cmd) == 3 {
		count, err = strconv.Atoi(string(cmd[2]))
		if err != nil {
			return RESP.MakeErrorData("count value must be an integer")
		}
	} else {
		count = 1
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

	set, ok := tem.(*Set)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	resMembers := set.Random(count)
	if len(resMembers) == 0 {
		return RESP.MakeNullBulkData()
	}
	res := make([]RESP.RedisData, 0, len(resMembers))
	for _, member := range resMembers {
		res = append(res, RESP.MakeBulkData([]byte(member)))
	}

	return RESP.MakeArrayData(res)
}

func sPopSet(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "spop" {
		logger.Error("sPopSet Function: cmdName is not spop")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) != 2 && len(cmd) != 3 {
		return RESP.MakeErrorData("wrong number of arguments for 'spop' command")
	}
	var count int
	var err error
	if len(cmd) == 3 {
		count, err = strconv.Atoi(string(cmd[2]))
		if err != nil || count < 0 {
			return RESP.MakeErrorData("count value must be a positive integer")
		}
	} else {
		count = 1
	}

	key := string(cmd[1])

	if !m.CheckTTL(key) {
		return RESP.MakeNullBulkData()
	}

	m.locks.Lock(key)
	defer m.locks.UnLock(key)
	tem, ok := m.db.Get(key)
	if !ok {
		return RESP.MakeNullBulkData()
	}
	set, ok := tem.(*Set)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	if count == 0 {
		return RESP.MakeEmptyArrayData()
	}

	defer func() {
		if set.Len() == 0 {
			m.db.Delete(key)
			m.DelTTL(key)
		}
	}()
	res := make([]RESP.RedisData, 0)
	if count == 1 {
		val := set.Pop()
		return RESP.MakeBulkData([]byte(val))
	} else {
		for i := 0; i < count; i++ {
			val := set.Pop()
			if val == "" {
				break
			}
			res = append(res, RESP.MakeBulkData([]byte(val)))
		}
	}
	return RESP.MakeArrayData(res)
}

func sMoveSet(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "smove" {
		logger.Error("sMoveSet Function: cmdName is not smove")
		return RESP.MakeErrorData("Server error")
	}

	if len(cmd) != 4 {
		return RESP.MakeErrorData("wrong number of arguments for 'smove' command")
	}

	srcKey := string(cmd[1])
	desKey := string(cmd[2])
	val := string(cmd[3])
	keys := []string{srcKey, desKey}
	m.CheckTTL(desKey)
	if !m.CheckTTL(srcKey) {
		return RESP.MakeIntData(0)
	}

	m.locks.LockMulti(keys)
	defer m.locks.UnLockMulti(keys)

	tem, ok := m.db.Get(srcKey)
	if !ok {
		return RESP.MakeIntData(0)
	}
	srcSet, ok := tem.(*Set)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	var desSet *Set
	var desExist bool
	tem, ok = m.db.Get(desKey)
	if !ok {
		desSet = NewSet()
		desExist = false
	} else {
		desSet, ok = tem.(*Set)
		if !ok {
			return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
		desExist = true
	}

	res := srcSet.Remove(val)
	if res == 0 {
		return RESP.MakeIntData(0)
	}
	desSet.Add(val)
	if !desExist {
		m.db.Set(desKey, desSet)
	}
	return RESP.MakeIntData(1)
}

func sMembersSet(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "smembers" {
		logger.Error("sMembersSet Function: cmdName is not smembers")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) != 2 {
		return RESP.MakeErrorData("wrong number of arguments for 'smembers' command")
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
	set, ok := tem.(*Set)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	members := set.Members()
	res := make([]RESP.RedisData, 0)
	for _, member := range members {
		res = append(res, RESP.MakeBulkData([]byte(member)))
	}
	return RESP.MakeArrayData(res)
}

func sIsMemberSet(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "sismember" {
		logger.Error("sIsMemberSet Function: cmdName is not sismember")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) != 3 {
		return RESP.MakeErrorData("wrong number of arguments for 'sismember' command")
	}

	key := string(cmd[1])
	val := string(cmd[2])
	if !m.CheckTTL(key) {
		return RESP.MakeIntData(0)
	}

	m.locks.RLock(key)
	defer m.locks.RUnLock(key)
	tem, ok := m.db.Get(key)
	if !ok {
		return RESP.MakeIntData(0)
	}
	set, ok := tem.(*Set)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	if set.Has(val) {
		return RESP.MakeIntData(1)
	}
	return RESP.MakeIntData(0)
}

func sInterStoreSet(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "sinterstore" {
		logger.Error("sInterStoreSet Function: cmdName is not sinterstore")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) < 3 {
		return RESP.MakeErrorData("wrong number of arguments for 'sinterstore' command")
	}
	// first check if the destination  is a set. if not, return error immediately.
	desKey := string(cmd[1])
	m.CheckTTL(desKey)
	tem, ok := m.db.Get(desKey)
	if ok {
		_, ok = tem.(*Set)
		if !ok {
			return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
	}
	// second, get the intersection set
	keys := make([]string, 0)
	for i := 2; i < len(cmd); i++ {
		key := string(cmd[i])
		if m.CheckTTL(key) {
			keys = append(keys, string(cmd[i]))
		}
	}
	if len(keys) == 0 {
		return RESP.MakeIntData(0)
	}

	m.locks.RLockMulti(keys)

	// 1. check if the keys are all set
	// 2. find the shortest set as primary set to decrease the time complexity
	sets := make([]*Set, 0)
	shortestSet := 0
	shortestLen := math.MaxInt
	for _, key := range keys {
		tem, ok := m.db.Get(key)
		if ok {
			set, ok := tem.(*Set)
			if !ok {
				m.locks.RUnLockMulti(keys)
				return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
			}
			sets = append(sets, set)
			if set.Len() < shortestLen {
				shortestLen = set.Len()
				shortestSet = len(sets) - 1
			}
		}
	}
	primSet := sets[shortestSet]
	sets = append(sets[:shortestSet], sets[shortestSet+1:]...)
	interSet := primSet.Intersect(sets...)
	m.locks.RUnLockMulti(keys)

	// third, store the intersection set to the destination key
	m.CheckTTL(desKey)
	m.locks.Lock(desKey)
	defer m.locks.UnLock(desKey)
	// have to check again, because the key may be set by other goroutine
	tem, ok = m.db.Get(desKey)
	if ok {
		_, ok = tem.(*Set)
		if !ok {
			return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
	}
	if interSet.Len() != 0 {
		m.db.Set(desKey, interSet)
	}
	return RESP.MakeIntData(int64(interSet.Len()))

}

func sInterSet(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "sinter" {
		logger.Error("sInterSet Function: cmdName is not sinter")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) < 2 {
		return RESP.MakeErrorData("wrong number of arguments for 'sinter' command")
	}

	keys := make([]string, 0, len(cmd)-1)
	for i := 1; i < len(cmd); i++ {
		keys = append(keys, string(cmd[i]))
	}

	for _, key := range keys {
		m.CheckTTL(key)
	}

	m.locks.RLockMulti(keys)
	defer m.locks.RUnLockMulti(keys)
	// 1. check if the keys are all set
	// 2. find the shortest set as primary set to decrease the time complexity
	sets := make([]*Set, 0)
	shortestSet := 0
	shortestLen := math.MaxInt
	for _, key := range keys {
		tem, ok := m.db.Get(key)
		if ok {
			set, ok := tem.(*Set)
			if !ok {
				return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
			}
			sets = append(sets, set)
			if set.Len() < shortestLen {
				shortestLen = set.Len()
				shortestSet = len(sets) - 1
			}
		}
	}
	primSet := sets[shortestSet]
	sets = append(sets[:shortestSet], sets[shortestSet+1:]...)
	interSet := primSet.Intersect(sets...)

	res := make([]RESP.RedisData, 0)
	for _, member := range interSet.Members() {
		res = append(res, RESP.MakeBulkData([]byte(member)))
	}

	return RESP.MakeArrayData(res)

}

func sDiffStoreSet(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "sdiffstore" {
		logger.Error("sDiffStoreSet Function: cmdName is not sdiffstore")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) < 3 {
		return RESP.MakeErrorData("wrong number of arguments for 'sdiffstore' command")
	}
	// first check if the destination  is a set. if not, return error immediately.
	desKey := string(cmd[1])
	m.CheckTTL(desKey)
	tem, ok := m.db.Get(desKey)
	if ok {
		_, ok = tem.(*Set)
		if !ok {
			return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
	}
	// second, get the difference set
	keys := make([]string, 0)
	for i := 2; i < len(cmd); i++ {
		key := string(cmd[i])
		if m.CheckTTL(key) {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return RESP.MakeIntData(0)
	}

	var diffRes *Set
	// Don't forget Unlock
	m.locks.RLockMulti(keys)
	tem, ok = m.db.Get(keys[0])
	if !ok {
		diffRes = NewSet() // if the first key is not exist, return empty set
	} else {
		primSet, ok := tem.(*Set)
		if !ok {
			m.locks.RUnLockMulti(keys)
			return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
		}

		setSlice := make([]*Set, 0)
		for i := 1; i < len(keys); i++ {
			tem, ok = m.db.Get(keys[i])
			if ok {
				set, ok := tem.(*Set)
				if !ok {
					m.locks.RUnLockMulti(keys)
					return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
				}
				setSlice = append(setSlice, set)
			}
		}
		diffRes = primSet.Difference(setSlice...)
	}
	m.locks.RUnLockMulti(keys)

	m.CheckTTL(desKey)
	m.locks.Lock(desKey)
	defer m.locks.UnLock(desKey)
	// have to check again, because the key may be set by other goroutine
	tem, ok = m.db.Get(desKey)
	if ok {
		_, ok = tem.(*Set)
		if !ok {
			return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
	}
	if diffRes.Len() != 0 {
		m.db.Set(desKey, diffRes)
	}
	return RESP.MakeIntData(int64(diffRes.Len()))

}

func sDiffSet(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "sdiff" {
		logger.Error("sDiffSet Function: cmdName is not sdiff")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) < 2 {
		return RESP.MakeErrorData("wrong number of arguments for 'sdiff' command")
	}

	keys := make([]string, 0, len(cmd)-1)
	for i := 1; i < len(cmd); i++ {
		keys = append(keys, string(cmd[i]))
	}

	for _, key := range keys {
		m.CheckTTL(key)
	}

	m.locks.RLockMulti(keys)
	defer m.locks.RUnLockMulti(keys)

	tem, ok := m.db.Get(keys[0])
	if !ok {
		return RESP.MakeEmptyArrayData()
	}
	primSet, ok := tem.(*Set)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	res := make([]RESP.RedisData, 0)
	// if cmd has no other keys, return the first key members
	if len(keys) == 1 {
		members := primSet.Members()
		for _, member := range members {
			res = append(res, RESP.MakeBulkData([]byte(member)))
		}
		return RESP.MakeArrayData(res)
	}
	setSlice := make([]*Set, 0)
	for i := 1; i < len(keys); i++ {
		tem, ok = m.db.Get(keys[i])
		if ok {
			set, ok := tem.(*Set)
			if !ok {
				return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
			}
			setSlice = append(setSlice, set)
		}
	}
	diffSet := primSet.Difference(setSlice...)
	for _, member := range diffSet.Members() {
		res = append(res, RESP.MakeBulkData([]byte(member)))
	}
	return RESP.MakeArrayData(res)
}

func sCardSet(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "scard" {
		logger.Error("sCardSet Function: cmdName is not scard")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) != 2 {
		return RESP.MakeErrorData("wrong number of arguments for 'scard' command")
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
	sets, ok := tem.(*Set)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	res := sets.Len()

	return RESP.MakeIntData(int64(res))

}

func sAddSet(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "sadd" {
		logger.Error("sAddSet Function: cmdName is not sadd")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) < 3 {
		return RESP.MakeErrorData("wrong number of arguments for 'sadd' command")
	}

	key := string(cmd[1])
	m.CheckTTL(key)
	m.locks.Lock(key)
	defer m.locks.UnLock(key)
	tem, ok := m.db.Get(key)
	if !ok {
		tem = NewSet()
		m.db.Set(key, tem)
	}
	sets, ok := tem.(*Set)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	res := 0
	for i := 2; i < len(cmd); i++ {
		res += sets.Add(string(cmd[i]))
	}

	return RESP.MakeIntData(int64(res))

}
