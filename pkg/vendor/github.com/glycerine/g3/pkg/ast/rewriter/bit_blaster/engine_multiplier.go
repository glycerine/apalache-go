package bit_blaster

// Source: src/ast/rewriter/bit_blaster/bit_blaster_tpl_def.h:159-332.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// MkMultiplier uses the retained default ripple multiplier. The optional
// Wallace-tree and Booth flags are never enabled by a retained constructor.
func (e *Engine) MkMultiplier(a, b []ast.ASTHandle) (out []ast.ASTRef, result error) {
	if len(a) == 0 || len(a) != len(b) {
		return nil, ast.ErrASTDomain
	}
	width := len(a)
	bNumber, aNumeral := e.Numeral(a)
	if aNumeral {
		a, b = b, a
	}
	if e.IsMinusOne(b) {
		return e.MkNeg(a)
	}
	if aNumber, yes := e.Numeral(a); yes {
		return e.NumToBits(aNumber.Mul(aNumber, bNumber), width)
	}
	m := e.config.manager
	if _, yes := e.Numeral(b); yes {
		discarded, err := e.constCaseMultiplier(b, a)
		if err != nil {
			return nil, err
		}
		if err = releaseRefs(m, discarded); err != nil {
			return nil, err
		}
	}
	var carries []ast.ASTRef
	defer func() {
		result = errors.Join(result, releaseRefs(m, carries))
		if result != nil {
			result = errors.Join(result, releaseRefs(m, out))
			out = nil
		}
	}()
	low, err := e.gateRef(ast.ASTOpAnd, a[0], b[0])
	if err != nil {
		return nil, err
	}
	out = append(out, low)
	for row := 1; row < width; row++ {
		if err := e.checkpoint(); err != nil {
			return nil, err
		}
		left, err := e.gateRef(ast.ASTOpAnd, a[0], b[row])
		if err != nil {
			return nil, err
		}
		right, err := e.gateRef(ast.ASTOpAnd, a[1], b[row-1])
		if err != nil {
			_ = m.DelRef(&left)
			return nil, err
		}
		leftHandle, err := m.RefGet(left)
		if err != nil {
			_ = m.DelRef(&left)
			_ = m.DelRef(&right)
			return nil, err
		}
		rightHandle, err := m.RefGet(right)
		if err != nil {
			_ = m.DelRef(&left)
			_ = m.DelRef(&right)
			return nil, err
		}
		bitOut, err := m.NewRef(ast.ASTHandle{})
		if err != nil {
			_ = m.DelRef(&left)
			_ = m.DelRef(&right)
			return nil, err
		}
		var nextCarries []ast.ASTRef
		if row+1 < width {
			carry, err := m.NewRef(ast.ASTHandle{})
			if err == nil {
				err = e.MkHalfAdder(leftHandle, rightHandle, bitOut, carry)
			}
			if err != nil {
				_ = m.DelRef(&bitOut)
				_ = m.DelRef(&carry)
				_ = m.DelRef(&left)
				_ = m.DelRef(&right)
				return nil, err
			}
			nextCarries = append(nextCarries, carry)
			for column := 2; column <= row; column++ {
				term, err := e.gateRef(ast.ASTOpAnd, a[column], b[row-column])
				if err != nil {
					_ = releaseRefs(m, nextCarries)
					_ = m.DelRef(&bitOut)
					_ = m.DelRef(&left)
					_ = m.DelRef(&right)
					return nil, err
				}
				termHandle, err := m.RefGet(term)
				if err != nil {
					_ = m.DelRef(&term)
					_ = releaseRefs(m, nextCarries)
					_ = m.DelRef(&bitOut)
					_ = m.DelRef(&left)
					_ = m.DelRef(&right)
					return nil, err
				}
				previous, err := m.RefGet(bitOut)
				if err != nil {
					_ = m.DelRef(&term)
					return nil, err
				}
				carryIn, err := m.RefGet(carries[column-2])
				if err != nil {
					_ = m.DelRef(&term)
					return nil, err
				}
				nextOut, err := m.NewRef(ast.ASTHandle{})
				if err != nil {
					_ = m.DelRef(&term)
					return nil, err
				}
				nextCarry, err := m.NewRef(ast.ASTHandle{})
				if err == nil {
					err = e.MkFullAdder(termHandle, previous, carryIn, nextOut, nextCarry)
				}
				cleanup := m.DelRef(&term)
				if err != nil || cleanup != nil {
					_ = m.DelRef(&nextOut)
					_ = m.DelRef(&nextCarry)
					return nil, errors.Join(err, cleanup)
				}
				if err = m.DelRef(&bitOut); err != nil {
					_ = m.DelRef(&nextOut)
					_ = m.DelRef(&nextCarry)
					return nil, err
				}
				bitOut = nextOut
				nextCarries = append(nextCarries, nextCarry)
			}
		} else {
			if err = e.config.simplify(ast.ASTOpXor, bitOut, leftHandle, rightHandle); err != nil {
				_ = m.DelRef(&bitOut)
				_ = m.DelRef(&left)
				_ = m.DelRef(&right)
				return nil, err
			}
			for column := 2; column <= row; column++ {
				term, err := e.gateRef(ast.ASTOpAnd, a[column], b[row-column])
				if err != nil {
					return nil, err
				}
				termHandle, err := m.RefGet(term)
				if err != nil {
					_ = m.DelRef(&term)
					return nil, err
				}
				previous, err := m.RefGet(bitOut)
				if err != nil {
					_ = m.DelRef(&term)
					return nil, err
				}
				carryIn, err := m.RefGet(carries[column-2])
				if err != nil {
					_ = m.DelRef(&term)
					return nil, err
				}
				nextOut, err := m.NewRef(ast.ASTHandle{})
				if err == nil {
					err = e.config.MkXor3(termHandle, previous, carryIn, nextOut)
				}
				cleanup := m.DelRef(&term)
				if err != nil || cleanup != nil {
					_ = m.DelRef(&nextOut)
					return nil, errors.Join(err, cleanup)
				}
				if err = m.DelRef(&bitOut); err != nil {
					_ = m.DelRef(&nextOut)
					return nil, err
				}
				bitOut = nextOut
			}
		}
		cleanup := errors.Join(m.DelRef(&left), m.DelRef(&right))
		if cleanup != nil {
			_ = m.DelRef(&bitOut)
			_ = releaseRefs(m, nextCarries)
			return nil, cleanup
		}
		out = append(out, bitOut)
		if err = releaseRefs(m, carries); err != nil {
			_ = releaseRefs(m, nextCarries)
			return nil, err
		}
		carries = nextCarries
	}
	return out, nil
}

// constCaseMultiplier reproduces the reached decision-circuit helper. Its
// public source caller discards the result because the helper returns false,
// but constructing it still affects allocation, cancellation, and budgeting.
func (e *Engine) constCaseMultiplier(a, b []ast.ASTHandle) (out []ast.ASTRef, result error) {
	width := len(a)
	caseSize, circuitSize := 1, width*width*5
	for i := 0; caseSize < circuitSize && i < width; i++ {
		if !e.config.manager.IsTrue(a[i]) && !e.config.manager.IsFalse(a[i]) {
			caseSize *= 2
		}
		if !e.config.manager.IsTrue(b[i]) && !e.config.manager.IsFalse(b[i]) {
			caseSize *= 2
		}
	}
	if caseSize >= circuitSize {
		return nil, nil
	}
	aCopy := append([]ast.ASTHandle(nil), a...)
	bCopy := append([]ast.ASTHandle(nil), b...)
	return e.constCaseMultiplierAt(true, 0, aCopy, bCopy)
}

func (e *Engine) constCaseMultiplierAt(inA bool, index int, a, b []ast.ASTHandle) (out []ast.ASTRef, result error) {
	m := e.config.manager
	width := len(a)
	isConstant := func(h ast.ASTHandle) bool { return m.IsTrue(h) || m.IsFalse(h) }
	for inA && index < width && isConstant(a[index]) {
		index++
	}
	if inA && index == width {
		inA, index = false, 0
	}
	for !inA && index < width && isConstant(b[index]) {
		index++
	}
	if index == width {
		aNumber, aOK := e.Numeral(a)
		bNumber, bOK := e.Numeral(b)
		if !aOK || !bOK {
			return nil, ast.ErrASTDomain
		}
		return e.NumToBits(aNumber.Mul(aNumber, bNumber), width)
	}
	x := b[index]
	if inA {
		x = a[index]
	}
	truth, err := m.MkTrue()
	if err != nil {
		return nil, err
	}
	falsehood, err := m.MkFalse()
	if err != nil {
		return nil, err
	}
	if inA {
		a[index] = truth
	} else {
		b[index] = truth
	}
	whenTrue, err := e.constCaseMultiplierAt(inA, index+1, a, b)
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, releaseRefs(m, whenTrue)) }()
	if inA {
		a[index] = falsehood
	} else {
		b[index] = falsehood
	}
	whenFalse, err := e.constCaseMultiplierAt(inA, index+1, a, b)
	if inA {
		a[index] = x
	} else {
		b[index] = x
	}
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, releaseRefs(m, whenFalse)) }()
	trueBits, err := e.handles(whenTrue)
	if err != nil {
		return nil, err
	}
	falseBits, err := e.handles(whenFalse)
	if err != nil {
		return nil, err
	}
	return e.MkMultiplexer(x, trueBits, falseBits)
}
