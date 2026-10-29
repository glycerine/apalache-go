package nlsat

// Source: src/nlsat/nlsat_assignment.h:31-101.

import (
	"errors"
	"io"
	"math"
	"reflect"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// Assignment owns algebraic cells separately from its assigned flags. Reset
// clears flags only: hidden old values remain owned until replaced or closed.
// Its borrowed manager must outlive it. Borrowed cells are invalidated by growth.
type Assignment struct {
	self            *Assignment
	manager         *polynomial.AlgebraicManager
	search          *searchState
	budget          *arena.Budget
	values          []polynomial.Algebraic
	assigned        []bool
	header          uint64
	closing, closed bool
}

var _ polynomial.PolynomialAlgebraicEvaluation = (*Assignment)(nil)
var _ polynomial.PolynomialAlgebraicEvaluation = UndefVarAssignment{}

func NewAssignment(m *polynomial.AlgebraicManager) (*Assignment, error) {
	if m == nil {
		return nil, polynomial.ErrAlgebraicOwner
	}
	q, err := m.RationalManager()
	if err != nil {
		return nil, err
	}
	b := q.Budget()
	n := uint64(reflect.TypeOf(Assignment{}).Size())
	if err = b.Reserve(n); err != nil {
		return nil, err
	}
	a := &Assignment{manager: m, budget: b, header: n}
	a.self = a
	return a, nil
}
func (a *Assignment) open() error {
	if a == nil || a.self != a {
		return polynomial.ErrAlgebraicOwner
	}
	if a.closed || a.closing {
		return arena.ErrClosed
	}
	_, err := a.manager.RationalManager()
	return err
}
func (a *Assignment) Manager() (*polynomial.AlgebraicManager, error) {
	if err := a.open(); err != nil {
		return nil, err
	}
	return a.manager, nil
}
func (a *Assignment) growValues(n uint32) error {
	if uint64(n) > uint64(^uint(0)>>1) {
		return arena.ErrBounds
	}
	if int(n) <= len(a.values) {
		return nil
	}
	bytes := uint64(n) * uint64(reflect.TypeOf(polynomial.Algebraic{}).Size())
	if err := a.budget.Reserve(bytes); err != nil {
		return err
	}
	next := make([]polynomial.Algebraic, n)
	copy(next, a.values)
	clear(a.values) // move owners; leave no duplicated owning cells behind
	a.budget.Release(uint64(cap(a.values)) * uint64(reflect.TypeOf(polynomial.Algebraic{}).Size()))
	a.values = next
	return nil
}
func (a *Assignment) growFlags(n uint32) error {
	if uint64(n) > uint64(^uint(0)>>1) {
		return arena.ErrBounds
	}
	if int(n) <= len(a.assigned) {
		return nil
	}
	if int(n) <= cap(a.assigned) {
		old := len(a.assigned)
		a.assigned = a.assigned[:n]
		clear(a.assigned[old:])
		return nil
	}
	if err := a.budget.Reserve(uint64(n)); err != nil {
		return err
	}
	next := make([]bool, n)
	copy(next, a.assigned)
	a.budget.Release(uint64(cap(a.assigned)))
	a.assigned = next
	return nil
}
func (a *Assignment) reserve(x uint32) error {
	if x == math.MaxUint32 {
		return arena.ErrBounds
	}
	if err := a.growValues(x + 1); err != nil {
		return err
	}
	return a.growFlags(x + 1)
}
func (a *Assignment) Set(x uint32, value *polynomial.Algebraic) error {
	if err := a.open(); err != nil {
		return err
	}
	if value == nil {
		return polynomial.ErrAlgebraicOwner
	}
	if err := a.reserve(x); err != nil {
		return err
	}
	a.assigned[x] = true
	return a.manager.Copy(&a.values[x], value)
}

// SetCore exchanges the old slot with the supplied owning value, rather than
// copying it. The caller receives and must eventually delete that old value.
func (a *Assignment) SetCore(x uint32, value *polynomial.Algebraic) error {
	if err := a.open(); err != nil {
		return err
	}
	if value == nil {
		return polynomial.ErrAlgebraicOwner
	}
	if err := a.reserve(x); err != nil {
		return err
	}
	a.assigned[x] = true
	return a.manager.Swap(&a.values[x], value)
}
func (a *Assignment) Contains(x uint32) (bool, error) {
	if err := a.open(); err != nil {
		return false, err
	}
	return uint64(x) < uint64(len(a.assigned)) && a.assigned[x], nil
}
func (a *Assignment) ResetVar(x uint32) error {
	if err := a.open(); err != nil {
		return err
	}
	if uint64(x) < uint64(len(a.assigned)) {
		a.assigned[x] = false
	}
	return nil
}
func (a *Assignment) Reset() error {
	if err := a.open(); err != nil {
		return err
	}
	a.assigned = a.assigned[:0]
	return nil
}

// Value is the source's raw lookup, including retained unassigned cells.
func (a *Assignment) Value(x uint32) (*polynomial.Algebraic, error) {
	if err := a.open(); err != nil {
		return nil, err
	}
	if uint64(x) >= uint64(len(a.values)) {
		return nil, arena.ErrBounds
	}
	return &a.values[x], nil
}
func assignmentAssert(line int32, text string, ok bool) error {
	d := diagnostic.BorrowProcessDebugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(d)
	if err != nil || !enabled || ok {
		return err
	}
	return diagnostic.DebugAssertionFailure(d, "src/nlsat/nlsat_assignment.h", line, text)
}
func (a *Assignment) Algebraic(x uint32) (*polynomial.Algebraic, error) {
	ok, err := a.Contains(x)
	if err != nil {
		return nil, err
	}
	if err = assignmentAssert(71, "is_assigned(x)", ok); err != nil {
		return nil, err
	}
	return a.Value(x)
}
func (a *Assignment) SwapVars(x, y uint32) error {
	if err := a.open(); err != nil {
		return err
	}
	ok := uint64(x) < uint64(len(a.values)) && uint64(y) < uint64(len(a.values))
	if err := assignmentAssert(73, "x < m_values.size() && y < m_values.size()", ok); err != nil {
		return err
	}
	if !ok || uint64(x) >= uint64(len(a.assigned)) || uint64(y) >= uint64(len(a.assigned)) {
		return arena.ErrBounds
	}
	a.assigned[x], a.assigned[y] = a.assigned[y], a.assigned[x]
	a.values[x], a.values[y] = a.values[y], a.values[x]
	return nil
}
func (a *Assignment) Swap(other *Assignment) error {
	if err := a.open(); err != nil {
		return err
	}
	if err := other.open(); err != nil {
		return err
	}
	if a.manager != other.manager {
		return arena.ErrConfig
	}
	a.values, other.values = other.values, a.values
	a.assigned, other.assigned = other.assigned, a.assigned
	return nil
}
func (a *Assignment) Copy(other *Assignment) error {
	if err := a.open(); err != nil {
		return err
	}
	if err := other.open(); err != nil {
		return err
	}
	if a.manager != other.manager {
		return arena.ErrConfig
	}
	// Preserve the source self-copy behavior: clearing the shared flag vector
	// makes other empty too, while every old numerical slot stays allocated.
	a.assigned = a.assigned[:0]
	if err := a.growFlags(uint32(len(other.assigned))); err != nil {
		return err
	}
	copy(a.assigned, other.assigned)
	if err := a.growValues(uint32(len(a.assigned))); err != nil {
		return err
	}
	for i, assigned := range a.assigned {
		if assigned {
			if err := a.manager.Copy(&a.values[i], &other.values[i]); err != nil {
				return err
			}
		}
	}
	return nil
}
func (a *Assignment) Close() error {
	if a == nil || a.self != a {
		return polynomial.ErrAlgebraicOwner
	}
	if a.closed {
		return nil
	}
	if a.search != nil {
		return ErrSearchLive
	}
	a.closing = true
	// The Boolean vector is the younger member and dies before numerical
	// deletion. On a stopped close it stays retired while values remain owned.
	a.budget.Release(uint64(cap(a.assigned)))
	a.assigned = nil
	for i := range a.values {
		if err := a.manager.Del(&a.values[i]); err != nil {
			return err
		}
	}
	a.budget.Release(uint64(cap(a.values)) * uint64(reflect.TypeOf(polynomial.Algebraic{}).Size()))
	a.budget.Release(a.header)
	a.values = nil
	a.assigned = nil
	a.header = 0
	a.closed = true
	return nil
}

// UndefVarAssignment masks membership only. As in source, direct lookup still
// returns the underlying value even for the temporarily unassigned variable.
type UndefVarAssignment struct {
	Assignment *Assignment
	Variable   uint32
}

func (u UndefVarAssignment) Manager() (*polynomial.AlgebraicManager, error) {
	return u.Assignment.Manager()
}
func (u UndefVarAssignment) Contains(x uint32) (bool, error) {
	if x == u.Variable {
		return false, nil
	}
	return u.Assignment.Contains(x)
}
func (u UndefVarAssignment) Algebraic(x uint32) (*polynomial.Algebraic, error) {
	return u.Assignment.Value(x)
}

type assignmentWriter struct {
	out io.Writer
	err error
}

func (w *assignmentWriter) Write(p []byte) (int, error) {
	if w.err == nil {
		n, err := w.out.Write(p)
		if err == nil && n != len(p) {
			err = io.ErrShortWrite
		}
		w.err = err
	}
	return len(p), nil
}
func (a *Assignment) Display(out io.Writer) error {
	if err := a.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	w := &assignmentWriter{out: out}
	for i, assigned := range a.assigned {
		if assigned {
			_, _ = io.WriteString(w, "x"+strconv.Itoa(i)+" := ")
			if err := a.manager.DisplayDecimal(w, &a.values[i], 10); err != nil {
				return errors.Join(err, w.err)
			}
			_, _ = io.WriteString(w, "\n")
		}
	}
	return w.err
}
