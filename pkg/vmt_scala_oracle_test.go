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

func TestReTLAVMTScalaOracleQuantifiedMultiArgumentShape(t *testing.T) {
	scalaExecutable := os.Getenv("APALACHE_SCALA")
	goExecutable := os.Getenv("APALACHE_GO")
	if scalaExecutable == "" || goExecutable == "" {
		t.Skip("set APALACHE_SCALA and APALACHE_GO to run the live ReTLA/VMT differential")
	}
	source, err := filepath.Abs("testdata/oracle/ReTLAVMTQuantified.tla")
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

	sharedFragments := []string{
		"(declare-sort A 0)",
		"(declare-fun f (A Int) A)",
		"(declare-fun flag () Bool)",
		"(declare-fun n () Int)",
		"(declare-fun x () A)",
		"(and (= x a_A) (= n 0) (= flag true) (forall",
		"(and (= x^ (ite flag b_A x)) (= n^ (ite (= n 0) 1 0)) (= flag^ (not flag)) (forall",
		"(! (or (= n 0) (= n 1)) :invar-property 0)",
		"(! true :invar-property 2)",
		"(! (or (not flag) (= (f a_A 0) x)) :invar-property 3)",
	}
	for _, fragment := range sharedFragments {
		if !strings.Contains(string(scalaDocument), fragment) || !strings.Contains(string(goDocument), fragment) {
			t.Errorf("Scala and Go quantified VMT must both contain %q", fragment)
		}
	}
	existential := regexp.MustCompile(`\(exists \(\(a\$[0-9]+ A\)\) \(= a\$[0-9]+ x\)\)`)
	if !existential.Match(scalaDocument) || !existential.Match(goDocument) {
		t.Errorf("Scala and Go must both retain the uninterpreted-sort existential")
	}
	if missing := undefinedVMTGeneratedFunctions(goDocument); len(missing) != 0 {
		t.Fatalf("Go quantified VMT has undefined generated functions: %v", missing)
	}
}

func TestReTLAVMTScalaOracleBooleanFunctionShape(t *testing.T) {
	scalaExecutable := os.Getenv("APALACHE_SCALA")
	goExecutable := os.Getenv("APALACHE_GO")
	if scalaExecutable == "" || goExecutable == "" {
		t.Skip("set APALACHE_SCALA and APALACHE_GO to run the live ReTLA/VMT differential")
	}
	source, err := filepath.Abs("testdata/oracle/ReTLAVMTBooleanFunction.tla")
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

	sharedFragments := []string{
		"(declare-fun flag () Bool)",
		"(declare-fun n () Int)",
		"(declare-fun pred (Int) Bool)",
		"(and (= flag false) (= n 0) (forall ((t_2 Int)) (= (pred t_2) (t_1 t_2))))",
		"(and (= flag^ (not flag)) (= n^ (ite flag 1 0)) (forall ((t_5 Int)) (= (pred^ t_5) (t_4 t_5))))",
		"(! (= (pred n) flag) :invar-property 0)",
		"(! (forall ((q$1 Bool)) (or q$1 (not q$1))) :invar-property 1)",
		"(! (exists ((i$2 Int)) (pred i$2)) :invar-property 2)",
	}
	for _, fragment := range sharedFragments {
		if !strings.Contains(string(scalaDocument), fragment) || !strings.Contains(string(goDocument), fragment) {
			t.Errorf("Scala and Go Boolean-function VMT must both contain %q", fragment)
		}
	}
	if missing := undefinedVMTGeneratedFunctions(goDocument); len(missing) != 0 {
		t.Fatalf("Go Boolean-function VMT has undefined generated functions: %v", missing)
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
