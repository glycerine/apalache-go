package rewriter

// Source: src/ast/rewriter/poly_rewriter_def.h:886-901,947-977;
// src/ast/rewriter/bv_rewriter.h:35-43.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
)

// bvPolynomialFlatten is the source TO_BUFFER macro. Expanding an operator
// replaces the current slot by arg0, appends siblings at the end and revisits
// that slot. It intentionally does not perform depth-first traversal.
func bvPolynomialFlatten(m *ast.ASTManager, kind int32, buffer *[]ast.ASTHandle, input ast.ASTHandle) error {
	*buffer = append(*buffer, input)
	for i := 0; i < len(*buffer); {
		h := (*buffer)[i]
		yes, e := m.IsBitVectorOperation(h, kind)
		if e != nil {
			return e
		}
		if !yes {
			i++
			continue
		}
		first, e := m.GetArg(h, 0)
		if e != nil {
			return e
		}
		(*buffer)[i] = first
		for j := uint32(1); ; j++ {
			n, e := m.GetNumArgs(h)
			if e != nil {
				return e
			}
			if j >= n {
				break
			}
			arg, e := m.GetArg(h, j)
			if e != nil {
				return e
			}
			*buffer = append(*buffer, arg)
		}
	}
	return nil
}

// BVPolynomialMerger owns one merge_muls invocation's borrowed factor buffers
// and exact empty-product numeral children. The caller keeps both inputs, the
// manager and its physical current-sort field alive until Close.
type BVPolynomialMerger struct {
	self                                 *BVPolynomialMerger
	manager                              *ast.ASTManager
	currentSort                          *ast.ASTHandle
	factors                              [2][]ast.ASTHandle
	residuals                            [2]ast.ASTHandle
	calls                                [2]ast.ASTBitVectorNumeralInvocation
	header                               uint64
	started, running, sourceDone, closed bool
	bodyCause, sourceCause, ownCause     error
}

func NewBVPolynomialMerger(m *ast.ASTManager, currentSort *ast.ASTHandle) (*BVPolynomialMerger, error) {
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	header := uint64(reflect.TypeOf(BVPolynomialMerger{}).Size())
	if e := m.Budget().Reserve(header); e != nil {
		return nil, e
	}
	p := &BVPolynomialMerger{manager: m, currentSort: currentSort, header: header}
	p.self = p
	return p, nil
}

func (p *BVPolynomialMerger) body(x, y ast.ASTHandle) (ast.ASTHandle, error) {
	m := p.manager
	if e := bvPolynomialFlatten(m, 6, &p.factors[0], x); e != nil {
		return ast.ASTHandle{}, e
	}
	if e := bvPolynomialFlatten(m, 6, &p.factors[1], y); e != nil {
		return ast.ASTHandle{}, e
	}
	one, two := p.factors[0], p.factors[1]
	k := 0
	for i := range one {
		x = one[i]
		j := k
		for j < len(two) && two[j] != x {
			j++
		}
		if j < len(two) {
			one[i], one[k] = one[k], one[i]
			two[j], two[k] = two[k], two[j]
			k++
		}
	}
	sort, e := m.GetSort(x)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	if p.currentSort == nil {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	*p.currentSort = sort
	if ast.ASTSourceDebug && (k == 0 || len(one) < k || len(two) < k) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	// Initializer expressions run left-to-right, including any numeral-one
	// constructor/full-expression DT inside an empty mk_mul_app call.
	for i := 0; i < 2; i++ {
		tail := p.factors[i][k:]
		p.residuals[i], e = m.MkBitVectorPolynomialMulWithInvocation(sort, ast.SliceArguments(tail), &p.calls[i])
		if e != nil {
			return ast.ASTHandle{}, e
		}
	}
	if k == len(one) {
		p.factors[0] = append(p.factors[0], ast.ASTHandle{})
	}
	sum, e := m.MkBitVectorPolynomialAdd(sort, ast.SliceArguments(p.residuals[:]))
	if e != nil {
		return ast.ASTHandle{}, e
	}
	p.factors[0][k] = sum
	return m.MkBitVectorPolynomialMul(sort, ast.SliceArguments(p.factors[0][:k+1]))
}

func (p *BVPolynomialMerger) childrenSource() error {
	for i := 1; i >= 0; i-- {
		if e := p.manager.BitVectorNumeralInvocationSourceReady(&p.calls[i]); e != nil {
			return e
		}
	}
	return nil
}

func (p *BVPolynomialMerger) Merge(x, y ast.ASTHandle) (ast.ASTHandle, error) {
	if p == nil || p.self != p || p.closed {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	if p.started || p.running {
		return ast.ASTHandle{}, ast.ErrASTPending
	}
	p.started, p.running = true, true
	defer func() { p.running = false }()
	node, body := p.body(x, y)
	p.bodyCause = body
	p.sourceCause = p.childrenSource()
	if body != nil {
		return ast.ASTHandle{}, body
	}
	if p.sourceCause != nil {
		return ast.ASTHandle{}, p.sourceCause
	}
	for i := 1; i >= 0; i-- {
		clear(p.factors[i])
		p.factors[i] = nil
	}
	p.sourceDone = true
	return node, nil
}

func (p *BVPolynomialMerger) SourceReady() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.sourceCause != nil {
		return p.sourceCause
	}
	if p.started && !p.sourceDone {
		return ast.ErrASTPending
	}
	return nil
}

func (p *BVPolynomialMerger) Close() (result error) {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	if p.running {
		return ast.ErrASTPending
	}
	defer func() {
		if result != nil && p.ownCause == nil {
			p.ownCause = result
		}
	}()
	for i := 1; i >= 0; i-- {
		if e := p.manager.CleanupBitVectorNumeralInvocation(&p.calls[i]); e != nil {
			return e
		}
	}
	for i := 1; i >= 0; i-- {
		clear(p.factors[i])
		p.factors[i] = nil
	}
	p.manager.Budget().Release(p.header)
	p.header = 0
	p.closed = true
	return nil
}
