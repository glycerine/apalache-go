package sat

// Source: src/sat/sat_solver.cpp.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

// Selected source stats/member facade; no additional solver counters are copied.
type satSolverDynamicSubsumption struct {
	self       *satSolverDynamicSubsumption
	analysis   *satSolverConflictAnalysis
	probing    *satSolverProbing
	budget     *arena.Budget
	ownerBytes uint64
	eliminated uint32
	closed     bool
}

func newSATSolverDynamicSubsumption(a *satSolverConflictAnalysis, p *satSolverProbing, b *arena.Budget) (*satSolverDynamicSubsumption, error) {
	if e := a.constructionBorrow(); e != nil {
		return nil, e
	}
	if e := p.constructionBorrow(); e != nil {
		return nil, e
	}
	if a.propagation != p.propagation() || b != a.budget {
		return nil, ErrSATOwner
	}
	n := uint64(reflect.TypeOf(satSolverDynamicSubsumption{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	d := &satSolverDynamicSubsumption{analysis: a, probing: p, budget: b, ownerBytes: n}
	d.self = d
	return d, nil
}
func (d *satSolverDynamicSubsumption) constructionBorrow() error {
	if d == nil || d.self != d {
		return ErrSATOwner
	}
	if d.closed {
		return arena.ErrClosed
	}
	if e := d.analysis.constructionBorrow(); e != nil {
		return e
	}
	return d.probing.constructionBorrow()
}
func (d *satSolverDynamicSubsumption) open() error {
	if d == nil || d.self != d {
		return ErrSATOwner
	}
	if d.closed {
		return arena.ErrClosed
	}
	if e := d.analysis.open(); e != nil {
		return e
	}
	return d.probing.open()
}
func (d *satSolverDynamicSubsumption) Close() error {
	if d == nil || d.self != d {
		return ErrSATOwner
	}
	if d.closed {
		return nil
	}
	d.budget.Release(d.ownerBytes)
	d.ownerBytes = 0
	d.closed = true
	return nil
}
func (d *satSolverDynamicSubsumption) marked(l Literal) (bool, error) {
	marks := d.analysis.propagation.variables.literalMark
	if uint64(l.index()) >= uint64(len(marks)) {
		if satStorageSourceDebug {
			return false, ErrSATAssertion
		}
		return false, arena.ErrBounds
	}
	return marks[l.index()], nil
}
func (d *satSolverDynamicSubsumption) mark(l Literal) error {
	yes, e := d.marked(l)
	if e != nil {
		return e
	}
	if satStorageSourceDebug && yes {
		return ErrSATAssertion
	}
	d.analysis.propagation.variables.literalMark[l.index()] = true
	return nil
}
func (d *satSolverDynamicSubsumption) unmark(l Literal) error {
	yes, e := d.marked(l)
	if e != nil {
		return e
	}
	if satStorageSourceDebug && !yes {
		return ErrSATAssertion
	}
	d.analysis.propagation.variables.literalMark[l.index()] = false
	return nil
}
func (d *satSolverDynamicSubsumption) eliminateIfMarked(l, fuip Literal) error {
	yes, e := d.marked(l)
	if e != nil {
		return e
	}
	if yes && fuip != l {
		return d.unmark(l)
	}
	return nil
}
func (d *satSolverDynamicSubsumption) run() error {
	if e := d.open(); e != nil {
		return e
	}
	lemma := &d.analysis.lemma
	n, e := lemma.Size()
	if e != nil {
		return e
	}
	for i := uint32(0); i < n; i++ {
		l, e := lemma.Read(i)
		if e != nil {
			return e
		}
		if e = d.mark(Literal(l)); e != nil {
			return e
		}
	}
	first, e := lemma.Read(0)
	if e != nil {
		return e
	}
	fuip := Literal(first)
	for i := uint32(0); i < n; i++ {
		value, e := lemma.Read(i)
		if e != nil {
			return e
		}
		l := Literal(value)
		yes, e := d.marked(l)
		if e != nil {
			return e
		}
		if !yes {
			continue
		}
		list, e := d.analysis.propagation.variables.watches.Get(l.Negated().index())
		if e != nil {
			return e
		}
		ref, e := list.ref()
		if e != nil {
			return e
		}
		it, e := list.owner.store.Iter(ref)
		if e != nil {
			return e
		}
		for {
			w, ok, e := it.Next()
			if e != nil {
				return e
			}
			if !ok {
				break
			}
			if w.Binary() {
				other, e := w.Literal()
				if e != nil {
					return e
				}
				if e = d.eliminateIfMarked(other.Negated(), fuip); e != nil {
					return e
				}
			} else if w.Ternary() {
				a, e := w.Literal1()
				if e != nil {
					return e
				}
				b, e := w.Literal2()
				if e != nil {
					return e
				}
				ma, e := d.marked(a)
				if e != nil {
					return e
				}
				firstBranch := false
				if ma {
					mb, e := d.marked(b.Negated())
					if e != nil {
						return e
					}
					if mb && fuip != b.Negated() {
						if e = d.unmark(b.Negated()); e != nil {
							return e
						}
						firstBranch = true
					}
				}
				if !firstBranch {
					ma, e := d.marked(a.Negated())
					if e != nil {
						return e
					}
					if ma {
						mb, e := d.marked(b)
						if e != nil {
							return e
						}
						if mb && fuip != a.Negated() {
							if e = d.unmark(a.Negated()); e != nil {
								return e
							}
						}
					}
				}
			} else {
				break
			}
		}
		cached, found, e := d.probing.cachedImpliedLits(l.Negated())
		if e != nil {
			return e
		}
		if found {
			block, end, e := cached.capture()
			if e != nil {
				return e
			}
			for j := uint32(0); j < end; j++ {
				other, e := cached.readCaptured(block, j)
				if e != nil {
					return e
				}
				if e = d.eliminateIfMarked(other.Negated(), fuip); e != nil {
					return e
				}
			}
		}
	}
	if satStorageSourceDebug {
		first, e := lemma.Read(0)
		if e != nil {
			return e
		}
		yes, e := d.marked(Literal(first))
		if e != nil {
			return e
		}
		if !yes {
			return ErrSATAssertion
		}
	}
	j := uint32(0)
	for i := uint32(0); i < n; i++ {
		x, e := lemma.Read(i)
		if e != nil {
			return e
		}
		l := Literal(x)
		yes, e := d.marked(l)
		if e != nil {
			return e
		}
		if yes {
			if e = d.unmark(l); e != nil {
				return e
			}
			if e = lemma.Write(j, x); e != nil {
				return e
			}
			j++
		}
	}
	d.eliminated += n - j
	if satStorageSourceDebug && j < 1 {
		return ErrSATAssertion
	}
	return lemma.Shrink(j)
}
