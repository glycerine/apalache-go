package pattern

// Source: src/ast/pattern/pattern_inference.h:227-241 and
// src/ast/pattern/pattern_inference.cpp:501-518.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// registerPreferred stores a borrowed declaration identity. Duplicate source
// registrations leave one set member and do not acquire an AST reference.
func (c *patternCandidates) registerPreferred(decl ast.ASTHandle) error {
	if c == nil || c.closed {
		return ast.ErrASTHandle
	}
	kind, err := c.manager.GetKind(decl)
	if err != nil {
		return err
	}
	if kind != ast.ASTFuncDecl {
		return ast.ErrASTDomain
	}
	c.preferred[decl] = struct{}{}
	return nil
}

// selectPreferred follows has_preferred_patterns. It returns true only after
// a preferred declaration with every bound variable produces an owning
// singleton pattern. When true, mk_patterns skips its ordinary filters.
func (c *patternCandidates) selectPreferred(candidates []ast.ASTHandle) ([]ast.ASTRef, bool, error) {
	if c == nil || c.closed {
		return nil, false, ast.ErrASTHandle
	}
	if len(c.preferred) == 0 {
		return nil, false, nil
	}
	var result []ast.ASTRef
	for _, candidate := range candidates {
		decl, err := c.manager.GetDecl(candidate)
		if err != nil {
			return result, len(result) != 0, err
		}
		if _, preferred := c.preferred[decl]; !preferred {
			continue
		}
		info, found := c.info[candidate]
		if !found {
			return result, len(result) != 0, ast.ErrASTDomain
		}
		if uint32(len(info.freeVars)) != c.numBindings {
			continue
		}
		if err := c.tracePreferredPattern(candidate); err != nil {
			return result, len(result) != 0, err
		}
		pattern, err := c.manager.MkFamilyApp(2, 0, nil, []ast.ASTHandle{candidate})
		if err != nil {
			return result, len(result) != 0, err
		}
		owned, err := c.manager.NewRef(pattern)
		if err != nil {
			return result, len(result) != 0, err
		}
		result = append(result, owned)
	}
	return result, len(result) != 0, nil
}

// Source: src/ast/pattern/pattern_inference.cpp:510. The trace precedes
// singleton pattern construction for each complete preferred declaration.
func (c *patternCandidates) tracePreferredPattern(candidate ast.ASTHandle) error {
	tags := c.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("pattern_inference") {
		return nil
	}
	return tags.EmitConditional("pattern_inference", "has_preferred_patterns", "src/ast/pattern/pattern_inference.cpp", 510,
		func() bool { return true }, func(w io.Writer) error {
			if _, err := io.WriteString(w, "found preferred pattern:\n"); err != nil {
				return err
			}
			if err := c.writePatternSMT2(w, candidate); err != nil {
				return err
			}
			_, err := io.WriteString(w, "\n")
			return err
		})
}
