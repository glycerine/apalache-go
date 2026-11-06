package smtcore

// Source: src/smt/theory_lra.cpp:862-984, concrete Relevant operator axioms.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

const smtArithOpIsInt int32 = 16

type smtLRAAxiomCall struct {
	owner *SMTLRATheory
	roots []ast.ASTRef
}

func (c *smtLRAAxiomCall) hold(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ref, err := c.owner.axiomManager.NewRef(h)
	if err == nil {
		c.roots = append(c.roots, ref)
	}
	return h, err
}
func (c *smtLRAAxiomCall) app(op int32, args ...ast.ASTHandle) (ast.ASTHandle, error) {
	return c.hold(c.owner.axiomManager.MkFamilyApp(c.owner.FamilyID(), op, nil, args))
}
func (c *smtLRAAxiomCall) number(n int64, integer bool) (ast.ASTHandle, error) {
	return c.hold(c.owner.axiomManager.ArithmeticNumeralRat(big.NewRat(n, 1), integer))
}
func (c *smtLRAAxiomCall) literal(h ast.ASTHandle) (SMTLiteral, error) {
	if err := c.owner.state.Internalize(h, false); err != nil {
		return SMTNullLiteral, err
	}
	return c.owner.state.Literal(h)
}
func (c *smtLRAAxiomCall) eq(a, b ast.ASTHandle) (SMTLiteral, error) {
	h, err := c.hold(c.owner.state.mkEqualityAtom(a, b))
	if err != nil {
		return SMTNullLiteral, err
	}
	return c.literal(h)
}
func (c *smtLRAAxiomCall) atom(op int32, a, b ast.ASTHandle) (SMTLiteral, error) {
	h, err := c.app(op, a, b)
	if err != nil {
		return SMTNullLiteral, err
	}
	return c.literal(h)
}
func (c *smtLRAAxiomCall) close() error {
	for i := len(c.roots) - 1; i >= 0; i-- {
		if err := c.owner.axiomManager.DelRef(&c.roots[i]); err != nil {
			c.owner.pendingAxiomRoots = append(c.owner.pendingAxiomRoots, c.roots[:i+1]...)
			return err
		}
	}
	return nil
}

func (c *smtLRAAxiomCall) base(literal SMTLiteral) (ast.ASTHandle, error) {
	if literal.Var() == SMTTrueBoolVar {
		return c.hold(c.owner.axiomManager.MkTrue())
	}
	return c.owner.state.BoolExpression(literal.Var())
}

func (c *smtLRAAxiomCall) unit(literal SMTLiteral) error {
	s := c.owner.state
	if err := s.addTheoryAxiom(c.owner.FamilyID(), []SMTLiteral{SMTFalseLiteral, literal}); err != nil {
		return err
	}
	if s.relevancy != nil && s.relevancy.enabled {
		expression, err := c.base(literal)
		if err != nil {
			return err
		}
		return s.markRelevant(expression)
	}
	return nil
}
func (c *smtLRAAxiomCall) binary(first, second SMTLiteral) error {
	if first == SMTFalseLiteral {
		return c.unit(second)
	}
	s := c.owner.state
	if err := s.addTheoryAxiom(c.owner.FamilyID(), []SMTLiteral{first, second}); err != nil {
		return err
	}
	if s.relevancy == nil || !s.relevancy.enabled {
		return nil
	}
	condition, err := c.base(first)
	if err != nil {
		return err
	}
	if err = s.markRelevant(condition); err != nil {
		return err
	}
	target, err := c.base(second)
	if err != nil {
		return err
	}
	if second.Sign() {
		target, err = c.hold(s.manager.MkNot(target))
		if err != nil {
			return err
		}
	}
	ref, err := s.manager.NewRef(target)
	if err != nil {
		return err
	}
	c.owner.axiomRefs = append(c.owner.axiomRefs, ref)
	return s.relevancy.AddWatchTarget(condition, first.Sign(), target)
}

func (t *SMTLRATheory) Relevant(term ast.ASTHandle) (result error) {
	if _, err := t.State(); err != nil {
		return err
	}
	if len(t.pendingAxiomRoots) != 0 {
		return ast.ErrASTHandle
	}
	if err := t.traceLRAExpression("relevant_eh", 864, "", term, "\n"); err != nil {
		return err
	}
	op, own, err := t.operation(term)
	if err != nil || !own {
		return err
	}
	if op != smtArithOpDiv && op != smtArithOpRem && op != smtArithOpMod && op != smtArithOpToInt && op != smtArithOpIsInt {
		return nil
	}
	c := smtLRAAxiomCall{owner: t}
	defer func() { result = errors.Join(result, c.close()) }()
	x, err := t.state.manager.GetArg(term, 0)
	if err != nil {
		return err
	}
	if op == smtArithOpToInt || op == smtArithOpIsInt {
		if op == smtArithOpToInt {
			kind, own, err := t.operation(x)
			if err != nil {
				return err
			}
			if own && kind == smtArithOpToReal {
				y, err := t.state.manager.GetArg(x, 0)
				if err != nil {
					return err
				}
				literal, err := c.eq(y, term)
				if err != nil {
					return err
				}
				return c.unit(literal)
			}
			toReal, err := c.app(smtArithOpToReal, term)
			if err != nil {
				return err
			}
			zero, err := c.number(0, false)
			if err != nil {
				return err
			}
			one, err := c.number(1, false)
			if err != nil {
				return err
			}
			difference, err := c.app(smtArithOpSub, toReal, x)
			if err != nil {
				return err
			}
			lo, err := c.atom(smtArithOpLE, difference, zero)
			if err != nil {
				return err
			}
			difference, err = c.app(smtArithOpSub, x, toReal)
			if err != nil {
				return err
			}
			hi, err := c.atom(smtArithOpGE, difference, one)
			if err != nil {
				return err
			}
			if err = c.unit(lo); err != nil {
				return err
			}
			return c.unit(hi.Negated())
		}
		toInt, err := c.app(smtArithOpToInt, x)
		if err != nil {
			return err
		}
		toReal, err := c.app(smtArithOpToReal, toInt)
		if err != nil {
			return err
		}
		equality, err := c.eq(toReal, x)
		if err != nil {
			return err
		}
		isInt, err := t.state.Literal(term)
		if err != nil {
			return err
		}
		if err = c.binary(isInt.Negated(), equality); err != nil {
			return err
		}
		return c.binary(isInt, equality.Negated())
	}
	y, err := t.state.manager.GetArg(term, 1)
	if err != nil {
		return err
	}
	value, number, err := t.numeral(y)
	if err != nil {
		return err
	}
	if op != smtArithOpRem && number && value.Sign() == 0 {
		return nil
	}
	zero, err := c.number(0, op != smtArithOpDiv)
	if err != nil {
		return err
	}
	if op == smtArithOpDiv {
		guard, err := c.eq(y, zero)
		if err != nil {
			return err
		}
		product, err := c.app(smtArithOpMul, y, term)
		if err != nil {
			return err
		}
		equality, err := c.eq(product, x)
		if err != nil {
			return err
		}
		return c.binary(guard, equality)
	}
	mod, err := c.app(smtArithOpMod, x, y)
	if err != nil {
		return err
	}
	ge, err := c.atom(smtArithOpGE, y, zero)
	if err != nil {
		return err
	}
	if op == smtArithOpRem {
		negative, err := c.app(smtArithOpUMinus, mod)
		if err != nil {
			return err
		}
		eq1, err := c.eq(term, mod)
		if err != nil {
			return err
		}
		eq2, err := c.eq(term, negative)
		if err != nil {
			return err
		}
		if err = c.binary(ge.Negated(), eq1); err != nil {
			return err
		}
		return c.binary(ge, eq2)
	}
	div, err := c.app(smtArithOpIDiv, x, y)
	if err != nil {
		return err
	}
	le, err := c.atom(smtArithOpLE, y, zero)
	if err != nil {
		return err
	}
	product, err := c.app(smtArithOpMul, y, div)
	if err != nil {
		return err
	}
	sum, err := c.app(smtArithOpAdd, product, mod)
	if err != nil {
		return err
	}
	equality, err := c.eq(sum, x)
	if err != nil {
		return err
	}
	modGE, err := c.atom(smtArithOpGE, mod, zero)
	if err != nil {
		return err
	}
	for _, pair := range [][2]SMTLiteral{{ge, equality}, {le, equality}, {ge, modGE}, {le, modGE}} {
		if err = c.binary(pair[0], pair[1]); err != nil {
			return err
		}
	}
	difference, err := c.app(smtArithOpSub, mod, y)
	if err != nil {
		return err
	}
	bound, err := c.atom(smtArithOpGE, difference, zero)
	if err != nil {
		return err
	}
	if err = c.binary(le, bound.Negated()); err != nil {
		return err
	}
	sum, err = c.app(smtArithOpAdd, mod, y)
	if err != nil {
		return err
	}
	bound, err = c.atom(smtArithOpGE, sum, zero)
	if err != nil {
		return err
	}
	return c.binary(ge, bound.Negated())
}
