package arith

// Source: src/tactic/arith/fm_tactic.cpp:1240-1275.
import "github.com/glycerine/g3/pkg/ast"

func (t *FMExpressionTable) SaveModelBounds(x uint32, variables *FMVariableTable, converter *FMModelConverter) error {
	if converter == nil || variables == nil {
		return ast.ErrASTDomain
	}
	term, err := t.Variable(x)
	if err != nil {
		return err
	}
	declaration, err := t.manager.GetDecl(term)
	if err != nil {
		return err
	}
	constraints := append([]*FMConstraint(nil), variables.Index.Lower[x]...)
	constraints = append(constraints, variables.Index.Upper[x]...)
	clauses := make([]ast.ASTHandle, 0, len(constraints))
	for _, c := range constraints {
		if c == nil || c.Dead {
			continue
		}
		clause, err := t.ConstraintExpression(c)
		if err != nil {
			return err
		}
		clauses = append(clauses, clause)
	}
	return converter.Insert(declaration, clauses)
}
