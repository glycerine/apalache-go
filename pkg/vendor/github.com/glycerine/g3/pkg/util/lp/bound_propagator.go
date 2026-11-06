package lp

// Source: src/util/lp/lp_bound_propagator.h/.cpp and lar_solver.h's
// adjust_column_index_to_term_index/get_term. Concrete exact LAR propagation.

import (
	"github.com/glycerine/g3/pkg/util/numeral"
)

const LARTermStartIndex uint32 = 1000000

// These two typed hooks are the source virtual operations overridden by
// theory_lra.cpp's local_bound_propagator. Values are borrowed for the call.
// A hook owns its own stopped calls/resources; the propagator never closes it.
type LARBoundInterest interface {
	BoundIsInteresting(column uint32, kind LConstraintKind, value *numeral.Rational) (bool, error)
}
type LARBoundEvidenceConsumer interface {
	Consume(value *numeral.Rational, witness uint32) error
}

// LARBoundPropagator owns implied bounds and the source strongest-bound lookup
// maps. Bounds and the live column/term directories are borrowed source model
// fields. Close releases only this owner, never the model, hooks or streams.
// Do not copy, mutate FoundBounds, or copy its owning implied-bound values.
type LARBoundPropagator struct {
	FoundBounds       []*ImpliedBound
	Interest          LARBoundInterest
	Evidence          LARBoundEvidenceConsumer
	bounds            *LARBounds
	columns           *[]uint32
	terms             *[]*LARTerm
	manager           *numeral.RationalManager
	lower, upper      map[uint32]uint32
	calls             exactEtaCalls
	work              *IndexedRationalVector
	temporary, stored *ImpliedBound
}

func NewLARBoundPropagator(bounds *LARBounds, columnsToExternal *[]uint32, terms *[]*LARTerm) *LARBoundPropagator {
	return &LARBoundPropagator{bounds: bounds, columns: columnsToExternal, terms: terms, manager: bounds.manager, lower: make(map[uint32]uint32), upper: make(map[uint32]uint32)}
}

func (p *LARBoundPropagator) NumberOfFoundBounds() uint32                { return uint32(len(p.FoundBounds)) }
func (p *LARBoundPropagator) ColumnType(j uint32) (ColumnType, error)    { return p.bounds.Types.At(j) }
func (p *LARBoundPropagator) LowBound(j uint32) (*RationalPair, error)   { return p.bounds.Low.At(j) }
func (p *LARBoundPropagator) UpperBound(j uint32) (*RationalPair, error) { return p.bounds.Upper.At(j) }

func (p *LARBoundPropagator) Consume(value *numeral.Rational, witness uint32) error {
	// The only retained explanation caller uses theory_lra's override.
	// The source base "doh" printer has no caller. Require the concrete hook
	// rather than constructing an unused output-stream adapter for it.
	if p.Evidence == nil {
		return ErrBounds
	}
	return p.Evidence.Consume(value, witness)
}

func (p *LARBoundPropagator) ready() error {
	if p.work != nil || p.temporary != nil || p.stored != nil || p.calls.pending() {
		return ErrExactVectorPending
	}
	return nil
}

func (p *LARBoundPropagator) termIndex(j uint32) (uint32, error) {
	if p.columns == nil || uint64(j) >= uint64(len(*p.columns)) {
		return 0, ErrBounds
	}
	ext := (*p.columns)[j]
	if ext < LARTermStartIndex {
		return j, nil
	}
	return ext, nil
}

// TryAddBound preserves copy/shift/filter/lookup/publication order. An error
// retains all actual temporaries and completed publication until explicit Close.
func (p *LARBoundPropagator) TryAddBound(value *numeral.Rational, j uint32, low, coefficientPositive bool, rowOrTerm uint32, strict bool) (err error) {
	if err = p.ready(); err != nil {
		return err
	}
	termJ, err := p.termIndex(j)
	if err != nil {
		return err
	}
	before, err := p.calls.begin(p.manager)
	if err != nil {
		return err
	}
	tracking := true
	defer func() {
		if tracking {
			err = p.calls.after(p.manager, before, err)
		}
		if err == nil && p.work != nil {
			err = p.work.Close()
			if err == nil {
				p.work = nil
			}
		}
	}()
	p.work = NewIndexedRationalVector(p.manager, 1)
	w := &p.work.Data[0]
	if err = p.manager.Copy(w, value); err != nil {
		return err
	}
	if termJ != j {
		j = termJ
		index := termJ - LARTermStartIndex
		if p.terms == nil || uint64(index) >= uint64(len(*p.terms)) || (*p.terms)[index] == nil {
			return ErrBounds
		}
		term := (*p.terms)[index]
		if term.manager != p.manager {
			return ErrExactVectorOwner
		}
		if err = term.ready(); err != nil {
			return err
		}
		if err = p.manager.Add(w, &term.Value, w); err != nil {
			return err
		}
	}
	kind := LConstraintLE
	if low {
		kind = LConstraintGE
	}
	if strict {
		kind /= 2
	}
	if p.Interest != nil {
		// End our numeric segment before entering the separately owned virtual
		// call. Its stopped scratch/allocations must not become our receipts.
		err = p.calls.after(p.manager, before, nil)
		tracking = false
		if err != nil {
			return err
		}
		var interesting bool
		interesting, err = p.Interest.BoundIsInteresting(j, kind, w)
		if err != nil || !interesting {
			return err
		}
		before, err = p.calls.begin(p.manager)
		if err != nil {
			return err
		}
		tracking = true
	}
	lookup := p.upper
	if low {
		lookup = p.lower
	}
	k, present := lookup[j]
	if present {
		if uint64(k) >= uint64(len(p.FoundBounds)) || p.FoundBounds[k] == nil {
			return ErrBounds
		}
		found := p.FoundBounds[k]
		if found.stopped || found.calls.pending() {
			return ErrExactVectorPending
		}
		var c int
		if low {
			c, err = p.manager.Compare(&found.Bound, w)
		} else {
			c, err = p.manager.Compare(w, &found.Bound)
		}
		if err != nil {
			return err
		}
		if c >= 0 {
			var equal bool
			equal, err = p.manager.Equal(w, &found.Bound)
			if err != nil {
				return err
			}
			if !equal || found.Strict || !strict {
				return nil
			}
		}
	} else {
		// The source map entry precedes temporary construction/vector push.
		lookup[j] = uint32(len(p.FoundBounds))
	}
	p.temporary, err = NewImpliedBound(p.manager, w, j, low, coefficientPositive, rowOrTerm, strict)
	if err != nil {
		return err
	}
	if present {
		if err = p.FoundBounds[k].Assign(p.temporary); err != nil {
			return err
		}
	} else {
		t := p.temporary
		p.stored, err = NewImpliedBound(p.manager, &t.Bound, t.Column, t.IsLowBound, t.CoefficientIsPositive, t.RowOrTermIndex, t.Strict)
		if err != nil {
			return err
		}
		p.FoundBounds = append(p.FoundBounds, p.stored)
		p.stored = nil
	}
	if err = p.temporary.Close(); err != nil {
		return err
	}
	p.temporary = nil
	return nil
}

func (p *LARBoundPropagator) Close() error {
	if err := p.calls.cleanupRaw(p.manager); err != nil {
		return err
	}
	if p.stored != nil {
		if err := p.stored.Close(); err != nil {
			return err
		}
		p.stored = nil
	}
	if p.temporary != nil {
		if err := p.temporary.Close(); err != nil {
			return err
		}
		p.temporary = nil
	}
	if p.work != nil {
		if err := p.work.Close(); err != nil {
			return err
		}
		p.work = nil
	}
	for len(p.FoundBounds) > 0 {
		i := len(p.FoundBounds) - 1
		if err := p.FoundBounds[i].Close(); err != nil {
			return err
		}
		p.FoundBounds = p.FoundBounds[:i]
	}
	if err := p.calls.cleanupRoots(p.manager); err != nil {
		return err
	}
	p.FoundBounds = nil
	p.lower = nil
	p.upper = nil
	return nil
}
