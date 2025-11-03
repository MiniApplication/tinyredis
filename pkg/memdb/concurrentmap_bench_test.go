package memdb

import (
	"fmt"
	"math/rand"
	"runtime"
	"strconv"
	"sync"
	"testing"
)

// BenchmarkConcurrentMapSet 测试并发写入性能
func BenchmarkConcurrentMapSet(b *testing.B) {
	sizes := []int{1000, 10000, 100000}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
			m := NewConcurrentMap(1024)
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				i := rand.Int()
				for pb.Next() {
					key := strconv.Itoa(i)
					m.Set(key, i)
					i++
				}
			})
		})
	}
}

// BenchmarkConcurrentMapGet 测试并发读取性能
func BenchmarkConcurrentMapGet(b *testing.B) {
	m := NewConcurrentMap(1024)
	// 预填充数据
	for i := 0; i < 100000; i++ {
		m.Set(strconv.Itoa(i), i)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := rand.Intn(100000)
		for pb.Next() {
			key := strconv.Itoa(i)
			m.Get(key)
			i = (i + 1) % 100000
		}
	})
}

// BenchmarkConcurrentMapMixed 测试混合读写性能
func BenchmarkConcurrentMapMixed(b *testing.B) {
	m := NewConcurrentMap(1024)
	// 预填充数据
	for i := 0; i < 10000; i++ {
		m.Set(strconv.Itoa(i), i)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := rand.Int()
		for pb.Next() {
			if i%10 < 8 { // 80% 读取
				m.Get(strconv.Itoa(i % 10000))
			} else { // 20% 写入
				m.Set(strconv.Itoa(i), i)
			}
			i++
		}
	})
}

// BenchmarkConcurrentMapDelete 测试删除性能
func BenchmarkConcurrentMapDelete(b *testing.B) {
	b.StopTimer()
	m := NewConcurrentMap(1024)
	keys := make([]string, b.N)
	for i := 0; i < b.N; i++ {
		keys[i] = strconv.Itoa(i)
		m.Set(keys[i], i)
	}
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		m.Delete(keys[i])
	}
}

// BenchmarkTreeifyThreshold 测试链表转红黑树的性能影响
func BenchmarkTreeifyThreshold(b *testing.B) {
	thresholds := []int{5, 8, 10, 15}
	for _, threshold := range thresholds {
		b.Run(fmt.Sprintf("threshold_%d", threshold), func(b *testing.B) {
			m := NewConcurrentMap(1)
			// 强制所有键映射到同一分片
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				key := fmt.Sprintf("key_%d", i%threshold*2)
				m.Set(key, i)
			}
		})
	}
}

// BenchmarkConcurrentMapKeys 测试获取所有键的性能
func BenchmarkConcurrentMapKeys(b *testing.B) {
	sizes := []int{100, 1000, 10000}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
			m := NewConcurrentMap(1024)
			for i := 0; i < size; i++ {
				m.Set(strconv.Itoa(i), i)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = m.Keys()
			}
		})
	}
}

// BenchmarkConcurrentMapSetIfNotExist 测试条件设置性能
func BenchmarkConcurrentMapSetIfNotExist(b *testing.B) {
	m := NewConcurrentMap(1024)
	// 预填充一半数据
	for i := 0; i < b.N/2; i++ {
		m.Set(strconv.Itoa(i*2), i*2)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := rand.Int()
		for pb.Next() {
			m.SetIfNotExist(strconv.Itoa(i), i)
			i++
		}
	})
}

// BenchmarkMemoryAllocation 测试内存分配
func BenchmarkMemoryAllocation(b *testing.B) {
	b.Run("linkedList", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ll := &linkedList{}
			for j := 0; j < 10; j++ {
				ll.Put(strconv.Itoa(j), j)
			}
		}
	})

	b.Run("treeWrapper", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			tree := newTreeWrapper()
			for j := 0; j < 10; j++ {
				tree.Put(strconv.Itoa(j), j)
			}
		}
	})
}

// BenchmarkConcurrentMapHighContention 测试高竞争场景
func BenchmarkConcurrentMapHighContention(b *testing.B) {
	shardCounts := []int{16, 256, 1024, 4096}
	for _, shardCount := range shardCounts {
		b.Run(fmt.Sprintf("shards_%d", shardCount), func(b *testing.B) {
			m := NewConcurrentMap(shardCount)
			numGoroutines := runtime.NumCPU() * 2
			keysPerGoroutine := 100

			b.ResetTimer()
			var wg sync.WaitGroup
			for g := 0; g < numGoroutines; g++ {
				wg.Add(1)
				go func(goroutineID int) {
					defer wg.Done()
					for i := 0; i < b.N/numGoroutines; i++ {
						// 每个goroutine操作有限的键集合，增加竞争
						key := strconv.Itoa(goroutineID*keysPerGoroutine + (i % keysPerGoroutine))
						if i%2 == 0 {
							m.Set(key, i)
						} else {
							m.Get(key)
						}
					}
				}(g)
			}
			wg.Wait()
		})
	}
}

// BenchmarkNumberOfLeadingZeros 测试位操作优化
func BenchmarkNumberOfLeadingZeros(b *testing.B) {
	nums := make([]int, 1000)
	for i := range nums {
		nums[i] = rand.Intn(1 << 30)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = numberOfLeadingZeros(nums[i%1000])
	}
}
