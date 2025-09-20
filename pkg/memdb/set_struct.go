package memdb

import (
	"math/rand"
	"sync/atomic"
)

type void struct{}

type Set struct {
	table map[string]void
	size  int64 // Atomic counter for thread-safe size tracking
}

func NewSet() *Set {
	return &Set{
		table: make(map[string]void),
		size:  0,
	}
}

func (s *Set) Add(key string) int {
	if _, exists := s.table[key]; exists {
		return 0
	}
	s.table[key] = void{}
	atomic.AddInt64(&s.size, 1)
	return 1
}

func (s *Set) Remove(key string) int {
	if _, exists := s.table[key]; exists {
		delete(s.table, key)
		atomic.AddInt64(&s.size, -1)
		return 1
	}
	return 0
}

func (s *Set) Len() int {
	return int(atomic.LoadInt64(&s.size))
}

func (s *Set) Has(key string) bool {
	_, ok := s.table[key]
	return ok
}

func (s *Set) Pop() string {
	for key := range s.table {
		s.Remove(key)
		return key
	}
	return ""
}

func (s *Set) Clear() {
	// For large sets, creating new map is more efficient than deleting each key
	if s.Len() > 256 {
		s.table = make(map[string]void)
	} else {
		for k := range s.table {
			delete(s.table, k)
		}
	}
	atomic.StoreInt64(&s.size, 0)
}

func (s *Set) Members() []string {
	res := make([]string, 0, len(s.table))
	for key := range s.table {
		res = append(res, key)
	}
	return res
}

func (s *Set) Union(sets ...*Set) *Set {
	// Pre-allocate with estimated size
	totalSize := s.Len()
	for _, set := range sets {
		totalSize += set.Len()
	}

	res := &Set{
		table: make(map[string]void, totalSize),
		size:  0,
	}

	// Direct copy without Has check
	for key := range s.table {
		res.table[key] = void{}
	}
	res.size = int64(len(res.table))

	for _, set := range sets {
		for key := range set.table {
			if _, exists := res.table[key]; !exists {
				res.table[key] = void{}
				atomic.AddInt64(&res.size, 1)
			}
		}
	}
	return res
}

func (s *Set) Intersect(sets ...*Set) *Set {
	if len(sets) == 0 {
		res := &Set{
			table: make(map[string]void, len(s.table)),
			size:  int64(len(s.table)),
		}
		for key := range s.table {
			res.table[key] = void{}
		}
		return res
	}

	// Find smallest set for optimization
	smallest := s
	for _, set := range sets {
		if set.Len() < smallest.Len() {
			smallest = set
		}
	}

	res := NewSet()
	// Only check keys from smallest set
	for key := range smallest.table {
		exists := true

		// Check if key exists in original set (if smallest is not s)
		if smallest != s {
			if _, ok := s.table[key]; !ok {
				continue
			}
		}

		// Check other sets
		for _, set := range sets {
			if set == smallest {
				continue
			}
			if _, ok := set.table[key]; !ok {
				exists = false
				break
			}
		}

		if exists {
			res.table[key] = void{}
			atomic.AddInt64(&res.size, 1)
		}
	}
	return res
}

func (s *Set) Difference(sets ...*Set) *Set {
	res := NewSet()
	for key := range s.table {
		res.Add(key)
	}
	for _, set := range sets {
		for key := range set.table {
			res.Remove(key)
		}
	}
	return res
}

func (s *Set) IsSubset(set *Set) bool {
	for key := range s.table {
		if !set.Has(key) {
			return false
		}
	}
	return true
}

// Random returns random members of the set.
// if count > 0, return min(len(set), count) unique random members
// if count < 0, return exactly |count| random members (may contain duplicates)
func (s *Set) Random(count int) []string {
	size := s.Len()
	if count == 0 || size == 0 {
		return []string{}
	}

	if count > 0 {
		// Return unique random members
		if count >= size {
			// Return all members
			return s.Members()
		}

		// For small counts, use reservoir sampling
		res := make([]string, 0, count)
		i := 0
		for key := range s.table {
			if i < count {
				res = append(res, key)
			} else {
				// Reservoir sampling
				j := rand.Intn(i + 1)
				if j < count {
					res[j] = key
				}
			}
			i++
		}
		return res
	} else {
		// Return with possible duplicates
		count = -count
		members := s.Members()
		if size == 0 {
			return []string{}
		}

		res := make([]string, count)
		for i := 0; i < count; i++ {
			res[i] = members[rand.Intn(size)]
		}
		return res
	}
}
