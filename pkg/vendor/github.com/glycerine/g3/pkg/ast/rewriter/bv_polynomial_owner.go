package rewriter

// Source: src/ast/rewriter/poly_rewriter.h:28-41,95-164;
// src/ast/rewriter/bv_rewriter.h:35-43.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// BVPolynomialRewriter owns the persistent BV specialization. Argument views
// borrow caller lifetimes through each invocation's source destruction. Its
// map and current sort borrow AST identities and acquire no AST references.
type BVPolynomialRewriter struct {
	self                     *BVPolynomialRewriter
	manager                  *ast.ASTManager
	trace                    *diagnostic.TraceTags
	currentSort              ast.ASTHandle
	positions                *bvPolynomialPositions
	config                   *BVPolynomialConfiguration
	addition                 *BVPolynomialAdditionCore
	multiplication           *BVPolynomialMultiplicationCore
	negation                 *ast.BitVectorPolynomialNegationNumerals
	subtraction              *bvPolynomialSubtraction
	cancellation             *bvPolynomialCancellation
	additionBits             ast.ASTBitVectorNumeralInvocation
	multiplicationNumerals   *ast.BitVectorMultiplicationNumerals
	bitBool                  *bvBitBoolInvocation
	blastEquality            *bvBlastEqInvocation
	concatEquality           *bvConcatEqualityInvocation
	isolation                *bvIsolationInvocation
	mulEquality              *ast.BitVectorMulEqualityNumerals
	multiplicationConcat     [2]ast.ASTHandle
	pair                     [2]ast.ASTHandle
	header                   uint64
	running, stopped, closed bool
	cause                    error
}

// A reached constructor failure returns the owner for explicit Close. The
// expression map is constructed before the two original parameter updates.
func NewBVPolynomialRewriter(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParams, local *uparams.ParamsRef) (*BVPolynomialRewriter, error) {
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	if c == nil || g == nil || g.ContextField != c {
		return nil, uparams.ErrParamsOwner
	}
	if local != nil {
		if e := c.Check(local); e != nil {
			return nil, e
		}
	}
	bytes := uint64(reflect.TypeOf(BVPolynomialRewriter{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	p := &BVPolynomialRewriter{manager: m, header: bytes}
	p.self = p
	var e error
	p.positions, e = newBVPolynomialPositions(m)
	if e != nil {
		return p, p.fail(e)
	}
	p.config, e = NewBVPolynomialConfiguration(c, g, local)
	if e != nil {
		return p, p.fail(e)
	}
	return p, nil
}
func (p *BVPolynomialRewriter) open() error {
	if p == nil || p.self != p || p.closed {
		return ast.ErrASTHandle
	}
	if p.stopped {
		return p.cause
	}
	if p.running {
		return ast.ErrASTPending
	}
	return nil
}
func (p *BVPolynomialRewriter) fail(e error) error {
	p.stopped = true
	if p.cause == nil {
		p.cause = e
	}
	return e
}
func (p *BVPolynomialRewriter) Update(local *uparams.ParamsRef) error {
	if e := p.open(); e != nil {
		return e
	}
	if e := p.config.Update(local); e != nil {
		return p.fail(e)
	}
	return nil
}
func (p *BVPolynomialRewriter) SetFlat(v bool) error {
	if e := p.open(); e != nil {
		return e
	}
	return p.config.SetFlat(v)
}
func (p *BVPolynomialRewriter) SetSortSums(v bool) error {
	if e := p.open(); e != nil {
		return e
	}
	return p.config.SetSortSums(v)
}
func (p *BVPolynomialRewriter) State() (BVPolynomialConfigurationState, error) {
	if e := p.open(); e != nil {
		return BVPolynomialConfigurationState{}, e
	}
	return p.config.State()
}
func (p *BVPolynomialRewriter) CurrentSort() (ast.ASTHandle, error) {
	if p == nil || p.self != p || p.closed {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	return p.currentSort, nil
}
func (p *BVPolynomialRewriter) RewriteAddition(args ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	return p.rewrite(args, out, true, false)
}
func (p *BVPolynomialRewriter) RewriteMultiplication(args ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	return p.rewrite(args, out, false, false)
}
func (p *BVPolynomialRewriter) MakeAddition(args ast.AppArguments, out ast.ASTRef) error {
	_, e := p.rewrite(args, out, true, true)
	return e
}
func (p *BVPolynomialRewriter) MakeMultiplication(args ast.AppArguments, out ast.ASTRef) error {
	_, e := p.rewrite(args, out, false, true)
	return e
}
func (p *BVPolynomialRewriter) MakeAdditionPair(a, b ast.ASTHandle, out ast.ASTRef) error {
	if e := p.open(); e != nil {
		return e
	}
	p.pair[0] = a
	p.pair[1] = b
	return p.MakeAddition(ast.SliceArguments(p.pair[:]), out)
}
func (p *BVPolynomialRewriter) MakeMultiplicationPair(a, b ast.ASTHandle, out ast.ASTRef) error {
	if e := p.open(); e != nil {
		return e
	}
	p.pair[0] = a
	p.pair[1] = b
	return p.MakeMultiplication(ast.SliceArguments(p.pair[:]), out)
}
func (p *BVPolynomialRewriter) rewrite(args ast.AppArguments, out ast.ASTRef, add, fallback bool) (st RewriteStatus, err error) {
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
	return p.rewriteBody(args, out, add, fallback)
}

// The nested source caller already owns the reentrancy/failure boundary.
func (p *BVPolynomialRewriter) rewriteBody(args ast.AppArguments, out ast.ASTRef, add, fallback bool) (st RewriteStatus, err error) {
	if args.Count() == 0 {
		return RewriteFailed, ast.ErrASTDomain
	}
	first, e := args.At(0)
	if e != nil {
		return RewriteFailed, e
	}
	// The singleton branch precedes current-sort lookup and helper placement.
	if args.Count() == 1 {
		return RewriteDone, p.manager.SetRef(out, first)
	}
	sort, e := p.manager.GetSort(first)
	if e != nil {
		return RewriteFailed, e
	}
	p.currentSort = sort
	if add {
		flat, o, e := p.config.AdditionOptions()
		if e != nil {
			return RewriteFailed, e
		}
		p.addition, e = newBVPolynomialAdditionCore(p.manager, p.positions, &p.currentSort)
		if e != nil {
			return RewriteFailed, e
		}
		st, e = p.addition.rewrite(sort, args, out, o, flat, false, fallback)
		if e != nil {
			return st, e
		}
		if e = p.addition.Close(); e != nil {
			return st, e
		}
		p.addition = nil
		return st, nil
	}
	flat, o, e := p.config.MultiplicationOptions()
	if e != nil {
		return RewriteFailed, e
	}
	p.multiplication, e = NewBVPolynomialMultiplicationCore(p.manager)
	if e != nil {
		return RewriteFailed, e
	}
	p.multiplication.sortBinding = &p.currentSort
	st, e = p.multiplication.rewrite(sort, args, out, o, flat, false, fallback)
	if e != nil {
		return st, e
	}
	if e = p.multiplication.Close(); e != nil {
		return st, e
	}
	p.multiplication = nil
	return st, nil
}
func (p *BVPolynomialRewriter) Close() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	if p.running {
		return ast.ErrASTPending
	}
	if e := p.closeInvocations(); e != nil {
		return e
	}
	if p.config != nil {
		if e := p.config.Close(); e != nil {
			return e
		}
		p.config = nil
	}
	if p.positions != nil {
		p.positions.close()
		p.positions = nil
	}
	p.manager.Budget().Release(p.header)
	p.header = 0
	p.closed = true
	return nil
}

// closeInvocations joins younger stopped source locals before the enclosing
// derived rewriter destroys its trailing/extraction members and base storage.
func (p *BVPolynomialRewriter) closeInvocations() error {
	if p.addition != nil {
		if e := p.addition.Close(); e != nil {
			return e
		}
		p.addition = nil
	}
	if p.multiplication != nil {
		if e := p.multiplication.Close(); e != nil {
			return e
		}
		p.multiplication = nil
	}
	if p.negation != nil {
		if e := p.negation.Close(); e != nil {
			return e
		}
		p.negation = nil
	}
	if p.subtraction != nil {
		if e := p.subtraction.Close(); e != nil {
			return e
		}
		p.subtraction = nil
	}
	if p.cancellation != nil {
		if e := p.cancellation.Close(); e != nil {
			return e
		}
		p.cancellation = nil
	}
	if p.mulEquality != nil {
		if e := p.mulEquality.Close(); e != nil {
			return e
		}
		p.mulEquality = nil
	}
	if p.isolation != nil {
		if e := p.isolation.close(); e != nil {
			return e
		}
		p.isolation = nil
	}
	if p.concatEquality != nil {
		p.concatEquality.close()
		p.concatEquality = nil
	}
	if p.blastEquality != nil {
		if e := p.blastEquality.close(); e != nil {
			return e
		}
		p.blastEquality = nil
	}
	if p.bitBool != nil {
		if e := p.bitBool.close(); e != nil {
			return e
		}
		p.bitBool = nil
	}
	if p.multiplicationNumerals != nil {
		if e := p.multiplicationNumerals.Close(); e != nil {
			return e
		}
		p.multiplicationNumerals = nil
	}
	if e := p.manager.CleanupBitVectorNumeralInvocation(&p.additionBits); e != nil {
		return e
	}
	return nil
}
