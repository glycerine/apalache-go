package duality

// Source: src/duality/duality_wrapper.h (context, object, symbol).
// Original author: Ken McMillan.

import (
	"errors"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

var (
	ErrWrapperOwner    = errors.New("g3: invalid duality wrapper owner or lifetime")
	ErrContextMismatch = errors.New("g3: duality wrapper context mismatch")
)

// ContextFamilies records the family identities cached by the source context
// constructor. Family registration and plugin installation remain distinct.
type ContextFamilies struct {
	Basic, Array, Arithmetic, BitVector, Datatype, Datalog ast.FamilyID
}

// Context borrows an AST manager and owns its copy-constructed configuration.
// The caller must keep the manager and the configuration's ParamsContext alive
// until Close returns.
type Context struct {
	activeProfiler *Profiler
	self           *Context
	manager        *ast.ASTManager
	config         *Config
	families       ContextFamilies
	closed         bool
}

// NewContext follows the source member/body order. In particular, the
// arithmetic utility interns its family before the constructor body repeats
// that lookup. No declaration plugin is installed here.
func NewContext(manager *ast.ASTManager, config *Config) (*Context, error) {
	if manager == nil || config == nil {
		return nil, ErrWrapperOwner
	}
	if err := config.open(); err != nil {
		return nil, err
	}
	copyConfig, err := config.Clone()
	if err != nil {
		return nil, err
	}
	c := &Context{manager: manager, config: copyConfig}
	c.self = c
	fail := func(cause error) (*Context, error) {
		cleanup := copyConfig.Close()
		c.closed = true
		if cleanup != nil {
			return nil, errors.Join(cause, cleanup)
		}
		return nil, cause
	}
	if _, err = manager.EnsureFamilyID("arith"); err != nil {
		return fail(err)
	}
	c.families.Basic = manager.BasicFamilyID()
	if c.families.Arithmetic, err = manager.EnsureFamilyID("arith"); err != nil {
		return fail(err)
	}
	if c.families.BitVector, err = manager.EnsureFamilyID("bv"); err != nil {
		return fail(err)
	}
	if c.families.Array, err = manager.EnsureFamilyID("array"); err != nil {
		return fail(err)
	}
	if c.families.Datatype, err = manager.EnsureFamilyID("datatype"); err != nil {
		return fail(err)
	}
	if c.families.Datalog, err = manager.EnsureFamilyID("datalog_relation"); err != nil {
		return fail(err)
	}
	return c, nil
}

func (c *Context) owned() error {
	if c == nil || c.self != c || c.manager == nil || c.config == nil || c.closed {
		return ErrWrapperOwner
	}
	return nil
}

func (c *Context) open() error {
	if err := c.owned(); err != nil {
		return err
	}
	if err := c.config.open(); err != nil {
		return err
	}
	_, err := c.manager.FamilyName(c.families.Basic)
	return err
}

// Manager returns the borrowed source AST manager.
func (c *Context) Manager() (*ast.ASTManager, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return c.manager, nil
}

// Families returns the source constructor's cached identities.
func (c *Context) Families() (ContextFamilies, error) {
	if err := c.open(); err != nil {
		return ContextFamilies{}, err
	}
	return c.families, nil
}

// Config returns the borrowed context-owned configuration.
func (c *Context) Config() (*Config, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return c.config, nil
}

func (c *Context) SetString(name, value string) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.config.SetString(name, value)
}

func (c *Context) SetBool(name string, value bool) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.config.SetBool(name, value)
}

func (c *Context) SetInt(name string, value int) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.config.SetInt(name, value)
}

// Close destroys the configuration copy. The AST manager remains borrowed.
func (c *Context) Close() error {
	if err := c.owned(); err != nil {
		return err
	}
	err := c.config.Close()
	c.closed = true
	return err
}

// Object is the non-owning base of Duality wrapper values.
type Object struct {
	self    *Object
	context *Context
}

func NewObject(context *Context) (*Object, error) {
	if err := context.open(); err != nil {
		return nil, err
	}
	o := &Object{context: context}
	o.self = o
	return o, nil
}

func (o *Object) owned() error {
	if o == nil || o.self != o || o.context == nil {
		return ErrWrapperOwner
	}
	return nil
}

func (o *Object) open() error {
	if err := o.owned(); err != nil {
		return err
	}
	return o.context.open()
}

func (o *Object) Context() (*Context, error) {
	if err := o.open(); err != nil {
		return nil, err
	}
	return o.context, nil
}

func (o *Object) Manager() (*ast.ASTManager, error) {
	if err := o.open(); err != nil {
		return nil, err
	}
	return o.context.manager, nil
}

// Clone implements the source object copy constructor.
func (o *Object) Clone() (*Object, error) {
	if err := o.open(); err != nil {
		return nil, err
	}
	return NewObject(o.context)
}

// CheckContext implements the source assertion comparing wrapper contexts.
func CheckContext(a, b *Object) error {
	if err := a.open(); err != nil {
		return err
	}
	if err := b.open(); err != nil {
		return err
	}
	if a.context != b.context {
		return ErrContextMismatch
	}
	return nil
}

// Symbol is the Duality wrapper around a concrete source symbol.
type Symbol struct {
	object Object
	value  usymbol.Symbol
}

func newSymbol(context *Context, value usymbol.Symbol) (*Symbol, error) {
	if err := context.open(); err != nil {
		return nil, err
	}
	if err := context.manager.Symbols.Validate(value); err != nil {
		return nil, err
	}
	s := &Symbol{value: value}
	s.object = Object{context: context}
	s.object.self = &s.object
	return s, nil
}

// StringSymbol implements context::str_symbol.
func (c *Context) StringSymbol(value string) (*Symbol, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	symbol, err := c.manager.Symbols.Intern(value)
	if err != nil {
		return nil, err
	}
	return newSymbol(c, symbol)
}

// IntSymbol implements context::int_symbol, including the source signed-int
// to boxed-unsigned representation.
func (c *Context) IntSymbol(value int) (*Symbol, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return newSymbol(c, usymbol.Numerical(uint32(value)))
}

func (s *Symbol) open() error {
	if s == nil {
		return ErrWrapperOwner
	}
	return s.object.open()
}

func (s *Symbol) Object() (*Object, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	return &s.object, nil
}

// Assign implements symbol::operator=, transferring both context and value.
func (s *Symbol) Assign(other *Symbol) error {
	if err := s.open(); err != nil {
		return err
	}
	if err := other.open(); err != nil {
		return err
	}
	s.object.context = other.object.context
	s.value = other.value
	return nil
}

// Equal preserves source symbol identity comparison without imposing an
// additional same-context assertion.
func (s *Symbol) Equal(other *Symbol) (bool, error) {
	if err := s.open(); err != nil {
		return false, err
	}
	if err := other.open(); err != nil {
		return false, err
	}
	return s.value == other.value, nil
}

// String implements Duality::symbol::str. Unlike the ordinary symbol display,
// numerical values have no k! prefix and are read as signed source integers.
func (s *Symbol) String() (string, error) {
	if err := s.open(); err != nil {
		return "", err
	}
	if s.value.IsNumerical() {
		number, err := s.value.Number()
		if err != nil {
			return "", err
		}
		return strconv.FormatInt(int64(int32(number)), 10), nil
	}
	text, present, err := s.object.context.manager.Symbols.BareString(s.value)
	if err != nil {
		return "", err
	}
	if !present {
		return "", ErrWrapperOwner
	}
	return text, nil
}

// Write implements the wrapper's ostream insertion and preserves short/error
// writer prefixes.
func (s *Symbol) Write(w io.Writer) error {
	if w == nil {
		return ErrWrapperOwner
	}
	text, err := s.String()
	if err != nil {
		return err
	}
	n, err := io.WriteString(w, text)
	if err == nil && n != len(text) {
		return io.ErrShortWrite
	}
	return err
}
