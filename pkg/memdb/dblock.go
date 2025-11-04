package memdb

import (
	"sort"
	"sync"

	"github.com/hsn0918/tinyredis/pkg/logger"
	"github.com/hsn0918/tinyredis/pkg/util"
)

type Locks struct {
	locks []*sync.RWMutex
}

func NewLocks(size int) *Locks {
	locks := make([]*sync.RWMutex, size)
	for i := 0; i < size; i++ {
		locks[i] = &sync.RWMutex{}
	}
	return &Locks{locks: locks}
}
func (l *Locks) GetKeyPos(key string) int {
	n := len(l.locks)
	if n == 0 {
		return -1
	}

	h := uint64(util.HashKey(key))

	if n&(n-1) == 0 {
		return int(h & uint64(n-1))
	}
	return int(h % uint64(n))
}
func (l *Locks) Lock(key string) {
	pos := l.GetKeyPos(key)
	if pos == -1 {
		logger.Errorf("Locks Lock key %s error: pos == -1", key)
		return
	}
	l.locks[pos].Lock()
}
func (l *Locks) UnLock(key string) {
	pos := l.GetKeyPos(key)
	if pos == -1 {
		logger.Errorf("Locks UnLock key %s error: pos == -1", key)
	}
	l.locks[pos].Unlock()
}
func (l *Locks) RLock(key string) {
	pos := l.GetKeyPos(key)
	if pos == -1 {
		logger.Errorf("Locks RLock key %s error: pos == -1", key)
	}
	l.locks[pos].RLock()
}
func (l *Locks) RUnLock(key string) {
	pos := l.GetKeyPos(key)
	if pos == -1 {
		logger.Errorf("Locks RUnLock key %s error: pos == -1", key)
	}
	l.locks[pos].RUnlock()
}
func (l *Locks) sortedLockPoses(keys []string) []int {
	set := make(map[int]struct{})
	for _, key := range keys {
		pos := l.GetKeyPos(key)
		if pos == -1 {
			logger.Errorf("Locks Lock key %s error: pos == -1", key)
			return nil
		}
		set[pos] = struct{}{}
	}
	poses := make([]int, 0, len(set))
	for pos := range set {
		poses = append(poses, pos)
	}
	sort.Ints(poses)
	return poses
}
func (l *Locks) LockMulti(keys []string) {
	poses := l.sortedLockPoses(keys)
	if poses == nil {
		return
	}
	for _, pos := range poses {
		l.locks[pos].Lock()
	}
}
func (l *Locks) UnLockMulti(keys []string) {
	poses := l.sortedLockPoses(keys)
	if poses == nil {
		return
	}
	for _, pos := range poses {
		l.locks[pos].Unlock()
	}
}
func (l *Locks) RLockMulti(keys []string) {
	poses := l.sortedLockPoses(keys)
	if poses == nil {
		return
	}
	for _, pos := range poses {
		l.locks[pos].RLock()
	}
}
func (l *Locks) RUnLockMulti(keys []string) {
	poses := l.sortedLockPoses(keys)
	if poses == nil {
		return
	}
	for _, pos := range poses {
		l.locks[pos].RUnlock()
	}
}
