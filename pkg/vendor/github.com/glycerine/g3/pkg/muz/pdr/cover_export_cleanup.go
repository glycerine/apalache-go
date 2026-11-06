package pdr

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/memory"
)

type pdrCoverExportOwner struct {
	manager         *ast.ASTManager
	substitution    ast.ASTExprSubstitution
	hasSubstitution bool
	replacer        *rewriter.DefaultExprReplacer
	formula         ast.ASTRef
	result          ast.ASTRef
}

type pdrCoverExportFailure struct {
	cause      error
	cleanupErr error
	owner      *pdrCoverExportOwner
	runtime    *memory.Runtime
}

func (f *pdrCoverExportFailure) Error() string { return errors.Join(f.cause, f.cleanupErr).Error() }
func (f *pdrCoverExportFailure) Unwrap() error { return errors.Join(f.cause, f.cleanupErr) }
func (f *pdrCoverExportFailure) Cleanup() error {
	if f == nil || f.owner == nil {
		return nil
	}
	f.cleanupErr = f.owner.close()
	if f.cleanupErr == nil {
		f.owner = nil
	}
	return f.cleanupErr
}
func (f *pdrCoverExportFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *pdrCoverExportFailure) OriginalCause() error          { return f.cause }
func (f *pdrCoverExportFailure) CleanupCause() error           { return f.cleanupErr }

func (o *pdrCoverExportOwner) closeServices() error {
	if o.replacer != nil {
		if err := o.replacer.Close(); err != nil {
			return err
		}
		o.replacer = nil
	}
	if o.hasSubstitution {
		if err := o.manager.DelExprSubstitution(&o.substitution); err != nil {
			return err
		}
		o.hasSubstitution = false
	}
	if o.formula != (ast.ASTRef{}) {
		if err := o.manager.DelRef(&o.formula); err != nil {
			return err
		}
	}
	return nil
}

func (o *pdrCoverExportOwner) close() error {
	if err := o.closeServices(); err != nil {
		return err
	}
	if o.result != (ast.ASTRef{}) {
		return o.manager.DelRef(&o.result)
	}
	return nil
}

func (o *pdrCoverExportOwner) finish(result *ast.ASTRef, resultErr *error) {
	cleanup := o.closeServices()
	if cleanup == nil && *resultErr == nil {
		return
	}
	if *result != (ast.ASTRef{}) {
		o.result = *result
		*result = ast.ASTRef{}
	}
	if cleanup == nil {
		cleanup = o.close()
	}
	if cleanup == nil {
		return
	}
	cause := *resultErr
	if cause == nil {
		cause = fmt.Errorf("pdr: close cover delta export: %w", cleanup)
	}
	*resultErr = &pdrCoverExportFailure{cause: cause, cleanupErr: cleanup, owner: o, runtime: o.manager.NumeralService().RuntimeOwner()}
}
