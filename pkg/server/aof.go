package server

import (
	"io"
	"os"
	"strings"

	"github.com/hsn/tiny-redis/pkg/RESP"
	"github.com/hsn/tiny-redis/pkg/logger"
)

const aofPath = "aof"
const aofFileSize = 10 << 10

func (h *Handler) aofLogger(aofPath string) {
	f, err := os.OpenFile(aofPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logger.Panic("Failed to open AOF file: ", err)
	}
	defer f.Close()

	for {
		select {
		case cmdBytes := <-h.aofChan:
			_, err := f.Write(cmdBytes)
			if err != nil {
				logger.Error("Failed to write to AOF file: ", err)
			}
		case <-h.stopCh:
			logger.Info("AOF logger shutting down")
			return
		}
	}
}
func (h *Handler) StartAOF(aofPath string) {
	go h.aofLogger(aofPath)
}
func (h *Handler) Stop() {
	close(h.stopCh)
}

func (h *Handler) loadAOF(aofPath string) {
	f, err := os.Open(aofPath)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Info("AOF file does not exist, starting with an empty database")
			return
		}
		return
	}
	defer f.Close()

	logger.Info("Starting data recovery from AOF file")

	ch := RESP.ParseStream(f)

	for parsedRes := range ch {
		if parsedRes.Err != nil {
			if parsedRes.Err == io.EOF {
				logger.Info("Finished reading the AOF file")
				break
			} else {
				logger.Error("Error parsing the AOF file: ", parsedRes.Err)
				continue
			}
		}

		if parsedRes.Data == nil {
			logger.Error("Empty command in AOF file")
			continue
		}

		arrayData, ok := parsedRes.Data.(*RESP.ArrayData)
		if !ok {
			logger.Error("Invalid command format in AOF file")
			continue
		}

		cmd := arrayData.ToCommand()
		h.memDb.ExecCommand(cmd)
	}

	logger.Info("AOF data recovery complete")
}
func IsWriteCommand(cmd [][]byte) bool {
	if len(cmd) == 0 {
		return false
	}

	switch strings.ToUpper(string(cmd[0])) {
	// String commands
	case "SET", "SETNX", "SETEX", "PSETEX", "DEL", "INCR", "DECR", "APPEND", "MSET", "MSETNX":
		return true

	// Hash commands
	case "HSET", "HSETNX", "HDEL", "HINCRBY", "HINCRBYFLOAT", "HMSET":
		return true

	// List commands
	case "LPUSH", "RPUSH", "LPOP", "RPOP", "LREM", "LSET", "LTRIM":
		return true

	// Set commands
	case "SADD", "SREM", "SPOP", "SMOVE":
		return true

	// Sorted Set commands
	case "ZADD", "ZREM", "ZINCRBY", "ZPOPMAX", "ZPOPMIN":
		return true

	// Generic commands
	case "EXPIRE", "EXPIREAT", "PERSIST", "RENAME", "RENAMENX", "FLUSHDB", "FLUSHALL":
		return true

	// Transactional commands (since they modify state in the context of a transaction)
	case "MULTI", "EXEC", "DISCARD", "WATCH", "UNWATCH":
		return true

	default:
		return false
	}
}
