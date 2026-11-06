package spacer

// Source: src/muz/spacer/spacer_context.cpp context::collect_statistics.

import "fmt"

func writeSpacerBrunchStatistics(snapshot StatisticsSnapshot, inductive uint32) error {
	return writeSpacerVerbose(0, fmt.Sprintf(
		"BRUNCH_STAT max_query_lvl %d\n"+
			"BRUNCH_STAT num_queries %d\n"+
			"BRUNCH_STAT num_reach_queries %d\n"+
			"BRUNCH_STAT num_reach_reuse %d\n"+
			"BRUNCH_STAT inductive_lvl %d\n"+
			"BRUNCH_STAT max_depth %d\n"+
			"BRUNCH_STAT cex_depth %d\n",
		snapshot.counters.maxQueryLevel,
		snapshot.counters.queries,
		snapshot.counters.reachQueries,
		snapshot.counters.reuseReach,
		inductive,
		snapshot.counters.maxDepth,
		snapshot.cexDepth,
	))
}
