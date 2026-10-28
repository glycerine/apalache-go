package apalache

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	MetricProfilerMinimumWeight int64 = 0
	MetricProfilerFileSync            = time.Minute
	MetricProfilerFilename            = "profile.csv"
)

// MetricProfilerListener attributes SMT construction costs to source-backed
// expressions and periodically rewrites profile.csv in descending weight order.
type MetricProfilerListener struct {
	runDir       string
	sourceStore  *SourceStore
	changes      *ChangeListener
	locator      *SourceLocator
	metrics      map[UID]SolverContextMetrics
	lastSync     time.Time
	syncInterval time.Duration
	lastError    error
}

func NewMetricProfilerListener(runDir string, sourceStore *SourceStore, changes *ChangeListener) *MetricProfilerListener {
	if sourceStore == nil {
		sourceStore = NewSourceStore()
	}
	if changes == nil {
		changes = NewChangeListener()
	}
	return &MetricProfilerListener{
		runDir: runDir, sourceStore: sourceStore, changes: changes,
		locator: NewSourceLocator(sourceStore.SourceMap(), changes),
		metrics: make(map[UID]SolverContextMetrics), lastSync: time.Now(),
		syncInterval: MetricProfilerFileSync,
	}
}

func (listener *MetricProfilerListener) OnRewrite(expression Expr, delta SolverContextMetrics) {
	if listener == nil || expression == nil {
		return
	}
	id := expression.ID()
	if !listener.changes.IsDefinedAt(id) && !listener.sourceStore.Contains(id) {
		return
	}
	if old, found := listener.metrics[id]; found {
		listener.metrics[id] = old.Add(delta)
	} else {
		listener.metrics[id] = delta
	}
	now := time.Now()
	if now.Sub(listener.lastSync) >= listener.syncInterval {
		listener.lastSync = now
		listener.lastError = listener.DumpToFile()
	}
}

type metricProfileEntry struct {
	id      UID
	metrics SolverContextMetrics
}

func (listener *MetricProfilerListener) Entries() map[UID]SolverContextMetrics {
	if listener == nil {
		return nil
	}
	return cloneMap(listener.metrics)
}

func (listener *MetricProfilerListener) LastError() error {
	if listener == nil {
		return nil
	}
	return listener.lastError
}

func (listener *MetricProfilerListener) DumpToFile() (result error) {
	if listener == nil {
		return fmt.Errorf("cannot dump a nil metric profiler")
	}
	entries := make([]metricProfileEntry, 0, len(listener.metrics))
	for id, metrics := range listener.metrics {
		if metrics.Weight() >= MetricProfilerMinimumWeight {
			entries = append(entries, metricProfileEntry{id: id, metrics: metrics})
		}
	}
	sort.Slice(entries, func(left, right int) bool {
		leftWeight, rightWeight := entries[left].metrics.Weight(), entries[right].metrics.Weight()
		if leftWeight != rightWeight {
			return leftWeight > rightWeight
		}
		return entries[left].id.id < entries[right].id.id
	})
	if err := os.MkdirAll(listener.runDir, 0o755); err != nil {
		return fmt.Errorf("create profiler run directory: %w", err)
	}
	path := filepath.Join(listener.runDir, MetricProfilerFilename)
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create metric profile %s: %w", path, err)
	}
	defer func() {
		if closeErr := file.Close(); result == nil && closeErr != nil {
			result = closeErr
		}
	}()
	writer := bufio.NewWriter(file)
	if _, err = fmt.Fprintln(writer, "# weight,nCells,nConsts,nSmtExprs,location"); err != nil {
		return err
	}
	for _, entry := range entries {
		location := fmt.Sprintf("undefinedFor%d", entry.id.id)
		if source, found := listener.locator.SourceOfID(entry.id); found {
			location = source.String()
		}
		if _, err = fmt.Fprintf(writer, "%d,%d,%d,%d,%s\n", entry.metrics.Weight(),
			entry.metrics.NCells, entry.metrics.NConstants, entry.metrics.NSMTExpressions, location); err != nil {
			return err
		}
	}
	return writer.Flush()
}

func (listener *MetricProfilerListener) Dispose() {
	if listener != nil {
		listener.lastError = listener.DumpToFile()
	}
}

var _ SymbolicStateRewriterListener = (*MetricProfilerListener)(nil)
