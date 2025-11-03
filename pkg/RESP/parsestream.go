package RESP

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/hsn0918/tinyredis/pkg/logger"
)

type ParsedRes struct {
	Data RedisData
	Err  error
}

// readState 用于跟踪 RESP 数据解析的读取状态。
type readState struct {
	bulkLen   int64      // bulkLen 表示当前块数据的长度，用于解析 RESP 块数据。
	arrayLen  int        // arrayLen 表示当前 RESP 数组的元素数量，用于解析 RESP 数组数据。
	multiLine bool       // multiLine 指示是否正在读取多行数据。如果为 true，则表示当前解析的是多行块数据。
	arrayData *ArrayData // arrayData 用于存储 RESP 数组数据的详细信息，它可能包含每个数组元素的类型和值等信息。
	inArray   bool       // inArray 表示当前是否正在解析 RESP 数组内部。
}

func ParseStream(reader io.Reader) <-chan *ParsedRes {
	ch := make(chan *ParsedRes)
	go parse(reader, ch)
	return ch

}

// Reply
// OKReply: +OK\r\n
// ErrorReply: -Error message\r\n
// IntReply: :123456\r\n
// BulkReply: $11\r\nhello world\r\n
// ArrayReply: *3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n
func parse(reader io.Reader, ch chan<- *ParsedRes) {
	bufReader := bufio.NewReader(reader)
	state := new(readState)
	for {
		var res RedisData
		var err error
		var msg []byte
		msg, err = readLine(bufReader, state)
		if err != nil {
			if err == io.EOF {
				ch <- &ParsedRes{Err: err}
				close(ch)
				return
			} else {
				logger.Error(err)
				ch <- &ParsedRes{Err: err}
				*state = readState{}
			}
			continue
		}
		// parse the read messages
		// if msg is an array or a bulk string, then parse their header first.
		// if msg is a normal line, parse it directly.
		if !state.multiLine {
			if msg[0] == '*' {
				err := parseArrayHeader(msg, state)
				if err != nil {
					logger.Error(err)
					ch <- &ParsedRes{
						Err: err,
					}
					*state = readState{}
				} else {
					if state.arrayLen == -1 {
						// null array
						ch <- &ParsedRes{
							Data: MakeArrayData(nil),
						}
						*state = readState{}
					} else if state.arrayLen == 0 {
						// empty array
						ch <- &ParsedRes{
							Data: MakeArrayData([]RedisData{}),
						}
						*state = readState{}
					}
				}
				continue
			}
			if msg[0] == '$' {
				err := parseBulkHeader(msg, state)
				if err != nil {
					logger.Error(err)
					ch <- &ParsedRes{
						Err: err,
					}
					*state = readState{}

				} else {
					if state.bulkLen == -1 {
						state.multiLine = false
						state.bulkLen = 0
						res = MakeNullBulkData()
						if state.inArray {
							state.arrayData.data = append(state.arrayData.data, res)
							if len(state.arrayData.data) == state.arrayLen {
								ch <- &ParsedRes{
									Data: state.arrayData,
									Err:  err,
								}
								*state = readState{}
							}
						} else {
							ch <- &ParsedRes{
								Data: res,
							}
						}

					}
				}
				continue
			}
			res, err = parseSingleLine(msg)
		} else {
			state.multiLine = false
			state.bulkLen = 0
			res, err = parseMultiLine(msg)
		}
		if err != nil {
			logger.Error(err)
			ch <- &ParsedRes{
				Err: err,
			}
			*state = readState{}
			continue
		}
		if state.inArray {
			state.arrayData.data = append(state.arrayData.data, res)
			if len(state.arrayData.data) == state.arrayLen {
				ch <- &ParsedRes{
					Data: state.arrayData,
					Err:  nil,
				}
				*state = readState{}
			}
		} else {
			ch <- &ParsedRes{
				Data: res,
				Err:  err,
			}
		}
	}
}

// Read a line or bulk line end of "\r\n" from a reader.
// Return:
//
//	[]byte: read bytes.
//	error: io.EOF or Protocol error
func readLine(bufReader *bufio.Reader, state *readState) ([]byte, error) {
	var msg []byte
	var err error
	if state.multiLine && state.bulkLen >= 0 {
		// read bulk line
		msg = make([]byte, state.bulkLen+2)
		_, err = io.ReadFull(bufReader, msg)
		if err != nil {
			return nil, err
		}
		state.bulkLen = 0
		if msg[len(msg)-1] != '\n' || msg[len(msg)-2] != '\r' {
			return nil, fmt.Errorf("Protocol error. Stream message %s is invalid.", string(msg))
		}
	} else {
		// read normal line
		msg, err = bufReader.ReadBytes('\n')
		if err != nil {
			return msg, err
		}
		if len(msg) == 0 || msg[len(msg)-2] != '\r' {
			return nil, fmt.Errorf("Protocol error. Stream message %s is invalid.", string(msg))
		}
	}
	return msg, err
}
func parseSingleLine(msg []byte) (RedisData, error) {
	// discard "\r\n"
	msgType := msg[0]
	msgData := string(msg[1 : len(msg)-2])
	var res RedisData

	switch msgType {
	case '+':
		// simple string
		res = MakeStringData(msgData)
	case '-':
		// error
		res = MakeErrorData(msgData)
	case ':':
		//    integer
		data, err := strconv.ParseInt(msgData, 10, 64)
		if err != nil {
			logger.Error("Protocol error: " + string(msg))
			return nil, err
		}
		res = MakeIntData(data)
	default:
		// plain string
		res = MakePlainData(msgData)
	}
	if res == nil {
		logger.Error("Protocol error: parseSingleLine get nil data")
		return nil, errors.New("Protocol error: " + string(msg))
	}
	return res, nil
}
func parseArrayHeader(msg []byte, state *readState) error {
	arrayLen, err := strconv.Atoi(string(msg[1 : len(msg)-2]))
	if err != nil || arrayLen < -1 {
		return errors.New("Protocol error: " + string(msg))
	}
	state.arrayLen = arrayLen
	state.inArray = true
	if arrayLen > 0 {
		state.arrayData = MakeArrayData(make([]RedisData, 0, arrayLen))
	} else {
		// defer allocation until needed for null/empty arrays
		state.arrayData = MakeArrayData([]RedisData{})
	}
	return nil
}
func parseBulkHeader(msg []byte, state *readState) error {
	bulkLen, err := strconv.ParseInt(string(msg[1:len(msg)-2]), 10, 64)
	if err != nil || bulkLen < -1 {
		return errors.New("Protocol error: " + string(msg))
	}
	state.bulkLen = bulkLen
	state.multiLine = true
	return nil
}
func parseMultiLine(msg []byte) (RedisData, error) {
	// discard "\r\n"
	if len(msg) < 2 {
		return nil, errors.New("protocol error: invalid bulk string")
	}
	msgData := msg[:len(msg)-2]
	res := MakeBulkData(msgData)
	return res, nil
}
