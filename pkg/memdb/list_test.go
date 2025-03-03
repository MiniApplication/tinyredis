package memdb

import (
	"bytes"
	"testing"

	RESP2 "github.com/hsn/tiny-redis/pkg/RESP"
	"github.com/hsn/tiny-redis/pkg/config"
)

func init() {
	config.Configures = &config.Config{
		ShardNum: 100,
	}
}

func TestLPosList(t *testing.T) {
	m := NewMemDb()
	lPushList(m, [][]byte{[]byte("lpush"), []byte("l1"), []byte("d"), []byte("b"), []byte("a"), []byte("c"), []byte("b"), []byte("a")})

	var res RESP2.RedisData
	//    test normal pos
	res = lPosList(m, [][]byte{[]byte("lpos"), []byte("l1"), []byte("a")})
	if !bytes.Equal(res.ToBytes(), RESP2.MakeIntData(0).ToBytes()) {
		t.Error("normal lpos error")
	}
	res = lPosList(m, [][]byte{[]byte("lpos"), []byte("l1"), []byte("d")})
	if !bytes.Equal(res.ToBytes(), RESP2.MakeIntData(5).ToBytes()) {
		t.Error("normal lpos error")
	}

	// test rank option
	res = lPosList(m, [][]byte{[]byte("lpos"), []byte("l1"), []byte("a"), []byte("rank"), []byte("2")})
	if !bytes.Equal(res.ToBytes(), RESP2.MakeIntData(3).ToBytes()) {
		t.Error("positive rank lpos error")
	}
	res = lPosList(m, [][]byte{[]byte("lpos"), []byte("l1"), []byte("b"), []byte("rank"), []byte("-2")})
	if !bytes.Equal(res.ToBytes(), RESP2.MakeIntData(1).ToBytes()) {
		t.Error("negative rank lpos error")
	}

	//     test count option
	res = lPosList(m, [][]byte{[]byte("lpos"), []byte("l1"), []byte("a"), []byte("count"), []byte("2")})
	if !bytes.Equal(res.ToBytes(), RESP2.MakeArrayData([]RESP2.RedisData{RESP2.MakeIntData(0), RESP2.MakeIntData(3)}).ToBytes()) {
		t.Error("count lpos error")
	}
	res = lPosList(m, [][]byte{[]byte("lpos"), []byte("l1"), []byte("c"), []byte("count"), []byte("1")})
	if !bytes.Equal(res.ToBytes(), RESP2.MakeArrayData([]RESP2.RedisData{RESP2.MakeIntData(2)}).ToBytes()) {
		t.Error("count lpos error")
	}
	res = lPosList(m, [][]byte{[]byte("lpos"), []byte("l1"), []byte("b"), []byte("count"), []byte("1"), []byte("rank"), []byte("-1")})
	if !bytes.Equal(res.ToBytes(), RESP2.MakeArrayData([]RESP2.RedisData{RESP2.MakeIntData(4)}).ToBytes()) {
		t.Error("count lpos error")
	}

	//    test maxlen option
	res = lPosList(m, [][]byte{[]byte("lpos"), []byte("l1"), []byte("a"), []byte("maxlen"), []byte("2"), []byte("count"), []byte("0")})
	if !bytes.Equal(res.ToBytes(), RESP2.MakeArrayData([]RESP2.RedisData{RESP2.MakeIntData(0)}).ToBytes()) {
		t.Error("maxlen lpos error")
	}
	res = lPosList(m, [][]byte{[]byte("lpos"), []byte("l1"), []byte("d"), []byte("maxlen"), []byte("3")})
	if !bytes.Equal(res.ToBytes(), RESP2.MakeBulkData(nil).ToBytes()) {
		t.Error("maxlen lpos error")
	}
}

func TestLRemList(t *testing.T) {
	m := NewMemDb()
	rPushList(m, [][]byte{[]byte("rpush"), []byte("l1"), []byte("0"), []byte("1"), []byte("1"), []byte("1"), []byte("2"), []byte("2")})
	var res RESP2.RedisData
	res = lRemList(m, [][]byte{[]byte("lrem"), []byte("l1"), []byte("0"), []byte("0")})
	if !bytes.Equal(res.ToBytes(), RESP2.MakeIntData(1).ToBytes()) {
		t.Error("lrem error")
	}
	res = lRemList(m, [][]byte{[]byte("lrem"), []byte("l1"), []byte("2"), []byte("1")})
	if !bytes.Equal(res.ToBytes(), RESP2.MakeIntData(2).ToBytes()) {
		t.Error("lrem error")
	}
	res = lRemList(m, [][]byte{[]byte("lrem"), []byte("l1"), []byte("0"), []byte("2")})
	if !bytes.Equal(res.ToBytes(), RESP2.MakeIntData(2).ToBytes()) {
		t.Error("lrem error")
	}
}
