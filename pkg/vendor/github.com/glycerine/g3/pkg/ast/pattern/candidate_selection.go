package pattern

// Source: src/ast/pattern/pattern_inference.cpp:396-432,559-566.

import (
	"sort"

	"github.com/glycerine/g3/pkg/ast"
)

// sortByWeight is the stable source order for multi-pattern candidates:
// descending distinct free-variable count, then ascending expression size.
// Equal-weight candidates keep their input order.
func (c *patternCandidates) sortByWeight(candidates []ast.ASTHandle) error {
	if c == nil || c.closed {
		return ast.ErrASTHandle
	}
	for _, h := range candidates {
		if _, found := c.info[h]; !found {
			return ast.ErrASTDomain
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := c.info[candidates[i]], c.info[candidates[j]]
		if len(a.freeVars) > len(b.freeVars) {
			return true
		}
		if len(a.freeVars) < len(b.freeVars) {
			return false
		}
		if a.size < b.size {
			return true
		}
		return false
	})
	return nil
}

// selectUnary follows candidates2unary_patterns. Returned ASTRef values own
// the source app_ref_buffer's pattern references; remaining candidates borrow
// the owning candidate vector. A stopped call returns its completed prefix.
func (c *patternCandidates) selectUnary(candidates []ast.ASTHandle) ([]ast.ASTRef, []ast.ASTHandle, error) {
	if c == nil || c.closed {
		return nil, nil, ast.ErrASTHandle
	}
	var patterns []ast.ASTRef
	var remaining []ast.ASTHandle
	for _, candidate := range candidates {
		info, found := c.info[candidate]
		if !found {
			return patterns, remaining, ast.ErrASTDomain
		}
		if uint32(len(info.freeVars)) != c.numBindings {
			remaining = append(remaining, candidate)
			continue
		}
		// The built-in pattern family is registered at source family ID 2.
		pattern, err := c.manager.MkFamilyApp(2, 0, nil, []ast.ASTHandle{candidate})
		if err != nil {
			return patterns, remaining, err
		}
		owned, err := c.manager.NewRef(pattern)
		if err != nil {
			return patterns, remaining, err
		}
		patterns = append(patterns, owned)
	}
	return patterns, remaining, nil
}
