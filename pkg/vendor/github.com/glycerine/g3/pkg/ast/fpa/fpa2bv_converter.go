package fpa

// Source: src/ast/fpa/fpa2bv_converter.cpp/.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
)

// SpecialValue selects one of the converter's canonical bit-vector encodings.
type SpecialValue uint8

const (
	PositiveInfinity SpecialValue = iota
	NegativeInfinity
	NaN
	PositiveZero
	NegativeZero
)

// Converter owns the state accumulated while converting floating-point terms.
// AST results returned by its methods have the same borrowed-handle contract as
// ASTManager factories.
type Converter struct {
	self              *Converter
	manager           *ast.ASTManager
	family            ast.FamilyID
	constants         map[ast.ASTHandle]ast.ASTHandle
	roundingConstants map[ast.ASTHandle]ast.ASTHandle
	convertedUFs      map[ast.ASTHandle]ast.ASTHandle
	minMaxSigns       map[ast.ASTHandle][2]ast.ASTHandle
	extraAssertions   []ast.ASTHandle
	header            uint64
	highUnspecified   bool
	constantFactory   func(ast.ASTHandle) (ast.ASTHandle, error)
	roundingFactory   func(ast.ASTHandle) (ast.ASTHandle, error)
	closed            bool
}

func NewConverter(m *ast.ASTManager) (owner *Converter, result error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	n := uint64(reflect.TypeOf(Converter{}).Size())
	if err := m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	owner = &Converter{
		manager: m, header: n, highUnspecified: true,
		constants:         make(map[ast.ASTHandle]ast.ASTHandle),
		roundingConstants: make(map[ast.ASTHandle]ast.ASTHandle),
		convertedUFs:      make(map[ast.ASTHandle]ast.ASTHandle),
		minMaxSigns:       make(map[ast.ASTHandle][2]ast.ASTHandle),
	}
	owner.self = owner
	fail := func(err error) (*Converter, error) { return owner, errors.Join(err, owner.Close()) }
	if _, err := m.RegisterArithmeticPlugin(); err != nil {
		return fail(err)
	}
	if _, err := m.RegisterBitVectorPlugin(); err != nil {
		return fail(err)
	}
	var err error
	owner.family, err = m.RegisterFloatingPointPlugin()
	if err != nil {
		return fail(err)
	}
	return owner, nil
}

// SetConstantFactories installs the source theory_fpa converter overrides.
// Nil callbacks retain the standalone tactic converter's fresh-constant path.
// Factories are borrowed and must remain valid until replaced or Close.
func (c *Converter) SetConstantFactories(
	floating func(ast.ASTHandle) (ast.ASTHandle, error),
	rounding func(ast.ASTHandle) (ast.ASTHandle, error),
) error {
	if err := c.open(); err != nil {
		return err
	}
	if len(c.constants) != 0 || len(c.roundingConstants) != 0 {
		return ast.ErrASTDomain
	}
	c.constantFactory = floating
	c.roundingFactory = rounding
	return nil
}

// SetHighUnspecified selects the source converter's deterministic high value
// for otherwise unspecified floating-point conversions.
func (c *Converter) SetHighUnspecified(value bool) error {
	if err := c.open(); err != nil {
		return err
	}
	c.highUnspecified = value
	return nil
}

func (c *Converter) open() error {
	if c == nil || c.self != c || c.manager == nil || c.closed {
		return ast.ErrASTHandle
	}
	_, err := c.manager.ProofMode()
	return err
}

func (c *Converter) Close() error {
	if c == nil || c.self != c || c.closed {
		return nil
	}
	result := c.Reset()
	c.closed = true
	c.manager.Budget().Release(c.header)
	c.header = 0
	c.manager = nil
	c.constantFactory = nil
	c.roundingFactory = nil
	c.self = nil
	return result
}

func (c *Converter) isFloatExpression(h ast.ASTHandle) (bool, error) {
	sort, err := c.manager.GetSort(h)
	if err != nil {
		return false, err
	}
	_, _, matched, err := c.manager.FloatingPointSortShape(sort)
	return matched, err
}

func (c *Converter) isRoundingModeExpression(h ast.ASTHandle) (bool, error) {
	sort, err := c.manager.GetSort(h)
	if err != nil {
		return false, err
	}
	return c.manager.IsRoundingModeSort(sort)
}

// MkFP constructs the converter's three-component floating-point value.
func (c *Converter) MkFP(sign, exponent, significand ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return c.manager.MkFamilyApp(c.family, ast.FPAOpFP, nil, []ast.ASTHandle{sign, exponent, significand})
}

// SplitFP returns the sign, exponent and trailing-significand components.
func (c *Converter) SplitFP(value ast.ASTHandle) (sign, exponent, significand ast.ASTHandle, result error) {
	if err := c.open(); err != nil {
		return sign, exponent, significand, err
	}
	kind, err := c.manager.GetKind(value)
	if err != nil || kind != ast.ASTApp {
		return sign, exponent, significand, ast.ErrASTDomain
	}
	family, err := c.manager.GetFamilyID(value)
	if err != nil {
		return sign, exponent, significand, err
	}
	op, err := c.manager.GetDeclKind(value)
	if err != nil || family != c.family || op != ast.FPAOpFP {
		return sign, exponent, significand, ast.ErrASTDomain
	}
	sign, err = c.manager.GetArg(value, 0)
	if err != nil {
		return sign, exponent, significand, err
	}
	exponent, err = c.manager.GetArg(value, 1)
	if err != nil {
		return sign, exponent, significand, err
	}
	significand, err = c.manager.GetArg(value, 2)
	return sign, exponent, significand, err
}

// JoinFP packs sign, exponent and significand into IEEE field order.
func (c *Converter) JoinFP(value ast.ASTHandle) (ast.ASTHandle, error) {
	sign, exponent, significand, err := c.SplitFP(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.manager.MkBitVectorConcat([]ast.ASTHandle{sign, exponent, significand})
}

func (c *Converter) splitRoundingMode(value ast.ASTHandle) (ast.ASTHandle, error) {
	family, err := c.manager.GetFamilyID(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	op, err := c.manager.GetDeclKind(value)
	if err != nil || family != c.family || op != ast.FPAOpBV2RM {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return c.manager.GetArg(value, 0)
}

func (c *Converter) wrapRoundingMode(value ast.ASTHandle) (ast.ASTHandle, error) {
	return c.manager.MkFamilyApp(c.family, ast.FPAOpBV2RM, nil, []ast.ASTHandle{value})
}

func (c *Converter) boolEq(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	equal, err := c.manager.AreEqual(left, right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if equal {
		return c.manager.MkTrue()
	}
	distinct, err := c.manager.AreDistinct(left, right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if distinct {
		return c.manager.MkFalse()
	}
	return c.manager.MkEq(left, right)
}

func (c *Converter) boolNot(value ast.ASTHandle) (ast.ASTHandle, error) {
	return c.manager.MkNotSimplified(value)
}

func (c *Converter) boolAnd(values ...ast.ASTHandle) (ast.ASTHandle, error) {
	result := values[:0]
	for _, value := range values {
		if c.manager.IsFalse(value) {
			return c.manager.MkFalse()
		}
		if !c.manager.IsTrue(value) {
			result = append(result, value)
		}
	}
	return c.manager.MkAndArray(result)
}

func (c *Converter) boolOr(values ...ast.ASTHandle) (ast.ASTHandle, error) {
	result := values[:0]
	for _, value := range values {
		if c.manager.IsTrue(value) {
			return c.manager.MkTrue()
		}
		if !c.manager.IsFalse(value) {
			result = append(result, value)
		}
	}
	return c.manager.MkOrArray(result)
}

func (c *Converter) boolXor(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	if c.manager.IsFalse(left) {
		return right, nil
	}
	if c.manager.IsFalse(right) {
		return left, nil
	}
	if c.manager.IsTrue(left) {
		return c.boolNot(right)
	}
	if c.manager.IsTrue(right) {
		return c.boolNot(left)
	}
	if left == right {
		return c.manager.MkFalse()
	}
	return c.manager.MkXor(left, right)
}

func (c *Converter) boolITE(condition, thenValue, elseValue ast.ASTHandle) (ast.ASTHandle, error) {
	if c.manager.IsTrue(condition) || thenValue == elseValue {
		return thenValue, nil
	}
	if c.manager.IsFalse(condition) {
		return elseValue, nil
	}
	return c.manager.MkIte(condition, thenValue, elseValue)
}

func (c *Converter) isNaN(value ast.ASTHandle) (ast.ASTHandle, error) {
	_, exponent, significand, err := c.SplitFP(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ew, err := c.manager.BitVectorSize(exponent)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sw, err := c.manager.BitVectorSize(significand)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	top, err := c.manager.BitVectorNumeralAllOnes(ew)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	zero, err := c.manager.BitVectorNumeralUnsigned(0, sw)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	expTop, err := c.boolEq(exponent, top)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sigZero, err := c.boolEq(significand, zero)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sigNonzero, err := c.boolNot(sigZero)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.boolAnd(expTop, sigNonzero)
}

// MkEq restores SMT floating-point equality over the component encoding. All
// NaN payloads denote the theory's single NaN value.
func (c *Converter) MkEq(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	leftFloat, err := c.isFloatExpression(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rightFloat, err := c.isFloatExpression(right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if leftFloat && rightFloat {
		ls, le, lm, err := c.SplitFP(left)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		rs, re, rm, err := c.SplitFP(right)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		eqs, err := c.boolEq(ls, rs)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		eqe, err := c.boolEq(le, re)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		eqm, err := c.boolEq(lm, rm)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		same, err := c.boolAnd(eqs, eqe, eqm)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		ln, err := c.isNaN(left)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		rn, err := c.isNaN(right)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		bothNaN, err := c.boolAnd(ln, rn)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return c.boolOr(bothNaN, same)
	}
	leftRM, err := c.isRoundingModeExpression(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rightRM, err := c.isRoundingModeExpression(right)
	if err != nil || !leftRM || !rightRM {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	a, err := c.splitRoundingMode(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	b, err := c.splitRoundingMode(right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.boolEq(a, b)
}

// MkITE applies a condition independently to encoded FP fields or RM bits.
func (c *Converter) MkITE(condition, thenValue, elseValue ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	float, err := c.isFloatExpression(thenValue)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if float {
		ts, te, tm, err := c.SplitFP(thenValue)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		fs, fe, fm, err := c.SplitFP(elseValue)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		s, err := c.boolITE(condition, ts, fs)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		e, err := c.boolITE(condition, te, fe)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		m, err := c.boolITE(condition, tm, fm)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return c.MkFP(s, e, m)
	}
	t, err := c.splitRoundingMode(thenValue)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	f, err := c.splitRoundingMode(elseValue)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	bits, err := c.boolITE(condition, t, f)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.wrapRoundingMode(bits)
}

// MkDistinct expands distinct using the converter's NaN-aware equality.
func (c *Converter) MkDistinct(arguments []ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	terms := make([]ast.ASTHandle, 0, len(arguments)*(len(arguments)-1)/2)
	for i := 0; i < len(arguments); i++ {
		for j := i + 1; j < len(arguments); j++ {
			equal, err := c.MkEq(arguments[i], arguments[j])
			if err != nil {
				return ast.ASTHandle{}, err
			}
			unequal, err := c.boolNot(equal)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			terms = append(terms, unequal)
		}
	}
	return c.boolAnd(terms...)
}

// MkRoundingMode maps an FPA declaration kind to the source three-bit code.
func (c *Converter) MkRoundingMode(kind int32) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	var value uint32
	switch kind {
	case ast.FPAOpRMNearestEven:
		value = 0
	case ast.FPAOpRMNearestAway:
		value = 1
	case ast.FPAOpRMTowardPositive:
		value = 2
	case ast.FPAOpRMTowardNegative:
		value = 3
	case ast.FPAOpRMTowardZero:
		value = 4
	default:
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	bits, err := c.manager.BitVectorNumeralUnsigned(value, 3)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.wrapRoundingMode(bits)
}

// MkSpecial creates the canonical converter encoding for an FP special value.
func (c *Converter) MkSpecial(sort ast.ASTHandle, value SpecialValue) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	ebits, sbits, matched, err := c.manager.FloatingPointSortShape(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !matched {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	var sign, significand uint32
	topExponent := false
	switch value {
	case PositiveInfinity:
		topExponent = true
	case NegativeInfinity:
		sign, topExponent = 1, true
	case NaN:
		topExponent, significand = true, 1
	case PositiveZero:
	case NegativeZero:
		sign = 1
	default:
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	s, err := c.manager.BitVectorNumeralUnsigned(sign, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var e ast.ASTHandle
	if topExponent {
		e, err = c.manager.BitVectorNumeralAllOnes(ebits)
	} else {
		e, err = c.manager.BitVectorNumeralUnsigned(0, ebits)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	m, err := c.manager.BitVectorNumeralUnsigned(significand, sbits-1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.MkFP(s, e, m)
}
