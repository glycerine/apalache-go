package oraclectl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Verdict string

const (
	VerdictSafe      Verdict = "safe"
	VerdictViolation Verdict = "violation"
	VerdictDeadlock  Verdict = "deadlock"
	VerdictUnknown   Verdict = "unknown"
	VerdictError     Verdict = "error"
)

type Outcome struct {
	Verdict  Verdict
	Property string
	Depth    int
	ExitCode int
	Trace    []string
	RawTrace []byte
	Stdout   string
	Stderr   string
}

type Command struct {
	Executable string
	Args       []string
	Dir        string
	Env        []string
}

type Case struct {
	Name             string   `json:"name"`
	Dir              string   `json:"dir,omitempty"`
	ScalaArgs        []string `json:"scala_args"`
	GoArgs           []string `json:"go_args"`
	CompareTrace     bool     `json:"compare_trace,omitempty"`
	ValidateBehavior bool     `json:"validate_behavior,omitempty"`
	Source           string   `json:"source,omitempty"`
	Init             string   `json:"init,omitempty"`
	Next             string   `json:"next,omitempty"`
	Rows             bool     `json:"rows,omitempty"`
}

type Manifest struct {
	Cases []Case `json:"cases"`
}

type DifferentialResult struct {
	Case  Case
	Scala Outcome
	Go    Outcome
}

func RunDifferential(ctx context.Context, scalaExecutable, goExecutable string, test Case) (DifferentialResult, error) {
	if test.Name == "" {
		return DifferentialResult{}, fmt.Errorf("oracle case has no name")
	}
	traceRoot, err := os.MkdirTemp("", "apalache-oracle-")
	if err != nil {
		return DifferentialResult{}, fmt.Errorf("create oracle trace directory: %w", err)
	}
	defer os.RemoveAll(traceRoot)
	scalaTraceDir := filepath.Join(traceRoot, "scala")
	if err = os.MkdirAll(scalaTraceDir, 0o700); err != nil {
		return DifferentialResult{}, fmt.Errorf("create Scala trace directory: %w", err)
	}
	scalaArgs := append([]string{"--run-dir=" + scalaTraceDir}, test.ScalaArgs...)
	scala, err := Run(ctx, Command{Executable: scalaExecutable, Args: scalaArgs, Dir: test.Dir})
	if err != nil {
		return DifferentialResult{}, fmt.Errorf("run Scala oracle for %s: %w", test.Name, err)
	}
	if (test.CompareTrace || test.ValidateBehavior) && (scala.Verdict == VerdictViolation || scala.Verdict == VerdictDeadlock) {
		scala.RawTrace, err = readScalaITFTraceData(scalaTraceDir)
		if err != nil {
			return DifferentialResult{Case: test, Scala: scala}, fmt.Errorf("read Scala trace for %s: %w", test.Name, err)
		}
		if test.CompareTrace {
			scala.Trace, err = normalizeITFTrace(scala.RawTrace)
			if err != nil {
				return DifferentialResult{Case: test, Scala: scala}, err
			}
		}
	}
	goTracePath := filepath.Join(traceRoot, "go.itf.json")
	goArgs := append([]string(nil), test.GoArgs...)
	if (test.CompareTrace || test.ValidateBehavior) && len(goArgs) > 0 && goArgs[0] == "check" {
		goArgs = append([]string{goArgs[0], "-itf=" + goTracePath}, goArgs[1:]...)
	}
	goResult, err := Run(ctx, Command{Executable: goExecutable, Args: goArgs, Dir: test.Dir})
	if err != nil {
		return DifferentialResult{}, fmt.Errorf("run Go checker for %s: %w", test.Name, err)
	}
	if (test.CompareTrace || test.ValidateBehavior) && (goResult.Verdict == VerdictViolation || goResult.Verdict == VerdictDeadlock) {
		goResult.RawTrace, err = os.ReadFile(goTracePath)
		if err != nil {
			return DifferentialResult{Case: test, Scala: scala, Go: goResult}, fmt.Errorf("read Go trace for %s: %w", test.Name, err)
		}
		if test.CompareTrace {
			goResult.Trace, err = normalizeITFTrace(goResult.RawTrace)
			if err != nil {
				return DifferentialResult{Case: test, Scala: scala, Go: goResult}, err
			}
		}
	}
	result := DifferentialResult{Case: test, Scala: scala, Go: goResult}
	if err := Compare(scala, goResult); err != nil {
		return result, fmt.Errorf("case %s: %w", test.Name, err)
	}
	return result, nil
}

func readScalaITFTrace(directory string) ([]string, error) {
	data, err := readScalaITFTraceData(directory)
	if err != nil {
		return nil, err
	}
	return normalizeITFTrace(data)
}

func readScalaITFTraceData(directory string) ([]byte, error) {
	paths, err := filepath.Glob(filepath.Join(directory, "*.itf.json"))
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no ITF trace in %s", directory)
	}
	sort.Strings(paths)
	return os.ReadFile(paths[0])
}

func readITFTrace(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return normalizeITFTrace(data)
}

func normalizeITFTrace(data []byte) ([]string, error) {
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	rawStates, ok := root["states"].([]any)
	if !ok || len(rawStates) == 0 {
		return nil, fmt.Errorf("ITF trace has no states")
	}
	states := make([]string, len(rawStates))
	for index, raw := range rawStates {
		state, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("ITF state %d is not an object", index)
		}
		delete(state, "#meta")
		// Scala's temporal encoding exposes its generated bookkeeping variables
		// in ITF. They are implementation details, not observable model state.
		for name := range state {
			if strings.HasPrefix(name, "__") {
				delete(state, name)
			}
		}
		normalized := normalizeITFValue(state)
		encoded, marshalErr := json.Marshal(normalized)
		if marshalErr != nil {
			return nil, marshalErr
		}
		states[index] = string(encoded)
	}
	return states, nil
}

func normalizeITFValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			result[key] = normalizeITFValue(child)
		}
		for _, unordered := range []string{"#set", "#map"} {
			if elements, ok := result[unordered].([]any); ok {
				sort.Slice(elements, func(left, right int) bool {
					leftJSON, _ := json.Marshal(elements[left])
					rightJSON, _ := json.Marshal(elements[right])
					return bytes.Compare(leftJSON, rightJSON) < 0
				})
			}
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, child := range typed {
			result[index] = normalizeITFValue(child)
		}
		return result
	default:
		return value
	}
}

var (
	goViolation         = regexp.MustCompile(`(?m)^(?:Invariant|Trace invariant|Action invariant)\s+(\S+)\s+violated at depth\s+(\d+)`)
	goDeadlock          = regexp.MustCompile(`(?m)^Deadlock found at depth\s+(\d+)`)
	scalaViolationDepth = regexp.MustCompile(`(?m)^State\s+(\d+):\s+(state|action|trace) invariant\b.*\bviolated\.`)
	scalaStateDepth     = regexp.MustCompile(`(?m)^State\s+(\d+):`)
	scalaStepDepth      = regexp.MustCompile(`(?m)^Step\s+(\d+):`)
	scalaResult         = regexp.MustCompile(`(?m)^The outcome is:\s*(NoError|Error|Deadlock)\b`)
	scalaSafe           = regexp.MustCompile(`Checker reports no error up to computation length\s+(\d+)`)
)

func Run(ctx context.Context, command Command) (Outcome, error) {
	if command.Executable == "" {
		return Outcome{}, fmt.Errorf("oracle executable is empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	executable := command.Executable
	if !filepath.IsAbs(executable) && strings.ContainsRune(executable, filepath.Separator) {
		absolute, err := filepath.Abs(executable)
		if err != nil {
			return Outcome{}, err
		}
		executable = absolute
	}
	process := exec.CommandContext(ctx, executable, command.Args...)
	process.Dir = command.Dir
	if command.Env != nil {
		process.Env = command.Env
	}
	var stdout, stderr bytes.Buffer
	process.Stdout, process.Stderr = &stdout, &stderr
	err := process.Run()
	exitCode := 0
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			exitCode = exitError.ExitCode()
		} else {
			return Outcome{}, err
		}
	}
	outcome := Normalize(stdout.String(), stderr.String(), exitCode)
	return outcome, nil
}

func Normalize(stdout, stderr string, exitCode int) Outcome {
	outcome := Outcome{Verdict: VerdictError, Depth: -1, ExitCode: exitCode, Stdout: stdout, Stderr: stderr}
	combined := stdout + "\n" + stderr
	if match := goViolation.FindStringSubmatch(combined); match != nil {
		outcome.Verdict = VerdictViolation
		outcome.Property = match[1]
		outcome.Depth, _ = strconv.Atoi(match[2])
		return outcome
	}
	if match := goDeadlock.FindStringSubmatch(combined); match != nil {
		outcome.Verdict = VerdictDeadlock
		outcome.Depth, _ = strconv.Atoi(match[1])
		return outcome
	}
	if strings.Contains(combined, "No counterexample found through depth") {
		outcome.Verdict = VerdictSafe
		return outcome
	}
	if match := scalaViolationDepth.FindStringSubmatch(combined); match != nil {
		outcome.Verdict = VerdictViolation
		outcome.Depth, _ = strconv.Atoi(match[1])
		// Scala reports an action invariant against its source state, whereas
		// traces and the Go checker number the transition's target state.
		if match[2] == "action" {
			outcome.Depth++
		}
		return outcome
	}
	if match := scalaResult.FindStringSubmatch(combined); match != nil {
		switch match[1] {
		case "NoError":
			outcome.Verdict = VerdictSafe
		case "Error":
			outcome.Verdict = VerdictViolation
		case "Deadlock":
			outcome.Verdict = VerdictDeadlock
			if depth, found := maximumReportedNumber(scalaStateDepth, combined); found {
				outcome.Depth = depth
			} else if step, found := maximumReportedNumber(scalaStepDepth, combined); found {
				// Step 0 selects Init. A disabled Next at Step n therefore
				// deadlocks in the state reached at trace depth n-1.
				outcome.Depth = max(0, step-1)
			}
		}
		return outcome
	}
	if scalaSafe.MatchString(combined) {
		outcome.Verdict = VerdictSafe
		return outcome
	}
	if strings.Contains(strings.ToLower(combined), "solver returned unknown") {
		outcome.Verdict = VerdictUnknown
	}
	return outcome
}

func maximumReportedNumber(pattern *regexp.Regexp, output string) (int, bool) {
	maximum, found := 0, false
	for _, match := range pattern.FindAllStringSubmatch(output, -1) {
		value, err := strconv.Atoi(match[1])
		if err == nil && (!found || value > maximum) {
			maximum, found = value, true
		}
	}
	return maximum, found
}

func Compare(left, right Outcome) error {
	if left.Verdict != right.Verdict {
		return fmt.Errorf("verdict mismatch: %s != %s", left.Verdict, right.Verdict)
	}
	if left.Property != "" && right.Property != "" && left.Property != right.Property {
		return fmt.Errorf("property mismatch: %s != %s", left.Property, right.Property)
	}
	if left.Depth >= 0 && right.Depth >= 0 && left.Depth != right.Depth {
		return fmt.Errorf("depth mismatch: %d != %d", left.Depth, right.Depth)
	}
	if left.Trace != nil || right.Trace != nil {
		if (left.Trace == nil) != (right.Trace == nil) {
			return fmt.Errorf("trace availability mismatch")
		}
		if len(left.Trace) != len(right.Trace) {
			return fmt.Errorf("trace length mismatch: %d != %d", len(left.Trace), len(right.Trace))
		}
		for index := range left.Trace {
			if left.Trace[index] != right.Trace[index] {
				return fmt.Errorf("trace state %d mismatch: %s != %s", index, left.Trace[index], right.Trace[index])
			}
		}
	}
	return nil
}
