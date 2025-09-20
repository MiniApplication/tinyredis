package memdb

import (
	"math"
	"math/rand/v2"
)

const (
	MaxLevel    = 32
	Probability = 0.25
)

// zSetNode 跳表节点
type zSetNode struct {
	member  string
	score   float64
	forward []*zSetNode
	span    []int // 每层的跨度，用于计算排名
}

// ZSet 有序集合，使用跳表+哈希表实现
type ZSet struct {
	header *zSetNode
	tail   *zSetNode
	level  int
	length int
	dict   map[string]float64
}

func NewZSetNode(level int, member string, score float64) *zSetNode {
	return &zSetNode{
		member:  member,
		score:   score,
		forward: make([]*zSetNode, level),
		span:    make([]int, level),
	}
}

func NewZSet() *ZSet {
	return &ZSet{
		header: NewZSetNode(MaxLevel, "", 0),
		tail:   nil,
		level:  1,
		length: 0,
		dict:   make(map[string]float64),
	}
}

// randomLevel generates a random level for a new node.
func randomLevel() int {
	level := 1
	for rand.Float64() < Probability && level < MaxLevel {
		level++
	}
	return level
}
func (z *ZSet) Add(member string, score float64) bool {
	// 检查是否已存在
	if oldScore, exists := z.dict[member]; exists {
		if oldScore == score {
			return false // 分数相同，无需更新
		}
		// 分数不同，先删除旧的
		z.deleteNode(member, oldScore)
	}

	update := make([]*zSetNode, MaxLevel)
	rank := make([]int, MaxLevel)
	current := z.header

	// 查找插入位置
	for i := z.level - 1; i >= 0; i-- {
		if i == z.level-1 {
			rank[i] = 0
		} else {
			rank[i] = rank[i+1]
		}

		for current.forward[i] != nil &&
			(current.forward[i].score < score ||
				(current.forward[i].score == score && current.forward[i].member < member)) {
			rank[i] += current.span[i]
			current = current.forward[i]
		}
		update[i] = current
	}

	newLevel := randomLevel()
	if newLevel > z.level {
		for i := z.level; i < newLevel; i++ {
			rank[i] = 0
			update[i] = z.header
			update[i].span[i] = z.length
		}
		z.level = newLevel
	}

	newNode := NewZSetNode(newLevel, member, score)
	for i := 0; i < newLevel; i++ {
		newNode.forward[i] = update[i].forward[i]
		update[i].forward[i] = newNode

		// 更新span
		newNode.span[i] = update[i].span[i] - (rank[0] - rank[i])
		update[i].span[i] = (rank[0] - rank[i]) + 1
	}

	// 更新未涉及层的span
	for i := newLevel; i < z.level; i++ {
		update[i].span[i]++
	}

	// 更新尾节点
	if newNode.forward[0] == nil {
		z.tail = newNode
	}

	// 更新字典
	z.dict[member] = score
	z.length++
	return true
}

// Get 获取成员的分数，使用哈希表O(1)查找
func (z *ZSet) Get(member string) float64 {
	if score, exists := z.dict[member]; exists {
		return score
	}
	return math.NaN() // 使用NaN表示不存在
}

// Remove 删除成员
func (z *ZSet) Remove(member string) bool {
	score, exists := z.dict[member]
	if !exists {
		return false
	}
	z.deleteNode(member, score)
	return true
}

// deleteNode 内部删除节点方法
func (z *ZSet) deleteNode(member string, score float64) {
	update := make([]*zSetNode, MaxLevel)
	current := z.header

	for i := z.level - 1; i >= 0; i-- {
		for current.forward[i] != nil &&
			(current.forward[i].score < score ||
				(current.forward[i].score == score && current.forward[i].member < member)) {
			current = current.forward[i]
		}
		update[i] = current
	}

	current = current.forward[0]
	if current != nil && current.score == score && current.member == member {
		for i := 0; i < z.level; i++ {
			if update[i].forward[i] != current {
				break
			}
			update[i].span[i] += current.span[i] - 1
			update[i].forward[i] = current.forward[i]
		}

		// 更新未涉及层的span
		for i := z.level - 1; i >= 0; i-- {
			if update[i].forward[i] == nil {
				update[i].span[i]--
			}
		}

		// 更新尾节点
		if current.forward[0] == nil {
			if update[0] == z.header {
				z.tail = nil
			} else {
				z.tail = update[0]
			}
		}

		for z.level > 1 && z.header.forward[z.level-1] == nil {
			z.level--
		}

		delete(z.dict, member)
		z.length--
	}
}

func (z *ZSet) Len() int {
	return z.length
}

// Count 统计分数范围内的成员数量
func (z *ZSet) Count(min, max float64) int {
	count := 0
	current := z.header.forward[0]

	for current != nil {
		if current.score > max {
			break
		}
		if current.score >= min {
			count++
		}
		current = current.forward[0]
	}

	return count
}

// RangeByScore 按分数范围获取成员
func (z *ZSet) RangeByScore(min, max float64, offset, count int) ([]string, []float64) {
	if offset < 0 {
		offset = 0
	}

	members := make([]string, 0)
	scores := make([]float64, 0)

	current := z.header.forward[0]
	skipped := 0

	for current != nil {
		if current.score > max {
			break
		}

		if current.score >= min {
			if skipped >= offset {
				members = append(members, current.member)
				scores = append(scores, current.score)
				if count > 0 && len(members) >= count {
					break
				}
			} else {
				skipped++
			}
		}
		current = current.forward[0]
	}

	return members, scores
}

// IncrBy 增加成员的分数
func (z *ZSet) IncrBy(member string, increment float64) float64 {
	oldScore, exists := z.dict[member]
	var newScore float64

	if exists {
		newScore = oldScore + increment
		z.deleteNode(member, oldScore)
	} else {
		newScore = increment
	}

	z.Add(member, newScore)
	return newScore
}

// RevRank 获取成员的反向排名
func (z *ZSet) RevRank(member string) int {
	rank := z.Rank(member)
	if rank == -1 {
		return -1
	}
	return z.length - rank - 1
}

// RemRangeByRank 按排名范围删除成员
func (z *ZSet) RemRangeByRank(start, stop int) int {
	if start < 0 {
		start = z.length + start
	}
	if stop < 0 {
		stop = z.length + stop
	}

	if start < 0 {
		start = 0
	}
	if stop >= z.length {
		stop = z.length - 1
	}

	if start > stop || start >= z.length {
		return 0
	}

	// 先收集要删除的成员
	members, _ := z.Range(start, stop)
	removed := 0

	for _, member := range members {
		if z.Remove(member) {
			removed++
		}
	}

	return removed
}

// RemRangeByScore 按分数范围删除成员
func (z *ZSet) RemRangeByScore(min, max float64) int {
	toRemove := make([]string, 0)
	current := z.header.forward[0]

	for current != nil {
		if current.score > max {
			break
		}
		if current.score >= min {
			toRemove = append(toRemove, current.member)
		}
		current = current.forward[0]
	}

	removed := 0
	for _, member := range toRemove {
		if z.Remove(member) {
			removed++
		}
	}

	return removed
}

// Range 获取指定排名范围的成员
func (z *ZSet) Range(start, stop int) ([]string, []float64) {
	if start < 0 {
		start = z.length + start
	}
	if stop < 0 {
		stop = z.length + stop
	}

	if start < 0 {
		start = 0
	}
	if stop >= z.length {
		stop = z.length - 1
	}

	if start > stop || start >= z.length {
		return nil, nil
	}

	members := make([]string, 0, stop-start+1)
	scores := make([]float64, 0, stop-start+1)

	current := z.header
	// 使用span快速跳过
	traversed := 0
	for i := z.level - 1; i >= 0; i-- {
		for current.forward[i] != nil && traversed+current.span[i] <= start {
			traversed += current.span[i]
			current = current.forward[i]
		}
	}

	current = current.forward[0]
	for i := start; i <= stop && current != nil; i++ {
		members = append(members, current.member)
		scores = append(scores, current.score)
		current = current.forward[0]
	}

	return members, scores
}

// RevRange 反向获取指定排名范围的成员
func (z *ZSet) RevRange(start, stop int) ([]string, []float64) {
	if start < 0 {
		start = z.length + start
	}
	if stop < 0 {
		stop = z.length + stop
	}

	if start < 0 {
		start = 0
	}
	if stop >= z.length {
		stop = z.length - 1
	}

	if start > stop || start >= z.length {
		return nil, nil
	}

	// 转换为正向索引
	realStart := z.length - stop - 1
	realStop := z.length - start - 1

	// 使用正向遍历获取结果
	members, scores := z.Range(realStart, realStop)

	// 反转结果
	for i, j := 0, len(members)-1; i < j; i, j = i+1, j-1 {
		members[i], members[j] = members[j], members[i]
		scores[i], scores[j] = scores[j], scores[i]
	}

	return members, scores
}

// Rank 获取成员排名（从0开始）
func (z *ZSet) Rank(member string) int {
	score, exists := z.dict[member]
	if !exists {
		return -1
	}

	rank := 0
	current := z.header

	for i := z.level - 1; i >= 0; i-- {
		for current.forward[i] != nil &&
			(current.forward[i].score < score ||
				(current.forward[i].score == score && current.forward[i].member <= member)) {
			rank += current.span[i]
			current = current.forward[i]
		}
	}

	if current != nil && current.member == member {
		return rank - 1
	}
	return -1
}
