package rel

import "github.com/glycerine/g3/pkg/smtcore"

// SetRelationExplanationNumericBacking selects the source numeric relation
// plugin used by relation-level explanation products for the current query.
func (m *RelationManager) SetRelationExplanationNumericBacking(kind string, checker *smtcore.SMTSolverConfig) error {
	if err := m.open(); err != nil {
		return err
	}
	switch kind {
	case "bound_relation", "interval_relation", "karr_relation":
	default:
		return ErrTableRelation
	}
	if checker != nil && checker.Params == nil {
		return ErrTableRelation
	}
	m.explanationNumericKind = kind
	m.explanationNumericCheck = checker
	return nil
}

func (m *RelationManager) clearRelationExplanationNumericBacking() {
	if m == nil {
		return
	}
	m.explanationNumericKind = ""
	m.explanationNumericCheck = nil
}
