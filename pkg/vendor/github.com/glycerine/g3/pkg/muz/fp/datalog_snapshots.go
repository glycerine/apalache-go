package fp

// Source: src/shell/datalog_frontend.cpp:168-242. The frontend saturates once,
// then observes every output relation without launching additional queries.

import (
	"github.com/glycerine/g3/pkg/ast"
)

type DatalogRelationSnapshot struct {
	Declaration ast.ASTHandle
	Facts       [][]uint64
}

// DatalogRelationSnapshots copies saturated finite tables in declaration and
// tuple insertion order. It is valid only after a Datalog query has run.
func (e *PropositionalEngine) DatalogRelationSnapshots(declarations []ast.ASTHandle) ([]DatalogRelationSnapshot, error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	if e.backend != "datalog" || !e.queried {
		return nil, ErrPropositionalEngine
	}
	result := make([]DatalogRelationSnapshot, 0, len(declarations))
	for _, declaration := range declarations {
		state := e.predicates[declaration]
		if state == nil || state.table == nil {
			return nil, ErrPropositionalHorn
		}
		facts, err := state.table.Facts()
		if err != nil {
			return nil, err
		}
		copyFacts := make([][]uint64, len(facts))
		for i, fact := range facts {
			copyFacts[i] = make([]uint64, len(fact))
			for j, value := range fact {
				copyFacts[i][j] = uint64(value)
			}
		}
		result = append(result, DatalogRelationSnapshot{Declaration: declaration, Facts: copyFacts})
	}
	return result, nil
}
