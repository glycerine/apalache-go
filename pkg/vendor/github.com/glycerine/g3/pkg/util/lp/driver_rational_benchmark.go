package lp

// Source: src/test/lp.cpp:2990-3046, retained scalar rational benchmarks.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/numeral"
	z3rand "github.com/glycerine/g3/pkg/util/random"
	"github.com/glycerine/g3/pkg/util/stopwatch"
)

// RationalBenchmarkDriver owns the scalar benchmark's inputs and lexical
// arithmetic temporaries. Plus selects the binary-plus source variant;
// otherwise accumulation uses compound addition. Run executes the literal
// 500000 iterations, with no algebraic shortcut. Failed work stays owned until
// Close; the manager and shared random generator are borrowed.
type RationalBenchmarkDriver struct {
	Plus                 bool
	CompletedIterations  uint32
	Coefficients, Values uint32
	ResultText           string
	manager              *numeral.RationalManager
	work                 *IndexedRationalVector
	calls                exactEtaCalls
	watch                stopwatch.Stopwatch
	out                  io.Writer
	started, closed      bool
}

func NewRationalBenchmarkDriver(manager *numeral.RationalManager, plus bool, config MPSDriverConfig) *RationalBenchmarkDriver {
	return &RationalBenchmarkDriver{manager: manager, Plus: plus, out: driverReportOutput(config.Output), watch: stopwatch.New(config.Clock)}
}

func (d *RationalBenchmarkDriver) Run(random *z3rand.Generator) error {
	return d.runIterations(random, 500000)
}

// The private bound supports focused tests of the unchanged source loop body.
// It is not a configurable public benchmark or full registered-run evidence.
func (d *RationalBenchmarkDriver) runIterations(random *z3rand.Generator, iterations uint32) error {
	if d.started || d.closed {
		return ErrExactVectorPending
	}
	d.started = true
	if d.manager == nil || random == nil {
		return ErrBounds
	}
	// c[0:10], x[10:20], r, val, product, copied-left temporary,
	// binary-plus return value and input constructor temporary.
	d.work = NewIndexedRationalVector(d.manager, 26)
	w := d.work.Data
	for i := 0; i < 20; i++ {
		n, denominator := random.Next()%100, 1+random.Next()%100
		before, err := d.calls.begin(d.manager)
		if err != nil {
			return err
		}
		err = d.manager.SetFraction(&w[25], int64(n), uint64(denominator))
		if err = d.calls.after(d.manager, before, err); err != nil {
			return err
		}
		before, err = d.calls.begin(d.manager)
		if err != nil {
			return err
		}
		err = d.manager.Copy(&w[i], &w[25])
		if err = d.calls.after(d.manager, before, err); err != nil {
			return err
		}
		if i < 10 {
			d.Coefficients++
		} else {
			d.Values++
		}
		if err = d.manager.Del(&w[25]); err != nil {
			return err
		}
		w[25] = numeral.NewRational()
	}
	if err := d.watch.Start(); err != nil {
		return err
	}
	for j := uint32(0); j < iterations; j++ {
		for i := 0; i < 10; i++ {
			before, err := d.calls.begin(d.manager)
			if err != nil {
				return err
			}
			err = etaRationalProduct(d.manager, &w[23], &w[22], &w[i], &w[10+i])
			if err = d.calls.after(d.manager, before, err); err != nil {
				return err
			}
			if err = d.add(&w[21], &w[22]); err != nil {
				return err
			}
			// Full-expression product dies before the next inner iteration.
			if err = d.manager.Del(&w[22]); err != nil {
				return err
			}
			w[22] = numeral.NewRational()
		}
		if err := d.add(&w[20], &w[21]); err != nil {
			return err
		}
		if err := d.manager.Del(&w[21]); err != nil {
			return err
		}
		w[21] = numeral.NewRational()
		d.CompletedIterations++
	}
	if err := d.watch.Stop(); err != nil {
		return err
	}
	if err := writeConstraintText(d.out, "operation with rationals no pairs "); err != nil {
		return err
	}
	seconds, err := d.watch.Seconds()
	if err != nil {
		return err
	}
	if err = writeDriverLine(d.out, floatLPNumberText(seconds)); err != nil {
		return err
	}
	before, err := d.calls.begin(d.manager)
	if err != nil {
		return err
	}
	number, err := d.manager.Float64(&w[20])
	if err == nil {
		d.ResultText = floatLPNumberText(number)
	}
	if err = d.calls.after(d.manager, before, err); err != nil {
		return err
	}
	if err = writeDriverLine(d.out, d.ResultText); err != nil {
		return err
	}
	return d.Close()
}

func (d *RationalBenchmarkDriver) add(left, right *numeral.Rational) error {
	before, err := d.calls.begin(d.manager)
	if err != nil {
		return err
	}
	if d.Plus {
		w := d.work.Data
		err = etaRationalSum(d.manager, &w[23], &w[24], left, right)
		if err == nil {
			err = d.manager.Copy(left, &w[24])
		}
		if err == nil {
			err = d.manager.Del(&w[24])
			if err == nil {
				w[24] = numeral.NewRational()
			}
		}
	} else {
		err = d.manager.Add(left, right, left)
	}
	return d.calls.after(d.manager, before, err)
}

func (d *RationalBenchmarkDriver) Close() error {
	if d.closed {
		return nil
	}
	if d.manager != nil {
		if err := d.calls.cleanupRaw(d.manager); err != nil {
			return err
		}
		if d.work != nil {
			if err := d.work.calls.cleanupRaw(d.manager); err != nil {
				return err
			}
			// Lexical expression work dies before x, then c; vector cells
			// within each source vector are destroyed from front to back.
			for i := 25; i >= 20; i-- {
				if err := d.manager.Del(&d.work.Data[i]); err != nil {
					return err
				}
				d.work.Data[i] = numeral.NewRational()
			}
			for i := 10; i < 20; i++ {
				if err := d.manager.Del(&d.work.Data[i]); err != nil {
					return err
				}
				d.work.Data[i] = numeral.NewRational()
			}
			for i := 0; i < 10; i++ {
				if err := d.manager.Del(&d.work.Data[i]); err != nil {
					return err
				}
				d.work.Data[i] = numeral.NewRational()
			}
			if err := d.work.calls.cleanupRoots(d.manager); err != nil {
				return err
			}
			d.work = nil
		}
		if err := d.calls.cleanupRoots(d.manager); err != nil {
			return err
		}
	}
	d.closed = true
	return nil
}
