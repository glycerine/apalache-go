package diagnostic

// Source: src/util/memory_manager.cpp:33-72; src/util/memory_manager.h:49-71.
// Source: src/util/error_codes.h; src/util/z3_exception.h.
// Source exception policy for the concrete memory owner.

import (
	umemory "github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func memoryErrorTransport(f umemory.Failure) error {
	switch f.Kind {
	case umemory.FailureMemout:
		return z3exception.MustZ3Error(z3exception.ErrorMemout)
	case umemory.FailureAllocExceeded:
		return z3exception.MustZ3Error(z3exception.ErrorAllocExceeded)
	case umemory.FailureVectorOverflow:
		return z3exception.NewDefaultException(f.Message())
	}
	return umemory.ErrFailureTransport
}
func newUtilMemoryRuntime(c umemory.RuntimeConfig) (*umemory.Runtime, error) {
	if c.Transport == nil && c.FatalPolicy == nil {
		c.Transport = memoryErrorTransport
	}
	return umemory.NewRuntime(c)
}
