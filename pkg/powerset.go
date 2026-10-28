package apalache

import "fmt"

func (rewriter *StratifiedRewriter) rewritePowerSet(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 1 {
		return scope, ArenaCell{}, fmt.Errorf("powerset requires one operand")
	}
	scope, domain, err := rewriter.Rewrite(arguments[0], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	if _, setTyped := domain.Type.TLAType().(SetType); !setTyped {
		return scope, ArenaCell{}, fmt.Errorf("powerset requires a set operand, found %s", domain.Type)
	}
	powerType, err := NewPowerSetCellType(domain.Type.TLAType())
	if err != nil {
		return scope, ArenaCell{}, err
	}
	if tagged, ok := expression.Tag().Type(); !ok || !EqualTypes(powerType.TLAType(), tagged) {
		return scope, ArenaCell{}, fmt.Errorf("powerset result type %s does not match expression type", powerType.TLAType())
	}
	newArena, result := scope.Arena.AppendCellType(powerType)
	scope.Arena = newArena.SetDomain(result, domain)
	return scope, result, nil
}
