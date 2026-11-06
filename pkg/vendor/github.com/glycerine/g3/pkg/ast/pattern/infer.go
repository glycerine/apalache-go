package pattern

// Source: src/ast/pattern/pattern_inference.cpp:520-574.

import "github.com/glycerine/g3/pkg/ast"

// inferPatternBody runs mk_patterns for one body. Configuration such as
// preferred declarations remains on the owner; quantifier bindings and
// excluded patterns are per-call inputs. Returned patterns own their AST
// references independently of the candidate reset.
func (c *patternCandidates) inferPatternBody(body ast.ASTHandle, numBindings uint32, noPatterns []ast.ASTHandle, maxExtraMulti uint32) (patterns []ast.ASTRef, err error) {
	if c == nil || c.closed {
		return nil, ast.ErrASTHandle
	}
	if len(c.candidates) != 0 || len(c.collectRefs) != 0 {
		return nil, ast.ErrASTPending
	}
	c.numBindings = numBindings
	c.noPatterns = noPatterns
	defer func() {
		if cleanupErr := c.resetCandidates(); err == nil {
			err = cleanupErr
		}
	}()
	if err = c.collect(body); err != nil {
		return nil, err
	}
	if err = c.tracePatternStage(531, body, c.candidates); err != nil {
		return nil, err
	}
	return c.selectPatterns(maxExtraMulti)
}
