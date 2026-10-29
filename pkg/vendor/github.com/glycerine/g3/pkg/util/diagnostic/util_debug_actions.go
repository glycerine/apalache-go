package diagnostic

// Source: src/util/debug.cpp:73-117; src/util/debug.h:42-56.

import (
	"errors"
	"os"
	"reflect"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

type DebugActionProfile uint8

const (
	DebugInteractive DebugActionProfile = iota
	DebugNoDebugger
)

var ErrDebugActionOwner = errors.New("g3: invalid debugger action owner")
var ErrDebugHostUnsupported = errors.New("g3: debugger host action profile is unsupported")

type DebugActionConfig struct {
	Budget  *arena.Budget
	Streams *MemoryStandardStreams
	Input   *NumericCharInput
	Profile DebugActionProfile
}

// This charged context owns the actual source command array and borrows the
// concrete input and standard streams. It never owns their files or lifetimes.
type DebugActionContext struct {
	self    *DebugActionContext
	budget  *arena.Budget
	streams *MemoryStandardStreams
	input   *NumericCharInput
	header  uint64
	profile DebugActionProfile
	command [1024]byte
	closed  bool
}

func NewDebugActionContext(c DebugActionConfig) (*DebugActionContext, error) {
	if c.Budget == nil || c.Profile > DebugNoDebugger {
		return nil, arena.ErrConfig
	}
	// The reached debugger uses the process's std::cin tied to its own
	// std::cout. Borrow exact owner identity without reading either lifetime.
	if c.Profile == DebugInteractive && (c.Streams == nil || c.Streams.self != c.Streams || c.Streams.budget != c.Budget || c.Input == nil || c.Input.self != c.Input || c.Input.budget != c.Budget || c.Input.tie != c.Streams.stdout) {
		return nil, arena.ErrConfig
	}
	n := uint64(reflect.TypeOf(DebugActionContext{}).Size())
	if e := c.Budget.Reserve(n); e != nil {
		return nil, e
	}
	a := &DebugActionContext{budget: c.Budget, streams: c.Streams, input: c.Input, header: n, profile: c.Profile}
	a.self = a
	return a, nil
}
func (a *DebugActionContext) open() error {
	if a == nil || a.self != a {
		return ErrDebugActionOwner
	}
	if a.closed {
		return arena.ErrClosed
	}
	return nil
}
func (a *DebugActionContext) Close() error {
	if a == nil || a.self != a {
		return ErrDebugActionOwner
	}
	if a.closed {
		return nil
	}
	a.budget.Release(a.header)
	a.closed = true
	return nil
}
func (a *DebugActionContext) emit(s string) error {
	out, e := a.streams.Stderr()
	if e != nil {
		return e
	}
	return out.AppendCString(s)
}
func (a *DebugActionContext) hostCommand() (string, error) {
	if e := a.open(); e != nil {
		return "", e
	}
	b := a.command[:0]
	b = append(b, "gdb -nw /proc/"...)
	b = strconv.AppendInt(b, int64(os.Getpid()), 10)
	b = append(b, "/exe "...)
	b = strconv.AppendInt(b, int64(os.Getpid()), 10)
	if len(b) >= len(a.command) {
		return "", arena.ErrBounds
	}
	a.command[len(b)] = 0
	return string(b), nil
}

// InvokeDebugger preserves prompt/extraction/switch order. Exit is the actual
// process action. The Linux null store raises Go's SIGSEGV diagnostic; its
// exit status differs from C++ undefined behavior.
func (a *DebugActionContext) InvokeDebugger() error {
	if e := a.open(); e != nil {
		return e
	}
	if a.profile == DebugNoDebugger {
		os.Exit(110)
	}
	for {
		if e := a.emit("(C)ontinue, (A)bort, (S)top, (T)hrow exception, Invoke (G)DB\n"); e != nil {
			return e
		}
		var result byte
		ok, e := a.input.ExtractChar(&result)
		if e != nil {
			return e
		}
		if !ok {
			os.Exit(110)
		}
		switch result {
		case 'C', 'c':
			return nil
		case 'A', 'a':
			os.Exit(1)
		case 'S', 's':
			return a.hostStop()
		case 'T', 't':
			return z3exception.NewDefaultException("assertion violation")
		case 'G', 'g':
			command, e := a.hostCommand()
			if e != nil {
				return e
			}
			if e = a.emit("invoking GDB...\n"); e != nil {
				return e
			}
			status, e := a.hostRunCommand(command)
			if e != nil {
				return e
			}
			if status == 0 {
				return a.emit("continuing the execution...\n")
			}
			if e = a.emit("error starting GDB...\n"); e != nil {
				return e
			}
			return a.hostStop()
		default:
			if e = a.emit("INVALID COMMAND\n"); e != nil {
				return e
			}
		}
	}
}
