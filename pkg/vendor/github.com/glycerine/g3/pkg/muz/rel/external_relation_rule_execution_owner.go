package rel

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/memory"
)

// externalRuleExecutionOwner keeps unpublished rule-evaluation relations alive
// until their native Close frontier has completed. Published predicate
// relations remain owned by the plugin and are never added here.
type externalRuleExecutionOwner struct {
	runtime   *memory.Runtime
	nested    memory.ConstructionFailure
	relations []*ExternalRelation
}

func (o *externalRuleExecutionOwner) adopt(relation *ExternalRelation) *ExternalRelation {
	if relation != nil {
		o.relations = append(o.relations, relation)
	}
	return relation
}

func (o *externalRuleExecutionOwner) release(relation *ExternalRelation) *ExternalRelation {
	for index := len(o.relations) - 1; index >= 0; index-- {
		if o.relations[index] == relation {
			o.relations[index] = nil
			break
		}
	}
	return relation
}

func (o *externalRuleExecutionOwner) closeOne(relation *ExternalRelation) error {
	if relation == nil {
		return nil
	}
	for index := len(o.relations) - 1; index >= 0; index-- {
		if o.relations[index] != relation {
			continue
		}
		err := relation.Close()
		if err == nil {
			o.relations[index] = nil
		}
		return err
	}
	return relation.Close()
}

func (o *externalRuleExecutionOwner) close() error {
	if o.nested != nil {
		if err := o.nested.Cleanup(); err != nil {
			return err
		}
		o.nested = nil
	}
	for index := len(o.relations) - 1; index >= 0; index-- {
		if o.relations[index] == nil {
			continue
		}
		if err := o.relations[index].Close(); err != nil {
			return err
		}
		o.relations[index] = nil
	}
	o.relations = nil
	return nil
}

type externalRuleExecutionCleanupFailure struct {
	owner             *externalRuleExecutionOwner
	original, cleanup error
	reported          error
	text              string
}

var _ memory.ConstructionFailure = (*externalRuleExecutionCleanupFailure)(nil)

func (f *externalRuleExecutionCleanupFailure) Error() string        { return f.text }
func (f *externalRuleExecutionCleanupFailure) Unwrap() error        { return f.reported }
func (f *externalRuleExecutionCleanupFailure) OriginalCause() error { return f.original }
func (f *externalRuleExecutionCleanupFailure) CleanupCause() error  { return f.cleanup }
func (f *externalRuleExecutionCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.runtime
}
func (f *externalRuleExecutionCleanupFailure) Cleanup() error {
	f.cleanup = f.owner.close()
	return f.cleanup
}

func finishExternalRuleExecution(owner *externalRuleExecutionOwner, cause error) error {
	if cause != nil {
		errors.As(cause, &owner.nested)
	}
	cleanup := owner.close()
	if cleanup == nil {
		return cause
	}
	original := cause
	if original == nil {
		original = cleanup
	}
	reported := errors.Join(cause, cleanup)
	return &externalRuleExecutionCleanupFailure{
		owner: owner, original: original, cleanup: cleanup,
		reported: reported, text: reported.Error(),
	}
}
