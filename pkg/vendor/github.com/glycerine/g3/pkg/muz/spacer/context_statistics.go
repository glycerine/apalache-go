package spacer

// Source: src/muz/spacer/spacer_context.h context::stats. These counters
// belong to the context, surviving replacement of its predicate transformers.

type contextStatistics struct {
	queries, reachQueries, reuseReach uint32
	maxQueryLevel, maxDepth           uint32
	queryLevelSet                     bool
	expandUndef, lemmas, restarts     uint32
}
