package numeral

// Source: src/util/mpq.cpp:156-184 raw decimal-display locals.

import (
	"errors"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

var ErrRawDisplayPending = errors.New("mpnum: raw display local cleanup is pending")

// These are the actual source locals. Their concrete storage is charged with
// the IntegerManager header. Raw mpz locals have no C++ numeral destructor:
// a failed body retains its cells until explicitly requested owning cleanup.
type rawDisplayScope struct {
	Roots             [3]Int
	order             [3]uint8
	count, cursor     uint8
	active            bool
	original, cleanup error
}

func (m *IntegerManager) beginRawDisplay(count uint8, order [3]uint8) error {
	if m.RawDisplay.active {
		return ErrRawDisplayPending
	}
	m.RawDisplay = rawDisplayScope{count: count, order: order, active: true}
	return nil
}

// Only a successful body reaches the explicit source end label. Errors during
// its Del sequence remain observable, with the exact release cursor retained.
func (m *IntegerManager) finishRawDisplay(err *error, length *uint32) {
	s := &m.RawDisplay
	if *err == nil {
		for s.cursor < s.count {
			if _, e := sourceNumeralDel(m, &s.Roots[s.order[s.cursor]]); e != nil {
				s.cleanup = e
				*err = e
				*length = 0
				return
			}
			s.cursor++
		}
		*s = rawDisplayScope{}
		return
	}
	s.original = *err
	for i := range s.Roots {
		if s.Roots[i].Cell != 0 {
			return
		}
	}
	*s = rawDisplayScope{}
}

// CleanupRawDisplay aborts actual retained raw locals. It is owning Go cleanup,
// rather than a claim that the failed C++ function executed its end label.
func (m *IntegerManager) CleanupRawDisplay() error {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.RawDisplay
	if !s.active {
		return nil
	}
	for i := range s.Roots {
		p := &s.Roots[i]
		if p.Cell == 0 {
			continue
		}
		var e error
		if m.SourceFinalized || m.ConstructorUnwound {
			if !m.IsFailureRef(p.Cell) {
				e = m.freeAfterSourceFinalize(p.Cell)
			}
			if e == nil {
				p.Cell = 0
			}
		} else {
			e = m.Del(p)
		}
		if e != nil {
			s.cleanup = e
			return e
		}
	}
	*s = rawDisplayScope{}
	return nil
}
