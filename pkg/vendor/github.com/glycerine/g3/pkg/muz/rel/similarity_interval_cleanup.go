package rel

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/memory"
)

type intervalSimilarityConstructionFailure struct {
	owner                       *arithmeticSimilarityProgram
	nested                      memory.ConstructionFailure
	runtime                     *memory.Runtime
	original, cleanup, reported error
	text                        string
}

func (f *intervalSimilarityConstructionFailure) Error() string                 { return f.text }
func (f *intervalSimilarityConstructionFailure) Unwrap() error                 { return f.reported }
func (f *intervalSimilarityConstructionFailure) OriginalCause() error          { return f.original }
func (f *intervalSimilarityConstructionFailure) CleanupCause() error           { return f.cleanup }
func (f *intervalSimilarityConstructionFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *intervalSimilarityConstructionFailure) Cleanup() error {
	if f.nested != nil {
		f.cleanup = f.nested.Cleanup()
		if f.cleanup != nil {
			return f.cleanup
		}
		f.nested = nil
	}
	f.cleanup = f.owner.Close()
	return f.cleanup
}
func finishIntervalSimilarityConstruction(cause error, owner *arithmeticSimilarityProgram) error {
	f := &intervalSimilarityConstructionFailure{owner: owner, original: cause, runtime: owner.manager.NumeralService().RuntimeOwner()}
	errors.As(cause, &f.nested)
	cleanup := f.Cleanup()
	if cleanup == nil {
		return cause
	}
	f.reported = errors.Join(cause, cleanup)
	f.text = f.reported.Error()
	return f
}

var _ memory.ConstructionFailure = (*intervalSimilarityConstructionFailure)(nil)
