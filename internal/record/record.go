package record

import (
	"encoding/binary"
	"fmt"
	"math"
)

type Kind uint8

const (
	Null Kind = iota
	Integer
	Float
	Boolean
	Text
)

type Value struct {
	Kind Kind    `json:"kind"`
	I    int64   `json:"i,omitempty"`
	F    float64 `json:"f,omitempty"`
	B    bool    `json:"b,omitempty"`
	S    string  `json:"s,omitempty"`
}

func (v Value) String() string {
	switch v.Kind {
	case Null:
		return "NULL"
	case Integer:
		return fmt.Sprintf("%d", v.I)
	case Float:
		return fmt.Sprintf("%g", v.F)
	case Boolean:
		if v.B {
			return "true"
		}
		return "false"
	case Text:
		return v.S
	default:
		return ""
	}
}

type Row struct {
	Values []Value `json:"values"`
}

func EncodeRow(row Row) ([]byte, error) {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(len(row.Values)))
	for _, v := range row.Values {
		buf = append(buf, byte(v.Kind))
		switch v.Kind {
		case Null:
		case Integer:
			raw := make([]byte, 8)
			binary.LittleEndian.PutUint64(raw, uint64(v.I))
			buf = append(buf, raw...)
		case Float:
			raw := make([]byte, 8)
			binary.LittleEndian.PutUint64(raw, math.Float64bits(v.F))
			buf = append(buf, raw...)
		case Boolean:
			if v.B {
				buf = append(buf, 1)
			} else {
				buf = append(buf, 0)
			}
		case Text:
			raw := make([]byte, 4)
			binary.LittleEndian.PutUint32(raw, uint32(len(v.S)))
			buf = append(buf, raw...)
			buf = append(buf, []byte(v.S)...)
		default:
			return nil, fmt.Errorf("unknown value kind %d", v.Kind)
		}
	}
	return buf, nil
}

func DecodeRow(b []byte) (Row, error) {
	if len(b) < 4 {
		return Row{}, fmt.Errorf("invalid row")
	}
	n := binary.LittleEndian.Uint32(b[:4])
	pos := 4
	row := Row{Values: make([]Value, 0, n)}
	for i := uint32(0); i < n; i++ {
		if pos >= len(b) {
			return Row{}, fmt.Errorf("truncated row")
		}
		kind := Kind(b[pos])
		pos++
		value := Value{Kind: kind}
		switch kind {
		case Null:
		case Integer:
			if pos+8 > len(b) {
				return Row{}, fmt.Errorf("truncated integer")
			}
			value.I = int64(binary.LittleEndian.Uint64(b[pos : pos+8]))
			pos += 8
		case Float:
			if pos+8 > len(b) {
				return Row{}, fmt.Errorf("truncated float")
			}
			value.F = math.Float64frombits(binary.LittleEndian.Uint64(b[pos : pos+8]))
			pos += 8
		case Boolean:
			if pos >= len(b) {
				return Row{}, fmt.Errorf("truncated boolean")
			}
			value.B = b[pos] != 0
			pos++
		case Text:
			if pos+4 > len(b) {
				return Row{}, fmt.Errorf("truncated text length")
			}
			size := binary.LittleEndian.Uint32(b[pos : pos+4])
			pos += 4
			if pos+int(size) > len(b) {
				return Row{}, fmt.Errorf("truncated text")
			}
			value.S = string(b[pos : pos+int(size)])
			pos += int(size)
		default:
			return Row{}, fmt.Errorf("unknown value kind %d", kind)
		}
		row.Values = append(row.Values, value)
	}
	return row, nil
}

func EncodeRows(rows []Row) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(len(rows)))
	for _, row := range rows {
		b, _ := EncodeRow(row)
		n := make([]byte, 4)
		binary.LittleEndian.PutUint32(n, uint32(len(b)))
		buf = append(buf, n...)
		buf = append(buf, b...)
	}
	return buf
}

func DecodeRows(buf []byte) ([]Row, error) {
	if len(buf) < 4 {
		return nil, fmt.Errorf("invalid table page")
	}
	n := binary.LittleEndian.Uint32(buf[:4])
	pos := 4
	rows := make([]Row, 0, n)
	for i := uint32(0); i < n; i++ {
		if pos+4 > len(buf) {
			return nil, fmt.Errorf("truncated table page")
		}
		size := binary.LittleEndian.Uint32(buf[pos : pos+4])
		pos += 4
		if pos+int(size) > len(buf) {
			return nil, fmt.Errorf("truncated row")
		}
		row, err := DecodeRow(buf[pos : pos+int(size)])
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
		pos += int(size)
	}
	return rows, nil
}
