package bit_blaster

// Source: src/ast/rewriter/bit_blaster/bit_blaster_tpl_def.h:659-825.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func releaseGroups(m *ast.ASTManager, groups ...[]ast.ASTRef) (result error) {
	for _, refs := range groups {
		result = errors.Join(result, releaseRefs(m, refs))
	}
	return result
}

func (e *Engine) MkSignedDiv(a, b []ast.ASTHandle) (out []ast.ASTRef, result error) {
	if len(a) == 0 || len(a) != len(b) {
		return nil, ast.ErrASTDomain
	}
	m := e.config.manager
	aSign, bSign := a[len(a)-1], b[len(b)-1]
	var owned [][]ast.ASTRef
	defer func() {
		result = errors.Join(result, releaseGroups(m, owned...))
		if result != nil {
			result = errors.Join(result, releaseRefs(m, out))
			out = nil
		}
	}()
	negate := func(bits []ast.ASTHandle) ([]ast.ASTHandle, error) {
		refs, err := e.MkNeg(bits)
		if err != nil {
			return nil, err
		}
		owned = append(owned, refs)
		return e.handles(refs)
	}
	if m.IsFalse(aSign) && m.IsFalse(bSign) {
		return e.MkUnsignedDiv(a, b)
	}
	if m.IsFalse(aSign) && m.IsTrue(bSign) {
		negativeB, err := negate(b)
		if err != nil {
			return nil, err
		}
		quotient, err := e.MkUnsignedDiv(a, negativeB)
		if err != nil {
			return nil, err
		}
		owned = append(owned, quotient)
		quotientBits, err := e.handles(quotient)
		if err != nil {
			return nil, err
		}
		return e.MkNeg(quotientBits)
	}
	if m.IsTrue(aSign) && m.IsFalse(bSign) {
		negativeA, err := negate(a)
		if err != nil {
			return nil, err
		}
		quotient, err := e.MkUnsignedDiv(negativeA, b)
		if err != nil {
			return nil, err
		}
		owned = append(owned, quotient)
		quotientBits, err := e.handles(quotient)
		if err != nil {
			return nil, err
		}
		return e.MkNeg(quotientBits)
	}
	if m.IsTrue(aSign) && m.IsTrue(bSign) {
		negativeA, err := negate(a)
		if err != nil {
			return nil, err
		}
		negativeB, err := negate(b)
		if err != nil {
			return nil, err
		}
		return e.MkUnsignedDiv(negativeA, negativeB)
	}
	absA, err := e.MkAbs(a)
	if err != nil {
		return nil, err
	}
	owned = append(owned, absA)
	absB, err := e.MkAbs(b)
	if err != nil {
		return nil, err
	}
	owned = append(owned, absB)
	absABits, err := e.handles(absA)
	if err != nil {
		return nil, err
	}
	absBBits, err := e.handles(absB)
	if err != nil {
		return nil, err
	}
	quotient, err := e.MkUnsignedDiv(absABits, absBBits)
	if err != nil {
		return nil, err
	}
	owned = append(owned, quotient)
	quotientBits, err := e.handles(quotient)
	if err != nil {
		return nil, err
	}
	negativeQuotient, err := e.MkNeg(quotientBits)
	if err != nil {
		return nil, err
	}
	owned = append(owned, negativeQuotient)
	sameSign, err := e.gateRef(ast.ASTOpIff, aSign, bSign)
	if err != nil {
		return nil, err
	}
	owned = append(owned, []ast.ASTRef{sameSign})
	condition, err := m.RefGet(sameSign)
	if err != nil {
		return nil, err
	}
	negativeQuotientBits, err := e.handles(negativeQuotient)
	if err != nil {
		return nil, err
	}
	return e.MkMultiplexer(condition, quotientBits, negativeQuotientBits)
}

func (e *Engine) MkSignedRem(a, b []ast.ASTHandle) (out []ast.ASTRef, result error) {
	if len(a) == 0 || len(a) != len(b) {
		return nil, ast.ErrASTDomain
	}
	m := e.config.manager
	aSign, bSign := a[len(a)-1], b[len(b)-1]
	var owned [][]ast.ASTRef
	defer func() {
		result = errors.Join(result, releaseGroups(m, owned...))
		if result != nil {
			result = errors.Join(result, releaseRefs(m, out))
			out = nil
		}
	}()
	negate := func(bits []ast.ASTHandle) ([]ast.ASTHandle, error) {
		refs, err := e.MkNeg(bits)
		if err != nil {
			return nil, err
		}
		owned = append(owned, refs)
		return e.handles(refs)
	}
	if m.IsFalse(aSign) && m.IsFalse(bSign) {
		return e.MkUnsignedRem(a, b)
	}
	if m.IsFalse(aSign) && m.IsTrue(bSign) {
		negativeB, err := negate(b)
		if err != nil {
			return nil, err
		}
		return e.MkUnsignedRem(a, negativeB)
	}
	if m.IsTrue(aSign) && m.IsFalse(bSign) {
		negativeA, err := negate(a)
		if err != nil {
			return nil, err
		}
		remainder, err := e.MkUnsignedRem(negativeA, b)
		if err != nil {
			return nil, err
		}
		owned = append(owned, remainder)
		remainderBits, err := e.handles(remainder)
		if err != nil {
			return nil, err
		}
		return e.MkNeg(remainderBits)
	}
	if m.IsTrue(aSign) && m.IsTrue(bSign) {
		negativeA, err := negate(a)
		if err != nil {
			return nil, err
		}
		negativeB, err := negate(b)
		if err != nil {
			return nil, err
		}
		remainder, err := e.MkUnsignedRem(negativeA, negativeB)
		if err != nil {
			return nil, err
		}
		owned = append(owned, remainder)
		remainderBits, err := e.handles(remainder)
		if err != nil {
			return nil, err
		}
		return e.MkNeg(remainderBits)
	}
	absA, err := e.MkAbs(a)
	if err != nil {
		return nil, err
	}
	owned = append(owned, absA)
	absB, err := e.MkAbs(b)
	if err != nil {
		return nil, err
	}
	owned = append(owned, absB)
	absAHandles, err := e.handles(absA)
	if err != nil {
		return nil, err
	}
	absBHandles, err := e.handles(absB)
	if err != nil {
		return nil, err
	}
	var remainder []ast.ASTRef
	if numeral, yes := e.Numeral(absBHandles); yes && positivePowerOfTwo(numeral) {
		shift := numeral.BitLen() - 1
		if shift == 0 {
			remainder, err = e.NumToBits(big.NewInt(0), len(a))
		} else {
			remainder, err = e.MkZeroExtend(absAHandles[:shift], len(a)-shift)
		}
	} else {
		remainder, err = e.MkUnsignedRem(absAHandles, absBHandles)
	}
	if err != nil {
		return nil, err
	}
	owned = append(owned, remainder)
	remainderBits, err := e.handles(remainder)
	if err != nil {
		return nil, err
	}
	negativeRemainder, err := e.MkNeg(remainderBits)
	if err != nil {
		return nil, err
	}
	owned = append(owned, negativeRemainder)
	negativeRemainderBits, err := e.handles(negativeRemainder)
	if err != nil {
		return nil, err
	}
	return e.MkMultiplexer(aSign, negativeRemainderBits, remainderBits)
}

func positivePowerOfTwo(value *big.Int) bool {
	if value == nil || value.Sign() <= 0 {
		return false
	}
	minusOne := new(big.Int).Sub(new(big.Int).Set(value), big.NewInt(1))
	return new(big.Int).And(new(big.Int).Set(value), minusOne).Sign() == 0
}

func (e *Engine) MkSignedMod(a, b []ast.ASTHandle) (out []ast.ASTRef, result error) {
	if len(a) == 0 || len(a) != len(b) {
		return nil, ast.ErrASTDomain
	}
	m := e.config.manager
	var owned [][]ast.ASTRef
	defer func() {
		result = errors.Join(result, releaseGroups(m, owned...))
		if result != nil {
			result = errors.Join(result, releaseRefs(m, out))
			out = nil
		}
	}()
	keep := func(refs []ast.ASTRef, err error) ([]ast.ASTHandle, error) {
		if err != nil {
			return nil, err
		}
		owned = append(owned, refs)
		return e.handles(refs)
	}
	absA, err := keep(e.MkAbs(a))
	if err != nil {
		return nil, err
	}
	absB, err := keep(e.MkAbs(b))
	if err != nil {
		return nil, err
	}
	u, err := keep(e.MkUnsignedRem(absA, absB))
	if err != nil {
		return nil, err
	}
	negativeU, err := keep(e.MkNeg(u))
	if err != nil {
		return nil, err
	}
	negativeUPlusB, err := keep(e.MkAdder(negativeU, b))
	if err != nil {
		return nil, err
	}
	uPlusB, err := keep(e.MkAdder(u, b))
	if err != nil {
		return nil, err
	}
	zeroRefs, err := e.NumToBits(big.NewInt(0), len(a))
	zero, err := keep(zeroRefs, err)
	if err != nil {
		return nil, err
	}
	uIsZero, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, err
	}
	owned = append(owned, []ast.ASTRef{uIsZero})
	if err = e.MkEq(u, zero, uIsZero); err != nil {
		return nil, err
	}
	equalZero, err := m.RefGet(uIsZero)
	if err != nil {
		return nil, err
	}
	aSign, bSign := a[len(a)-1], b[len(b)-1]
	ite1, err := keep(e.MkMultiplexer(bSign, negativeU, negativeUPlusB))
	if err != nil {
		return nil, err
	}
	ite2, err := keep(e.MkMultiplexer(bSign, uPlusB, u))
	if err != nil {
		return nil, err
	}
	body, err := keep(e.MkMultiplexer(aSign, ite1, ite2))
	if err != nil {
		return nil, err
	}
	return e.MkMultiplexer(equalZero, u, body)
}
