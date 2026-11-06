package transforms

// Source: src/muz/transforms/dl_mk_slice.cpp, translate_proof and
// translate_hyper_res. This stage reconstructs original rules by unification.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
)

type slicedProofResult struct {
	proof ast.ASTRef
	rule  *base.Rule
}

// Convert translates asserted leaves and supported hyper-resolution nodes to
// proofs over original rules. It returns an owned proof. As in the source
// converter, resolution uses rule unification. Substitutions on asserted rules
// use Horn conversion for compound predicate heads. Derived substitutions
// require an unambiguous variable correspondence from predicate columns and,
// when needed, structurally matching interpreted constraints. Source
// nonlinear nodes use forward body-to-head pivots; reverse source nodes are
// binary. Other pivot layouts return ErrSliceProofUnsupported.
// Coalesce derivations and configured rule-normalization rewrite wrappers
// are reconstructed from their translated parents.
// Unrecognized nodes and hyper-resolution without mapped premise rules pass
// through unchanged, matching translate_proof's source fallback.
func (c *SliceProofConverter) Convert(proof ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	if c == nil || c.manager == nil || c.rules == nil {
		return result, ast.ErrASTHandle
	}
	m := c.manager
	scope, err := ast.NewScopedProof(m)
	if err != nil {
		return result, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, scope.Close())
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, m.DelRef(&result))
		}
	}()
	memo := make(map[ast.ASTHandle]slicedProofResult)
	defer func() {
		for _, v := range memo {
			if v.rule != nil {
				resultErr = errors.Join(resultErr, v.rule.Close())
			}
			if v.proof != (ast.ASTRef{}) {
				ref := v.proof
				resultErr = errors.Join(resultErr, m.DelRef(&ref))
			}
		}
	}()
	var translate func(ast.ASTHandle) (slicedProofResult, error)
	translate = func(node ast.ASTHandle) (value slicedProofResult, nodeErr error) {
		if v, ok := memo[node]; ok {
			return v, nil
		}
		defer func() {
			if nodeErr == nil {
				memo[node] = value
			} else {
				if value.rule != nil {
					if ce := value.rule.Close(); ce != nil {
						nodeErr = errors.Join(nodeErr, fmt.Errorf("failed node rule cleanup: %w", ce))
					}
				}
				if value.proof != (ast.ASTRef{}) {
					if ce := m.DelRef(&value.proof); ce != nil {
						nodeErr = errors.Join(nodeErr, fmt.Errorf("failed node proof cleanup: %w", ce))
					}
				}
			}
		}()
		var fact ast.ASTHandle
		asserted, e := m.MatchProofAsserted(node, &fact)
		if e != nil {
			return value, e
		}
		if asserted {
			if original, found := c.byFormula[fact]; found {
				value.proof, _, e = c.ConvertAsserted(node)
				if e != nil {
					return value, e
				}
				value.rule, e = original.rule.Retain()
				return value, e
			}
		}
		isHyper, e := m.IsProofKind(node, ast.ASTPrHyperResolve)
		if e != nil {
			return value, e
		}
		if !isHyper {
			valid, cause := m.IsProof(node)
			if cause != nil || !valid {
				return value, errors.Join(ast.ErrASTDomain, cause)
			}
			if source, target, matched, cause := matchCoalesceProof(m, node); cause != nil {
				return value, fmt.Errorf("slice wrapper conversion: %w", cause)
			} else if matched {
				a, cause := translate(source)
				if cause != nil {
					return value, fmt.Errorf("slice wrapper conversion: %w", cause)
				}
				b, cause := translate(target)
				if cause != nil {
					return value, fmt.Errorf("slice wrapper conversion: %w", cause)
				}
				if a.rule != nil && b.rule != nil {
					for _, v := range []slicedProofResult{a, b} {
						p, e := m.RefGet(v.proof)
						if e != nil {
							return value, e
						}
						if e = v.rule.SetProof(p); e != nil {
							return value, e
						}
					}
					plugin := NewRuleCoalescePluginWithProofTrace(m, c.rules, 50, true)
					compatible, e := plugin.sameBody(a.rule, b.rule)
					if e != nil || !compatible {
						return value, errors.Join(ErrSliceProofUnsupported, e, plugin.Close())
					}
					value.rule, cause = plugin.merge(b.rule, a.rule)
					if cause != nil {
						cause = fmt.Errorf("merging originals: %w", cause)
					}
					cause = errors.Join(cause, plugin.Close())
					if cause != nil {
						return value, fmt.Errorf("slice wrapper conversion: %w", cause)
					}
					p, found, cause := value.rule.Proof()
					if cause != nil {
						return value, fmt.Errorf("slice wrapper conversion: %w", cause)
					}
					if !found {
						return value, ast.ErrASTDomain
					}
					value.proof, cause = m.NewRef(p)
					if cause != nil {
						cause = fmt.Errorf("wrapper proof reference: %w", cause)
					}
					return value, cause
				}
			}
			if c.normalizationParams != nil {
				isMP, cause := m.IsProofKind(node, ast.ASTPrModusPonens)
				if cause != nil {
					return value, fmt.Errorf("slice wrapper conversion: %w", cause)
				}
				if isMP {
					parent, cause := m.GetProofParent(node, 0)
					if cause != nil {
						return value, fmt.Errorf("slice wrapper conversion: %w", cause)
					}
					rewrite, cause := m.GetProofParent(node, 1)
					if cause != nil {
						return value, fmt.Errorf("slice wrapper conversion: %w", cause)
					}
					var before, after ast.ASTHandle
					matched, cause := m.MatchProofRewrite(rewrite, &before, &after)
					if cause != nil {
						return value, fmt.Errorf("slice wrapper conversion: %w", cause)
					}
					oldFact, cause := m.GetProofFact(parent)
					if cause != nil {
						return value, fmt.Errorf("slice wrapper conversion: %w", cause)
					}
					newFact, cause := m.GetProofFact(node)
					if cause != nil {
						return value, fmt.Errorf("slice wrapper conversion: %w", cause)
					}
					if matched && before == oldFact && after == newFact {
						original, cause := translate(parent)
						if cause != nil {
							return value, fmt.Errorf("slice wrapper conversion: %w", cause)
						}
						if original.rule != nil {
							p, e := m.RefGet(original.proof)
							if e != nil {
								return value, e
							}
							if e = original.rule.SetProof(p); e != nil {
								return value, e
							}
							normalizer := NewRuleInlinerPluginWithServices(m, c.rules, nil, false, false, c.normalizationParams, c.normalizationGlobals, 100)
							if c.normalization != nil {
								normalizer.SetNormalizationParams(*c.normalization)
							}
							value.rule, cause = normalizer.NormalizeResolvedRule(original.rule)
							if cause != nil {
								cause = fmt.Errorf("normalizing original: %w", cause)
							}
							cause = errors.Join(cause, normalizer.Close())
							if cause != nil {
								return value, fmt.Errorf("slice wrapper conversion: %w", cause)
							}
							if value.rule == nil {
								return value, fmt.Errorf("%w: normalized original rule is inconsistent", ErrSliceProofUnsupported)
							}
							p, found, cause := value.rule.Proof()
							if cause != nil {
								return value, fmt.Errorf("slice wrapper conversion: %w", cause)
							}
							if !found {
								return value, ast.ErrASTDomain
							}
							value.proof, cause = m.NewRef(p)
							if cause != nil {
								cause = fmt.Errorf("wrapper proof reference: %w", cause)
							}
							return value, cause
						}
					}
				}
			}
			// Source translate_proof preserves unhandled terms without
			// descending into their parents, including unmapped assertions.
			// Live coalesce and configured normalization wrappers above are rebuilt
			// explicitly; unrelated proof terms keep this fallback.
			value.proof, e = m.NewRef(node)
			return value, e
		}
		conclusion, e := m.NewRef(ast.ASTHandle{})
		if e != nil {
			return value, e
		}
		defer func() { nodeErr = errors.Join(nodeErr, m.DelRef(&conclusion)) }()
		output, e := ast.NewASTProofHyperOutput(m)
		if e != nil {
			return value, e
		}
		defer func() { nodeErr = errors.Join(nodeErr, output.Close()) }()
		var premises []ast.ASTHandle
		defer func() {
			for _, premise := range premises {
				nodeErr = errors.Join(nodeErr, m.DecRef(premise))
			}
		}()
		matched, e := m.MatchProofHyperResolve(node, &premises, conclusion, output)
		if e != nil || !matched || len(premises) == 0 {
			return value, errors.Join(ast.ErrASTDomain, e)
		}
		// translate_hyper_res declines conversion when any premise has no
		// original rule, and translate_proof then keeps the whole node intact.
		for _, premise := range premises {
			original, cause := translate(premise)
			if cause != nil {
				return value, cause
			}
			if original.rule == nil {
				value.proof, e = m.NewRef(node)
				return value, e
			}
		}
		var instantiated []slicedProofResult
		defer func() {
			for _, temporary := range instantiated {
				nodeErr = errors.Join(nodeErr, temporary.rule.Close())
				ref := temporary.proof
				nodeErr = errors.Join(nodeErr, m.DelRef(&ref))
			}
		}()
		prepare := func(index int) (slicedProofResult, error) {
			original, cause := translate(premises[index])
			if cause != nil {
				return original, cause
			}
			substitution, cause := output.Substitution(uint32(index))
			if cause != nil || len(substitution) == 0 {
				return original, cause
			}
			variables, found, cause := c.OriginalVariables(premises[index])
			if cause != nil {
				return slicedProofResult{}, cause
			}
			if original.rule == nil {
				return slicedProofResult{}, fmt.Errorf("%w: substituted premise has no original rule", ErrSliceProofUnsupported)
			}
			if !found {
				variables, found, cause = c.derivedVariables(premises[index], original.rule)
				if cause != nil {
					cause = fmt.Errorf("derived variables: %w", cause)
				}
				if cause != nil {
					return slicedProofResult{}, cause
				}
			}
			if !found {
				return slicedProofResult{}, fmt.Errorf("%w: cannot map derived premise variables", ErrSliceProofUnsupported)
			}
			rule, ref, cause := c.instantiateAsserted(original.rule, original.proof, variables, substitution)
			if cause != nil {
				cause = fmt.Errorf("instantiate premise %d: %w", index, cause)
			}
			if cause != nil {
				return slicedProofResult{}, cause
			}
			value := slicedProofResult{proof: ref, rule: rule}
			instantiated = append(instantiated, value)
			return value, nil
		}
		first, e := prepare(0)
		if e != nil {
			return value, e
		}
		if first.rule == nil {
			return value, fmt.Errorf("%w: first premise has no original rule", ErrSliceProofUnsupported)
		}
		value.proof, e = m.CopyRef(first.proof)
		if e != nil {
			return value, e
		}
		value.rule, e = first.rule.Retain()
		if e != nil {
			return value, e
		}
		originalPositive, e := first.rule.PositiveTailSize()
		if e != nil {
			return value, e
		}
		positions := make([]int, originalPositive)
		for i := range positions {
			positions[i] = i
		}
		seenPositions := make(map[uint32]bool)
		for i := 1; i < len(premises); i++ {
			other, cause := prepare(i)
			if cause != nil {
				return value, cause
			}
			if other.rule == nil {
				return value, fmt.Errorf("%w: premise %d has no original rule", ErrSliceProofUnsupported, i)
			}
			position, cause := output.Position(uint32(i - 1))
			if cause != nil {
				return value, cause
			}
			if position.First == 0 && position.Second > 0 && len(premises) == 2 {
				positive, cause := other.rule.PositiveTailSize()
				if cause != nil {
					return value, cause
				}
				if position.Second > uint32(positive) {
					return value, fmt.Errorf("%w: reverse resolution position", ErrSliceProofUnsupported)
				}
				var reverseMap []int
				next, nextProof, cause := c.resolveProof(other.rule, other.proof, value.rule, value.proof, int(position.Second-1), &reverseMap)
				if cause != nil {
					return value, cause
				}
				cause = errors.Join(value.rule.Close(), m.DelRef(&value.proof))
				value.rule, value.proof = next, nextProof
				if cause != nil {
					return value, cause
				}
				continue
			}
			if position.First == 0 || position.Second != 0 {
				return value, fmt.Errorf("%w: resolution positions", ErrSliceProofUnsupported)
			}
			originalIndex := position.First - 1
			if originalIndex >= uint32(originalPositive) || seenPositions[originalIndex] {
				return value, fmt.Errorf("%w: invalid or repeated body position", ErrSliceProofUnsupported)
			}
			seenPositions[originalIndex] = true
			index := positions[originalIndex]
			if index < 0 {
				// A previous resolution removed an identical antecedent after
				// unification; resolving the duplicate again is unnecessary.
				continue
			}
			var tailMap []int
			next, nextProof, cause := c.resolveProof(value.rule, value.proof, other.rule, other.proof, index, &tailMap)
			if cause != nil {
				cause = fmt.Errorf("resolve premise %d: %w", i, cause)
			}
			if cause != nil {
				return value, cause
			}
			for j, old := range positions {
				if old >= 0 {
					positions[j] = tailMap[old]
				}
			}
			cause = errors.Join(value.rule.Close(), m.DelRef(&value.proof))
			value.rule, value.proof = next, nextProof
			if cause != nil {
				return value, cause
			}
		}
		return value, nil
	}
	converted, err := translate(proof)
	if err != nil {
		return result, err
	}
	return m.CopyRef(converted.proof)
}

func (c *SliceProofConverter) resolveProof(first *base.Rule, firstProof ast.ASTRef, second *base.Rule, secondProof ast.ASTRef, tailIndex int, tailMap *[]int) (rule *base.Rule, proof ast.ASTRef, resultErr error) {
	m := c.manager
	defer func() {
		if resultErr != nil {
			if rule != nil {
				resultErr = errors.Join(resultErr, rule.Close())
				rule = nil
			}
			if proof != (ast.ASTRef{}) {
				resultErr = errors.Join(resultErr, m.DelRef(&proof))
			}
		}
	}()
	u, err := NewRuleUnifier(m)
	if err != nil {
		return nil, proof, err
	}
	defer func() { resultErr = errors.Join(resultErr, u.Close()) }()
	ok, err := u.UnifyRules(first, tailIndex, second)
	if err != nil {
		return nil, proof, err
	}
	if !ok {
		return nil, proof, fmt.Errorf("%w: rules do not unify", ErrSliceProofUnsupported)
	}
	a, err := u.RuleSubstitution(first, true)
	if err != nil {
		return nil, proof, err
	}
	defer func() { resultErr = errors.Join(resultErr, releaseTransformRefs(m, a)) }()
	b, err := u.RuleSubstitution(second, false)
	if err != nil {
		return nil, proof, err
	}
	defer func() { resultErr = errors.Join(resultErr, releaseTransformRefs(m, b)) }()
	substitutions := make([][]ast.ASTHandle, 2)
	for index, refs := range [][]ast.ASTRef{a, b} {
		substitutions[index] = make([]ast.ASTHandle, len(refs))
		for j, ref := range refs {
			substitutions[index][j], err = m.RefGet(ref)
			if err != nil {
				return nil, proof, err
			}
		}
	}
	var renaming []ast.ASTRef
	rule, renaming, err = u.ResolveRawWithRenaming(c.rules, first, tailIndex, second)
	if err != nil {
		return nil, proof, err
	}
	defer func() { resultErr = errors.Join(resultErr, releaseTransformRefs(m, renaming)) }()
	if len(renaming) != 0 {
		mapping := make([]ast.ASTHandle, len(renaming))
		for i, ref := range renaming {
			if ref != (ast.ASTRef{}) {
				mapping[i], err = m.RefGet(ref)
				if err != nil {
					return rule, proof, err
				}
			}
		}
		sub, e := rewriter.NewVarSubstitution(m, false)
		if e != nil {
			if sub != nil {
				e = errors.Join(e, sub.Close())
			}
			return rule, proof, e
		}
		defer func() { resultErr = errors.Join(resultErr, sub.Close()) }()
		var rewritten []ast.ASTRef
		defer func() { resultErr = errors.Join(resultErr, releaseTransformRefs(m, rewritten)) }()
		for i := range substitutions {
			for j, term := range substitutions[i] {
				ref, e := m.NewRef(ast.ASTHandle{})
				if e != nil {
					return rule, proof, e
				}
				rewritten = append(rewritten, ref)
				if e = sub.Substitute(term, mapping, ref); e != nil {
					return rule, proof, e
				}
				substitutions[i][j], e = m.RefGet(ref)
				if e != nil {
					return rule, proof, e
				}
			}
		}
	}
	if err = c.mapResolvedPositiveTails(first, rule, tailIndex, substitutions[0], tailMap); err != nil {
		return rule, proof, err
	}
	formula, err := rule.ToFormula()
	if err != nil {
		return rule, proof, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&formula)) }()
	fact, err := m.RefGet(formula)
	if err != nil {
		return rule, proof, err
	}
	left, err := m.RefGet(firstProof)
	if err != nil {
		return rule, proof, err
	}
	right, err := m.RefGet(secondProof)
	if err != nil {
		return rule, proof, err
	}
	position := ast.ASTProofPosition{First: uint32(tailIndex + 1), Second: 0}
	// Reserve the owner before constructing a proof that owns both premises.
	// Reference-page growth must not strand a completed hyper-resolution node.
	proof, err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return rule, proof, err
	}
	node, err := m.MkProofHyperResolve([]ast.ASTHandle{left, right}, fact, []ast.ASTProofPosition{position}, substitutions)
	if err != nil {
		return rule, proof, err
	}
	err = m.SetRef(proof, node)
	return rule, proof, err
}

// mapResolvedPositiveTails tracks target antecedents after substitution,
// duplicate removal, and variable compression in the resolved rule.
func (c *SliceProofConverter) mapResolvedPositiveTails(first, resolved *base.Rule, selected int, substitution []ast.ASTHandle, out *[]int) (resultErr error) {
	count, err := first.PositiveTailSize()
	if err != nil {
		return err
	}
	resolvedCount, err := resolved.PositiveTailSize()
	if err != nil {
		return err
	}
	mapping := make([]int, count)
	for i := range mapping {
		mapping[i] = -1
	}
	var sub *rewriter.VarSubstitution
	if len(substitution) != 0 {
		sub, err = rewriter.NewVarSubstitution(c.manager, false)
		if sub != nil {
			defer func() { resultErr = errors.Join(resultErr, sub.Close()) }()
		}
		if err != nil {
			return err
		}
	}
	for i := 0; i < count; i++ {
		if i == selected {
			continue
		}
		term, _, err := first.Tail(i)
		if err != nil {
			return err
		}
		var transformed ast.ASTRef
		if sub != nil {
			transformed, err = c.manager.NewRef(ast.ASTHandle{})
			if err != nil {
				return err
			}
			if err = sub.Substitute(term, substitution, transformed); err == nil {
				term, err = c.manager.RefGet(transformed)
			}
		}
		if err == nil {
			for j := 0; j < resolvedCount; j++ {
				var candidate ast.ASTHandle
				candidate, _, err = resolved.Tail(j)
				if err != nil || candidate == term {
					if err == nil {
						mapping[i] = j
					}
					break
				}
			}
		}
		if transformed != (ast.ASTRef{}) {
			err = errors.Join(err, c.manager.DelRef(&transformed))
		}
		if err != nil {
			return err
		}
		if mapping[i] < 0 {
			return fmt.Errorf("%w: cannot map resolved antecedent", ErrSliceProofUnsupported)
		}
	}
	*out = mapping
	return nil
}
