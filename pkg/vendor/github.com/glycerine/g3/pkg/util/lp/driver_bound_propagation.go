package lp

// Source: src/test/lp.cpp:2759-2967, retained --bp scenarios.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/numeral"
)

type BoundPropagationCase uint8

const (
	BoundPropagationTotalUpper BoundPropagationCase = iota
	BoundPropagationTriangle
	BoundPropagationOneRow
	BoundPropagationBoundedRow
	BoundPropagationTwoRows
	BoundPropagationMixedRow
	BoundPropagationTotalLower
)

// BoundPropagationExample owns failed children until Close. Successful source
// runs release numerical owners and retain native inspection counts.
type BoundPropagationExample struct {
	Solver                                   *LARSolver
	Propagator                               *LARBoundPropagator
	Status                                   Status
	Constraints, Terms, Columns, Rows, Found uint32
	PresolveWithDouble                       bool
	manager                                  *numeral.RationalManager
	out                                      io.Writer
	literals                                 *IndexedRationalVector
	calls                                    exactEtaCalls
	started, closed                          bool
}

func NewBoundPropagationExample(m *numeral.RationalManager, out io.Writer) *BoundPropagationExample {
	return &BoundPropagationExample{manager: m, out: driverReportOutput(out)}
}

func (d *BoundPropagationExample) Run(which BoundPropagationCase) error {
	if d.started || d.closed {
		return ErrExactVectorPending
	}
	d.started = true
	if which > BoundPropagationTotalLower {
		return ErrBounds
	}
	if which == BoundPropagationTotalUpper || which == BoundPropagationTotalLower {
		text := "test_total_case_u\n"
		if which == BoundPropagationTotalLower {
			text = "test_total_case_l\n"
		}
		if err := writeConstraintText(d.out, text); err != nil {
			return err
		}
	}
	if d.manager == nil {
		return ErrBounds
	}
	var err error
	d.Solver, err = NewLARSolver(d.manager)
	if err != nil {
		return err
	}
	s := d.Solver
	s.Settings.MessageOut, s.Settings.DebugOut = d.out, d.out
	d.literals = NewIndexedRationalVector(d.manager, 6)
	for i, v := range []int32{-3, -1, 0, 1, 2, 3} {
		before, e := d.calls.begin(d.manager)
		if e != nil {
			return e
		}
		e = d.manager.SetInt32(&d.literals.Data[i], v)
		if e = d.calls.after(d.manager, before, e); e != nil {
			return e
		}
	}
	w := d.literals.Data
	nvars := uint32(2)
	if which == BoundPropagationTriangle || which == BoundPropagationTwoRows || which == BoundPropagationTotalUpper || which == BoundPropagationTotalLower {
		nvars = 3
	}
	for j := uint32(0); j < nvars; j++ {
		if _, err = s.AddVar(j); err != nil {
			return err
		}
	}
	coeffs := []RationalSparseEntry{{Index: 0, Value: w[3]}, {Index: 1, Value: w[1]}}
	kind, rhs := LConstraintEQ, &w[3]
	switch which {
	case BoundPropagationTriangle:
		coeffs[1].Index = 2
		if _, err = s.AddTerm(coeffs, &w[2]); err != nil {
			return err
		}
		coeffs[1].Index = 1
		if _, err = s.AddTerm(coeffs, &w[2]); err != nil {
			return err
		}
		kind, rhs = LConstraintLE, &w[2]
	case BoundPropagationTwoRows, BoundPropagationTotalUpper, BoundPropagationTotalLower:
		coeffs[1].Value = w[4]
		coeffs = append(coeffs, RationalSparseEntry{Index: 2, Value: w[5]})
		kind = LConstraintGE
		if which == BoundPropagationTotalUpper {
			kind = LConstraintLE
		}
	}
	// Source AddConstraint is disabled in native and asserts in debug. Retain
	// that call rather than replacing the example with a corrected model.
	if _, err = s.AddConstraint(coeffs, kind, rhs); err != nil {
		return err
	}
	if which == BoundPropagationTriangle {
		coeffs[0].Index, coeffs[1].Index = 1, 2
		if _, err = s.AddConstraint(coeffs, LConstraintLE, &w[2]); err != nil {
			return err
		}
	} else if which == BoundPropagationTwoRows {
		coeffs[0].Value, coeffs[2].Value = w[5], w[3]
		if _, err = s.AddConstraint(coeffs, LConstraintGE, &w[3]); err != nil {
			return err
		}
	}
	switch which {
	case BoundPropagationTriangle, BoundPropagationOneRow:
		_, err = s.AddVarBound(0, LConstraintLE, &w[3])
	case BoundPropagationBoundedRow:
		if _, err = s.AddVarBound(0, LConstraintGE, &w[0]); err != nil {
			return err
		}
		if _, err = s.AddVarBound(0, LConstraintLE, &w[5]); err != nil {
			return err
		}
		_, err = s.AddVarBound(0, LConstraintLE, &w[3])
	case BoundPropagationMixedRow:
		_, err = s.AddVarBound(1, LConstraintLE, &w[3])
	case BoundPropagationTwoRows:
		if _, err = s.AddVarBound(0, LConstraintLE, &w[4]); err != nil {
			return err
		}
		_, err = s.AddVarBound(1, LConstraintLE, &w[3])
	case BoundPropagationTotalUpper:
		for j := uint32(0); j < 3; j++ {
			if _, err = s.AddVarBound(j, LConstraintGE, &w[2]); err != nil {
				return err
			}
		}
	case BoundPropagationTotalLower:
		if _, err = s.AddVarBound(0, LConstraintLE, &w[3]); err != nil {
			return err
		}
		if _, err = s.AddVarBound(1, LConstraintLE, &w[3]); err != nil {
			return err
		}
		s.Settings.PresolveWithDoubleSolverForLAR = true
		_, err = s.AddVarBound(2, LConstraintLE, &w[2])
	}
	if err != nil {
		return err
	}
	d.Status, err = s.Solve()
	if err != nil {
		return err
	}
	d.Propagator = NewLARBoundPropagator(s.Bounds, &s.ColumnsToExternal, &s.Terms)
	if err = s.PropagateBoundsForTouchedRows(d.Propagator); err != nil {
		return err
	}
	d.Constraints, d.Terms = s.NumberOfConstraints(), uint32(len(s.OriginalTerms))
	d.Columns, d.Rows = s.Core.RA.ColumnCount(), s.Core.RA.RowCount()
	d.Found = d.Propagator.NumberOfFoundBounds()
	d.PresolveWithDouble = s.Settings.PresolveWithDoubleSolverForLAR
	if which == BoundPropagationTriangle {
		if err = writeDriverLine(d.out, " bound ev from test_bound_propagation_one_small_sample1"); err != nil {
			return err
		}
		for _, b := range d.Propagator.FoundBounds {
			if err = writeConstraintText(d.out, "bound\n"); err != nil {
				return err
			}
			if err = s.PrintImpliedBound(b, d.out); err != nil {
				return err
			}
		}
	}
	// The source local ev is never populated. Its first final assertion fails;
	// earlier AddConstraint already stops every debug run before this point.
	if which == BoundPropagationTotalLower {
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
	}
	return d.Close()
}

func (d *BoundPropagationExample) Close() error {
	if d.closed {
		return nil
	}
	if d.manager == nil {
		d.closed = true
		return nil
	}
	if err := d.calls.cleanupRaw(d.manager); err != nil {
		return err
	}
	if d.Propagator != nil {
		if err := d.Propagator.Close(); err != nil {
			return err
		}
		d.Propagator = nil
	}
	if d.Solver != nil {
		if err := d.Solver.Close(); err != nil {
			return err
		}
		d.Solver = nil
	}
	if d.literals != nil {
		if err := d.literals.Close(); err != nil {
			return err
		}
		d.literals = nil
	}
	if err := d.calls.cleanupRoots(d.manager); err != nil {
		return err
	}
	d.closed = true
	return nil
}

// BoundPropagationDriver executes the seven registered source cases in order.
// A stopped Current remains owned until Close; Completed contains only fully
// completed native snapshots and has no live numerical children.
type BoundPropagationDriver struct {
	Current         *BoundPropagationExample
	Completed       []*BoundPropagationExample
	manager         *numeral.RationalManager
	out             io.Writer
	started, closed bool
}

func NewBoundPropagationDriver(m *numeral.RationalManager, out io.Writer) *BoundPropagationDriver {
	return &BoundPropagationDriver{manager: m, out: driverReportOutput(out)}
}

func (d *BoundPropagationDriver) Run() error {
	if d.started || d.closed {
		return ErrExactVectorPending
	}
	d.started = true
	for c := BoundPropagationTotalUpper; c <= BoundPropagationTotalLower; c++ {
		d.Current = NewBoundPropagationExample(d.manager, d.out)
		if err := d.Current.Run(c); err != nil {
			return err
		}
		d.Completed = append(d.Completed, d.Current)
		d.Current = nil
	}
	return d.Close()
}

func (d *BoundPropagationDriver) Close() error {
	if d.closed {
		return nil
	}
	if d.Current != nil {
		if err := d.Current.Close(); err != nil {
			return err
		}
		d.Current = nil
	}
	d.closed = true
	return nil
}
