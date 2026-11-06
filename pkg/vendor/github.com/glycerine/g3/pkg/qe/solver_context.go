package qe

// Source: src/qe/qe.h:43-104,140-214; src/qe/qe.cpp:1244-1320.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

// SolverPlugin is the dependency-ready portion of qe_solver_plugin used by
// i_solver_context. Theory-specific branching, substitution, and projection
// remain on the concrete plugins that implement them.
//
// A SolverContext takes ownership after AddPlugin succeeds.
type SolverPlugin interface {
	FamilyID() ast.FamilyID
	MkAtom(expression ast.ASTHandle, positive bool) (result ast.ASTHandle, handled bool, err error)
	IsUninterpreted(application ast.ASTHandle) (bool, error)
	Close() error
}

// SolverContext is the common source context for QE solver plugins. The AST
// manager and variable handles are borrowed. Registered plugins are owned.
type SolverContext struct {
	self            *SolverContext
	manager         *ast.ASTManager
	variables       []ast.ASTHandle
	variableOwners  []ast.ASTRef
	contains        []*ast.ContainsApp
	currentFormula  ast.ASTRef
	plugins         []SolverPlugin
	closing         uint32
	containsClosing uint32
	closed          bool
}

func NewSolverContext(manager *ast.ASTManager, variables []ast.ASTHandle) (*SolverContext, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	c := &SolverContext{manager: manager, variables: append([]ast.ASTHandle(nil), variables...), variableOwners: make([]ast.ASTRef, len(variables))}
	c.self = c
	for _, variable := range c.variables {
		kind, err := manager.GetKind(variable)
		if err != nil || kind != ast.ASTApp {
			return nil, errors.Join(ast.ErrASTDomain, err)
		}
	}
	return c, nil
}

func (c *SolverContext) open() error {
	if c == nil || c.self != c {
		return ast.ErrASTHandle
	}
	if c.closed {
		return arena.ErrClosed
	}
	return nil
}

func (c *SolverContext) Manager() (*ast.ASTManager, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return c.manager, nil
}

func (c *SolverContext) NumVariables() (uint32, error) {
	if err := c.open(); err != nil {
		return 0, err
	}
	return uint32(len(c.variables)), nil
}

func (c *SolverContext) Variable(index uint32) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if index >= uint32(len(c.variables)) {
		return ast.ASTHandle{}, arena.ErrBounds
	}
	return c.variables[index], nil
}

// IsVariable preserves the source's first-match linear scan.
func (c *SolverContext) IsVariable(expression ast.ASTHandle) (index uint32, found bool, err error) {
	if err = c.open(); err != nil {
		return 0, false, err
	}
	for i, variable := range c.variables {
		if variable == expression {
			return uint32(i), true, nil
		}
	}
	return 0, false, nil
}

// AddPlugin publishes a plugin at its source family slot. Failed additions do
// not transfer ownership.
func (c *SolverContext) AddPlugin(plugin SolverPlugin) error {
	if err := c.open(); err != nil {
		return err
	}
	if plugin == nil {
		return ast.ErrASTHandle
	}
	family := plugin.FamilyID()
	if family == ast.NullFamilyID || family < 0 {
		return ast.ErrASTDomain
	}
	if int64(family) >= int64(^uint(0)>>1) {
		return arena.ErrBounds
	}
	for len(c.plugins) <= int(family) {
		c.plugins = append(c.plugins, nil)
	}
	if c.plugins[family] != nil {
		return ast.ErrASTDomain
	}
	c.plugins[family] = plugin
	return nil
}

func (c *SolverContext) Plugin(family ast.FamilyID) (SolverPlugin, bool, error) {
	if err := c.open(); err != nil {
		return nil, false, err
	}
	if family < 0 || int64(family) >= int64(len(c.plugins)) || c.plugins[family] == nil {
		return nil, false, nil
	}
	return c.plugins[family], true, nil
}

func (c *SolverContext) expressionSortFamily(expression ast.ASTHandle) (ast.FamilyID, error) {
	if kind, err := c.manager.GetKind(expression); err != nil {
		return ast.NullFamilyID, err
	} else if kind != ast.ASTApp {
		return ast.NullFamilyID, ast.ErrASTDomain
	}
	sort, err := c.manager.GetSort(expression)
	if err != nil {
		return ast.NullFamilyID, err
	}
	return c.manager.GetFamilyID(sort)
}

func (c *SolverContext) HasPlugin(expression ast.ASTHandle) (bool, error) {
	if err := c.open(); err != nil {
		return false, err
	}
	family, err := c.expressionSortFamily(expression)
	if err != nil {
		return false, err
	}
	return family >= 0 && int64(family) < int64(len(c.plugins)) && c.plugins[family] != nil, nil
}

func (c *SolverContext) PluginFor(expression ast.ASTHandle) (SolverPlugin, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	family, err := c.expressionSortFamily(expression)
	if err != nil {
		return nil, err
	}
	plugin, found, err := c.Plugin(family)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ast.ErrASTDomain
	}
	return plugin, nil
}

// MkAtom tries plugins in ascending family-slot order. The fallback is the
// original atom for positive polarity and an explicit negation otherwise.
func (c *SolverContext) MkAtom(expression ast.ASTHandle, positive bool) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	for _, plugin := range c.plugins {
		if plugin == nil {
			continue
		}
		result, handled, err := plugin.MkAtom(expression, positive)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if handled {
			return result, nil
		}
	}
	if positive {
		return expression, nil
	}
	return c.manager.MkNot(expression)
}

// Close follows the source destructor's ascending family order. A failing
// plugin remains published so cleanup can resume without skipping it.
func (c *SolverContext) Close() error {
	if c == nil || c.self != c {
		return ast.ErrASTHandle
	}
	if c.closed {
		return nil
	}
	for c.closing < uint32(len(c.plugins)) {
		plugin := c.plugins[c.closing]
		if plugin != nil {
			if err := plugin.Close(); err != nil {
				return err
			}
			c.plugins[c.closing] = nil
		}
		c.closing++
	}
	if err := c.closeMutationState(); err != nil {
		return err
	}
	c.variables = nil
	c.closed = true
	return nil
}
