package api

// Source: src/api/api_stats.cpp:133-135; src/util/memory_manager.cpp:162-171.

import "github.com/glycerine/g3/pkg/util/params"

// EstimatedAllocSize reads the generated process's synchronized source memory
// counter. The source API does not initialize memory when queried.
func EstimatedAllocSize() uint64 {
	g := params.BorrowProcessGlobalParamsService()
	if g == nil || g.Context == nil {
		return 0
	}
	q := g.Context.Numerals()
	if q == nil || q.RuntimeOwner() == nil {
		return 0
	}
	p := q.RuntimeOwner().ProcessOwner()
	if p == nil {
		return 0
	}
	return p.AllocationSize()
}
