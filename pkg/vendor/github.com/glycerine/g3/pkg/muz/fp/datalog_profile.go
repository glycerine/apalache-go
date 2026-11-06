package fp

// Source: src/shell/datalog_frontend.cpp:50-111 display_statistics relation,
// rule and generated-instruction observations after saturation.

import "github.com/glycerine/g3/pkg/ast"

type DatalogRelationProfile struct {
	Declaration ast.ASTHandle
	TupleCount  uint64
}

type DatalogProfile struct {
	RuleCount, RelationCount, TupleCount uint64
	Compiled                             bool
	Relations                            []DatalogRelationProfile
}

func (e *PropositionalEngine) DatalogProfile() (DatalogProfile, error) {
	if err := e.open(); err != nil {
		return DatalogProfile{}, err
	}
	if e.backend != "datalog" || !e.queried {
		return DatalogProfile{}, ErrPropositionalEngine
	}
	rules, err := e.set.Len()
	if err != nil {
		return DatalogProfile{}, err
	}
	result := DatalogProfile{RuleCount: uint64(rules), RelationCount: uint64(len(e.predicateOrder)), Compiled: e.compiled, Relations: make([]DatalogRelationProfile, 0, len(e.predicateOrder))}
	for _, declaration := range e.predicateOrder {
		state := e.predicates[declaration]
		if state == nil || state.table == nil {
			return DatalogProfile{}, ErrPropositionalHorn
		}
		count, err := state.table.Len()
		if err != nil {
			return DatalogProfile{}, err
		}
		result.TupleCount += uint64(count)
		result.Relations = append(result.Relations, DatalogRelationProfile{Declaration: declaration, TupleCount: uint64(count)})
	}
	return result, nil
}
