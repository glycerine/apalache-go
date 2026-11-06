package numeral

// Source: src/util/mpn.cpp:371-399; src/util/buffer.h:84-95.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// Each invocation borrows reusable, concrete Go scratch slices. Numeral cells
// remain owned by their managers; these temporary words need no Runtime blocks.
type naturalFormattingScope struct {
	invocation                  uint64
	Locals                      [3][]uint32
	CompletedField              uint8
	Active, PendingField        bool
	originalCause, cleanupCause error
}

func (m *NaturalManager) beginFormattingLocals(n uint32) error {
	return m.beginFormattingScope(&m.Formatting, n)
}
func (m *NaturalManager) beginFormattingScope(s *naturalFormattingScope, n uint32) error {
	return m.beginFormattingScopeTracked(s, n, nil)
}
func (m *NaturalManager) beginFormattingScopeTracked(s *naturalFormattingScope, n uint32, receipt *NumeralInsertionReceipt) error {
	if e := m.check(); e != nil {
		return e
	}
	if s.Active || s.PendingField {
		return arena.ErrInvalidSlot
	}
	if s.invocation == math.MaxUint64 || n == math.MaxUint32 {
		return arena.ErrBounds
	}
	*s = naturalFormattingScope{Locals: s.Locals, Active: true, invocation: s.invocation + 1}
	if receipt != nil {
		receipt.formatting = s
		receipt.formattingOwned = true
		receipt.formattingInvocation = s.invocation
	}
	for i, size := range [3]uint32{n, n + 1, 1} {
		if uint64(cap(s.Locals[i])) < uint64(size) {
			s.Locals[i] = make([]uint32, size)
		} else {
			s.Locals[i] = s.Locals[i][:size]
			clear(s.Locals[i])
		}
		s.CompletedField |= 1 << uint(i)
	}
	return nil
}
func (m *NaturalManager) unwindFormattingLocals() error {
	return m.unwindFormattingScope(&m.Formatting)
}
func (m *NaturalManager) unwindFormattingScope(s *naturalFormattingScope) error {
	for i := range s.Locals {
		s.Locals[i] = s.Locals[i][:0]
	}
	s.CompletedField = 0
	return nil
}
func (m *NaturalManager) finishFormattingLocals() error {
	return m.finishFormattingScope(&m.Formatting)
}
func (m *NaturalManager) finishFormattingScope(s *naturalFormattingScope) error {
	if e := m.unwindFormattingScope(s); e != nil {
		return e
	}
	*s = naturalFormattingScope{Locals: s.Locals, invocation: s.invocation}
	return nil
}

// CleanupFormatting drops cached scratch after ending the active invocation.
func (m *NaturalManager) CleanupFormatting() error {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	if m.ClosedField || m.runtime == nil {
		return arena.ErrClosed
	}
	return m.cleanupFormattingScope(&m.Formatting)
}
func (m *NaturalManager) cleanupFormattingScope(s *naturalFormattingScope) error {
	if s.Active {
		return arena.ErrInvalidSlot
	}
	*s = naturalFormattingScope{invocation: s.invocation}
	return nil
}

// The source decrements size_t j before reversing. At zero the first
// dereference endangers the caller's buffer while all local objects live.
func formattingReversalGuard(digits uint32) error {
	if digits == 0 {
		return arena.ErrBounds
	}
	return nil
}
