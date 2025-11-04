package server

import (
	"fmt"
	"strings"

	"github.com/hsn0918/tinyredis/pkg/RESP"
	"github.com/hsn0918/tinyredis/pkg/config"
	"github.com/hsn0918/tinyredis/pkg/memdb"
)

// ExecNode is the minimal interface the connection handler needs.
type ExecNode interface {
	ApplyCommand(cmd [][]byte) ([]byte, error)
	ReadCommand(cmd [][]byte) ([]byte, error)
	Stop() error
}

// localNode executes commands directly against a local MemDb without Raft.
type localNode struct {
	cfg *config.Config
	db  *memdb.MemDb
}

func newLocalNode(cfg *config.Config) *localNode {
	db := memdb.NewMemDb()
	// Provide basic replication/info metadata indicating standalone mode.
	db.SetReplicationInfoFetcher(func() RESP.RedisData {
		b := &strings.Builder{}
		b.WriteString("# Replication\n")
		b.WriteString("role:master\n")
		if cfg != nil && cfg.NodeID != "" {
			b.WriteString(fmt.Sprintf("node_id:%s\n", cfg.NodeID))
		}
		b.WriteString("raft_state:disabled\n")
		return RESP.MakeBulkData([]byte(b.String()))
	})
	return &localNode{cfg: cfg, db: db}
}

func (n *localNode) ApplyCommand(cmd [][]byte) ([]byte, error) {
	res := n.db.ExecCommand(cmd)
	if res == nil {
		return RESP.MakeErrorData("unknown error").ToBytes(), nil
	}
	return res.ToBytes(), nil
}

func (n *localNode) ReadCommand(cmd [][]byte) ([]byte, error) {
	res := n.db.ExecCommand(cmd)
	if res == nil {
		return RESP.MakeErrorData("unknown error").ToBytes(), nil
	}
	return res.ToBytes(), nil
}

func (n *localNode) Stop() error { return nil }
