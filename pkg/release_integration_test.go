//go:build release

package apalache

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
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
	for _, name := range []string{"apalache", "LICENSE", "NOTICE", "README.md"} {
		if _, err := os.Stat(filepath.Join(extractRoot, releaseName, name)); err != nil {
			t.Fatalf("release entry %s: %v", name, err)
		}
	}

	binary := filepath.Join(extractRoot, releaseName, "apalache")
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
