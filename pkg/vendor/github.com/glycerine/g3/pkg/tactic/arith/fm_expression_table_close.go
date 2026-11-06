package arith

// Source: src/tactic/arith/fm_tactic.cpp:807-846 (owned ast_ref vectors).
import "github.com/glycerine/g3/pkg/ast"

func (t *FMExpressionTable) Close() error {
	if t == nil {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	for i := range t.terms {
		if t.terms[i].IsNull() {
			continue
		}
		consumed, err := t.manager.DecRefConsumed(t.terms[i])
		if consumed {
			t.terms[i] = ast.ASTHandle{}
		}
		if err != nil {
			return err
		}
	}
	for i := 1; i < len(t.booleans); i++ {
		if t.booleans[i].IsNull() {
			continue
		}
		consumed, err := t.manager.DecRefConsumed(t.booleans[i])
		if consumed {
			t.booleans[i] = ast.ASTHandle{}
		}
		if err != nil {
			return err
		}
	}
	t.closed = true
	t.terms = nil
	t.booleans = nil
	return nil
}
