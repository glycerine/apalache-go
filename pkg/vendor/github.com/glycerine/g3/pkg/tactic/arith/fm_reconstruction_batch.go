package arith

// Source: src/tactic/arith/fm_tactic.cpp:1520-1549.
import "github.com/glycerine/g3/pkg/ast"

type FMReconstructed struct {
	Expression ast.ASTHandle
	Dependency ast.ASTDependency
}

func (t *FMExpressionTable) Remaining(variables *FMVariableTable) ([]FMReconstructed, error) {
	if variables == nil {
		return nil, ast.ErrASTDomain
	}
	seen := make(map[*FMConstraint]bool)
	result := make([]FMReconstructed, 0)
	collect := func(groups map[uint32][]*FMConstraint) error {
		for x := uint32(0); x < uint32(len(variables.Integer)); x++ {
			for _, c := range groups[x] {
				if c == nil || c.Dead || seen[c] {
					continue
				}
				expression, err := t.ConstraintExpression(c)
				if err != nil {
					return err
				}
				seen[c] = true
				c.Dead = true
				result = append(result, FMReconstructed{Expression: expression, Dependency: c.Dependency})
			}
		}
		return nil
	}
	if err := collect(variables.Index.Upper); err != nil {
		return nil, err
	}
	if err := collect(variables.Index.Lower); err != nil {
		return nil, err
	}
	return result, nil
}
