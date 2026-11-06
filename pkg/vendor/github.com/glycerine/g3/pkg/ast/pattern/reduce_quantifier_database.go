package pattern

// Source: src/ast/pattern/pattern_inference.cpp:591-612.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (c *patternCandidates) reduceQuantifierDatabase(q ast.ASTHandle, proofOut ast.ASTRef, proofs bool) (out ast.ASTRef, matched bool, err error) {
	if c.database == nil {
		return out, false, ast.ErrASTHandle
	}
	if err = c.database.InitializeDatabase(); err != nil {
		return out, false, err
	}
	patterns, weight, matched, err := c.database.MatchQuantifier(q)
	defer func() {
		if cleanup := c.releasePatternRefs(patterns); err == nil {
			err = cleanup
		}
	}()
	if err != nil || !matched {
		return out, matched, err
	}
	oldPatterns, err := c.manager.GetNumPatterns(q)
	if err != nil {
		return out, false, err
	}
	updated := q
	if oldPatterns == 0 {
		handles := make([]ast.ASTHandle, len(patterns))
		for i := range patterns {
			handles[i], err = c.manager.RefGet(patterns[i])
			if err != nil {
				return out, false, err
			}
		}
		body, e := c.manager.GetQuantifierBody(q)
		if e != nil {
			return out, false, e
		}
		updated, err = c.manager.UpdateQuantifierPatterns(q, handles, body)
		if err != nil {
			return out, false, err
		}
	}
	updated, err = c.manager.UpdateQuantifierWeight(updated, int32(weight))
	if err != nil {
		return out, false, err
	}
	out, err = c.manager.NewRef(updated)
	if err != nil {
		return out, false, err
	}
	if proofs {
		proof, proofErr := c.manager.MkProofRewrite(q, updated)
		if proofErr == nil {
			proofErr = c.manager.SetRef(proofOut, proof)
		}
		if proofErr != nil {
			cleanup := c.manager.DelRef(&out)
			if cleanup != nil && out != (ast.ASTRef{}) {
				c.pendingPatternRefs = append(c.pendingPatternRefs, out)
			}
			return ast.ASTRef{}, false, errors.Join(proofErr, cleanup)
		}
	}
	return out, true, nil
}
