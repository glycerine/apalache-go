package bit_blaster

// Source: src/ast/rewriter/bit_blaster/bit_blaster_tpl.h:78-130;
// src/ast/rewriter/bit_blaster/bit_blaster_tpl_def.h:511-539,826-907,1122-1178.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (e *Engine) copyBits(bits []ast.ASTHandle) (out []ast.ASTRef, result error) {
	defer func() {
		if result != nil {
			result = errors.Join(result, releaseRefs(e.config.manager, out))
			out = nil
		}
	}()
	for _, bit := range bits {
		ref, err := e.config.manager.NewRef(bit)
		if err != nil {
			return nil, err
		}
		out = append(out, ref)
	}
	return out, nil
}

func (e *Engine) gateRef(operation int32, args ...ast.ASTHandle) (ref ast.ASTRef, result error) {
	ref, err := e.config.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, err
	}
	if err = e.config.simplify(operation, ref, args...); err != nil {
		return ast.ASTRef{}, errors.Join(err, e.config.manager.DelRef(&ref))
	}
	return ref, nil
}

func (e *Engine) MkMultiplexer(condition ast.ASTHandle, thenBits, elseBits []ast.ASTHandle) (out []ast.ASTRef, result error) {
	if len(thenBits) != len(elseBits) {
		return nil, ast.ErrASTDomain
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, releaseRefs(e.config.manager, out))
			out = nil
		}
	}()
	for i := range thenBits {
		ref, err := e.gateRef(ast.ASTOpIte, condition, thenBits[i], elseBits[i])
		if err != nil {
			return nil, err
		}
		out = append(out, ref)
	}
	return out, nil
}

func (e *Engine) MkAbs(bits []ast.ASTHandle) (out []ast.ASTRef, result error) {
	if len(bits) == 0 {
		return nil, ast.ErrASTDomain
	}
	most := bits[len(bits)-1]
	if e.config.manager.IsFalse(most) {
		return e.copyBits(bits)
	}
	if e.config.manager.IsTrue(most) {
		return e.MkNeg(bits)
	}
	negative, err := e.MkNeg(bits)
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, releaseRefs(e.config.manager, negative)) }()
	negativeBits, err := e.handles(negative)
	if err != nil {
		return nil, err
	}
	return e.MkMultiplexer(most, negativeBits, bits)
}

func (e *Engine) MkEq(a, b []ast.ASTHandle, out ast.ASTRef) (result error) {
	if len(a) == 0 || len(a) != len(b) {
		return ast.ErrASTDomain
	}
	terms := make([]ast.ASTRef, 0, len(a))
	defer func() { result = errors.Join(result, releaseRefs(e.config.manager, terms)) }()
	for i := range a {
		term, err := e.gateRef(ast.ASTOpIff, a[i], b[i])
		if err != nil {
			return err
		}
		terms = append(terms, term)
	}
	handles, err := e.handles(terms)
	if err != nil {
		return err
	}
	return e.config.simplify(ast.ASTOpAnd, out, handles...)
}

func (e *Engine) MkRotateLeft(bits []ast.ASTHandle, amount uint64) ([]ast.ASTRef, error) {
	if len(bits) == 0 {
		return nil, ast.ErrASTDomain
	}
	n := int(amount % uint64(len(bits)))
	rotated := make([]ast.ASTHandle, 0, len(bits))
	rotated = append(rotated, bits[len(bits)-n:]...)
	rotated = append(rotated, bits[:len(bits)-n]...)
	return e.copyBits(rotated)
}

func (e *Engine) MkRotateRight(bits []ast.ASTHandle, amount uint64) ([]ast.ASTRef, error) {
	if len(bits) == 0 {
		return nil, ast.ErrASTDomain
	}
	n := amount % uint64(len(bits))
	return e.MkRotateLeft(bits, uint64(len(bits))-n)
}

func (e *Engine) MkSignExtend(bits []ast.ASTHandle, extra int) ([]ast.ASTRef, error) {
	if len(bits) == 0 || extra < 0 {
		return nil, ast.ErrASTDomain
	}
	extended := make([]ast.ASTHandle, len(bits), len(bits)+extra)
	copy(extended, bits)
	for i := 0; i < extra; i++ {
		extended = append(extended, bits[len(bits)-1])
	}
	return e.copyBits(extended)
}

func (e *Engine) MkZeroExtend(bits []ast.ASTHandle, extra int) ([]ast.ASTRef, error) {
	if len(bits) == 0 || extra < 0 {
		return nil, ast.ErrASTDomain
	}
	falsehood, err := e.config.manager.MkFalse()
	if err != nil {
		return nil, err
	}
	extended := make([]ast.ASTHandle, len(bits), len(bits)+extra)
	copy(extended, bits)
	for i := 0; i < extra; i++ {
		extended = append(extended, falsehood)
	}
	return e.copyBits(extended)
}

func (e *Engine) MkIsEq(bits []ast.ASTHandle, value uint64, out ast.ASTRef) (result error) {
	if len(bits) == 0 {
		return ast.ErrASTDomain
	}
	terms := make([]ast.ASTRef, 0, len(bits))
	defer func() { result = errors.Join(result, releaseRefs(e.config.manager, terms)) }()
	for i, bit := range bits {
		var term ast.ASTRef
		var err error
		if i < 64 && value&(uint64(1)<<i) != 0 {
			term, err = e.config.manager.NewRef(bit)
		} else {
			term, err = e.gateRef(ast.ASTOpNot, bit)
		}
		if err != nil {
			return err
		}
		terms = append(terms, term)
	}
	handles, err := e.handles(terms)
	if err != nil {
		return err
	}
	return e.config.simplify(ast.ASTOpAnd, out, handles...)
}

func (e *Engine) MkEqs(bits []ast.ASTHandle) (out []ast.ASTRef, result error) {
	if len(bits) == 0 {
		return nil, ast.ErrASTDomain
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, releaseRefs(e.config.manager, out))
			out = nil
		}
	}()
	for i := range bits {
		ref, err := e.config.manager.NewRef(ast.ASTHandle{})
		if err != nil {
			return nil, err
		}
		if err = e.MkIsEq(bits, uint64(i), ref); err != nil {
			_ = e.config.manager.DelRef(&ref)
			return nil, err
		}
		out = append(out, ref)
	}
	return out, nil
}

func (e *Engine) mapUnary(operation int32, bits []ast.ASTHandle) (out []ast.ASTRef, result error) {
	defer func() {
		if result != nil {
			result = errors.Join(result, releaseRefs(e.config.manager, out))
			out = nil
		}
	}()
	for _, bit := range bits {
		ref, err := e.gateRef(operation, bit)
		if err != nil {
			return nil, err
		}
		out = append(out, ref)
	}
	return out, nil
}

func (e *Engine) mapBinary(operation int32, negate bool, a, b []ast.ASTHandle) (out []ast.ASTRef, result error) {
	if len(a) != len(b) {
		return nil, ast.ErrASTDomain
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, releaseRefs(e.config.manager, out))
			out = nil
		}
	}()
	for i := range a {
		ref, err := e.gateRef(operation, a[i], b[i])
		if err != nil {
			return nil, err
		}
		if negate {
			h, err := e.config.manager.RefGet(ref)
			if err != nil {
				_ = e.config.manager.DelRef(&ref)
				return nil, err
			}
			negative, err := e.gateRef(ast.ASTOpNot, h)
			cleanup := e.config.manager.DelRef(&ref)
			if err != nil || cleanup != nil {
				if err == nil {
					_ = e.config.manager.DelRef(&negative)
				}
				return nil, errors.Join(err, cleanup)
			}
			ref = negative
		}
		out = append(out, ref)
	}
	return out, nil
}

func (e *Engine) MkNot(bits []ast.ASTHandle) ([]ast.ASTRef, error) {
	return e.mapUnary(ast.ASTOpNot, bits)
}
func (e *Engine) MkAnd(a, b []ast.ASTHandle) ([]ast.ASTRef, error) {
	return e.mapBinary(ast.ASTOpAnd, false, a, b)
}
func (e *Engine) MkOr(a, b []ast.ASTHandle) ([]ast.ASTRef, error) {
	return e.mapBinary(ast.ASTOpOr, false, a, b)
}
func (e *Engine) MkXor(a, b []ast.ASTHandle) ([]ast.ASTRef, error) {
	return e.mapBinary(ast.ASTOpXor, false, a, b)
}
func (e *Engine) MkXnor(a, b []ast.ASTHandle) ([]ast.ASTRef, error) {
	return e.mapBinary(ast.ASTOpIff, false, a, b)
}
func (e *Engine) MkNand(a, b []ast.ASTHandle) ([]ast.ASTRef, error) {
	return e.mapBinary(ast.ASTOpAnd, true, a, b)
}
func (e *Engine) MkNor(a, b []ast.ASTHandle) ([]ast.ASTRef, error) {
	return e.mapBinary(ast.ASTOpOr, true, a, b)
}

func (e *Engine) MkReductionAnd(bits []ast.ASTHandle, out ast.ASTRef) error {
	return e.config.simplify(ast.ASTOpAnd, out, bits...)
}
func (e *Engine) MkReductionOr(bits []ast.ASTHandle, out ast.ASTRef) error {
	return e.config.simplify(ast.ASTOpOr, out, bits...)
}
func (e *Engine) MkComp(a, b []ast.ASTHandle, out ast.ASTRef) error {
	return e.MkEq(a, b, out)
}

func (e *Engine) MkCarrySaveAdder(a, b, c []ast.ASTHandle) (sum, carry []ast.ASTRef, result error) {
	if len(a) != len(b) || len(a) != len(c) {
		return nil, nil, ast.ErrASTDomain
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, releaseRefs(e.config.manager, sum), releaseRefs(e.config.manager, carry))
			sum, carry = nil, nil
		}
	}()
	for i := range a {
		sumRef, err := e.config.manager.NewRef(ast.ASTHandle{})
		if err != nil {
			return nil, nil, err
		}
		if err = e.config.MkXor3(a[i], b[i], c[i], sumRef); err != nil {
			_ = e.config.manager.DelRef(&sumRef)
			return nil, nil, err
		}
		carryRef, err := e.config.manager.NewRef(ast.ASTHandle{})
		if err == nil {
			err = e.config.MkCarry(a[i], b[i], c[i], carryRef)
		}
		if err != nil {
			_ = e.config.manager.DelRef(&sumRef)
			if carryRef != (ast.ASTRef{}) {
				_ = e.config.manager.DelRef(&carryRef)
			}
			return nil, nil, err
		}
		sum = append(sum, sumRef)
		carry = append(carry, carryRef)
	}
	return sum, carry, nil
}
