package spacer

// Source: spacer_generalizers.cpp lemma_bool_inductive_generalizer::stats and
// spacer_context.cpp init_lemma_generalizers/reset_statistics.

import (
	"github.com/glycerine/g3/pkg/util/statistics"
	"time"
)

type boolGeneralizerStatistics struct {
	count, failures uint64
	elapsed         time.Duration
}

func (s *boolGeneralizerStatistics) collect(destination *statistics.Statistics) {
	destination.UpdateUint64("bool inductive gen", s.count)
	destination.UpdateUint64("bool inductive gen failures", s.failures)
	destination.UpdateDouble("time.spacer.solve.reach.gen.bool_ind", s.elapsed.Seconds())
}
