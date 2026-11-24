package wal

import (
	"encoding/binary"
	"fmt"
)

type Kind uint8

const (
	RecordBegin Kind = iota + 1
	RecordUpdate
	RecordCommit
	RecordAbort
)

type Record struct {
	LSN      uint64
	TxnID    uint64
	Kind     Kind
	PageID   uint32
	Payload  []byte
	Checksum uint32
}

const HeaderSize = 29

func (r *Record) Encode() []byte {
	buf := make([]byte, HeaderSize+len(r.Payload))
	binary.LittleEndian.PutUint64(buf[0:8], r.LSN)
	binary.LittleEndian.PutUint64(buf[8:16], r.TxnID)
	buf[16] = byte(r.Kind)
	binary.LittleEndian.PutUint32(buf[17:21], r.PageID)
	binary.LittleEndian.PutUint32(buf[21:25], uint32(len(r.Payload)))
	copy(buf[HeaderSize:], r.Payload)
	r.Checksum = checksum(buf[:25], r.Payload)
	binary.LittleEndian.PutUint32(buf[25:29], r.Checksum)
	return buf
}

func Decode(buf []byte) (Record, error) {
	if len(buf) < HeaderSize {
		return Record{}, fmt.Errorf("truncated WAL record")
	}
	r := Record{
		LSN:      binary.LittleEndian.Uint64(buf[0:8]),
		TxnID:    binary.LittleEndian.Uint64(buf[8:16]),
		Kind:     Kind(buf[16]),
		PageID:   binary.LittleEndian.Uint32(buf[17:21]),
		Checksum: binary.LittleEndian.Uint32(buf[25:29]),
	}
	size := binary.LittleEndian.Uint32(buf[21:25])
	if int(size) != len(buf)-HeaderSize {
		return Record{}, fmt.Errorf("invalid WAL payload length")
	}
	r.Payload = append([]byte{}, buf[HeaderSize:]...)
	if got := checksum(buf[:25], r.Payload); got != r.Checksum {
		return Record{}, fmt.Errorf("WAL checksum mismatch")
	}
	return r, nil
}

func checksum(header []byte, payload []byte) uint32 {
	var sum uint32 = 2166136261
	for _, b := range header {
		sum ^= uint32(b)
		sum *= 16777619
	}
	for _, b := range payload {
		sum ^= uint32(b)
		sum *= 16777619
	}
	return sum
}
