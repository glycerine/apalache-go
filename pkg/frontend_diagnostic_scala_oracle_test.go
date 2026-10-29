//go:build oracle

package apalache

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"
)

var scalaTypeDiagnostic = regexp.MustCompile(`(?m)^\[Bug914\.tla:(\d+):(\d+)-(\d+):(\d+)\]: (.*?)\s+E@`)

func TestFrontendTypeDiagnosticsMatchScalaExactly(t *testing.T) {
	scalaExecutable := os.Getenv("APALACHE_SCALA")
	if scalaExecutable == "" {
		t.Skip("set APALACHE_SCALA to run the live frontend diagnostic oracle")
	}
	source, err := filepath.Abs(filepath.Join("..", "test", "tla", "Bug914.tla"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, scalaExecutable, "typecheck", source)
	output, commandErr := command.CombinedOutput()
	if commandErr == nil {
		t.Fatal("Scala unexpectedly accepted the negative Bug914 fixture")
	}
	matches := scalaTypeDiagnostic.FindAllStringSubmatch(string(output), -1)
	if len(matches) != 2 {
		t.Fatalf("Scala type diagnostics = %q", output)
	}

	result := ParseAndTypeCheckFile(ctx, source, FrontendOptions{}, TypeCheckOptions{})
	if result.OK() || len(result.Diagnostics) != len(matches) {
		t.Fatalf("Go type diagnostics = %+v", result.Diagnostics)
	}
	for index, match := range matches {
		want := Position{Filename: "Bug914.tla"}
		coordinates := []*int{&want.Line, &want.Column, &want.EndLine, &want.EndColumn}
		for coordinate, target := range coordinates {
			value, conversionErr := strconv.Atoi(match[coordinate+1])
			if conversionErr != nil {
				t.Fatal(conversionErr)
			}
			*target = value
		}
		if result.Diagnostics[index].Position != want {
			t.Errorf("diagnostic %d range = %+v, Scala %+v", index, result.Diagnostics[index].Position, want)
		}
		if result.Diagnostics[index].Message != match[5] {
			t.Errorf("diagnostic %d message = %q, Scala %q", index, result.Diagnostics[index].Message, match[5])
		}
	}
}
