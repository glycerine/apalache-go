package pdr

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/memory"
)

type pdrModelExportOwner struct {
	manager         *ast.ASTManager
	model           *model.Model
	interpretations []*model.FunctionInterpretation
	abstractors     []*ast.ExprAbstractor
	references      []ast.ASTRef
}

type pdrModelExportFailure struct {
	cause      error
	cleanupErr error
	owner      *pdrModelExportOwner
	runtime    *memory.Runtime
}

func (f *pdrModelExportFailure) Error() string { return errors.Join(f.cause, f.cleanupErr).Error() }
func (f *pdrModelExportFailure) Unwrap() error { return errors.Join(f.cause, f.cleanupErr) }
func (f *pdrModelExportFailure) Cleanup() error {
	if f == nil || f.owner == nil {
		return nil
	}
	f.cleanupErr = f.owner.close()
	if f.cleanupErr == nil {
		f.owner = nil
	}
	return f.cleanupErr
}
func (f *pdrModelExportFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *pdrModelExportFailure) OriginalCause() error          { return f.cause }
func (f *pdrModelExportFailure) CleanupCause() error           { return f.cleanupErr }

func (o *pdrModelExportOwner) releaseLastReference() error {
	if len(o.references) == 0 {
		return nil
	}
	index := len(o.references) - 1
	if o.references[index] != (ast.ASTRef{}) {
		if err := o.manager.DelRef(&o.references[index]); err != nil {
			return err
		}
	}
	o.references = o.references[:index]
	return nil
}

func (o *pdrModelExportOwner) close() error {
	for len(o.interpretations) != 0 {
		index := len(o.interpretations) - 1
		if o.interpretations[index] != nil {
			if err := o.interpretations[index].Close(); err != nil {
				return err
			}
		}
		o.interpretations = o.interpretations[:index]
	}
	if o.model != nil {
		if err := o.model.Close(); err != nil {
			return err
		}
		o.model = nil
	}
	for len(o.references) != 0 {
		if err := o.releaseLastReference(); err != nil {
			return err
		}
	}
	for len(o.abstractors) != 0 {
		index := len(o.abstractors) - 1
		if err := o.abstractors[index].Close(); err != nil {
			return err
		}
		o.abstractors = o.abstractors[:index]
	}
	return nil
}

func (o *pdrModelExportOwner) releaseTemporaries() error {
	for len(o.references) != 0 {
		if err := o.releaseLastReference(); err != nil {
			return err
		}
	}
	for len(o.abstractors) != 0 {
		index := len(o.abstractors) - 1
		if err := o.abstractors[index].Close(); err != nil {
			return err
		}
		o.abstractors = o.abstractors[:index]
	}
	return nil
}

func (o *pdrModelExportOwner) dispose(cause error) error {
	cleanup := o.close()
	if cleanup == nil {
		return cause
	}
	return &pdrModelExportFailure{cause: cause, cleanupErr: cleanup, owner: o, runtime: o.manager.NumeralService().RuntimeOwner()}
}
