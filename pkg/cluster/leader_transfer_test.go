package cluster_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/require"

	"github.com/hsn0918/tinyredis/pkg/cluster"
	"github.com/hsn0918/tinyredis/pkg/config"
)

func TestLeaderFailoverTransfersLeadership(t *testing.T) {
	cfg1 := newIntegrationConfig(t, "node-1")
	cfg1.RaftBootstrap = true
	config.Configures = cfg1
	node1, err := cluster.NewNode(cfg1)
	if err != nil {
		if strings.Contains(err.Error(), "operation not permitted") {
			t.Skip("network operations not permitted in test environment")
		}
		require.NoError(t, err)
	}
	t.Cleanup(func() {
		if node1 != nil {
			require.NoError(t, node1.Stop())
		}
	})

	httpAddr1 := waitForHTTPAddr(t, node1)

	cfg2 := newIntegrationConfig(t, "node-2")
	cfg2.JoinAddr = httpAddr1
	config.Configures = cfg2
	node2, err := cluster.NewNode(cfg2)
	if err != nil {
		if strings.Contains(err.Error(), "operation not permitted") {
			t.Skip("network operations not permitted in test environment")
		}
		require.NoError(t, err)
	}
	t.Cleanup(func() {
		if node2 != nil {
			require.NoError(t, node2.Stop())
		}
	})

	cfg3 := newIntegrationConfig(t, "node-3")
	cfg3.JoinAddr = httpAddr1
	config.Configures = cfg3
	node3, err := cluster.NewNode(cfg3)
	if err != nil {
		if strings.Contains(err.Error(), "operation not permitted") {
			t.Skip("network operations not permitted in test environment")
		}
		require.NoError(t, err)
	}
	t.Cleanup(func() {
		if node3 != nil {
			require.NoError(t, node3.Stop())
		}
	})

	nodes := []*cluster.Node{node1, node2, node3}

	leader := waitForLeader(t, nodes...)
	require.NotNil(t, leader)

	resp, err := leader.ApplyCommand([][]byte{
		[]byte("set"),
		[]byte("foo"),
		[]byte("bar"),
	})
	require.NoError(t, err)
	require.Equal(t, "+OK\r\n", string(resp))
	require.NoError(t, leader.Barrier(5*time.Second))

	leaderID := leader.ID()
	require.NoError(t, leader.Stop())

	for i, n := range nodes {
		if n == leader {
			nodes[i] = nil
			break
		}
	}
	pruned := pruneNilNodes(nodes)

	newLeader := waitForLeader(t, pruned...)
	require.NotNil(t, newLeader)
	require.NotEqual(t, leaderID, newLeader.ID(), "leadership should transfer to a different node")

	readResp, err := newLeader.ReadCommand([][]byte{
		[]byte("get"),
		[]byte("foo"),
	})
	require.NoError(t, err)
	require.Equal(t, "$3\r\nbar\r\n", string(readResp))

	// Prevent double-stop in cleanup.
	switch leaderID {
	case "node-1":
		node1 = nil
	case "node-2":
		node2 = nil
	case "node-3":
		node3 = nil
	}
}

func newIntegrationConfig(t *testing.T, nodeID string) *config.Config {
	t.Helper()
	dir := t.TempDir()
	cfg := config.NewDefaultConfig()
	cfg.NodeID = nodeID
	cfg.RaftDir = filepath.Join(dir, "raft")
	cfg.RaftBind = "127.0.0.1:0"
	cfg.RaftHTTPAddr = "127.0.0.1:0"
	cfg.RaftBootstrap = false
	cfg.JoinAddr = ""
	cfg.Host = "127.0.0.1"
	cfg.Port = 0
	return cfg
}

func waitForHTTPAddr(t *testing.T, node *cluster.Node) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if addr := node.HTTPAddr(); addr != "" {
			return addr
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("raft http server did not start")
	return ""
}

func waitForLeader(t *testing.T, nodes ...*cluster.Node) *cluster.Node {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		for _, node := range nodes {
			if node == nil {
				continue
			}
			if node.RaftState() == raft.Leader {
				return node
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("no leader elected")
	return nil
}

func pruneNilNodes(nodes []*cluster.Node) []*cluster.Node {
	out := make([]*cluster.Node, 0, len(nodes))
	for _, n := range nodes {
		if n != nil {
			out = append(out, n)
		}
	}
	return out
}
