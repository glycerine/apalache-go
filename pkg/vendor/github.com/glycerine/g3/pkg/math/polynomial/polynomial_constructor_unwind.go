package polynomial

// Source: src/math/polynomial/polynomial.cpp:1821-1850,2324-2344;
// src/util/mpzzp.h:97-105; src/util/vector.h.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// Only completed selected members unwind. No imp destructor body executes:
// body-created cells and the body monomial-manager retain remain named roots
// for explicit owning abort. The missing m_upm is not synthesized here.
func (m *MultivariatePolynomialCore) unwindConstructorMembers() error {
	m.constructorUnwinding = true
	m.tmp1 = monomialTemp{}
	if m.constructorUnwindStage == 0 {
		if m.idsInitialized {
			m.idsSourceDestroyed = true
			m.ids.Cleanup(0)
			m.idsInitialized = false
		}
		m.constructorUnwindStage++
	}
	if m.constructorUnwindStage == 1 {
		m.directorySourceDestroyed = m.directoryInitialized
		// Native directory storage has no destructor allocation or release failure.
		if m.directoryInitialized {
			m.polynomials = nil
		}
		m.constructorUnwindStage++
	}
	if m.constructorUnwindStage == 2 {
		if m.modular != nil {
			m.modularSourceDestroyed = true
			if e := m.modular.CleanupConstructedMembers(&m.constructorModularIndex); e != nil {
				return e
			}
		}
		m.constructorUnwindStage++
	}
	m.constructorUnwinding = false
	return nil
}

// After source member destruction, owning abort still has genuine numeral
// fields. It uses the borrowed Integer owner directly, never a destroyed
// modular source object or a repaired ID/directory member.
func (m *MultivariatePolynomialCore) deleteNumeral(a *unumeral.Int) error {
	if m.modularSourceDestroyed {
		return m.integer.Del(a)
	}
	return m.modular.Del(a)
}
