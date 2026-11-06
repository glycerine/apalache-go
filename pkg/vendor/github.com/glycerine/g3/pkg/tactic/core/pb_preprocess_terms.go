package core

// Source: src/tactic/core/pb_preprocess_tactic.cpp.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type pbPreprocessTerm struct {
	expression  ast.ASTHandle
	coefficient *big.Rat
}

type pbPreprocessConstraint struct {
	terms []pbPreprocessTerm
	bound *big.Rat
}

func (t *PBPreprocessTactic) rational(value ast.ASTHandle, coefficient *uint32) (_ *big.Rat, result error) {
	var owned = func() (v unumeral.OwnedRational, e error) {
		if coefficient == nil {
			return t.manager.PseudoBooleanBound(value)
		}
		return t.manager.PseudoBooleanCoefficient(value, *coefficient)
	}
	v, e := owned()
	if e != nil {
		return nil, e
	}
	defer func() { result = errors.Join(result, t.manager.NumeralService().Del(&v)) }()
	text, e := t.manager.NumeralService().String(&v)
	if e != nil {
		return nil, e
	}
	q, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, ast.ErrASTDomain
	}
	return q, nil
}

func (t *PBPreprocessTactic) negated(h ast.ASTHandle) (ast.ASTHandle, bool, error) {
	m := t.manager
	k, e := m.GetKind(h)
	if e != nil || k != ast.ASTApp {
		return ast.ASTHandle{}, false, e
	}
	f, e := m.GetFamilyID(h)
	if e != nil || f != m.BasicFamilyID() {
		return ast.ASTHandle{}, false, e
	}
	op, e := m.GetDeclKind(h)
	if e != nil || op != ast.ASTOpNot {
		return ast.ASTHandle{}, false, e
	}
	a, e := m.GetArg(h, 0)
	return a, e == nil, e
}

func (t *PBPreprocessTactic) isConstant(h ast.ASTHandle) (bool, error) {
	m := t.manager
	k, e := m.GetKind(h)
	if e != nil || k != ast.ASTApp {
		return false, e
	}
	n, e := m.GetNumArgs(h)
	if e != nil || n != 0 {
		return false, e
	}
	f, e := m.GetFamilyID(h)
	return e == nil && f == ast.NullFamilyID, e
}

func (t *PBPreprocessTactic) parse(h ast.ASTHandle) (*pbPreprocessConstraint, error) {
	m := t.manager
	if yes, e := t.isConstant(h); e != nil {
		return nil, e
	} else if yes {
		return &pbPreprocessConstraint{terms: []pbPreprocessTerm{{h, big.NewRat(1, 1)}}, bound: big.NewRat(1, 1)}, nil
	}
	if child, yes, e := t.negated(h); e != nil {
		return nil, e
	} else if yes {
		if constant, x := t.isConstant(child); x != nil || !constant {
			return nil, x
		}
		return &pbPreprocessConstraint{terms: []pbPreprocessTerm{{h, big.NewRat(1, 1)}}, bound: big.NewRat(1, 1)}, nil
	}
	ge, e := m.IsPseudoBooleanGE(h)
	if e != nil {
		return nil, e
	}
	if ge {
		bound, x := t.rational(h, nil)
		if x != nil {
			return nil, x
		}
		n, x := m.GetNumArgs(h)
		if x != nil {
			return nil, x
		}
		result := &pbPreprocessConstraint{bound: bound}
		for i := uint32(0); i < n; i++ {
			a, y := m.GetArg(h, i)
			if y != nil {
				return nil, y
			}
			if ok, y := t.pureLiteral(a); y != nil || !ok {
				return nil, y
			}
			c, y := t.rational(h, &i)
			if y != nil {
				return nil, y
			}
			result.terms = append(result.terms, pbPreprocessTerm{a, c})
		}
		return result, nil
	}
	f, e := m.GetFamilyID(h)
	if e != nil {
		return nil, e
	}
	op, e := m.GetDeclKind(h)
	if e != nil {
		return nil, e
	}
	if f != m.BasicFamilyID() || op != ast.ASTOpOr {
		return nil, nil
	}
	n, e := m.GetNumArgs(h)
	if e != nil {
		return nil, e
	}
	result := &pbPreprocessConstraint{bound: big.NewRat(1, 1)}
	for i := uint32(0); i < n; i++ {
		a, x := m.GetArg(h, i)
		if x != nil {
			return nil, x
		}
		if ok, x := t.pureLiteral(a); x != nil || !ok {
			return nil, x
		}
		result.terms = append(result.terms, pbPreprocessTerm{a, big.NewRat(1, 1)})
	}
	return result, nil
}

func (t *PBPreprocessTactic) pureLiteral(h ast.ASTHandle) (bool, error) {
	if t.manager.IsTrue(h) || t.manager.IsFalse(h) {
		return true, nil
	}
	if yes, e := t.isConstant(h); e != nil || yes {
		return yes, e
	}
	child, yes, e := t.negated(h)
	if e != nil || !yes {
		return false, e
	}
	return t.isConstant(child)
}
