package server

import (
	"context"
	"errors"
	"net"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"

	"github.com/hsn0918/tinyredis/pkg/cluster"
	"github.com/hsn0918/tinyredis/pkg/config"
	"github.com/hsn0918/tinyredis/pkg/logger"
)

// Start starts a tinyredis node with graceful shutdown support.
func Start(cfg *config.Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Build execution backend: standalone (no Raft) or clustered (Raft).
	var (
		execNode ExecNode
		err      error
	)
	if cfg.Standalone {
		execNode = newLocalNode(cfg)
		logger.Infof("starting in standalone mode (no Raft)")
	} else {
		var raftNode *cluster.Node
		raftNode, err = cluster.NewNode(cfg)
		if err != nil {
			logger.Panic("failed to create raft node: ", err)
			return err
		}
		execNode = raftNode
		defer func() {
			if stopErr := raftNode.Stop(); stopErr != nil {
				logger.Error("failed to stop raft node: ", stopErr)
			}
		}()
	}

	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Panic(err)
		return err
	}
	defer func() {
		if closeErr := listener.Close(); closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
			logger.Error("listener close error: ", closeErr)
		}
	}()

	logger.Infof("server listening on %s", addr)

	var (
		wg      sync.WaitGroup
		handler = NewHandler(execNode)
	)

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				break
			}
			logger.Error("accept connection error: ", err)
			continue
		}

		wg.Add(1)
		go func(c net.Conn) {
			defer wg.Done()
			if err := handler.Handle(ctx, c); err != nil && !errors.Is(err, context.Canceled) {
				logger.Error("connection error: ", err)
			}
		}(conn)
	}

	stop()
	wg.Wait()
	return nil
}
