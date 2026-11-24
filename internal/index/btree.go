package index

import (
	"sort"

	"forgedb/internal/storage"
)

type BPlusTree struct {
	pager *storage.Pager
	Root  uint32
}

func Create(pager *storage.Pager) (*BPlusTree, error) {
	page, err := pager.Allocate(storage.PageTypeIndex)
	if err != nil {
		return nil, err
	}
	node := &Node{ID: page.ID, Leaf: true, Keys: []int64{}, Values: []uint32{}, Next: 0}
	if err := writeNode(pager, node); err != nil {
		return nil, err
	}
	return &BPlusTree{pager: pager, Root: page.ID}, nil
}

func Open(pager *storage.Pager, root uint32) *BPlusTree {
	return &BPlusTree{pager: pager, Root: root}
}

func writeNode(pager *storage.Pager, node *Node) error {
	payload, err := node.Encode()
	if err != nil {
		return err
	}
	page, err := pager.Read(node.ID)
	if err != nil {
		return err
	}
	page.Data = [storage.PageSize - storage.PageHeaderSize]byte{}
	copy(page.Data[:], payload)
	return pager.Write(page)
}

func (t *BPlusTree) readNode(id uint32) (*Node, error) {
	page, err := t.pager.Read(id)
	if err != nil {
		return nil, err
	}
	return DecodeNode(id, page.Data[:])
}

func (t *BPlusTree) Search(key int64) (uint32, bool, error) {
	node, err := t.readNode(t.Root)
	if err != nil {
		return 0, false, err
	}
	for !node.Leaf {
		i := sort.Search(len(node.Keys), func(i int) bool { return key < node.Keys[i] })
		node, err = t.readNode(node.Children[i])
		if err != nil {
			return 0, false, err
		}
	}
	i := sort.Search(len(node.Keys), func(i int) bool { return node.Keys[i] >= key })
	if i < len(node.Keys) && node.Keys[i] == key {
		return node.Values[i], true, nil
	}
	return 0, false, nil
}

func (t *BPlusTree) Range(min, max int64) ([]uint32, error) {
	node, err := t.readNode(t.Root)
	if err != nil {
		return nil, err
	}
	for !node.Leaf {
		i := sort.Search(len(node.Keys), func(i int) bool { return min < node.Keys[i] })
		node, err = t.readNode(node.Children[i])
		if err != nil {
			return nil, err
		}
	}
	values := []uint32{}
	for node != nil {
		for i, key := range node.Keys {
			if key < min {
				continue
			}
			if key > max {
				return values, nil
			}
			values = append(values, node.Values[i])
		}
		if node.Next == 0 {
			break
		}
		node, err = t.readNode(node.Next)
		if err != nil {
			return nil, err
		}
	}
	return values, nil
}

func (t *BPlusTree) Insert(key int64, value uint32) error {
	promotedKey, promotedChild, split, err := t.insert(t.Root, key, value)
	if err != nil {
		return err
	}
	if !split {
		return nil
	}
	page, err := t.pager.Allocate(storage.PageTypeIndex)
	if err != nil {
		return err
	}
	root := &Node{
		ID:       page.ID,
		Leaf:     false,
		Keys:     []int64{promotedKey},
		Children: []uint32{t.Root, promotedChild},
	}
	if err := writeNode(t.pager, root); err != nil {
		return err
	}
	t.Root = root.ID
	return nil
}

func (t *BPlusTree) insert(id uint32, key int64, value uint32) (int64, uint32, bool, error) {
	node, err := t.readNode(id)
	if err != nil {
		return 0, 0, false, err
	}
	if node.Leaf {
		i := sort.Search(len(node.Keys), func(i int) bool { return node.Keys[i] >= key })
		if i < len(node.Keys) && node.Keys[i] == key {
			node.Values[i] = value
			return 0, 0, false, writeNode(t.pager, node)
		}
		node.Keys = append(node.Keys, 0)
		copy(node.Keys[i+1:], node.Keys[i:])
		node.Keys[i] = key
		node.Values = append(node.Values, 0)
		copy(node.Values[i+1:], node.Values[i:])
		node.Values[i] = value
		if len(node.Keys) <= MaxKeys {
			return 0, 0, false, writeNode(t.pager, node)
		}
		mid := len(node.Keys) / 2
		page, err := t.pager.Allocate(storage.PageTypeIndex)
		if err != nil {
			return 0, 0, false, err
		}
		right := &Node{
			ID:     page.ID,
			Leaf:   true,
			Keys:   append([]int64{}, node.Keys[mid:]...),
			Values: append([]uint32{}, node.Values[mid:]...),
			Next:   node.Next,
		}
		node.Keys = append([]int64{}, node.Keys[:mid]...)
		node.Values = append([]uint32{}, node.Values[:mid]...)
		node.Next = right.ID
		if err := writeNode(t.pager, right); err != nil {
			return 0, 0, false, err
		}
		if err := writeNode(t.pager, node); err != nil {
			return 0, 0, false, err
		}
		return right.Keys[0], right.ID, true, nil
	}
	i := sort.Search(len(node.Keys), func(i int) bool { return key < node.Keys[i] })
	promotedKey, promotedChild, split, err := t.insert(node.Children[i], key, value)
	if err != nil || !split {
		if err == nil {
			err = writeNode(t.pager, node)
		}
		return 0, 0, false, err
	}
	j := sort.Search(len(node.Keys), func(j int) bool { return promotedKey < node.Keys[j] })
	node.Keys = append(node.Keys, 0)
	copy(node.Keys[j+1:], node.Keys[j:])
	node.Keys[j] = promotedKey
	node.Children = append(node.Children, 0)
	copy(node.Children[j+2:], node.Children[j+1:])
	node.Children[j+1] = promotedChild
	if len(node.Keys) <= MaxKeys {
		return 0, 0, false, writeNode(t.pager, node)
	}
	mid := len(node.Keys) / 2
	promoted := node.Keys[mid]
	page, err := t.pager.Allocate(storage.PageTypeIndex)
	if err != nil {
		return 0, 0, false, err
	}
	right := &Node{
		ID:       page.ID,
		Leaf:     false,
		Keys:     append([]int64{}, node.Keys[mid+1:]...),
		Children: append([]uint32{}, node.Children[mid+1:]...),
	}
	node.Keys = append([]int64{}, node.Keys[:mid]...)
	node.Children = append([]uint32{}, node.Children[:mid+1]...)
	if err := writeNode(t.pager, right); err != nil {
		return 0, 0, false, err
	}
	if err := writeNode(t.pager, node); err != nil {
		return 0, 0, false, err
	}
	return promoted, right.ID, true, nil
}

func (t *BPlusTree) Delete(key int64) (bool, error) {
	found := false
	_, err := t.Range(MinInt64, MaxInt64)
	if err != nil {
		return false, err
	}
	entries := make([]struct {
		key   int64
		value uint32
	}, 0)
	node, err := t.readNode(t.Root)
	if err != nil {
		return false, err
	}
	for !node.Leaf {
		node, err = t.readNode(node.Children[0])
		if err != nil {
			return false, err
		}
	}
	for node != nil {
		for i, k := range node.Keys {
			entries = append(entries, struct {
				key   int64
				value uint32
			}{k, node.Values[i]})
		}
		if node.Next == 0 {
			break
		}
		node, err = t.readNode(node.Next)
		if err != nil {
			return false, err
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].key < entries[j].key })
	rootPage, err := t.pager.Allocate(storage.PageTypeIndex)
	if err != nil {
		return false, err
	}
	newRoot := &Node{ID: rootPage.ID, Leaf: true, Keys: []int64{}, Values: []uint32{}, Next: 0}
	if err := writeNode(t.pager, newRoot); err != nil {
		return false, err
	}
	t.Root = newRoot.ID
	for _, entry := range entries {
		if entry.key == key {
			found = true
			continue
		}
		if err := t.Insert(entry.key, entry.value); err != nil {
			return false, err
		}
	}
	return found, nil
}

const (
	MinInt64 = int64(-1 << 63)
	MaxInt64 = int64(1<<63 - 1)
)
