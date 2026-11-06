package rewriter

// Source: src/ast/rewriter/bv_trailing.cpp:98-244,399-401.

import "github.com/glycerine/g3/pkg/ast"

// Remove follows the original depth-five structural algorithm. Unknown input
// and a zero limit leave out unchanged; full removal publishes null.
func (p *BVTrailing) Remove(arg ast.ASTHandle, n uint32, out ast.ASTRef) (removed uint32, body error) {
	if e := p.enter(); e != nil {
		return 0, e
	}
	defer func() { p.leave(body) }()
	return p.remove(arg, n, out, bvTrailingDepth, &p.removeCall)
}
func (p *BVTrailing) remove(arg ast.ASTHandle, n uint32, out ast.ASTRef, depth uint32, call *bvTrailingCall) (uint32, error) {
	removed, err := p.removeCore(arg, n, out, depth, call)
	if err != nil {
		return removed, err
	}
	return removed, p.traceRemove(arg, out)
}

func (p *BVTrailing) removeCore(arg ast.ASTHandle, n uint32, out ast.ASTRef, depth uint32, call *bvTrailingCall) (uint32, error) {
	m := p.manager
	if ast.ASTSourceDebug {
		yes, e := m.IsBitVector(arg)
		if e != nil {
			return 0, e
		}
		if !yes {
			return 0, ast.ErrASTDomain
		}
	}
	if depth == 0 || n == 0 {
		return 0, nil
	}
	call.child = nil
	matched, removed, e := m.RemoveBitVectorNumeralTrailingZerosWithInvocation(arg, n, out, &call.numeral)
	if e != nil || matched {
		return removed, e
	}
	for _, kind := range []int32{6, 4, 37} {
		yes, e := m.IsBitVectorOperation(arg, kind)
		if e != nil {
			return 0, e
		}
		if !yes {
			continue
		}
		switch kind {
		case 6:
			return p.removeMul(arg, n, out, depth, call)
		case 4:
			return p.removeAdd(arg, n, out, depth, call)
		case 37:
			return p.removeConcat(arg, n, out, depth, call)
		}
	}
	return 0, nil
}
func (p *BVTrailing) removeAdd(arg ast.ASTHandle, n uint32, out ast.ASTRef, depth uint32, call *bvTrailingCall) (removed uint32, body error) {
	m := p.manager
	num, e := m.GetNumArgs(arg)
	if e != nil {
		return 0, e
	}
	if depth <= 1 {
		return 0, m.SetRef(out, arg)
	}
	lo, _, e := p.countTracked(arg, depth, call)
	if e != nil {
		return 0, e
	}
	toRemove := lo
	if n < toRemove {
		toRemove = n
	}
	if toRemove == 0 {
		return 0, m.SetRef(out, arg)
	}
	width, e := m.BitVectorSize(arg)
	if e != nil {
		return 0, e
	}
	if toRemove == width {
		return width, m.SetRef(out, ast.ASTHandle{})
	}
	f, e := p.beginFrame(bvTrailingAddFrame, call)
	if e != nil {
		return 0, e
	}
	defer f.finish(&body)
	if e = f.newRef(0); e != nil {
		return 0, e
	}
	for i := uint32(0); i < num; i++ {
		curr, e := m.GetArg(arg, i)
		if e != nil {
			return 0, e
		}
		ret, e := p.remove(curr, toRemove, f.refs[0], depth-1, &f.call)
		if e != nil {
			return 0, e
		}
		// VERIFY is evaluated in all source builds; failing it reaches
		// UNREACHABLE (release UB). The Go adapter rejects that domain.
		if ret != toRemove {
			return 0, ast.ErrASTDomain
		}
		h, e := m.RefGet(f.refs[0])
		if e != nil {
			return 0, e
		}
		if e = f.pushArgValue(h); e != nil {
			return 0, e
		}
	}
	h, e := f.makeApp(arg, 4)
	if e != nil {
		return 0, e
	}
	return toRemove, m.SetRef(out, h)
}
func (p *BVTrailing) removeMul(arg ast.ASTHandle, n uint32, out ast.ASTRef, depth uint32, call *bvTrailingCall) (removed uint32, body error) {
	m := p.manager
	num, e := m.GetNumArgs(arg)
	if e != nil {
		return 0, e
	}
	if depth <= 1 || num == 0 {
		return 0, m.SetRef(out, arg)
	}
	f, e := p.beginFrame(bvTrailingMulFrame, call)
	if e != nil {
		return 0, e
	}
	defer f.finish(&body)
	if e = f.newRef(0); e != nil {
		return 0, e
	}
	coefficient, e := m.GetArg(arg, 0)
	if e != nil {
		return 0, e
	}
	ret, e := p.remove(coefficient, n, f.refs[0], depth-1, &f.call)
	if e != nil {
		return 0, e
	}
	if ast.ASTSourceDebug && ret > n {
		return 0, ast.ErrASTDomain
	}
	if ret == 0 {
		return 0, m.SetRef(out, arg)
	}
	tmp, e := m.RefGet(f.refs[0])
	if e != nil {
		return 0, e
	}
	f.scopePlaced = true
	yes, one, _, e := m.ReadBitVectorNumeralScope(tmp, &f.scope)
	if e != nil {
		return 0, e
	}
	if !yes || !one {
		if e = f.pushArgValue(tmp); e != nil {
			return 0, e
		}
	}
	width, e := m.BitVectorSize(coefficient)
	if e != nil {
		return 0, e
	}
	newWidth := width - ret
	if newWidth == 0 {
		return ret, m.SetRef(out, ast.ASTHandle{})
	}
	if ast.ASTSourceDebug {
		w, e := m.BitVectorSize(tmp)
		if e != nil {
			return 0, e
		}
		if w != newWidth {
			return 0, ast.ErrASTDomain
		}
	}
	for i := uint32(1); i < num; i++ {
		curr, e := m.GetArg(arg, i)
		if e != nil {
			return 0, e
		}
		h, e := p.extract.Apply(newWidth-1, 0, curr)
		if e != nil {
			return 0, e
		}
		if e = f.pushArgValue(h); e != nil {
			return 0, e
		}
	}
	size := len(f.args)
	var h ast.ASTHandle
	switch size {
	case 0:
		h, e = m.BitVectorNumeralUnsignedInScope(1, newWidth, &f.scope)
	case 1:
		h = f.args[0]
	default:
		h, e = f.makeApp(arg, 6)
	}
	if e != nil {
		return 0, e
	}
	return ret, m.SetRef(out, h)
}
func (p *BVTrailing) removeConcat(arg ast.ASTHandle, n uint32, out ast.ASTRef, depth uint32, call *bvTrailingCall) (removed uint32, body error) {
	m := p.manager
	if depth <= 1 {
		return 0, m.SetRef(out, arg)
	}
	num, e := m.GetNumArgs(arg)
	if e != nil {
		return 0, e
	}
	f, e := p.beginFrame(bvTrailingConcatFrame, call)
	if e != nil {
		return 0, e
	}
	defer f.finish(&body)
	if e = f.newRef(0); e != nil {
		return 0, e
	}
	i := num
	for i != 0 && removed < n {
		i--
		curr, e := m.GetArg(arg, i)
		if e != nil {
			return removed, e
		}
		// Source passes n, not the remaining request. Preserve over-removal
		// across completely removed operands and unchanged-output branches.
		ret, e := p.remove(curr, n, f.refs[0], depth-1, &f.call)
		if e != nil {
			return removed, e
		}
		width, e := m.BitVectorSize(curr)
		if e != nil {
			return removed, e
		}
		removed += ret
		if ret < width {
			break
		}
	}
	if removed == 0 {
		return 0, m.SetRef(out, arg)
	}
	last, e := m.RefGet(f.refs[0])
	if e != nil {
		return removed, e
	}
	if i == 0 && last.IsNull() {
		if ast.ASTSourceDebug {
			width, e := m.BitVectorSize(arg)
			if e != nil {
				return removed, e
			}
			if removed != width {
				return removed, ast.ErrASTDomain
			}
		}
		return removed, m.SetRef(out, ast.ASTHandle{})
	}
	for j := uint32(0); j < i; j++ {
		h, e := m.GetArg(arg, j)
		if e != nil {
			return removed, e
		}
		if e = f.pushArgValue(h); e != nil {
			return removed, e
		}
	}
	if !last.IsNull() {
		if e = f.pushArgValue(last); e != nil {
			return removed, e
		}
	}
	size := len(f.args)
	var h ast.ASTHandle
	if size == 1 {
		h = f.args[0]
	} else {
		h, e = f.makeApp(arg, 37)
	}
	if e != nil {
		return removed, e
	}
	return removed, m.SetRef(out, h)
}
