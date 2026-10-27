package oraclectl

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
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
	Name      string   `json:"name"`
	Dir       string   `json:"dir,omitempty"`
	ScalaArgs []string `json:"scala_args"`
	GoArgs    []string `json:"go_args"`
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
	scala, err := Run(ctx, Command{Executable: scalaExecutable, Args: test.ScalaArgs, Dir: test.Dir})
	if err != nil {
		return DifferentialResult{}, fmt.Errorf("run Scala oracle for %s: %w", test.Name, err)
	}
	goResult, err := Run(ctx, Command{Executable: goExecutable, Args: test.GoArgs, Dir: test.Dir})
	if err != nil {
		return DifferentialResult{}, fmt.Errorf("run Go checker for %s: %w", test.Name, err)
	}
	result := DifferentialResult{Case: test, Scala: scala, Go: goResult}
	if err := Compare(scala, goResult); err != nil {
		return result, fmt.Errorf("case %s: %w", test.Name, err)
	}
	return result, nil
}

var (
	goViolation = regexp.MustCompile(`(?m)^(?:Invariant|Trace invariant|Action invariant)\s+(\S+)\s+violated at depth\s+(\d+)`)
	goDeadlock  = regexp.MustCompile(`(?m)^Deadlock found at depth\s+(\d+)`)
	scalaResult = regexp.MustCompile(`(?m)^The outcome is:\s*(NoError|Error|Deadlock)\b`)
	scalaSafe   = regexp.MustCompile(`Checker reports no error up to computation length\s+(\d+)`)
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
	if match := scalaResult.FindStringSubmatch(combined); match != nil {
		switch match[1] {
		case "NoError":
			outcome.Verdict = VerdictSafe
		case "Error":
			outcome.Verdict = VerdictViolation
		case "Deadlock":
			outcome.Verdict = VerdictDeadlock
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
	return nil
}
