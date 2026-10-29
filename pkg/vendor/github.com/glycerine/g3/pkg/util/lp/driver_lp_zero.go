package lp

// Source: src/test/lp.cpp:58-63,515-569, retained --test_lp_0.

import (
	"io"
	"strconv"
)

type driverSimpleColumnNamer struct{}

func (driverSimpleColumnNamer) ColumnName(j uint32) string {
	return "x" + strconv.FormatUint(uint64(j), 10)
}

// LPZeroDriver owns the literal source model and borrows its slice headers to
// the actual upper-only primal constructor. A stopped core stays reachable for
// inspection and Close. The source default strategy is deliberately retained.
type LPZeroDriver struct {
	Matrix             *FloatStaticMatrix
	Core               *FloatPrimalCoreSolver
	Settings           *Settings
	B, X, Costs, Upper []float64
	Basis, NBasis      []uint32
	Heading            []int32
	Types              []ColumnType
	Iterations         uint32
	out                io.Writer
	started, closed    bool
}

func NewLPZeroDriver(out io.Writer) *LPZeroDriver {
	return &LPZeroDriver{out: driverReportOutput(out)}
}

func (d *LPZeroDriver) Run() error {
	if d.started || d.closed {
		return ErrExactVectorPending
	}
	d.started = true
	if err := writeDriverLine(d.out, " test_lp_0 "); err != nil {
		return err
	}
	d.Matrix = NewFloatStaticMatrix(3, 7)
	for i, row := range [][]float64{
		{3, 2, 1, 2, 1, 0, 0},
		{1, 1, 1, 1, 0, 1, 0},
		{4, 3, 3, 4, 0, 0, 1},
	} {
		for j, value := range row {
			if value == 0 {
				continue
			}
			if err := d.Matrix.Set(uint32(i), uint32(j), value); err != nil {
				return err
			}
		}
	}
	d.X = []float64{225, 117, 420, 0, 0, 0, 0}
	for i := uint32(0); i < d.Matrix.RowCount(); i++ {
		value, err := d.Matrix.DotRow(i, d.X)
		if err != nil {
			return err
		}
		d.B = append(d.B, value)
	}
	d.Basis = []uint32{0, 1, 2}
	d.Costs = []float64{19, 13, 12, 17, 0, 0, 0}
	d.Types = make([]ColumnType, 7)
	for j := range d.Types {
		d.Types[j] = ColumnLowBound
	}
	var err error
	d.Settings, err = NewSettings()
	if err != nil {
		return err
	}
	d.Settings.MessageOut, d.Settings.DebugOut = d.out, d.out
	d.Core, err = NewFloatPrimalCoreSolverWithZeroLowBounds(FloatCoreInputs{
		A: d.Matrix, B: &d.B, X: &d.X, Costs: &d.Costs,
		Basis: &d.Basis, NBasis: &d.NBasis, Heading: &d.Heading,
		Settings: d.Settings, Names: driverSimpleColumnNamer{}, ColumnTypes: &d.Types, UpperBounds: &d.Upper,
	})
	if err != nil {
		return err
	}
	d.Iterations, err = d.Core.Solve()
	if err != nil {
		return err
	}
	return d.Close()
}

func (d *LPZeroDriver) Close() error {
	if d.closed {
		return nil
	}
	if d.Core != nil {
		if err := d.Core.Close(); err != nil {
			return err
		}
		d.Core = nil
	}
	d.closed = true
	return nil
}
