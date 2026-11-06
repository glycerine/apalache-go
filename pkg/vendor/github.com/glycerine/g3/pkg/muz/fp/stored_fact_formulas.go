package fp

import (
	"github.com/glycerine/g3/pkg/ast"
)

// StoredFactFormulas returns owning ground applications for tuples currently
// held by concrete Datalog relation tables. Public source rule enumeration
// deliberately omits these; the explicit Go replay exporter uses them.
func (e *PropositionalEngine) StoredFactFormulas() (result []ast.ASTRef, resultErr error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	attempt := &storedFactFormulaAttempt{manager: e.manager}
	defer func() {
		if resultErr != nil {
			resultErr = attempt.fail(resultErr)
			result = nil
		}
	}()
	for _, declaration := range e.predicateOrder {
		state := e.predicates[declaration]
		if state == nil || state.table == nil {
			continue
		}
		facts, err := state.table.Facts()
		if err != nil {
			return nil, err
		}
		for _, fact := range facts {
			values, err := e.relations.DecodeFact(state.sorts, fact)
			attempt.current = values
			if err != nil {
				return nil, err
			}
			handles := make([]ast.ASTHandle, len(values))
			for i := range values {
				handles[i], err = e.manager.RefGet(values[i])
				if err != nil {
					break
				}
			}
			var application ast.ASTHandle
			if err == nil {
				application, err = e.manager.MkApp(declaration, handles)
			}
			var reference ast.ASTRef
			if err == nil {
				reference, err = e.manager.NewRef(application)
			}
			if reference != (ast.ASTRef{}) {
				attempt.outputs = append(attempt.outputs, reference)
			}
			if err == nil {
				err = attempt.clearCurrent()
			}
			if err != nil {
				return nil, err
			}
		}
	}
	result = attempt.outputs
	attempt.outputs = nil
	return result, nil
}
