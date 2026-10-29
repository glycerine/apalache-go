package apalache

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"
)

// WriteCounterexample writes one Scala-compatible counterexample format to a
// caller-owned stream. A Trace keeps the constant initializer in state zero;
// the TLA, TLC, and legacy JSON formats expose subsequent states as State0...
func WriteCounterexample(output io.Writer, format string, trace Trace[Expr]) error {
	return writeCounterexampleAt(output, format, trace, time.Now())
}

func writeCounterexampleAt(output io.Writer, format string, trace Trace[Expr], now time.Time) error {
	if output == nil {
		return fmt.Errorf("counterexample writer has no output")
	}
	var data []byte
	var err error
	switch format {
	case "tla", "tlc", "json":
		states, conversionErr := counterexampleDecodedStates(trace)
		if conversionErr != nil {
			return conversionErr
		}
		switch format {
		case "tla":
			var text string
			text, err = formatCounterexampleTraceTLA(trace, states)
			if err == nil {
				text += "(* Created by Apalache on " + counterexampleTimestamp(now) + " *)\n" +
					"(* https://github.com/apalache-mc/apalache *)\n"
			}
			data = []byte(text)
		case "tlc":
			var text string
			text, err = FormatDecodedTraceTLC(trace.module, states)
			if err == nil {
				text = strings.Replace(text, "Created by Apalache\n",
					"Created by Apalache on "+counterexampleTimestamp(now)+"\n", 1)
			}
			data = []byte(text)
		case "json":
			data, err = MarshalDecodedTraceApalacheJSON(trace.module, states, trace.data)
			if err == nil {
				data, err = indentCounterexampleJSON(data)
			}
		}
	case "itf.json":
		data, err = MarshalITFCounterexample(trace, ITFOptions{})
	default:
		return fmt.Errorf("unknown counterexample format requested: %s", format)
	}
	if err != nil {
		return err
	}
	_, err = output.Write(data)
	return err
}

func indentCounterexampleJSON(compact []byte) ([]byte, error) {
	var output bytes.Buffer
	if err := json.Indent(&output, compact, "", "  "); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func counterexampleTimestamp(now time.Time) string {
	return now.Format("Mon Jan 02 15:04:05 MST 2006")
}

func formatCounterexampleTraceTLA(trace Trace[Expr], states []DecodedState) (string, error) {
	module, err := decodedTraceModule(trace.module, states, trace.data)
	if err != nil {
		return "", err
	}
	declarationIndex := 0
	return formatTLAModuleWith(module, TLAFormatOptions{ExtendedModules: []string{trace.module.Name()}},
		func(declaration Decl) (string, error) {
			formatted, formatErr := FormatTLADeclaration(declaration, nil)
			if formatErr != nil {
				return "", formatErr
			}
			comment := "The following formula holds true in the last state and violates the invariant"
			if declarationIndex == 0 {
				comment = "Constant initialization state" + traceLabelSuffix(trace.labels, 0)
			} else if declarationIndex <= len(states) {
				stateIndex := declarationIndex - 1
				if stateIndex == 0 {
					comment = "Initial state" + traceLabelSuffix(trace.labels, declarationIndex)
				} else {
					comment = fmt.Sprintf("State%d%s", stateIndex, traceLabelSuffix(trace.labels, declarationIndex))
				}
			}
			declarationIndex++
			return "(* " + comment + " *)\n" + formatted, nil
		})
}

func traceLabelSuffix(labels [][]string, index int) string {
	if index < 0 || index >= len(labels) || len(labels[index]) == 0 {
		return ""
	}
	return " [" + strings.Join(labels[index], ", ") + "]"
}

func counterexampleDecodedStates(trace Trace[Expr]) ([]DecodedState, error) {
	if trace.module == nil {
		return nil, fmt.Errorf("cannot write a counterexample without a module")
	}
	if len(trace.states) < 2 {
		return nil, fmt.Errorf("counterexample requires a constant initializer and at least one state")
	}
	constants := cloneTraceState(trace.states[0])
	states := make([]DecodedState, len(trace.states)-1)
	for index, source := range trace.states[1:] {
		values := cloneTraceState(source)
		if index == 0 {
			for name, value := range constants {
				values[name] = value
			}
		}
		states[index] = DecodedState{Values: values}
	}
	return states, nil
}

// WriteCounterexampleAllFormats writes TLA+, TLC output, legacy Apalache IR
// JSON, and ITF JSON into the configured run directory (and custom mirror).
func WriteCounterexampleAllFormats(
	manager *OutputManager,
	prefix string,
	suffix string,
	trace Trace[Expr],
) ([]string, error) {
	if manager == nil {
		return nil, fmt.Errorf("counterexample output manager is nil")
	}
	files := []struct{ format, name string }{
		{"tla", prefix + suffix + ".tla"},
		{"tlc", "MC" + prefix + suffix + ".out"},
		{"json", prefix + suffix + ".json"},
		{"itf.json", prefix + suffix + ".itf.json"},
	}
	runDir, hasRunDir := manager.RunDirPath()
	written := make([]string, 0, len(files))
	for _, file := range files {
		created, err := manager.WithWriterInRunDir([]string{file.name}, func(output io.Writer) error {
			return WriteCounterexample(output, file.format, trace)
		})
		if err != nil {
			return written, err
		}
		if created && hasRunDir {
			written = append(written, filepath.Clean(filepath.Join(runDir, file.name)))
		}
	}
	return written, nil
}
