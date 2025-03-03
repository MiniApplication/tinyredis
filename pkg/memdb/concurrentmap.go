package memdb

import (
	"sync"
	"sync/atomic"

	"github.com/hsn/tiny-redis/pkg/util"

	"github.com/emirpasic/gods/trees/redblacktree"
)

const (
	MaxConSize         = 1<<31 - 1
	TreeifyThreshold   = 8 // 链表转换为红黑树的阈值
	UntreeifyThreshold = 6 // 红黑树退化为链表的阈值
)

// listNode represents a node in a linked list.
type listNode struct {
	key   string
	value any
	next  *listNode
}

type dataStructure interface {
	Get(key string) (any, bool)
	Put(key string, value any) bool
	Remove(key string) bool
	Size() int
	Keys() []string
}

type linkedList struct {
	head  *listNode
	count int32
}

func (l *linkedList) Get(key string) (any, bool) {
	node := l.head
	for node != nil {
		if node.key == key {
			return node.value, true
		}
		node = node.next
	}
	return nil, false
}

func (l *linkedList) Put(key string, value any) bool {
	node := l.head
	for node != nil {
		if node.key == key {
			node.value = value
			return false
		}
		node = node.next
	}
	l.head = &listNode{key: key, value: value, next: l.head}
	atomic.AddInt32(&l.count, 1)
	return true
}

func (l *linkedList) Remove(key string) bool {
	var prev *listNode
	node := l.head
	for node != nil {
		if node.key == key {
			if prev == nil {
				l.head = node.next
			} else {
				prev.next = node.next
			}
			atomic.AddInt32(&l.count, -1)
			return true
		}
		prev = node
		node = node.next
	}
	return false
}

func (l *linkedList) Size() int {
	return int(atomic.LoadInt32(&l.count))
}

func (l *linkedList) Keys() []string {
	keys := make([]string, 0, l.Size())
	node := l.head
	for node != nil {
		keys = append(keys, node.key)
		node = node.next
	}
	return keys
}

// treeWrapper 包装 redblacktree.Tree 以实现 dataStructure 接口
type treeWrapper struct {
	tree *redblacktree.Tree
}

func newTreeWrapper() *treeWrapper {
	return &treeWrapper{
		tree: redblacktree.NewWithStringComparator(),
	}
}

func (t *treeWrapper) Get(key string) (any, bool) {
	return t.tree.Get(key)
}

func (t *treeWrapper) Put(key string, value any) bool {
	_, found := t.tree.Get(key)
	t.tree.Put(key, value)
	return !found
}

func (t *treeWrapper) Remove(key string) bool {
	_, found := t.tree.Get(key)
	if found {
		t.tree.Remove(key)
	}
	return found
}

func (t *treeWrapper) Size() int {
	return t.tree.Size()
}

func (t *treeWrapper) Keys() []string {
	keys := make([]string, 0, t.Size())
	it := t.tree.Iterator()
	for it.Next() {
		keys = append(keys, it.Key().(string))
	}
	return keys
}

// shard represents a shard within the ConcurrentMap.
type shard struct {
	data dataStructure
	rwMu sync.RWMutex
}

// ConcurrentMap 管理一个分片表，包含多个哈希表分片，提供线程安全的操作。
type ConcurrentMap struct {
	shards    []*shard
	shardMask int
	count     int32
}

// NewConcurrentMap 创建一个新的 ConcurrentMap 实例
func NewConcurrentMap(size int) *ConcurrentMap {
	if size > MaxConSize || size <= 0 {
		size = MaxConSize
	}
	// 确保 size 是 2 的幂
	size = 1 << uint(32-numberOfLeadingZeros(size-1))
	m := &ConcurrentMap{
		shards:    make([]*shard, size),
		shardMask: size - 1,
	}
	for i := 0; i < size; i++ {
		m.shards[i] = &shard{
			data: &linkedList{},
		}
	}
	return m
}

func numberOfLeadingZeros(i int) int {
	if i <= 0 {
		return 32
	}
	n := 1
	if i>>16 == 0 {
		n += 16
		i <<= 16
	}
	if i>>24 == 0 {
		n += 8
		i <<= 8
	}
	if i>>28 == 0 {
		n += 4
		i <<= 4
	}
	if i>>30 == 0 {
		n += 2
		i <<= 2
	}
	n -= i >> 31
	return n
}

// getKeyPos 计算键的哈希值并找到对应的分片
func (m *ConcurrentMap) getKeyPos(key string) int {
	hash := util.HashKey(key)
	return hash & m.shardMask
}

// Set 在 ConcurrentMap 中设置一个键值对。如果成功，返回 1，否则返回 0
func (m *ConcurrentMap) Set(key string, value any) int {
	pos := m.getKeyPos(key)
	shard := m.shards[pos]
	shard.rwMu.Lock()
	defer shard.rwMu.Unlock()

	if shard.data.Put(key, value) {
		atomic.AddInt32(&m.count, 1)
		// 检查是否需要转换为红黑树
		if ll, ok := shard.data.(*linkedList); ok && ll.Size() >= TreeifyThreshold {
			tree := newTreeWrapper()
			for _, key := range ll.Keys() {
				if val, ok := ll.Get(key); ok {
					tree.Put(key, val)
				}
			}
			shard.data = tree
		}
		return 1
	}
	return 0
}

// Get 从 ConcurrentMap 中获取一个键的值
func (m *ConcurrentMap) Get(key string) (any, bool) {
	pos := m.getKeyPos(key)
	shard := m.shards[pos]
	shard.rwMu.RLock()
	defer shard.rwMu.RUnlock()

	return shard.data.Get(key)
}

// Delete 从 ConcurrentMap 中删除一个键值对。如果成功，返回 1，否则返回 0
func (m *ConcurrentMap) Delete(key string) int {
	pos := m.getKeyPos(key)
	shard := m.shards[pos]
	shard.rwMu.Lock()
	defer shard.rwMu.Unlock()

	if shard.data.Remove(key) {
		atomic.AddInt32(&m.count, -1)
		// 检查是否需要将红黑树转换回链表
		if tree, ok := shard.data.(*treeWrapper); ok && tree.Size() <= UntreeifyThreshold {
			list := &linkedList{}
			keys := tree.Keys()
			for _, key := range keys {
				if val, ok := tree.Get(key); ok {
					list.Put(key, val)
				}
			}
			shard.data = list
		}
		return 1
	}
	return 0
}

// SetIfExist 在键存在时存储键值对。如果键存在，则替换为新值，返回 1，否则返回 0
func (m *ConcurrentMap) SetIfExist(key string, value any) int {
	pos := m.getKeyPos(key)
	shard := m.shards[pos]
	shard.rwMu.Lock()
	defer shard.rwMu.Unlock()

	if _, found := shard.data.Get(key); found {
		shard.data.Put(key, value)
		return 1
	}
	return 0
}

// SetIfNotExist 在键不存在时存储键值对。如果键不存在，则存储并返回 1，否则返回 0
func (m *ConcurrentMap) SetIfNotExist(key string, value any) int {
	pos := m.getKeyPos(key)
	shard := m.shards[pos]
	shard.rwMu.Lock()
	defer shard.rwMu.Unlock()

	if _, found := shard.data.Get(key); !found {
		shard.data.Put(key, value)
		atomic.AddInt32(&m.count, 1)
		return 1
	}
	return 0
}

// Len 返回 ConcurrentMap 中键值对的总数
func (m *ConcurrentMap) Len() int {
	return int(atomic.LoadInt32(&m.count))
}

// Clear 清空 ConcurrentMap 中的所有键值对
func (m *ConcurrentMap) Clear() {
	*m = *NewConcurrentMap(len(m.shards))
}

// Keys 返回 ConcurrentMap 中所有的键
func (m *ConcurrentMap) Keys() []string {
	totalLen := m.Len()
	if totalLen == 0 {
		return nil
	}

	keys := make([]string, 0, totalLen)
	for _, shard := range m.shards {
		shard.rwMu.RLock()
		keys = append(keys, shard.data.Keys()...)
		shard.rwMu.RUnlock()
	}
	return keys
}
