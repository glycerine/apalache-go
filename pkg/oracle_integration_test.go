//go:build oracle

package apalache

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glycerine/apalache-go/internal/oraclectl"
)

func TestScalaDifferentialSafetyManifest(t *testing.T) {
	scalaExecutable := os.Getenv("APALACHE_SCALA")
	goExecutable := os.Getenv("APALACHE_GO")
	if scalaExecutable == "" || goExecutable == "" {
		t.Skip("set APALACHE_SCALA and APALACHE_GO to run live differential tests")
	}
	data, err := os.ReadFile("testdata/oracle/safety.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest oraclectl.Manifest
	if err = json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	for _, test := range manifest.Cases {
		t.Run(test.Name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			result, err := oraclectl.RunDifferential(ctx, scalaExecutable, goExecutable, test)
			if err != nil {
				t.Fatalf("%v\nScala stdout:\n%s\nScala stderr:\n%s\nGo stdout:\n%s\nGo stderr:\n%s",
					err, result.Scala.Stdout, result.Scala.Stderr, result.Go.Stdout, result.Go.Stderr)
			}
			if test.ValidateBehavior {
				validateOracleBehaviors(t, ctx, test, result)
			}
		})
	}
}

func validateOracleBehaviors(
	t *testing.T, ctx context.Context, test oraclectl.Case, result oraclectl.DifferentialResult,
) {
	t.Helper()
	if test.Source == "" {
		t.Fatal("behavior validation requires an oracle source")
	}
	source := test.Source
	if test.Dir != "" && !filepath.IsAbs(source) {
		source = filepath.Join(test.Dir, source)
	}
	frontend := ParseAndTypeCheckFile(ctx, source, FrontendOptions{}, TypeCheckOptions{UseRows: test.Rows})
	if !frontend.OK() {
		t.Fatalf("behavior-validation frontend diagnostics: %v", frontend.Diagnostics)
	}
	init, next := test.Init, test.Next
	if init == "" {
		init = "Init"
	}
	if next == "" {
		next = "Next"
	}
	prepared, err := PrepareSafetyTransitions(frontend.Native, SafetyPipelineOptions{
		Specification: SpecificationConfiguration{Behavior: InitNextSpec{Init: init, Next: next}},
		Sources:       frontend.Sources,
	})
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"Scala": result.Scala.RawTrace, "Go": result.Go.RawTrace} {
		states, decodeErr := UnmarshalITFTrace(data)
		if decodeErr != nil {
			t.Fatalf("decode %s trace: %v", name, decodeErr)
		}
		valid, validateErr := ValidateTraceBehavior(ctx, prepared.Module, states)
		if validateErr != nil || !valid {
			t.Fatalf("%s trace is not a valid behavior: %v, %v", name, valid, validateErr)
		}
	}
}
