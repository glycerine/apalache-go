package ast

// Source: src/ast/rewriter/poly_rewriter_def.h:453-487;
// src/ast/rewriter/bv_rewriter.h:35-43.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"reflect"
	"sort"
)

// BitVectorCoefficientHoist holds one source invocation's real locals and
// exact children. It borrows the manager and caller's owning argument slice. Go
// wrapper storage is charged; native stack/layout/allocation ABI is not claimed.
type BitVectorCoefficientHoist struct {
	self                                 *BitVectorCoefficientHoist
	manager                              *ASTManager
	compare                              ASTBitVectorNumeralInvocation
	reads                                [2]unumeral.RationalServiceInvocation
	values                               [2]unumeral.OwnedRational
	placed                               [2]bool
	pps                                  []ASTHandle
	mulArgs                              [2]ASTHandle
	header                               uint64
	started, running, sourceDone, closed bool
	bodyCause, sourceCause, ownCause     error
}

func NewBitVectorCoefficientHoist(m *ASTManager) (*BitVectorCoefficientHoist, error) {
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	header := uint64(reflect.TypeOf(BitVectorCoefficientHoist{}).Size())
	if e := m.Budget().Reserve(header); e != nil {
		return nil, e
	}
	p := &BitVectorCoefficientHoist{manager: m, header: header}
	p.self = p
	return p, nil
}

// Sorting follows the source coefficient comparator through Go's library.
func (p *BitVectorCoefficientHoist) sort(v []ASTHandle) error {
	var cause error
	sort.Slice(v, func(i, j int) bool {
		if cause != nil {
			return false
		}
		var less bool
		less, cause = p.manager.BitVectorPolynomialHoistLessWithInvocation(v[i], v[j], &p.compare)
		return less
	})
	return cause
}

// The caller owns every slice entry. Acquire the replacement before retiring
// the old root, keeping both reachable when a stopped DEC consumes neither.
func (p *BitVectorCoefficientHoist) replace(args *[]ASTHandle, i uint32, h ASTHandle) error {
	if e := p.manager.IncRef(h); e != nil {
		return e
	}
	consumed, e := p.manager.DecRefConsumed((*args)[i])
	if consumed {
		(*args)[i] = h
	} else {
		*args = append(*args, h)
	}
	if e != nil {
		return e
	}
	if !consumed {
		return ErrASTPending
	}
	return nil
}

func (p *BitVectorCoefficientHoist) shrink(args *[]ASTHandle, n uint32) error {
	values := *args
	for i := int(n); i < len(values); i++ {
		consumed, e := p.manager.DecRefConsumed(values[i])
		next := i
		if consumed {
			values[i] = ASTHandle{}
			next++
		}
		if e != nil || !consumed {
			*args = append(values[:n], values[next:]...)
			clear(values[len(*args):])
			if e != nil {
				return e
			}
			return ErrASTPending
		}
	}
	*args = values[:n]
	return nil
}

func (p *BitVectorCoefficientHoist) retireRead(i int) error {
	c := &p.reads[i]
	if e := c.SourceReady(); e != nil {
		return e
	}
	if e := c.Cleanup(false); e != nil {
		return e
	}
	if e := c.Cleanup(true); e != nil {
		return e
	}
	if e := c.EndContext(); e != nil {
		return e
	}
	*c = unumeral.RationalServiceInvocation{}
	return nil
}

func (p *BitVectorCoefficientHoist) product(t ASTHandle, i int) (bool, ASTHandle, error) {
	m := p.manager
	mul, e := m.IsBitVectorOperation(t, 6)
	if e != nil || !mul {
		return false, ASTHandle{}, e
	}
	n, e := m.GetNumArgs(t)
	if e != nil || n != 2 {
		return false, ASTHandle{}, e
	}
	first, e := m.GetArg(t, 0)
	if e != nil {
		return false, ASTHandle{}, e
	}
	var width uint32
	yes, e := m.BitVectorNumeralValueWithInvocation(first, &p.values[i], &width, &p.reads[i])
	if e != nil {
		return false, ASTHandle{}, e
	}
	if e = p.retireRead(i); e != nil {
		return false, ASTHandle{}, e
	}
	if !yes {
		return false, ASTHandle{}, nil
	}
	pp, e := m.GetArg(t, 1)
	return true, pp, e
}

func (p *BitVectorCoefficientHoist) body(currentSort ASTHandle, args *[]ASTHandle) error {
	m := p.manager
	if args == nil {
		return ErrASTDomain
	}
	size := uint32(len(*args))
	if e := p.sort(*args); e != nil {
		return e
	}
	var e error
	// These locals are constructed after the entire sort, as in the source.
	for i := range p.values {
		p.values[i], e = m.numerals.NewValue()
		if e != nil {
			return e
		}
		p.placed[i] = true
	}
	var i, j uint32
	for i < size {
		mon := (*args)[i]
		isMul, pp, e := p.product(mon, 0)
		if e != nil {
			return e
		}
		if isMul && i < size-1 {
			next := (*args)[i+1]
			isNext, ppNext, e := p.product(next, 1)
			if e != nil {
				return e
			}
			equal := false
			if isNext {
				equal, e = m.numerals.Equal(&p.values[0], &p.values[1])
				if e != nil {
					return e
				}
			}
			if isNext && equal {
				clear(p.pps)
				p.pps = p.pps[:0]
				p.pps = append(p.pps, pp)
				p.pps = append(p.pps, ppNext)
				i += 2
				for i < size {
					next = (*args)[i]
					isNext, ppNext, e = p.product(next, 1)
					if e != nil {
						return e
					}
					if !isNext {
						break
					}
					equal, e = m.numerals.Equal(&p.values[0], &p.values[1])
					if e != nil {
						return e
					}
					if !equal {
						break
					}
					p.pps = append(p.pps, ppNext)
					i++
				}
				if ASTSourceDebug {
					literal, e := m.GetArg(mon, 0)
					if e != nil {
						return e
					}
					var width uint32
					yes, e := m.BitVectorNumeralValueWithInvocation(literal, &p.values[1], &width, &p.reads[1])
					if e != nil {
						return e
					}
					if e = p.retireRead(1); e != nil {
						return e
					}
					equal, e = m.numerals.Equal(&p.values[0], &p.values[1])
					if e != nil {
						return e
					}
					if !yes || !equal {
						return ErrASTDomain
					}
				}
				// Preserve the debug assertion's separate read and the braced
				// initializer's left-to-right publication before the sum factory.
				literal, e := m.GetArg(mon, 0)
				if e != nil {
					return e
				}
				p.mulArgs[0] = literal
				sum, e := m.MkBitVectorPolynomialAdd(currentSort, SliceArguments(p.pps))
				if e != nil {
					return e
				}
				p.mulArgs[1] = sum
				mul, e := m.MkBitVectorPolynomialMul(currentSort, SliceArguments(p.mulArgs[:]))
				if e != nil {
					return e
				}
				if e = p.replace(args, j, mul); e != nil {
					return e
				}
				j++
				continue
			}
		}
		if e = p.replace(args, j, mon); e != nil {
			return e
		}
		j++
		i++
	}
	return p.shrink(args, j)
}

func (p *BitVectorCoefficientHoist) childrenSource() error {
	if e := p.manager.BitVectorNumeralInvocationSourceReady(&p.compare); e != nil {
		return e
	}
	for i := 1; i >= 0; i-- {
		if e := p.reads[i].SourceReady(); e != nil {
			return e
		}
	}
	return nil
}

func (p *BitVectorCoefficientHoist) sourceEnd() error {
	if e := p.childrenSource(); e != nil {
		return e
	}
	clear(p.pps)
	p.pps = nil
	for i := 1; i >= 0; i-- {
		if p.placed[i] {
			if e := p.manager.numerals.Del(&p.values[i]); e != nil {
				return e
			}
			p.placed[i] = false
		}
	}
	p.sourceDone = true
	return nil
}

// Hoist sorts and compacts the caller-owned slice in place. Each entry must
// hold an acquired AST reference, which remains the caller's responsibility.
// This owner names one invocation; failed calls retain their real prefix for
// caller cleanup and never retry.
func (p *BitVectorCoefficientHoist) Hoist(currentSort ASTHandle, args *[]ASTHandle) (result error) {
	if p == nil || p.self != p || p.closed {
		return ErrASTHandle
	}
	if p.started || p.running {
		return ErrASTPending
	}
	p.started, p.running = true, true
	defer func() { p.running = false }()
	p.bodyCause = p.body(currentSort, args)
	if p.bodyCause != nil {
		p.sourceCause = p.childrenSource()
		return p.bodyCause
	}
	p.sourceCause = p.sourceEnd()
	return p.sourceCause
}

// SourceReady is historical and is unchanged by owning continuation.
func (p *BitVectorCoefficientHoist) SourceReady() error {
	if p == nil || p.self != p {
		return ErrASTHandle
	}
	if p.sourceCause != nil {
		return p.sourceCause
	}
	if p.started && !p.sourceDone {
		return ErrASTPending
	}
	return nil
}

func (p *BitVectorCoefficientHoist) Close() (result error) {
	if p == nil || p.self != p {
		return ErrASTHandle
	}
	if p.closed {
		return nil
	}
	if p.running {
		return ErrASTPending
	}
	defer func() {
		if result != nil && p.ownCause == nil {
			p.ownCause = result
		}
	}()
	if e := p.manager.CleanupBitVectorNumeralInvocation(&p.compare); e != nil {
		return e
	}
	for i := 1; i >= 0; i-- {
		if e := p.reads[i].Cleanup(false); e != nil {
			return e
		}
	}
	clear(p.pps)
	p.pps = nil
	for i := 1; i >= 0; i-- {
		if p.placed[i] {
			if e := p.manager.numerals.CleanupValue(&p.values[i]); e != nil {
				return e
			}
			p.placed[i] = false
		}
	}
	for i := 1; i >= 0; i-- {
		if e := p.reads[i].Cleanup(true); e != nil {
			return e
		}
	}
	for i := 1; i >= 0; i-- {
		if e := p.reads[i].EndContext(); e != nil {
			return e
		}
	}
	p.manager.Budget().Release(p.header)
	p.header = 0
	p.closed = true
	return nil
}

func (p *BitVectorCoefficientHoist) Causes() (body, source, own error) {
	if p == nil || p.self != p {
		return ErrASTHandle, nil, nil
	}
	return p.bodyCause, p.SourceReady(), p.ownCause
}
