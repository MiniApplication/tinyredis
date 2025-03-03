package memdb

import "math/rand/v2"

const (
	MaxLevel    = 32
	Probability = 0.25
)

type zSetNode struct {
	member  string
	score   float64
	forward []*zSetNode
}
type ZSet struct {
	header *zSetNode
	level  int
	length int
}

func NewZSetNode(level int, member string, score float64) *zSetNode {
	return &zSetNode{
		member:  member,
		score:   score,
		forward: make([]*zSetNode, level),
	}
}

func NewZSet() *ZSet {
	return &ZSet{
		header: NewZSetNode(MaxLevel, "", 0),
		level:  1,
		length: 0, // 初始化长度为 0
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
	update := make([]*zSetNode, MaxLevel)
	current := z.header

	// 先查找是否已存在
	for i := z.level - 1; i >= 0; i-- {
		for current.forward[i] != nil &&
			(current.forward[i].score < score ||
				(current.forward[i].score == score && current.forward[i].member < member)) {
			current = current.forward[i]
		}
		update[i] = current
	}

	current = current.forward[0]

	// 如果成员已存在且分数相同，返回 false
	if current != nil && current.member == member {
		if current.score == score {
			return false
		}
		// 如果分数不同，删除旧节点
		for i := 0; i < z.level; i++ {
			if update[i].forward[i] != current {
				break
			}
			update[i].forward[i] = current.forward[i]
		}
	}

	newLevel := randomLevel()
	if newLevel > z.level {
		for i := z.level; i < newLevel; i++ {
			update[i] = z.header
		}
		z.level = newLevel
	}

	newNode := NewZSetNode(newLevel, member, score)
	for i := 0; i < newLevel; i++ {
		newNode.forward[i] = update[i].forward[i]
		update[i].forward[i] = newNode
	}

	if current == nil || current.member != member {
		z.length++
	}
	return true
}

func (z *ZSet) Get(member string) float64 {
	current := z.header
	for i := z.level - 1; i >= 0; i-- {
		for current.forward[i] != nil && current.forward[i].member < member {
			current = current.forward[i]
		}
	}
	current = current.forward[0]
	if current != nil && current.member == member {
		return current.score
	}
	return -1
}
func (z *ZSet) Remove(member string, score float64) bool {
	update := make([]*zSetNode, MaxLevel)
	current := z.header

	for i := z.level - 1; i >= 0; i-- {
		for current.forward[i] != nil && current.forward[i].score < score {
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
			update[i].forward[i] = current.forward[i]
		}

		for z.level > 1 && z.header.forward[z.level-1] == nil {
			z.level--
		}

		z.length--
		return true
	}

	return false
}

func (z *ZSet) Len() int {
	return z.length
}

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

	current := z.header.forward[0]
	for i := 0; i < start && current != nil; i++ {
		current = current.forward[0]
	}

	for i := start; i <= stop && current != nil; i++ {
		members = append(members, current.member)
		scores = append(scores, current.score)
		current = current.forward[0]
	}

	return members, scores
}

func (z *ZSet) Rank(member string) int {
	rank := 0
	current := z.header

	for i := z.level - 1; i >= 0; i-- {
		for current.forward[i] != nil &&
			(current.forward[i].member < member ||
				(current.forward[i].member == member && current.forward[i].score <= current.score)) {
			if current.forward[i].member != member {
				rank++
			}
			current = current.forward[i]
		}
	}

	if current != nil && current.member == member {
		return rank
	}
	return -1
}
