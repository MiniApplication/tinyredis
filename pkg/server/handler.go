package server

import (
	"io"
	"net"

	"github.com/hsn0918/tinyredis/pkg/RESP"
	"github.com/hsn0918/tinyredis/pkg/cluster"
	"github.com/hsn0918/tinyredis/pkg/logger"
)

type Handler struct {
	node *cluster.Node
}

func NewHandler(node *cluster.Node) *Handler {
	return &Handler{node: node}
}

func (h *Handler) Handle(conn net.Conn) {
	defer func() {
		if err := conn.Close(); err != nil {
			logger.Error(err)
		}
	}()
	ch := RESP.ParseStream(conn)
	for parsedRes := range ch {
		if parsedRes.Err != nil {
			if parsedRes.Err == io.EOF {
				logger.Info("Close connection", conn.RemoteAddr().String())
			} else {
				logger.Panic("Handle connection", conn.RemoteAddr().String(), "panic: ", parsedRes.Err.Error())
			}
			return
		}
		if parsedRes.Data == nil {
			logger.Error("empty parsedRes.Data from ", conn.RemoteAddr().String())
			continue
		}
		arrayData, ok := parsedRes.Data.(*RESP.ArrayData)
		if !ok {
			logger.Error("parsedRes.Data is not ArrayData from ", conn.RemoteAddr().String())
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
			errData := RESP.MakeErrorData(err.Error())
			resp = errData.ToBytes()
		}
		if _, err := conn.Write(resp); err != nil {
			logger.Error("writer response to ", conn.RemoteAddr().String(), " error: ", err.Error())
		}
	}
}
