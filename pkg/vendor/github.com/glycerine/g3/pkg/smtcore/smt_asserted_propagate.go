package smtcore

// Source: src/smt/asserted_formulas.cpp propagate_values(),
// update_substitution(), is_gt(), and compute_depth().

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

func (k *SMTKernel) assertedRelation(h ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	op, basic, err := k.internalizer.basicOperation(h)
	if err != nil || !basic || op != ast.ASTOpEq && op != ast.ASTOpIff {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	left, err := k.manager.GetArg(h, 0)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	right, err := k.manager.GetArg(h, 1)
	return left, right, err == nil, err
}

func (k *SMTKernel) assertedDepth(h ast.ASTHandle, cache map[ast.ASTHandle]uint32) (uint32, error) {
	if value, found := cache[h]; found {
		return value, nil
	}
	kind, err := k.manager.GetKind(h)
	if err != nil {
		return 0, err
	}
	depth := uint32(1)
	if kind == ast.ASTApp {
		count, err := k.manager.GetNumArgs(h)
		if err != nil {
			return 0, err
		}
		for i := uint32(0); i < count; i++ {
			child, err := k.manager.GetArg(h, i)
			if err != nil {
				return 0, err
			}
			childDepth, err := k.assertedDepth(child, cache)
			if err != nil {
				return 0, err
			}
			if childDepth+1 > depth {
				depth = childDepth + 1
			}
		}
	}
	cache[h] = depth
	return depth, nil
}

func (k *SMTKernel) assertedGreater(left, right ast.ASTHandle, depth map[ast.ASTHandle]uint32) (bool, error) {
	if left == right {
		return false, nil
	}
	lv, err := k.manager.IsValue(left)
	if err != nil {
		return false, err
	}
	rv, err := k.manager.IsValue(right)
	if err != nil {
		return false, err
	}
	if lv != rv {
		return !lv, nil
	}
	if err := k.assertedSourceAssertion(545, "is_ground(lhs) && is_ground(rhs)", func() (bool, error) {
		leftGround, err := k.manager.IsGround(left)
		if err != nil || !leftGround {
			return false, err
		}
		return k.manager.IsGround(right)
	}); err != nil {
		return false, err
	}
	ld, err := k.assertedDepth(left, depth)
	if err != nil {
		return false, err
	}
	rd, err := k.assertedDepth(right, depth)
	if err != nil || ld != rd {
		return ld > rd, err
	}
	lk, err := k.manager.GetKind(left)
	if err != nil {
		return false, err
	}
	rk, err := k.manager.GetKind(right)
	if err != nil || lk != ast.ASTApp || rk != ast.ASTApp {
		return false, err
	}
	ldec, err := k.manager.GetDecl(left)
	if err != nil {
		return false, err
	}
	rdec, err := k.manager.GetDecl(right)
	if err != nil {
		return false, err
	}
	lid, err := k.manager.GetASTID(ldec)
	if err != nil {
		return false, err
	}
	rid, err := k.manager.GetASTID(rdec)
	if err != nil || lid != rid {
		return lid > rid, err
	}
	count, err := k.manager.GetNumArgs(left)
	if err != nil {
		return false, err
	}
	rcount, err := k.manager.GetNumArgs(right)
	if err != nil || count != rcount {
		return count > rcount, err
	}
	for i := uint32(0); i < count; i++ {
		la, err := k.manager.GetArg(left, i)
		if err != nil {
			return false, err
		}
		ra, err := k.manager.GetArg(right, i)
		if err != nil {
			return false, err
		}
		if la != ra {
			return k.assertedGreater(la, ra, depth)
		}
	}
	if err := k.assertedSourceAssertion(563, "UNREACHABLE", func() (bool, error) { return false, nil }); err != nil {
		return false, err
	}
	return false, nil
}

func (k *SMTKernel) addAssertionSubstitution(assertion SMTKernelAssertion, depth map[ast.ASTHandle]uint32) error {
	ground, err := k.manager.IsGround(assertion.Formula)
	if err != nil {
		return err
	}
	if ground {
		left, right, relation, err := k.assertedRelation(assertion.Formula)
		if err != nil {
			return err
		}
		if relation {
			if greater, err := k.assertedGreater(left, right, depth); err != nil {
				return err
			} else if greater {
				if err = k.traceAssertedRewrite(506, "insert ", left, right); err != nil {
					return err
				}
				return k.manager.ScopedExprSubstitutionInsert(k.assertedScopedSubst, left, right, assertion.Proof, ast.ASTDependency{})
			}
			if greater, err := k.assertedGreater(right, left, depth); err != nil {
				return err
			} else if greater {
				if err = k.traceAssertedRewrite(511, "insert ", right, left); err != nil {
					return err
				}
				proof := assertion.Proof
				if !proof.IsNull() {
					proof, err = k.manager.MkProofSymmetry(proof)
					if err != nil {
						return err
					}
				}
				return k.manager.ScopedExprSubstitutionInsert(k.assertedScopedSubst, right, left, proof, ast.ASTDependency{})
			}
			return k.traceAssertedExpression("propagate_values", "update_substitution", 515, "incompatible ", assertion.Formula)
		}
	}
	key := assertion.Formula
	value, err := k.manager.MkTrue()
	if err != nil {
		return err
	}
	proof := assertion.Proof
	if isNot, child, err := k.manager.IsNot(assertion.Formula); err != nil {
		return err
	} else if isNot {
		key = child
		value, err = k.manager.MkFalse()
		if err != nil {
			return err
		}
		if !proof.IsNull() {
			proof, err = k.manager.MkProofIffFalse(proof)
		}
	} else if !proof.IsNull() {
		proof, err = k.manager.MkProofIffTrue(proof)
	}
	if err != nil {
		return err
	}
	return k.manager.ScopedExprSubstitutionInsert(k.assertedScopedSubst, key, value, proof, ast.ASTDependency{})
}

func (k *SMTKernel) rewritePendingAssertion(index int) (bool, error) {
	old := k.assertions[index]
	formula, proof, err := k.preprocessAssertion(old.Formula, old.Proof)
	if err != nil {
		return false, err
	}
	if k.manager.IsFalse(formula) {
		k.assertedInconsistent = true
	}
	if formula == old.Formula {
		if k.preprocess {
			err = k.manager.DecRef(formula)
			if !proof.IsNull() {
				err = errors.Join(err, k.manager.DecRef(proof))
			}
		}
		return false, err
	}
	if !old.Proof.IsNull() {
		err = k.manager.DecRef(old.Proof)
	}
	err = errors.Join(err, k.manager.DecRef(old.Formula))
	k.assertions[index] = SMTKernelAssertion{Formula: formula, Proof: proof}
	return true, err
}

func (k *SMTKernel) propagatePendingAssertions() (result error) {
	if !k.preprocess || !k.preprocessState.PropagateValues || k.preprocessor == nil {
		return nil
	}
	if err := k.flushAssertedCache(); err != nil {
		return err
	}
	if k.assertedInconsistent {
		return nil
	}
	// Source ++num_iterations < 2 permits one forward/reverse pair on the
	// persistent substitution. Temporary bindings die at each direction pop.
	changed := false
	for _, reverse := range []bool{false, true} {
		if err := k.manager.ScopedExprSubstitutionPush(k.assertedScopedSubst); err != nil {
			return err
		}
		label, line := "before:\n", 451
		if reverse {
			label, line = "middle:\n", 461
		}
		if err := k.traceAssertedBody("propagate_values", "propagate_values", line, func(out io.Writer) error {
			if _, err := io.WriteString(out, label); err != nil {
				return err
			}
			return k.DisplayAssertedFormulas(out)
		}); err != nil {
			if reverse {
				return errors.Join(err, k.manager.ScopedExprSubstitutionPop(k.assertedScopedSubst, 1), k.flushAssertedCache())
			}
			return errors.Join(err, k.flushAssertedCache(), k.manager.ScopedExprSubstitutionPop(k.assertedScopedSubst, 1))
		}
		func() {
			defer func() {
				if reverse {
					result = errors.Join(result, k.manager.ScopedExprSubstitutionPop(k.assertedScopedSubst, 1), k.flushAssertedCache())
				} else {
					// Source forward cleanup flushes before popping; reverse
					// cleanup pops before its final flush.
					result = errors.Join(result, k.flushAssertedCache(), k.manager.ScopedExprSubstitutionPop(k.assertedScopedSubst, 1))
				}
			}()
			depth := make(map[ast.ASTHandle]uint32)
			for step := 0; step < len(k.assertions)-k.assertedHead && result == nil; step++ {
				i := k.assertedHead + step
				if reverse {
					i = len(k.assertions) - 1 - step
				}
				var one bool
				one, result = k.rewritePendingAssertion(i)
				changed = changed || one
				if result == nil {
					result = k.addAssertionSubstitution(k.assertions[i], depth)
				}
			}
		}()
		if result != nil {
			return result
		}
		if reverse {
			if err := k.traceAssertedBody("propagate_values", "propagate_values", 469, func(out io.Writer) error {
				if _, err := io.WriteString(out, "after:\n"); err != nil {
					return err
				}
				return k.DisplayAssertedFormulas(out)
			}); err != nil {
				return err
			}
		}
	}
	if changed {
		return k.reducePendingAssertions()
	}
	return nil
}
