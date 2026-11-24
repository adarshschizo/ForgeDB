package transaction

import (
	"fmt"
	"sync"
)

type LockMode uint8

const (
	Shared LockMode = iota
	Exclusive
)

type LockManager struct {
	mu    sync.Mutex
	locks map[string]map[uint64]LockMode
}

func NewLockManager() *LockManager {
	return &LockManager{locks: make(map[string]map[uint64]LockMode)}
}

func (l *LockManager) Acquire(resource string, txnID uint64, mode LockMode) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	owners := l.locks[resource]
	if owners == nil {
		owners = make(map[uint64]LockMode)
		l.locks[resource] = owners
	}
	if current, ok := owners[txnID]; ok {
		if current == Exclusive || mode == Shared {
			owners[txnID] = current
			return nil
		}
		owners[txnID] = Exclusive
		return nil
	}
	if mode == Exclusive {
		for owner, ownerMode := range owners {
			if owner != txnID {
				_ = ownerMode
				return fmt.Errorf("exclusive lock conflict on %q", resource)
			}
		}
	}
	if mode == Shared {
		if ownerMode, ok := owners[0]; ok && ownerMode == Exclusive {
			return fmt.Errorf("shared lock conflict on %q", resource)
		}
	}
	for _, ownerMode := range owners {
		if ownerMode == Exclusive {
			return fmt.Errorf("shared lock conflict on %q", resource)
		}
	}
	owners[txnID] = mode
	return nil
}

func (l *LockManager) Release(txnID uint64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for resource, owners := range l.locks {
		delete(owners, txnID)
		if len(owners) == 0 {
			delete(l.locks, resource)
		}
	}
}
