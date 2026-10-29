package nlsat

// Source: src/nlsat/nlsat_interval_set.cpp:168-643,675-744.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *IntervalSetManager) lowerLower(a, b *infeasibleInterval) (int, error) {
	if a.lowerInf && b.lowerInf {
		return 0, nil
	}
	if a.lowerInf {
		return -1, nil
	}
	if b.lowerInf {
		return 1, nil
	}
	s, err := m.am.Compare(&a.lower, &b.lower)
	if err != nil || s != 0 {
		return s, err
	}
	if a.lowerOpen == b.lowerOpen {
		return 0, nil
	}
	if a.lowerOpen {
		return 1, nil
	}
	return -1, nil
}
func (m *IntervalSetManager) upperUpper(a, b *infeasibleInterval) (int, error) {
	if a.upperInf && b.upperInf {
		return 0, nil
	}
	if a.upperInf {
		return 1, nil
	}
	if b.upperInf {
		return -1, nil
	}
	s, err := m.am.Compare(&a.upper, &b.upper)
	if err != nil || s != 0 {
		return s, err
	}
	if a.upperOpen == b.upperOpen {
		return 0, nil
	}
	if a.upperOpen {
		return -1, nil
	}
	return 1, nil
}
func (m *IntervalSetManager) upperLower(a, b *infeasibleInterval) (int, error) {
	if a.upperInf || b.lowerInf {
		return 1, nil
	}
	s, err := m.am.Compare(&a.upper, &b.lower)
	if err != nil || s != 0 {
		return s, err
	}
	if !a.upperOpen && !b.lowerOpen {
		return 0, nil
	}
	return -1, nil
}
func (m *IntervalSetManager) adjacent(a, b *infeasibleInterval) (bool, error) {
	if a.upperInf || b.lowerInf {
		return false, arena.ErrBounds
	}
	s, err := m.am.Compare(&a.upper, &b.lower)
	if err != nil {
		return false, err
	}
	if s == 0 {
		return !a.upperOpen || !b.lowerOpen, nil
	}
	return false, nil
}
func (m *IntervalSetManager) MkUnion(s1, s2 *IntervalSet) (result *IntervalSet, err error) {
	return m.mkUnionWithReceipt(s1, s2, nil)
}
func (m *IntervalSetManager) mkUnionWithReceipt(s1, s2 *IntervalSet, receipt *intervalUnionReceipt) (result *IntervalSet, err error) {
	if err = m.open(); err != nil {
		return nil, err
	}
	if err = m.checkSet(s1); err != nil {
		return nil, err
	}
	if err = m.checkSet(s2); err != nil {
		return nil, err
	}
	if s1 == nil || s1 == s2 {
		return s2, nil
	}
	if s2 == nil {
		return s1, nil
	}
	if s1.full {
		return s1, nil
	}
	if s2.full {
		return s2, nil
	}
	b := &intervalBuffer{m: m}
	if receipt != nil {
		*receipt = intervalUnionReceipt{m, b}
	}
	defer m.finishBuffer(b, &err)
	i1, i2 := 0, 0
	for {
		if i1 >= len(s1.intervals) {
			for i2 < len(s2.intervals) {
				if err = b.push(&s2.intervals[i2]); err != nil {
					return nil, err
				}
				i2++
			}
			break
		}
		if i2 >= len(s2.intervals) {
			for i1 < len(s1.intervals) {
				if err = b.push(&s1.intervals[i1]); err != nil {
					return nil, err
				}
				i1++
			}
			break
		}
		a, c := &s1.intervals[i1], &s2.intervals[i2]
		ll, e := m.lowerLower(a, c)
		if e != nil {
			return nil, e
		}
		uu, e := m.upperUpper(a, c)
		if e != nil {
			return nil, e
		}
		if ll <= 0 {
			if uu == 0 {
				if err = b.push(a); err != nil {
					return nil, err
				}
				i1++
				i2++
			} else if uu > 0 {
				i2++
			} else {
				ul, e := m.upperLower(a, c)
				if e != nil {
					return nil, e
				}
				if ul < 0 {
					if err = b.push(a); err != nil {
						return nil, err
					}
					i1++
				} else if ul == 0 {
					if ll != 0 {
						cut := *a
						cut.upperOpen = true
						cut.upperInf = false
						if err = b.push(&cut); err != nil {
							return nil, err
						}
					}
					i1++
				} else {
					if ll != 0 {
						cut := *a
						cut.upperOpen = !c.lowerOpen
						cut.upperInf = false
						cut.upper = c.lower
						if err = b.push(&cut); err != nil {
							return nil, err
						}
					}
					i1++
				}
			}
		} else {
			if uu == 0 {
				if err = b.push(c); err != nil {
					return nil, err
				}
				i1++
				i2++
			} else if uu < 0 {
				i1++
			} else {
				ul, e := m.upperLower(c, a)
				if e != nil {
					return nil, e
				}
				if ul < 0 {
					if err = b.push(c); err != nil {
						return nil, err
					}
					i2++
				} else if ul == 0 {
					cut := *c
					cut.upperOpen = true
					cut.upperInf = false
					if err = b.push(&cut); err != nil {
						return nil, err
					}
					i2++
				} else {
					cut := *c
					cut.upperOpen = !a.lowerOpen
					cut.upperInf = false
					cut.upper = a.lower
					if err = b.push(&cut); err != nil {
						return nil, err
					}
					i2++
				}
			}
		}
	}
	if len(b.entries) == 0 {
		return nil, arena.ErrBounds
	}
	// Preserve the source compression swaps. Discarded endpoints remain owned
	// in the tail until their ordered deletion below.
	j := 0
	for i := 1; i < len(b.entries); i++ {
		curr, next := b.entries[j], b.entries[i]
		merge := false
		if curr.justification == next.justification {
			merge, err = m.adjacent(curr, next)
			if err != nil {
				return nil, err
			}
		}
		if merge {
			curr.upperInf = next.upperInf
			curr.upperOpen = next.upperOpen
			if err = m.am.Swap(&curr.upper, &next.upper); err != nil {
				return nil, err
			}
		} else {
			j++
			if i != j {
				dst := b.entries[j]
				dst.lowerInf = next.lowerInf
				dst.lowerOpen = next.lowerOpen
				if err = m.am.Swap(&dst.lower, &next.lower); err != nil {
					return nil, err
				}
				dst.upperInf = next.upperInf
				dst.upperOpen = next.upperOpen
				if err = m.am.Swap(&dst.upper, &next.upper); err != nil {
					return nil, err
				}
				dst.justification = next.justification
			}
		}
	}
	j++
	for i := j; i < len(b.entries); i++ {
		if err = m.am.Del(&b.entries[i].lower); err != nil {
			return nil, err
		}
		if err = m.am.Del(&b.entries[i].upper); err != nil {
			return nil, err
		}
	}
	b.entries = b.entries[:j]
	full := b.entries[0].lowerInf && b.entries[j-1].upperInf
	for i := 0; i+1 < j && full; i++ {
		full, err = m.adjacent(b.entries[i], b.entries[i+1])
		if err != nil {
			return nil, err
		}
	}
	return m.publish(b, full)
}
func (m *IntervalSetManager) Subset(s1, s2 *IntervalSet) (bool, error) {
	if err := m.open(); err != nil {
		return false, err
	}
	if err := m.checkSet(s1); err != nil {
		return false, err
	}
	if err := m.checkSet(s2); err != nil {
		return false, err
	}
	if s1 == s2 || s1 == nil {
		return true, nil
	}
	if s2 == nil {
		return false, nil
	}
	if s2.full {
		return true, nil
	}
	if s1.full {
		return false, nil
	}
	i1, i2 := 0, 0
	for i1 < len(s1.intervals) && i2 < len(s2.intervals) {
		a, b := &s1.intervals[i1], &s2.intervals[i2]
		ll, err := m.lowerLower(a, b)
		if err != nil {
			return false, err
		}
		if ll < 0 {
			return false, nil
		}
		for i2 < len(s2.intervals) {
			b = &s2.intervals[i2]
			uu, err := m.upperUpper(a, b)
			if err != nil {
				return false, err
			}
			if uu == 0 {
				i1++
				i2++
				break
			} else if uu < 0 {
				i1++
				break
			} else {
				ul, err := m.upperLower(b, a)
				if err != nil {
					return false, err
				}
				if ul < 0 {
					i2++
					break
				}
				if i2 == len(s2.intervals)-1 {
					return false, nil
				}
				adjacent, err := m.adjacent(b, &s2.intervals[i2+1])
				if err != nil {
					return false, err
				}
				if !adjacent {
					return false, nil
				}
				i2++
			}
		}
	}
	return i1 == len(s1.intervals), nil
}
func (m *IntervalSetManager) SetEq(s1, s2 *IntervalSet) (bool, error) {
	if err := m.open(); err != nil {
		return false, err
	}
	if err := m.checkSet(s1); err != nil {
		return false, err
	}
	if err := m.checkSet(s2); err != nil {
		return false, err
	}
	if s1 == nil || s2 == nil {
		return s1 == s2, nil
	}
	if s1.full || s2.full {
		return s1.full == s2.full, nil
	}
	ok, err := m.Subset(s1, s2)
	if err != nil || !ok {
		return ok, err
	}
	return m.Subset(s2, s1)
}
func (m *IntervalSetManager) Eq(s1, s2 *IntervalSet) (bool, error) {
	if err := m.open(); err != nil {
		return false, err
	}
	if err := m.checkSet(s1); err != nil {
		return false, err
	}
	if err := m.checkSet(s2); err != nil {
		return false, err
	}
	if s1 == nil || s2 == nil {
		return s1 == s2, nil
	}
	if len(s1.intervals) != len(s2.intervals) {
		return false, nil
	}
	for j := range s1.intervals {
		a, b := &s1.intervals[j], &s2.intervals[j]
		if a.lowerInf != b.lowerInf || a.upperInf != b.upperInf || a.lowerOpen != b.lowerOpen || a.upperOpen != b.upperOpen || a.justification != b.justification {
			return false, nil
		}
		cmp, err := m.am.Compare(&a.lower, &b.lower)
		if err != nil {
			return false, err
		}
		if cmp != 0 {
			return false, nil
		}
		cmp, err = m.am.Compare(&a.upper, &b.upper)
		if err != nil {
			return false, err
		}
		if cmp != 0 {
			return false, nil
		}
	}
	return true, nil
}
func (m *IntervalSetManager) PeekInComplement(s *IntervalSet, isInt bool, w *polynomial.Algebraic, randomize bool) (err error) {
	if err = m.open(); err != nil {
		return err
	}
	if err = m.checkSet(s); err != nil {
		return err
	}
	if w == nil {
		return arena.ErrConfig
	}
	if err = intervalAssert(676, "!is_full(s)", s == nil || !s.full); err != nil {
		return err
	}
	if s != nil && s.full {
		return arena.ErrBounds
	}
	if s == nil {
		q, e := m.am.RationalManager()
		if e != nil {
			return e
		}
		value := unumeral.NewRational()
		defer func() { err = errors.Join(err, q.Del(&value)) }()
		if randomize {
			num := int64(-1)
			if m.rand.Next()%2 == 0 {
				num = 1
			}
			denK := m.rand.Next() % 4
			den := uint64(1)
			if !isInt {
				den = 1 << denK
			}
			err = q.SetFraction(&value, num, den)
		} else {
			err = q.SetInt32(&value, 0)
		}
		if err != nil {
			return err
		}
		return m.am.SetRational(w, &value)
	}
	var n uint32
	num := len(s.intervals)
	if !s.intervals[0].lowerInf {
		n++
		if err = m.am.IntLT(&s.intervals[0].lower, w); err != nil {
			return err
		}
		if !randomize {
			return nil
		}
	}
	if !s.intervals[num-1].upperInf {
		n++
		if n == 1 || m.rand.Next()%n == 0 {
			if err = m.am.IntGT(&s.intervals[num-1].upper, w); err != nil {
				return err
			}
		}
		if !randomize {
			return nil
		}
	}
	for i := 1; i < num; i++ {
		cmp, e := m.am.Compare(&s.intervals[i-1].upper, &s.intervals[i].lower)
		if e != nil {
			return e
		}
		if cmp < 0 {
			n++
			if n == 1 || m.rand.Next()%n == 0 {
				if err = m.am.Select(&s.intervals[i-1].upper, &s.intervals[i].lower, w); err != nil {
					return err
				}
			}
			if !randomize {
				return nil
			}
		}
	}
	if n > 0 {
		return nil
	}
	irrational := -1
	for i := 1; i < num; i++ {
		if s.intervals[i-1].upperOpen && s.intervals[i].lowerOpen {
			rational, e := m.am.IsRational(&s.intervals[i-1].upper)
			if e != nil {
				return e
			}
			if rational {
				return m.am.Copy(w, &s.intervals[i-1].upper)
			}
			if irrational < 0 {
				irrational = i - 1
			}
		}
	}
	if irrational < 0 {
		return arena.ErrBounds
	}
	return m.am.Copy(w, &s.intervals[irrational].upper)
}
