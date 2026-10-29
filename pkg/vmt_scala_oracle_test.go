//go:build oracle

package apalache

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

var (
	vmtGeneratedDefinition = regexp.MustCompile(`\(define-fun\s+(t_[0-9]+)\b`)
	vmtGeneratedReference  = regexp.MustCompile(`\((t_[0-9]+)(?:\s|\))`)
	vmtGeneratedBinding    = regexp.MustCompile(`\((t_[0-9]+)\s+(?:[A-Z][A-Za-z0-9_]*|\()`)
)

func TestReTLAVMTScalaOracleDocumentsMissingGeneratedDefinitions(t *testing.T) {
	scalaExecutable := os.Getenv("APALACHE_SCALA")
	goExecutable := os.Getenv("APALACHE_GO")
	if scalaExecutable == "" || goExecutable == "" {
		t.Skip("set APALACHE_SCALA and APALACHE_GO to run the live ReTLA/VMT differential")
	}
	source, err := filepath.Abs("testdata/oracle/ReTLAVMT.tla")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	scalaRun := filepath.Join(t.TempDir(), "scala-run")
	runOracleCommand(t, ctx, scalaExecutable,
		"--run-dir="+scalaRun, "transpile", "--inv=Inv", source)
	scalaDocument, err := os.ReadFile(filepath.Join(scalaRun, "output.vmt"))
	if err != nil {
		t.Fatal(err)
	}

	goOutput := filepath.Join(t.TempDir(), "go.vmt")
	runOracleCommand(t, ctx, goExecutable,
		"transpile", "-inv", "Inv", "-output", goOutput, source)
	goDocument, err := os.ReadFile(goOutput)
	if err != nil {
		t.Fatal(err)
	}

	for _, fragment := range []string{
		"(declare-sort A 0)", "(declare-fun f (A) A)",
		":init true", ":action Next$0000", ":invar-property 0",
	} {
		if !strings.Contains(string(scalaDocument), fragment) || !strings.Contains(string(goDocument), fragment) {
			t.Errorf("Scala and Go VMT must both contain %q", fragment)
		}
	}
	if missing := undefinedVMTGeneratedFunctions(goDocument); len(missing) != 0 {
		t.Fatalf("Go VMT has undefined generated functions: %v", missing)
	}
	if missing := undefinedVMTGeneratedFunctions(scalaDocument); len(missing) == 0 {
		t.Fatal("Scala VMT no longer omits generated function definitions; review and remove the documented correction")
	}
}

func runOracleCommand(t *testing.T, ctx context.Context, executable string, arguments ...string) {
	t.Helper()
	command := exec.CommandContext(ctx, executable, arguments...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", executable, arguments, err, output)
	}
}

func undefinedVMTGeneratedFunctions(document []byte) []string {
	definitions := make(map[string]struct{})
	for _, match := range vmtGeneratedDefinition.FindAllSubmatch(document, -1) {
		definitions[string(match[1])] = struct{}{}
	}
	bindings := make(map[string]struct{})
	for _, match := range vmtGeneratedBinding.FindAllSubmatch(document, -1) {
		bindings[string(match[1])] = struct{}{}
	}
	missingSet := make(map[string]struct{})
	for _, match := range vmtGeneratedReference.FindAllSubmatch(document, -1) {
		name := string(match[1])
		if _, found := definitions[name]; found {
			continue
		}
		if _, found := bindings[name]; !found {
			missingSet[name] = struct{}{}
		}
	}
	missing := make([]string, 0, len(missingSet))
	for name := range missingSet {
		missing = append(missing, name)
	}
	sort.Strings(missing)
	return missing
}

func TestUndefinedVMTGeneratedFunctions(t *testing.T) {
	document := []byte(fmt.Sprintf("%s\n%s\n%s\n",
		"(define-fun t_1 ((x Int)) Int x)",
		"(define-fun Init () Bool (= (t_1 0) (t_2 0)))",
		"(define-fun Next () Bool (= (t_2 0) (t_3 0)))"))
	missing := undefinedVMTGeneratedFunctions(document)
	if fmt.Sprint(missing) != "[t_2 t_3]" {
		t.Fatalf("undefined generated functions = %v", missing)
	}
}
