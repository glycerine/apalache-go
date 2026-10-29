package rewriter

// Source: src/ast/rewriter/poly_rewriter.h:32,62;
// src/ast/rewriter/poly_rewriter_def.h:527-660.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
)

type BVPolynomialAdditionOptions struct{ SortSums, ASTOrder, HoistCoefficients, HoistMultiplication bool }

// BVPolynomialAdditionCore owns the selected nonflat addition invocation.
// Inputs borrow caller lifetimes through source exit.
type BVPolynomialAdditionCore struct {
	pair                                 [2]ast.ASTHandle
	flatArgs                             []ast.ASTHandle
	currentSort                          ast.ASTHandle
	sortBinding                          *ast.ASTHandle
	ownsPositions                        bool
	localsEnded                          bool
	family                               ast.FamilyID
	self                                 *BVPolynomialAdditionCore
	manager                              *ast.ASTManager
	positions                            *bvPolynomialPositions
	numeric                              *ast.BitVectorPolynomialAdditionNumerals
	visited                              map[ast.ASTHandle]bool
	multiple                             map[ast.ASTHandle]bool
	vector                               []ast.ASTHandle
	cmul                                 *ast.BitVectorCoefficientHoist
	mul                                  *BVPolynomialMultiplicationHoist
	factory                              ast.ASTBitVectorNumeralInvocation
	header                               uint64
	started, running, sourceDone, closed bool
	bodyCause, sourceCause               error
}

func NewBVPolynomialAdditionCore(m *ast.ASTManager) (*BVPolynomialAdditionCore, error) {
	return newBVPolynomialAdditionCore(m, nil, nil)
}

func newBVPolynomialAdditionCore(m *ast.ASTManager, table *bvPolynomialPositions, sort *ast.ASTHandle) (*BVPolynomialAdditionCore, error) {
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	header := uint64(reflect.TypeOf(BVPolynomialAdditionCore{}).Size())
	if e := m.Budget().Reserve(header); e != nil {
		return nil, e
	}
	owns := table == nil
	if owns {
		var e error
		table, e = newBVPolynomialPositions(m)
		if e != nil {
			m.Budget().Release(header)
			return nil, e
		}
	} else if table.manager != m {
		m.Budget().Release(header)
		return nil, ast.ErrASTHandle
	} else if e := table.ready(); e != nil {
		m.Budget().Release(header)
		return nil, e
	}
	p := &BVPolynomialAdditionCore{manager: m, positions: table, header: header, ownsPositions: owns, sortBinding: sort}
	p.self = p
	return p, nil
}

func (p *BVPolynomialAdditionCore) append(h ast.ASTHandle) error {
	return appendRewriterRoot(p.manager, &p.vector, h)
}

func (p *BVPolynomialAdditionCore) sort(o BVPolynomialAdditionOptions, start uint32) error {
	if uint64(start) > uint64(len(p.vector)) {
		return ast.ErrASTDomain
	}
	return bvPolynomialSortMonomials(p.manager, p.family, o.ASTOrder, p.vector[start:])
}
func (p *BVPolynomialAdditionCore) body(sort ast.ASTHandle, args ast.AppArguments, out ast.ASTRef, o BVPolynomialAdditionOptions) (RewriteStatus, error) {
	m := p.manager
	if args.Count() < 2 {
		return RewriteFailed, ast.ErrASTDomain
	}
	var e error
	p.numeric, e = ast.NewBitVectorPolynomialAdditionNumerals(m)
	if e != nil {
		return RewriteFailed, e
	}
	if e = p.numeric.Begin(); e != nil {
		return RewriteFailed, e
	}
	p.visited = make(map[ast.ASTHandle]bool)
	p.multiple = make(map[ast.ASTHandle]bool)
	numCoeffs := uint32(0)
	hasMultiple, ordered := false, true
	var prev ast.ASTHandle
	fid, e := m.GetFamilyID(sort)
	if e != nil {
		return RewriteFailed, e
	}
	p.family = fid
	for i := uint32(0); i < args.Count(); i++ {
		arg, e := args.At(i)
		if e != nil {
			return RewriteFailed, e
		}
		numeral, e := p.numeric.Read(arg)
		if e != nil {
			return RewriteFailed, e
		}
		if numeral {
			numCoeffs++
			if e = p.numeric.AddConstant(); e != nil {
				return RewriteFailed, e
			}
			ordered = !o.SortSums || i == 0
		} else if o.SortSums && ordered {
			if prev != (ast.ASTHandle{}) {
				less, e := bvMonomialLess(m, fid, o.ASTOrder, arg, prev)
				if e != nil {
					return RewriteFailed, e
				}
				if less {
					ordered = false
				}
			}
			prev = arg
		}
		pp, e := bvPowerProduct(m, arg)
		if e != nil {
			return RewriteFailed, e
		}
		marked := p.visited[pp]
		if marked {
			p.multiple[pp] = true
			hasMultiple = true
		} else {
			p.visited[pp] = true
		}
	}
	if e = p.numeric.Normalize(sort, false); e != nil {
		return RewriteFailed, e
	}
	if hasMultiple {
		if e = p.numeric.BeginCoefficients(); e != nil {
			return RewriteFailed, e
		}
		if e = p.positions.reset(); e != nil {
			return RewriteFailed, e
		}
		for i := uint32(0); i < args.Count(); i++ {
			arg, e := args.At(i)
			if e != nil {
				return RewriteFailed, e
			}
			numeral, e := m.IsBitVectorNumeral(arg)
			if e != nil {
				return RewriteFailed, e
			}
			if numeral {
				continue
			}
			pp, e := p.numeric.PowerProduct(arg)
			if e != nil {
				return RewriteFailed, e
			}
			marked := p.multiple[pp]
			if !marked {
				continue
			}
			pos, found, e := p.positions.find(pp)
			if e != nil {
				return RewriteFailed, e
			}
			if found {
				if e = p.numeric.AccumulateCoefficient(pos); e != nil {
					return RewriteFailed, e
				}
			} else {
				count, e := p.numeric.CoefficientCount()
				if e != nil {
					return RewriteFailed, e
				}
				if e = p.positions.insert(pp, count); e != nil {
					return RewriteFailed, e
				}
				if e = p.numeric.AppendCoefficient(); e != nil {
					return RewriteFailed, e
				}
			}
		}
	} else if ordered && !o.HoistMultiplication && !o.HoistCoefficients {
		if numCoeffs == 0 {
			return RewriteFailed, nil
		}
		if numCoeffs == 1 {
			first, e := args.At(0)
			if e != nil {
				return RewriteFailed, e
			}
			numeral, e := p.numeric.Read(first)
			if e != nil {
				return RewriteFailed, e
			}
			if numeral {
				zero, e := p.numeric.Zero(true)
				if e != nil {
					return RewriteFailed, e
				}
				if !zero {
					return RewriteFailed, nil
				}
			}
		}
	}
	zero, e := p.numeric.Zero(false)
	if e != nil {
		return RewriteFailed, e
	}
	if !zero {
		constant, e := p.numeric.Constant(sort)
		if e != nil {
			return RewriteFailed, e
		}
		if e = p.append(constant); e != nil {
			return RewriteFailed, e
		}
	}
	if hasMultiple {
		clear(p.visited)
	}
	for i := uint32(0); i < args.Count(); i++ {
		arg, e := args.At(i)
		if e != nil {
			return RewriteFailed, e
		}
		numeral, e := m.IsBitVectorNumeral(arg)
		if e != nil {
			return RewriteFailed, e
		}
		if numeral {
			continue
		}
		if !hasMultiple {
			if e = p.append(arg); e != nil {
				return RewriteFailed, e
			}
			continue
		}
		pp, e := bvPowerProduct(m, arg)
		if e != nil {
			return RewriteFailed, e
		}
		marked := p.multiple[pp]
		if !marked {
			if e = p.append(arg); e != nil {
				return RewriteFailed, e
			}
			continue
		}
		seen := p.visited[pp]
		if seen {
			continue
		}
		p.visited[pp] = true
		pos, found, e := p.positions.find(pp)
		if e != nil {
			return RewriteFailed, e
		}
		if !found {
			return RewriteFailed, ast.ErrASTDomain
		}
		if e = p.numeric.LoadCoefficient(pos); e != nil {
			return RewriteFailed, e
		}
		if e = p.numeric.Normalize(sort, true); e != nil {
			return RewriteFailed, e
		}
		cancelled, e := p.numeric.Zero(true)
		if e != nil {
			return RewriteFailed, e
		}
		if !cancelled {
			product, e := p.numeric.Product(sort, pp)
			if e != nil {
				return RewriteFailed, e
			}
			if e = p.append(product); e != nil {
				return RewriteFailed, e
			}
		}
	}
	if o.HoistCoefficients {
		p.cmul, e = ast.NewBitVectorCoefficientHoist(m)
		if e != nil {
			return RewriteFailed, e
		}
		if e = p.cmul.Hoist(sort, &p.vector); e != nil {
			return RewriteFailed, e
		}
	} else if (hasMultiple && o.SortSums) || (!hasMultiple && !ordered) {
		start := uint32(0)
		if !zero {
			start = 1
		}
		if e = p.sort(o, start); e != nil {
			return RewriteFailed, e
		}
	}
	view := ast.SliceArguments(p.vector)
	node, e := m.MkBitVectorPolynomialAddWithInvocation(sort, view, &p.factory)
	if e != nil {
		return RewriteFailed, e
	}
	if e = m.SetRef(out, node); e != nil {
		return RewriteFailed, e
	}
	if o.HoistMultiplication {
		p.mul, e = NewBVPolynomialMultiplicationHoist(m)
		if e != nil {
			return RewriteFailed, e
		}
		p.mul.sortBinding = p.sortSlot()
		changed, e := p.mul.Hoist(true, out)
		if e != nil {
			return RewriteFailed, e
		}
		if changed {
			return RewriteFull, nil
		}
	}
	return RewriteDone, nil
}
func (p *BVPolynomialAdditionCore) childrenSource() error {
	if p.numeric != nil {
		if e := p.numeric.SourceChildren(); e != nil {
			return e
		}
	}
	if p.cmul != nil {
		if e := p.cmul.SourceReady(); e != nil {
			return e
		}
	}
	if p.mul != nil {
		if e := p.mul.SourceReady(); e != nil {
			return e
		}
	}
	return p.manager.BitVectorNumeralInvocationSourceReady(&p.factory)
}
func (p *BVPolynomialAdditionCore) sourceEnd() error {
	if p.localsEnded {
		return p.childrenSource()
	}
	if e := p.childrenSource(); e != nil {
		return e
	}
	if e := shrinkRewriterRoots(p.manager, &p.vector, 0); e != nil {
		return e
	}
	p.vector = nil
	if p.numeric != nil {
		if e := p.numeric.EndCoefficients(); e != nil {
			return e
		}
	}
	p.multiple = nil
	p.visited = nil
	if p.numeric != nil {
		if e := p.numeric.SourceEnd(); e != nil {
			return e
		}
	}
	p.localsEnded = true
	return nil
}

// Rewrite is the original mk_nflat_add_core entry with the caller's current sort.
func (p *BVPolynomialAdditionCore) Rewrite(sort ast.ASTHandle, args ast.AppArguments, out ast.ASTRef, o BVPolynomialAdditionOptions) (RewriteStatus, error) {
	return p.rewrite(sort, args, out, o, false, false, false)
}

// RewriteFlat specializes mk_flat_add_core. Expansion is exactly depth one.
// Source: src/ast/rewriter/poly_rewriter_def.h:371-400.
func (p *BVPolynomialAdditionCore) RewriteFlat(sort ast.ASTHandle, args ast.AppArguments, out ast.ASTRef, o BVPolynomialAdditionOptions) (RewriteStatus, error) {
	return p.rewrite(sort, args, out, o, true, false, false)
}

// RewriteApplication is mk_add_core: a singleton assigns before sort lookup;
// otherwise the physical current sort comes from the first argument.
// Source: src/ast/rewriter/poly_rewriter.h:137-148.
func (p *BVPolynomialAdditionCore) RewriteApplication(args ast.AppArguments, out ast.ASTRef, flat bool, o BVPolynomialAdditionOptions) (RewriteStatus, error) {
	return p.rewrite(ast.ASTHandle{}, args, out, o, flat, true, false)
}
func (p *BVPolynomialAdditionCore) rewrite(sort ast.ASTHandle, args ast.AppArguments, out ast.ASTRef, o BVPolynomialAdditionOptions, flat, application, fallback bool) (RewriteStatus, error) {
	if p == nil || p.self != p || p.closed {
		return RewriteFailed, ast.ErrASTHandle
	}
	if p.started || p.running {
		return RewriteFailed, ast.ErrASTPending
	}
	p.started, p.running = true, true
	defer func() { p.running = false }()
	st, e := p.dispatch(sort, args, out, o, flat, application)
	p.bodyCause = e
	if e != nil {
		p.sourceCause = p.childrenSource()
		return st, e
	}
	p.sourceCause = p.sourceEnd()
	if p.sourceCause == nil {
		clear(p.flatArgs)
		p.flatArgs = nil
		if fallback && st == RewriteFailed {
			node, body := p.manager.MkBitVectorPolynomialAddWithInvocation(*p.sortSlot(), args, &p.factory)
			if body == nil {
				body = p.manager.SetRef(out, node)
			}
			if body != nil {
				p.bodyCause = body
				p.sourceCause = p.childrenSource()
				return st, body
			}
		}
		p.sourceCause = p.childrenSource()
		p.sourceDone = p.sourceCause == nil
	}
	return st, p.sourceCause
}
func (p *BVPolynomialAdditionCore) dispatch(sort ast.ASTHandle, args ast.AppArguments, out ast.ASTRef, o BVPolynomialAdditionOptions, flat, application bool) (RewriteStatus, error) {
	if application {
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
		sort, e = p.manager.GetSort(first)
		if e != nil {
			return RewriteFailed, e
		}
	}
	*p.sortSlot() = sort
	if !flat {
		return p.body(sort, args, out, o)
	}
	return p.flatBody(sort, args, out, o)
}
func (p *BVPolynomialAdditionCore) SourceReady() error {
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
func (p *BVPolynomialAdditionCore) Close() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	if p.running {
		return ast.ErrASTPending
	}
	if p.mul != nil {
		if e := p.mul.Close(); e != nil {
			return e
		}
	}
	if p.cmul != nil {
		if e := p.cmul.Close(); e != nil {
			return e
		}
	}
	if e := p.manager.CleanupBitVectorNumeralInvocation(&p.factory); e != nil {
		return e
	}
	if e := shrinkRewriterRoots(p.manager, &p.vector, 0); e != nil {
		return e
	}
	p.vector = nil
	p.multiple = nil
	p.visited = nil
	if p.numeric != nil {
		if e := p.numeric.Close(); e != nil {
			return e
		}
	}
	clear(p.flatArgs)
	p.flatArgs = nil
	if p.ownsPositions {
		p.positions.close()
	}
	p.manager.Budget().Release(p.header)
	p.header = 0
	p.closed = true
	return nil
}

func (p *BVPolynomialAdditionCore) sortSlot() *ast.ASTHandle {
	if p.sortBinding != nil {
		return p.sortBinding
	}
	return &p.currentSort
}
