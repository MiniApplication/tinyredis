package memdb

import (
	"bytes"
	"testing"
	"time"

	"github.com/hsn0918/tinyredis/pkg/config"
)

func init() {
	config.Configures = &config.Config{
		ShardNum: 100,
	}
}

func TestSetString(t *testing.T) {
	mem := NewMemDb()

	// test set
	res := setString(mem, [][]byte{[]byte("set"), []byte("a"), []byte("a")})
	if !bytes.Equal(res.ToBytes(), []byte("+OK\r\n")) {
		t.Error("set reply error")
	}
	val, ok := mem.db.Get("a")
	if !ok || !bytes.Equal(val.([]byte), []byte("a")) {
		t.Error("set value error")
	}

	// test opt xx and ex
	res = setString(mem, [][]byte{[]byte("set"), []byte("a"), []byte("b"), []byte("xx"), []byte("ex"), []byte("100")})
	if !bytes.Equal(res.ToBytes(), []byte("+OK\r\n")) {
		t.Error("set reply error")
	}
	val, ok = mem.db.Get("a")
	if !ok || !bytes.Equal(val.([]byte), []byte("b")) {
		t.Error("set value error")
	}

	// 检查 TTL 是否正确设置
	if expireAt, exists := mem.ttl.ExpireAt("a"); !exists ||
		expireAt-time.Now().Unix() > 100 ||
		expireAt-time.Now().Unix() < 99 {
		t.Error("set ttl error")
	}

	// test opt keepttl
	res = setString(mem, [][]byte{[]byte("set"), []byte("a"), []byte("c"), []byte("get"), []byte("keepttl")})
	if !bytes.Equal(res.ToBytes(), []byte("$1\r\nb\r\n")) {
		t.Error("set reply error")
	}

	// 检查 keepttl 是否正确保持了 TTL
	if _, exists := mem.ttl.ExpireAt("a"); !exists {
		t.Error("set keepttl error")
	}
}
