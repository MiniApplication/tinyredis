package memdb

import (
	"sync"

	"github.com/hsn/tiny-redis/pkg/util"

	"github.com/emirpasic/gods/trees/redblacktree"
)

const (
	MaxConSize       = 1<<31 - 1
	TreeifyThreshold = 8 // 链表转换为红黑树的阈值
)

// listNode represents a node in a linked list.
type listNode struct {
	key   string
	value any
	next  *listNode
}

// shard represents a shard within the ConcurrentMap.
type shard struct {
	head  *listNode          // 链表头
	tree  *redblacktree.Tree // 当元素数量超过 TreeifyThreshold 时，将链表转换为红黑树
	count int                // 当前桶中的元素数量
	rwMu  *sync.RWMutex      // 读写锁用于并发访问控制

	_ [40]byte // 缓存行填充，防止伪共享
}

// ConcurrentMap 管理一个分片表，包含多个哈希表分片，提供线程安全的操作。
type ConcurrentMap struct {
	table []*shard // 哈希表分片数组
	size  int      // 哈希表的大小
	count int      // 所有分片中键值对的总数
}

// NewConcurrentMap 创建一个新的 ConcurrentMap 实例
func NewConcurrentMap(size int) *ConcurrentMap {
	if size > MaxConSize || size <= 0 {
		size = MaxConSize
	}
	m := &ConcurrentMap{
		table: make([]*shard, size),
		size:  size,
		count: 0,
	}
	for i := 0; i < size; i++ {
		m.table[i] = &shard{
			rwMu: &sync.RWMutex{},
		}
	}
	return m
}

// getKeyPos 计算键的哈希值并找到对应的分片
func (m *ConcurrentMap) getKeyPos(key string) int {
	hash := util.HashKey(key)
	pos := hash % m.size
	if pos < 0 {
		pos += m.size
	}
	return pos
}

// Set 在 ConcurrentMap 中设置一个键值对。如果成功，返回 1，否则返回 0
func (m *ConcurrentMap) Set(key string, value any) int {
	added := 0
	pos := m.getKeyPos(key)
	shard := m.table[pos]
	shard.rwMu.Lock()
	defer shard.rwMu.Unlock()

	if shard.tree != nil {
		// 红黑树已存在，直接使用树插入
		_, exists := shard.tree.Get(key)
		shard.tree.Put(key, value)
		if !exists {
			m.count++
			shard.count++
			added = 1
		}
	} else {
		// 使用链表插入新元素
		node := shard.head
		for node != nil {
			if node.key == key {
				node.value = value
				return added
			}
			node = node.next
		}
		// 在链表头部插入新节点
		shard.head = &listNode{key: key, value: value, next: shard.head}
		shard.count++
		m.count++
		added = 1

		// 检查是否需要将链表转换为红黑树
		if shard.count >= TreeifyThreshold {
			shard.treeify()
		}
	}
	return added
}

func (s *shard) treeify() {
	if s.head == nil {
		return
	}
	s.tree = redblacktree.NewWithStringComparator()
	node := s.head
	for node != nil {
		s.tree.Put(node.key, node.value)
		node = node.next
	}
	s.head = nil
}

// Get 从 ConcurrentMap 中获取一个键的值
func (m *ConcurrentMap) Get(key string) (any, bool) {
	pos := m.getKeyPos(key)
	shard := m.table[pos]
	shard.rwMu.RLock()
	defer shard.rwMu.RUnlock()

	if shard.tree != nil {
		// 从红黑树中查找
		value, found := shard.tree.Get(key)
		return value, found
	} else {
		// 从链表中查找
		node := shard.head
		for node != nil {
			if node.key == key {
				return node.value, true
			}
			node = node.next
		}
	}
	return nil, false
}

// Delete 从 ConcurrentMap 中删除一个键值对。如果成功，返回 1，否则返回 0
func (m *ConcurrentMap) Delete(key string) int {
	pos := m.getKeyPos(key)
	shard := m.table[pos]
	shard.rwMu.Lock()
	defer shard.rwMu.Unlock()

	if shard.tree != nil {
		// 从红黑树中删除
		_, found := shard.tree.Get(key)
		if found {
			shard.tree.Remove(key)
			shard.count--
			m.count--
			return 1
		}
	} else {
		// 从链表中删除
		var prev *listNode
		node := shard.head
		for node != nil {
			if node.key == key {
				if prev == nil {
					shard.head = node.next
				} else {
					prev.next = node.next
				}
				shard.count--
				m.count--
				return 1
			}
			prev = node
			node = node.next
		}
	}
	return 0
}

// SetIfExist 在键存在时存储键值对。如果键存在，则替换为新值，返回 1，否则返回 0
func (m *ConcurrentMap) SetIfExist(key string, value any) int {
	pos := m.getKeyPos(key)
	shard := m.table[pos]
	shard.rwMu.Lock()
	defer shard.rwMu.Unlock()

	if shard.tree != nil {
		if _, found := shard.tree.Get(key); found {
			shard.tree.Put(key, value)
			return 1
		}
	} else {
		node := shard.head
		for node != nil {
			if node.key == key {
				node.value = value
				return 1
			}
			node = node.next
		}
	}
	return 0
}

// SetIfNotExist 在键不存在时存储键值对。如果键不存在，则存储并返回 1，否则返回 0
func (m *ConcurrentMap) SetIfNotExist(key string, value any) int {
	pos := m.getKeyPos(key)
	shard := m.table[pos]
	shard.rwMu.Lock()
	defer shard.rwMu.Unlock()

	if shard.tree != nil {
		if _, found := shard.tree.Get(key); !found {
			shard.tree.Put(key, value)
			shard.count++
			m.count++
			return 1
		}
	} else {
		node := shard.head
		for node != nil {
			if node.key == key {
				return 0
			}
			node = node.next
		}
		// 在链表头部插入新节点
		shard.head = &listNode{key: key, value: value, next: shard.head}
		shard.count++
		m.count++

		// 检查是否需要将链表转换为红黑树
		if shard.count >= TreeifyThreshold {
			shard.treeify()
		}
		return 1
	}
	return 0
}

// Len 返回 ConcurrentMap 中键值对的总数
func (m *ConcurrentMap) Len() int {
	return m.count
}

// Clear 清空 ConcurrentMap 中的所有键值对
func (m *ConcurrentMap) Clear() {
	*m = *NewConcurrentMap(m.size)
}

// Keys 返回 ConcurrentMap 中所有的键
func (m *ConcurrentMap) Keys() []string {
	keys := make([]string, m.count)
	i := 0
	for _, shard := range m.table {
		shard.rwMu.RLock()
		if shard.tree != nil {
			it := shard.tree.Iterator()
			for it.Next() {
				keys[i] = it.Key().(string)
				i++
			}
		} else {
			node := shard.head
			for node != nil {
				keys[i] = node.key
				i++
				node = node.next
			}
		}
		shard.rwMu.RUnlock()
	}
	return keys
}
