package bv

// Source: src/tactic/bv/bv1_blaster_tactic.cpp rw_cfg and rw.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/messages"
)

type bv1BlasterImplementation struct {
	rewriter.DefaultConfiguration
	owner           *BV1BlasterTactic
	traversal       *rewriter.Traversal
	family          ast.FamilyID
	one, zero       ast.ASTHandle
	held, scratch   []ast.ASTHandle
	bindings        []BitBlasterBinding
	constants       map[ast.ASTHandle]ast.ASTHandle
	out, proof      ast.ASTRef
	maxMemory       uint64
	maxSteps, steps uint32
	produceModels   bool // Read by the source policy; goal metadata selects conversion.
}

func (r *bv1BlasterImplementation) hold(h ast.ASTHandle, scratch bool) error {
	if err := r.owner.manager.IncRef(h); err != nil {
		return err
	}
	if scratch {
		r.scratch = append(r.scratch, h)
	} else {
		r.held = append(r.held, h)
	}
	return nil
}
func (r *bv1BlasterImplementation) releaseScratch(limit int) error {
	for len(r.scratch) > limit {
		i := len(r.scratch) - 1
		if err := r.owner.manager.DecRef(r.scratch[i]); err != nil {
			return err
		}
		r.scratch = r.scratch[:i]
	}
	return nil
}
func (r *bv1BlasterImplementation) MaxStepsExceeded(steps uint32) (bool, error) {
	if r.owner.cooperation != nil {
		if err := r.owner.cooperation.Cooperate("bv1 blaster"); err != nil {
			return false, err
		}
	}
	used, err := r.owner.manager.NumeralService().RuntimeOwner().AllocationSize()
	if err != nil {
		return false, err
	}
	if used > r.maxMemory {
		return false, tactic.NewTacticException(messages.MaxMemory)
	}
	return steps > r.maxSteps, nil
}
func (r *bv1BlasterImplementation) RewritePatterns() bool { return false }

// Source get_bits is one concat level, not a recursive flattening operation.
func (r *bv1BlasterImplementation) bits(h ast.ASTHandle) ([]ast.ASTHandle, error) {
	m := r.owner.manager
	concat, err := m.IsBitVectorOperation(h, 37)
	if err != nil {
		return nil, err
	}
	if !concat {
		if ast.ASTSourceDebug {
			n, err := m.BitVectorSize(h)
			if err != nil {
				return nil, err
			}
			if n != 1 {
				return nil, ast.ErrASTDomain
			}
		}
		return []ast.ASTHandle{h}, nil
	}
	n, err := m.GetNumArgs(h)
	if err != nil {
		return nil, err
	}
	result := make([]ast.ASTHandle, n)
	for i := range result {
		result[i], err = m.GetArg(h, uint32(i))
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}
func (r *bv1BlasterImplementation) save(h ast.ASTHandle) (ast.ASTHandle, error) {
	if err := r.hold(h, true); err != nil {
		return ast.ASTHandle{}, err
	}
	return h, nil
}
func (r *bv1BlasterImplementation) constant(d ast.ASTHandle) (ast.ASTHandle, error) {
	m := r.owner.manager
	if h, ok := r.constants[d]; ok {
		return h, nil
	}
	sort, err := m.GetRange(d)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	n, err := m.BitVectorSize(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if n == 1 {
		return m.MkApp(d, nil)
	}
	bitSort, err := m.BitVectorSort(1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	bits := make([]ast.ASTHandle, n)
	for i := range bits {
		h, e := m.MkFreshConstNullable(nil, bitSort)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		bits[i], err = r.save(h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	h, err := m.MkBitVectorConcat(bits)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = r.hold(h, false); err != nil {
		return ast.ASTHandle{}, err
	}
	if err = r.hold(d, false); err != nil {
		return ast.ASTHandle{}, err
	}
	r.constants[d] = h
	r.bindings = append(r.bindings, BitBlasterBinding{Variable: d, Bits: h})
	return h, nil
}
func (r *bv1BlasterImplementation) numeral(d ast.ASTHandle) (result ast.ASTHandle, err error) {
	m := r.owner.manager
	h, err := m.MkApp(d, nil)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	text, yes, err := m.BitVectorNumeralText(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !yes {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	n, err := m.BitVectorSize(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	v, yes := new(big.Int).SetString(text, 10)
	if !yes {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	bits := make([]ast.ASTHandle, n)
	for i := range bits {
		if v.Bit(int(n)-1-i) == 0 {
			bits[i] = r.zero
		} else {
			bits[i] = r.one
		}
	}
	return m.MkBitVectorConcat(bits)
}
func (r *bv1BlasterImplementation) extraction(d, h ast.ASTHandle) (result ast.ASTHandle, err error) {
	m := r.owner.manager
	bits, err := r.bits(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	parameters, err := m.GetDeclParametersCopy(d)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() {
		for i := range parameters {
			err = errors.Join(err, m.DelParameter(&parameters[i]))
		}
	}()
	if len(parameters) != 2 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	high, err := m.ParameterInt(parameters[0])
	if err != nil {
		return ast.ASTHandle{}, err
	}
	low, err := m.ParameterInt(parameters[1])
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if low < 0 || high < low || int(high) >= len(bits) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return m.MkBitVectorConcat(bits[len(bits)-1-int(high) : len(bits)-int(low)])
}
func (r *bv1BlasterImplementation) blastTerm(h ast.ASTHandle) (ast.ASTHandle, error) {
	m := r.owner.manager
	n, err := m.BitVectorSize(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if n == 1 {
		return h, nil
	}
	if err = r.hold(h, true); err != nil {
		return ast.ASTHandle{}, err
	}
	bits := make([]ast.ASTHandle, n)
	for i := range bits {
		index := n - 1 - uint32(i)
		b, e := m.MkBitVectorExtract(index, index, h)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		bits[i], err = r.save(b)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return m.MkBitVectorConcat(bits)
}
func (r *bv1BlasterImplementation) ReduceApp(d ast.ASTHandle, args ast.AppArguments, out, proof ast.ASTRef) (status rewriter.RewriteStatus, err error) {
	m := r.owner.manager
	if err = m.ResetRef(proof); err != nil {
		return rewriter.RewriteFailed, err
	}
	start := len(r.scratch)
	defer func() { err = errors.Join(err, r.releaseScratch(start)) }()
	family, err := m.GetFamilyID(d)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	op, err := m.GetDeclKind(d)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	sort, err := m.GetRange(d)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	returnsBV, err := m.IsBitVectorSort(sort)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	a := make([]ast.ASTHandle, args.Count())
	for i := range a {
		a[i], err = args.At(uint32(i))
		if err != nil {
			return rewriter.RewriteFailed, err
		}
	}
	var value ast.ASTHandle
	switch {
	case family == ast.NullFamilyID && len(a) == 0 && returnsBV:
		value, err = r.constant(d)
	case family == m.BasicFamilyID() && op == ast.ASTOpEq:
		if len(a) != 2 {
			return rewriter.RewriteFailed, ast.ErrASTDomain
		}
		yes, e := m.IsBitVector(a[0])
		if e != nil || !yes {
			return rewriter.RewriteFailed, e
		}
		left, e := r.bits(a[0])
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		right, e := r.bits(a[1])
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if len(left) != len(right) {
			return rewriter.RewriteFailed, ast.ErrASTDomain
		}
		eqs := make([]ast.ASTHandle, len(left))
		for i := range eqs {
			j := len(left) - 1 - i
			h, e := m.MkEq(left[j], right[j])
			if e != nil {
				return rewriter.RewriteFailed, e
			}
			eqs[i], err = r.save(h)
			if err != nil {
				return rewriter.RewriteFailed, err
			}
		}
		value, err = m.MkAndArgs(eqs)
	case family == m.BasicFamilyID() && op == ast.ASTOpIte:
		if len(a) != 3 {
			return rewriter.RewriteFailed, ast.ErrASTDomain
		}
		yes, e := m.IsBitVector(a[1])
		if e != nil || !yes {
			return rewriter.RewriteFailed, e
		}
		left, e := r.bits(a[1])
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		right, e := r.bits(a[2])
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if len(left) != len(right) {
			return rewriter.RewriteFailed, ast.ErrASTDomain
		}
		bits := make([]ast.ASTHandle, len(left))
		for i := range bits {
			h, e := m.MkIte(a[0], left[i], right[i])
			if e != nil {
				return rewriter.RewriteFailed, e
			}
			bits[i], err = r.save(h)
			if err != nil {
				return rewriter.RewriteFailed, err
			}
		}
		value, err = m.MkBitVectorConcat(bits)
	case family == r.family:
		switch op {
		case 0:
			value, err = r.numeral(d)
		case 40:
			if len(a) != 1 {
				return rewriter.RewriteFailed, ast.ErrASTDomain
			}
			value, err = r.extraction(d, a[0])
		case 37:
			var bits []ast.ASTHandle
			for _, arg := range a {
				next, e := r.bits(arg)
				if e != nil {
					return rewriter.RewriteFailed, e
				}
				bits = append(bits, next...)
			}
			value, err = m.MkBitVectorConcat(bits)
		default:
			// XOR and all other BV operators are rejected by is_target before
			// traversal. The source's private XOR helper has no retained caller.
			return rewriter.RewriteFailed, ast.ErrASTDomain
		}
	case returnsBV:
		var h ast.ASTHandle
		h, err = m.MkApp(d, a)
		if err == nil {
			value, err = r.blastTerm(h)
		}
	default:
		return rewriter.RewriteFailed, nil
	}
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	return rewriter.RewriteDone, m.SetRef(out, value)
}
