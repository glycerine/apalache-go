package diagnostic

// Source: src/util/debug.cpp:37-42; src/util/debug.h:58-65.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	utext "github.com/glycerine/g3/pkg/util/text"
)

var ErrDebugNotificationBinding = errors.New("g3: assertion notification streams are unbound")

// BindAssertionStreams borrows the actual standard-stream service. It never
// queries an assertion flag or the pointed stream's lifetime. The enclosing
// reflected DebugContext header charges this pointer exactly once.
func (d *DebugContext) BindAssertionStreams(s *MemoryStandardStreams) error {
	if e := d.open(); e != nil {
		return e
	}
	if s != nil && (s.self != s || s.budget != d.BudgetField) {
		return arena.ErrConfig
	}
	if d.assertionActions != nil && d.assertionActions.profile == DebugInteractive && d.assertionActions.streams != s {
		return arena.ErrConfig
	}
	d.assertionStreams = s
	return nil
}

// This concrete call-local selector holds borrowed source text only. It owns
// neither a spelling nor an output buffer; CString length is read at insertion.
type debugNotificationText struct {
	text     string
	region   *arena.BytesRegion
	pointer  utext.RegionCString
	borrowed bool
}

func (t debugNotificationText) insert(out *utext.NumericTextStream) error {
	if t.borrowed {
		return smallObjectCStringInto(out, t.region, t.pointer)
	}
	return out.AppendCString(t.text)
}
func (d *DebugContext) NotifyAssertionViolation(fileName string, line int32, condition string) error {
	return d.notifyAssertionViolation(debugNotificationText{text: fileName}, line, debugNotificationText{text: condition})
}
func (d *DebugContext) NotifyBorrowedAssertionViolation(fileRegion *arena.BytesRegion, fileName utext.RegionCString, line int32, conditionRegion *arena.BytesRegion, condition utext.RegionCString) error {
	return d.notifyAssertionViolation(debugNotificationText{region: fileRegion, pointer: fileName, borrowed: true}, line, debugNotificationText{region: conditionRegion, pointer: condition, borrowed: true})
}

// Source has four separate std::cerr statements. Each chain reaches the actual
// borrowed stderr binding and evaluates its own insertions in source order.
// The selected file engine's badbit suppresses emission, not later arguments.
func (d *DebugContext) notifyAssertionViolation(fileName debugNotificationText, line int32, condition debugNotificationText) error {
	if e := d.open(); e != nil {
		return e
	}
	if d.assertionStreams == nil {
		return ErrDebugNotificationBinding
	}
	out, e := d.assertionStreams.Stderr()
	if e != nil {
		return e
	}
	if e = out.AppendCString("ASSERTION VIOLATION\n"); e != nil {
		return e
	}
	out, e = d.assertionStreams.Stderr()
	if e != nil {
		return e
	}
	if e = out.AppendCString("File: "); e != nil {
		return e
	}
	if e = fileName.insert(out); e != nil {
		return e
	}
	if e = out.AppendCString("\n"); e != nil {
		return e
	}
	out, e = d.assertionStreams.Stderr()
	if e != nil {
		return e
	}
	if e = out.AppendCString("Line: "); e != nil {
		return e
	}
	if e = out.AppendInt32(line); e != nil {
		return e
	}
	if e = out.AppendCString("\n"); e != nil {
		return e
	}
	out, e = d.assertionStreams.Stderr()
	if e != nil {
		return e
	}
	if e = condition.insert(out); e != nil {
		return e
	}
	return out.AppendCString("\n")
}
