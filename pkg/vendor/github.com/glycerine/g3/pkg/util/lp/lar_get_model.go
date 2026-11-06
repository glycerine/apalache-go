package lp

// Source: src/util/lp/lar_solver.h:1278-1303.

import "github.com/glycerine/g3/pkg/util/numeral"

// LARRationalModel owns concrete column values in a borrowed manager. Values
// are borrowed for inspection; Close the model before closing its manager.
// GetModel preserves entries outside the source X extent. After an interrupted
// GetModel, close this output before its solver: a pending assignment can still
// borrow the solver's arithmetic local. Do not copy owners.
type LARRationalModel struct {
	Values  map[uint32]*numeral.Rational
	manager *numeral.RationalManager
	calls   exactEtaCalls
	stopped bool
}

func NewLARRationalModel(m *numeral.RationalManager) *LARRationalModel {
	return &LARRationalModel{Values: make(map[uint32]*numeral.Rational), manager: m}
}

func (m *LARRationalModel) set(j uint32, value *numeral.Rational) (err error) {
	if m.stopped || m.calls.pending() {
		return ErrExactVectorPending
	}
	defer func() { m.stopped = err != nil }()
	v := m.Values[j]
	if v == nil {
		zero := numeral.NewRational()
		v = &zero
		m.Values[j] = v // Source operator[] creates the entry before assignment.
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	err = m.manager.Copy(v, value)
	return m.calls.after(m.manager, before, err)
}

func (m *LARRationalModel) Close() error {
	if err := m.calls.cleanupRaw(m.manager); err != nil {
		return err
	}
	for j, value := range m.Values {
		if err := m.manager.Del(value); err != nil {
			return err
		}
		delete(m.Values, j)
	}
	return m.calls.cleanupRoots(m.manager)
}

// GetModel uses the actual strict-bound delta and halves it on a collision
// between distinct pairs. Earlier output prefixes survive retries and errors.
func (s *LARSolver) GetModel(out *LARRationalModel) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	if out == nil || out.manager != s.manager {
		return ErrExactVectorOwner
	}
	if out.stopped || out.calls.pending() {
		return ErrExactVectorPending
	}
	s.rationalWork = NewIndexedRationalVector(s.manager, 4)
	w := s.rationalWork.Data // delta, product, current single, constant two.
	before, e := s.calls.begin(s.manager)
	if e != nil {
		return e
	}
	err = func() error {
		if e := s.manager.SetInt32(&w[0], 1); e != nil {
			return e
		}
		if e := s.manager.SetInt32(&w[3], 2); e != nil {
			return e
		}
		return s.manager.Div(&w[0], &w[3], &w[0])
	}()
	if err = s.calls.after(s.manager, before, err); err != nil {
		return err
	}
	if err = s.Core.FindDeltaForStrictBounds(&w[0], &w[0]); err != nil {
		return err
	}
	if sourceDebugEnabled && s.Bounds.Status.Value != StatusOptimal {
		return ErrAssertion
	}
	for {
		s.modelPairs = NewIndexedPairVector(s.manager, 0)
		s.modelSingles = NewIndexedRationalVector(s.manager, 0)
		pairs := make(map[[2]uint32][]uint32)
		singles := make(map[uint32][]uint32)
		collision := false
		for j := range s.Core.RX.Data {
			rp := &s.Core.RX.Data[j]
			if err = s.insertModelPair(pairs, rp); err != nil {
				return err
			}
			if err = s.evaluateModelSingle(rp); err != nil {
				return err
			}
			if err = s.insertModelSingle(singles, &w[2]); err != nil {
				return err
			}
			collision = len(s.modelPairs.Data) != len(s.modelSingles.Data)
			if collision {
				before, e := s.calls.begin(s.manager)
				if e != nil {
					return e
				}
				e = s.manager.Div(&w[0], &w[3], &w[0])
				if err = s.calls.after(s.manager, before, e); err != nil {
					return err
				}
			} else if err = out.set(uint32(j), &w[2]); err != nil {
				return err
			}
			if err = s.manager.Del(&w[2]); err != nil {
				return err
			}
			w[2] = numeral.NewRational()
			if collision {
				break
			}
		}
		if err = s.closeModelSets(); err != nil {
			return err
		}
		if !collision {
			break
		}
	}
	if err = s.rationalWork.Close(); err != nil {
		return err
	}
	s.rationalWork = nil
	return nil
}

func (s *LARSolver) evaluateModelSingle(rp *RationalPair) (err error) {
	w := s.rationalWork.Data
	before, e := s.calls.begin(s.manager)
	if e != nil {
		return e
	}
	defer func() { err = s.calls.after(s.manager, before, err) }()
	if err = s.manager.Mul(&w[0], &rp.Y, &w[1]); err != nil {
		return err
	}
	if err = s.manager.Add(&rp.X, &w[1], &w[2]); err != nil {
		return err
	}
	if err = s.manager.Del(&w[1]); err != nil {
		return err
	}
	w[1] = numeral.NewRational()
	return nil
}

func (s *LARSolver) insertModelPair(buckets map[[2]uint32][]uint32, p *RationalPair) error {
	x, err := s.manager.Hash(&p.X)
	if err != nil {
		return err
	}
	y, err := s.manager.Hash(&p.Y)
	if err != nil {
		return err
	}
	key := [2]uint32{x, y}
	for _, i := range buckets[key] {
		equal, e := p.Equal(s.manager, &s.modelPairs.Data[i])
		if e != nil {
			return e
		}
		if equal {
			return nil
		}
	}
	i := uint32(len(s.modelPairs.Data))
	if err = s.modelPairs.resizeData(i + 1); err != nil {
		return err
	}
	if err = s.modelPairs.copySlot(&s.modelPairs.Data[i], p); err != nil {
		return err
	}
	buckets[key] = append(buckets[key], i)
	return nil
}

func (s *LARSolver) insertModelSingle(buckets map[uint32][]uint32, value *numeral.Rational) error {
	key, err := s.manager.Hash(value)
	if err != nil {
		return err
	}
	for _, i := range buckets[key] {
		equal, e := s.manager.Equal(value, &s.modelSingles.Data[i])
		if e != nil {
			return e
		}
		if equal {
			return nil
		}
	}
	i := uint32(len(s.modelSingles.Data))
	if err = s.modelSingles.resizeData(i + 1); err != nil {
		return err
	}
	if err = s.modelSingles.copySlot(&s.modelSingles.Data[i], value); err != nil {
		return err
	}
	buckets[key] = append(buckets[key], i)
	return nil
}

func (s *LARSolver) closeModelSets() error {
	if s.modelSingles != nil {
		if err := s.modelSingles.Close(); err != nil {
			return err
		}
		s.modelSingles = nil
	}
	if s.modelPairs != nil {
		if err := s.modelPairs.Close(); err != nil {
			return err
		}
		s.modelPairs = nil
	}
	return nil
}
