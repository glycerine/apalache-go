package lp

// Source: src/test/lp.cpp:3050-3078; numeric_pair.h:97-101,164-168,216-218;
// indexed_vector.hpp:25-30. Retained test_rationals benchmark.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/numeral"
	z3rand "github.com/glycerine/g3/pkg/util/random"
	"github.com/glycerine/g3/pkg/util/stopwatch"
)

// PairBenchmarkDriver owns the literal numeric_pair<mpq> benchmark. Run
// executes one million outer iterations with ten scalar/pair products each.
// The manager and shared RNG are borrowed; failed temporaries remain for Close.
// ResultText and counters remain inspectable after successful lexical cleanup.
type PairBenchmarkDriver struct {
	Coefficients, Values, CompletedIterations, CompletedProducts uint32
	ResultText                                                   string
	manager                                                      *numeral.RationalManager
	scalars                                                      *IndexedRationalVector
	pairs                                                        *IndexedPairVector
	calls                                                        exactEtaCalls
	watch                                                        stopwatch.Stopwatch
	out                                                          io.Writer
	started, closed                                              bool
}

func NewPairBenchmarkDriver(manager *numeral.RationalManager, config MPSDriverConfig) *PairBenchmarkDriver {
	return &PairBenchmarkDriver{manager: manager, out: driverReportOutput(config.Output), watch: stopwatch.New(config.Clock)}
}

func (d *PairBenchmarkDriver) Run(random *z3rand.Generator) error {
	return d.runIterations(random, 1000000)
}

// Focused tests use the same body with a private bound. Public Run retains
// the literal registered source count; no product or accumulation is hoisted.
func (d *PairBenchmarkDriver) runIterations(random *z3rand.Generator, iterations uint32) error {
	if d.started || d.closed {
		return ErrExactVectorPending
	}
	d.started = true
	if d.manager == nil || random == nil {
		return ErrBounds
	}
	// c[0:10], copied-left product temporary, two coordinate expressions,
	// input constructor temporary; x[0:10], r, product, conversion temporary.
	d.scalars = NewIndexedRationalVector(d.manager, 14)
	d.pairs = NewIndexedPairVector(d.manager, 13)
	s, p := d.scalars.Data, d.pairs.Data
	for i := 0; i < 20; i++ {
		n, denominator := random.Next()%100, 1+random.Next()%100
		before, err := d.calls.begin(d.manager)
		if err != nil {
			return err
		}
		err = d.manager.SetFraction(&s[13], int64(n), uint64(denominator))
		if err = d.calls.after(d.manager, before, err); err != nil {
			return err
		}
		before, err = d.calls.begin(d.manager)
		if err != nil {
			return err
		}
		if i < 10 {
			err = d.manager.Copy(&s[i], &s[13])
		} else {
			// Implicit scalar-to-pair construction precedes vector's owning copy.
			err = d.manager.Copy(&p[12].X, &s[13])
			if err == nil {
				err = p[i-10].Copy(d.manager, &p[12])
			}
		}
		if err = d.calls.after(d.manager, before, err); err != nil {
			return err
		}
		if i < 10 {
			d.Coefficients++
		} else {
			d.Values++
		}
		if err = p[12].Del(d.manager); err != nil {
			return err
		}
		if err = d.manager.Del(&s[13]); err != nil {
			return err
		}
		s[13] = numeral.NewRational()
	}
	if err := writeConstraintText(d.out, "x = "); err != nil {
		return err
	}
	for i := 0; i < 10; i++ {
		before, err := d.calls.begin(d.manager)
		if err != nil {
			return err
		}
		text, err := p[i].String(d.manager)
		if err = d.calls.after(d.manager, before, err); err != nil {
			return err
		}
		if err = writeConstraintText(d.out, text); err != nil {
			return err
		}
		if err = writeConstraintText(d.out, " "); err != nil {
			return err
		}
	}
	if err := writeDriverLine(d.out, ""); err != nil {
		return err
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
			// The complete pair constructor precedes r += product. Coordinate
			// expressions use copied-left rational multiplication and return copies.
			err = etaRationalProduct(d.manager, &s[10], &s[11], &s[i], &p[i].X)
			if err == nil {
				err = etaRationalProduct(d.manager, &s[10], &s[12], &s[i], &p[i].Y)
			}
			if err == nil {
				err = d.manager.Copy(&p[11].X, &s[11])
			}
			if err == nil {
				err = d.manager.Copy(&p[11].Y, &s[12])
			}
			if err = d.calls.after(d.manager, before, err); err != nil {
				return err
			}
			// Constructor argument temporaries end before compound accumulation.
			for k := 12; k >= 11; k-- {
				if err = d.manager.Del(&s[k]); err != nil {
					return err
				}
				s[k] = numeral.NewRational()
			}
			before, err = d.calls.begin(d.manager)
			if err != nil {
				return err
			}
			err = p[10].Add(d.manager, &p[10], &p[11])
			if err = d.calls.after(d.manager, before, err); err != nil {
				return err
			}
			if err = p[11].Del(d.manager); err != nil {
				return err
			}
			d.CompletedProducts++
		}
		d.CompletedIterations++
	}
	if err := d.watch.Stop(); err != nil {
		return err
	}
	if err := writeConstraintText(d.out, "operation with rationals "); err != nil {
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
	// lp_settings.h:336-342 selects the concrete pair overload here.
	// The earlier vector stream uses numeric_pair::to_string's generic
	// coordinate helper (declared before these overloads), hence exact inputs.
	thousand, tmp, quotient, sum := &s[10], &s[11], &s[12], &s[13]
	err = d.manager.SetInt32(thousand, 1000)
	if err == nil {
		err = etaRationalQuotient(d.manager, tmp, quotient, &p[10].Y, thousand)
	}
	if err == nil {
		err = etaRationalSum(d.manager, tmp, sum, &p[10].X, quotient)
	}
	var number float64
	if err == nil {
		number, err = d.manager.Float64(sum)
	}
	// The arithmetic expression's temporaries die before the stream formats r.
	if err == nil {
		for i := 13; i >= 10; i-- {
			if err = d.manager.Del(&s[i]); err != nil {
				break
			}
			s[i] = numeral.NewRational()
		}
	}
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

func (d *PairBenchmarkDriver) Close() error {
	if d.closed {
		return nil
	}
	if d.manager != nil {
		if err := d.calls.cleanupRaw(d.manager); err != nil {
			return err
		}
		if d.pairs != nil {
			if err := d.pairs.calls.cleanupRaw(d.manager); err != nil {
				return err
			}
			// r, product and constructor temporary die before vector x.
			for i := 12; i >= 10; i-- {
				if err := d.pairs.Data[i].Del(d.manager); err != nil {
					return err
				}
				d.pairs.Data[i] = NewRationalPair()
			}
			for i := 0; i < 10; i++ {
				if err := d.pairs.Data[i].Del(d.manager); err != nil {
					return err
				}
				d.pairs.Data[i] = NewRationalPair()
			}
			if err := d.pairs.calls.cleanupRoots(d.manager); err != nil {
				return err
			}
			d.pairs = nil
		}
		if d.scalars != nil {
			if err := d.scalars.calls.cleanupRaw(d.manager); err != nil {
				return err
			}
			for i := 13; i >= 10; i-- {
				if err := d.manager.Del(&d.scalars.Data[i]); err != nil {
					return err
				}
				d.scalars.Data[i] = numeral.NewRational()
			}
			// Vector c is declared before x and destroys its cells forward.
			for i := 0; i < 10; i++ {
				if err := d.manager.Del(&d.scalars.Data[i]); err != nil {
					return err
				}
				d.scalars.Data[i] = numeral.NewRational()
			}
			if err := d.scalars.calls.cleanupRoots(d.manager); err != nil {
				return err
			}
			d.scalars = nil
		}
		if err := d.calls.cleanupRoots(d.manager); err != nil {
			return err
		}
	}
	d.closed = true
	return nil
}
