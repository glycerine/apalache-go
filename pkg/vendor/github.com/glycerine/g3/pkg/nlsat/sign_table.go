package nlsat

// Source: src/nlsat/nlsat_evaluator.cpp:41-366 (evaluator::imp::sign_table).

import (
	"errors"
	"io"
	"reflect"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type signSection struct {
	root     polynomial.Algebraic
	position uint32
}
type signPolynomial struct {
	numRoots, firstSection, firstSign uint32
}

// signTable owns one algebraic copy per distinct root. Sorted positions may
// change when a later polynomial contributes roots; stable section IDs do not.
type signTable struct {
	self                  *signTable
	am                    *polynomial.AlgebraicManager
	budget                *arena.Budget
	sections              []*signSection
	sorted, polySections  []uint32
	polySigns             []int
	info                  []signPolynomial
	newSorted, sectionIDs []uint32
	header                uint64
	failed, closed        bool
}

func newSignTable(am *polynomial.AlgebraicManager) (*signTable, error) {
	if am == nil {
		return nil, arena.ErrConfig
	}
	q, err := am.RationalManager()
	if err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(signTable{}).Size())
	if err = q.Budget().Reserve(n); err != nil {
		return nil, err
	}
	t := &signTable{am: am, budget: q.Budget(), header: n}
	t.self = t
	return t, nil
}
func (t *signTable) owner() error {
	if t == nil || t.self != t {
		return polynomial.ErrAlgebraicOwner
	}
	if t.closed {
		return arena.ErrClosed
	}
	_, err := t.am.RationalManager()
	return err
}
func (t *signTable) open() error {
	if err := t.owner(); err != nil {
		return err
	}
	if t.failed {
		return polynomial.ErrAlgebraicPending
	}
	return nil
}
func signCapacity(n int) (int, error) {
	if n == 0 {
		return 4, nil
	}
	if uint64(n) > uint64(^uint32(0))/2 || n > int(^uint(0)>>1)/2 {
		return 0, arena.ErrBounds
	}
	return n * 2, nil
}
func (t *signTable) appendIndex(dst *[]uint32, v uint32) error {
	if len(*dst) == cap(*dst) {
		n, err := signCapacity(cap(*dst))
		if err != nil {
			return err
		}
		if err = t.budget.Reserve(uint64(n) * 4); err != nil {
			return err
		}
		next := make([]uint32, len(*dst), n)
		copy(next, *dst)
		t.budget.Release(uint64(cap(*dst)) * 4)
		*dst = next
	}
	*dst = append(*dst, v)
	return nil
}
func (t *signTable) appendSign(v int) error {
	if len(t.polySigns) == cap(t.polySigns) {
		n, err := signCapacity(cap(t.polySigns))
		if err != nil {
			return err
		}
		size := uint64(reflect.TypeOf(int(0)).Size())
		if err = t.budget.Reserve(uint64(n) * size); err != nil {
			return err
		}
		next := make([]int, len(t.polySigns), n)
		copy(next, t.polySigns)
		t.budget.Release(uint64(cap(t.polySigns)) * size)
		t.polySigns = next
	}
	t.polySigns = append(t.polySigns, normalizeSign(v))
	return nil
}
func (t *signTable) appendInfo(v signPolynomial) error {
	if len(t.info) == cap(t.info) {
		n, err := signCapacity(cap(t.info))
		if err != nil {
			return err
		}
		size := uint64(reflect.TypeOf(signPolynomial{}).Size())
		if err = t.budget.Reserve(uint64(n) * size); err != nil {
			return err
		}
		next := make([]signPolynomial, len(t.info), n)
		copy(next, t.info)
		t.budget.Release(uint64(cap(t.info)) * size)
		t.info = next
	}
	t.info = append(t.info, v)
	return nil
}
func (t *signTable) mkSection(v *polynomial.Algebraic, pos uint32) (uint32, error) {
	if len(t.sections) == cap(t.sections) {
		n, err := signCapacity(cap(t.sections))
		if err != nil {
			return 0, err
		}
		size := uint64(reflect.TypeOf((*signSection)(nil)).Size())
		if err = t.budget.Reserve(uint64(n) * size); err != nil {
			return 0, err
		}
		next := make([]*signSection, len(t.sections), n)
		copy(next, t.sections)
		t.budget.Release(uint64(cap(t.sections)) * size)
		t.sections = next
	}
	if err := t.budget.Reserve(uint64(reflect.TypeOf(signSection{}).Size())); err != nil {
		return 0, err
	}
	id := uint32(len(t.sections))
	s := &signSection{}
	t.sections = append(t.sections, s)
	if err := t.am.Copy(&s.root, v); err != nil {
		return 0, err
	}
	s.position = pos
	return id, nil
}
func (t *signTable) merge(roots *polynomial.AlgebraicVector) error {
	t.newSorted = t.newSorted[:0]
	i1, i2, j := uint32(0), uint32(0), uint32(0)
	sz1, sz2 := uint32(len(t.sorted)), roots.Len()
	for i1 < sz1 && i2 < sz2 {
		id := t.sorted[i1]
		s := t.sections[id]
		r, err := roots.Borrow(i2)
		if err != nil {
			return err
		}
		c, err := t.am.Compare(&s.root, r)
		if err != nil {
			return err
		}
		if c == 0 {
			if err = t.appendIndex(&t.newSorted, id); err != nil {
				return err
			}
			if err = t.appendIndex(&t.sectionIDs, id); err != nil {
				return err
			}
			s.position = j
			i1++
			i2++
		} else if c < 0 {
			if err = t.appendIndex(&t.newSorted, id); err != nil {
				return err
			}
			s.position = j
			i1++
		} else {
			newID, err := t.mkSection(r, j)
			if err != nil {
				return err
			}
			if err = t.appendIndex(&t.newSorted, newID); err != nil {
				return err
			}
			if err = t.appendIndex(&t.sectionIDs, newID); err != nil {
				return err
			}
			i2++
		}
		j++
	}
	if err := atomAssert("nlsat_evaluator.cpp", 123, "i1 == sz1 || i2 == sz2", i1 == sz1 || i2 == sz2); err != nil {
		return err
	}
	for i1 < sz1 {
		id := t.sorted[i1]
		if err := t.appendIndex(&t.newSorted, id); err != nil {
			return err
		}
		t.sections[id].position = j
		i1++
		j++
	}
	for i2 < sz2 {
		r, err := roots.Borrow(i2)
		if err != nil {
			return err
		}
		id, err := t.mkSection(r, j)
		if err != nil {
			return err
		}
		if err = t.appendIndex(&t.newSorted, id); err != nil {
			return err
		}
		if err = t.appendIndex(&t.sectionIDs, id); err != nil {
			return err
		}
		i2++
		j++
	}
	t.sorted, t.newSorted = t.newSorted, t.sorted
	return nil
}
func (t *signTable) add(roots *polynomial.AlgebraicVector, signs []int) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if roots == nil || uint64(len(signs)) != uint64(roots.Len())+1 {
		return arena.ErrConfig
	}
	defer func() {
		if err != nil {
			t.failed = true
		}
	}()
	t.sectionIDs = t.sectionIDs[:0]
	if roots.Len() != 0 {
		if err = t.merge(roots); err != nil {
			return err
		}
	}
	firstSign, firstSection := uint32(len(t.polySigns)), uint32(len(t.polySections))
	for _, s := range signs {
		if err = t.appendSign(s); err != nil {
			return err
		}
	}
	for _, id := range t.sectionIDs {
		if err = t.appendIndex(&t.polySections, id); err != nil {
			return err
		}
	}
	if err = t.appendInfo(signPolynomial{roots.Len(), firstSection, firstSign}); err != nil {
		return err
	}
	return t.checkInvariant()
}
func (t *signTable) addConst(sign int) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			t.failed = true
		}
	}()
	firstSign, firstSection := uint32(len(t.polySigns)), uint32(len(t.polySections))
	if err = t.appendSign(sign); err != nil {
		return err
	}
	return t.appendInfo(signPolynomial{0, firstSection, firstSign})
}
func (t *signTable) numCells() uint32 { return uint32(len(t.sections))*2 + 1 }
func (t *signTable) rootID(c uint32) (uint32, error) {
	if err := t.open(); err != nil {
		return 0, err
	}
	if err := atomAssert("nlsat_evaluator.cpp", 187, "c < num_cells()", c < t.numCells()); err != nil {
		return 0, err
	}
	if err := atomAssert("nlsat_evaluator.cpp", 205, "is_section(c)", c%2 == 1); err != nil {
		return 0, err
	}
	if c >= t.numCells() || c%2 != 1 {
		return 0, arena.ErrBounds
	}
	return t.sorted[c/2], nil
}
func (t *signTable) root(id uint32) (*polynomial.Algebraic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if id == ^uint32(0) {
		return &polynomial.Algebraic{}, nil
	}
	if err := atomAssert("nlsat_evaluator.cpp", 215, "idx < m_sections.size()", uint64(id) < uint64(len(t.sections))); err != nil {
		return nil, err
	}
	if uint64(id) >= uint64(len(t.sections)) {
		return nil, arena.ErrBounds
	}
	return &t.sections[id].root, nil
}
func (t *signTable) cellID(p signPolynomial, i uint32) uint32 {
	return t.sections[t.polySections[p.firstSection+i]].position*2 + 1
}
func (t *signTable) signAt(info, c uint32) (int, error) {
	if err := t.open(); err != nil {
		return 0, err
	}
	if uint64(info) >= uint64(len(t.info)) || c >= t.numCells() {
		return 0, arena.ErrBounds
	}
	p := t.info[info]
	if p.numRoots < 8 {
		i := uint32(0)
		for ; i < p.numRoots; i++ {
			cell := t.cellID(p, i)
			if cell == c {
				return 0, nil
			}
			if cell > c {
				break
			}
		}
		return t.polySigns[p.firstSign+i], nil
	}
	first, last := t.cellID(p, 0), t.cellID(p, p.numRoots-1)
	if c < first {
		return t.polySigns[p.firstSign], nil
	}
	if c == first || c == last {
		return 0, nil
	}
	if c > last {
		return t.polySigns[p.firstSign+p.numRoots], nil
	}
	low, high := uint32(0), p.numRoots-1
	for {
		if err := atomAssert("nlsat_evaluator.cpp", 262, "0 <= low && high < static_cast<int>(num_roots)", high < p.numRoots); err != nil {
			return 0, err
		}
		if err := atomAssert("nlsat_evaluator.cpp", 263, "cell_id(pinfo, low) < c", t.cellID(p, low) < c); err != nil {
			return 0, err
		}
		if err := atomAssert("nlsat_evaluator.cpp", 264, "c < cell_id(pinfo, high)", c < t.cellID(p, high)); err != nil {
			return 0, err
		}
		if high == low+1 {
			if err := atomAssert("nlsat_evaluator.cpp", 266, "cell_id(pinfo, low) < c", t.cellID(p, low) < c); err != nil {
				return 0, err
			}
			if err := atomAssert("nlsat_evaluator.cpp", 267, "c < cell_id(pinfo, low+1)", c < t.cellID(p, low+1)); err != nil {
				return 0, err
			}
			return t.polySigns[p.firstSign+low+1], nil
		}
		if err := atomAssert("nlsat_evaluator.cpp", 270, "high > low + 1", high > low+1); err != nil {
			return 0, err
		}
		mid := low + (high-low)/2
		if err := atomAssert("nlsat_evaluator.cpp", 272, "low < mid && mid < high", low < mid && mid < high); err != nil {
			return 0, err
		}
		cell := t.cellID(p, mid)
		if cell == c {
			return 0, nil
		}
		if c < cell {
			high = mid
		} else {
			if err := atomAssert("nlsat_evaluator.cpp", 281, "mid_cell_id < c", cell < c); err != nil {
				return 0, err
			}
			low = mid
		}
	}
}

func (t *signTable) checkInvariant() error {
	enabled, err := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
	if err != nil || !enabled {
		return err
	}
	if err = atomAssert("nlsat_evaluator.cpp", 289, "m_sections.size() == m_sorted_sections.size()", len(t.sections) == len(t.sorted)); err != nil {
		return err
	}
	for i, id := range t.sorted {
		if err = atomAssert("nlsat_evaluator.cpp", 291, "m_sorted_sections[i] < m_sections.size()", uint64(id) < uint64(len(t.sections))); err != nil {
			return err
		}
		if uint64(id) >= uint64(len(t.sections)) {
			return arena.ErrBounds
		}
		if err = atomAssert("nlsat_evaluator.cpp", 292, "m_sections[m_sorted_sections[i]].m_pos == i", t.sections[id].position == uint32(i)); err != nil {
			return err
		}
	}
	var totalSections, totalSigns uint32
	for _, p := range t.info {
		if err = atomAssert("nlsat_evaluator.cpp", 297, "m_info[i].m_first_section <= m_poly_sections.size()", uint64(p.firstSection) <= uint64(len(t.polySections))); err != nil {
			return err
		}
		if err = atomAssert("nlsat_evaluator.cpp", 298, "m_info[i].m_num_roots == 0 || m_info[i].m_first_section < m_poly_sections.size()", p.numRoots == 0 || uint64(p.firstSection) < uint64(len(t.polySections))); err != nil {
			return err
		}
		if err = atomAssert("nlsat_evaluator.cpp", 299, "m_info[i].m_first_sign < m_poly_signs.size()", uint64(p.firstSign) < uint64(len(t.polySigns))); err != nil {
			return err
		}
		totalSections += p.numRoots
		totalSigns += p.numRoots + 1
	}
	if err = atomAssert("nlsat_evaluator.cpp", 303, "total_num_sections == m_poly_sections.size()", uint64(totalSections) == uint64(len(t.polySections))); err != nil {
		return err
	}
	return atomAssert("nlsat_evaluator.cpp", 304, "total_num_signs == m_poly_signs.size()", uint64(totalSigns) == uint64(len(t.polySigns)))
}
func (t *signTable) reset() error {
	if err := t.owner(); err != nil {
		return err
	}
	t.failed = true
	for i, s := range t.sections {
		if s == nil {
			continue
		}
		if err := t.am.Del(&s.root); err != nil {
			return err
		}
		t.budget.Release(uint64(reflect.TypeOf(signSection{}).Size()))
		t.sections[i] = nil
	}
	t.sections = t.sections[:0]
	t.sorted = t.sorted[:0]
	t.polySections = t.polySections[:0]
	t.polySigns = t.polySigns[:0]
	t.info = t.info[:0]
	t.failed = false
	return nil
}
func (t *signTable) close() error {
	if t != nil && t.self == t && t.closed {
		return nil
	}
	if err := t.reset(); err != nil {
		return err
	}
	t.budget.Release(uint64(cap(t.sections)) * uint64(reflect.TypeOf((*signSection)(nil)).Size()))
	t.budget.Release(uint64(cap(t.sorted)+cap(t.polySections)+cap(t.newSorted)+cap(t.sectionIDs)) * 4)
	t.budget.Release(uint64(cap(t.polySigns)) * uint64(reflect.TypeOf(int(0)).Size()))
	t.budget.Release(uint64(cap(t.info)) * uint64(reflect.TypeOf(signPolynomial{}).Size()))
	t.budget.Release(t.header)
	t.sections = nil
	t.sorted = nil
	t.polySections = nil
	t.newSorted = nil
	t.sectionIDs = nil
	t.polySigns = nil
	t.info = nil
	t.header = 0
	t.closed = true
	return nil
}
func (t *signTable) display(out io.Writer, raw bool) error {
	if err := t.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	w := &assignmentWriter{out: out}
	_, _ = io.WriteString(w, "sections:\n  ")
	for i, id := range t.sorted {
		if i > 0 {
			_, _ = io.WriteString(w, " < ")
		}
		if err := t.am.DisplayDecimal(w, &t.sections[id].root, 10); err != nil {
			return errors.Join(err, w.err)
		}
	}
	_, _ = io.WriteString(w, "\n")
	if raw {
		_, _ = io.WriteString(w, "poly_info:\n")
	} else {
		_, _ = io.WriteString(w, "sign variations:\n")
	}
	for i, p := range t.info {
		if raw {
			_, _ = io.WriteString(w, "  roots:")
			for j := uint32(0); j < p.numRoots; j++ {
				_, _ = io.WriteString(w, " "+strconv.FormatUint(uint64(t.polySections[p.firstSection+j]), 10))
			}
			_, _ = io.WriteString(w, ", signs:")
			for j := uint32(0); j <= p.numRoots; j++ {
				_, _ = io.WriteString(w, " "+signCharacter(t.polySigns[p.firstSign+j]))
			}
		} else {
			_, _ = io.WriteString(w, "  ")
			for c := uint32(0); c < t.numCells(); c++ {
				if c > 0 {
					_, _ = io.WriteString(w, " ")
				}
				s, err := t.signAt(uint32(i), c)
				if err != nil {
					return errors.Join(err, w.err)
				}
				_, _ = io.WriteString(w, signCharacter(s))
			}
		}
		_, _ = io.WriteString(w, "\n")
	}
	return w.err
}
func signCharacter(s int) string {
	if s < 0 {
		return "-"
	}
	if s == 0 {
		return "0"
	}
	return "+"
}
