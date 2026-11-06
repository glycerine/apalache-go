package pdr

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/memory"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// pdrValidationAttempt is the concrete owner for temporary kernels and AST
// roots created by source result validation. Every frontier advances when a
// native decrement consumes its owner, even if the deletion transport refuses
// later cleanup work.
type pdrValidationAttempt struct {
	manager        *ast.ASTManager
	nested         memory.ConstructionFailure
	solvers        []*smtcore.SMTSolver
	evaluators     []*model.ModelEvaluator
	modelOwners    []*pdrModelExportOwner
	boogieProofs   []*base.BoogieProof
	abstractors    []*ast.ExprAbstractor
	streams        []*utext.NumericTextStream
	rules          []*base.Rule
	sideConditions []ast.ASTHandle
	references     []ast.ASTRef
	proofModes     []ast.ASTProofMode
}

func newPDRValidationAttempt(manager *ast.ASTManager) *pdrValidationAttempt {
	return &pdrValidationAttempt{manager: manager}
}

func (a *pdrValidationAttempt) Close() error {
	if a.nested != nil {
		if err := a.nested.Cleanup(); err != nil {
			return err
		}
		a.nested = nil
	}
	// display_certificate destroys its Boogie formatter before releasing the
	// source proof reference. BoogieProof.Close is itself replayable.
	for len(a.boogieProofs) != 0 {
		i := len(a.boogieProofs) - 1
		if err := a.boogieProofs[i].Close(); err != nil {
			return err
		}
		a.boogieProofs = a.boogieProofs[:i]
	}
	// Release scalar AST guards before composite services. A native callback
	// refuses here without partially tearing down an SMT solver whose Close
	// contract cannot replay already-consumed internal owners.
	for len(a.sideConditions) != 0 {
		i := len(a.sideConditions) - 1
		consumed, err := a.manager.DecRefConsumed(a.sideConditions[i])
		if consumed {
			a.sideConditions = a.sideConditions[:i]
		}
		if err != nil {
			return err
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	for len(a.references) != 0 {
		i := len(a.references) - 1
		err := a.manager.DelRef(&a.references[i])
		if a.references[i] == (ast.ASTRef{}) {
			a.references = a.references[:i]
		}
		if err != nil {
			return err
		}
		if len(a.references) != i {
			return ast.ErrASTPending
		}
	}
	for len(a.solvers) != 0 {
		i := len(a.solvers) - 1
		if err := a.solvers[i].Close(); err != nil {
			return err
		}
		a.solvers = a.solvers[:i]
	}
	for len(a.evaluators) != 0 {
		i := len(a.evaluators) - 1
		if err := a.evaluators[i].Close(); err != nil {
			return err
		}
		a.evaluators = a.evaluators[:i]
	}
	for len(a.modelOwners) != 0 {
		i := len(a.modelOwners) - 1
		if err := a.modelOwners[i].close(); err != nil {
			return err
		}
		a.modelOwners = a.modelOwners[:i]
	}
	for len(a.abstractors) != 0 {
		i := len(a.abstractors) - 1
		if err := a.abstractors[i].Close(); err != nil {
			return err
		}
		a.abstractors = a.abstractors[:i]
	}
	for len(a.streams) != 0 {
		i := len(a.streams) - 1
		if err := a.streams[i].Close(); err != nil {
			return err
		}
		a.streams = a.streams[:i]
	}
	for len(a.rules) != 0 {
		i := len(a.rules) - 1
		if err := a.rules[i].Close(); err != nil {
			return err
		}
		a.rules = a.rules[:i]
	}
	for len(a.proofModes) != 0 {
		i := len(a.proofModes) - 1
		if err := a.manager.ToggleProofMode(a.proofModes[i]); err != nil {
			return err
		}
		a.proofModes = a.proofModes[:i]
	}
	return nil
}

type pdrValidationCleanupFailure struct {
	cause, cleanupErr error
	exposed           error
	message           string
	attempt           *pdrValidationAttempt
}

func (f *pdrValidationCleanupFailure) Error() string { return f.message }
func (f *pdrValidationCleanupFailure) Unwrap() error { return f.exposed }
func (f *pdrValidationCleanupFailure) Cleanup() error {
	f.cleanupErr = f.attempt.Close()
	return f.cleanupErr
}
func (f *pdrValidationCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.attempt.manager.NumeralService().RuntimeOwner()
}
func (f *pdrValidationCleanupFailure) OriginalCause() error { return f.cause }
func (f *pdrValidationCleanupFailure) CleanupCause() error  { return f.cleanupErr }

func finishPDRValidation(attempt *pdrValidationAttempt, cause error) error {
	if cause != nil {
		var nested memory.ConstructionFailure
		if errors.As(cause, &nested) {
			attempt.nested = nested
		}
	}
	cleanup := attempt.Close()
	if cleanup == nil {
		return cause
	}
	exposed := cleanup
	if cause == nil {
		cause = cleanup
	} else {
		exposed = errors.Join(cause, cleanup)
	}
	return &pdrValidationCleanupFailure{cause: cause, cleanupErr: cleanup, exposed: exposed, message: exposed.Error(), attempt: attempt}
}

var _ memory.ConstructionFailure = (*pdrValidationCleanupFailure)(nil)
