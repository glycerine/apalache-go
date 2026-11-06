package polynomial

// Source: src/util/id_gen.h:25-85; src/math/polynomial/polynomial.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

// recycleID preserves memory_manager's sticky out-of-memory transport when the
// native Go free-list backing cannot grow on the shared owner budget.
func (m *MultivariatePolynomialCore) recycleID(id uint32) error {
	if e := m.ids.Recycle(id); e != nil {
		if errors.Is(e, arena.ErrBudget) || errors.Is(e, arena.ErrInjectedFailure) {
			return m.memory.RecordAllocationFailure()
		}
		return e
	}
	return nil
}

func (m *MonomialManager) recycleID(id uint32) error {
	if e := m.ids.Recycle(id); e != nil {
		if errors.Is(e, arena.ErrBudget) || errors.Is(e, arena.ErrInjectedFailure) {
			return m.memory.RecordAllocationFailure()
		}
		return e
	}
	return nil
}
