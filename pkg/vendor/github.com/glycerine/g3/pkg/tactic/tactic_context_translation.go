package tactic

// Source: src/tactic/tactical.cpp tactic translation. This concrete traversal
// rebuilds the live composition nodes while target-specific tactic leaves are
// translated by their owning subsystem.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// translateContextProbe copies the built-in probe tree into the target budget.
// Subsystem probes need their own target services and are rejected for now.
func translateContextProbe(value ProbeEvaluator, budget *arena.Budget, translateLeaf func(ProbeEvaluator) (ProbeEvaluator, error)) (result ProbeEvaluator, resultErr error) {
	p, ok := value.(*Probe)
	if !ok {
		return translateLeaf(value)
	}
	if err := p.Check(); err != nil {
		return nil, err
	}
	if p.left == nil {
		return NewProbe(budget, p.kind, p.constant)
	}
	left, err := translateContextProbe(p.left, budget, translateLeaf)
	if err != nil {
		return nil, err
	}
	if err = left.IncRef(); err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, left.DecRef()) }()
	if p.right == nil {
		return NewNotProbe(budget, left)
	}
	right, err := translateContextProbe(p.right, budget, translateLeaf)
	if err != nil {
		return nil, err
	}
	if err = right.IncRef(); err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, right.DecRef()) }()
	return NewBinaryProbe(budget, p.kind, left, right)
}

// TranslateWithContext rebuilds supported tactic compositions for a target
// parameter service. translateLeaf handles the concrete subsystem tactics;
// the caller owns the returned uncounted tactic in the usual tactic protocol.
func TranslateWithContext(value Tactic, m *ast.ASTManager, params *uparams.ParamsContext, translateLeaf func(Tactic) (Tactic, error), translateProbe func(ProbeEvaluator) (ProbeEvaluator, error)) (result Tactic, resultErr error) {
	if value == nil || m == nil || params == nil || translateLeaf == nil || translateProbe == nil {
		return nil, ast.ErrASTHandle
	}
	owned := make([]Tactic, 0, 2)
	defer func() {
		for i := len(owned) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, owned[i].DecRef())
		}
		if resultErr != nil && result != nil {
			if e := result.IncRef(); e == nil {
				resultErr = errors.Join(resultErr, result.DecRef())
			} else {
				resultErr = errors.Join(resultErr, e)
			}
			result = nil
		}
	}()
	child := func(v Tactic) (Tactic, error) {
		translated, err := TranslateWithContext(v, m, params, translateLeaf, translateProbe)
		if err != nil {
			return nil, err
		}
		if err = translated.IncRef(); err != nil {
			return nil, err
		}
		owned = append(owned, translated)
		return translated, nil
	}
	switch t := value.(type) {
	case *primitiveTactic:
		return t.Translate(m)
	case *andThenTactic:
		if err := t.open(); err != nil {
			return nil, err
		}
		a, err := child(t.first)
		if err != nil {
			return nil, err
		}
		b, err := child(t.second)
		if err != nil {
			return nil, err
		}
		return newAndThenTactic(a, b)
	case *orElseTactic:
		if err := t.open(); err != nil {
			return nil, err
		}
		children := make([]Tactic, 0, len(t.children))
		for _, v := range t.children {
			c, err := child(v)
			if err != nil {
				return nil, err
			}
			children = append(children, c)
		}
		return NewOrElseTactic(children...)
	case *usingParamsTactic:
		if err := t.open(); err != nil {
			return nil, err
		}
		c, err := child(t.child)
		if err != nil {
			return nil, err
		}
		copied, err := t.context.TranslateTo(&t.params, params)
		if err != nil {
			return nil, err
		}
		defer func() { resultErr = errors.Join(resultErr, params.Del(&copied)) }()
		return NewUsingParamsTactic(params, c, &copied)
	case *annotateTactic:
		if err := t.open(); err != nil {
			return nil, err
		}
		c, err := child(t.child)
		if err != nil {
			return nil, err
		}
		return NewAnnotateTactic(t.name, c)
	case *unaryTactic:
		if err := t.open(); err != nil {
			return nil, err
		}
		c, err := child(t.child)
		if err != nil {
			return nil, err
		}
		return newUnaryTactic(t.kind, c, t.threshold)
	case *conditionalTactic:
		if err := t.open(); err != nil {
			return nil, err
		}
		var probe ProbeEvaluator
		if t.probe != nil {
			var err error
			probe, err = translateContextProbe(t.probe, m.Budget(), translateProbe)
			if err != nil {
				return nil, err
			}
			if err = probe.IncRef(); err != nil {
				return nil, err
			}
			defer func() { resultErr = errors.Join(resultErr, probe.DecRef()) }()
		}
		first, err := child(t.thenTactic)
		if err != nil {
			return nil, err
		}
		var second Tactic
		if t.elseTactic != nil {
			second, err = child(t.elseTactic)
			if err != nil {
				return nil, err
			}
		}
		return newConditionalTactic(t.kind, probe, first, second)
	default:
		return translateLeaf(value)
	}
}
