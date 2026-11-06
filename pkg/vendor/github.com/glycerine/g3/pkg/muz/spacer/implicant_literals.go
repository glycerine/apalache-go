package spacer

// Source: src/muz/spacer/spacer_util.cpp compute_implicant_literals and
// implicant_picker.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/memory"
)

// implicantCleanupFailure preserves every native owner whose release was
// refused. Each successful release advances its slice, so Cleanup is
// idempotent and resumes at the exact source-destruction frontier.
type implicantCleanupFailure struct {
	manager    *ast.ASTManager
	evaluator  *model.ModelEvaluator
	temporary  []ast.ASTRef
	generated  []ast.ASTHandle
	result     []ast.ASTRef
	runtime    *memory.Runtime
	cause      error
	cleanupErr error
	reported   error
	reportText string
}

func (f *implicantCleanupFailure) Error() string                 { return f.reportText }
func (f *implicantCleanupFailure) Unwrap() error                 { return f.reported }
func (f *implicantCleanupFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *implicantCleanupFailure) OriginalCause() error          { return f.cause }
func (f *implicantCleanupFailure) CleanupCause() error           { return f.cleanupErr }

func (f *implicantCleanupFailure) seal(cause, cleanup error) {
	if f.reported != nil {
		return
	}
	if cause == nil {
		cause = cleanup
	}
	f.cause, f.cleanupErr = cause, cleanup
	f.reported = errors.Join(cause, cleanup)
	f.reportText = f.reported.Error()
}

func (f *implicantCleanupFailure) cleanup() error {
	if f.evaluator != nil {
		if err := f.evaluator.Close(); err != nil {
			return err
		}
		f.evaluator = nil
	}
	if len(f.temporary) != 0 {
		if err := releaseNormalizeRefs(f.manager, f.temporary); err != nil {
			return err
		}
		f.temporary = nil
	}
	for len(f.generated) != 0 {
		i := len(f.generated) - 1
		consumed, err := f.manager.DecRefConsumed(f.generated[i])
		if consumed {
			f.generated = f.generated[:i]
		}
		if err != nil {
			return err
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	for len(f.result) != 0 {
		i := len(f.result) - 1
		err := f.manager.DelRef(&f.result[i])
		if f.result[i] == (ast.ASTRef{}) {
			f.result = f.result[:i]
		}
		if err != nil {
			return err
		}
		if len(f.result) != i {
			return ast.ErrASTPending
		}
	}
	return nil
}

func (f *implicantCleanupFailure) Cleanup() error {
	f.cleanupErr = f.cleanup()
	if f.cleanupErr != nil {
		f.seal(f.cause, f.cleanupErr)
	}
	return f.cleanupErr
}

func finishImplicantSuccess(cleanup *implicantCleanupFailure, result []ast.ASTRef) ([]ast.ASTRef, error) {
	// The result remains caller-owned unless destruction refuses. Transfer it
	// to the retry receipt before suppressing publication on that failure.
	cleanup.result = nil
	cleanup.cause = nil
	cleanup.cleanupErr = cleanup.cleanup()
	if cleanup.cleanupErr == nil {
		return result, nil
	}
	cleanup.result = result
	cleanup.seal(cleanup.cleanupErr, cleanup.cleanupErr)
	return nil, cleanup
}

// computeImplicantLiterals follows Spacer's source picker: select one
// model-true path through Boolean structure and retain model-true theory
// literals without running the separate model-cone minimizer.
func computeImplicantLiterals(manager *ast.ASTManager, md *model.ModelCore, config model.ModelEvaluatorConfig, formulas []ast.ASTHandle) (output []ast.ASTRef, resultErr error) {
	if manager == nil || md == nil || md.DerivedModel() == nil {
		return nil, ast.ErrASTHandle
	}
	// Source expr_ref locals and m_todo own generated expressions. Keep
	// native references until extraction finishes, including failure exits.
	var generated []ast.ASTHandle
	ownGenerated := func(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return h, err
		}
		if err = manager.IncRef(h); err != nil {
			return ast.ASTHandle{}, err
		}
		generated = append(generated, h)
		return h, nil
	}
	evaluator, err := newSpacerModelEvaluator(md.DerivedModel().Core(), config)
	if err != nil {
		return nil, err
	}
	cleanup := &implicantCleanupFailure{manager: manager, evaluator: evaluator, generated: generated, runtime: config.Memory}
	if err = evaluator.SetModelCompletion(false); err != nil {
		if cleanup.cleanupErr = cleanup.cleanup(); cleanup.cleanupErr != nil {
			cleanup.seal(err, cleanup.cleanupErr)
			return nil, cleanup
		}
		return nil, err
	}
	var result []ast.ASTRef
	var flattenedRefs []ast.ASTRef
	defer func() {
		cleanup.temporary = flattenedRefs
		cleanup.generated = generated
		if resultErr != nil {
			cleanup.result = result
			if cleanup.cleanupErr = cleanup.cleanup(); cleanup.cleanupErr != nil {
				cleanup.seal(resultErr, cleanup.cleanupErr)
				output, resultErr = nil, cleanup
				return
			}
			output = nil
			return
		}
		output, resultErr = finishImplicantSuccess(cleanup, result)
	}()
	evalBool := func(expression ast.ASTHandle) (bool, bool, error) {
		return evaluateSpacerModelBoolean(manager, evaluator, config, expression)
	}

	truth := func(expression ast.ASTHandle) (bool, error) {
		t, f, e := evalBool(expression)
		return t && !f, e
	}
	falsehood := func(expression ast.ASTHandle) (bool, error) {
		t, f, e := evalBool(expression)
		return f && !t, e
	}
	addLiteral := func(expression ast.ASTHandle) error {
		t, f, evalErr := evalBool(expression)
		if evalErr != nil {
			return evalErr
		}
		if t == f {
			return ast.ErrASTDomain
		}
		literal := expression
		if f {
			literal, evalErr = ownGenerated(manager.MkNot(expression))
			if evalErr != nil {
				return evalErr
			}
		}
		// Source canonicalizes a binary distinct literal before arithmetic
		// splitting; larger distinct applications are expanded by the picker.
		literalFamily, e := manager.GetFamilyID(literal)
		if e != nil {
			return e
		}
		literalKind, e := manager.GetDeclKind(literal)
		if e != nil {
			return e
		}
		if literalFamily == manager.BasicFamilyID() && literalKind == ast.ASTOpDistinct {
			n, e := manager.GetNumArgs(literal)
			if e != nil {
				return e
			}
			if n == 2 {
				a, e := manager.GetArg(literal, 0)
				if e != nil {
					return e
				}
				b, e := manager.GetArg(literal, 1)
				if e != nil {
					return e
				}
				eq, e := ownGenerated(manager.MkEq(a, b))
				if e != nil {
					return e
				}
				literal, e = ownGenerated(manager.MkNot(eq))
				if e != nil {
					return e
				}
			}
		}
		if isNot, inner, getErr := manager.IsNot(literal); getErr != nil {
			return getErr
		} else if isNot {
			innerFamily, familyErr := manager.GetFamilyID(inner)
			if familyErr != nil {
				return familyErr
			}
			kind, kindErr := manager.GetDeclKind(inner)
			if kindErr != nil {
				return kindErr
			}
			if innerFamily == manager.BasicFamilyID() {
				switch kind {
				case ast.ASTOpXor:
					a, x := manager.GetArg(inner, 0)
					b, y := manager.GetArg(inner, 1)
					if x != nil || y != nil {
						return errors.Join(x, y)
					}
					literal, evalErr = ownGenerated(manager.MkEq(a, b))
				case ast.ASTOpEq:
					a, x := manager.GetArg(inner, 0)
					b, y := manager.GetArg(inner, 1)
					if x != nil || y != nil {
						return errors.Join(x, y)
					}
					sort, x := manager.GetSort(a)
					if x != nil {
						return x
					}
					family, x := manager.GetFamilyID(sort)
					if x != nil {
						return x
					}
					arithmetic, x := manager.ArithmeticFamilyID()
					if x != nil {
						return x
					}
					// Source add_literal only splits Int/Real disequalities.
					// Every other disequality must still reach the output below.
					if family != arithmetic {
						break
					}
					left, x := ownGenerated(manager.MkArithmeticLT(a, b))
					if x != nil {
						return x
					}
					if yes, x := truth(left); x != nil {
						return x
					} else if yes {
						literal = left
					} else {
						literal, evalErr = ownGenerated(manager.MkArithmeticLT(b, a))
					}
				}
			}
			if evalErr != nil {
				return evalErr
			}
		}
		if yes, evalErr := truth(literal); evalErr != nil {
			return evalErr
		} else if !yes {
			return ast.ErrASTDomain
		}
		ref, evalErr := manager.NewRef(literal)
		if evalErr == nil {
			result = append(result, ref)
		}
		return evalErr
	}
	var pick func(ast.ASTHandle, map[ast.ASTHandle]bool) error
	pick = func(root ast.ASTHandle, visited map[ast.ASTHandle]bool) error {
		todo := []ast.ASTHandle{root}
		for len(todo) != 0 {
			i := len(todo) - 1
			a := todo[i]
			todo = todo[:i]
			if visited[a] {
				continue
			}
			visited[a] = true
			t, f, evalErr := evalBool(a)
			if evalErr != nil {
				return evalErr
			}
			if !t && !f {
				continue
			}
			family, evalErr := manager.GetFamilyID(a)
			if evalErr != nil {
				return evalErr
			}
			basic := manager.BasicFamilyID()
			if family != basic {
				if evalErr = addLiteral(a); evalErr != nil {
					return evalErr
				}
				continue
			}
			kind, evalErr := manager.GetDeclKind(a)
			if evalErr != nil {
				return evalErr
			}
			count, evalErr := manager.GetNumArgs(a)
			if evalErr != nil {
				return evalErr
			}
			args := make([]ast.ASTHandle, count)
			for j := range args {
				args[j], evalErr = manager.GetArg(a, uint32(j))
				if evalErr != nil {
					return evalErr
				}
			}
			switch kind {
			case ast.ASTOpTrue, ast.ASTOpFalse:
			case ast.ASTOpNot:
				todo = append(todo, args[0])
			case ast.ASTOpDistinct:
				if f {
					// project_plugin::pick_equality remembers the first expression
					// for each identical model value. Keep those values owned while
					// looking for the first duplicate, rather than reevaluating all
					// preceding arguments for every new term.
					values := make(map[ast.ASTHandle]ast.ASTHandle)
					var valueRefs []ast.ASTRef
					found := false
					pickErr := func() error {
						for _, argument := range args {
							ref, e := evaluator.EvaluateValue(argument)
							if e != nil {
								return e
							}
							valueRefs = append(valueRefs, ref)
							value, e := manager.RefGet(ref)
							if e != nil {
								return e
							}
							if previous, ok := values[value]; ok {
								eq, e := ownGenerated(manager.MkEq(argument, previous))
								if e != nil {
									return e
								}
								todo = append(todo, eq)
								found = true
								return nil
							}
							values[value] = argument
						}
						return nil
					}()
					if e := errors.Join(pickErr, releaseNormalizeRefs(manager, valueRefs)); e != nil {
						return e
					}
					if !found {
						return ast.ErrASTDomain
					}
				} else if len(args) == 2 {
					if evalErr = addLiteral(a); evalErr != nil {
						return evalErr
					}
				} else {
					expanded, e := ownGenerated(manager.MkDistinctExpanded(args))
					if e != nil {
						return e
					}
					todo = append(todo, expanded)
				}
			case ast.ASTOpAnd:
				if t {
					todo = append(todo, args...)
				} else {
					for _, child := range args {
						yes, e := falsehood(child)
						if e != nil {
							return e
						}
						if yes {
							todo = append(todo, child)
							break
						}
					}
				}
			case ast.ASTOpOr:
				if f {
					todo = append(todo, args...)
				} else {
					for _, child := range args {
						yes, e := truth(child)
						if e != nil {
							return e
						}
						if yes {
							todo = append(todo, child)
							break
						}
					}
				}
			case ast.ASTOpIff, ast.ASTOpEq:
				if len(args) == 2 {
					equal, e := manager.AreEqual(args[0], args[1])
					if e != nil {
						return e
					}
					distinct, e := manager.AreDistinct(args[0], args[1])
					if e != nil {
						return e
					}
					if !equal && !distinct {
						boolSort, e := manager.MkBoolSort()
						if e != nil {
							return e
						}
						sort0, e := manager.GetSort(args[0])
						if e != nil {
							return e
						}
						sort1, e := manager.GetSort(args[1])
						if e != nil {
							return e
						}
						leftConst, e := implicantUninterpretedConstant(manager, args[0])
						if e != nil {
							return e
						}
						rightConst, e := implicantUninterpretedConstant(manager, args[1])
						if e != nil {
							return e
						}
						if sort0 == boolSort && sort1 == boolSort && (!leftConst || !rightConst) {
							todo = append(todo, args...)
						} else if evalErr = addLiteral(a); evalErr != nil {
							return evalErr
						}
					}
				}
			case ast.ASTOpIte:
				if len(args) == 3 {
					equal, e := manager.AreEqual(args[1], args[2])
					if e != nil {
						return e
					}
					if equal {
						todo = append(todo, args[1])
						continue
					}
					// Preserve source &&/else-if short circuiting: later probes
					// may be expensive or interrupt the model evaluator.
					thenTrue, e := truth(args[1])
					if e != nil {
						return e
					}
					if thenTrue {
						elseTrue, e := truth(args[2])
						if e != nil {
							return e
						}
						if elseTrue {
							todo = append(todo, args[1:]...)
							continue
						}
					}
					thenFalse, e := falsehood(args[1])
					if e != nil {
						return e
					}
					if thenFalse {
						elseFalse, e := falsehood(args[2])
						if e != nil {
							return e
						}
						if elseFalse {
							todo = append(todo, args[1:]...)
							continue
						}
					}
					conditionTrue, e := truth(args[0])
					if e != nil {
						return e
					}
					if conditionTrue {
						todo = append(todo, args[0], args[1])
						continue
					}
					conditionFalse, e := falsehood(args[0])
					if e != nil {
						return e
					}
					if conditionFalse {
						todo = append(todo, args[0], args[2])
					}
				}
			case ast.ASTOpXor:
				todo = append(todo, args...)
			case ast.ASTOpImplies:
				if t {
					if yes, e := truth(args[1]); e != nil {
						return e
					} else if yes {
						todo = append(todo, args[1])
					} else if no, e := falsehood(args[0]); e != nil {
						return e
					} else if no {
						todo = append(todo, args[0])
					}
				} else {
					todo = append(todo, args...)
				}
			default:
				if evalErr = addLiteral(a); evalErr != nil {
					return evalErr
				}
			}
		}
		return nil
	}
	// Source flatten_and mutates its owning input vector, including swapping
	// appended children into the current slot and collapsing false conjuncts.
	flattened, err := manager.FlattenAnd(append([]ast.ASTHandle(nil), formulas...))
	if err != nil {
		return nil, err
	}
	flattenedRefs, err = refsFromHandles(manager, flattened)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, releaseNormalizeRefs(manager, flattenedRefs)) }()
	allTrue := true
	if len(flattened) != 0 {
		conjunction, e := ownGenerated(manager.MkAndArray(flattened))
		if e != nil {
			return nil, e
		}
		allTrue, err = truth(conjunction)
		if err != nil {
			return nil, err
		}
	}
	visited := make(map[ast.ASTHandle]bool)
	for _, formula := range flattened {
		yes := allTrue
		if !yes {
			yes, err = truth(formula)
			if err != nil {
				return nil, err
			}
		}
		if yes {
			if err = pick(formula, visited); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

func implicantUninterpretedConstant(m *ast.ASTManager, h ast.ASTHandle) (bool, error) {
	family, err := m.GetFamilyID(h)
	if err != nil || family != ast.NullFamilyID {
		return false, err
	}
	count, err := m.GetNumArgs(h)
	return count == 0, err
}
