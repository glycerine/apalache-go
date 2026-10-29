package ast

// Source: src/ast/fpa_decl_plugin.cpp:873-933.

func (m *ASTManager) floatingPointValue(fid FamilyID, r astNodeRef, unique bool) (bool, error) {
	if m.nodeAt(r.slot).kind != ASTApp {
		return false, nil
	}
	app := m.appAt(r.slot)
	family, kind, e := m.declIdentity(app.decl)
	if e != nil || family != fid {
		return false, e
	}
	switch kind {
	case astFPARMNearestEven, astFPARMNearestAway, astFPARMTowardPositive, astFPARMTowardNegative, astFPARMTowardZero:
		return true, nil
	case astFPANum, astFPAPlusInf, astFPAMinusInf, astFPAPlusZero, astFPAMinusZero, astFPANaN:
		return !unique, nil
	case astFPAFP:
		if app.count != 3 {
			return false, ErrASTDomain
		}
		for i := uint32(0); i < app.count; i++ {
			arg, e := m.readInternal(app.args, i)
			if e != nil {
				return false, e
			}
			var value bool
			if unique {
				value, e = m.isUniqueValue(arg)
			} else {
				value, e = m.isValue(arg)
			}
			if e != nil || !value {
				return false, e
			}
		}
		return true, nil
	default:
		return false, nil
	}
}
