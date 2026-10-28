package apalache

import "fmt"

// ConstrainedTransitionExecutor adds disjunctive constraints over paths of a
// particular length. The first expression describes specification parameters;
// as in Scala, only the tail is paired with state bindings.
type ConstrainedTransitionExecutor struct {
	*TransitionExecutor
	pathConstraints map[int][][]Expr
}

func NewConstrainedTransitionExecutor(executor *TransitionExecutor) (*ConstrainedTransitionExecutor, error) {
	if executor == nil {
		return nil, fmt.Errorf("constrained transition executor requires an executor")
	}
	return &ConstrainedTransitionExecutor{TransitionExecutor: executor, pathConstraints: make(map[int][][]Expr)}, nil
}

func (executor *ConstrainedTransitionExecutor) AddPathOrConstraint(constraint []Expr) error {
	if len(constraint) == 0 {
		return fmt.Errorf("path constraint cannot be empty")
	}
	for _, expression := range constraint {
		if err := requireBooleanExpression(expression, "path constraint"); err != nil {
			return err
		}
	}
	copyConstraint := append([]Expr(nil), constraint...)
	executor.pathConstraints[len(copyConstraint)] = append(executor.pathConstraints[len(copyConstraint)], copyConstraint)
	return executor.assertPathOrConstraint(copyConstraint)
}

func (executor *ConstrainedTransitionExecutor) AssumeTransition(number int) error {
	if err := executor.TransitionExecutor.AssumeTransition(number); err != nil {
		return err
	}
	return executor.AssertPathConstraints()
}

func (executor *ConstrainedTransitionExecutor) AssertPathConstraints() error {
	length := len(executor.states) + 1 // parameter head plus committed states
	if executor.picked {
		length++
	}
	for _, constraint := range executor.pathConstraints[length] {
		if err := executor.assertPathOrConstraint(constraint); err != nil {
			return err
		}
	}
	return nil
}

func (executor *ConstrainedTransitionExecutor) assertPathOrConstraint(constraint []Expr) error {
	bindings := append([]Binding(nil), executor.states...)
	if executor.picked {
		bindings = append(bindings, executor.scope.Binding.Shift(executor.constants))
	}
	if len(bindings)+1 < len(constraint) {
		return nil
	}
	disjuncts := make([]Expr, len(constraint)-1)
	for index, expression := range constraint[1:] {
		replacements := bindings[index].Map()
		disjuncts[index] = RewriteExpressions(IdleTracker{}, func(candidate Expr) (Expr, bool) {
			name, ok := candidate.(*NameExpr)
			if !ok {
				return nil, false
			}
			cell, found := replacements[name.Name()]
			if !found {
				return nil, false
			}
			return cell.NameExpr(), true
		})(expression)
	}
	disjunction := MustOperExpr(Op("OR"), Typed(BoolT1), disjuncts...)
	return executor.TransitionExecutor.AssertState(disjunction)
}
