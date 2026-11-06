package shell

// Source: src/shell/main.cpp:113-289, concrete Linux command-line scan.

import (
	"errors"
	"math"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/text"
)

type InputKind uint8

const (
	InputUnspecified InputKind = iota
	InputSMT2
	InputDatalog
	InputDIMACS
	InputWCNF
	InputOPB
	InputZ3Log
	InputMPS
)

type CommandAction uint8

const (
	CommandContinue CommandAction = iota
	CommandHelp
	CommandVersion
	CommandDisplayParams
	CommandDisplayParamDescriptions
	CommandDisplayModules
	CommandDisplayModule
	CommandDisplayParameter
)

var ErrShellCommandLine = errors.New("shell: invalid command line option")
var ErrShellOptionArgument = errors.New("shell: required option argument is missing")
var ErrShellOptionService = errors.New("shell: option service is unavailable")

// CommandLine borrows the process services and owns only parser state. Args
// excludes argv[0], exactly as the source loop starts at argv[1].
type CommandLine struct {
	Input              *InputFileOption
	Timer              *TimeoutService
	Warnings           *diagnostic.WarningService
	Globals            *params.GlobalParamsService
	Trace              *diagnostic.TraceTags
	Debug              *diagnostic.DebugContext
	Kind               InputKind
	StandardInput      bool
	DisplayStatistics  bool
	DisplayIStatistics bool
	Action             CommandAction
	ActionArgument     string
}

func NewCommandLine(warnings *diagnostic.WarningService, timer *TimeoutService, globals *params.GlobalParamsService) *CommandLine {
	if warnings == nil {
		warnings = diagnostic.BorrowProcessWarningService()
	}
	if timer == nil {
		timer = BorrowProcessTimeoutService()
	}
	return &CommandLine{Input: NewInputFileOption(warnings), Timer: timer, Warnings: warnings, Globals: globals, Trace: diagnostic.BorrowProcessTraceTags()}
}

func shellCString(s string) string { return s[:text.CStringLength(s)] }

// sourceLongPrefix selects the signed decimal prefix used by strtol on the
// selected LP64 profile. Range saturation follows the source conversion.
func sourceLongPrefix(s string) int64 {
	s = shellCString(s)
	i := 0
	for i < len(s) && strings.ContainsRune(" \t\n\v\f\r", rune(s[i])) {
		i++
	}
	start := i
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	digits := i
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == digits {
		return 0
	}
	v, err := strconv.ParseInt(s[start:i], 10, 64)
	if err != nil {
		if s[start] == '-' {
			return math.MinInt64
		}
		return math.MaxInt64
	}
	return v
}

// Parse follows the source's terminal --, option, assignment, filename order.
// Display/help/version actions stop scanning at the source exit point; their
// output belongs to the still-unported executable frontend.
func (c *CommandLine) Parse(args []string) error {
	if c == nil || c.Input == nil || c.Warnings == nil {
		return ErrShellOptionService
	}
	for i := 0; i < len(args); i++ {
		arg := shellCString(args[i])
		if arg == "--" {
			return c.Input.AcceptAfterDoubleDash(args[i+1:])
		}
		if len(arg) != 0 && arg[0] == '-' {
			name := arg[1:]
			if len(name) != 0 && name[0] == '-' {
				name = name[1:]
			}
			var value *string
			if colon := strings.IndexByte(name, ':'); colon >= 0 {
				v := name[colon+1:]
				value = &v
				name = name[:colon]
			}
			switch name {
			case "h", "?", "help":
				c.Action = CommandHelp
				return nil
			case "version":
				c.Action = CommandVersion
				return nil
			case "smt2":
				c.Kind = InputSMT2
			case "dl":
				c.Kind = InputDatalog
			case "in":
				c.StandardInput = true
			case "dimacs":
				c.Kind = InputDIMACS
			case "wcnf":
				c.Kind = InputWCNF
			case "bpo":
				c.Kind = InputOPB
			case "log":
				c.Kind = InputZ3Log
			case "st":
				c.DisplayStatistics = true
			case "ist":
				c.DisplayIStatistics = true
			case "v":
				if value == nil {
					return sourceCommandLineError(ErrShellOptionArgument, "option argument (-v:level) is missing.")
				}
				if err := diagnostic.SetProcessVerbosityLevel(uint32(sourceLongPrefix(*value))); err != nil {
					return err
				}
			case "file":
				if err := c.Input.ApplyFileOption(value); err != nil {
					return err
				}
			case "T":
				if value == nil {
					return sourceCommandLineError(ErrMissingGlobalTimeout, ErrMissingGlobalTimeout.Error())
				}
				if c.Timer == nil {
					return ErrShellOptionService
				}
				if err := ApplyGlobalTimeoutOption(value, c.Timer); err != nil {
					return err
				}
			case "t", "memory":
				if value == nil {
					if name == "memory" {
						return sourceCommandLineError(ErrShellOptionArgument, "option argument (-memory:val) is missing.")
					}
					return sourceCommandLineError(ErrShellOptionArgument, "option argument (-t:timeout) is missing.")
				}
				if c.Globals == nil {
					return ErrShellOptionService
				}
				key := "timeout"
				if name == "memory" {
					key = "memory_max_size"
				}
				if err := c.Globals.SourceSetName(key, *value); err != nil {
					return err
				}
			case "nw":
				if err := c.Warnings.Enable(false); err != nil {
					return err
				}
			case "p":
				c.Action = CommandDisplayParams
				return nil
			case "pd":
				c.Action = CommandDisplayParamDescriptions
				return nil
			case "pm":
				if value == nil {
					c.Action = CommandDisplayModules
				} else {
					c.Action, c.ActionArgument = CommandDisplayModule, *value
				}
				return nil
			case "pp":
				if value == nil {
					return sourceCommandLineError(ErrShellOptionArgument, "option argument (-pp:name) is missing.")
				}
				c.Action, c.ActionArgument = CommandDisplayParameter, *value
				return nil
			case "tr":
				if !shellTraceOptions {
					return sourceCommandLineError(ErrShellCommandLine, "invalid command line option: "+arg)
				}
				if value == nil {
					return sourceCommandLineError(ErrShellOptionArgument, "option argument (-tr:tag) is missing.")
				}
				if c.Trace == nil {
					return ErrShellOptionService
				}
				c.Trace.Enable(*value)
			case "dbg":
				if !shellDebugOptions {
					return sourceCommandLineError(ErrShellCommandLine, "invalid command line option: "+arg)
				}
				if value == nil {
					return sourceCommandLineError(ErrShellOptionArgument, "option argument (-dbg:tag) is missing.")
				}
				if c.Debug == nil {
					return ErrShellOptionService
				}
				if err := c.Debug.EnableDebug(*value); err != nil {
					return err
				}
			default:
				return sourceCommandLineError(ErrShellCommandLine, "invalid command line option: "+arg)
			}
			continue
		}
		if len(arg) != 0 && arg[0] != '"' {
			if equal := strings.IndexByte(arg, '='); equal >= 0 {
				if c.Globals == nil {
					return ErrShellOptionService
				}
				if err := c.Globals.SourceSetName(arg[:equal], arg[equal+1:]); err != nil {
					return err
				}
				continue
			}
		}
		if err := c.Input.AcceptPositional(arg); err != nil {
			return err
		}
	}
	return nil
}
