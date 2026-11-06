package diagnostic

// Source: src/util/debug.h:60-65; src/util/debug.cpp:37-42,73-117.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

const DebugBuild = sourceDebug

var ErrDebugAssertion = errors.New("g3: source assertion failure")

// BindAssertionActions borrows identity without reaching action/input/file
// readiness. Notification and interactive output share one actual binding.
// NO_DEBUGGER has no stream/input borrow and erases them at its action site.

func (d *DebugContext) BindAssertionActions(a *DebugActionContext) error {
	if e := d.open(); e != nil {
		return e
	}
	if a != nil && (a.self != a || a.budget != d.BudgetField || a.profile == DebugInteractive && d.assertionStreams != a.streams) {
		return arena.ErrConfig
	}
	d.assertionActions = a
	return nil
}

// This leaf is reached only after the source macro switch and false condition.
// A real C action returns to the caller's next source statement. The unbound
// ErrDebugAssertion route preserves the legacy explicit Go assertion adapter.
func DebugAssertionFailure(d *DebugContext, file string, line int32, condition string) error {
	if d == nil || d.assertionActions == nil {
		return ErrDebugAssertion
	}
	if e := d.NotifyAssertionViolation(file, line, condition); e != nil {
		return e
	}
	return d.assertionActions.InvokeDebugger()
}

// HasAssertionActions reads the borrowed binding without reaching its lifetime.
func (d *DebugContext) HasAssertionActions() bool { return d != nil && d.assertionActions != nil }
