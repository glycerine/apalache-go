package smtcore

// Source: src/smt/theory_arith_core.h internalize_div/idiv/mod/rem,
// mk_axiom, mk_div_axiom, mk_idiv_mod_axioms and mk_rem_axiom.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

const (
	smtArithOpIDiv int32 = 11
	smtArithOpRem  int32 = 12
	smtArithOpMod  int32 = 13
)

type smtArithmeticDivisionCall struct {
	theory *SMTLinearArithmeticTheory
	refs   []ast.ASTRef
}

func (c *smtArithmeticDivisionCall) hold(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ref, err := c.theory.state.manager.NewRef(h)
	if err == nil {
		c.refs = append(c.refs, ref)
	}
	return h, err
}
func (c *smtArithmeticDivisionCall) app(op int32, args ...ast.ASTHandle) (ast.ASTHandle, error) {
	return c.hold(c.theory.state.manager.MkFamilyApp(c.theory.FamilyID(), op, nil, args))
}
func (c *smtArithmeticDivisionCall) close() (result error) {
	for i := len(c.refs) - 1; i >= 0; i-- {
		result = errors.Join(result, c.theory.state.manager.DelRef(&c.refs[i]))
	}
	return result
}

// Source mk_axiom asserts (antecedent OR consequent) and arranges relevance
// of the consequent when the antecedent becomes false.
func (t *SMTLinearArithmeticTheory) assertGuardedArithmeticAxiom(antecedent, consequent ast.ASTHandle) (result error) {
	s := t.state
	m := s.manager
	refs := make([]ast.ASTRef, 0, 2)
	defer func() {
		for i := range refs {
			result = errors.Join(result, m.DelRef(&refs[i]))
		}
	}()
	for _, h := range []ast.ASTHandle{antecedent, consequent} {
		ref, err := m.NewRef(h)
		if err != nil {
			return err
		}
		refs = append(refs, ref)
		if err = s.Internalize(h, true); err != nil {
			return err
		}
	}
	a, err := s.Literal(antecedent)
	if err != nil {
		return err
	}
	b, err := s.Literal(consequent)
	if err != nil {
		return err
	}
	if err = s.addTheoryAxiom(t.FamilyID(), []SMTLiteral{a, b}); err != nil {
		return err
	}
	if s.relevancy != nil {
		if a == SMTFalseLiteral {
			return s.markRelevant(consequent)
		}
		if a.Var() == SMTTrueBoolVar {
			return nil
		}
		atom, err := s.BoolExpression(a.Var())
		if err != nil {
			return err
		}
		if err = s.markRelevant(atom); err != nil {
			return err
		}
		return s.relevancy.AddWatchTarget(atom, a.Sign(), consequent)
	}
	return nil
}

func (t *SMTLinearArithmeticTheory) installDivisionAxioms(term ast.ASTHandle, op int32) (result error) {
	if t.divisionTerms[term] {
		return nil
	}
	t.divisionTerms[term] = true
	t.divisionTermTrail = append(t.divisionTermTrail, term)
	m := t.state.manager
	c := smtArithmeticDivisionCall{theory: t}
	defer func() { result = errors.Join(result, c.close()) }()
	p, err := m.GetArg(term, 0)
	if err != nil {
		return err
	}
	q, err := m.GetArg(term, 1)
	if err != nil {
		return err
	}
	var mod ast.ASTHandle
	if op != smtArithOpDiv {
		mod, err = c.app(smtArithOpMod, p, q)
		if err != nil {
			return err
		}
	}
	if op == smtArithOpIDiv {
		if err = t.state.Internalize(mod, false); err != nil {
			return err
		}
		if t.state.relevancy != nil {
			return t.state.relevancy.AddDependency(term, mod)
		}
		return nil
	}
	value, numeral, err := t.numeral(q)
	if err != nil {
		return err
	}
	if op != smtArithOpRem && numeral && value.Sign() == 0 {
		return nil
	}
	if op == smtArithOpRem {
		positive, err := c.hold(m.MkEq(term, mod))
		if err != nil {
			return err
		}
		negativeMod, err := c.app(smtArithOpUMinus, mod)
		if err != nil {
			return err
		}
		negative, err := c.hold(m.MkEq(term, negativeMod))
		if err != nil {
			return err
		}
		if numeral {
			if value.Sign() < 0 {
				return t.assertArithmeticAxiom(negative)
			}
			return t.assertArithmeticAxiom(positive)
		}
		zero, err := c.hold(m.ArithmeticIntegerNumeralText("0"))
		if err != nil {
			return err
		}
		less, err := c.app(smtArithOpLT, q, zero)
		if err != nil {
			return err
		}
		if err = t.assertGuardedArithmeticAxiom(less, positive); err != nil {
			return err
		}
		notLess, err := c.hold(m.MkNot(less))
		if err != nil {
			return err
		}
		return t.assertGuardedArithmeticAxiom(notLess, negative)
	}
	integer := op != smtArithOpDiv
	zero, err := c.hold(m.ArithmeticNumeralRat(new(big.Rat), integer))
	if err != nil {
		return err
	}
	eqZero, err := c.hold(m.MkEq(q, zero))
	if err != nil {
		return err
	}
	quotient := term
	if integer {
		quotient, err = c.app(smtArithOpIDiv, p, q)
		if err != nil {
			return err
		}
	}
	product, err := c.app(smtArithOpMul, q, quotient)
	if err != nil {
		return err
	}
	sum := product
	if integer {
		sum, err = c.app(smtArithOpAdd, product, mod)
		if err != nil {
			return err
		}
	}
	equation, err := c.hold(m.MkEq(sum, p))
	if err != nil {
		return err
	}
	assert := func(formula ast.ASTHandle) error {
		if numeral {
			return t.assertArithmeticAxiom(formula)
		}
		return t.assertGuardedArithmeticAxiom(eqZero, formula)
	}
	if err = assert(equation); err != nil {
		return err
	}
	if !integer {
		return nil
	}
	lower, err := c.app(smtArithOpGE, mod, zero)
	if err != nil {
		return err
	}
	if err = assert(lower); err != nil {
		return err
	}
	var upperBound ast.ASTHandle
	if numeral {
		bound := new(big.Rat).Abs(value)
		bound.Sub(bound, big.NewRat(1, 1))
		upperBound, err = c.hold(m.ArithmeticNumeralRat(bound, true))
	} else {
		less, e := c.app(smtArithOpLT, q, zero)
		if e != nil {
			return e
		}
		negative, e := c.app(smtArithOpUMinus, q)
		if e != nil {
			return e
		}
		abs, e := c.hold(m.MkIte(less, negative, q))
		if e != nil {
			return e
		}
		one, e := c.hold(m.ArithmeticIntegerNumeralText("1"))
		if e != nil {
			return e
		}
		upperBound, err = c.app(smtArithOpSub, abs, one)
	}
	if err != nil {
		return err
	}
	upper, err := c.app(smtArithOpLE, mod, upperBound)
	if err != nil {
		return err
	}
	if err = assert(upper); err != nil {
		return err
	}
	return nil
}
