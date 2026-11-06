package qe

// Source: src/muz/spacer/spacer_util.cpp:1191-1279,
// index_term_finder and mbqi_project.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
)

// modelProjectionTerms follows for_each_expr's postorder and includes
// quantifier annotations. Candidate terms retain the source traversal order.
func (p *LegacyProjection) modelProjectionTerms(root, variable ast.ASTHandle) ([]ast.ASTHandle, error) {
	type frame struct {
		node  ast.ASTHandle
		ready bool
	}
	todo := []frame{{node: root}}
	seen := make(map[ast.ASTHandle]bool)
	var result []ast.ASTHandle
	arrays, err := p.manager.ArrayFamilyID()
	if err != nil {
		return nil, err
	}
	for len(todo) > 0 {
		f := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		kind, err := p.manager.GetKind(f.node)
		if err != nil {
			return nil, err
		}
		if !f.ready {
			if seen[f.node] {
				continue
			}
			seen[f.node] = true
			var children []ast.ASTHandle
			switch kind {
			case ast.ASTApp:
				todo = append(todo, frame{node: f.node, ready: true})
				n, err := p.manager.GetNumArgs(f.node)
				if err != nil {
					return nil, err
				}
				for j := uint32(0); j < n; j++ {
					child, err := p.manager.GetArg(f.node, j)
					if err != nil {
						return nil, err
					}
					children = append(children, child)
				}
			case ast.ASTQuantifier:
				body, err := p.manager.GetQuantifierBody(f.node)
				if err != nil {
					return nil, err
				}
				children = append(children, body)
				n, err := p.manager.GetNumPatterns(f.node)
				if err != nil {
					return nil, err
				}
				for j := uint32(0); j < n; j++ {
					child, err := p.manager.GetPattern(f.node, j)
					if err != nil {
						return nil, err
					}
					children = append(children, child)
				}
				n, err = p.manager.GetNumNoPatterns(f.node)
				if err != nil {
					return nil, err
				}
				for j := uint32(0); j < n; j++ {
					child, err := p.manager.GetNoPattern(f.node, j)
					if err != nil {
						return nil, err
					}
					children = append(children, child)
				}
			}
			for j := len(children); j > 0; j-- {
				todo = append(todo, frame{node: children[j-1]})
			}
			continue
		}
		family, err := p.manager.GetFamilyID(f.node)
		if err != nil {
			return nil, err
		}
		op, err := p.manager.GetDeclKind(f.node)
		if err != nil {
			return nil, err
		}
		if family == arrays && op == ast.ASTArrayOpSelect {
			index, err := p.manager.GetArg(f.node, 1)
			if err != nil {
				return nil, err
			}
			if index != variable {
				result = append(result, index)
			}
		} else if family == p.manager.BasicFamilyID() && op == ast.ASTOpEq {
			left, err := p.manager.GetArg(f.node, 0)
			if err != nil {
				return nil, err
			}
			right, err := p.manager.GetArg(f.node, 1)
			if err != nil {
				return nil, err
			}
			if left == variable {
				result = append(result, right)
			} else if right == variable {
				result = append(result, left)
			}
		}
	}
	return result, nil
}

// evaluateModelProjectionValue follows model_evaluator_util::eval: its
// caller-owned output remains valid (possibly null) after a typed exception.
// Interrupted traversals are deliberately not reset here.
func (p *LegacyProjection) evaluateModelProjectionValue(evaluator *model.ModelEvaluator, expression ast.ASTHandle) (ast.ASTRef, error) {
	// model_evaluator_util::eval applies its model_completion argument on every
	// probe. Do not inherit completion from an evaluator's preceding consumer:
	// MBQI must compare only interpretations already present in the model.
	if err := evaluator.SetModelCompletion(false); err != nil {
		return ast.ASTRef{}, err
	}
	value, err := p.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, err
	}
	if err = evaluator.Evaluate(expression, value); err != nil {
		if rewriter.IsRewriterException(err) {
			err = p.traceModelTermEvaluationFailure(err)
		}
		if err != nil {
			return ast.ASTRef{}, errors.Join(err, p.manager.DelRef(&value))
		}
	}
	return value, nil
}

func (p *LegacyProjection) projectModelTerms(md *model.ModelCore, current ast.ASTRef, variables []ast.ASTHandle) (remaining []ast.ASTHandle, resultErr error) {
	evaluator, err := newLegacyModelEvaluator(md, model.ModelEvaluatorConfig{Context: p.config.Context, Globals: p.config.Globals, Memory: p.manager.NumeralService().RuntimeOwner(), Cooperation: p.config.Cooperation, Trace: p.config.Trace})
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
	if err = evaluator.SetModelCompletion(false); err != nil {
		return nil, err
	}
	root, err := p.manager.RefGet(current)
	if err != nil {
		return nil, err
	}
	warm, err := p.evaluateModelProjectionValue(evaluator, root)
	if err != nil {
		return nil, err
	}
	if err = p.manager.DelRef(&warm); err != nil {
		return nil, err
	}
	remaining = append([]ast.ASTHandle(nil), variables...)
	for i := 0; i < len(remaining); {
		eliminated, err := p.projectModelTerm(evaluator, current, remaining[i])
		if err != nil {
			return nil, err
		}
		if eliminated {
			remaining[i] = remaining[len(remaining)-1]
			remaining = remaining[:len(remaining)-1]
		} else {
			i++
		}
	}
	return remaining, nil
}

func (p *LegacyProjection) projectModelTerm(evaluator *model.ModelEvaluator, current ast.ASTRef, variable ast.ASTHandle) (_ bool, resultErr error) {
	value, err := p.evaluateModelProjectionValue(evaluator, variable)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, p.manager.DelRef(&value)) }()
	valueHandle, err := p.manager.RefGet(value)
	if err != nil {
		return false, err
	}
	root, err := p.manager.RefGet(current)
	if err != nil {
		return false, err
	}
	if err := p.traceModelTermInput(variable, root); err != nil {
		return false, err
	}
	terms, err := p.modelProjectionTerms(root, variable)
	if err != nil {
		return false, err
	}
	if err := p.traceModelTermCandidates(terms); err != nil {
		return false, err
	}
	for _, term := range terms {
		evaluated, err := p.evaluateModelProjectionValue(evaluator, term)
		if err != nil {
			return false, err
		}
		handle, err := p.manager.RefGet(evaluated)
		if err == nil {
			err = p.traceModelTermValues(term, handle, valueHandle)
		}
		err = errors.Join(err, p.manager.DelRef(&evaluated))
		if err != nil {
			return false, err
		}
		if handle != valueHandle {
			continue
		}
		occurs, err := p.manager.OccursExpr(variable, term)
		if err != nil {
			return false, err
		}
		if occurs {
			continue
		}
		if err := p.traceModelTermProjection(variable, term, true); err != nil {
			return false, err
		}
		replace, err := rewriter.NewExprSafeReplace(p.manager)
		if err != nil {
			return false, err
		}
		err = replace.Insert(variable, term)
		if err == nil {
			err = replace.Apply(root, current)
		}
		err = errors.Join(err, replace.Close())
		return err == nil, err
	}
	return false, p.traceModelTermProjection(variable, root, false)
}
