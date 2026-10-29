package ast

// Source: src/ast/fpa_decl_plugin.cpp:141-168.

import unumeral "github.com/glycerine/g3/pkg/util/numeral"

// ReadFPARoundingMode recognizes one of the five source rounding constants.
func (m *ASTManager) ReadFPARoundingMode(h ASTHandle) (found bool, mode unumeral.ArbitraryFloatRoundingMode, err error) {
	if err = m.lock(); err != nil {
		return false, 0, err
	}
	defer m.mu.Unlock()
	r, err := m.requireExpr(h)
	if err != nil {
		return false, 0, err
	}
	return m.readFPARoundingMode(r)
}

func (m *ASTManager) readFPARoundingMode(r astNodeRef) (bool, unumeral.ArbitraryFloatRoundingMode, error) {
	if m.floatingPointPlugin == nil || m.floatingPointPlugin.sourceEnded {
		return false, 0, ErrASTHandle
	}
	if m.nodeAt(r.slot).kind != ASTApp {
		return false, 0, nil
	}
	family, kind, err := m.declIdentity(m.appAt(r.slot).decl)
	if err != nil {
		return false, 0, err
	}
	if family != m.floatingPointPlugin.family {
		return false, 0, nil
	}
	switch kind {
	case astFPARMNearestAway:
		return true, unumeral.ArbitraryFloatNearestAway, nil
	case astFPARMNearestEven:
		return true, unumeral.ArbitraryFloatNearestEven, nil
	case astFPARMTowardNegative:
		return true, unumeral.ArbitraryFloatTowardNegative, nil
	case astFPARMTowardPositive:
		return true, unumeral.ArbitraryFloatTowardPositive, nil
	case astFPARMTowardZero:
		return true, unumeral.ArbitraryFloatTowardZero, nil
	}
	return false, 0, nil
}
