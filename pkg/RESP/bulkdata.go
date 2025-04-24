package RESP

import "strconv"

type BulkData struct {
	data []byte
}

func MakeBulkData(data []byte) *BulkData {
	return &BulkData{
		data: data,
	}
}

func MakeNullBulkData() *BulkData {
	return &BulkData{data: nil}
}

func (r *BulkData) ToBytes() []byte {
	if r.data == nil {
		return []byte("$-1\r\n")
	}
	return []byte("$" + strconv.Itoa(len(r.data)) + CRLF + string(r.data) + CRLF)
}

func (r *BulkData) Data() []byte {
	return r.data
}

func (r *BulkData) ByteData() []byte {
	return r.data
}
