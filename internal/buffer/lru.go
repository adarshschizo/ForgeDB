package buffer

import "container/list"

type LRU struct {
	items *list.List
	nodes map[uint32]*list.Element
}

func NewLRU() *LRU {
	return &LRU{items: list.New(), nodes: make(map[uint32]*list.Element)}
}

func (l *LRU) Touch(pageID uint32) {
	if node, ok := l.nodes[pageID]; ok {
		l.items.MoveToFront(node)
	} else {
		l.nodes[pageID] = l.items.PushFront(pageID)
	}
}

func (l *LRU) Remove(pageID uint32) {
	if node, ok := l.nodes[pageID]; ok {
		l.items.Remove(node)
		delete(l.nodes, pageID)
	}
}

func (l *LRU) Victim() (uint32, bool) {
	if l.items.Len() == 0 {
		return 0, false
	}
	return l.items.Back().Value.(uint32), true
}
