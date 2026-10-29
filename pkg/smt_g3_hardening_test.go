package apalache

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestG3AssertionDebuggerIsNoninteractive(t *testing.T) {
	if os.Getenv("APALACHE_G3_ASSERTION_HELPER") == "1" {
		// The interactive g3 default consumes this test's stdin choice and
		// returns. Apalache's hardened profile terminates immediately with 110.
		_ = diagnostic.BorrowProcessDebugContext().InvokeAssertionDebugger()
		os.Exit(0)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestG3AssertionDebuggerIsNoninteractive$")
	command.Env = append(os.Environ(), "APALACHE_G3_ASSERTION_HELPER=1")
	command.Stdin = strings.NewReader("c\n")
	err := command.Run()
	if ctx.Err() != nil {
		t.Fatalf("g3 assertion debugger waited for interactive input: %v", ctx.Err())
	}
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 110 {
		t.Fatalf("g3 assertion debugger exit = %v, want noninteractive status 110", err)
	}
}

func TestG3ProcessAssertionsAreDisabled(t *testing.T) {
	enabled, err := diagnostic.BorrowProcessDebugContext().AssertionsEnabled()
	if err != nil {
		t.Fatal(err)
	}
	if enabled {
		t.Fatal("g3 process assertions remain enabled")
	}
}
