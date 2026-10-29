package lp

// Source: src/test/lp.cpp:2711-2759, retained --term and --eti examples.

import (
	"io"
	"sort"

	"github.com/glycerine/g3/pkg/util/numeral"
)

type DriverLARModelValue struct {
	Index uint32
	Name  string
	Value float64
}

// LARExampleDriver owns the actual example solver, literals and model. Failed
// work stays reachable for Close. Successful runs close their numerical owners
// and retain native status/model snapshots. The source AddConstraint has no
// native effect and asserts in debug; these examples preserve that behavior.
type LARExampleDriver struct {
	Solver                      *LARSolver
	Model                       *LARRationalModel
	Status                      Status
	Solution                    []DriverLARModelValue
	Constraints, Terms, Columns uint32
	manager                     *numeral.RationalManager
	out                         io.Writer
	literals                    *IndexedRationalVector
	calls                       exactEtaCalls
	started, closed             bool
}

func NewLARExampleDriver(manager *numeral.RationalManager, out io.Writer) *LARExampleDriver {
	return &LARExampleDriver{manager: manager, out: driverReportOutput(out)}
}

func (d *LARExampleDriver) begin() error {
	if d.started || d.closed {
		return ErrExactVectorPending
	}
	d.started = true
	if d.manager == nil {
		return ErrBounds
	}
	var err error
	d.Solver, err = NewLARSolver(d.manager)
	if err != nil {
		return err
	}
	d.Solver.Settings.MessageOut, d.Solver.Settings.DebugOut = d.out, d.out
	// These immutable small numerals are borrowed by concrete input entries.
	// Actual solver children retain their own arithmetic invocation receipts.
	d.literals = NewIndexedRationalVector(d.manager, 4)
	for i, value := range []int32{1, 3, 0, -1} {
		before, err := d.calls.begin(d.manager)
		if err != nil {
			return err
		}
		err = d.manager.SetInt32(&d.literals.Data[i], value)
		if err = d.calls.after(d.manager, before, err); err != nil {
			return err
		}
	}
	return nil
}

func (d *LARExampleDriver) variables() (x, y uint32, err error) {
	x, err = d.Solver.AddVar(0)
	if err != nil {
		return
	}
	y, err = d.Solver.AddVar(1)
	return
}

func (d *LARExampleDriver) solveAndReport() error {
	var err error
	d.Status, err = d.Solver.Solve()
	if err != nil {
		return err
	}
	d.Constraints = d.Solver.NumberOfConstraints()
	d.Terms = uint32(len(d.Solver.OriginalTerms))
	d.Columns = d.Solver.Core.RA.ColumnCount()
	text, err := StatusString(d.Status)
	if err != nil {
		return err
	}
	return writeDriverLine(d.out, text)
}

func (d *LARExampleDriver) RunTerm() error {
	if err := d.begin(); err != nil {
		return err
	}
	x, y, err := d.variables()
	if err != nil {
		return err
	}
	w := d.literals.Data
	entries := []RationalSparseEntry{{Index: x, Value: w[0]}, {Index: y, Value: w[0]}}
	z, err := d.Solver.AddTerm(entries, &w[1])
	if err != nil {
		return err
	}
	entries = append(entries, RationalSparseEntry{Index: z, Value: w[0]})
	if _, err = d.Solver.AddConstraint(entries, LConstraintEQ, &w[2]); err != nil {
		return err
	}
	if err = d.solveAndReport(); err != nil {
		return err
	}
	d.Model = NewLARRationalModel(d.manager)
	if err = d.Solver.GetModel(d.Model); err != nil {
		return err
	}
	// Source unordered-map bucket order is replaced by ascending column IDs.
	indices := make([]uint32, 0, len(d.Model.Values))
	for j := range d.Model.Values {
		indices = append(indices, j)
	}
	sort.Slice(indices, func(i, j int) bool { return indices[i] < indices[j] })
	for _, j := range indices {
		name := d.Solver.ColumnName(j)
		if _, err = io.WriteString(d.out, name+" = "); err != nil {
			return err
		}
		before, err := d.calls.begin(d.manager)
		if err != nil {
			return err
		}
		value, e := d.manager.Float64(d.Model.Values[j])
		if err = d.calls.after(d.manager, before, e); err != nil {
			return err
		}
		if _, err = io.WriteString(d.out, floatLPNumberText(value)+","); err != nil {
			return err
		}
		d.Solution = append(d.Solution, DriverLARModelValue{j, name, value})
	}
	if err = writeDriverLine(d.out, ""); err != nil {
		return err
	}
	return d.Close()
}

func (d *LARExampleDriver) RunEvidence() error {
	if err := d.begin(); err != nil {
		return err
	}
	x, y, err := d.variables()
	if err != nil {
		return err
	}
	w := d.literals.Data
	if _, err = d.Solver.AddVarBound(x, LConstraintLE, &w[3]); err != nil {
		return err
	}
	if _, err = d.Solver.AddVarBound(y, LConstraintGE, &w[2]); err != nil {
		return err
	}
	entries := []RationalSparseEntry{{Index: x, Value: w[0]}, {Index: y, Value: w[0]}}
	if _, err = d.Solver.AddConstraint(entries, LConstraintGE, &w[0]); err != nil {
		return err
	}
	entries[1].Value = w[3]
	if _, err = d.Solver.AddConstraint(entries, LConstraintGE, &w[2]); err != nil {
		return err
	}
	if err = d.solveAndReport(); err != nil {
		return err
	}
	// The source model is empty and never populated in this example.
	if d.Solver.Bounds.Status.Value != StatusInfeasible {
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
	}
	return d.Close()
}

func (d *LARExampleDriver) Close() error {
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
	if d.Model != nil {
		if err := d.Model.Close(); err != nil {
			return err
		}
		d.Model = nil
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
