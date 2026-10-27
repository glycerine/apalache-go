//go:build release

package apalache

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"debug/buildinfo"
	"debug/elf"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestReleaseArchiveRunsWithoutSourceOrExternalRuntime(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	tempDir := t.TempDir()
	buildRoot := filepath.Join(tempDir, "build")
	distRoot := filepath.Join(tempDir, "dist")
	const version = "v0.0.0-test"
	makeRelease := func() ([]byte, error) {
		command := exec.CommandContext(ctx, "make",
			"VERSION="+version, "BUILD_ROOT="+buildRoot, "DIST_ROOT="+distRoot, "dist")
		return command.CombinedOutput()
	}
	if output, err := makeRelease(); err != nil {
		t.Fatalf("build release archive: %v\n%s", err, output)
	}

	releaseName := "apalache-go_" + version + "_" + runtime.GOOS + "-" + runtime.GOARCH
	archivePath := filepath.Join(distRoot, releaseName+".tar.gz")
	firstArchive, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if output, err := makeRelease(); err != nil {
		t.Fatalf("rebuild release archive: %v\n%s", err, output)
	}
	secondArchive, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstArchive, secondArchive) {
		t.Fatal("release archive is not reproducible across identical builds")
	}
	extractRoot := filepath.Join(tempDir, "extracted")
	if err := extractReleaseArchive(archivePath, extractRoot, releaseName); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"apalache", "DEPENDENCY_AUDIT.md", "LICENSE", "NOTICE", "README.md"} {
		if _, err := os.Stat(filepath.Join(extractRoot, releaseName, name)); err != nil {
			t.Fatalf("release entry %s: %v", name, err)
		}
	}

	binary := filepath.Join(extractRoot, releaseName, "apalache")
	build, buildErr := buildinfo.ReadFile(binary)
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	wantModules := map[string]string{
		"github.com/cespare/xxhash/v2": "v2.3.0",
		"github.com/glycerine/g3/pkg":  "v0.0.0-20261027115743-fe4e2f4f6dae",
		"github.com/glycerine/insdict": "v0.14.6",
		"github.com/glycerine/rbtree":  "v0.2.2",
		"github.com/glycerine/tlago":   "v0.1.9",
		"golang.org/x/text":            "v0.28.0",
	}
	for _, dependency := range build.Deps {
		if dependency.Replace != nil {
			t.Fatalf("release module %s has unaudited replacement %s %s",
				dependency.Path, dependency.Replace.Path, dependency.Replace.Version)
		}
		wantVersion, reviewed := wantModules[dependency.Path]
		if !reviewed {
			t.Fatalf("release contains unaudited module %s %s", dependency.Path, dependency.Version)
		}
		if dependency.Version != wantVersion {
			t.Fatalf("release module %s = %s, audited %s", dependency.Path, dependency.Version, wantVersion)
		}
		delete(wantModules, dependency.Path)
	}
	if len(wantModules) != 0 {
		t.Fatalf("audited modules absent from release: %v", wantModules)
	}
	if runtime.GOOS == "linux" {
		executable, openErr := elf.Open(binary)
		if openErr != nil {
			t.Fatal(openErr)
		}
		libraries, libraryErr := executable.ImportedLibraries()
		closeErr := executable.Close()
		if libraryErr != nil || closeErr != nil {
			t.Fatalf("inspect release native dependencies: %v; close: %v", libraryErr, closeErr)
		}
		if len(libraries) != 0 {
			t.Fatalf("release binary has native library dependencies: %v", libraries)
		}
	}
	versionCommand := exec.CommandContext(ctx, binary, "version")
	versionCommand.Env = []string{"HOME=" + tempDir, "PATH=" + filepath.Join(tempDir, "no-executables")}
	if output, err := versionCommand.CombinedOutput(); err != nil || string(output) != version+"\n" {
		t.Fatalf("release version = %q, %v", output, err)
	}

	runDir := filepath.Join(tempDir, "run")
	if err := os.Mkdir(runDir, 0o700); err != nil {
		t.Fatal(err)
	}
	specPath := filepath.Join(runDir, "ReleaseSmoke.tla")
	spec := `---- MODULE ReleaseSmoke ----
EXTENDS Integers, SequencesExt
Init == TRUE
Next == TRUE
Inv == Reverse(<<1, 2, 3>>) = <<3, 2, 1>>
====
`
	if err := os.WriteFile(specPath, []byte(spec), 0o600); err != nil {
		t.Fatal(err)
	}
	check := exec.CommandContext(ctx, binary,
		"check", "-length", "0", "-deadlock=false", "-inv", "Inv", specPath)
	check.Dir = runDir
	check.Env = []string{"HOME=" + runDir, "PATH=" + filepath.Join(runDir, "no-executables")}
	output, err := check.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "No counterexample found through depth 0") {
		t.Fatalf("release smoke check: %v\n%s", err, output)
	}
	typedJSON := filepath.Join(runDir, "ReleaseSmoke.json")
	typecheckJSON := releaseCommand(ctx, binary, runDir, "typecheck", "-output", typedJSON, specPath)
	if output, err = typecheckJSON.CombinedOutput(); err != nil {
		t.Fatalf("release JSON typecheck: %v\n%s", err, output)
	}
	checkJSON := releaseCommand(ctx, binary, runDir,
		"check", "--algo=incremental", "--smt-encoding=oopsla19", "--smt-solver=z3",
		"--timeout-smt=30", "-length", "0", "-deadlock=false", "-inv", "Inv", typedJSON)
	output, err = checkJSON.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "No counterexample found through depth 0") {
		t.Fatalf("release JSON smoke check: %v\n%s", err, output)
	}

	vmtPath := filepath.Join(runDir, "ReleaseVMT.tla")
	vmtSpec := `---- MODULE ReleaseVMT ----
\* @type: Int;
VARIABLE x
Init == x = 0
Next == x' = x
Inv == x = x
====
`
	if err := os.WriteFile(vmtPath, []byte(vmtSpec), 0o600); err != nil {
		t.Fatal(err)
	}
	vmtOutput := filepath.Join(runDir, "release.vmt")
	transpile := releaseCommand(ctx, binary, runDir,
		"transpile", "-inv", "Inv", "-output", vmtOutput, vmtPath)
	output, err = transpile.CombinedOutput()
	if err != nil || !strings.Contains(string(output), vmtOutput) {
		t.Fatalf("release VMT transpile: %v\n%s", err, output)
	}
	vmtDocument, err := os.ReadFile(vmtOutput)
	if err != nil || !bytes.Contains(vmtDocument, []byte(":action Next$0000")) {
		t.Fatalf("release VMT output: %v\n%s", err, vmtDocument)
	}

	unsafePath := filepath.Join(runDir, "Unsafe.tla")
	unsafeSpec := `---- MODULE Unsafe ----
EXTENDS Integers
\* @type: Int;
VARIABLE x
Init == x = 0
Next == x' = x + 1
Inv == x = 0
====
`
	if err := os.WriteFile(unsafePath, []byte(unsafeSpec), 0o600); err != nil {
		t.Fatal(err)
	}
	tracePath := filepath.Join(runDir, "counterexample.itf.json")
	unsafe := releaseCommand(ctx, binary, runDir,
		"check", "-length", "1", "-deadlock=false", "-inv", "Inv", "-itf", tracePath, unsafePath)
	output, err = unsafe.CombinedOutput()
	if exitCode(err) != 12 || !strings.Contains(string(output), "Invariant Inv violated at depth 1") {
		t.Fatalf("release counterexample: %v\n%s", err, output)
	}
	traceData, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatal(err)
	}
	trace, err := UnmarshalITFTrace(traceData)
	if err != nil || len(trace) != 2 {
		t.Fatalf("release ITF trace = %#v, %v", trace, err)
	}

	parsePath := filepath.Join(runDir, "BadParse.tla")
	if err := os.WriteFile(parsePath, []byte("---- MODULE BadParse ----\nValue == (1 + )\n====\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	parse := releaseCommand(ctx, binary, runDir, "parse", parsePath)
	if output, err = parse.CombinedOutput(); exitCode(err) != 4 || len(output) == 0 {
		t.Fatalf("release parse failure: %v\n%s", err, output)
	}

	typePath := filepath.Join(runDir, "BadType.tla")
	if err := os.WriteFile(typePath, []byte("---- MODULE BadType ----\nCONSTANT N\nValue == N\n====\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	typecheck := releaseCommand(ctx, binary, runDir, "typecheck", typePath)
	if output, err = typecheck.CombinedOutput(); exitCode(err) != 4 || !strings.Contains(string(output), "type annotation") {
		t.Fatalf("release type failure: %v\n%s", err, output)
	}

	if runtime.GOOS != "windows" {
		cancelPath := filepath.Join(runDir, "Cancel.tla")
		cancelSpec := `---- MODULE Cancel ----
EXTENDS Integers
\* @type: Int;
VARIABLE x
Init == x = 0
Next == x' = x + 1
Inv == TRUE
====
`
		if err := os.WriteFile(cancelPath, []byte(cancelSpec), 0o600); err != nil {
			t.Fatal(err)
		}
		cancel := releaseCommand(ctx, binary, runDir,
			"check", "-length", "100000", "-deadlock=false", "-inv", "Inv", cancelPath)
		if err := cancel.Start(); err != nil {
			t.Fatal(err)
		}
		select {
		case <-time.After(50 * time.Millisecond):
			if err := cancel.Process.Signal(os.Interrupt); err != nil {
				t.Fatal(err)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		if err := cancel.Wait(); exitCode(err) != 1 {
			t.Fatalf("release cancellation exit = %d, %v", exitCode(err), err)
		}
	}
}

func releaseCommand(ctx context.Context, binary, dir string, args ...string) *exec.Cmd {
	command := exec.CommandContext(ctx, binary, args...)
	command.Dir = dir
	command.Env = []string{"HOME=" + dir, "PATH=" + filepath.Join(dir, "no-executables")}
	return command
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitError, ok := err.(*exec.ExitError); ok {
		return exitError.ExitCode()
	}
	return -1
}

func extractReleaseArchive(archivePath, destination, releaseName string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()
	reader := tar.NewReader(gzipReader)
	prefix := releaseName + "/"
	for {
		header, nextErr := reader.Next()
		if nextErr == io.EOF {
			return nil
		}
		if nextErr != nil {
			return nextErr
		}
		clean := filepath.Clean(header.Name)
		if clean != releaseName && !strings.HasPrefix(clean, prefix) {
			return &os.PathError{Op: "extract unsafe archive entry", Path: header.Name, Err: os.ErrInvalid}
		}
		target := filepath.Join(destination, clean)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			output, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(output, reader)
			closeErr := output.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		default:
			return &os.PathError{Op: "extract unsupported archive entry", Path: header.Name, Err: os.ErrInvalid}
		}
	}
}
