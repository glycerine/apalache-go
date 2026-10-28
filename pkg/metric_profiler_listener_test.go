package apalache

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMetricProfilerListenerTracesAccumulatesSortsAndDumps(t *testing.T) {
	store := NewSourceStore()
	changes := NewChangeListener()
	original := NewNameExpr("x", Typed(IntT1))
	location := SourceLocation{Filename: "Profile", Region: NewSourceRegion(3, 2, 3, 7)}
	if err := store.Add(original.ID(), location); err != nil {
		t.Fatal(err)
	}
	translated := NewNameExpr("$C$5", Typed(IntT1))
	changes.OnExpressionTransformation(original, translated)
	ignored := NewNameExpr("ignored", Typed(IntT1))
	listener := NewMetricProfilerListener(t.TempDir(), store, changes)
	listener.OnRewrite(ignored, SolverContextMetrics{NCells: 100})
	listener.OnRewrite(translated, SolverContextMetrics{NCells: 1, NConstants: 2, NSMTExpressions: 9})
	listener.OnRewrite(translated, SolverContextMetrics{NCells: 2, NConstants: 99, NSMTExpressions: 16})
	entries := listener.Entries()
	want := SolverContextMetrics{NCells: 3, NConstants: 4, NSMTExpressions: 25}
	if len(entries) != 1 || entries[translated.ID()] != want {
		t.Fatalf("profile entries = %#v", entries)
	}

	second := NewNameExpr("y", Typed(IntT1))
	secondLocation := SourceLocation{Filename: "Profile", Region: NewSourceRegion(4, 1, 4, 2)}
	if err := store.Add(second.ID(), secondLocation); err != nil {
		t.Fatal(err)
	}
	// The listener snapshots the source map at construction, as Scala's
	// SourceLocator does, so a direct late source is eligible but unresolved.
	listener.OnRewrite(second, SolverContextMetrics{NCells: 20})
	if err := listener.DumpToFile(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(listener.runDir, MetricProfilerFilename))
	if err != nil {
		t.Fatal(err)
	}
	wantLines := []string{
		"# weight,nCells,nConsts,nSmtExprs,location",
		"20,20,0,0,undefinedFor" + second.ID().String(),
		"12,3,4,25," + location.String(),
	}
	gotLines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(gotLines) != len(wantLines) || gotLines[0] != wantLines[0] ||
		gotLines[1] != wantLines[1] || gotLines[2] != wantLines[2] {
		t.Fatalf("profile lines = %#v", gotLines)
	}
}

func TestMetricProfilerListenerReportsWriteErrors(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	listener := NewMetricProfilerListener(blocker, nil, nil)
	if err := listener.DumpToFile(); err == nil {
		t.Fatal("profile dump through a regular file succeeded")
	}
	listener.Dispose()
	if listener.LastError() == nil {
		t.Fatal("dispose discarded the profile write error")
	}
}

func TestMetricProfilerListenerPeriodicSyncAndDispose(t *testing.T) {
	store := NewSourceStore()
	expression := NewNameExpr("x", Typed(IntT1))
	if err := store.Add(expression.ID(), SourceLocation{Filename: "M", Region: NewSourceRegion(1, 1, 1, 1)}); err != nil {
		t.Fatal(err)
	}
	listener := NewMetricProfilerListener(t.TempDir(), store, nil)
	listener.syncInterval = 0
	listener.lastSync = time.Time{}
	listener.OnRewrite(expression, SolverContextMetrics{NCells: 1})
	path := filepath.Join(listener.runDir, MetricProfilerFilename)
	if _, err := os.Stat(path); err != nil || listener.LastError() != nil {
		t.Fatalf("periodic profile: stat=%v error=%v", err, listener.LastError())
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	listener.Dispose()
	if _, err := os.Stat(path); err != nil || listener.LastError() != nil {
		t.Fatalf("disposed profile: stat=%v error=%v", err, listener.LastError())
	}
}
