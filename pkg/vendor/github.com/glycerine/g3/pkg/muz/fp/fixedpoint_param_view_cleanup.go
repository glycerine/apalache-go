package fp

import "github.com/glycerine/g3/pkg/util/memory"

// A parameter view is usually a local temporary. Transport its live decrement
// owner through the returned error when the caller's deferred Close refuses.
type fixedpointParamViewCleanupFailure struct {
	view    *fixedpointParamView
	runtime *memory.Runtime
	cause   error
	cleanup error
}

func (f *fixedpointParamViewCleanupFailure) Error() string                 { return f.cause.Error() }
func (f *fixedpointParamViewCleanupFailure) Unwrap() error                 { return f.cause }
func (f *fixedpointParamViewCleanupFailure) OriginalCause() error          { return f.cause }
func (f *fixedpointParamViewCleanupFailure) CleanupCause() error           { return f.cleanup }
func (f *fixedpointParamViewCleanupFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *fixedpointParamViewCleanupFailure) Cleanup() error {
	if f.view == nil {
		return nil
	}
	f.cleanup = f.view.closeOwned()
	if f.cleanup == nil {
		f.view = nil
	}
	return f.cleanup
}

func (v *fixedpointParamView) Close() error {
	if err := v.closeOwned(); err != nil {
		return &fixedpointParamViewCleanupFailure{
			view: v, runtime: v.context.NumeralsField.RuntimeOwner(), cause: err, cleanup: err,
		}
	}
	return nil
}
