package sat

import (
	"math"
	"reflect"
	"strings"

	"github.com/glycerine/g3/pkg/arena"
)

type satCutUpdate func(uint32, satCut)

type satCutSet struct {
	self       *satCutSet
	variable   uint32
	cuts       []satCut
	budget     *arena.Budget
	ownerBytes uint64
	cutBytes   uint64
	closed     bool
}

func newSATCutSet(b *arena.Budget) (*satCutSet, error) {
	n := uint64(reflect.TypeOf(satCutSet{}).Size())
	if err := b.Reserve(n); err != nil {
		return nil, err
	}
	s := &satCutSet{variable: math.MaxUint32, budget: b, ownerBytes: n}
	s.self = s
	return s, nil
}

func (s *satCutSet) open() error {
	if s == nil || s.self != s {
		return ErrSATOwner
	}
	if s.closed {
		return arena.ErrClosed
	}
	return nil
}

func (s *satCutSet) Init(_ uint32, variable uint32) error {
	if err := s.open(); err != nil {
		return err
	}
	s.variable = variable
	s.cuts = s.cuts[:0]
	return nil
}

func (s *satCutSet) Var() uint32 { return s.variable }
func (s *satCutSet) Size() uint32 {
	if s == nil || s.closed {
		return 0
	}
	return uint32(len(s.cuts))
}
func (s *satCutSet) At(i uint32) (satCut, bool) {
	if s == nil || s.closed || i >= uint32(len(s.cuts)) {
		return satCut{}, false
	}
	return s.cuts[i], true
}
func (s *satCutSet) Back() (satCut, bool) {
	if s == nil || s.closed || len(s.cuts) == 0 {
		return satCut{}, false
	}
	return s.cuts[len(s.cuts)-1], true
}
func (s *satCutSet) String() string {
	if s == nil || s.closed {
		return ""
	}
	var b strings.Builder
	for _, c := range s.cuts {
		b.WriteString(c.String())
		b.WriteByte('\n')
	}
	return b.String()
}
