package wal

import (
	"encoding/binary"
	"io"
	"os"
)

type Log struct {
	file    *os.File
	nextLSN uint64
}

func Open(path string) (*Log, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}
	log := &Log{file: f}
	records, err := log.ReadAll()
	if err != nil {
		f.Close()
		return nil, err
	}
	for _, record := range records {
		if record.LSN >= log.nextLSN {
			log.nextLSN = record.LSN + 1
		}
	}
	return log, nil
}

func (l *Log) Append(kind Kind, txnID uint64, pageID uint32, payload []byte) (Record, error) {
	r := Record{LSN: l.nextLSN, TxnID: txnID, Kind: kind, PageID: pageID, Payload: payload}
	l.nextLSN++
	if _, err := l.file.Write(r.Encode()); err != nil {
		return Record{}, err
	}
	return r, nil
}

func (l *Log) Sync() error  { return l.file.Sync() }
func (l *Log) Close() error { return l.file.Close() }

func (l *Log) RecordCount() (int, error) {
	records, err := l.ReadAll()
	if err != nil {
		return 0, err
	}
	return len(records), nil
}

func (l *Log) ReadAll() ([]Record, error) {
	offset, err := l.file.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}
	_ = offset
	records := []Record{}
	for {
		header := make([]byte, 25)
		n, err := io.ReadFull(l.file, header)
		if err == io.EOF {
			break
		}
		if err != nil {
			if err == io.ErrUnexpectedEOF {
				break
			}
			return nil, err
		}
		size := binary.LittleEndian.Uint32(header[21:25])
		remaining := make([]byte, HeaderSize-25+int(size))
		n, err = io.ReadFull(l.file, remaining)
		if err != nil {
			break
		}
		_ = n
		buf := append(header, remaining...)
		record, err := Decode(buf)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	_, _ = l.file.Seek(0, io.SeekEnd)
	return records, nil
}
