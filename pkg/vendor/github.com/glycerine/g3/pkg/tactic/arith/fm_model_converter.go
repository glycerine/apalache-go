package arith

// Source: src/tactic/arith/fm_tactic.cpp:40-242, the retained model converter.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
)

type FMModelConverter struct {
	self      *FMModelConverter
	manager   *ast.ASTManager
	services  model.ModelEvaluatorConfig
	variables []ast.ASTHandle
	clauses   [][]ast.ASTHandle
	header    uint64
	refs      uint32
	closed    bool
}

var _ tactic.ModelConverter = (*FMModelConverter)(nil)

func NewFMModelConverter(m *ast.ASTManager, s model.ModelEvaluatorConfig) (*FMModelConverter, error) {
	if m == nil || s.Context == nil || s.Globals == nil || s.Memory == nil || s.Cooperation == nil {
		return nil, ast.ErrASTHandle
	}
	n := uint64(reflect.TypeOf(FMModelConverter{}).Size())
	if e := m.Budget().Reserve(n); e != nil {
		return nil, e
	}
	c := &FMModelConverter{manager: m, services: s, header: n}
	c.self = c
	return c, nil
}
func (c *FMModelConverter) open() error {
	if c == nil || c.self != c || c.closed {
		return tactic.ErrModelConverterOwner
	}
	return nil
}
func (c *FMModelConverter) IncRef() error {
	if e := c.open(); e != nil {
		return e
	}
	c.refs++
	return nil
}
func (c *FMModelConverter) DecRef() error { _, e := c.DecRefConsumed(); return e }
func (c *FMModelConverter) DecRefConsumed() (bool, error) {
	if e := c.open(); e != nil {
		return false, e
	}
	if c.refs == 0 {
		return false, tactic.ErrModelConverterOwner
	}
	c.refs--
	if c.refs == 0 {
		return true, c.Close()
	}
	return true, nil
}
func (c *FMModelConverter) ApplyUnindexed(*tactic.ModelRef) error         { return c.open() }
func (c *FMModelConverter) ApplyLabels(*tactic.ModelLabels, uint32) error { return c.open() }
func (c *FMModelConverter) Cancel() error                                 { return c.open() }
func (c *FMModelConverter) Close() error {
	if c == nil || c.self != c {
		return tactic.ErrModelConverterOwner
	}
	if c.closed {
		return nil
	}
	for i, h := range c.variables {
		if !h.IsNull() {
			consumed, e := c.manager.DecRefConsumed(h)
			if consumed {
				c.variables[i] = ast.ASTHandle{}
			}
			if e != nil {
				return e
			}
		}
	}
	for i := range c.clauses {
		for j, h := range c.clauses[i] {
			if !h.IsNull() {
				consumed, e := c.manager.DecRefConsumed(h)
				if consumed {
					c.clauses[i][j] = ast.ASTHandle{}
				}
				if e != nil {
					return e
				}
			}
		}
	}
	c.clauses = nil
	c.variables = nil
	c.manager.Budget().Release(c.header)
	c.header = 0
	c.closed = true
	return nil
}
