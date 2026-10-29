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

var scalaTypeDiagnostic = regexp.MustCompile(`(?m)^\[([^:]+):(\d+):(\d+)-(\d+):(\d+)\]: (.*?)\s+E@`)

func TestFrontendTypeDiagnosticsMatchScalaExactly(t *testing.T) {
	scalaExecutable := os.Getenv("APALACHE_SCALA")
	if scalaExecutable == "" {
		t.Skip("set APALACHE_SCALA to run the live frontend diagnostic oracle")
	}
	for _, fixture := range []string{"Bug914.tla", "Bug925.tla"} {
		t.Run(fixture, func(t *testing.T) {
			source, err := filepath.Abs(filepath.Join("..", "test", "tla", fixture))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			command := exec.CommandContext(ctx, scalaExecutable, "typecheck", source)
			output, commandErr := command.CombinedOutput()
			if commandErr == nil {
				t.Fatalf("Scala unexpectedly accepted the negative %s fixture", fixture)
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
				want := Position{Filename: match[1]}
				coordinates := []*int{&want.Line, &want.Column, &want.EndLine, &want.EndColumn}
				for coordinate, target := range coordinates {
					value, conversionErr := strconv.Atoi(match[coordinate+2])
					if conversionErr != nil {
						t.Fatal(conversionErr)
					}
					*target = value
				}
				if result.Diagnostics[index].Position != want {
					t.Errorf("diagnostic %d range = %+v, Scala %+v", index, result.Diagnostics[index].Position, want)
				}
				if result.Diagnostics[index].Message != match[6] {
					t.Errorf("diagnostic %d message = %q, Scala %q", index, result.Diagnostics[index].Message, match[6])
				}
			}
		})
	}
}
