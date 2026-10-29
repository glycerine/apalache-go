package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:603,612.

import "github.com/glycerine/g3/pkg/util/diagnostic"

func (m *AlgebraicRootCore) assertIsolationScratchEmpty() error {
	d, enabled, e := algebraicAssertionSwitch()
	if e != nil || !enabled {
		return e
	}
	if m.roots.state.size != 0 || m.lowers.state.size != 0 || m.uppers.state.size != 0 {
		return diagnostic.DebugAssertionFailure(d, "src/math/polynomial/algebraic_numbers.cpp", 603, "m_isolate_roots.empty() && m_isolate_lowers.empty() && m_isolate_uppers.empty()")
	}
	return nil
}
func (m *AlgebraicRootCore) assertIsolationBoundsSizes() error {
	d, enabled, e := algebraicAssertionSwitch()
	if e != nil || !enabled {
		return e
	}
	if m.uppers.state.size != m.lowers.state.size {
		return diagnostic.DebugAssertionFailure(d, "src/math/polynomial/algebraic_numbers.cpp", 612, "m_isolate_uppers.size() == m_isolate_lowers.size()")
	}
	return nil
}
