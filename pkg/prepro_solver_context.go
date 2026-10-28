package apalache

import (
	"context"
	"fmt"
)

type preproFactKey struct {
	left, right string
}

type preproFact struct {
	value bool
}

// PreproSolverContext propagates cached top-level cell equality and
// membership facts before delegating expressions to another solver context.
type PreproSolverContext struct {
	context GroundSolverContext
	facts   map[preproFactKey]preproFact
	stack   []map[preproFactKey]preproFact
}

func NewPreproSolverContext(context GroundSolverContext) (*PreproSolverContext, error) {
	if context == nil {
		return nil, fmt.Errorf("preprocessing solver context requires a delegate")
	}
	return &PreproSolverContext{context: context, facts: make(map[preproFactKey]preproFact)}, nil
}

func (prepro *PreproSolverContext) Config() SolverConfig          { return prepro.context.Config() }
func (prepro *PreproSolverContext) Level() int                    { return prepro.context.Level() }
func (prepro *PreproSolverContext) Metrics() SolverContextMetrics { return prepro.context.Metrics() }
func (prepro *PreproSolverContext) SetSMTListener(listener SMTListener) {
	prepro.context.SetSMTListener(listener)
}
func (prepro *PreproSolverContext) DeclareCell(cell ArenaCell) error {
	return prepro.context.DeclareCell(cell)
}
func (prepro *PreproSolverContext) DeclareMembership(set, element ArenaCell) error {
	return prepro.context.DeclareMembership(set, element)
}
func (prepro *PreproSolverContext) Check(ctx context.Context) (SMTStatus, error) {
	return prepro.context.Check(ctx)
}
func (prepro *PreproSolverContext) CheckWithTimeout(ctx context.Context, seconds int) (SMTStatus, error) {
	return prepro.context.CheckWithTimeout(ctx, seconds)
}
func (prepro *PreproSolverContext) CheckConsistency(arena *PureArena) error {
	return prepro.context.CheckConsistency(arena)
}
func (prepro *PreproSolverContext) Log(message string) { prepro.context.Log(message) }
func (prepro *PreproSolverContext) Logs() []string     { return prepro.context.Logs() }

func (prepro *PreproSolverContext) Assert(expression Expr) error {
	simplified, err := simplifyPreproShallow(expression)
	if err != nil {
		return err
	}
	simplified = prepro.Preprocess(simplified)
	simplified, err = simplifyPreproShallow(simplified)
	if err != nil {
		return err
	}
	prepro.cacheTopLevel(simplified)
	return prepro.context.Assert(simplified)
}

func simplifyPreproShallow(expression Expr) (Expr, error) {
	if name, ok := expression.(*NameExpr); ok {
		switch name.Name() {
		case ArenaFalseName:
			return boolExpr(false), nil
		case ArenaTrueName:
			return boolExpr(true), nil
		}
	}
	if operator, ok := expression.(*OperExpr); ok {
		if _, membership := membershipPolarity(operator.Operator().Name()); membership {
			return expression, nil
		}
	}
	return simplifyConstantShallow(expression)
}

func (prepro *PreproSolverContext) Evaluate(expression Expr) (Expr, error) {
	return prepro.context.Evaluate(prepro.Preprocess(expression))
}

func (prepro *PreproSolverContext) Preprocess(expression Expr) Expr {
	operator, ok := expression.(*OperExpr)
	if !ok {
		return expression
	}
	arguments := operator.Arguments()
	name := operator.Operator().Name()
	if len(arguments) == 2 {
		left, leftName := arguments[0].(*NameExpr)
		right, rightName := arguments[1].(*NameExpr)
		if leftName && rightName {
			if positive, recognized := equalityPolarity(name); recognized {
				key := equalityFactKey(left.Name(), right.Name())
				if fact, found := prepro.facts[key]; found {
					return boolExpr(fact.value == positive)
				}
			}
			if positive, recognized := membershipPolarity(name); recognized {
				key := preproFactKey{left: left.Name(), right: right.Name()}
				if fact, found := prepro.facts[key]; found {
					return boolExpr(fact.value == positive)
				}
			}
		}
		if _, membership := membershipPolarity(name); membership {
			return expression
		}
	}
	rewritten := make([]Expr, len(arguments))
	for index, argument := range arguments {
		if name == "FUN_APP" && index == 0 {
			rewritten[index] = argument
		} else {
			rewritten[index] = prepro.Preprocess(argument)
		}
	}
	return MustOperExpr(operator.Operator(), operator.Tag(), rewritten...)
}

func (prepro *PreproSolverContext) cacheTopLevel(expression Expr) {
	operator, ok := expression.(*OperExpr)
	if !ok || len(operator.Arguments()) != 2 {
		return
	}
	left, leftName := operator.Arguments()[0].(*NameExpr)
	right, rightName := operator.Arguments()[1].(*NameExpr)
	if !leftName || !rightName {
		return
	}
	if _, err := ArenaCellIDFromName(left.Name()); err != nil {
		return
	}
	if _, err := ArenaCellIDFromName(right.Name()); err != nil {
		return
	}
	if positive, recognized := equalityPolarity(operator.Operator().Name()); recognized {
		prepro.facts[equalityFactKey(left.Name(), right.Name())] = preproFact{value: positive}
		prepro.Log(fmt.Sprintf(";;    -> pp eq cached as %t ", positive))
		return
	}
	if positive, recognized := membershipPolarity(operator.Operator().Name()); recognized {
		key := preproFactKey{left: left.Name(), right: right.Name()}
		prepro.facts[key] = preproFact{value: positive}
		prepro.Log(fmt.Sprintf(";;    -> pp in cached as %t ", positive))
	}
}

func equalityPolarity(operator string) (bool, bool) {
	switch operator {
	case "EQ":
		return true, true
	case "NE":
		return false, true
	default:
		return false, false
	}
}

func membershipPolarity(operator string) (bool, bool) {
	switch operator {
	case "SET_IN", "Apalache!SelectInSet", "Apalache!SelectInFun", "Apalache!StoreInSet":
		return true, true
	case "SET_NOT_IN", "Apalache!storeNotInSet", "Apalache!storeNotInFun":
		return false, true
	default:
		return false, false
	}
}

func equalityFactKey(left, right string) preproFactKey {
	if left > right {
		left, right = right, left
	}
	return preproFactKey{left: left, right: right}
}

func clonePreproFacts(facts map[preproFactKey]preproFact) map[preproFactKey]preproFact {
	cloned := make(map[preproFactKey]preproFact, len(facts))
	for key, fact := range facts {
		cloned[key] = fact
	}
	return cloned
}

func (prepro *PreproSolverContext) Push() error {
	if err := prepro.context.Push(); err != nil {
		return err
	}
	prepro.stack = append(prepro.stack, clonePreproFacts(prepro.facts))
	return nil
}

func (prepro *PreproSolverContext) Pop(count ...int) error {
	n := 1
	if len(count) > 1 {
		return prepro.context.Pop(count...)
	}
	if len(count) == 1 {
		n = count[0]
	}
	if err := prepro.context.Pop(n); err != nil {
		return err
	}
	if n == 0 {
		return nil
	}
	prepro.facts = prepro.stack[len(prepro.stack)-n]
	prepro.stack = prepro.stack[:len(prepro.stack)-n]
	return nil
}

func (prepro *PreproSolverContext) Dispose() error {
	clear(prepro.facts)
	prepro.stack = nil
	return prepro.context.Dispose()
}

var _ GroundSolverContext = (*PreproSolverContext)(nil)
