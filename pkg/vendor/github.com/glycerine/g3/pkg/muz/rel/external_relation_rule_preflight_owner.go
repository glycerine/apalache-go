package rel

// Source: src/muz/rel/dl_compiler.cpp make_add_constant_column and external
// full-column setup, specialized to the live external-rule executor.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/memory"
)

// externalRulePreflightOwner follows source construction order: singleton
// constants are built before full relations. Cleanup therefore drains a nested
// constructor receipt, then totals in reverse order, then singletons in reverse.
type externalRulePreflightOwner struct {
	plugin             *ExternalRelationPlugin
	nested             memory.ConstructionFailure
	totals, singletons []*ExternalRelation
}

func (o *externalRulePreflightOwner) close() error {
	if o.nested != nil {
		if err := o.nested.Cleanup(); err != nil {
			return err
		}
		o.nested = nil
	}
	for index := len(o.totals) - 1; index >= 0; index-- {
		if o.totals[index] == nil {
			continue
		}
		if err := o.totals[index].Close(); err != nil {
			return err
		}
		o.totals[index] = nil
	}
	for index := len(o.singletons) - 1; index >= 0; index-- {
		if o.singletons[index] == nil {
			continue
		}
		if err := o.singletons[index].Close(); err != nil {
			return err
		}
		o.singletons[index] = nil
	}
	return nil
}

type externalRulePreflightCleanupFailure struct {
	owner                 *externalRulePreflightOwner
	cause, cleanup, shown error
	message               string
}

var _ memory.ConstructionFailure = (*externalRulePreflightCleanupFailure)(nil)

func (f *externalRulePreflightCleanupFailure) Error() string        { return f.message }
func (f *externalRulePreflightCleanupFailure) Unwrap() error        { return f.shown }
func (f *externalRulePreflightCleanupFailure) OriginalCause() error { return f.cause }
func (f *externalRulePreflightCleanupFailure) CleanupCause() error  { return f.cleanup }
func (f *externalRulePreflightCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.plugin.ast.NumeralService().RuntimeOwner()
}
func (f *externalRulePreflightCleanupFailure) Cleanup() error {
	f.cleanup = f.owner.close()
	return f.cleanup
}

func finishExternalRulePreflight(owner *externalRulePreflightOwner, cause error) error {
	if cause != nil {
		errors.As(cause, &owner.nested)
	}
	cleanup := owner.close()
	if cleanup == nil {
		return cause
	}
	if cause == nil {
		cause = cleanup
		return &externalRulePreflightCleanupFailure{owner: owner, cause: cause, cleanup: cleanup, shown: cause, message: cause.Error()}
	}
	shown := errors.Join(cause, cleanup)
	return &externalRulePreflightCleanupFailure{owner: owner, cause: cause, cleanup: cleanup, shown: shown, message: shown.Error()}
}
