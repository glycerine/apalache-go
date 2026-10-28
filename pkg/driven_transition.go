package apalache

import (
	"fmt"
	"sort"
)

// DrivenTransitionConstructor builds a transition whose primed output
// variables are defined by expressions evaluated in a supplied trace state.
type DrivenTransitionConstructor struct {
	tracker     TransformationTracker
	expressions map[string]Expr
}

func NewDrivenTransitionConstructor(tracker TransformationTracker, expressions map[string]Expr) *DrivenTransitionConstructor {
	if tracker == nil {
		tracker = IdleTracker{}
	}
	return &DrivenTransitionConstructor{tracker: tracker, expressions: cloneMap(expressions)}
}

func (constructor *DrivenTransitionConstructor) MakeTransition(state TraceState) (Expr, error) {
	if constructor == nil {
		return nil, fmt.Errorf("cannot build a driven transition with a nil constructor")
	}
	names := make([]string, 0, len(constructor.expressions))
	for name := range constructor.expressions {
		names = append(names, name)
	}
	sort.Strings(names)
	builder := NewTypedBuilder()
	assignments := make([]Expr, 0, len(names))
	for _, name := range names {
		expression := constructor.expressions[name]
		if expression == nil {
			return nil, fmt.Errorf("driven expression %s is nil", name)
		}
		if _, typed := expression.Tag().Type(); !typed {
			return nil, fmt.Errorf("driven expression %s is untyped", name)
		}
		primed, err := builder.Prime(NewNameExpr(name, expression.Tag()))
		if err != nil {
			return nil, fmt.Errorf("prime driven variable %s: %w", name, err)
		}
		assignment, err := builder.Assign(primed, expression)
		if err != nil {
			return nil, fmt.Errorf("assign driven variable %s: %w", name, err)
		}
		assignments = append(assignments, assignment)
	}
	transition, err := builder.And(assignments...)
	if err != nil {
		return nil, fmt.Errorf("conjoin driven assignments: %w", err)
	}
	replace := RewriteExpressions(constructor.tracker, func(expression Expr) (Expr, bool) {
		name, ok := expression.(*NameExpr)
		if !ok {
			return nil, false
		}
		value, found := state[name.Name()]
		return value, found
	})
	return replace(transition), nil
}
