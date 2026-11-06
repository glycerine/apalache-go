package qe

// Source: src/qe/qe_bool_plugin.cpp. Boolean branching, assignment,
// substitution, model projection, weight and source context solving use the
// shared safe replacer and the solver context's variable/elimination callbacks.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// BoolPlugin owns its safe replacer and borrows the solver context and manager.
type BoolPlugin struct {
	self     *BoolPlugin
	context  *SolverContext
	manager  *ast.ASTManager
	replacer *astrewriter.ExprSafeReplace
	closed   bool
}

func NewBoolPlugin(context *SolverContext) (*BoolPlugin, error) {
	if context == nil {
		return nil, ast.ErrASTHandle
	}
	manager, err := context.Manager()
	if err != nil {
		return nil, err
	}
	replacer, err := astrewriter.NewExprSafeReplace(manager)
	if err != nil {
		return nil, err
	}
	p := &BoolPlugin{context: context, manager: manager, replacer: replacer}
	p.self = p
	return p, nil
}

func (p *BoolPlugin) open() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return arena.ErrClosed
	}
	if p.context == nil || p.manager == nil || p.replacer == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (p *BoolPlugin) FamilyID() ast.FamilyID {
	if p == nil || p.manager == nil {
		return ast.NullFamilyID
	}
	return p.manager.BasicFamilyID()
}

// MkAtom retains qe_solver_plugin's default unhandled result.
func (p *BoolPlugin) MkAtom(ast.ASTHandle, bool) (ast.ASTHandle, bool, error) {
	if err := p.open(); err != nil {
		return ast.ASTHandle{}, false, err
	}
	return ast.ASTHandle{}, false, nil
}

func (p *BoolPlugin) IsUninterpreted(application ast.ASTHandle) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	kind, err := p.manager.GetKind(application)
	if err != nil {
		return false, err
	}
	if kind != ast.ASTApp {
		return false, ast.ErrASTDomain
	}
	return false, nil
}

func (p *BoolPlugin) checkInputs(variable *ast.ContainsApp, formula ast.ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	if variable == nil {
		return ast.ErrASTHandle
	}
	target, err := variable.Target()
	if err != nil {
		return err
	}
	if kind, cause := p.manager.GetKind(target); cause != nil || kind != ast.ASTApp {
		return errors.Join(ast.ErrASTDomain, cause)
	}
	if _, err = p.manager.GetKind(formula); err != nil {
		return err
	}
	return nil
}

// GetNumBranches returns the source's two Boolean co-factors.
func (p *BoolPlugin) GetNumBranches(variable *ast.ContainsApp, formula ast.ASTHandle, branches *numeral.OwnedRational) (bool, error) {
	if branches == nil {
		return false, ast.ErrASTHandle
	}
	if err := p.checkInputs(variable, formula); err != nil {
		return false, err
	}
	if err := p.manager.NumeralService().SetUnsigned(branches, 2); err != nil {
		return false, err
	}
	return true, nil
}

func (p *BoolPlugin) branchTruth(value *numeral.OwnedRational) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	if value == nil {
		return false, ast.ErrASTHandle
	}
	service := p.manager.NumeralService()
	// C++ only SASSERTs that this is zero or one. In release builds, both
	// assign and subst use is_one() as the branch test, so every other value
	// follows the false branch too.
	return service.IsOne(value)
}

// Assign has no side effects for Boolean branches; the source method only
// asserts that the branch is zero or one.
func (p *BoolPlugin) Assign(variable *ast.ContainsApp, formula ast.ASTHandle, value *numeral.OwnedRational) error {
	if err := p.checkInputs(variable, formula); err != nil {
		return err
	}
	_, err := p.branchTruth(value)
	return err
}

// Substitute replaces the selected Boolean application by its branch value.
// Formula and the optional definition are caller-owned references updated in
// the same order as the source: formula first, then realizer definition.
func (p *BoolPlugin) Substitute(variable *ast.ContainsApp, value *numeral.OwnedRational, formula ast.ASTRef, definition *ast.ASTRef) error {
	if err := p.open(); err != nil {
		return err
	}
	current, err := p.manager.RefGet(formula)
	if err != nil {
		return err
	}
	if err = p.checkInputs(variable, current); err != nil {
		return err
	}
	truth, err := p.branchTruth(value)
	if err != nil {
		return err
	}
	replacement, err := p.manager.MkBoolVal(truth)
	if err != nil {
		return err
	}
	target, err := variable.Target()
	if err != nil {
		return err
	}
	if err = p.replacer.ApplySubstitution(target, replacement, formula); err != nil {
		return err
	}
	if definition != nil {
		if err = p.manager.SetRef(*definition, replacement); err != nil {
			return err
		}
	}
	return nil
}

func (p *BoolPlugin) Close() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	if p.replacer != nil {
		if err := p.replacer.Close(); err != nil {
			return err
		}
		p.replacer = nil
	}
	p.closed = true
	return nil
}
