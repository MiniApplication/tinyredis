package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync/atomic"
	"time"

	"github.com/hsn0918/tinyredis/pkg/RESP"
	"github.com/hsn0918/tinyredis/pkg/logger"
)

type Handler struct {
	node       ExecNode
	sessionSeq atomic.Uint64
}

func NewHandler(node ExecNode) *Handler {
	return &Handler{node: node}
}

func (h *Handler) Handle(ctx context.Context, conn net.Conn) error {
	sessionID := h.nextSessionID()
	remote := conn.RemoteAddr().String()
	log := logger.With(
		slog.String("session_id", sessionID),
		slog.String("remote_addr", remote),
	)

	log.Info("connection opened")

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()

	var lastErr error
	defer func() {
		close(done)
		if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			log.Error("connection close error", "err", err)
		}
		if lastErr != nil && !errors.Is(lastErr, context.Canceled) {
			log.Error("connection terminated", "err", lastErr)
		} else {
			log.Info("connection closed")
		}
	}()

	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("connection panic: %v", r)
			log.Error("panic recovered", "err", err)
			lastErr = err
		}
	}()

	cmdStream := RESP.ParseStream(conn)
	for parsed := range cmdStream {
		if parsed.Err != nil {
			if parsed.Err == io.EOF {
				break
			}
			lastErr = parsed.Err
			log.Error("resp parse error", "err", parsed.Err)
			break
		}
		if parsed.Data == nil {
			log.Warn("empty RESP payload")
			continue
		}
		arrayData, ok := parsed.Data.(*RESP.ArrayData)
		if !ok {
			log.Warn("invalid RESP type (expected array)")
			continue
		}

		cmd := arrayData.ToCommand()
		var (
			resp []byte
			err  error
		)

		if IsWriteCommand(cmd) {
			resp, err = h.node.ApplyCommand(cmd)
		} else {
			resp, err = h.node.ReadCommand(cmd)
		}
		if err != nil {
			log.Error("command execution error", "err", err)
			resp = RESP.MakeErrorData(err.Error()).ToBytes()
		}

		if _, err := conn.Write(resp); err != nil {
			if !errors.Is(err, net.ErrClosed) {
				log.Error("write response error", "err", err)
				lastErr = err
			}
			break
		}
	}

	if lastErr == nil && ctx.Err() != nil {
		lastErr = ctx.Err()
	}
	return lastErr
}

func (h *Handler) nextSessionID() string {
	seq := h.sessionSeq.Add(1)
	return fmt.Sprintf("sess-%d-%d", time.Now().UnixMilli(), seq)
}
