package fp

// Display and rule enumeration borrow the persistent context while owning
// temporary rule snapshots and formula roots until native cleanup completes.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
)

type contextFormulaAttempt struct {
	manager      *ast.ASTManager
	properties   *base.RuleProperties
	sets         []*base.RuleSet
	rules        []*base.Rule
	references   []ast.ASTRef
	substitution *rewriter.VarSubstitution
	nested       memory.ConstructionFailure
}

func (a *contextFormulaAttempt) closeRules() error {
	for len(a.rules) != 0 {
		last := len(a.rules) - 1
		if a.rules[last] != nil {
			if err := a.rules[last].Close(); err != nil {
				return err
			}
		}
		a.rules = a.rules[:last]
	}
	return nil
}

func (a *contextFormulaAttempt) close() error {
	if a.nested != nil {
		if err := a.nested.Cleanup(); err != nil {
			return err
		}
		a.nested = nil
	}
	if a.properties != nil {
		if err := a.properties.Close(); err != nil {
			return err
		}
		a.properties = nil
	}
	if a.substitution != nil {
		if err := a.substitution.Close(); err != nil {
			return err
		}
		a.substitution = nil
	}
	if err := a.closeRules(); err != nil {
		return err
	}
	for len(a.sets) != 0 {
		last := len(a.sets) - 1
		if a.sets[last] != nil {
			if err := a.sets[last].Close(); err != nil {
				return err
			}
		}
		a.sets = a.sets[:last]
	}
	for len(a.references) != 0 {
		last := len(a.references) - 1
		if a.references[last] != (ast.ASTRef{}) {
			if err := a.manager.DelRef(&a.references[last]); err != nil {
				return err
			}
		}
		a.references = a.references[:last]
	}
	return nil
}

type contextFormulaFailure struct {
	attempt                  *contextFormulaAttempt
	runtime                  *memory.Runtime
	cause, cleanup, reported error
}

func (f *contextFormulaFailure) Error() string                 { return f.reported.Error() }
func (f *contextFormulaFailure) Unwrap() error                 { return f.reported }
func (f *contextFormulaFailure) OriginalCause() error          { return f.cause }
func (f *contextFormulaFailure) CleanupCause() error           { return f.cleanup }
func (f *contextFormulaFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *contextFormulaFailure) Cleanup() error                { f.cleanup = f.attempt.close(); return f.cleanup }

func (a *contextFormulaAttempt) finish(cause error) error {
	if cause != nil {
		errors.As(cause, &a.nested)
	}
	if cleanup := a.close(); cleanup != nil {
		return &contextFormulaFailure{
			attempt: a, runtime: a.manager.NumeralService().RuntimeOwner(),
			cause: cause, cleanup: cleanup, reported: errors.Join(cause, cleanup),
		}
	}
	return cause
}
