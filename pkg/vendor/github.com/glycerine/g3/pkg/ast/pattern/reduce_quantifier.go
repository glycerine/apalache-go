package pattern

// Source: src/ast/pattern/pattern_inference.cpp:576-721.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type arithmeticPatternMode uint32

const (
	arithmeticPatternsNo arithmeticPatternMode = iota
	arithmeticPatternsConservative
	arithmeticPatternsFull
)

type quantifierInferenceOptions struct {
	arithmeticMode               arithmeticPatternMode
	maxExtraMulti                uint32
	noPatternWeight              int32
	arithmeticWeight             uint32
	nonNestedArithmeticWeight    uint32
	useDatabase, pullQuantifiers bool
	warnings                     bool
	warningService               *diagnostic.WarningService
}

// Without a quantifier below the body, pull_quant cannot change the formula.
// Its remaining rewriting cases still require the separate pull_quant port.
func (c *patternCandidates) bodyContainsQuantifier(body ast.ASTHandle) (bool, error) {
	seen := make(map[ast.ASTHandle]struct{})
	work := []ast.ASTHandle{body}
	for len(work) != 0 {
		last := len(work) - 1
		current := work[last]
		work = work[:last]
		if _, visited := seen[current]; visited {
			continue
		}
		seen[current] = struct{}{}
		kind, err := c.manager.GetKind(current)
		if err != nil {
			return false, err
		}
		if kind == ast.ASTQuantifier {
			return true, nil
		}
		if kind != ast.ASTApp {
			continue
		}
		n, err := c.manager.GetNumArgs(current)
		if err != nil {
			return false, err
		}
		for i := uint32(0); i < n; i++ {
			child, err := c.manager.GetArg(current, i)
			if err != nil {
				return false, err
			}
			work = append(work, child)
		}
	}
	return false, nil
}

func (c *patternCandidates) quantifierWarning(opts quantifierInferenceOptions, q ast.ASTHandle, format string, values ...any) error {
	if !opts.warnings {
		return nil
	}
	name, err := c.quantifierName(q)
	if err != nil {
		return err
	}
	warnings := opts.warningService
	if warnings == nil {
		warnings = diagnostic.BorrowProcessWarningService()
	}
	return warnings.Warningf(format, append([]any{name}, values...)...)
}

func (c *patternCandidates) quantifierName(q ast.ASTHandle) (string, error) {
	qid, _, err := c.manager.GetQuantifierIDs(q)
	if err != nil {
		return "", err
	}
	return c.manager.Symbols.String(qid)
}

func (c *patternCandidates) releasePatternRefs(patterns []ast.ASTRef) error {
	for i := range patterns {
		if err := c.manager.DelRef(&patterns[i]); err != nil {
			for _, pending := range patterns[i:] {
				if pending != (ast.ASTRef{}) {
					c.pendingPatternRefs = append(c.pendingPatternRefs, pending)
				}
			}
			return err
		}
	}
	return nil
}

func (c *patternCandidates) releasePendingPatternRefs() error {
	for len(c.pendingPatternRefs) != 0 {
		if err := c.manager.DelRef(&c.pendingPatternRefs[0]); err != nil {
			if c.pendingPatternRefs[0] == (ast.ASTRef{}) {
				c.pendingPatternRefs = c.pendingPatternRefs[1:]
			}
			return err
		}
		c.pendingPatternRefs = c.pendingPatternRefs[1:]
	}
	return nil
}

// reduceQuantifierNoDatabase retains its historical Go name while applying the
// complete source inference/retry sequence, including the optional database.
// The result reference is independently owned.
func (c *patternCandidates) reduceQuantifierNoDatabase(q, newBody ast.ASTHandle, newNoPatterns []ast.ASTHandle, opts quantifierInferenceOptions) (out ast.ASTRef, changed bool, err error) {
	return c.reduceQuantifierNoDatabaseCore(q, newBody, newNoPatterns, opts, ast.ASTRef{})
}

func (c *patternCandidates) reduceQuantifierNoDatabaseWithProof(q, newBody ast.ASTHandle, newNoPatterns []ast.ASTHandle, opts quantifierInferenceOptions, proof ast.ASTRef) (out ast.ASTRef, changed bool, err error) {
	if proof == (ast.ASTRef{}) {
		return out, false, ast.ErrASTHandle
	}
	return c.reduceQuantifierNoDatabaseCore(q, newBody, newNoPatterns, opts, proof)
}

func (c *patternCandidates) reduceQuantifierNoDatabaseCore(q, newBody ast.ASTHandle, newNoPatterns []ast.ASTHandle, opts quantifierInferenceOptions, proofOut ast.ASTRef) (out ast.ASTRef, changed bool, err error) {
	if c == nil || c.closed {
		return out, false, ast.ErrASTHandle
	}
	if len(c.pendingPatternRefs) != 0 {
		return out, false, ast.ErrASTPending
	}
	if err = c.traceReduceQuantifier(q, false); err != nil {
		return out, false, err
	}
	forall, err := c.manager.IsForall(q)
	if err != nil || !forall {
		return out, false, err
	}
	proofs, err := c.manager.ProofsEnabled()
	if err != nil {
		return out, false, err
	}
	if proofs && proofOut == (ast.ASTRef{}) {
		return out, false, ast.ErrASTPending
	}
	if !proofs && proofOut != (ast.ASTRef{}) {
		return out, false, ast.ErrASTDomain
	}
	if opts.useDatabase {
		out, matched, err := c.reduceQuantifierDatabase(q, proofOut, proofs)
		if err != nil || matched {
			return out, matched, err
		}
	}
	oldPatterns, err := c.manager.GetNumPatterns(q)
	if err != nil || oldPatterns != 0 {
		return out, false, err
	}
	originalNoPatterns, err := c.manager.GetNumNoPatterns(q)
	if err != nil {
		return out, false, err
	}
	weight, err := c.manager.GetQuantifierWeight(q)
	if err != nil {
		return out, false, err
	}
	if opts.noPatternWeight >= 0 {
		weight = opts.noPatternWeight
	}
	bindings, err := c.manager.GetNumDeclarations(q)
	if err != nil {
		return out, false, err
	}
	arith, err := c.manager.ArithmeticFamilyID()
	if err != nil {
		return out, false, err
	}
	_, priorForbidden := c.forbiddenFamilies[arith]
	if opts.arithmeticMode == arithmeticPatternsConservative {
		c.forbiddenFamilies[arith] = struct{}{}
	}
	defer func() {
		if priorForbidden {
			c.forbiddenFamilies[arith] = struct{}{}
		} else {
			delete(c.forbiddenFamilies, arith)
		}
	}()
	var patterns []ast.ASTRef
	defer func() {
		if cleanupErr := c.releasePatternRefs(patterns); err == nil {
			err = cleanupErr
		}
	}()
	patterns, err = c.inferPatternBody(newBody, bindings, newNoPatterns, opts.maxExtraMulti)
	if err != nil {
		return out, false, err
	}
	if len(patterns) == 0 && originalNoPatterns != 0 {
		patterns, err = c.inferPatternBody(newBody, bindings, nil, opts.maxExtraMulti)
		if err != nil {
			return out, false, err
		}
		if len(patterns) != 0 {
			if err = c.quantifierWarning(opts, q, "ignoring nopats annotation because Z3 couldn't find any other pattern (quantifier id: %s)"); err != nil {
				return out, false, err
			}
		}
	}
	if opts.arithmeticMode == arithmeticPatternsConservative {
		if !priorForbidden {
			delete(c.forbiddenFamilies, arith)
		}
		if len(patterns) == 0 {
			oldBlock := c.blockLoops
			c.blockLoops = false
			patterns, err = c.inferPatternBody(newBody, bindings, newNoPatterns, opts.maxExtraMulti)
			c.blockLoops = oldBlock
			if err != nil {
				return out, false, err
			}
			if len(patterns) != 0 {
				if w := int32(opts.arithmeticWeight); w > weight {
					weight = w
				}
				if err = c.quantifierWarning(opts, q, "using arith. in pattern (quantifier id: %s), the weight was increased to %d (this value can be modified using PI_ARITH_WEIGHT=<val>).", weight); err != nil {
					return out, false, err
				}
			}
		}
	}
	if opts.arithmeticMode != arithmeticPatternsNo && len(patterns) == 0 {
		oldNested, oldBlock := c.nestedArithOnly, c.blockLoops
		c.nestedArithOnly, c.blockLoops = false, false
		patterns, err = c.inferPatternBody(newBody, bindings, newNoPatterns, opts.maxExtraMulti)
		c.nestedArithOnly, c.blockLoops = oldNested, oldBlock
		if err != nil {
			return out, false, err
		}
		if len(patterns) != 0 {
			if w := int32(opts.nonNestedArithmeticWeight); w > weight {
				weight = w
			}
			if err = c.quantifierWarning(opts, q, "using non nested arith. pattern (quantifier id: %s), the weight was increased to %d (this value can be modified using PI_NON_NESTED_ARITH_WEIGHT=<val>).", weight); err != nil {
				return out, false, err
			}
		}
	}
	baseQ, outputBody := q, newBody
	pulledWithPatterns := false
	pulledProof := ast.ASTHandle{}
	if len(patterns) == 0 && opts.pullQuantifiers {
		contained, e := c.bodyContainsQuantifier(newBody)
		if e != nil {
			return out, false, e
		}
		if contained {
			preliminary, e := c.manager.UpdateQuantifierPatterns(q, nil, newBody)
			if e != nil {
				return out, false, e
			}
			preliminaryRef, e := c.manager.NewRef(preliminary)
			if e != nil {
				return out, false, e
			}
			defer func() {
				if cleanup := c.manager.DelRef(&preliminaryRef); cleanup != nil {
					if preliminaryRef != (ast.ASTRef{}) {
						c.pendingPatternRefs = append(c.pendingPatternRefs, preliminaryRef)
					}
					err = errors.Join(err, cleanup)
				}
			}()
			originalWeight, e := c.manager.GetQuantifierWeight(q)
			if e != nil {
				return out, false, e
			}
			if weight != originalWeight {
				preliminary, e = c.manager.UpdateQuantifierWeight(preliminary, weight)
				if e != nil {
					return out, false, e
				}
				if e = c.manager.SetRef(preliminaryRef, preliminary); e != nil {
					return out, false, e
				}
			}
			pulledRef, e := c.pullNoProof(preliminary)
			if e != nil {
				return out, false, e
			}
			defer func() {
				if cleanup := c.manager.DelRef(&pulledRef); cleanup != nil {
					if pulledRef != (ast.ASTRef{}) {
						c.pendingPatternRefs = append(c.pendingPatternRefs, pulledRef)
					}
					err = errors.Join(err, cleanup)
				}
			}()
			pulled, e := c.manager.RefGet(pulledRef)
			if e != nil {
				return out, false, e
			}
			if pulled != preliminary {
				if proofs {
					pulledProof, e = c.manager.MkProofRewrite(preliminary, pulled)
					if e != nil {
						return out, false, e
					}
				}
				pulledBody, e := c.manager.GetQuantifierBody(pulled)
				if e != nil {
					return out, false, e
				}
				pulledBindings, e := c.manager.GetNumDeclarations(pulled)
				if e != nil {
					return out, false, e
				}
				patterns, err = c.inferPatternBody(pulledBody, pulledBindings, nil, opts.maxExtraMulti)
				if err != nil {
					return out, false, err
				}
				if len(patterns) != 0 {
					if err = c.quantifierWarning(opts, q, "pulled nested quantifier to be able to find an useable pattern (quantifier id: %s)"); err != nil {
						return out, false, err
					}
					baseQ, outputBody = pulled, pulledBody
					pulledWithPatterns = true
					weight, err = c.manager.GetQuantifierWeight(pulled)
					if err != nil {
						return out, false, err
					}
				}
			}
		}
	}
	oldBody, err := c.manager.GetQuantifierBody(q)
	if err != nil {
		return out, false, err
	}
	if len(patterns) == 0 {
		if err = c.quantifierWarning(opts, q, "failed to find a pattern for quantifier (quantifier id: %s)"); err != nil {
			return out, false, err
		}
		if err = c.traceReduceQuantifier(q, true); err != nil {
			return out, false, err
		}
		if newBody == oldBody {
			return out, false, nil
		}
	}
	handles := make([]ast.ASTHandle, len(patterns))
	for i, ref := range patterns {
		handles[i], err = c.manager.RefGet(ref)
		if err != nil {
			return out, false, err
		}
	}
	updated, err := c.manager.UpdateQuantifierPatterns(baseQ, handles, outputBody)
	if err != nil {
		return out, false, err
	}
	if originalWeight, e := c.manager.GetQuantifierWeight(baseQ); e != nil {
		return out, false, e
	} else if weight != originalWeight {
		updated, err = c.manager.UpdateQuantifierWeight(updated, weight)
		if err != nil {
			return out, false, err
		}
	}
	out, err = c.manager.NewRef(updated)
	if err != nil {
		return out, false, err
	}
	if pulledWithPatterns {
		if err = c.tracePulledQuantifier(updated); err != nil {
			if cleanup := c.manager.DelRef(&out); cleanup != nil {
				if out != (ast.ASTRef{}) {
					c.pendingPatternRefs = append(c.pendingPatternRefs, out)
				}
				err = errors.Join(err, cleanup)
			}
			return ast.ASTRef{}, false, err
		}
	}
	if proofs {
		failProof := func(primary error) (ast.ASTRef, bool, error) {
			cleanup := c.manager.DelRef(&out)
			if cleanup != nil && out != (ast.ASTRef{}) {
				c.pendingPatternRefs = append(c.pendingPatternRefs, out)
			}
			return ast.ASTRef{}, false, errors.Join(primary, cleanup)
		}
		bodyProof, e := c.manager.MkProofReflexivity(outputBody)
		if e != nil {
			return failProof(e)
		}
		proof, e := c.manager.MkProofQuantIntro(baseQ, updated, bodyProof)
		if e != nil {
			return failProof(e)
		}
		if pulledWithPatterns {
			proof, e = c.manager.MkProofTransitivity(pulledProof, proof)
			if e != nil {
				return failProof(e)
			}
		}
		if e = c.manager.SetRef(proofOut, proof); e != nil {
			return failProof(e)
		}
	}
	return out, true, nil
}
