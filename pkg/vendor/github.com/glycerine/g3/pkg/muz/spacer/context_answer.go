package spacer

// Source: src/muz/spacer/spacer_context.cpp context::get_ground_sat_answer.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func closeReachFacts(facts []*ReachFact) (result error) {
	for i := len(facts) - 1; i >= 0; i-- {
		result = errors.Join(result, facts[i].Close())
	}
	return result
}

func (c *Context) groundSatAnswer() (result ast.ASTRef, resultErr error) {
	defer func() {
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, c.manager.DelRef(&result))
		}
	}()
	queryFacts, err := c.query.ReachFacts()
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, closeReachFacts(queryFacts)) }()
	if len(queryFacts) == 0 {
		truth, makeErr := c.manager.MkTrue()
		if makeErr != nil {
			return ast.ASTRef{}, makeErr
		}
		return c.manager.NewRef(truth)
	}
	queryFact := queryFacts[len(queryFacts)-1]
	rootFacts, err := queryFact.Justifications()
	if err != nil {
		return ast.ASTRef{}, err
	}
	if len(rootFacts) != 1 {
		warningErr := writeSpacerVerbose(1, "Warning: counterexample is trivial or non-existent\n")
		closeErr := errors.Join(warningErr, closeReachFacts(rootFacts))
		truth, makeErr := c.manager.MkTrue()
		if makeErr != nil || closeErr != nil {
			return ast.ASTRef{}, errors.Join(makeErr, closeErr)
		}
		return c.manager.NewRef(truth)
	}
	queryRule, err := queryFact.Rule()
	if err != nil {
		return ast.ASTRef{}, errors.Join(err, closeReachFacts(rootFacts))
	}
	rootTransformers, predErr := c.query.RulePredecessors(queryRule)
	err = errors.Join(predErr, queryRule.Close())
	if err != nil || len(rootTransformers) != 1 {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return ast.ASTRef{}, errors.Join(err, closeReachFacts(rootFacts))
	}

	facts := rootFacts
	transformers := []*PredicateTransformer{rootTransformers[0]}
	var counterexample []ast.ASTRef
	truth, err := c.manager.MkTrue()
	if err != nil {
		return ast.ASTRef{}, errors.Join(err, closeReachFacts(facts), releaseNormalizeRefs(c.manager, counterexample))
	}
	truthRef, err := c.manager.NewRef(truth)
	if err != nil {
		return ast.ASTRef{}, errors.Join(err, closeReachFacts(facts), releaseNormalizeRefs(c.manager, counterexample))
	}
	constraints := []ast.ASTRef{truthRef}
	defer func() {
		resultErr = errors.Join(resultErr, closeReachFacts(facts), releaseNormalizeRefs(c.manager, constraints), releaseNormalizeRefs(c.manager, counterexample))
	}()
	rootHead, err := rootTransformers[0].Head()
	if err != nil {
		return ast.ASTRef{}, err
	}
	rootArity, err := c.manager.GetArity(rootHead)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if rootArity == 0 {
		root, err := c.manager.MkApp(rootHead, nil)
		if err != nil {
			return ast.ASTRef{}, err
		}
		rootRef, err := c.manager.NewRef(root)
		if err != nil {
			return ast.ASTRef{}, err
		}
		counterexample = append(counterexample, rootRef)
	}

	// C++ keeps one counterexample solver for the whole derivation and
	// scopes each local consistency check, preserving its incremental state.
	local, err := c.owner.Fresh(2)
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, local.Close()) }()
	if err = local.SetProduceModels(true); err != nil {
		return ast.ASTRef{}, err
	}

	for current := 0; current < len(facts); current++ {
		fact, transformer := facts[current], transformers[current]
		rule, ruleErr := fact.Rule()
		if ruleErr != nil {
			return ast.ASTRef{}, ruleErr
		}
		children, childErr := fact.Justifications()
		childTransformers, predErr := transformer.RulePredecessors(rule)
		if childErr != nil || predErr != nil || len(children) != len(childTransformers) {
			if childErr == nil && predErr == nil {
				childErr = ast.ErrASTDomain
			}
			return ast.ASTRef{}, errors.Join(childErr, predErr, rule.Close(), closeReachFacts(children))
		}
		facts = append(facts, children...)
		transformers = append(transformers, childTransformers...)

		localErr := local.Push()
		pushed := localErr == nil
		if localErr == nil {
			var constraint ast.ASTHandle
			constraint, localErr = c.manager.RefGet(constraints[current])
			if localErr == nil {
				localErr = local.Assert(constraint)
			}
		}

		var renamed []ast.ASTRef
		for index, child := range children {
			if localErr != nil {
				break
			}
			formula, getErr := child.Fact()
			if getErr != nil {
				localErr = getErr
				break
			}
			origin, renameErr := c.owner.FormulaNewToOld(formula, uint32(index), false)
			if renameErr != nil {
				localErr = renameErr
				break
			}
			renamed = append(renamed, origin)
			originHandle, getErr := c.manager.RefGet(origin)
			if getErr == nil {
				getErr = local.Assert(originHandle)
			}
			localErr = getErr
		}
		// Source checks the full tagged transition system together with the
		// selected tag, preserving all guards in the local consistency check.
		transition, transitionErr := transformer.Transition()
		if localErr == nil && transitionErr == nil {
			localErr = local.Assert(transition)
		} else {
			localErr = errors.Join(localErr, transitionErr)
		}
		tag, tagErr := transformer.RuleTag(rule)
		if localErr == nil && tagErr == nil {
			localErr = local.Assert(tag)
		} else {
			localErr = errors.Join(localErr, tagErr)
		}
		var localModel *model.ModelCore
		if localErr == nil {
			status, checkErr := local.CheckSat(nil)
			localErr = checkErr
			if localErr == nil && status != lbool.True {
				localErr = fmt.Errorf("spacer: counterexample reconstruction returned %s", status)
			}
			if localErr == nil {
				localModel, localErr = local.AcquireModel()
			}
		}
		if pushed {
			localErr = errors.Join(localErr, local.Pop(1))
		}
		localErr = errors.Join(localErr, releaseNormalizeRefs(c.manager, renamed), rule.Close())
		if localErr != nil {
			if localModel != nil {
				localErr = errors.Join(localErr, localModel.DecRef())
			}
			return ast.ASTRef{}, localErr
		}

		evaluator, evalErr := newSpacerModelEvaluator(localModel, c.owner.contexts[0].config.ModelEvaluator)
		if evalErr != nil {
			return ast.ASTRef{}, errors.Join(evalErr, localModel.DecRef())
		}
		if evalErr = evaluator.SetModelCompletion(true); evalErr != nil {
			return ast.ASTRef{}, errors.Join(evalErr, evaluator.Close(), localModel.DecRef())
		}
		for index, childTransformer := range childTransformers {
			signature, signatureErr := childTransformer.Signature()
			var values []ast.ASTRef
			var equalities []ast.ASTHandle
			for _, declaration := range signature {
				if signatureErr != nil {
					break
				}
				originDeclaration, convertErr := c.owner.OldToOld(declaration, 0, uint32(index))
				var origin ast.ASTHandle
				if convertErr == nil {
					origin, convertErr = c.manager.MkApp(originDeclaration, nil)
				}
				var value ast.ASTRef
				if convertErr == nil {
					value, convertErr = evaluator.EvaluateValue(origin)
				}
				if convertErr == nil {
					values = append(values, value)
					valueHandle, getErr := c.manager.RefGet(value)
					if getErr == nil {
						var equality ast.ASTHandle
						equality, getErr = c.manager.MkEq(origin, valueHandle)
						if getErr == nil {
							equalities = append(equalities, equality)
						}
					}
					convertErr = getErr
				}
				signatureErr = convertErr
			}
			var arguments []ast.ASTHandle
			for _, value := range values {
				argument, getErr := c.manager.RefGet(value)
				if getErr != nil {
					signatureErr = errors.Join(signatureErr, getErr)
					break
				}
				arguments = append(arguments, argument)
			}
			if signatureErr == nil {
				head, getErr := childTransformer.Head()
				var application ast.ASTHandle
				if getErr == nil {
					application, getErr = c.manager.MkApp(head, arguments)
				}
				if getErr == nil {
					var applicationRef ast.ASTRef
					applicationRef, getErr = c.manager.NewRef(application)
					if getErr == nil {
						counterexample = append(counterexample, applicationRef)
					}
				}
				signatureErr = getErr
			}
			if signatureErr == nil {
				ground, getErr := c.manager.MkAndArray(equalities)
				if getErr == nil {
					var next ast.ASTRef
					next, getErr = c.owner.FormulaOldToNew(ground, uint32(index), false)
					if getErr == nil {
						constraints = append(constraints, next)
					}
				}
				signatureErr = getErr
			}
			signatureErr = errors.Join(signatureErr, releaseNormalizeRefs(c.manager, values))
			if signatureErr != nil {
				evalErr = errors.Join(evalErr, signatureErr)
				break
			}
		}
		evalErr = errors.Join(evalErr, evaluator.Close(), localModel.DecRef())
		if evalErr != nil {
			return ast.ASTRef{}, evalErr
		}
	}

	if err = c.traceGroundCounterexample(counterexample); err != nil {
		return ast.ASTRef{}, err
	}

	handles := make([]ast.ASTHandle, len(counterexample))
	for index := range counterexample {
		handles[index], err = c.manager.RefGet(counterexample[index])
		if err != nil {
			return ast.ASTRef{}, err
		}
	}
	// Reserve the result slot before constructing the conjunction: a failed
	// NewRef afterward would leave its newly created child references unowned.
	result, err = c.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	answer, err := c.manager.MkAndArray(handles)
	if err != nil {
		return result, err
	}
	return result, c.manager.SetRef(result, answer)
}
