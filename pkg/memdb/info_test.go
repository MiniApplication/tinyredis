package memdb

import (
	"strings"
	"testing"

	"github.com/hsn0918/tinyredis/pkg/RESP"
)

func TestInfoReplicationUsesFetcher(t *testing.T) {
	db := NewMemDb()
	defer stopTTLManager(db)

	db.SetReplicationInfoFetcher(func() RESP.RedisData {
		payload := strings.Join([]string{
			"# Replication",
			"role:master",
			"node_id:node-1",
			"leader_id:node-1",
			"leader_raft_addr:127.0.0.1:7000",
			"known_peers:2",
			"peer0:id=node-1,raft_addr=127.0.0.1:7000,suffrage=Voter,self=true",
			"peer1:id=node-2,raft_addr=127.0.0.1:7001,suffrage=Voter,self=false",
			"",
		}, "\n")
		return RESP.MakeBulkData([]byte(payload))
	})

	data := info(db, [][]byte{[]byte("info"), []byte("replication")})
	bulk, ok := data.(*RESP.BulkData)
	if !ok {
		t.Fatalf("expected BulkData, got %T", data)
	}
	payload := string(bulk.ByteData())

	assertContains(t, payload, "role:master")
	assertContains(t, payload, "node_id:node-1")
	assertContains(t, payload, "leader_id:node-1")
	assertContains(t, payload, "peer0:id=node-1")
	assertContains(t, payload, "peer1:id=node-2")
}

func TestInfoReplicationHandlesMissingFetcher(t *testing.T) {
	db := NewMemDb()
	defer stopTTLManager(db)

	data := info(db, [][]byte{[]byte("info"), []byte("replication")})
	bulk, ok := data.(*RESP.BulkData)
	if !ok {
		t.Fatalf("expected BulkData, got %T", data)
	}
	payload := string(bulk.ByteData())

	assertContains(t, payload, "role:unknown")
	assertContains(t, payload, "replication_metadata_unavailable")
}

func TestInfoReplicationFetcherWithoutHeader(t *testing.T) {
	db := NewMemDb()
	defer stopTTLManager(db)

	db.SetReplicationInfoFetcher(func() RESP.RedisData {
		return RESP.MakeBulkData([]byte("role:slave\nleader_id:node-1\n"))
	})

	data := info(db, [][]byte{[]byte("info"), []byte("replication")})
	bulk, ok := data.(*RESP.BulkData)
	if !ok {
		t.Fatalf("expected BulkData, got %T", data)
	}
	payload := string(bulk.ByteData())

	assertContains(t, payload, "# Replication")
	assertContains(t, payload, "role:slave")
	assertContains(t, payload, "leader_id:node-1")
}

func assertContains(t *testing.T, payload, substr string) {
	t.Helper()
	if !strings.Contains(payload, substr) {
		t.Fatalf("expected %q to contain %q", payload, substr)
	}
}

func stopTTLManager(db *MemDb) {
	if db == nil || db.ttl == nil {
		return
	}
	go func() {
		db.ttl.stopChan <- struct{}{}
	}()
}
