package spacer

// Source: src/muz/spacer/spacer_context.cpp context scoped_watch owners.

import (
	"github.com/glycerine/g3/pkg/util/statistics"
	"github.com/glycerine/g3/pkg/util/stopwatch"
)

type contextTiming struct {
	initRules, solve, propagate stopwatch.Stopwatch
	reach, isReach, children    stopwatch.Stopwatch
}

func (w *contextTiming) collect(destination *statistics.Statistics) error {
	for _, entry := range []struct {
		key   statistics.Key
		watch *stopwatch.Stopwatch
	}{
		{"time.spacer.init_rules", &w.initRules},
		{"time.spacer.solve", &w.solve},
		{"time.spacer.solve.propagate", &w.propagate},
		{"time.spacer.solve.reach", &w.reach},
		{"time.spacer.solve.reach.is-reach", &w.isReach},
		{"time.spacer.solve.reach.children", &w.children},
	} {
		seconds, err := entry.watch.Seconds()
		if err != nil {
			return err
		}
		destination.UpdateDouble(entry.key, seconds)
	}
	return nil
}

func (w *contextTiming) reset() {
	w.initRules.Reset()
	w.solve.Reset()
	w.propagate.Reset()
	w.reach.Reset()
	w.isReach.Reset()
	w.children.Reset()
}
