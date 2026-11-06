package bit_blaster

// Source: src/ast/rewriter/bit_blaster/bit_blaster_tpl.h:22-77;
// src/ast/rewriter/bit_blaster/bit_blaster_tpl_def.h:27-157.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/messages"
)

// Engine is the concrete AST specialization of bit_blaster_tpl. Inputs borrow
// their handles. Returned reference slices own their expressions and must be
// released by the caller.
type Engine struct {
	config      *Config
	maxMemory   uint64
	cooperation *cooperate.Cooperation
}

func NewEngine(config *Config) (*Engine, error) {
	if config == nil || config.manager == nil {
		return nil, ast.ErrASTHandle
	}
	return &Engine{config: config, maxMemory: ^uint64(0)}, nil
}

func (e *Engine) SetMaxMemoryMB(megabytes uint32) error {
	if e == nil || e.config == nil || e.config.manager == nil {
		return ast.ErrASTHandle
	}
	e.maxMemory = memory.MegabytesToBytes(megabytes)
	return nil
}

func (e *Engine) checkpoint() error {
	if e == nil || e.config == nil || e.config.manager == nil {
		return ast.ErrASTHandle
	}
	runtime := e.config.manager.NumeralService().RuntimeOwner()
	used, err := runtime.AllocationSize()
	if err != nil {
		return err
	}
	if used > e.maxMemory {
		return rewriter.NewRewriterException(messages.MaxMemory)
	}
	canceled, err := e.config.manager.Canceled()
	if err != nil {
		return err
	}
	if canceled {
		return rewriter.NewRewriterException(e.config.manager.ResourceLimit().CancelMessage())
	}
	if e.cooperation != nil {
		return e.cooperation.Cooperate("bit-blaster")
	}
	return nil
}

func (e *Engine) BindCooperation(owner *cooperate.Cooperation) error {
	if e == nil || e.config == nil || owner == nil {
		return ast.ErrASTHandle
	}
	if _, err := owner.Enabled(); err != nil {
		return err
	}
	e.cooperation = owner
	return nil
}

func releaseRefs(m *ast.ASTManager, refs []ast.ASTRef) (result error) {
	for i := range refs {
		if refs[i] != (ast.ASTRef{}) {
			result = errors.Join(result, m.DelRef(&refs[i]))
		}
	}
	return result
}

func (e *Engine) handles(refs []ast.ASTRef) ([]ast.ASTHandle, error) {
	result := make([]ast.ASTHandle, len(refs))
	for i := range refs {
		h, err := e.config.manager.RefGet(refs[i])
		if err != nil {
			return nil, err
		}
		result[i] = h
	}
	return result, nil
}

// Numeral recognizes a little-endian Boolean bit vector and returns its
// nonnegative value. The returned integer is independent of the engine.
func (e *Engine) Numeral(bits []ast.ASTHandle) (*big.Int, bool) {
	value := new(big.Int)
	for i, bit := range bits {
		switch {
		case e.config.manager.IsTrue(bit):
			value.SetBit(value, i, 1)
		case e.config.manager.IsFalse(bit):
		default:
			return value, false
		}
	}
	return value, true
}

func (e *Engine) IsMinusOne(bits []ast.ASTHandle) bool {
	for _, bit := range bits {
		if !e.config.manager.IsTrue(bit) {
			return false
		}
	}
	return true
}

// NumToBits returns the low width bits of a nonnegative integer, least
// significant bit first, matching _num2bits in the source.
func (e *Engine) NumToBits(value *big.Int, width int) (out []ast.ASTRef, result error) {
	if value == nil || value.Sign() < 0 || width < 0 {
		return nil, ast.ErrASTDomain
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, releaseRefs(e.config.manager, out))
			out = nil
		}
	}()
	for i := 0; i < width; i++ {
		var h ast.ASTHandle
		var err error
		if value.Bit(i) == 0 {
			h, err = e.config.manager.MkFalse()
		} else {
			h, err = e.config.manager.MkTrue()
		}
		if err != nil {
			return nil, err
		}
		ref, err := e.config.manager.NewRef(h)
		if err != nil {
			return nil, err
		}
		out = append(out, ref)
	}
	return out, nil
}

func (e *Engine) MkHalfAdder(a, b ast.ASTHandle, out, carry ast.ASTRef) error {
	if err := e.config.simplify(ast.ASTOpXor, out, a, b); err != nil {
		return err
	}
	return e.config.simplify(ast.ASTOpAnd, carry, a, b)
}

func (e *Engine) MkFullAdder(a, b, carryIn ast.ASTHandle, out, carry ast.ASTRef) error {
	if err := e.config.MkXor3(a, b, carryIn, out); err != nil {
		return err
	}
	return e.config.MkCarry(a, b, carryIn, carry)
}

func (e *Engine) MkNeg(bits []ast.ASTHandle) (out []ast.ASTRef, result error) {
	if len(bits) == 0 {
		return nil, ast.ErrASTDomain
	}
	m := e.config.manager
	truth, err := m.MkTrue()
	if err != nil {
		return nil, err
	}
	carry, err := m.NewRef(truth)
	if err != nil {
		return nil, err
	}
	defer func() {
		if carry != (ast.ASTRef{}) {
			result = errors.Join(result, m.DelRef(&carry))
		}
		if result != nil {
			result = errors.Join(result, releaseRefs(m, out))
			out = nil
		}
	}()
	for i, bit := range bits {
		notBit, err := m.NewRef(ast.ASTHandle{})
		if err != nil {
			return nil, err
		}
		if err = e.config.simplify(ast.ASTOpNot, notBit, bit); err != nil {
			_ = m.DelRef(&notBit)
			return nil, err
		}
		notHandle, err := m.RefGet(notBit)
		if err != nil {
			_ = m.DelRef(&notBit)
			return nil, err
		}
		carryHandle, err := m.RefGet(carry)
		if err != nil {
			_ = m.DelRef(&notBit)
			return nil, err
		}
		bitOut, err := m.NewRef(ast.ASTHandle{})
		if err != nil {
			_ = m.DelRef(&notBit)
			return nil, err
		}
		var nextCarry ast.ASTRef
		if i+1 == len(bits) {
			err = e.config.simplify(ast.ASTOpXor, bitOut, notHandle, carryHandle)
		} else {
			nextCarry, err = m.NewRef(ast.ASTHandle{})
			if err == nil {
				err = e.MkHalfAdder(notHandle, carryHandle, bitOut, nextCarry)
			}
		}
		cleanup := m.DelRef(&notBit)
		if err != nil || cleanup != nil {
			_ = m.DelRef(&bitOut)
			if nextCarry != (ast.ASTRef{}) {
				_ = m.DelRef(&nextCarry)
			}
			return nil, errors.Join(err, cleanup)
		}
		out = append(out, bitOut)
		if err = m.DelRef(&carry); err != nil {
			if nextCarry != (ast.ASTRef{}) {
				_ = m.DelRef(&nextCarry)
			}
			return nil, err
		}
		carry = nextCarry
	}
	return out, nil
}

func (e *Engine) MkAdder(a, b []ast.ASTHandle) (out []ast.ASTRef, result error) {
	if len(a) == 0 || len(a) != len(b) {
		return nil, ast.ErrASTDomain
	}
	m := e.config.manager
	falsehood, err := m.MkFalse()
	if err != nil {
		return nil, err
	}
	carry, err := m.NewRef(falsehood)
	if err != nil {
		return nil, err
	}
	defer func() {
		if carry != (ast.ASTRef{}) {
			result = errors.Join(result, m.DelRef(&carry))
		}
		if result != nil {
			result = errors.Join(result, releaseRefs(m, out))
			out = nil
		}
	}()
	for i := range a {
		carryHandle, err := m.RefGet(carry)
		if err != nil {
			return nil, err
		}
		bitOut, err := m.NewRef(ast.ASTHandle{})
		if err != nil {
			return nil, err
		}
		var nextCarry ast.ASTRef
		if i+1 == len(a) {
			err = e.config.MkXor3(a[i], b[i], carryHandle, bitOut)
		} else {
			nextCarry, err = m.NewRef(ast.ASTHandle{})
			if err == nil {
				err = e.MkFullAdder(a[i], b[i], carryHandle, bitOut, nextCarry)
			}
		}
		if err != nil {
			_ = m.DelRef(&bitOut)
			if nextCarry != (ast.ASTRef{}) {
				_ = m.DelRef(&nextCarry)
			}
			return nil, err
		}
		out = append(out, bitOut)
		if err = m.DelRef(&carry); err != nil {
			if nextCarry != (ast.ASTRef{}) {
				_ = m.DelRef(&nextCarry)
			}
			return nil, err
		}
		carry = nextCarry
	}
	return out, nil
}

// MkSubtracter computes a-b modulo 2^len(a). The returned carry is true iff
// unsigned a is at least b. Both the bit refs and carry ref are caller-owned.
func (e *Engine) MkSubtracter(a, b []ast.ASTHandle) (out []ast.ASTRef, carry ast.ASTRef, result error) {
	if len(a) == 0 || len(a) != len(b) {
		return nil, ast.ASTRef{}, ast.ErrASTDomain
	}
	m := e.config.manager
	truth, err := m.MkTrue()
	if err != nil {
		return nil, ast.ASTRef{}, err
	}
	carry, err = m.NewRef(truth)
	if err != nil {
		return nil, ast.ASTRef{}, err
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, releaseRefs(m, out))
			out = nil
			if carry != (ast.ASTRef{}) {
				result = errors.Join(result, m.DelRef(&carry))
			}
		}
	}()
	for i := range a {
		notB, err := m.NewRef(ast.ASTHandle{})
		if err != nil {
			return nil, carry, err
		}
		if err = e.config.simplify(ast.ASTOpNot, notB, b[i]); err != nil {
			_ = m.DelRef(&notB)
			return nil, carry, err
		}
		notBHandle, err := m.RefGet(notB)
		if err != nil {
			_ = m.DelRef(&notB)
			return nil, carry, err
		}
		carryHandle, err := m.RefGet(carry)
		if err != nil {
			_ = m.DelRef(&notB)
			return nil, carry, err
		}
		bitOut, err := m.NewRef(ast.ASTHandle{})
		if err != nil {
			_ = m.DelRef(&notB)
			return nil, carry, err
		}
		nextCarry, err := m.NewRef(ast.ASTHandle{})
		if err == nil {
			err = e.MkFullAdder(a[i], notBHandle, carryHandle, bitOut, nextCarry)
		}
		cleanup := m.DelRef(&notB)
		if err != nil || cleanup != nil {
			_ = m.DelRef(&bitOut)
			if nextCarry != (ast.ASTRef{}) {
				_ = m.DelRef(&nextCarry)
			}
			return nil, carry, errors.Join(err, cleanup)
		}
		out = append(out, bitOut)
		if err = m.DelRef(&carry); err != nil {
			_ = m.DelRef(&nextCarry)
			return nil, carry, err
		}
		carry = nextCarry
	}
	return out, carry, nil
}
