package rewriter

// Source: src/ast/rewriter/poly_rewriter_def.h:683-701.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
)

type bvPolynomialSubtraction struct {
	self                   *bvPolynomialSubtraction
	manager                *ast.ASTManager
	literal                *ast.BitVectorPolynomialSubtractionLiteral
	minusOne               ast.ASTRef
	minusPlaced            bool
	args                   []ast.ASTHandle
	pair                   [2]ast.ASTHandle
	call                   ast.ASTBitVectorNumeralInvocation
	header                 uint64
	bodyCause, sourceCause error
	sourceDone, closed     bool
}

func newBVPolynomialSubtraction(m *ast.ASTManager) (*bvPolynomialSubtraction, error) {
	bytes := uint64(reflect.TypeOf(bvPolynomialSubtraction{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	p := &bvPolynomialSubtraction{manager: m, header: bytes}
	p.self = p
	return p, nil
}
func (p *bvPolynomialSubtraction) append(h ast.ASTHandle) error {
	return appendRewriterRoot(p.manager, &p.args, h)
}
func (p *bvPolynomialSubtraction) body(sort ast.ASTHandle, args ast.AppArguments, out ast.ASTRef) error {
	m := p.manager
	var e error
	p.literal, e = ast.NewBitVectorPolynomialSubtractionLiteral(m)
	if e != nil {
		return e
	}
	e = p.literal.ConstructReference(sort, &p.minusOne)
	p.minusPlaced = p.minusOne != (ast.ASTRef{})
	if e != nil {
		return e
	}
	first, e := args.At(0)
	if e != nil {
		return e
	}
	if e = p.append(first); e != nil {
		return e
	}
	minus, e := m.RefGet(p.minusOne)
	if e != nil {
		return e
	}
	for i := uint32(1); i < args.Count(); i++ {
		arg, e := args.At(i)
		if e != nil {
			return e
		}
		zero, e := m.IsBitVectorPolynomialZeroWithInvocation(arg, &p.call)
		if e != nil {
			return e
		}
		if zero {
			continue
		}
		p.pair[0] = minus
		p.pair[1] = arg
		node, e := m.MkBitVectorPolynomialMulWithInvocation(sort, ast.SliceArguments(p.pair[:]), &p.call)
		if e != nil {
			return e
		}
		if e = p.append(node); e != nil {
			return e
		}
	}
	view := ast.SliceArguments(p.args)
	node, e := m.MkBitVectorPolynomialAddWithInvocation(sort, view, &p.call)
	if e != nil {
		return e
	}
	return m.SetRef(out, node)
}
func (p *bvPolynomialSubtraction) childrenSource() error {
	if p.literal != nil {
		if e := p.literal.SourceReady(); e != nil {
			return e
		}
	}
	return p.manager.BitVectorNumeralInvocationSourceReady(&p.call)
}
func (p *bvPolynomialSubtraction) rewrite(sort ast.ASTHandle, args ast.AppArguments, out ast.ASTRef) error {
	p.bodyCause = p.body(sort, args, out)
	p.sourceCause = p.childrenSource()
	if p.bodyCause != nil {
		return p.bodyCause
	}
	if p.sourceCause != nil {
		return p.sourceCause
	}
	if e := shrinkRewriterRoots(p.manager, &p.args, 0); e != nil {
		p.sourceCause = e
		return e
	}
	p.args = nil
	if p.minusPlaced {
		if e := p.manager.DelRef(&p.minusOne); e != nil {
			p.sourceCause = e
			return e
		}
		p.minusPlaced = false
	}
	p.sourceDone = true
	return nil
}
func (p *bvPolynomialSubtraction) Close() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	if e := p.manager.CleanupBitVectorNumeralInvocation(&p.call); e != nil {
		return e
	}
	if p.literal != nil {
		if e := p.literal.Close(); e != nil {
			return e
		}
	}
	if e := shrinkRewriterRoots(p.manager, &p.args, 0); e != nil {
		return e
	}
	p.args = nil
	if p.minusPlaced {
		if e := p.manager.DelRef(&p.minusOne); e != nil {
			return e
		}
		p.minusPlaced = false
	}
	p.manager.Budget().Release(p.header)
	p.header = 0
	p.closed = true
	return nil
}
func (p *BVPolynomialRewriter) RewriteSubtraction(args ast.AppArguments, out ast.ASTRef) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	defer func() {
		p.running = false
		if err != nil {
			p.fail(err)
		}
	}()
	if args.Count() == 0 {
		return RewriteFailed, ast.ErrASTDomain
	}
	first, e := args.At(0)
	if e != nil {
		return RewriteFailed, e
	}
	if args.Count() == 1 {
		return RewriteDone, p.manager.SetRef(out, first)
	}
	sort, e := p.manager.GetSort(first)
	if e != nil {
		return RewriteFailed, e
	}
	p.currentSort = sort
	n, e := newBVPolynomialSubtraction(p.manager)
	if e != nil {
		return RewriteFailed, e
	}
	p.subtraction = n
	if e = n.rewrite(p.currentSort, args, out); e != nil {
		return Rewrite2, e
	}
	if e = n.Close(); e != nil {
		return Rewrite2, e
	}
	p.subtraction = nil
	return Rewrite2, nil
}

// Source: src/ast/rewriter/poly_rewriter.h:169-172.
func (p *BVPolynomialRewriter) MakeSubtractionPair(left, right ast.ASTHandle, out ast.ASTRef) error {
	if e := p.open(); e != nil {
		return e
	}
	p.pair[0] = left
	p.pair[1] = right
	_, e := p.RewriteSubtraction(ast.SliceArguments(p.pair[:]), out)
	return e
}
