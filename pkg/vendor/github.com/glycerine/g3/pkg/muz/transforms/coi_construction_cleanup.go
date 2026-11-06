package transforms

// Source: src/muz/transforms/dl_mk_coi_filter.cpp scoped intermediate rules,
// rule sets and extension-model converters.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
)

type coneConstructionFailure struct {
	manager        *ast.ASTManager
	runtime        *memory.Runtime
	rules          []*base.Rule
	sets           []*base.RuleSet
	steps          []ConeModelStep
	nested         memory.ConstructionFailure
	cause, cleanup error
	reported       error
	text           string
}

var _ memory.ConstructionFailure = (*coneConstructionFailure)(nil)

func (f *coneConstructionFailure) Error() string        { return f.text }
func (f *coneConstructionFailure) Unwrap() error        { return f.reported }
func (f *coneConstructionFailure) OriginalCause() error { return f.cause }
func (f *coneConstructionFailure) CleanupCause() error  { return f.cleanup }
func (f *coneConstructionFailure) RuntimeOwner() *memory.Runtime {
	if f.runtime != nil {
		return f.runtime
	}
	if f.manager == nil || f.manager.NumeralService() == nil {
		return nil
	}
	return f.manager.NumeralService().RuntimeOwner()
}

func (f *coneConstructionFailure) Cleanup() error {
	if f.nested != nil {
		f.cleanup = f.nested.Cleanup()
		if f.cleanup != nil {
			return f.cleanup
		}
		f.nested = nil
	}
	for len(f.rules) != 0 {
		last := len(f.rules) - 1
		if f.rules[last] != nil {
			f.cleanup = f.rules[last].Close()
			if f.cleanup != nil {
				return f.cleanup
			}
		}
		f.rules[last] = nil
		f.rules = f.rules[:last]
	}
	for len(f.sets) != 0 {
		last := len(f.sets) - 1
		if f.sets[last] != nil {
			f.cleanup = f.sets[last].Close()
			if f.cleanup != nil {
				return f.cleanup
			}
		}
		f.sets[last] = nil
		f.sets = f.sets[:last]
	}
	f.cleanup = closeConeModelSteps(f.steps)
	if f.cleanup == nil {
		f.steps = nil
	}
	return f.cleanup
}

// closeConeConstruction consumes temporary owners, transporting any refused
// cleanup behind the original failure so the caller can resume it exactly.
func closeConeConstruction(manager *ast.ASTManager, cause error, rules []*base.Rule, sets []*base.RuleSet, steps []ConeModelStep) error {
	if len(rules) == 0 && len(sets) == 0 && len(steps) == 0 {
		return cause
	}
	failure := &coneConstructionFailure{manager: manager, rules: rules, sets: sets, steps: steps, cause: cause}
	_ = errors.As(cause, &failure.nested)
	if manager == nil && failure.nested != nil {
		failure.runtime = failure.nested.RuntimeOwner()
	}
	cleanup := failure.Cleanup()
	if cleanup == nil {
		return cause
	}
	if failure.cause == nil {
		failure.cause = cleanup
	}
	failure.reported = errors.Join(cause, cleanup)
	failure.text = failure.reported.Error()
	return failure
}
