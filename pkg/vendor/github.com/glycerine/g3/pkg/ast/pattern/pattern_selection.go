package pattern

// Source: src/ast/pattern/pattern_inference.cpp:520-574, after collect.

import "github.com/glycerine/g3/pkg/ast"

// selectPatterns applies mk_patterns' selection stages to candidates already
// collected from a body. The returned references own every completed pattern,
// including the prefix returned when a later stage stops with an error.
func (c *patternCandidates) selectPatterns(maxExtraMulti uint32) ([]ast.ASTRef, error) {
	if c == nil || c.closed {
		return nil, ast.ErrASTHandle
	}
	if len(c.candidates) == 0 {
		return nil, nil
	}
	loopFiltered, err := c.filterLooping()
	if err != nil {
		return nil, err
	}
	if err = c.tracePatternStage(542, ast.ASTHandle{}, loopFiltered); err != nil {
		return nil, err
	}
	if len(loopFiltered) == 0 {
		return nil, ast.ErrASTDomain
	}
	preferred, found, err := c.selectPreferred(loopFiltered)
	if err != nil || found {
		return preferred, err
	}
	filtered, err := c.filterBigger(loopFiltered)
	if err != nil {
		return nil, err
	}
	if err = c.tracePatternStage(551, ast.ASTHandle{}, filtered); err != nil {
		return nil, err
	}
	if len(filtered) == 0 {
		return nil, ast.ErrASTDomain
	}
	unary, remaining, err := c.selectUnary(filtered)
	if err != nil {
		return unary, err
	}
	// The source grants one extra multi-pattern when no unary pattern was
	// produced, then compares the total result size with this limit.
	multiLimit := maxExtraMulti
	if len(unary) == 0 {
		multiLimit++
	}
	if multiLimit == 0 || len(remaining) == 0 {
		return unary, nil
	}
	if err := c.sortByWeight(remaining); err != nil {
		return unary, err
	}
	if err := c.tracePatternStage(562, ast.ASTHandle{}, remaining); err != nil {
		return unary, err
	}
	return c.selectMulti(remaining, multiLimit, unary)
}
