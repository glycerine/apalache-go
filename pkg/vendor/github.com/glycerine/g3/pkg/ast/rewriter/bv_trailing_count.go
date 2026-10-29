package rewriter

// Source: src/ast/rewriter/bv_trailing.cpp:246-336.

import "github.com/glycerine/g3/pkg/ast"

func (p *BVTrailing) countCore(arg ast.ASTHandle, depth uint32, call *bvTrailingCall) (min, max uint32, body error) {
	m := p.manager
	matched, count, e := m.CountBitVectorNumeralTrailingZerosWithInvocation(arg, &call.numeral)
	if e != nil || matched {
		return count, count, e
	}
	for _, kind := range []int32{6, 4, 37} {
		yes, e := m.IsBitVectorOperation(arg, kind)
		if e != nil {
			return 0, 0, e
		}
		if !yes {
			continue
		}
		switch kind {
		case 6:
			return p.countMul(arg, depth, call)
		case 4:
			return p.countAdd(arg, depth, call)
		case 37:
			return p.countConcat(arg, depth, call)
		}
	}
	max, body = m.BitVectorSize(arg)
	return 0, max, body
}

func (p *BVTrailing) countConcat(arg ast.ASTHandle, depth uint32, call *bvTrailingCall) (min, max uint32, body error) {
	m := p.manager
	// Source's shallow guard reads width but does not return before the scan.
	if depth <= 1 {
		if _, e := m.BitVectorSize(arg); e != nil {
			return 0, 0, e
		}
	}
	num, e := m.GetNumArgs(arg)
	if e != nil {
		return 0, 0, e
	}
	updateMin, updateMax := true, true
	for num != 0 && updateMax {
		num--
		curr, e := m.GetArg(arg, num)
		if e != nil {
			return min, max, e
		}
		width, e := m.BitVectorSize(curr)
		if e != nil {
			return min, max, e
		}
		lo, hi, e := p.countTracked(curr, depth-1, call)
		if e != nil {
			return min, max, e
		}
		if ast.ASTSourceDebug && width == lo && width != hi {
			return min, max, ast.ErrASTDomain
		}
		max += hi
		if updateMin {
			min += lo
		}
		updateMin = updateMin && width == lo
		updateMax = updateMax && width == hi
	}
	return min, max, nil
}

func (p *BVTrailing) countAdd(arg ast.ASTHandle, depth uint32, call *bvTrailingCall) (min, max uint32, body error) {
	m := p.manager
	if depth <= 1 {
		if _, e := m.BitVectorSize(arg); e != nil {
			return 0, 0, e
		}
	}
	num, e := m.GetNumArgs(arg)
	if e != nil {
		return 0, 0, e
	}
	width, e := m.BitVectorSize(arg)
	if e != nil {
		return 0, 0, e
	}
	min, max = width, width
	knownParity, odd := true, false
	for i := uint32(0); i < num; i++ {
		curr, e := m.GetArg(arg, i)
		if e != nil {
			return min, max, e
		}
		lo, hi, e := p.countTracked(curr, depth-1, call)
		if e != nil {
			return min, max, e
		}
		if lo < min {
			min = lo
		}
		knownParity = knownParity && (hi == 0 || lo != 0)
		if knownParity && hi == 0 {
			odd = !odd
		}
		if !knownParity && min == 0 {
			break
		}
	}
	if knownParity && odd {
		max = 0
	} else {
		max = width
	}
	return min, max, nil
}

func (p *BVTrailing) countMul(arg ast.ASTHandle, depth uint32, call *bvTrailingCall) (min, max uint32, body error) {
	m := p.manager
	if depth <= 1 {
		if _, e := m.BitVectorSize(arg); e != nil {
			return 0, 0, e
		}
	}
	num, e := m.GetNumArgs(arg)
	if e != nil {
		return 0, 0, e
	}
	if num == 0 {
		return 0, 0, nil
	}
	curr, e := m.GetArg(arg, 0)
	if e != nil {
		return 0, 0, e
	}
	min, max, body = p.countTracked(curr, depth-1, call)
	if body != nil {
		return
	}
	if num != 1 {
		max, body = m.BitVectorSize(arg)
	}
	return
}
