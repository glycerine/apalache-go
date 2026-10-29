package tactic

// Source: src/tactic/goal.h.

import (
	"github.com/glycerine/g3/pkg/ast"
)

func (g *Goal) Manager() *ast.ASTManager { return g.manager }
func (g *Goal) CheckOwner() error        { return g.open() }
func (g *Goal) Inconsistent() (bool, error) {
	if e := g.open(); e != nil {
		return false, e
	}
	return g.inconsistent, nil
}
func (g *Goal) ModelsEnabled() (bool, error) {
	if e := g.open(); e != nil {
		return false, e
	}
	return g.modelsEnabled, nil
}
func (g *Goal) Precision() (GoalPrecision, error) {
	if e := g.open(); e != nil {
		return GoalPrecise, e
	}
	return g.precision, nil
}
func (g *Goal) ProofsEnabled() (bool, error) {
	if e := g.open(); e != nil {
		return false, e
	}
	return g.proofsEnabled, nil
}
func (g *Goal) UnsatCoresEnabled() (bool, error) {
	if e := g.open(); e != nil {
		return false, e
	}
	return g.coreEnabled, nil
}
func (g *Goal) Depth() (uint32, error) {
	if e := g.open(); e != nil {
		return 0, e
	}
	return g.depth, nil
}
