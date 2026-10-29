package qe

// Source: src/qe/qe_lite.cpp fm::constraint and fm::fm retained constraint paths.

import (
	"errors"
	"io"
	"reflect"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// Native pointer identity replaces the source constraint ID/set indirection.
// The retained entry point always supplies null dependencies, including joins;
// no dependency payload or unused source mark bit is needed at this boundary.
type fmConstraint struct {
	owner                *fmStore
	literals             []int32
	variables            []uint32
	coefficients         []numeral.OwnedRational
	constant             numeral.OwnedRational
	strict, dead, closed bool
	bytes                uint64
}

type fmVariable struct {
	expression         ast.ASTRef
	integer, forbidden bool
	lowers, uppers     []*fmConstraint
}

type fmStore struct {
	self                 *fmStore
	recognizer           *fmRecognizer
	constraints          []*fmConstraint
	variables            []fmVariable
	variableIndex        map[ast.ASTHandle]uint32
	booleans             []ast.ASTRef
	booleanIndex         map[ast.ASTHandle]int32
	outputs              []ast.ASTRef
	todo                 []*fmConstraint
	todoPosition         map[*fmConstraint]int
	pending              []*fmScope
	counter              uint32
	inconsistent, closed bool
	bytes                uint64
}

type fmScope struct {
	store    *fmStore
	numbers  []*numeral.OwnedRational
	refs     []ast.ASTRef
	failures []memory.ConstructionFailure
}

func newFMStore(r *fmRecognizer) (*fmStore, error) {
	if r == nil || r.manager == nil || r.selection == nil {
		return nil, ast.ErrASTHandle
	}
	if err := r.selection.check(); err != nil {
		return nil, err
	}
	if r.selection.manager != r.manager {
		return nil, ast.ErrASTDomain
	}
	n := uint64(reflect.TypeOf(fmStore{}).Size())
	if err := r.manager.Budget().Reserve(n); err != nil {
		return nil, err
	}
	s := &fmStore{recognizer: r, bytes: n, variableIndex: make(map[ast.ASTHandle]uint32), booleanIndex: make(map[ast.ASTHandle]int32), booleans: []ast.ASTRef{{}}, todoPosition: make(map[*fmConstraint]int)}
	s.self = s
	return s, nil
}

func (s *fmStore) check() error {
	if s == nil || s.self != s || s.closed {
		return ast.ErrASTHandle
	}
	return nil
}
func (s *fmStore) begin() (*fmScope, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	if err := s.drain(); err != nil {
		return nil, err
	}
	c := &fmScope{store: s}
	s.pending = append(s.pending, c)
	return c, nil
}
func (c *fmScope) keep(n numeral.OwnedRational, err error) (*numeral.OwnedRational, error) {
	if n == (numeral.OwnedRational{}) {
		return nil, err
	}
	p := new(numeral.OwnedRational)
	*p = n
	c.numbers = append(c.numbers, p)
	return p, err
}
func (c *fmScope) number() (*numeral.OwnedRational, error) {
	return c.keep(c.store.recognizer.manager.NumeralService().NewValue())
}
func (c *fmScope) hold(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	r, err := c.store.recognizer.manager.NewRef(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	c.refs = append(c.refs, r)
	return h, nil
}
func (c *fmScope) close() error {
	for len(c.failures) != 0 {
		i := len(c.failures) - 1
		if err := c.failures[i].Cleanup(); err != nil {
			return err
		}
		c.failures = c.failures[:i]
	}
	for len(c.refs) != 0 {
		i := len(c.refs) - 1
		if c.refs[i] != (ast.ASTRef{}) {
			if err := c.store.recognizer.manager.DelRef(&c.refs[i]); err != nil {
				return err
			}
		}
		c.refs = c.refs[:i]
	}
	for len(c.numbers) != 0 {
		i := len(c.numbers) - 1
		if *c.numbers[i] != (numeral.OwnedRational{}) {
			if err := c.store.recognizer.manager.NumeralService().Del(c.numbers[i]); err != nil {
				return err
			}
		}
		c.numbers = c.numbers[:i]
	}
	return nil
}

type fmStoreFailure struct {
	owner          *fmStore
	cause, cleanup error
}

var _ memory.ConstructionFailure = (*fmStoreFailure)(nil)

func (e *fmStoreFailure) Error() string        { return e.Unwrap().Error() }
func (e *fmStoreFailure) Unwrap() error        { return errors.Join(e.cause, e.cleanup) }
func (e *fmStoreFailure) OriginalCause() error { return e.cause }
func (e *fmStoreFailure) CleanupCause() error  { return e.cleanup }
func (e *fmStoreFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.recognizer.manager.NumeralService().RuntimeOwner()
}
func (e *fmStoreFailure) Cleanup() error { return e.owner.drain() }

func (s *fmStore) finish(c *fmScope, err *error) {
	var failure memory.ConstructionFailure
	if errors.As(*err, &failure) {
		c.failures = append(c.failures, failure)
	}
	if cleanup := s.drain(); cleanup != nil {
		*err = &fmStoreFailure{owner: s, cause: *err, cleanup: cleanup}
	}
}
func (s *fmStore) drain() error {
	for len(s.pending) != 0 {
		i := len(s.pending) - 1
		if err := s.pending[i].close(); err != nil {
			return err
		}
		s.pending = s.pending[:i]
	}
	return nil
}

func (s *fmStore) newConstraint(literals []int32, variables []uint32, coefficients []*numeral.OwnedRational, constant *numeral.OwnedRational, strict bool) (*fmConstraint, error) {
	if len(variables) != len(coefficients) {
		return nil, ast.ErrASTDomain
	}
	bytes := uint64(reflect.TypeOf(fmConstraint{}).Size()) + uint64(len(literals))*4 + uint64(len(variables))*(4+uint64(reflect.TypeOf(numeral.OwnedRational{}).Size()))
	m := s.recognizer.manager
	if err := m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	c := &fmConstraint{owner: s, bytes: bytes, literals: append([]int32(nil), literals...), variables: append([]uint32(nil), variables...), coefficients: make([]numeral.OwnedRational, len(variables)), strict: strict}
	// Publish the actual partial owner before any numeral construction.
	s.constraints = append(s.constraints, c)
	n := m.NumeralService()
	for i, a := range coefficients {
		if err := s.recognizer.emit("mk_constraint", 1255, func(out io.Writer) error {
			_, _ = io.WriteString(out, "xs["+strconv.Itoa(i)+"]: "+strconv.FormatUint(uint64(variables[i]), 10)+"\n")
			return nil
		}); err != nil {
			return nil, err
		}
		var err error
		c.coefficients[i], err = n.NewValue()
		if err != nil {
			return nil, err
		}
		if err = n.Copy(&c.coefficients[i], a); err != nil {
			return nil, err
		}
	}
	var err error
	c.constant, err = n.NewValue()
	if err != nil {
		return nil, err
	}
	if err = n.Copy(&c.constant, constant); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *fmStore) deleteConstraint(c *fmConstraint) error {
	if c == nil || c.owner != s {
		return ast.ErrASTDomain
	}
	if c.closed {
		return nil
	}
	n := s.recognizer.manager.NumeralService()
	for i := len(c.coefficients) - 1; i >= 0; i-- {
		if c.coefficients[i] != (numeral.OwnedRational{}) {
			if err := n.Del(&c.coefficients[i]); err != nil {
				return err
			}
		}
	}
	if c.constant != (numeral.OwnedRational{}) {
		if err := n.Del(&c.constant); err != nil {
			return err
		}
	}
	s.eraseTodo(c)
	s.recognizer.manager.Budget().Release(c.bytes)
	c.bytes = 0
	c.literals = nil
	c.variables = nil
	c.coefficients = nil
	c.closed = true
	return nil
}

func (s *fmStore) insertTodo(c *fmConstraint) {
	if _, ok := s.todoPosition[c]; ok {
		return
	}
	s.todoPosition[c] = len(s.todo)
	s.todo = append(s.todo, c)
}
func (s *fmStore) eraseTodo(c *fmConstraint) {
	pos, ok := s.todoPosition[c]
	if !ok {
		return
	}
	delete(s.todoPosition, c)
	last := len(s.todo) - 1
	if pos != last {
		s.todo[pos] = s.todo[last]
		s.todoPosition[s.todo[pos]] = pos
	}
	s.todo = s.todo[:last]
}
func (s *fmStore) popTodo() *fmConstraint {
	if len(s.todo) == 0 {
		return nil
	}
	c := s.todo[len(s.todo)-1]
	s.eraseTodo(c)
	return c
}

func (s *fmStore) Close() error {
	if s == nil || s.self != s {
		return ast.ErrASTHandle
	}
	if s.closed {
		return nil
	}
	if err := s.drain(); err != nil {
		return err
	}
	for i := len(s.constraints) - 1; i >= 0; i-- {
		if err := s.deleteConstraint(s.constraints[i]); err != nil {
			return err
		}
	}
	m := s.recognizer.manager
	for i := len(s.outputs) - 1; i >= 0; i-- {
		if s.outputs[i] != (ast.ASTRef{}) {
			if err := m.DelRef(&s.outputs[i]); err != nil {
				return err
			}
		}
	}
	for i := len(s.variables) - 1; i >= 0; i-- {
		if s.variables[i].expression != (ast.ASTRef{}) {
			if err := m.DelRef(&s.variables[i].expression); err != nil {
				return err
			}
		}
	}
	for i := len(s.booleans) - 1; i > 0; i-- {
		if s.booleans[i] != (ast.ASTRef{}) {
			if err := m.DelRef(&s.booleans[i]); err != nil {
				return err
			}
		}
	}
	m.Budget().Release(s.bytes)
	s.bytes = 0
	s.constraints = nil
	s.variables = nil
	s.variableIndex = nil
	s.booleans = nil
	s.booleanIndex = nil
	s.outputs = nil
	s.todo = nil
	s.todoPosition = nil
	s.closed = true
	return nil
}
