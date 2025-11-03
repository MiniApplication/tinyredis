package memdb

import "strconv"

// Hash 优化的哈希表结构
type Hash struct {
	table map[string][]byte
	size  int
}

func NewHash() *Hash {
	return &Hash{
		table: make(map[string][]byte, 16),
		size:  0,
	}
}

func (h *Hash) Set(key string, value []byte) {
	if _, exists := h.table[key]; !exists {
		h.size++
	}
	h.table[key] = value
}

func (h *Hash) Get(key string) []byte {
	return h.table[key] // 保持简单高效
}

func (h *Hash) Del(key string) int {
	if _, ok := h.table[key]; ok {
		delete(h.table, key)
		h.size--
		return 1
	}
	return 0
}

func (h *Hash) Len() int {
	return h.size
}

func (h *Hash) Keys() []string {
	keys := make([]string, 0, len(h.table))
	for key := range h.table {
		keys = append(keys, key)
	}
	return keys
}

func (h *Hash) Values() [][]byte {
	values := make([][]byte, 0, len(h.table))
	for _, value := range h.table {
		values = append(values, value)
	}
	return values
}

func (h *Hash) Clear() {
	if h.Len() > 256 {
		h.table = make(map[string][]byte, 16)
	} else {
		for k := range h.table {
			delete(h.table, k)
		}
	}
	h.size = 0
}

func (h *Hash) IsEmpty() bool {
	return h.size == 0
}

func (h *Hash) Exist(key string) bool {
	_, ok := h.table[key]
	return ok
}

func (h *Hash) StrLen(key string) int {
	return len(h.table[key])
}

func (h *Hash) Random(count int) []string {
	size := h.Len()
	if count == 0 || size == 0 {
		return []string{}
	}

	if count > 0 {
		if count > size {
			count = size
		}
		res := make([]string, 0, count) // 预分配容量
		for key := range h.table {
			res = append(res, key)
			if len(res) == count {
				break
			}
		}
		return res
	} else {
		// 负数表示可重复选择
		res := make([]string, 0, -count)
		for len(res) < -count {
			for key := range h.table {
				res = append(res, key)
				if len(res) == -count {
					return res
				}
			}
		}
		return res
	}
}

func (h *Hash) RandomWithValue(count int) [][]byte {
	size := h.Len()
	if count == 0 || size == 0 {
		return [][]byte{}
	}

	if count > 0 {
		if count > size {
			count = size
		}
		res := make([][]byte, 0, count*2) // 预分配容量
		for key, val := range h.table {
			res = append(res, []byte(key), val)
			if len(res) >= count*2 {
				break
			}
		}
		return res
	} else {
		res := make([][]byte, 0, -count*2)
		for len(res) < -count*2 {
			for key, val := range h.table {
				res = append(res, []byte(key), val)
				if len(res) >= -count*2 {
					return res
				}
			}
		}
		return res
	}
}

func (h *Hash) Table() map[string][]byte {
	return h.table
}

func (h *Hash) IncrBy(key string, incr int) (int, bool) {
	currentVal := h.table[key]
	if len(currentVal) == 0 {
		newVal := strconv.Itoa(incr)
		h.table[key] = []byte(newVal)
		h.size++
		return incr, true
	}

	value, err := strconv.Atoi(string(currentVal))
	if err != nil {
		return 0, false
	}
	value += incr
	h.table[key] = []byte(strconv.Itoa(value))
	return value, true
}

func (h *Hash) IncrByFloat(key string, incr float64) (float64, bool) {
	currentVal := h.table[key]
	if len(currentVal) == 0 {
		newVal := strconv.FormatFloat(incr, 'f', -1, 64)
		h.table[key] = []byte(newVal)
		h.size++
		return incr, true
	}

	value, err := strconv.ParseFloat(string(currentVal), 64)
	if err != nil {
		return 0, false
	}
	value += incr
	h.table[key] = []byte(strconv.FormatFloat(value, 'f', -1, 64))
	return value, true
}
