package pattern

// Source: src/ast/pattern/pattern_inference.cpp:435-482.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

const maxPatternSplits = 32

type prePattern struct {
	exprs    []ast.ASTHandle
	freeVars map[uint32]struct{}
	index    uint32
}

func subsetFreeVars(subset, superset map[uint32]struct{}) bool {
	for v := range subset {
		if _, found := superset[v]; !found {
			return false
		}
	}
	return true
}

// selectMulti follows candidates2multi_patterns' growing pre-pattern queue.
// Candidate handles are borrowed; each appended ASTRef independently owns the
// resulting pattern. The result slice may already contain unary patterns and
// its completed prefix remains owned by the caller if a later step stops.
func (c *patternCandidates) selectMulti(candidates []ast.ASTHandle, maxPatterns uint32, result []ast.ASTRef) ([]ast.ASTRef, error) {
	if c == nil || c.closed {
		return result, ast.ErrASTHandle
	}
	if len(candidates) == 0 {
		return result, ast.ErrASTDomain
	}
	states := []*prePattern{{freeVars: make(map[uint32]struct{})}}
	c.tracePrePatternAllocated(states[0], 439)
	defer func() {
		for _, state := range states {
			if state != nil {
				c.tracePrePatternDeallocated(state)
			}
		}
	}()
	var splits uint32
	trace := c.traceTags
	if trace == nil {
		trace = diagnostic.BorrowProcessTraceTags()
	}
	for j := 0; j < len(states); j++ {
		current := states[j]
		if uint32(len(current.freeVars)) == c.numBindings {
			pattern, err := c.manager.MkFamilyApp(2, 0, nil, current.exprs)
			if err != nil {
				return result, err
			}
			owned, err := c.manager.NewRef(pattern)
			if err != nil {
				return result, err
			}
			result = append(result, owned)
			if uint64(len(result)) >= uint64(maxPatterns) {
				return result, nil
			}
		} else if uint64(current.index) < uint64(len(candidates)) {
			n := candidates[current.index]
			info, found := c.info[n]
			if !found {
				return result, ast.ErrASTDomain
			}
			if !subsetFreeVars(info.freeVars, current.freeVars) {
				next := &prePattern{
					exprs:    append(append([]ast.ASTHandle(nil), current.exprs...), n),
					freeVars: make(map[uint32]struct{}, len(current.freeVars)+len(info.freeVars)),
					index:    current.index + 1,
				}
				c.tracePrePatternAllocated(next, 455)
				for v := range current.freeVars {
					next.freeVars[v] = struct{}{}
				}
				for v := range info.freeVars {
					next.freeVars[v] = struct{}{}
				}
				states = append(states, next)
				if splits < maxPatternSplits {
					states[j] = nil
					current.index++
					states = append(states, current)
					splits++
				}
			} else {
				states[j] = nil
				current.index++
				states = append(states, current)
			}
		}
		if trace.ShouldTrace("pattern_inference") {
			if err := trace.Emit("pattern_inference", "candidates2multi_patterns", "src/ast/pattern/pattern_inference.cpp", 474,
				fmt.Sprintf("m_pre_patterns.size(): %d\nnum_splits: %d\n", len(states), splits)); err != nil {
				return result, err
			}
		}
	}
	return result, nil
}
