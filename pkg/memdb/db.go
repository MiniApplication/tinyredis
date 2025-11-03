package memdb

import (
	"container/heap"
	"math/bits"
	"strings"
	"sync"
	"time"

	"github.com/hsn0918/tinyredis/pkg/RESP"
	"github.com/hsn0918/tinyredis/pkg/config"
	"github.com/hsn0918/tinyredis/pkg/logger"
	"github.com/hsn0918/tinyredis/pkg/util"
)

// TTLItem represents a key with its expiration time
type TTLItem struct {
	key      string
	expireAt int64
	index    int
}

var ttlItemPool = sync.Pool{
	New: func() any { return &TTLItem{} },
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
	shards    []*ttlShard
	shardMask int
	stopChan  chan struct{}
	db        *ConcurrentMap
}

type ttlShard struct {
	lock sync.Mutex
	heap TTLHeap
	keys map[string]*TTLItem
}

func NewTTLManager(db *ConcurrentMap) *TTLManager {
	shardCount := ttlShardCount(config.Configures.ShardNum)
	shards := make([]*ttlShard, shardCount)
	for i := range shards {
		sh := &ttlShard{
			heap: make(TTLHeap, 0),
			keys: make(map[string]*TTLItem),
		}
		heap.Init(&sh.heap)
		shards[i] = sh
	}

	tm := &TTLManager{
		shards:    shards,
		shardMask: shardCount - 1,
		stopChan:  make(chan struct{}),
		db:        db,
	}
	go tm.cleanupLoop()
	return tm
}

func ttlShardCount(shardHint int) int {
	if shardHint <= 0 {
		shardHint = 1
	}
	if shardHint > 256 {
		shardHint = 256
	}
	if shardHint < 16 {
		shardHint = 16
	}
	return 1 << uint(bits.Len(uint(shardHint-1)))
}

func acquireTTLItem(key string, expire int64) *TTLItem {
	item := ttlItemPool.Get().(*TTLItem)
	item.key = key
	item.expireAt = expire
	item.index = -1
	return item
}

func releaseTTLItem(item *TTLItem) {
	if item == nil {
		return
	}
	item.key = ""
	item.expireAt = 0
	item.index = -1
	ttlItemPool.Put(item)
}

func (tm *TTLManager) shardFor(key string) *ttlShard {
	hash := util.HashKey(key)
	return tm.shards[hash&tm.shardMask]
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
	for _, shard := range tm.shards {
		expired := shard.collectExpired(now)
		for _, key := range expired {
			tm.db.Delete(key)
		}
	}
}

func (s *ttlShard) collectExpired(now int64) []string {
	s.lock.Lock()
	defer s.lock.Unlock()

	var expired []string
	for s.heap.Len() > 0 {
		item := s.heap[0]
		if item.expireAt > now {
			break
		}
		popped := heap.Pop(&s.heap).(*TTLItem)
		delete(s.keys, popped.key)
		expired = append(expired, popped.key)
		releaseTTLItem(popped)
	}
	return expired
}

func (tm *TTLManager) SetTTL(key string, expireAt int64) {
	shard := tm.shardFor(key)
	shard.lock.Lock()
	defer shard.lock.Unlock()

	if item, exists := shard.keys[key]; exists {
		item.expireAt = expireAt
		heap.Fix(&shard.heap, item.index)
		return
	}

	item := acquireTTLItem(key, expireAt)
	heap.Push(&shard.heap, item)
	shard.keys[key] = item
}

func (tm *TTLManager) RemoveTTL(key string) {
	shard := tm.shardFor(key)
	shard.lock.Lock()
	if item, exists := shard.keys[key]; exists {
		removed := heap.Remove(&shard.heap, item.index).(*TTLItem)
		delete(shard.keys, key)
		releaseTTLItem(removed)
	}
	shard.lock.Unlock()
}

func (tm *TTLManager) ExpireAt(key string) (int64, bool) {
	shard := tm.shardFor(key)
	shard.lock.Lock()
	item, exists := shard.keys[key]
	if !exists {
		shard.lock.Unlock()
		return 0, false
	}
	expire := item.expireAt
	shard.lock.Unlock()
	return expire, true
}

func (tm *TTLManager) CheckTTL(key string) bool {
	now := time.Now().Unix()
	shard := tm.shardFor(key)

	shard.lock.Lock()
	item, exists := shard.keys[key]
	if !exists {
		shard.lock.Unlock()
		return true
	}
	if item.expireAt > now {
		shard.lock.Unlock()
		return true
	}

	removed := heap.Remove(&shard.heap, item.index).(*TTLItem)
	delete(shard.keys, key)
	shard.lock.Unlock()

	releaseTTLItem(removed)
	tm.db.Delete(key)
	return false
}

// MemDb is the memory cache database
// All key:value pairs are stored in db
// All ttl keys are stored in ttlKeys
// locks is used to lock a key for db to ensure some atomic operations
type MemDb struct {
	db    *ConcurrentMap
	ttl   *TTLManager
	locks *Locks
	// replicationFetcher returns runtime replication metadata using RESP types.
	replicationFetcher func() RESP.RedisData
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

// SetReplicationInfoFetcher registers a callback used by administrative commands
// (e.g. INFO replication) to obtain cluster metadata. The callback must return
// RESP-encoded data (typically BulkData) and may be nil to disable reporting.
func (m *MemDb) SetReplicationInfoFetcher(fetcher func() RESP.RedisData) {
	m.replicationFetcher = fetcher
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
	expireAt, exists := m.ttl.ExpireAt(key)
	if !exists {
		return -1
	}
	ttl := expireAt - time.Now().Unix()
	if ttl <= 0 {
		return -1
	}
	return ttl
}
