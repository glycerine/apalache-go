package spacer

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

type arrayGeneralizerAttempt struct {
	manager                  *ast.ASTManager
	checker                  *VirtualSolver
	candidates, core, output []ast.ASTRef
}

func (o *arrayGeneralizerAttempt) closeRefs(values *[]ast.ASTRef) error {
	for len(*values) != 0 {
		index := len(*values) - 1
		if (*values)[index] == (ast.ASTRef{}) {
			*values = (*values)[:index]
			continue
		}
		err := o.manager.DelRef(&(*values)[index])
		if (*values)[index] == (ast.ASTRef{}) {
			*values = (*values)[:index]
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *arrayGeneralizerAttempt) close() error {
	if o.checker != nil {
		if err := o.checker.Close(); err != nil {
			return err
		}
		o.checker = nil
	}
	if err := o.closeRefs(&o.candidates); err != nil {
		return err
	}
	if err := o.closeRefs(&o.core); err != nil {
		return err
	}
	return o.closeRefs(&o.output)
}

type arrayGeneralizerCleanupFailure struct {
	owner             *arrayGeneralizerAttempt
	original, cleanup error
	reported          error
	text              string
}

var _ memory.ConstructionFailure = (*arrayGeneralizerCleanupFailure)(nil)

func (f *arrayGeneralizerCleanupFailure) Error() string        { return f.text }
func (f *arrayGeneralizerCleanupFailure) Unwrap() error        { return f.reported }
func (f *arrayGeneralizerCleanupFailure) OriginalCause() error { return f.original }
func (f *arrayGeneralizerCleanupFailure) CleanupCause() error  { return f.cleanup }
func (f *arrayGeneralizerCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.manager.NumeralService().RuntimeOwner()
}
func (f *arrayGeneralizerCleanupFailure) Cleanup() error {
	f.cleanup = f.owner.close()
	return f.cleanup
}

func finishArrayGeneralizerAttempt(owner *arrayGeneralizerAttempt, cause error) error {
	if cause == nil {
		published := owner.output
		owner.output = nil
		cleanup := owner.close()
		if cleanup == nil {
			return nil
		}
		owner.output = published
		return &arrayGeneralizerCleanupFailure{
			owner: owner, original: cleanup, cleanup: cleanup,
			reported: cleanup, text: cleanup.Error(),
		}
	}
	cleanup := owner.close()
	if cleanup == nil {
		return cause
	}
	reported := errors.Join(cause, cleanup)
	return &arrayGeneralizerCleanupFailure{
		owner: owner, original: cause, cleanup: cleanup,
		reported: reported, text: reported.Error(),
	}
}
