package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:461-467.

import (
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/messages"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

var ErrSubpavingMaxMemory = z3exception.NewDefaultException(messages.MaxMemory)

// sourceCheckpoint preserves the source order: resource increment, allocation
// check, then cooperate("subpaving"). A numeral runtime without a shared process
// still exposes its own concrete source allocation counter.
func sourceCheckpoint(limit *control.Reslimit, runtime *memory.Runtime, process *memory.ProcessAccounting, maxMemory uint64, canceled error, cooperation *cooperate.Cooperation) error {
	if !limit.Inc() {
		return canceled
	}
	if process != nil {
		if process.AllocationSize() > maxMemory {
			return ErrSubpavingMaxMemory
		}
	} else if runtime != nil {
		used, err := runtime.AllocationSize()
		if err != nil {
			return err
		}
		if used > maxMemory {
			return ErrSubpavingMaxMemory
		}
	}
	if cooperation != nil {
		return cooperation.Cooperate("subpaving")
	}
	return nil
}
