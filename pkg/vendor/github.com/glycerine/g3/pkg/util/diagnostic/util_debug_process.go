package diagnostic

// Source: src/util/debug.cpp:28-65. The assertion switch and debug-tag table
// belong to the process, not to an individual heap instantiation.

import "github.com/glycerine/g3/pkg/arena"

var defaultProcessDebugContext = newProcessDebugContext()
var processDebugContext = defaultProcessDebugContext

func newProcessDebugContext() *DebugContext {
	b := arena.NewBudget(^uint64(0))
	r, err := arena.NewBytesRegion(arena.BytesRegionConfig{DirectOnly: true, Descriptors: arena.Config{Budget: b}})
	if err != nil {
		panic(err)
	}
	d, err := NewDebugContext(r, b)
	if err != nil {
		panic(err)
	}
	streams, err := NewMemoryStandardStreams(MemoryStandardStreamsConfig{Budget: b})
	if err != nil {
		panic(err)
	}
	input, err := NewDebuggerCharInput(streams, nil)
	if err != nil {
		panic(err)
	}
	actions, err := NewDebugActionContext(DebugActionConfig{Budget: b, Streams: streams, Input: input, Profile: DebugInteractive})
	if err != nil {
		panic(err)
	}
	if err = d.BindAssertionStreams(streams); err != nil {
		panic(err)
	}
	if err = d.BindAssertionActions(actions); err != nil {
		panic(err)
	}
	return d
}

// BorrowProcessDebugContext returns the current source-global assertion owner.
// The caller must not close it. Explicit generated-process initialization can
// temporarily bind its concrete debug owner until that binding is closed.
func BorrowProcessDebugContext() *DebugContext { return processDebugContext }

// BindProcessDebugContext changes only the borrowed process pointer and returns
// the previous owner for an exact restoration after generated finalization.
func BindProcessDebugContext(next *DebugContext) (*DebugContext, error) {
	if next == nil {
		return nil, arena.ErrConfig
	}
	if _, err := next.AssertionsEnabled(); err != nil {
		return nil, err
	}
	previous := processDebugContext
	processDebugContext = next
	return previous, nil
}
