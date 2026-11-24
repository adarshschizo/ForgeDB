package storage

import (
	"encoding/binary"
	"fmt"
)

const PageSize = 4096
const PageHeaderSize = 16

type PageType uint8

const (
	PageTypeFree PageType = iota
	PageTypeMetadata
	PageTypeTable
	PageTypeIndex
)

type Page struct {
	ID       uint32
	Type     PageType
	Flags    uint8
	Checksum uint32
	Next     uint32
	Data     [PageSize - PageHeaderSize]byte
}

func NewPage(id uint32, typ PageType) *Page { return &Page{ID: id, Type: typ} }

func (p *Page) Encode() []byte {
	p.Checksum = Checksum(p.Data[:])
	buf := make([]byte, PageSize)
	binary.LittleEndian.PutUint32(buf[0:4], p.ID)
	buf[4] = byte(p.Type)
	buf[5] = p.Flags
	binary.LittleEndian.PutUint32(buf[8:12], p.Checksum)
	binary.LittleEndian.PutUint32(buf[12:16], p.Next)
	copy(buf[PageHeaderSize:], p.Data[:])
	return buf
}

func (p *Page) Decode(buf []byte) error {
	if len(buf) != PageSize {
		return fmt.Errorf("invalid page size: got %d, want %d", len(buf), PageSize)
	}
	p.ID = binary.LittleEndian.Uint32(buf[0:4])
	p.Type = PageType(buf[4])
	p.Flags = buf[5]
	p.Checksum = binary.LittleEndian.Uint32(buf[8:12])
	p.Next = binary.LittleEndian.Uint32(buf[12:16])
	copy(p.Data[:], buf[PageHeaderSize:])
	if got := Checksum(p.Data[:]); got != p.Checksum {
		return fmt.Errorf("page %d checksum mismatch", p.ID)
	}
	return nil
}

func Checksum(data []byte) uint32 {
	var sum uint32 = 2166136261
	for _, b := range data {
		sum ^= uint32(b)
		sum *= 16777619
	}
	return sum
}
