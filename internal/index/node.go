package index

import (
	"encoding/binary"
	"fmt"

	"forgedb/internal/storage"
)

const MaxKeys = 64

type Node struct {
	ID       uint32
	Leaf     bool
	Keys     []int64
	Values   []uint32
	Children []uint32
	Next     uint32
}

func (n *Node) Encode() ([]byte, error) {
	if len(n.Keys) > MaxKeys {
		return nil, fmt.Errorf("node %d exceeds key capacity", n.ID)
	}
	buf := make([]byte, 16)
	binary.LittleEndian.PutUint32(buf[0:4], n.ID)
	if n.Leaf {
		buf[4] = 1
	} else {
		buf[4] = 0
	}
	binary.LittleEndian.PutUint16(buf[6:8], uint16(len(n.Keys)))
	binary.LittleEndian.PutUint32(buf[12:16], n.Next)
	for _, key := range n.Keys {
		raw := make([]byte, 8)
		binary.LittleEndian.PutUint64(raw, uint64(key))
		buf = append(buf, raw...)
	}
	if n.Leaf {
		for _, value := range n.Values {
			raw := make([]byte, 4)
			binary.LittleEndian.PutUint32(raw, value)
			buf = append(buf, raw...)
		}
	} else {
		for _, child := range n.Children {
			raw := make([]byte, 4)
			binary.LittleEndian.PutUint32(raw, child)
			buf = append(buf, raw...)
		}
	}
	if len(buf) > storage.PageSize-storage.PageHeaderSize {
		return nil, fmt.Errorf("node %d payload too large", n.ID)
	}
	return buf, nil
}

func DecodeNode(id uint32, data []byte) (*Node, error) {
	if len(data) < 16 {
		return nil, fmt.Errorf("invalid B+ tree node")
	}
	nodeID := binary.LittleEndian.Uint32(data[0:4])
	if nodeID != 0 && nodeID != id {
		return nil, fmt.Errorf("node id mismatch")
	}
	n := &Node{ID: id, Leaf: data[4] == 1, Next: binary.LittleEndian.Uint32(data[12:16])}
	count := int(binary.LittleEndian.Uint16(data[6:8]))
	pos := 16
	n.Keys = make([]int64, 0, count)
	for i := 0; i < count; i++ {
		if pos+8 > len(data) {
			return nil, fmt.Errorf("truncated B+ tree keys")
		}
		n.Keys = append(n.Keys, int64(binary.LittleEndian.Uint64(data[pos:pos+8])))
		pos += 8
	}
	if n.Leaf {
		n.Values = make([]uint32, 0, count)
		for i := 0; i < count; i++ {
			if pos+4 > len(data) {
				return nil, fmt.Errorf("truncated B+ tree values")
			}
			n.Values = append(n.Values, binary.LittleEndian.Uint32(data[pos:pos+4]))
			pos += 4
		}
	} else {
		n.Children = make([]uint32, 0, count+1)
		for i := 0; i < count+1; i++ {
			if pos+4 > len(data) {
				return nil, fmt.Errorf("truncated B+ tree children")
			}
			n.Children = append(n.Children, binary.LittleEndian.Uint32(data[pos:pos+4]))
			pos += 4
		}
	}
	return n, nil
}
