package sat

// Source: src/sat/sat_par.{h,cpp}.

import (
	"sync"

	"github.com/glycerine/g3/pkg/arena"
)

// ParallelUnitExchange is the concrete shared unit store used by source SAT
// workers. The solver borrows it; the creator owns its lifetime.
type ParallelUnitExchange struct {
	self    *ParallelUnitExchange
	mu      sync.Mutex
	units   []Literal
	unitSet map[uint32]struct{}
	closed  bool
}

func NewParallelUnitExchange() *ParallelUnitExchange {
	p := &ParallelUnitExchange{unitSet: make(map[uint32]struct{})}
	p.self = p
	return p
}

func (p *ParallelUnitExchange) openLocked() error {
	if p == nil || p.self != p {
		return ErrSATOwner
	}
	if p.closed {
		return arena.ErrClosed
	}
	return nil
}

func (p *ParallelUnitExchange) Close() error {
	if p == nil || p.self != p {
		return ErrSATOwner
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.units = nil
	p.unitSet = nil
	p.closed = true
	return nil
}
