package memdb

import (
	"github.com/hsn0918/tinyredis/pkg/RESP"
)

type cmdExecutor func(m *MemDb, cmd [][]byte) RESP.RedisData

var CmdTable = make(map[string]*command)

type command struct {
	executor cmdExecutor
}

func RegisterCommand(cmdName string, executor cmdExecutor) {
	CmdTable[cmdName] = &command{executor: executor}
}
