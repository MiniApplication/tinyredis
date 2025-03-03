package memdb

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hsn/tiny-redis/pkg/RESP"
	"github.com/hsn/tiny-redis/pkg/logger"
)

func RegisterZSetCommands() {
	RegisterCommand("zadd", zAddZset)
	RegisterCommand("zcard", zCardZset)
	RegisterCommand("zcount", zCountZset)
	RegisterCommand("zrange", zRangeZset)
	RegisterCommand("zrank", zRankZset)
	RegisterCommand("zrem", zRemZset)
	RegisterCommand("zscore", zScoreZset)
}

func zAddZset(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "zadd" {
		logger.Error("zAddZset Function: cmdName is not zadd")
		return nil
	}
	if len(cmd) < 3 || len(cmd)%2 != 0 {
		return RESP.MakeErrorData("wrong number of arguments for 'zadd' command")
	}
	key := string(cmd[1])
	m.CheckTTL(key)
	m.locks.Lock(key)
	defer m.locks.UnLock(key)
	temp, ok := m.db.Get(key)
	if !ok {
		temp = NewZSet()
		m.db.Set(key, temp)
	}
	zset, ok := temp.(*ZSet)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	res := 0
	for i := 2; i < len(cmd); i += 2 {
		member := string(cmd[i+1])
		score, err := strconv.ParseFloat(string(cmd[i]), 64)
		if err != nil {
			return RESP.MakeErrorData("ERR value is not a valid float")
		}
		if zset.Add(member, score) {
			res++
		}
	}
	return RESP.MakeIntData(int64(res))
}

func zCardZset(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "zcard" || len(cmd) != 2 {
		return RESP.MakeErrorData("ERR wrong number of arguments for 'zcard' command")
	}

	key := string(cmd[1])
	if !m.CheckTTL(key) {
		return RESP.MakeIntData(0)
	}

	m.locks.RLock(key)
	defer m.locks.RUnLock(key)

	temp, ok := m.db.Get(key)
	if !ok {
		return RESP.MakeIntData(0)
	}

	zset, ok := temp.(*ZSet)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	return RESP.MakeIntData(int64(zset.Len()))
}

func zCountZset(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "zcount" || len(cmd) != 4 {
		return RESP.MakeErrorData("ERR wrong number of arguments for 'zcount' command")
	}

	key := string(cmd[1])
	if !m.CheckTTL(key) {
		return RESP.MakeIntData(0)
	}

	min, err := strconv.ParseFloat(string(cmd[2]), 64)
	if err != nil {
		return RESP.MakeErrorData("ERR min value is not a valid float")
	}

	max, err := strconv.ParseFloat(string(cmd[3]), 64)
	if err != nil {
		return RESP.MakeErrorData("ERR max value is not a valid float")
	}

	m.locks.RLock(key)
	defer m.locks.RUnLock(key)

	temp, ok := m.db.Get(key)
	if !ok {
		return RESP.MakeIntData(0)
	}

	zset, ok := temp.(*ZSet)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	count := zset.Count(min, max)
	return RESP.MakeIntData(int64(count))
}

func zRangeZset(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "zrange" || len(cmd) < 4 {
		return RESP.MakeErrorData("ERR wrong number of arguments for 'zrange' command")
	}

	key := string(cmd[1])
	if !m.CheckTTL(key) {
		return RESP.MakeArrayData(nil)
	}

	start, err := strconv.Atoi(string(cmd[2]))
	if err != nil {
		return RESP.MakeErrorData("ERR start value is not an integer")
	}

	stop, err := strconv.Atoi(string(cmd[3]))
	if err != nil {
		return RESP.MakeErrorData("ERR stop value is not an integer")
	}

	withScores := false
	if len(cmd) == 5 && strings.ToLower(string(cmd[4])) == "withscores" {
		withScores = true
	}

	m.locks.RLock(key)
	defer m.locks.RUnLock(key)

	temp, ok := m.db.Get(key)
	if !ok {
		return RESP.MakeArrayData(nil)
	}

	zset, ok := temp.(*ZSet)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	members, scores := zset.Range(start, stop)
	if !withScores {
		result := make([]RESP.RedisData, len(members))
		for i, member := range members {
			result[i] = RESP.MakeBulkData([]byte(member))
		}
		return RESP.MakeArrayData(result)
	}

	result := make([]RESP.RedisData, len(members)*2)
	for i := 0; i < len(members); i++ {
		result[i*2] = RESP.MakeBulkData([]byte(members[i]))
		result[i*2+1] = RESP.MakeBulkData([]byte(fmt.Sprintf("%v", scores[i])))
	}
	return RESP.MakeArrayData(result)
}

func zRankZset(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "zrank" || len(cmd) != 3 {
		return RESP.MakeErrorData("ERR wrong number of arguments for 'zrank' command")
	}

	key := string(cmd[1])
	if !m.CheckTTL(key) {
		return RESP.MakeNullBulkData()
	}

	member := string(cmd[2])

	m.locks.RLock(key)
	defer m.locks.RUnLock(key)

	temp, ok := m.db.Get(key)
	if !ok {
		return RESP.MakeNullBulkData()
	}

	zset, ok := temp.(*ZSet)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	rank := zset.Rank(member)
	if rank == -1 {
		return RESP.MakeNullBulkData()
	}
	return RESP.MakeIntData(int64(rank))
}

func zRemZset(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "zrem" || len(cmd) < 3 {
		return RESP.MakeErrorData("ERR wrong number of arguments for 'zrem' command")
	}

	key := string(cmd[1])
	if !m.CheckTTL(key) {
		return RESP.MakeIntData(0)
	}

	m.locks.Lock(key)
	defer m.locks.UnLock(key)

	temp, ok := m.db.Get(key)
	if !ok {
		return RESP.MakeIntData(0)
	}

	zset, ok := temp.(*ZSet)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	removed := 0
	for i := 2; i < len(cmd); i++ {
		member := string(cmd[i])
		score := zset.Get(member)
		if score != -1 && zset.Remove(member, score) {
			removed++
		}
	}

	return RESP.MakeIntData(int64(removed))
}

func zScoreZset(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "zscore" || len(cmd) != 3 {
		return RESP.MakeErrorData("ERR wrong number of arguments for 'zscore' command")
	}

	key := string(cmd[1])
	if !m.CheckTTL(key) {
		return RESP.MakeNullBulkData()
	}

	member := string(cmd[2])

	m.locks.RLock(key)
	defer m.locks.RUnLock(key)

	temp, ok := m.db.Get(key)
	if !ok {
		return RESP.MakeNullBulkData()
	}

	zset, ok := temp.(*ZSet)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	score := zset.Get(member)
	if score == -1 {
		return RESP.MakeNullBulkData()
	}
	return RESP.MakeBulkData([]byte(fmt.Sprintf("%v", score)))
}
