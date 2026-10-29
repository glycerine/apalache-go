package euclid

// Source: src/math/euclid/euclidean_solver.cpp:329-419.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func sourceAddmulShapeAssertions(first, firstIDs, second, secondIDs int) error {
	d := diagnostic.BorrowProcessDebugContext()
	enabled, e := diagnostic.SourceAssertionsEnabled(d)
	if e != nil {
		return e
	}
	if enabled && first != firstIDs {
		if e := diagnostic.DebugAssertionFailure(d, "src/math/euclid/euclidean_solver.cpp", 352, "as1.size() == xs1.size()"); e != nil {
			return e
		}
	}
	enabled, e = diagnostic.SourceAssertionsEnabled(d)
	if e != nil {
		return e
	}
	if enabled && second != secondIDs {
		return diagnostic.DebugAssertionFailure(d, "src/math/euclid/euclidean_solver.cpp", 353, "as2.size() == xs2.size()")
	}
	return nil
}

// addmulInt is the reached mpz specialization of the source sorted-row merge.
// newAs and newXs are reusable output buffers, distinct from both input rows.
func (s *solverSetup) addmulInt(as1 []numeral.Int, xs1 []uint32, k *numeral.Int,
	as2 []numeral.Int, xs2 []uint32, newAs *numeral.ScopedIntVector,
	newXs *[]uint32, updateOcc, updateQueue bool, eqIndex, exceptVar uint32) (err error) {
	if err = s.open(); err != nil {
		return err
	}
	if len(as1) != len(xs1) || len(as2) != len(xs2) || k == nil || newAs == nil || newXs == nil {
		return arena.ErrInvalidSlot
	}
	z := s.manager.Integers()
	newA := numeral.NewInt(0)
	defer func() { err = errors.Join(err, z.Del(&newA)) }()
	if err = sourceAddmulShapeAssertions(len(as1), len(xs1), len(as2), len(xs2)); err != nil {
		return err
	}
	if err = newAs.Reset(); err != nil {
		return err
	}
	*newXs = (*newXs)[:0]
	i1, i2 := 0, 0
	for i1 < len(xs1) || i2 < len(xs2) {
		if i1 == len(xs1) || (i2 < len(xs2) && xs2[i2] < xs1[i1]) {
			x := xs2[i2]
			if err = s.noteNewTerm(x, updateOcc, updateQueue, eqIndex); err != nil {
				return err
			}
			zero := numeral.NewInt(0)
			if err = newAs.Push(&zero); err != nil {
				return err
			}
			var slot *numeral.Int
			if slot, err = newAs.BorrowSlot(uint32(len(*newXs))); err != nil {
				return err
			}
			if err = z.Mul(k, &as2[i2], slot); err != nil {
				return err
			}
			*newXs = append(*newXs, x)
			i2++
			continue
		}
		if i2 == len(xs2) || xs1[i1] < xs2[i2] {
			if err = newAs.Push(&as1[i1]); err != nil {
				return err
			}
			*newXs = append(*newXs, xs1[i1])
			i1++
			continue
		}
		x := xs1[i1]
		if err = z.AddMul(&as1[i1], k, &as2[i2], &newA); err != nil {
			return err
		}
		if err = s.traceIntAddMul(i1, i2, &newA, &as1[i1], k, &as2[i2]); err != nil {
			return err
		}
		var zero bool
		if zero, err = z.IsZero(&newA); err != nil {
			return err
		}
		if zero {
			if err = s.noteCanceledTerm(x, updateOcc, updateQueue, eqIndex, exceptVar); err != nil {
				return err
			}
		} else {
			if err = newAs.Push(&newA); err != nil {
				return err
			}
			*newXs = append(*newXs, x)
		}
		i1++
		i2++
	}
	return nil
}

// addmulRational is the reached mpq justification specialization. Its source
// call never updates occurrences or the queue.
func (s *solverSetup) addmulRational(bs1 []numeral.Rational, js1 []uint32, k *numeral.Int,
	bs2 []numeral.Rational, js2 []uint32, newBs *numeral.ScopedRationalVector,
	newJs *[]uint32) (err error) {
	if err = s.open(); err != nil {
		return err
	}
	if len(bs1) != len(js1) || len(bs2) != len(js2) || k == nil || newBs == nil || newJs == nil {
		return arena.ErrInvalidSlot
	}
	m := s.manager
	newB := numeral.NewRational()
	defer func() { err = errors.Join(err, m.Del(&newB)) }()
	if err = sourceAddmulShapeAssertions(len(bs1), len(js1), len(bs2), len(js2)); err != nil {
		return err
	}
	if err = newBs.Reset(); err != nil {
		return err
	}
	*newJs = (*newJs)[:0]
	i1, i2 := 0, 0
	for i1 < len(js1) || i2 < len(js2) {
		if i1 == len(js1) || (i2 < len(js2) && js2[i2] < js1[i1]) {
			zero := numeral.NewRational()
			if err = newBs.Push(&zero); err != nil {
				return err
			}
			var slot *numeral.Rational
			if slot, err = newBs.BorrowSlot(uint32(len(*newJs))); err != nil {
				return err
			}
			if err = m.MulInt(k, &bs2[i2], slot); err != nil {
				return err
			}
			*newJs = append(*newJs, js2[i2])
			i2++
			continue
		}
		if i2 == len(js2) || js1[i1] < js2[i2] {
			if err = newBs.Push(&bs1[i1]); err != nil {
				return err
			}
			*newJs = append(*newJs, js1[i1])
			i1++
			continue
		}
		if err = m.AddMulInt(&bs1[i1], k, &bs2[i2], &newB); err != nil {
			return err
		}
		if err = s.traceRationalAddMul(i1, i2, &newB, &bs1[i1], k, &bs2[i2]); err != nil {
			return err
		}
		var zero bool
		if zero, err = m.IsZero(&newB); err != nil {
			return err
		}
		if !zero {
			if err = newBs.Push(&newB); err != nil {
				return err
			}
			*newJs = append(*newJs, js1[i1])
		}
		i1++
		i2++
	}
	return nil
}

func (s *solverSetup) noteNewTerm(x uint32, updateOcc, updateQueue bool, eqIndex uint32) error {
	if x >= uint32(len(s.solved)) {
		return arena.ErrBounds
	}
	if updateOcc {
		s.occurrences[x] = append(s.occurrences[x], eqIndex)
	}
	if updateQueue && s.solved[x] != nullEquationIndex {
		return s.queue.Insert(int32(x))
	}
	return nil
}

func (s *solverSetup) noteCanceledTerm(x uint32, updateOcc, updateQueue bool, eqIndex, exceptVar uint32) error {
	if x >= uint32(len(s.solved)) {
		return arena.ErrBounds
	}
	if updateOcc && x != exceptVar {
		s.occurrences[x] = eraseOccurrenceValue(s.occurrences[x], eqIndex)
	}
	if updateQueue && s.solved[x] != nullEquationIndex && s.queue.Contains(int32(x)) {
		return s.queue.Erase(int32(x))
	}
	return nil
}
