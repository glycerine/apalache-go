package smtcore

// Source: src/smt/theory_bv.cpp:185-198,554-685,1298-1305.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// Concrete lexical roots keep native operands and conversion terms alive across
// recursively drained bit-blaster temporaries and context rewriter caches.
type smtBVExpressionCall struct {
	theory *SMTBitVectorTheory
	refs   []ast.ASTRef
}

func (c *smtBVExpressionCall) hold(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ref, err := c.theory.state.manager.NewRef(h)
	if err == nil {
		c.refs = append(c.refs, ref)
	}
	return h, err
}
func (c *smtBVExpressionCall) close() (result error) {
	for i := len(c.refs) - 1; i >= 0; i-- {
		result = errors.Join(result, c.theory.state.manager.DelRef(&c.refs[i]))
	}
	return result
}
func (c *smtBVExpressionCall) number(n *big.Int) (ast.ASTHandle, error) {
	return c.hold(c.theory.state.manager.ArithmeticIntegerNumeralText(n.String()))
}
func (c *smtBVExpressionCall) app(op int32, args ...ast.ASTHandle) (ast.ASTHandle, error) {
	family, err := c.theory.state.manager.ArithmeticFamilyID()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.hold(c.theory.state.manager.MkFamilyApp(family, op, nil, args))
}
func (c *smtBVExpressionCall) bits(expression ast.ASTHandle) ([]ast.ASTHandle, error) {
	t := c.theory
	s := t.state
	node := s.context.enodes[expression]
	if node == nil {
		return nil, ast.ErrASTDomain
	}
	v, err := t.nativeVariable(node)
	if err != nil {
		return nil, err
	}
	if v < 0 || int(v) >= len(t.bitLiterals) {
		return nil, ast.ErrASTDomain
	}
	bits := make([]ast.ASTHandle, 0, len(t.bitLiterals[v]))
	for _, literal := range t.bitLiterals[v] {
		bit, err := c.hold(s.literalExpression(literal))
		if err != nil {
			return nil, err
		}
		bits = append(bits, bit)
	}
	return bits, nil
}

func (t *SMTBitVectorTheory) publishConversionEquality(left, right ast.ASTHandle) error {
	literal, err := t.MkEqualityLiteral(left, right, false)
	if err != nil {
		return err
	}
	return t.publishConversionLiteral(literal)
}

func (t *SMTBitVectorTheory) publishConversionLiteral(literal SMTLiteral) error {
	s := t.state
	if literal.Var() != SMTTrueBoolVar {
		expression, err := s.BoolExpression(literal.Var())
		if err != nil {
			return err
		}
		if err = s.markRelevant(expression); err != nil {
			return err
		}
	}
	return s.addTheoryAxiom(t.FamilyID(), []SMTLiteral{literal})
}

func (t *SMTBitVectorTheory) assertBVToIntAxiom(expression ast.ASTHandle) (result error) {
	s, err := t.State()
	if err != nil {
		return err
	}
	if err = bvAssert(571, "get_context().e_internalized(n)", func() (bool, error) { return s.context.enodes[expression] != nil, nil }); err != nil {
		return err
	}
	if err = bvAssert(572, "m_util.is_bv2int(n)", func() (bool, error) {
		op, own, e := t.operation(expression)
		return own && op == 58, e
	}); err != nil {
		return err
	}
	if err = t.traceBVBVToIntAxiom(expression); err != nil {
		return err
	}
	c := smtBVExpressionCall{theory: t}
	defer func() { result = errors.Join(result, c.close()) }()
	argument, err := s.manager.GetArg(expression, 0)
	if err != nil {
		return err
	}
	if err = bvAssert(578, "m_util.is_bv_sort(m.get_sort(k))", func() (bool, error) { return s.manager.IsBitVector(argument) }); err != nil {
		return err
	}
	bits, err := c.bits(argument)
	if err != nil {
		return err
	}
	width, err := s.manager.BitVectorSize(argument)
	if err != nil {
		return err
	}
	if uint64(len(bits)) != uint64(width) {
		return ast.ErrASTDomain
	}
	zero, err := c.number(big.NewInt(0))
	if err != nil {
		return err
	}
	args := make([]ast.ASTHandle, 0, len(bits))
	weight := big.NewInt(1)
	for _, bit := range bits {
		number, err := c.number(weight)
		if err != nil {
			return err
		}
		term, err := c.hold(s.manager.MkIte(bit, number, zero))
		if err != nil {
			return err
		}
		args = append(args, term)
		weight.Lsh(weight, 1)
	}
	var sum ast.ASTHandle
	if len(args) == 1 {
		sum = args[0]
	} else {
		sum, err = c.app(smtArithOpAdd, args...)
		if err != nil {
			return err
		}
	}
	// A complete source context always owns this rewriter, even with
	// assertion preprocessing disabled. Standalone consumers can bind it.
	if s.theoryRewriter == nil {
		return ast.ErrASTDomain
	}
	sumRef, err := s.manager.NewRef(sum)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, s.manager.DelRef(&sumRef)) }()
	if err = s.theoryRewriter.Rewrite(sum, sumRef); err != nil {
		return err
	}
	sum, err = s.manager.RefGet(sumRef)
	if err != nil {
		return err
	}
	literal, err := t.MkEqualityLiteral(expression, sum, false)
	if err != nil {
		return err
	}
	if err = t.traceBVBVToIntEquality(expression, sum, literal); err != nil {
		return err
	}
	return t.publishConversionLiteral(literal)
}

func (t *SMTBitVectorTheory) assertIntToBVAxiom(expression ast.ASTHandle) (result error) {
	s, err := t.State()
	if err != nil {
		return err
	}
	if err = bvAssert(644, "get_context().e_internalized(n)", func() (bool, error) { return s.context.enodes[expression] != nil, nil }); err != nil {
		return err
	}
	if err = bvAssert(645, "m_util.is_int2bv(n)", func() (bool, error) {
		op, own, e := t.operation(expression)
		return own && op == 57, e
	}); err != nil {
		return err
	}
	c := smtBVExpressionCall{theory: t}
	defer func() { result = errors.Join(result, c.close()) }()
	argument, err := s.manager.GetArg(expression, 0)
	if err != nil {
		return err
	}
	converted, err := c.hold(s.manager.MkBitVectorToInt(expression))
	if err != nil {
		return err
	}
	width, err := s.manager.BitVectorSize(expression)
	if err != nil {
		return err
	}
	modulus, err := c.number(new(big.Int).Lsh(big.NewInt(1), uint(width)))
	if err != nil {
		return err
	}
	rhs, err := c.app(smtArithOpMod, argument, modulus)
	if err != nil {
		return err
	}
	literal, err := t.MkEqualityLiteral(converted, rhs, false)
	if err != nil {
		return err
	}
	if err = t.traceBVIntToBVMainEquality(converted, rhs); err != nil {
		return err
	}
	if err = t.publishConversionLiteral(literal); err != nil {
		return err
	}
	bits, err := c.bits(expression)
	if err != nil {
		return err
	}
	if uint64(len(bits)) != uint64(width) {
		return ast.ErrASTDomain
	}
	for index, bit := range bits {
		divisor, err := c.number(new(big.Int).Lsh(big.NewInt(1), uint(index)))
		if err != nil {
			return err
		}
		two, err := c.number(big.NewInt(2))
		if err != nil {
			return err
		}
		quotient, err := c.app(smtArithOpIDiv, argument, divisor)
		if err != nil {
			return err
		}
		parity, err := c.app(smtArithOpMod, quotient, two)
		if err != nil {
			return err
		}
		one, err := c.number(big.NewInt(1))
		if err != nil {
			return err
		}
		condition, err := c.hold(s.manager.MkEq(parity, one))
		if err != nil {
			return err
		}
		if err = t.traceBVIntToBVBitEquality(bit, condition); err != nil {
			return err
		}
		if err = t.publishConversionEquality(bit, condition); err != nil {
			return err
		}
	}
	return nil
}

// Source context::mark_as_relevant(literal) marks the underlying Boolean atom,
// independently of the literal sign. Reserved true has no Go bool expression.
func (t *SMTBitVectorTheory) markLiteralRelevant(literal SMTLiteral) error {
	if literal.Var() == SMTTrueBoolVar {
		return nil
	}
	expression, err := t.state.BoolExpression(literal.Var())
	if err != nil {
		return err
	}
	return t.state.markRelevant(expression)
}
