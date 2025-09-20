package util

import (
	"fmt"
	"math"
	"strconv"
	"testing"
)

// TestHashKeyDistribution 测试哈希函数的分布均匀性
func TestHashKeyDistribution(t *testing.T) {
	const (
		numKeys   = 100000
		numBucket = 1024
	)

	buckets := make([]int, numBucket)

	// 测试连续数字键的分布
	for i := 0; i < numKeys; i++ {
		key := "key:" + strconv.Itoa(i)
		hash := HashKey(key)
		bucket := hash % numBucket
		if bucket < 0 {
			bucket = -bucket
		}
		buckets[bucket]++
	}

	// 计算分布的标准差
	avg := float64(numKeys) / float64(numBucket)
	var variance float64
	minCount := numKeys
	maxCount := 0

	for _, count := range buckets {
		diff := float64(count) - avg
		variance += diff * diff
		if count < minCount {
			minCount = count
		}
		if count > maxCount {
			maxCount = count
		}
	}

	stdDev := math.Sqrt(variance / float64(numBucket))
	distribution := (stdDev / avg) * 100

	t.Logf("Average per bucket: %.2f", avg)
	t.Logf("Standard deviation: %.2f", stdDev)
	t.Logf("Distribution quality: %.2f%%", distribution)
	t.Logf("Min/Max bucket count: %d/%d", minCount, maxCount)

	// 分布质量应该小于10%（经验值）
	if distribution > 10 {
		t.Errorf("Poor hash distribution: %.2f%% (expected < 10%%)", distribution)
	}
}

// BenchmarkHashKey 测试哈希函数性能
func BenchmarkHashKey(b *testing.B) {
	keys := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		keys[i] = fmt.Sprintf("test:key:%d:data", i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = HashKey(keys[i%1000])
	}
}

// BenchmarkHashKeyParallel 测试并发哈希性能
func BenchmarkHashKeyParallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := "key:" + strconv.Itoa(i)
			_ = HashKey(key)
			i++
		}
	})
}

// TestHashKeyCollision 测试哈希冲突率
func TestHashKeyCollision(t *testing.T) {
	const numKeys = 50000
	seen := make(map[int]string)
	collisions := 0

	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("user:%d:profile", i)
		hash := HashKey(key)

		// 模拟1024个分片的情况
		bucket := hash & 1023

		if existingKey, exists := seen[bucket]; exists && existingKey != key {
			collisions++
		} else {
			seen[bucket] = key
		}
	}

	collisionRate := float64(collisions) / float64(numKeys) * 100
	t.Logf("Collision rate for %d keys in 1024 buckets: %.2f%%", numKeys, collisionRate)

	// 对于1024个桶和50000个键，冲突率应该很高是正常的
	// 我们只是确保哈希函数工作正常
	if len(seen) < 1000 {
		t.Errorf("Too many hash collisions, only %d unique buckets used", len(seen))
	}
}

// TestHashKeyDifferentPatterns 测试不同模式的键
func TestHashKeyDifferentPatterns(t *testing.T) {
	patterns := []struct {
		name    string
		genKey  func(int) string
		numKeys int
	}{
		{"sequential", func(i int) string { return fmt.Sprintf("key:%d", i) }, 10000},
		{"uuid-like", func(i int) string { return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", i, i, i, i, i) }, 10000},
		{"prefix-heavy", func(i int) string { return fmt.Sprintf("prefix:prefix:prefix:%d", i) }, 10000},
		{"suffix-heavy", func(i int) string { return fmt.Sprintf("%d:suffix:suffix:suffix", i) }, 10000},
	}

	const numBucket = 256

	for _, pattern := range patterns {
		t.Run(pattern.name, func(t *testing.T) {
			buckets := make([]int, numBucket)

			for i := 0; i < pattern.numKeys; i++ {
				key := pattern.genKey(i)
				hash := HashKey(key)
				bucket := hash % numBucket
				if bucket < 0 {
					bucket = -bucket
				}
				buckets[bucket]++
			}

			// 计算分布
			avg := float64(pattern.numKeys) / float64(numBucket)
			var variance float64
			for _, count := range buckets {
				diff := float64(count) - avg
				variance += diff * diff
			}
			stdDev := math.Sqrt(variance / float64(numBucket))
			distribution := (stdDev / avg) * 100

			t.Logf("Pattern %s: distribution quality %.2f%%", pattern.name, distribution)
		})
	}
}
