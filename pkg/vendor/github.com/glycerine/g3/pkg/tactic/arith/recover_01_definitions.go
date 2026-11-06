package arith

// Source: src/tactic/arith/recover_01_tactic.cpp, mk_ivar and process.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

type recover01Boolean struct {
	atom    ast.ASTHandle
	atomRef ast.ASTRef
	fresh   ast.ASTRef
}

type recover01Definitions struct {
	manager   *ast.ASTManager
	goal      *tactic.Goal
	sub       ast.ASTExprSubstitution
	booleans  []recover01Boolean
	variables []ast.ASTHandle
	lookup    map[ast.ASTHandle]int
}

func (d *recover01Definitions) Close() (err error) {
	for i := range d.booleans {
		err = errors.Join(err, d.manager.DelRef(&d.booleans[i].atomRef))
		err = errors.Join(err, d.manager.DelRef(&d.booleans[i].fresh))
	}
	d.booleans = nil
	d.lookup = nil
	return err
}

func (d *recover01Definitions) booleanVariable(atom ast.ASTHandle) (ast.ASTHandle, error) {
	if i, ok := d.lookup[atom]; ok {
		return d.manager.RefGet(d.booleans[i].fresh)
	}
	m := d.manager
	family, err := m.RegisterArithmeticPlugin()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sort, err := m.MkFamilySort(family, 1, nil)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	fresh, err := m.MkFreshConstNullable(nil, sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	atomRef, err := m.NewRef(atom)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ref, err := m.NewRef(fresh)
	if err != nil {
		return ast.ASTHandle{}, errors.Join(err, m.DelRef(&atomRef))
	}
	d.booleans = append(d.booleans, recover01Boolean{atom: atom, atomRef: atomRef, fresh: ref})
	if d.lookup == nil {
		d.lookup = make(map[ast.ASTHandle]int)
	}
	d.lookup[atom] = len(d.booleans) - 1
	zero, err := m.ArithmeticIntegerNumeralText("0")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	lower, err := m.MkArithmeticLE(zero, fresh)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = d.goal.Assert(lower); err != nil {
		return ast.ASTHandle{}, err
	}
	one, err := m.ArithmeticIntegerNumeralText("1")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	upper, err := m.MkArithmeticLE(fresh, one)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = d.goal.Assert(upper); err != nil {
		return ast.ASTHandle{}, err
	}
	equal, err := m.MkEq(fresh, one)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = m.ExprSubstitutionInsert(d.sub, atom, equal, ast.ASTHandle{}, ast.ASTDependency{}); err != nil {
		return ast.ASTHandle{}, err
	}
	return fresh, nil
}

func (d *recover01Definitions) build(variable ast.ASTHandle, literals []ast.ASTHandle, coefficients []big.Rat, trace ...func(ast.ASTHandle, ast.ASTHandle) error) (err error) {
	if d == nil || d.manager == nil || d.goal == nil || len(literals) == 0 || len(literals) != len(coefficients) {
		return ast.ErrASTDomain
	}
	m := d.manager
	family, err := m.RegisterArithmeticPlugin()
	if err != nil {
		return err
	}
	integer, valid, err := arithmeticConstant(m, variable, family)
	if err != nil {
		return err
	}
	if !valid {
		return ast.ErrASTDomain
	}
	var held []ast.ASTRef
	defer func() {
		for i := len(held) - 1; i >= 0; i-- {
			err = errors.Join(err, m.DelRef(&held[i]))
		}
	}()
	hold := func(h ast.ASTHandle) error {
		r, e := m.NewRef(h)
		if e == nil {
			held = append(held, r)
		}
		return e
	}
	terms := make([]ast.ASTHandle, 0, len(literals))
	for i, lit := range literals {
		negative, atom, e := m.IsNot(lit)
		if e != nil {
			return e
		}
		if !negative {
			atom = lit
		}
		fresh, e := d.booleanVariable(atom)
		if e != nil {
			return e
		}
		value := fresh
		if !integer {
			value, e = m.MkFamilyApp(family, 14, nil, []ast.ASTHandle{fresh})
			if e != nil {
				return e
			}
			if e = hold(value); e != nil {
				return e
			}
		}
		if negative {
			one, x := m.ArithmeticNumeralRat(big.NewRat(1, 1), integer)
			if x != nil {
				return x
			}
			value, e = m.MkArithmeticSub(one, value)
			if e != nil {
				return e
			}
			if e = hold(value); e != nil {
				return e
			}
		}
		coefficient, e := m.ArithmeticNumeralRat(&coefficients[i], integer)
		if e != nil {
			return e
		}
		term, e := m.MkArithmeticMul(coefficient, value)
		if e != nil {
			return e
		}
		if e = hold(term); e != nil {
			return e
		}
		terms = append(terms, term)
	}
	definition := terms[0]
	if len(terms) > 1 {
		definition, err = m.MkFamilyApp(family, 6, nil, terms)
		if err != nil {
			return err
		}
		if err = hold(definition); err != nil {
			return err
		}
	}
	if err = m.ExprSubstitutionInsert(d.sub, variable, definition, ast.ASTHandle{}, ast.ASTDependency{}); err != nil {
		return err
	}
	if len(trace) != 0 && trace[0] != nil {
		if err = trace[0](variable, definition); err != nil {
			return err
		}
	}
	d.variables = append(d.variables, variable)
	return nil
}
