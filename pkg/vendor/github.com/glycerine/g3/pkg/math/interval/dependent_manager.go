package interval

// Source: src/math/interval/dep_intervals.h:151-178.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type DependentIntervalManager struct {
	Numbers      *numeral.RationalManager
	Dependencies *IntervalDependencyManager
}

func NewDependentIntervalManager(numbers *numeral.RationalManager) (*DependentIntervalManager, error) {
	if numbers == nil {
		return nil, arena.ErrInvalidSlot
	}
	return &DependentIntervalManager{Numbers: numbers, Dependencies: &IntervalDependencyManager{}}, nil
}

func (m *DependentIntervalManager) NewInterval() (*DependentRationalInterval, error) {
	if m == nil || m.Numbers == nil || m.Dependencies == nil {
		return nil, arena.ErrInvalidSlot
	}
	value, err := NewRationalInterval(m.Numbers)
	if err != nil {
		return nil, err
	}
	return &DependentRationalInterval{manager: m, Value: value}, nil
}

func (m *DependentIntervalManager) Leaf(value uint32) *IntervalDependency {
	if m == nil || m.Dependencies == nil {
		return nil
	}
	return m.Dependencies.Leaf(value)
}

func (m *DependentIntervalManager) Join(left, right *IntervalDependency) *IntervalDependency {
	if m == nil || m.Dependencies == nil {
		return nil
	}
	return m.Dependencies.Join(left, right)
}
