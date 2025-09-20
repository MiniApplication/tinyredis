package memdb

import (
	"math/bits"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/hsn0918/tinyredis/pkg/util"

	"github.com/emirpasic/gods/trees/redblacktree"
)

const (
	MaxConSize         = 1<<31 - 1
	TreeifyThreshold   = 8 // 链表转换为红黑树的阈值
	UntreeifyThreshold = 6 // 红黑树退化为链表的阈值
)

// listNode represents a node in a linked list.
// 优化内存布局，将指针放在前面提升缓存局部性
type listNode struct {
	next  *listNode
	key   string
	value any
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
	tail  *listNode // 添加尾指针，优化插入操作
	count int32
}

func (l *linkedList) Get(key string) (any, bool) {
	// 快速路径：空链表检查
	if l.head == nil {
		return nil, false
	}

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
	// 快速路径：空链表直接插入
	if l.head == nil {
		newNode := &listNode{key: key, value: value}
		l.head = newNode
		l.tail = newNode
		atomic.AddInt32(&l.count, 1)
		return true
	}

	// 检查是否已存在
	node := l.head
	for node != nil {
		if node.key == key {
			node.value = value
			return false
		}
		node = node.next
	}

	// 头部插入新节点（缓存友好）
	newNode := &listNode{next: l.head, key: key, value: value}
	l.head = newNode
	atomic.AddInt32(&l.count, 1)
	return true
}

func (l *linkedList) Remove(key string) bool {
	// 快速路径：空链表
	if l.head == nil {
		return false
	}

	// 特殊处理头节点
	if l.head.key == key {
		l.head = l.head.next
		if l.head == nil {
			l.tail = nil
		}
		atomic.AddInt32(&l.count, -1)
		return true
	}

	var prev *listNode
	node := l.head
	for node != nil {
		if node.key == key {
			prev.next = node.next
			if node == l.tail {
				l.tail = prev
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
// 优化内存布局，加入padding避免false sharing
type shard struct {
	rwMu sync.RWMutex
	data dataStructure
	_    [64 - unsafe.Sizeof(sync.RWMutex{}) - unsafe.Sizeof((*linkedList)(nil))]byte // CPU cache line padding
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

// 使用内置位操作函数优化
func numberOfLeadingZeros(i int) int {
	if i <= 0 {
		return 32
	}
	return bits.LeadingZeros32(uint32(i))
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

	isNew := shard.data.Put(key, value)
	if isNew {
		atomic.AddInt32(&m.count, 1)
		// 检查是否需要转换为红黑树
		if ll, ok := shard.data.(*linkedList); ok && ll.Size() >= TreeifyThreshold {
			// 优化：直接遍历链表节点，避免Keys()分配
			tree := newTreeWrapper()
			node := ll.head
			for node != nil {
				tree.Put(node.key, node.value)
				node = node.next
			}
			shard.data = tree
		}
	}

	shard.rwMu.Unlock()
	if isNew {
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

	if shard.data.Remove(key) {
		atomic.AddInt32(&m.count, -1)
		// 检查是否需要将红黑树转换回链表
		if tree, ok := shard.data.(*treeWrapper); ok && tree.Size() <= UntreeifyThreshold {
			list := &linkedList{}
			// 优化：使用迭代器避免Keys()分配
			it := tree.tree.Iterator()
			for it.Next() {
				list.Put(it.Key().(string), it.Value())
			}
			shard.data = list
		}
		shard.rwMu.Unlock()
		return 1
	}
	shard.rwMu.Unlock()
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

	// 优化：先用读锁检查
	shard.rwMu.RLock()
	if _, found := shard.data.Get(key); found {
		shard.rwMu.RUnlock()
		return 0
	}
	shard.rwMu.RUnlock()

	// 再用写锁设置（double-check pattern）
	shard.rwMu.Lock()
	if _, found := shard.data.Get(key); !found {
		if shard.data.Put(key, value) {
			atomic.AddInt32(&m.count, 1)
			// 检查是否需要转换为红黑树
			if ll, ok := shard.data.(*linkedList); ok && ll.Size() >= TreeifyThreshold {
				tree := newTreeWrapper()
				node := ll.head
				for node != nil {
					tree.Put(node.key, node.value)
					node = node.next
				}
				shard.data = tree
			}
		}
		shard.rwMu.Unlock()
		return 1
	}
	shard.rwMu.Unlock()
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

	// 优化：预分配精确容量，避免扩容
	keys := make([]string, 0, totalLen)

	// 批量收集keys，减少锁的获取次数
	for _, shard := range m.shards {
		if shard.data.Size() == 0 {
			continue // 跳过空分片
		}
		shard.rwMu.RLock()
		keys = append(keys, shard.data.Keys()...)
		shard.rwMu.RUnlock()
	}
	return keys
}
