package memdb

import (
	"container/heap"
	"strings"
	"sync"
	"time"

	"github.com/hsn/tiny-redis/pkg/RESP"
	"github.com/hsn/tiny-redis/pkg/config"
	"github.com/hsn/tiny-redis/pkg/logger"
)

// TTLItem represents a key with its expiration time
type TTLItem struct {
	key      string
	expireAt int64
	index    int
}

// TTLHeap 是一个最小堆，用于高效管理过期键
type TTLHeap []*TTLItem

func (h TTLHeap) Len() int           { return len(h) }
func (h TTLHeap) Less(i, j int) bool { return h[i].expireAt < h[j].expireAt }
func (h TTLHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *TTLHeap) Push(x interface{}) {
	n := len(*h)
	item := x.(*TTLItem)
	item.index = n
	*h = append(*h, item)
}

func (h *TTLHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*h = old[0 : n-1]
	return item
}

// TTLManager 管理键的过期时间
type TTLManager struct {
	heap     TTLHeap
	keyMap   map[string]*TTLItem
	lock     sync.RWMutex
	stopChan chan struct{}
	db       *ConcurrentMap
}

func NewTTLManager(db *ConcurrentMap) *TTLManager {
	tm := &TTLManager{
		heap:     make(TTLHeap, 0),
		keyMap:   make(map[string]*TTLItem),
		stopChan: make(chan struct{}),
		db:       db,
	}
	heap.Init(&tm.heap)
	go tm.cleanupLoop()
	return tm
}

func (tm *TTLManager) cleanupLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tm.cleanup()
		case <-tm.stopChan:
			return
		}
	}
}

func (tm *TTLManager) cleanup() {
	now := time.Now().Unix()
	tm.lock.Lock()
	defer tm.lock.Unlock()

	for tm.heap.Len() > 0 {
		item := tm.heap[0]
		if item.expireAt > now {
			break
		}
		heap.Pop(&tm.heap)
		delete(tm.keyMap, item.key)
		tm.db.Delete(item.key)
	}
}

func (tm *TTLManager) SetTTL(key string, expireAt int64) {
	tm.lock.Lock()
	defer tm.lock.Unlock()

	if item, exists := tm.keyMap[key]; exists {
		item.expireAt = expireAt
		heap.Fix(&tm.heap, item.index)
	} else {
		item := &TTLItem{
			key:      key,
			expireAt: expireAt,
		}
		heap.Push(&tm.heap, item)
		tm.keyMap[key] = item
	}
}

func (tm *TTLManager) CheckTTL(key string) bool {
	tm.lock.RLock()
	item, exists := tm.keyMap[key]
	tm.lock.RUnlock()

	if !exists {
		return true
	}

	now := time.Now().Unix()
	if item.expireAt > now {
		return true
	}

	tm.lock.Lock()
	defer tm.lock.Unlock()

	// 双重检查，避免并发问题
	if item, exists = tm.keyMap[key]; exists && item.expireAt <= now {
		heap.Remove(&tm.heap, item.index)
		delete(tm.keyMap, key)
		return false
	}
	return true
}

func (tm *TTLManager) RemoveTTL(key string) {
	tm.lock.Lock()
	defer tm.lock.Unlock()

	if item, exists := tm.keyMap[key]; exists {
		heap.Remove(&tm.heap, item.index)
		delete(tm.keyMap, key)
	}
}

// MemDb is the memory cache database
// All key:value pairs are stored in db
// All ttl keys are stored in ttlKeys
// locks is used to lock a key for db to ensure some atomic operations
type MemDb struct {
	db    *ConcurrentMap
	ttl   *TTLManager
	locks *Locks
}

func NewMemDb() *MemDb {
	db := NewConcurrentMap(config.Configures.ShardNum)
	return &MemDb{
		db:    db,
		ttl:   NewTTLManager(db),
		locks: NewLocks(config.Configures.ShardNum * 2),
	}
}

func (m *MemDb) ExecCommand(cmd [][]byte) RESP.RedisData {
	if len(cmd) == 0 {
		return nil
	}
	var res RESP.RedisData
	cmdName := strings.ToLower(string(cmd[0]))
	command, ok := CmdTable[cmdName]
	if !ok {
		res = RESP.MakeErrorData("error: unsupported command")
	} else {
		execFunc := command.executor
		res = execFunc(m, cmd)
	}
	return res
}

// CheckTTL checks ttl keys and delete expired keys
// return false if key is expired,else true
// Attention: Don't lock this function because it has called locks.Lock(key) for atomic deleting expired key.
// Otherwise, it will cause a deadlock.
func (m *MemDb) CheckTTL(key string) bool {
	if !m.ttl.CheckTTL(key) {
		m.locks.Lock(key)
		defer m.locks.UnLock(key)
		m.db.Delete(key)
		return false
	}
	return true
}

// SetTTL sets ttl for keys
// return bool to check if ttl set success
// return int to check if the key is a new ttl key
func (m *MemDb) SetTTL(key string, expireAt int64) int {
	if _, ok := m.db.Get(key); !ok {
		logger.Debug("SetTTL: key not exist")
		return 0
	}
	m.ttl.SetTTL(key, expireAt)
	return 1
}

func (m *MemDb) DelTTL(key string) int {
	m.ttl.RemoveTTL(key)
	return 1
}

// Get 获取键的值
func (m *MemDb) Get(key string) (interface{}, bool) {
	return m.db.Get(key)
}

// Set 设置键值对
func (m *MemDb) Set(key string, value interface{}) {
	m.db.Set(key, value)
}

// Keys 获取所有键
func (m *MemDb) Keys() []string {
	return m.db.Keys()
}

// Size 获取键值对数量
func (m *MemDb) Size() int {
	return m.db.Len()
}

// GetTTL 获取键的 TTL（秒）
func (m *MemDb) GetTTL(key string) int64 {
	m.ttl.lock.RLock()
	item, exists := m.ttl.keyMap[key]
	m.ttl.lock.RUnlock()

	if !exists {
		return -1
	}

	ttl := item.expireAt - time.Now().Unix()
	if ttl <= 0 {
		return -1
	}
	return ttl
}
