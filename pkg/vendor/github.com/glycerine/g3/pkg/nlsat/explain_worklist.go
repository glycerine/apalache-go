package nlsat

// Source: src/nlsat/nlsat_explain.cpp:49-118.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
)

// projectionWorklist is explain::imp::todo_set over its real polynomial cache.
// It owns polynomial references; insertion canonicalizes before checking marks.
// Projection, normalization and the complete explain functor remain separate.
type projectionWorklist struct {
	self                    *projectionWorklist
	search                  *searchState
	set                     *polynomial.PolynomialRefVector
	inSet                   []bool
	header                  uint64
	running, failed, closed bool
}

func newProjectionWorklist(r *searchState) (*projectionWorklist, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	if r.projection != nil {
		return nil, arena.ErrConfig
	}
	h := uint64(reflect.TypeOf(projectionWorklist{}).Size())
	if err := r.store.atoms.ineq.budget.Reserve(h); err != nil {
		return nil, err
	}
	p := &projectionWorklist{search: r, header: h}
	p.self = p
	r.projection = p
	var err error
	p.set, err = polynomial.NewPolynomialRefVector(r.store.atoms.ineq.core)
	p.failed = err != nil
	return p, err
}
func (p *projectionWorklist) owner() error {
	if p == nil || p.self != p {
		return ErrSearchOwner
	}
	if p.closed {
		return arena.ErrClosed
	}
	if err := p.search.owner(); err != nil {
		return err
	}
	if p.search.projection != p {
		return ErrSearchOwner
	}
	return nil
}
func (p *projectionWorklist) open() error {
	if err := p.owner(); err != nil {
		return err
	}
	if p.set == nil {
		return ErrSearchPending
	}
	return p.search.open()
}
func (p *projectionWorklist) insert(value polynomial.Polynomial) (err error) {
	if err = p.open(); err != nil {
		return err
	}
	return p.insertCore(value)
}

// The actual explanation caller is already executing and borrowing this
// worklist. Validate that specific binding instead of opening a second search.
func (p *projectionWorklist) insertFromExplanation(n *explanationAssumptions, value polynomial.Polynomial) error {
	if err := p.borrowedByExplanation(n); err != nil {
		return err
	}
	return p.insertCore(value)
}

func (p *projectionWorklist) borrowedByExplanation(n *explanationAssumptions) error {
	if err := p.owner(); err != nil {
		return err
	}
	if n == nil || n.self != n || n.search != p.search || n.search.explanationAssumptions != n || n.projectionTodo != p || !n.running {
		return ErrSearchOwner
	}
	if p.running || p.failed || p.set == nil {
		return ErrSearchPending
	}
	return nil
}
func (p *projectionWorklist) insertCore(value polynomial.Polynomial) (err error) {
	p.running = true
	defer func() { p.running = false; p.failed = err != nil }()
	f := p.search.store.atoms.ineq
	value, err = f.polynomials.MkUnique(value)
	if err != nil {
		return err
	}
	pid, err := f.core.PolynomialID(value)
	if err != nil {
		return err
	}
	if uint64(pid) < uint64(len(p.inSet)) && p.inSet[pid] {
		return nil
	}
	if uint64(pid) >= uint64(len(p.inSet)) {
		p.inSet = append(p.inSet, make([]bool, int(uint64(pid)+1)-len(p.inSet))...)
	}
	p.inSet[pid] = true
	return p.set.Push(value)
}
func (p *projectionWorklist) reset() (err error) {
	if err = p.open(); err != nil {
		return err
	}
	return p.resetCore()
}
func (p *projectionWorklist) resetFromExplanation(n *explanationAssumptions) error {
	if err := p.borrowedByExplanation(n); err != nil {
		return err
	}
	return p.resetCore()
}
func (p *projectionWorklist) resetCore() (err error) {
	p.running = true
	defer func() { p.running = false; p.failed = err != nil }()
	n, err := p.set.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < n; i++ {
		v, e := p.set.Get(i)
		if e != nil {
			return e
		}
		pid, e := p.search.store.atoms.ineq.core.PolynomialID(v)
		if e != nil {
			return e
		}
		p.inSet[pid] = false
	}
	return p.set.Reset()
}
func (p *projectionWorklist) maxVariableCore() (uint32, error) {
	max := uint32(NullVariable)
	n, err := p.set.Size()
	if err != nil {
		return max, err
	}
	for i := uint32(0); i < n; i++ {
		v, e := p.set.Get(i)
		if e != nil {
			return max, e
		}
		x, e := p.search.store.atoms.ineq.core.MaxVariable(v)
		if e != nil {
			return max, e
		}
		if e = atomAssert("nlsat_explain.cpp", 84, "x != null_var", x != NullVariable); e != nil {
			return max, e
		}
		if max == NullVariable || x > max {
			max = x
		}
	}
	return max, nil
}
func (p *projectionWorklist) maxVariable() (uint32, error) {
	if err := p.open(); err != nil {
		return NullVariable, err
	}
	return p.maxVariableCore()
}
func (p *projectionWorklist) empty() (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	n, err := p.set.Size()
	return n == 0, err
}
func (p *projectionWorklist) emptyFromExplanation(n *explanationAssumptions) (bool, error) {
	if err := p.borrowedByExplanation(n); err != nil {
		return false, err
	}
	count, err := p.set.Size()
	return count == 0, err
}
func (p *projectionWorklist) removeMaxPolynomials(out *polynomial.PolynomialRefVector) (max uint32, err error) {
	if err = p.open(); err != nil {
		return NullVariable, err
	}
	return p.removeMaxPolynomialsCore(out)
}
func (p *projectionWorklist) removeMaxPolynomialsFromExplanation(n *explanationAssumptions, out *polynomial.PolynomialRefVector) (uint32, error) {
	if err := p.borrowedByExplanation(n); err != nil {
		return NullVariable, err
	}
	return p.removeMaxPolynomialsCore(out)
}
func (p *projectionWorklist) removeMaxPolynomialsCore(out *polynomial.PolynomialRefVector) (max uint32, err error) {
	if out == nil {
		return NullVariable, arena.ErrConfig
	}
	p.running = true
	defer func() { p.running = false; p.failed = err != nil }()
	if err = out.Reset(); err != nil {
		return NullVariable, err
	}
	max, err = p.maxVariableCore()
	if err != nil {
		return max, err
	}
	n, err := p.set.Size()
	if err != nil {
		return max, err
	}
	j := uint32(0)
	core := p.search.store.atoms.ineq.core
	for i := uint32(0); i < n; i++ {
		v, e := p.set.Get(i)
		if e != nil {
			return max, e
		}
		x, e := core.MaxVariable(v)
		if e != nil {
			return max, e
		}
		if e = atomAssert("nlsat_explain.cpp", 104, "y <= x", x <= max); e != nil {
			return max, e
		}
		if x == max {
			if err = out.Push(v); err != nil {
				return max, err
			}
			pid, e := core.PolynomialID(v)
			if e != nil {
				return max, e
			}
			p.inSet[pid] = false
		} else {
			if err = p.set.Set(j, v); err != nil {
				return max, err
			}
			j++
		}
	}
	return max, p.set.Shrink(j)
}
func (p *projectionWorklist) cleanup() error {
	if err := p.owner(); err != nil {
		return err
	}
	if p.running {
		return ErrSearchPending
	}
	if p.set != nil {
		if err := p.set.CleanupRetained(); err != nil {
			return err
		}
	}
	// Cache and borrowed output-vector calls retain their own accepted work.
	// In particular, this does not erase marks or roll back a partial SET.
	p.failed = false
	return nil
}
func (p *projectionWorklist) close() error {
	if p != nil && p.self == p && p.closed {
		return nil
	}
	if err := p.cleanup(); err != nil {
		return err
	}
	// A stopped owning destructor still blocks search even if its lexical
	// operation cleanup has already completed.
	p.failed = true
	if p.set != nil {
		if err := p.set.Close(); err != nil {
			return err
		}
		p.set = nil
	}
	p.search.projection = nil
	p.search.store.atoms.ineq.budget.Release(p.header)
	p.header = 0
	p.inSet = nil
	p.closed = true
	return nil
}
