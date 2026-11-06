package spacer

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

// literalExpansion owns the source extra_lits vector. Construction pins are
// released when finish publishes its roots, before candidate checks begin.
type literalExpansion struct {
	manager              *ast.ASTManager
	literals             []ast.ASTHandle
	refs                 []ast.ASTHandle
	original, cleanupErr error
	reported             error
	reportText           string
}

func (e *literalExpansion) keep(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = e.manager.IncRef(h); err != nil {
		return ast.ASTHandle{}, err
	}
	e.refs = append(e.refs, h)
	return h, nil
}

// finish transfers the construction scope to the source conjunction vector.
// Pin its roots before releasing replaced literals and temporary child pins.
func (e *literalExpansion) finish(literals []ast.ASTHandle) error {
	construction := literalExpansion{manager: e.manager, refs: e.refs}
	e.refs = nil
	for _, literal := range literals {
		if _, err := e.keep(literal, nil); err != nil {
			e.refs = append(e.refs, construction.refs...)
			return err
		}
	}
	err := construction.Close()
	e.refs = append(e.refs, construction.refs...)
	if err != nil {
		return err
	}
	e.literals = literals
	return nil
}

func (e *literalExpansion) Close() (result error) {
	for len(e.refs) != 0 {
		i := len(e.refs) - 1
		consumed, err := e.manager.DecRefConsumed(e.refs[i])
		result = errors.Join(result, err)
		if !consumed {
			return result
		}
		e.refs = e.refs[:i]
	}
	e.literals = nil
	return result
}

func finishLiteralExpansionFailure(e *literalExpansion, cause error) error {
	cleanup := e.Close()
	if cleanup == nil {
		return cause
	}
	reported := errors.Join(cause, cleanup)
	e.original, e.cleanupErr = cause, cleanup
	e.reported, e.reportText = reported, reported.Error()
	return e
}

func (e *literalExpansion) Error() string  { return e.reportText }
func (e *literalExpansion) Unwrap() error  { return e.reported }
func (e *literalExpansion) Cleanup() error { e.cleanupErr = e.Close(); return e.cleanupErr }
func (e *literalExpansion) RuntimeOwner() *memory.Runtime {
	return e.manager.NumeralService().RuntimeOwner()
}
func (e *literalExpansion) OriginalCause() error { return e.original }
func (e *literalExpansion) CleanupCause() error  { return e.cleanupErr }
