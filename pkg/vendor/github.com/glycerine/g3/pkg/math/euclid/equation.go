package euclid

// Source: src/math/euclid/euclidean_solver.cpp:69-88,153-170,291-312.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// equation owns the numerals acquired from the solver's reusable buffers.
// IDs and justification IDs are scalar values; buffers remain separate owners.
type equation struct {
	manager              *numeral.RationalManager
	trace                *diagnostic.TraceTags
	as                   []numeral.Int
	xs                   []uint32
	c                    numeral.Int
	bs                   []numeral.Rational
	js                   []uint32
	header               uint64
	asCursor, bsCursor   int
	constantDone, closed bool
}

func makeEquation(m *numeral.RationalManager, as []numeral.Int, xs []uint32, c *numeral.Int,
	bs []numeral.Rational, js []uint32, asBuffer *numeral.ScopedIntVector,
	bsBuffer *numeral.ScopedRationalVector, sortJustifications bool, tags *diagnostic.TraceTags) (result *equation, err error) {
	if m == nil || c == nil || asBuffer == nil || bsBuffer == nil || len(as) != len(xs) || len(bs) != len(js) {
		return nil, arena.ErrInvalidSlot
	}
	header := uint64(reflect.TypeOf(equation{}).Size())
	if err = m.BudgetField.Reserve(header); err != nil {
		return nil, err
	}
	eq := &equation{manager: m, trace: tags, header: header}
	eq.traceAllocated()
	defer func() {
		if err != nil {
			if closeErr := eq.Close(); closeErr != nil {
				// The caller must retain an owner whose destruction stopped.
				result = eq
				err = errors.Join(err, closeErr)
			}
		}
	}()
	for i, x := range xs {
		if err = asBuffer.Set(x, &as[i]); err != nil {
			return nil, err
		}
		eq.as = append(eq.as, numeral.Int{})
		eq.xs = append(eq.xs, x)
	}
	if err = sortIntCore(eq.as, eq.xs, asBuffer); err != nil {
		return nil, err
	}
	if err = m.Integers().Copy(&eq.c, c); err != nil {
		return nil, err
	}
	for i, j := range js {
		if err = bsBuffer.Set(j, &bs[i]); err != nil {
			return nil, err
		}
		eq.bs = append(eq.bs, numeral.NewRational())
		eq.js = append(eq.js, j)
	}
	if sortJustifications {
		if err = sortRationalCore(eq.bs, eq.js, bsBuffer); err != nil {
			return nil, err
		}
	}
	return eq, nil
}

// Close follows del_eq: constant, integer coefficients, rational
// justifications, then equation storage. It may be retried after a failure.
func (e *equation) Close() error {
	if e == nil || e.closed {
		return nil
	}
	if !e.constantDone {
		if err := e.manager.Integers().Del(&e.c); err != nil {
			return err
		}
		e.constantDone = true
	}
	for e.asCursor < len(e.as) {
		if err := e.manager.Integers().Del(&e.as[e.asCursor]); err != nil {
			return err
		}
		e.asCursor++
	}
	for e.bsCursor < len(e.bs) {
		if err := e.manager.Del(&e.bs[e.bsCursor]); err != nil {
			return err
		}
		e.bsCursor++
	}
	e.as, e.xs, e.bs, e.js = nil, nil, nil, nil
	if e.header != 0 {
		e.manager.BudgetField.Release(e.header)
		e.traceDeallocated()
	}
	e.header = 0
	e.closed = true
	return nil
}
