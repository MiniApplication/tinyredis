package server

import (
	"net"
	"strconv"
	"sync"

	"github.com/hsn/tiny-redis/pkg/config"
	"github.com/hsn/tiny-redis/pkg/logger"
)

// Start starts a simple redis server
func Start(cfg *config.Config) error {
	listener, err := net.Listen("tcp", cfg.Host+":"+strconv.Itoa(cfg.Port))
	if err != nil {
		logger.Panic(err)
		return err
	}
	defer func() {
		err := listener.Close()
		if err != nil {
			logger.Error(err)
		}
	}()
	logger.Info("Server Listen at", cfg.Host+":"+strconv.Itoa(cfg.Port))

	var sg sync.WaitGroup
	handler := NewHandler()
	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Error(err.Error())
			break
		}
		logger.Info(conn.RemoteAddr().String(), " connected")
		sg.Add(1)
		go func() {
			defer sg.Done()
			defer func() {
				if r := recover(); r != nil {
					logger.Panic("Recovered from panic in connection handler: ", r)
				}
			}()
			handler.Handle(conn)
		}()

	}
	sg.Wait()
	return nil
}
