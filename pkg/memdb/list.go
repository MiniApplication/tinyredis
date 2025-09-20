package memdb

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/hsn0918/tinyredis/pkg/RESP"
	"github.com/hsn0918/tinyredis/pkg/logger"
)

func RegisterListCommands() {
	RegisterCommand("llen", lLenList)
	RegisterCommand("lindex", lIndexList)
	RegisterCommand("lpos", lPosList)
	RegisterCommand("lpop", lPopList)
	RegisterCommand("rpop", rPopList)
	RegisterCommand("lpush", lPushList)
	RegisterCommand("lpushx", lPushXList)
	RegisterCommand("rpush", rPushList)
	RegisterCommand("rpushx", rPushXList)
	RegisterCommand("lset", lSetList)
	RegisterCommand("lrem", lRemList)
	RegisterCommand("ltrim", lTrimList)
	RegisterCommand("lrange", lRangeList)
	RegisterCommand("lmove", lMoveList)
	//RegisterCommand("blpop", blPopList)
	//RegisterCommand("brpop", brPopList)
}

func lMoveList(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "lmove" {
		logger.Error("lMoveList Function : cmdName is not lmove")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) != 5 {
		return RESP.MakeErrorData("wrong number of arguments for 'lmove' command")
	}

	src := string(cmd[1])
	des := string(cmd[2])
	srcDrc := strings.ToLower(string(cmd[3]))
	desDrc := strings.ToLower(string(cmd[4]))
	if (srcDrc != "left" && srcDrc != "right") || (desDrc != "left" && desDrc != "right") {
		return RESP.MakeErrorData("options must be left or right")
	}
	if !m.CheckTTL(src) {
		return RESP.MakeNullBulkData()
	}

	m.CheckTTL(des)

	keys := []string{src, des}

	m.locks.LockMulti(keys)
	defer m.locks.UnLockMulti(keys)

	srcTem, ok := m.db.Get(src)
	if !ok {
		return RESP.MakeNullBulkData()
	}
	srcList, ok := srcTem.(*List)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	defer func() {
		if srcList.Len() == 0 {
			m.db.Delete(src)
			m.DelTTL(src)
		}
	}()
	if srcList.Len() == 0 {
		return RESP.MakeNullBulkData()
	}

	desTem, ok := m.db.Get(des)
	if !ok {
		desTem = NewList()
		m.db.Set(des, desTem)
	}
	desList, ok := desTem.(*List)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	// pop from src
	var popElem *ListNode
	if srcDrc == "left" {
		popElem = srcList.LPop()
	} else {
		popElem = srcList.RPop()
	}
	//    insert to des
	if desDrc == "left" {
		desList.LPush(popElem.Val)
	} else {
		desList.RPush(popElem.Val)
	}
	return RESP.MakeBulkData(popElem.Val)

}
func lRangeList(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "lrange" {
		logger.Error("lRangeList Function : cmdName is not lrange")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) != 4 {
		return RESP.MakeErrorData("wrong number of arguments for 'lrange' command")
	}

	start, err1 := strconv.Atoi(string(cmd[2]))
	end, err2 := strconv.Atoi(string(cmd[3]))
	if err1 != nil || err2 != nil {
		return RESP.MakeErrorData("index must be an integer")
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
	list, ok := tem.(*List)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	temRes := list.Range(start, end)
	if temRes == nil {
		return RESP.MakeEmptyArrayData()
	}
	res := make([]RESP.RedisData, len(temRes))
	for i := 0; i < len(temRes); i++ {
		res[i] = RESP.MakeBulkData(temRes[i])
	}
	return RESP.MakeArrayData(res)
}

func lTrimList(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "ltrim" {
		logger.Error("lTrimList Function : cmdName is not ltrim")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) != 4 {
		return RESP.MakeErrorData("wrong number of arguments for 'ltrim' command")
	}
	start, err1 := strconv.Atoi(string(cmd[2]))
	end, err2 := strconv.Atoi(string(cmd[3]))
	if err1 != nil || err2 != nil {
		return RESP.MakeErrorData("start and end must be an integer")
	}
	key := string(cmd[1])
	if !m.CheckTTL(key) {
		return RESP.MakeStringData("OK")
	}

	m.locks.Lock(key)
	defer m.locks.UnLock(key)
	tem, ok := m.db.Get(key)
	if !ok {
		return RESP.MakeStringData("OK")
	}
	list, ok := tem.(*List)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	defer func() {
		if list.Len() == 0 {
			m.db.Delete(key)
			m.DelTTL(key)
		}
	}()

	list.Trim(start, end)
	return RESP.MakeStringData("OK")
}

func lRemList(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "lrem" {
		logger.Error("lRemList Function : cmdName is not lrem")
		return RESP.MakeErrorData("Server error")
	}

	if len(cmd) != 4 {
		return RESP.MakeErrorData("wrong number of arguments for 'lrem' command")
	}

	count, err := strconv.Atoi(string(cmd[2]))
	if err != nil {
		return RESP.MakeErrorData("count must be an integer")
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

	list, ok := tem.(*List)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	defer func() {
		if list.Len() == 0 {
			m.db.Delete(key)
			m.DelTTL(key)
		}
	}()
	res := list.RemoveElement(cmd[3], count)

	return RESP.MakeIntData(int64(res))

}

func lSetList(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "lset" {
		logger.Error("lSetList Function : cmdName is not lset")
		return RESP.MakeErrorData("server error")
	}
	if len(cmd) != 4 {
		return RESP.MakeErrorData("wrong number of arguments for 'lset' command")
	}

	index, err := strconv.Atoi(string(cmd[2]))
	if err != nil {
		return RESP.MakeErrorData("index must be an integer")
	}
	key := string(cmd[1])

	if !m.CheckTTL(key) {
		return RESP.MakeErrorData("key not exist")
	}

	m.locks.Lock(key)
	defer m.locks.UnLock(key)

	tem, ok := m.db.Get(key)
	if !ok {
		return RESP.MakeErrorData("key not exist")
	}

	list, ok := tem.(*List)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	success := list.Set(index, cmd[3])
	if !success {
		return RESP.MakeErrorData("index out of range")
	}
	return RESP.MakeStringData("OK")

}

func rPushXList(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "rpushx" {
		logger.Error("rPushXList Function : cmdName is not rpushx")
		return RESP.MakeErrorData("server error")
	}
	if len(cmd) < 3 {
		return RESP.MakeErrorData("wrong number of arguments for 'rpushX' command")
	}
	key := string(cmd[1])
	m.CheckTTL(key)

	m.locks.Lock(key)
	defer m.locks.UnLock(key)

	var list *List
	tem, ok := m.db.Get(key)
	if !ok {
		return RESP.MakeIntData(0)
	} else {
		list, ok = tem.(*List)
		if !ok {
			return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
	}
	for i := 2; i < len(cmd); i++ {
		list.RPush(cmd[i])
	}
	return RESP.MakeIntData(int64(list.Len()))
}

func rPushList(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "rpush" {
		logger.Error("rPushList Function : cmdName is not rpush")
		return RESP.MakeErrorData("Server error")

	}
	if len(cmd) < 3 {
		return RESP.MakeErrorData("wrong number of arguments for 'rpush' command")
	}

	key := string(cmd[1])
	m.CheckTTL(key)

	m.locks.Lock(key)
	defer m.locks.UnLock(key)

	var list *List
	tem, ok := m.db.Get(key)
	if !ok {
		list = NewList()
		m.db.Set(key, list)
	} else {
		list, ok = tem.(*List)
		if !ok {
			return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
	}
	for i := 2; i < len(cmd); i++ {
		list.RPush(cmd[i])
	}
	return RESP.MakeIntData(int64(list.Len()))
}

func lPushXList(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "lpushx" {
		logger.Error("lPushXList Function : cmdName is not lpushx")
		return RESP.MakeErrorData("Server Error")
	}
	if len(cmd) < 3 {
		return RESP.MakeErrorData("wrong number of arguments for 'lpushx' command")
	}
	key := string(cmd[1])
	m.CheckTTL(key)

	m.locks.Lock(key)
	defer m.locks.UnLock(key)

	var list *List
	tem, ok := m.db.Get(key)
	if !ok {
		return RESP.MakeIntData(0)
	} else {
		list, ok = tem.(*List)
		if !ok {
			return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
	}
	for i := 2; i < len(cmd); i++ {
		list.LPush(cmd[i])
	}
	return RESP.MakeIntData(int64(list.Len()))
}
func lPushList(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "lpush" {
		logger.Error("lPushList Function : cmdName is not lpush")
		return RESP.MakeErrorData("Server Error")
	}
	if len(cmd) < 3 {
		return RESP.MakeErrorData("wrong number of arguments for 'lpush' command")
	}
	key := string(cmd[1])
	m.CheckTTL(key)

	m.locks.Lock(key)
	defer m.locks.UnLock(key)

	var list *List
	tem, ok := m.db.Get(key)
	if !ok {
		list = NewList()
		m.db.Set(key, list)
	} else {
		list, ok = tem.(*List)
		if !ok {
			return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
	}
	for i := 2; i < len(cmd); i++ {
		list.LPush(cmd[i])
	}
	return RESP.MakeIntData(int64(list.Len()))
}
func rPopList(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "rpop" {
		logger.Error("rPopList: command is not rpop")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) != 2 && len(cmd) != 3 {
		return RESP.MakeErrorData("wrong number of arguments for 'rpop' command")
	}
	var cnt int
	var err error
	if len(cmd) == 3 {
		cnt, err = strconv.Atoi(string(cmd[2]))
		if err != nil || cnt <= 0 {
			return RESP.MakeErrorData("count value must be a positive integer")
		}
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
	list, ok := tem.(*List)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	defer func() {
		if list.Len() == 0 {
			m.db.Delete(key)
			m.DelTTL(key)
		}
	}()
	// if cnt is not set, return last element
	if cnt == 0 {
		e := list.RPop()
		if e == nil {
			return RESP.MakeNullBulkData()
		}
		return RESP.MakeBulkData(e.Val)
	}

	// return cnt number elements as array
	res := make([]RESP.RedisData, 0)
	for i := 0; i < cnt; i++ {
		e := list.RPop()
		if e == nil {
			break
		}
		res = append(res, RESP.MakeBulkData(e.Val))
	}
	return RESP.MakeArrayData(res)
}

func lPopList(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "lpop" {
		logger.Error("lPopList: command is not lpop")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) != 2 && len(cmd) != 3 {
		return RESP.MakeErrorData("wrong number of arguments for 'lpop' command")
	}
	var cnt int
	var err error
	if len(cmd) == 3 {
		cnt, err = strconv.Atoi(string(cmd[2]))
		if err != nil || cnt <= 0 {
			return RESP.MakeErrorData("count value must be a positive integer")
		}
	}
	key := string(cmd[1])
	if !m.CheckTTL(key) {
		return RESP.MakeBulkData(nil)
	}

	m.locks.Lock(key)
	defer m.locks.UnLock(key)
	tem, ok := m.db.Get(key)
	if !ok {
		return RESP.MakeBulkData(nil)
	}
	list, ok := tem.(*List)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	// remove the key when list is empty
	defer func() {
		if list.Len() == 0 {
			m.db.Delete(key)
			m.DelTTL(key)
		}
	}()
	// if cnt is not set, return first element
	if cnt == 0 {
		e := list.LPop()
		if e == nil {
			return RESP.MakeBulkData(nil)
		}
		return RESP.MakeBulkData(e.Val)
	}
	// return cnt number elements as array
	res := make([]RESP.RedisData, 0)
	for i := 0; i < cnt; i++ {
		e := list.LPop()
		if e == nil {
			break
		}
		res = append(res, RESP.MakeBulkData(e.Val))
	}
	return RESP.MakeArrayData(res)

}

func lPosList(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "lpos" {
		logger.Error("lPosList Function: cmdName is not lpos")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) < 3 || len(cmd)&1 != 1 {
		return RESP.MakeErrorData("wrong number of arguments for 'lpos' command")
	}
	var rank, count, maxLen, reverse bool
	var rankVal, countVal, maxLenVal int
	var key string
	var elem []byte
	var err error

	var pos int
	key = string(cmd[1])
	elem = cmd[2]
	// handle params
	for i := 3; i < len(cmd); i += 2 {
		switch strings.ToLower(string(cmd[i])) {
		case "rank":
			rank = true
			rankVal, err = strconv.Atoi(string(cmd[i+1]))
			if err != nil || rankVal == 0 {
				return RESP.MakeErrorData("rank value should 1,2,3... or -1,-2,-3...")
			}
		case "count":
			count = true
			countVal, err = strconv.Atoi(string(cmd[i+1]))
			if err != nil || countVal < 0 {
				return RESP.MakeErrorData("count value is not an positive integer")
			}
		case "maxlen":
			maxLen = true
			maxLenVal, err = strconv.Atoi(string(cmd[i+1]))
			if err != nil || maxLenVal < 0 {
				return RESP.MakeErrorData("maxlen value is not an positive integer")
			}
		default:
			return RESP.MakeErrorData(fmt.Sprintf("unsupported option %s", string(cmd[i])))
		}
	}
	if !m.CheckTTL(key) {
		return RESP.MakeNullBulkData()
	}

	m.locks.RLock(key)
	defer m.locks.RUnLock(key)

	tem, ok := m.db.Get(key)
	if !ok {
		return RESP.MakeNullBulkData()
	}

	list, ok := tem.(*List)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	if list.Len() == 0 {
		return RESP.MakeNullBulkData()
	}
	if count && countVal == 0 {
		countVal = list.Len()
	}

	if maxLen && maxLenVal == 0 {
		maxLenVal = list.Len()
	}

	// normally pos without options
	if !rank && !count && !maxLen {
		pos := list.Pos(elem)
		if pos == -1 {
			return RESP.MakeNullBulkData()
		} else {
			return RESP.MakeIntData(int64(pos))
		}
	}
	// handle options
	var now *ListNode
	if rank {
		if rankVal > 0 {
			pos = -1
			for now = list.Head.Next; now != list.Tail; now = now.Next {
				pos++
				if bytes.Equal(now.Val, elem) {
					rankVal--
				}
				if maxLen {
					maxLenVal--
					if maxLenVal == 0 {
						break
					}
				}
				if rankVal == 0 {
					break
				}
			}
		} else {
			reverse = true
			pos = list.Len()
			for now = list.Tail.Prev; now != list.Head; now = now.Prev {
				pos--
				if bytes.Equal(now.Val, elem) {
					rankVal++
				}
				if maxLen {
					maxLenVal--
					if maxLenVal == 0 {
						break
					}
				}
				if rankVal == 0 {
					break
				}
			}
		}
	} else {
		now = list.Head.Next
		pos = 0
		if maxLen {
			maxLenVal--
		}
	}

	// when rank is out of range, return nil
	if (rank && rankVal != 0) || now == list.Tail || now == list.Head {
		return RESP.MakeNullBulkData()
	}

	res := make([]RESP.RedisData, 0)
	if !count {
		// if count is not set, return first find pos inside maxLen range
		for ; now != list.Tail; now = now.Next {
			if bytes.Equal(now.Val, elem) {
				return RESP.MakeIntData(int64(pos))
			}
			pos++
			if maxLen {
				if maxLenVal <= 0 {
					break
				}
				maxLenVal--
			}
		}
		return RESP.MakeNullBulkData()
	} else {
		if !reverse {
			for ; now != list.Tail && countVal != 0; now = now.Next {
				if bytes.Equal(now.Val, elem) {
					res = append(res, RESP.MakeIntData(int64(pos)))
					countVal--
				}
				pos++
				if maxLen {
					if maxLenVal <= 0 {
						break
					}
					maxLenVal--
				}
			}
		} else {
			for ; now != list.Head && countVal != 0; now = now.Prev {
				if bytes.Equal(now.Val, elem) {
					res = append(res, RESP.MakeIntData(int64(pos)))
					countVal--
				}
				pos--
				if maxLen {
					if maxLenVal <= 0 {
						break
					}
					maxLenVal--
				}
			}
		}
	}
	if len(res) == 0 {
		return RESP.MakeNullBulkData()
	}
	return RESP.MakeArrayData(res)

}

func lIndexList(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "lindex" {
		logger.Error("lIndexList Function: cmdName is not lindex")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) != 3 {
		return RESP.MakeErrorData("wrong number of arguments for 'lindex' command")
	}
	key := string(cmd[1])
	index, err := strconv.Atoi(string(cmd[2]))
	if err != nil {
		return RESP.MakeErrorData("index is not an integer")
	}
	if !m.CheckTTL(key) {
		return RESP.MakeBulkData(nil)
	}

	m.locks.RLock(key)
	defer m.locks.RUnLock(key)
	v, ok := m.db.Get(key)
	if !ok {
		return RESP.MakeBulkData(nil)
	}
	typeV, ok := v.(*List)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	resNode := typeV.Index(index)
	if resNode == nil {
		return RESP.MakeNullBulkData()
	}
	return RESP.MakeBulkData(resNode.Val)
}

func lLenList(m *MemDb, cmd [][]byte) RESP.RedisData {
	if strings.ToLower(string(cmd[0])) != "llen" {
		logger.Error("lLenList Function: cmdName is not llen")
		return RESP.MakeErrorData("Server error")
	}
	if len(cmd) != 2 {
		return RESP.MakeErrorData("wrong number of arguments for 'llen' command")
	}
	key := string(cmd[1])
	if !m.CheckTTL(key) {
		return RESP.MakeIntData(0)
	}
	m.locks.RLock(key)
	defer m.locks.RUnLock(key)
	val, ok := m.db.Get(key)
	if !ok {
		return RESP.MakeIntData(0)
	}
	listVal, ok := val.(*List)
	if !ok {
		return RESP.MakeErrorData("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	res := listVal.Len()
	return RESP.MakeIntData(int64(res))

}

// TODO: blpop from list
//func blPopList(m *MemDb, cmd [][]byte) resp.RedisData {
//	return nil
//}

// TODO: brpop from list
//func brPopList(m *MemDb, cmd [][]byte) resp.RedisData {
//    return nil
//}
