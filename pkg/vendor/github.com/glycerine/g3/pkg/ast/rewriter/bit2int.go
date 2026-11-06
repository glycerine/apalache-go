package rewriter

// Source: src/ast/rewriter/bit2int.cpp:30-107,412-420;
// src/ast/rewriter/bit2int.h:27-90.

import (
	"errors"
	"math/big"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type bit2IntTerm struct {
	bits  ast.ASTHandle
	width uint32
	sign  bool
}

type bit2IntCall struct {
	owner *Bit2Int
	refs  []ast.ASTRef
}

func (c *bit2IntCall) hold(h ast.ASTHandle) (ast.ASTHandle, error) {
	ref, err := c.owner.manager.NewRef(h)
	if err == nil {
		c.refs = append(c.refs, ref)
	}
	return h, err
}

func (c *bit2IntCall) adopt(ref ast.ASTRef) (ast.ASTHandle, error) {
	h, err := c.owner.manager.RefGet(ref)
	if err == nil {
		c.refs = append(c.refs, ref)
	}
	return h, err
}

func (c *bit2IntCall) close() (result error) {
	for i := len(c.refs); i > 0; {
		i--
		result = errors.Join(result, c.owner.manager.DelRef(&c.refs[i]))
	}
	return result
}

// Bit2Int propagates unsigned bv2int terms through the retained arithmetic
// fragment. It owns its source cache and the one-bit zero seed.
type Bit2Int struct {
	self                  *Bit2Int
	manager               *ast.ASTManager
	bvFamily, arithFamily ast.FamilyID
	cache                 map[ast.ASTHandle]ast.ASTRef
	keys                  []ast.ASTHandle
	bit0                  ast.ASTRef
	params                *uparams.ParamsContext
	globals               *uparams.GlobalParams
	bv                    *BVRewriter
	header                uint64
	closed                bool
}

func NewBit2Int(m *ast.ASTManager) (*Bit2Int, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	bytes := uint64(reflect.TypeOf(Bit2Int{}).Size())
	if err := m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	b := &Bit2Int{manager: m, cache: make(map[ast.ASTHandle]ast.ASTRef), header: bytes}
	b.self = b
	var err error
	b.arithFamily, err = m.RegisterArithmeticPlugin()
	if err != nil {
		return b, err
	}
	b.bvFamily, err = m.RegisterBitVectorPlugin()
	if err != nil {
		return b, err
	}
	zero, err := m.BitVectorNumeralUint64(0, 1)
	if err != nil {
		return b, err
	}
	b.bit0, err = m.NewRef(zero)
	if err != nil {
		return b, err
	}
	b.params, err = uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	if err != nil {
		return b, err
	}
	b.globals, err = uparams.NewGlobalParams(m.Symbols, b.params, uparams.GlobalParamsConfig{Budget: m.Budget(), Register: func(uparams.GlobalParamsRegistrar) error { return nil }})
	if err != nil {
		return b, err
	}
	b.bv, err = NewBVRewriter(m, b.params, b.globals, nil)
	return b, err
}

func (b *Bit2Int) open() error {
	if b == nil || b.self != b || b.closed || b.manager == nil || b.bv == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (b *Bit2Int) getB2ISize(term ast.ASTHandle) (uint32, error) {
	matched, err := b.manager.IsBitVectorOperation(term, 58)
	if err != nil || !matched {
		if err != nil {
			return 0, err
		}
		return 0, ast.ErrASTDomain
	}
	arg, err := b.manager.GetArg(term, 0)
	if err != nil {
		return 0, err
	}
	return b.manager.BitVectorSize(arg)
}

func bit2IntNumeralBits(value *big.Int) uint32 {
	if value == nil {
		return 0
	}
	n := new(big.Int).Abs(value)
	if n.Sign() == 0 {
		return 1
	}
	return uint32(n.BitLen())
}

func (b *Bit2Int) extractBV(c *bit2IntCall, term ast.ASTHandle) (bit2IntTerm, bool, error) {
	matched, err := b.manager.IsBitVectorOperation(term, 58)
	if err != nil {
		return bit2IntTerm{}, false, err
	}
	if matched {
		bits, err := b.manager.GetArg(term, 0)
		if err != nil {
			return bit2IntTerm{}, false, err
		}
		width, err := b.manager.BitVectorSize(bits)
		if err != nil {
			return bit2IntTerm{}, false, err
		}
		bits, err = c.hold(bits)
		return bit2IntTerm{bits: bits, width: width}, true, err
	}
	text, integer, matched, err := b.manager.ArithmeticNumeralText(term)
	if err != nil || !matched || !integer {
		return bit2IntTerm{}, false, err
	}
	value, ok := new(big.Int).SetString(text, 10)
	if !ok {
		return bit2IntTerm{}, false, ast.ErrASTDomain
	}
	width := bit2IntNumeralBits(value)
	service := b.manager.NumeralService()
	rational, err := service.NewValue()
	if err != nil {
		return bit2IntTerm{}, false, err
	}
	defer service.Del(&rational)
	if err = service.SetString(&rational, text); err != nil {
		return bit2IntTerm{}, false, err
	}
	bits, err := b.manager.BitVectorNumeral(&rational, width)
	if err != nil {
		return bit2IntTerm{}, false, err
	}
	bits, err = c.hold(bits)
	return bit2IntTerm{bits: bits, width: width, sign: value.Sign() < 0}, true, err
}

func (b *Bit2Int) alignSize(c *bit2IntCall, term ast.ASTHandle, width uint32) (ast.ASTHandle, error) {
	actual, err := b.manager.BitVectorSize(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if actual > width {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return b.simplifyZeroExtend(c, width-actual, term)
}

func (b *Bit2Int) alignSizes(c *bit2IntCall, left, right ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, error) {
	lw, err := b.manager.BitVectorSize(left)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	rw, err := b.manager.BitVectorSize(right)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if lw < rw {
		left, err = b.alignSize(c, left, rw)
	} else if rw < lw {
		right, err = b.alignSize(c, right, lw)
	}
	return left, right, err
}

func (b *Bit2Int) flushCache() (result error) {
	for i := len(b.keys); i > 0; {
		i--
		key := b.keys[i]
		ref := b.cache[key]
		delete(b.cache, key)
		result = errors.Join(result, b.manager.DelRef(&ref))
	}
	b.keys = b.keys[:0]
	return result
}

func (b *Bit2Int) cacheResult(key, value ast.ASTHandle) error {
	if prior, ok := b.cache[key]; ok {
		return b.manager.SetRef(prior, value)
	}
	ref, err := b.manager.NewRef(value)
	if err != nil {
		return err
	}
	b.cache[key] = ref
	b.keys = append(b.keys, key)
	return nil
}

func (b *Bit2Int) Close() (result error) {
	if b == nil || b.self != b || b.manager == nil {
		return ast.ErrASTHandle
	}
	if b.closed {
		return nil
	}
	result = errors.Join(result, b.flushCache())
	if b.bit0 != (ast.ASTRef{}) {
		result = errors.Join(result, b.manager.DelRef(&b.bit0))
	}
	if b.bv != nil {
		result = errors.Join(result, b.bv.Close())
	}
	if b.globals != nil {
		result = errors.Join(result, b.globals.Close())
	}
	if b.params != nil {
		result = errors.Join(result, b.params.Close())
	}
	b.manager.Budget().Release(b.header)
	b.header = 0
	b.closed = true
	return result
}
