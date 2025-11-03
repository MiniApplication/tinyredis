package memdb

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/hsn0918/tinyredis/pkg/RESP"
	"github.com/hsn0918/tinyredis/pkg/logger"
)

func RegisterInfoCommands() {
	RegisterCommand("client", client)
	RegisterCommand("config", infoConfig)
	RegisterCommand("scan", scan)
	RegisterCommand("info", info)
	RegisterCommand("quit", quit)
}

// client
func client(m *MemDb, cmd [][]byte) RESP.RedisData {
	return RESP.MakeBulkData([]byte("OK"))
}

// config
func infoConfig(m *MemDb, cmd [][]byte) RESP.RedisData {
	return RESP.MakeBulkData([]byte("OK"))
}

// scan
func scan(m *MemDb, cmd [][]byte) RESP.RedisData {
	return RESP.MakeNullBulkData()
}

// quit
func quit(m *MemDb, cmd [][]byte) RESP.RedisData {
	return RESP.MakeBulkData([]byte("OK"))
}

// info
func info(m *MemDb, cmd [][]byte) RESP.RedisData {
	if len(cmd) == 0 {
		return RESP.MakeErrorData("error: command args number is invalid")
	}
	if strings.ToLower(string(cmd[0])) != "info" {
		logger.Error("info command invoked with unexpected name: ", string(cmd[0]))
		return RESP.MakeErrorData("server error")
	}
	if len(cmd) > 2 {
		return RESP.MakeErrorData("error: command args number is invalid")
	}

	section := ""
	if len(cmd) == 2 {
		section = strings.ToLower(string(cmd[1]))
	}

	var builder strings.Builder
	switch section {
	case "":
		builder.WriteString(formatServerSection())
		builder.WriteString("\n")
		builder.WriteString(formatReplicationSection(m))
	case "replication":
		builder.WriteString(formatReplicationSection(m))
	default:
		builder.WriteString("# ")
		builder.WriteString(strings.Title(section))
		builder.WriteString("\n")
		builder.WriteString(fmt.Sprintf("warning:section_%s_not_available\n", section))
	}
	return RESP.MakeBulkData([]byte(builder.String()))
}

func formatServerSection() string {
	var builder strings.Builder
	builder.WriteString("# Server\n")
	builder.WriteString("redis_mode:cluster\n")
	builder.WriteString(fmt.Sprintf("os:%s %s\n", runtime.GOOS, runtime.GOARCH))
	builder.WriteString(fmt.Sprintf("go_version:%s\n", runtime.Version()))
	builder.WriteString(fmt.Sprintf("process_id:%d\n", os.Getpid()))
	builder.WriteString(fmt.Sprintf("server_time_unix:%d\n", time.Now().Unix()))
	return builder.String()
}

func formatReplicationSection(m *MemDb) string {
	var builder strings.Builder
	builder.WriteString("# Replication\n")

	if m == nil || m.replicationFetcher == nil {
		builder.WriteString("role:unknown\n")
		builder.WriteString("detail:replication_metadata_unavailable\n")
		return builder.String()
	}

	data := m.replicationFetcher()
	if data == nil {
		builder.WriteString("role:loading\n")
		return builder.String()
	}

	payload := string(data.ByteData())
	if strings.TrimSpace(payload) == "" {
		builder.WriteString("role:loading\n")
		return builder.String()
	}

	trimmed := strings.TrimLeft(payload, "\r\n\t ")
	if strings.HasPrefix(trimmed, "# Replication") {
		return trimmed
	}

	builder.WriteString(trimmed)
	if !strings.HasSuffix(trimmed, "\n") {
		builder.WriteString("\n")
	}
	return builder.String()
}
