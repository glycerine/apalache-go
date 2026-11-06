package duality

// Source: src/duality/duality_wrapper.h (config, decl_kind, sort_kind).
// Original author: Ken McMillan.

import (
	"fmt"

	uparams "github.com/glycerine/g3/pkg/util/params"
)

// DeclKind is the compact operation classification used by the Duality
// wrapper. It deliberately does not expose the larger internal AST kind space.
type DeclKind uint8

const (
	DeclTrue DeclKind = iota
	DeclFalse
	DeclAnd
	DeclOr
	DeclNot
	DeclIff
	DeclIte
	DeclEqual
	DeclImplies
	DeclDistinct
	DeclXor
	DeclOeq
	DeclInterp
	DeclLeq
	DeclGeq
	DeclLt
	DeclGt
	DeclPlus
	DeclSub
	DeclUminus
	DeclTimes
	DeclDiv
	DeclIdiv
	DeclRem
	DeclMod
	DeclPower
	DeclToReal
	DeclToInt
	DeclIsInt
	DeclSelect
	DeclStore
	DeclConstArray
	DeclArrayDefault
	DeclArrayMap
	DeclSetUnion
	DeclSetIntersect
	DeclSetDifference
	DeclSetComplement
	DeclSetSubSet
	DeclAsArray
	DeclNumeral
	DeclForall
	DeclExists
	DeclVariable
	DeclUninterpreted
	DeclOtherBasic
	DeclOtherArith
	DeclOtherArray
	DeclOther
)

// SortKind is the source wrapper's deliberately small sort classification.
type SortKind uint8

const (
	SortBool SortKind = iota
	SortInt
	SortReal
	SortArray
	SortUninterpreted
	SortUnknown
)

// Config owns the params_ref used by a Duality context. The source relies on
// params_ref copy construction and destruction; Clone and Close make those
// actions explicit in Go.
type Config struct {
	self   *Config
	params *uparams.ParamsContext
	value  uparams.ParamsRef
	closed bool
}

// NewConfig constructs the source default empty configuration, or copy-
// constructs it from initial when supplied.
func NewConfig(params *uparams.ParamsContext, initial *uparams.ParamsRef) (*Config, error) {
	if params == nil {
		return nil, fmt.Errorf("invalid duality configuration")
	}
	value, err := params.NewValue()
	if err != nil {
		return nil, err
	}
	c := &Config{params: params, value: value}
	c.self = c
	if initial != nil {
		if err = params.Assign(&c.value, initial); err != nil {
			_ = params.Del(&c.value)
			c.closed = true
			return nil, err
		}
	}
	return c, nil
}

func (c *Config) open() error {
	if c == nil || c.self != c || c.params == nil || c.closed {
		return fmt.Errorf("invalid duality configuration")
	}
	return nil
}

// Clone performs the source config copy constructor.
func (c *Config) Clone() (*Config, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return NewConfig(c.params, &c.value)
}

// SetString implements config::set(char const*, char const*).
func (c *Config) SetString(name, value string) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.params.SetStringName(&c.value, &name, &value)
}

// SetBool implements config::set(char const*, bool).
func (c *Config) SetBool(name string, value bool) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.params.SetBoolName(&c.value, &name, value)
}

// SetInt implements config::set(char const*, int). The source passes the
// signed value to set_uint, so negative values retain the C++ unsigned wrap.
func (c *Config) SetInt(name string, value int) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.params.SetUIntName(&c.value, &name, uint32(value))
}

// Ref returns the configuration's borrowed params_ref for source consumers.
// It remains valid only until the next mutating call or Close.
func (c *Config) Ref() (*uparams.ParamsRef, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return &c.value, nil
}

// Close performs the source params_ref destructor.
func (c *Config) Close() error {
	if err := c.open(); err != nil {
		return err
	}
	err := c.params.Del(&c.value)
	c.closed = true
	return err
}
