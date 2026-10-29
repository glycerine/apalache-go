package diagnostic

// Source: src/util/debug.h:63-92; src/util/error_codes.h:29.

import (
	"errors"
	"os"
)

var ErrDebugNativeUnreachable = errors.New("g3: reached GNU release __builtin_unreachable source undefined domain")

// InvokeAssertionDebugger is the actual action phase after notification and,
// for XASSERT, the caller's literal EXTRA block. It does not read the switch.
func (d *DebugContext) InvokeAssertionDebugger() error {
	if e := d.open(); e != nil {
		return e
	}
	if d.assertionActions == nil {
		return ErrDebugAssertion
	}
	return d.assertionActions.InvokeDebugger()
}

// UnreachableSource deliberately ignores assertions_enabled in DEBUG. The
// selected GNU release profile has no defined return path; its typed guard
// precedes every erased provider read, rather than pretending to continue.
func (d *DebugContext) UnreachableSource(file string, line int32) error {
	if !sourceDebug {
		return ErrDebugNativeUnreachable
	}
	if e := d.NotifyAssertionViolation(file, line, "UNREACHABLE CODE WAS REACHED."); e != nil {
		return e
	}
	return d.InvokeAssertionDebugger()
}
func (d *DebugContext) verificationMessage(expression string) error {
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
	if e = out.AppendCString("Failed to verify: "); e != nil {
		return e
	}
	if e = out.AppendCString(expression); e != nil {
		return e
	}
	return out.AppendCString("\n")
}

// These false leaves are called only after the caller evaluates its actual
// VERIFY/ENSURE condition. Both macros evaluate it in release as well as DEBUG;
// neither macro consults assertions_enabled or substitutes a callback.
func (d *DebugContext) VerifyFailure(file string, line int32, expression string) error {
	if e := d.verificationMessage(expression); e != nil {
		return e
	}
	return d.UnreachableSource(file, line)
}

func (d *DebugContext) EnsureFailure(expression string) error {
	if e := d.verificationMessage(expression); e != nil {
		return e
	}
	os.Exit(-1)
	return nil
}
func (d *DebugContext) NotImplementedYet(file string, line int32) error {
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
	if e = out.AppendCString("NOT IMPLEMENTED YET!\n"); e != nil {
		return e
	}
	if e = d.UnreachableSource(file, line); e != nil {
		return e
	}
	os.Exit(107)
	return nil
}
