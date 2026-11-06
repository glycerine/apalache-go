package core

import (
	"fmt"
	"io"
	"sort"
)

func (t *CollectStatisticsTactic) write() error {
	keys := make([]string, 0, len(t.stats))
	for key := range t.stats {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if _, err := io.WriteString(t.output, "(\n"); err != nil {
		return err
	}
	for _, key := range keys {
		if _, err := fmt.Fprintf(t.output, " :%s    %d\n", key, t.stats[key]); err != nil {
			return err
		}
	}
	_, err := io.WriteString(t.output, ")\n")
	return err
}

// Snapshot exposes the deterministic accumulated counters without weakening
// the source CollectStatistics virtual, which intentionally remains empty.
func (t *CollectStatisticsTactic) Snapshot() (map[string]uint64, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	out := make(map[string]uint64, len(t.stats))
	for k, v := range t.stats {
		out[k] = v
	}
	return out, nil
}
