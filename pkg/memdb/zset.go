package memdb

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/hsn0918/tinyredis/pkg/RESP"
	"github.com/hsn0918/tinyredis/pkg/logger"
)

func RegisterZSetCommands() {
	RegisterCommand("zadd", zAddZset)
	RegisterCommand("zcard", zCardZset)
	RegisterCommand("zcount", zCountZset)
	RegisterCommand("zrange", zRangeZset)
	RegisterCommand("zrevrange", zRevRangeZset)
	RegisterCommand("zrangebyscore", zRangeByScoreZset)
	RegisterCommand("zrank", zRankZset)
	RegisterCommand("zrevrank", zRevRankZset)
	RegisterCommand("zrem", zRemZset)
	RegisterCommand("zremrangebyrank", zRemRangeByRankZset)
	RegisterCommand("zremrangebyscore", zRemRangeByScoreZset)
	RegisterCommand("zscore", zScoreZset)
	RegisterCommand("zincrby", zIncrByZset)
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
		if zset.Remove(member) {
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
	if math.IsNaN(score) {
		return RESP.MakeNullBulkData()
	}
	return RESP.MakeBulkData([]byte(fmt.Sprintf("%v", score)))
}

// zRevRangeZset 实现 ZREVRANGE 命令
func zRevRangeZset(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "zrevrange" || len(cmd) < 4 {
		return RESP.MakeErrorData("ERR wrong number of arguments for 'zrevrange' command")
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

	members, scores := zset.RevRange(start, stop)
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

// zRangeByScoreZset 实现 ZRANGEBYSCORE 命令
func zRangeByScoreZset(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "zrangebyscore" || len(cmd) < 4 {
		return RESP.MakeErrorData("ERR wrong number of arguments for 'zrangebyscore' command")
	}

	key := string(cmd[1])
	if !m.CheckTTL(key) {
		return RESP.MakeArrayData(nil)
	}

	// 解析min
	minStr := string(cmd[2])
	var min float64
	if minStr == "-inf" {
		min = math.Inf(-1)
	} else {
		var err error
		min, err = strconv.ParseFloat(minStr, 64)
		if err != nil {
			return RESP.MakeErrorData("ERR min value is not a valid float")
		}
	}

	// 解析max
	maxStr := string(cmd[3])
	var max float64
	if maxStr == "+inf" || maxStr == "inf" {
		max = math.Inf(1)
	} else {
		var err error
		max, err = strconv.ParseFloat(maxStr, 64)
		if err != nil {
			return RESP.MakeErrorData("ERR max value is not a valid float")
		}
	}

	// 解析可选参数
	withScores := false
	offset := 0
	count := -1

	for i := 4; i < len(cmd); i++ {
		arg := strings.ToLower(string(cmd[i]))
		if arg == "withscores" {
			withScores = true
		} else if arg == "limit" && i+2 < len(cmd) {
			var err error
			offset, err = strconv.Atoi(string(cmd[i+1]))
			if err != nil {
				return RESP.MakeErrorData("ERR offset value is not an integer")
			}
			count, err = strconv.Atoi(string(cmd[i+2]))
			if err != nil {
				return RESP.MakeErrorData("ERR count value is not an integer")
			}
			i += 2
		}
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

	members, scores := zset.RangeByScore(min, max, offset, count)
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

// zRevRankZset 实现 ZREVRANK 命令
func zRevRankZset(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "zrevrank" || len(cmd) != 3 {
		return RESP.MakeErrorData("ERR wrong number of arguments for 'zrevrank' command")
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

	rank := zset.RevRank(member)
	if rank == -1 {
		return RESP.MakeNullBulkData()
	}
	return RESP.MakeIntData(int64(rank))
}

// zIncrByZset 实现 ZINCRBY 命令
func zIncrByZset(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "zincrby" || len(cmd) != 4 {
		return RESP.MakeErrorData("ERR wrong number of arguments for 'zincrby' command")
	}

	key := string(cmd[1])
	incr, err := strconv.ParseFloat(string(cmd[2]), 64)
	if err != nil {
		return RESP.MakeErrorData("ERR increment value is not a valid float")
	}
	member := string(cmd[3])

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

	newScore := zset.IncrBy(member, incr)
	return RESP.MakeBulkData([]byte(fmt.Sprintf("%v", newScore)))
}

// zRemRangeByRankZset 实现 ZREMRANGEBYRANK 命令
func zRemRangeByRankZset(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "zremrangebyrank" || len(cmd) != 4 {
		return RESP.MakeErrorData("ERR wrong number of arguments for 'zremrangebyrank' command")
	}

	key := string(cmd[1])
	if !m.CheckTTL(key) {
		return RESP.MakeIntData(0)
	}

	start, err := strconv.Atoi(string(cmd[2]))
	if err != nil {
		return RESP.MakeErrorData("ERR start value is not an integer")
	}

	stop, err := strconv.Atoi(string(cmd[3]))
	if err != nil {
		return RESP.MakeErrorData("ERR stop value is not an integer")
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

	removed := zset.RemRangeByRank(start, stop)
	return RESP.MakeIntData(int64(removed))
}

// zRemRangeByScoreZset 实现 ZREMRANGEBYSCORE 命令
func zRemRangeByScoreZset(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "zremrangebyscore" || len(cmd) != 4 {
		return RESP.MakeErrorData("ERR wrong number of arguments for 'zremrangebyscore' command")
	}

	key := string(cmd[1])
	if !m.CheckTTL(key) {
		return RESP.MakeIntData(0)
	}

	// 解析min
	minStr := string(cmd[2])
	var min float64
	if minStr == "-inf" {
		min = math.Inf(-1)
	} else {
		var err error
		min, err = strconv.ParseFloat(minStr, 64)
		if err != nil {
			return RESP.MakeErrorData("ERR min value is not a valid float")
		}
	}

	// 解析max
	maxStr := string(cmd[3])
	var max float64
	if maxStr == "+inf" || maxStr == "inf" {
		max = math.Inf(1)
	} else {
		var err error
		max, err = strconv.ParseFloat(maxStr, 64)
		if err != nil {
			return RESP.MakeErrorData("ERR max value is not a valid float")
		}
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

	removed := zset.RemRangeByScore(min, max)
	return RESP.MakeIntData(int64(removed))
}
