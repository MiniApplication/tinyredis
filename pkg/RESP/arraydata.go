package RESP

import "strconv"

type ArrayData struct {
	data []RedisData
}

func MakeArrayData(data []RedisData) *ArrayData {
	return &ArrayData{data: data}
}
func MakeEmptyArrayData() *ArrayData {
	return &ArrayData{data: nil}
}
func (a *ArrayData) ToBytes() []byte {
	if a.data == nil {
		return []byte("*-1" + CRLF)
	}
	res := []byte("*" + strconv.Itoa(len(a.data)) + CRLF)
	for _, v := range a.data {
		res = append(res, v.ToBytes()...)
	}
	return res
}
func (a *ArrayData) Data() []RedisData {
	return a.data
}
func (a *ArrayData) ToCommand() [][]byte {
	res := make([][]byte, 0, len(a.data))
	for _, v := range a.data {
		res = append(res, v.ByteData())
	}
	return res
}
func (a *ArrayData) ByteData() []byte {
	if len(a.data) == 0 {
		return []byte{}
	}
	total := 0
	parts := make([][]byte, len(a.data))
	for i, v := range a.data {
		raw := v.ByteData()
		parts[i] = raw
		total += len(raw)
	}
	res := make([]byte, 0, total)
	for _, p := range parts {
		res = append(res, p...)
	}
	return res
}
