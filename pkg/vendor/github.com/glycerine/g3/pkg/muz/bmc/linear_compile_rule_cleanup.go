package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp linear::compile rule-local
// var_subst and expr_ref lifetimes.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/memory"
)

type linearCompileRuleOwner struct {
	manager *ast.ASTManager
	nested  memory.ConstructionFailure
	sub     *rewriter.VarSubstitution
	refs    []ast.ASTRef
	result  ast.ASTRef
}

func (o *linearCompileRuleOwner) newSubstitution(standardOrder bool) error {
	var err error
	o.sub, err = rewriter.NewVarSubstitution(o.manager, standardOrder)
	return err
}

func (o *linearCompileRuleOwner) closeTemps() error {
	if o.nested != nil {
		if err := o.nested.Cleanup(); err != nil {
			return err
		}
		o.nested = nil
	}
	if o.sub != nil {
		if err := o.sub.Close(); err != nil {
			return err
		}
		o.sub = nil
	}
	return closeASTRefs(o.manager, o.refs)
}

func (o *linearCompileRuleOwner) closeAll() error {
	if err := o.closeTemps(); err != nil {
		return err
	}
	if o.result != (ast.ASTRef{}) {
		return o.manager.DelRef(&o.result)
	}
	return nil
}

type linearCompileRuleFailure struct {
	owner             *linearCompileRuleOwner
	original, cleanup error
	reported          error
	text              string
}

var _ memory.ConstructionFailure = (*linearCompileRuleFailure)(nil)

func (f *linearCompileRuleFailure) Error() string        { return f.text }
func (f *linearCompileRuleFailure) Unwrap() error        { return f.reported }
func (f *linearCompileRuleFailure) OriginalCause() error { return f.original }
func (f *linearCompileRuleFailure) CleanupCause() error  { return f.cleanup }
func (f *linearCompileRuleFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.manager.NumeralService().RuntimeOwner()
}
func (f *linearCompileRuleFailure) Cleanup() error {
	f.cleanup = f.owner.closeAll()
	return f.cleanup
}

func finishLinearCompileRule(owner *linearCompileRuleOwner, cause error) error {
	if cause != nil {
		errors.As(cause, &owner.nested)
	}
	cleanup := owner.closeTemps()
	if cleanup == nil && cause != nil {
		cleanup = owner.closeAll()
	}
	if cleanup == nil {
		return cause
	}
	original := cause
	if original == nil {
		original = cleanup
	}
	reported := errors.Join(cause, cleanup)
	return &linearCompileRuleFailure{owner: owner, original: original, cleanup: cleanup, reported: reported, text: reported.Error()}
}
