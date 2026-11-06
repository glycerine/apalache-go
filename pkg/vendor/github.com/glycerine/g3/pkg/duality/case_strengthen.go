package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/qe"
)

// StrengthenFormulaByCaseSplitting selects unpaired ground literals beneath
// quantifiers, substitutes their model values, and preserves necessary model
// facts. Source redundancy/simplification passes precede native qe-lite.
// Returned formula and case literals own independent references.
// Source: duality_rpfp.cpp:2562-2620.
func (g *RPFP) StrengthenFormulaByCaseSplitting(formula *Expr, simplifier *interp.Manager, engine *qe.Lite) (out *Expr, cases []*Expr, result error) {
	if err := g.open(); err != nil {
		return nil, nil, err
	}
	if err := formula.open(); err != nil {
		return nil, nil, err
	}
	if formula.value.object.context != g.context {
		return nil, nil, ErrContextMismatch
	}
	if simplifier == nil {
		return nil, nil, ErrWrapperOwner
	}
	c := g.context
	m := c.manager
	serviceManager, err := simplifier.ASTManager()
	if err != nil {
		return nil, nil, err
	}
	if serviceManager != m {
		return nil, nil, ErrContextMismatch
	}
	var pins, selected []*Expr
	defer func() {
		for _, pin := range pins {
			result = errors.Join(result, pin.Close())
		}
		if result != nil {
			for _, term := range selected {
				result = errors.Join(result, term.Close())
			}
			cases = nil
			if out != nil {
				result = errors.Join(result, out.Close())
				out = nil
			}
		}
	}()
	own := func(term *Expr, err error) (*Expr, error) {
		if term != nil {
			pins = append(pins, term)
		}
		return term, err
	}
	wrap := func(h ast.ASTHandle, err error) (*Expr, error) {
		if err != nil {
			return nil, err
		}
		return own(NewExpr(c, h))
	}
	fromRef := func(ref ast.ASTRef, err error) (*Expr, error) {
		if err != nil {
			return nil, err
		}
		h, readErr := m.RefGet(ref)
		if readErr != nil {
			return nil, errors.Join(readErr, m.DelRef(&ref))
		}
		term, createErr := wrap(h, nil)
		return term, errors.Join(createErr, m.DelRef(&ref))
	}
	simplify := func(term *Expr) (*Expr, error) {
		h, err := term.Raw()
		if err != nil {
			return nil, err
		}
		return fromRef(simplifier.Simplify(h))
	}
	literals, err := GetGroundLitsUnderQuants(formula)
	if err != nil {
		return nil, nil, err
	}
	pins = append(pins, literals...)
	set := make(map[ast.ASTHandle]bool)
	for _, lit := range literals {
		h, err := lit.Raw()
		if err != nil {
			return nil, nil, err
		}
		set[h] = true
	}
	substitution := make(map[ast.ASTHandle]ast.ASTHandle)
	truthMemo := make(map[ast.ASTHandle]int)
	var conjuncts []ast.ASTHandle
	for _, lit := range literals {
		h, err := lit.Raw()
		if err != nil {
			return nil, nil, err
		}
		negative, atom, err := m.IsNot(h)
		if err != nil {
			return nil, nil, err
		}
		opposite := atom
		if !negative {
			other, err := wrap(m.MkNot(h))
			if err != nil {
				return nil, nil, err
			}
			opposite, err = other.Raw()
			if err != nil {
				return nil, nil, err
			}
			atom = h
		}
		if set[opposite] {
			continue
		}
		caseCopy, err := lit.Clone()
		if err != nil {
			return nil, nil, err
		}
		selected = append(selected, caseCopy)
		targetValue := negative
		value, err := own(c.BoolVal(targetValue))
		if err != nil {
			return nil, nil, err
		}
		valueRaw, err := value.Raw()
		if err != nil {
			return nil, nil, err
		}
		kind, err := m.GetKind(atom)
		if err != nil {
			return nil, nil, err
		}
		if kind == ast.ASTQuantifier {
			substitution[atom] = valueRaw
			continue
		}
		truth, err := g.subtermTruth(truthMemo, atom)
		if err != nil {
			return nil, nil, err
		}
		desired := 0
		if targetValue {
			desired = 1
		}
		if truth == desired {
			substitution[atom] = valueRaw
			continue
		}
		if truth != 0 && truth != 1 {
			continue
		}
		value, err = own(c.BoolVal(truth == 1))
		if err != nil {
			return nil, nil, err
		}
		valueRaw, err = value.Raw()
		if err != nil {
			return nil, nil, err
		}
		substitution[atom] = valueRaw
		fact := atom
		if truth == 0 {
			negated, err := wrap(m.MkNot(atom))
			if err != nil {
				return nil, nil, err
			}
			fact, err = negated.Raw()
			if err != nil {
				return nil, nil, err
			}
		}
		conjuncts = append(conjuncts, fact)
	}
	current := formula
	if len(substitution) > 0 {
		root, err := formula.Raw()
		if err != nil {
			return nil, nil, err
		}
		current, err = fromRef(substituteExpression(c, root, substitution))
		if err != nil {
			return nil, nil, err
		}
		if len(conjuncts) > 0 {
			conjunction, err := wrap(m.MkAndArgs(conjuncts))
			if err != nil {
				return nil, nil, err
			}
			current, err = own(current.And(conjunction))
			if err != nil {
				return nil, nil, err
			}
		}
		current, err = simplify(current)
		if err != nil {
			return nil, nil, err
		}
	}
	before, err := current.Raw()
	if err != nil {
		return nil, nil, err
	}
	current, err = own(c.RemoveRedundancy(current))
	if err != nil {
		return nil, nil, err
	}
	after, err := current.Raw()
	if err != nil {
		return nil, nil, err
	}
	changed := before != after
	current, err = simplify(current)
	if err != nil {
		return nil, nil, err
	}
	if changed {
		current, err = own(c.RemoveRedundancy(current))
		if err != nil {
			return nil, nil, err
		}
		current, err = simplify(current)
		if err != nil {
			return nil, nil, err
		}
	}
	out, err = AdjustQuantifiers(current, engine)
	if err != nil {
		return nil, nil, err
	}
	return out, selected, nil
}
