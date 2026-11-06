package fp

// Source: src/tactic/horn_subsume_model_converter.cpp Horn-clause
// normalization and reverse model reconstruction.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/util/cooperate"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (e *PropositionalEngine) normalizeInlineModelRule(rule transforms.InlineModelRule) (declaration ast.ASTHandle, result ast.ASTRef, resultErr error) {
	head, err := e.manager.RefGet(rule.Head)
	if err != nil {
		return declaration, result, err
	}
	body, err := e.manager.RefGet(rule.Body)
	if err != nil {
		return declaration, result, err
	}
	declaration, err = e.manager.GetDecl(head)
	if err != nil {
		return declaration, result, err
	}
	if family, getErr := e.manager.GetFamilyID(declaration); getErr != nil || family != ast.NullFamilyID {
		if getErr == nil {
			getErr = ast.ErrASTDomain
		}
		return ast.ASTHandle{}, result, getErr
	}
	free, err := astrewriter.NewExprFreeVars(e.manager)
	if err != nil {
		return ast.ASTHandle{}, result, err
	}
	if err = free.Collect(head); err == nil {
		err = free.Accumulate(body)
	}
	if err != nil {
		return ast.ASTHandle{}, result, err
	}
	boolean, err := e.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, result, err
	}
	if err = free.SetDefaultSort(boolean); err != nil {
		return ast.ASTHandle{}, result, err
	}
	freeCount := free.Size()
	arity, err := e.manager.GetNumArgs(head)
	if err != nil {
		return ast.ASTHandle{}, result, err
	}
	if arity == 0 && freeCount == 0 {
		result, err = e.manager.NewRef(body)
		return declaration, result, err
	}

	// Source mk_horn owns the formal variables, substitution bindings,
	// conjunction and result throughout normalization. Keep those concrete
	// scopes alive even when constructing the substitution engine fails.
	attempt := &contextFormulaAttempt{manager: e.manager}
	defer func() {
		if resultErr != nil && result != (ast.ASTRef{}) {
			attempt.references = append(attempt.references, result)
			result = ast.ASTRef{}
		}
		resultErr = attempt.finish(resultErr)
		if resultErr != nil && result != (ast.ASTRef{}) {
			attempt.references = append(attempt.references, result)
			result = ast.ASTRef{}
		}
	}()
	reserve := func() (ast.ASTRef, error) {
		ref, err := e.manager.NewRef(ast.ASTHandle{})
		if ref != (ast.ASTRef{}) {
			attempt.references = append(attempt.references, ref)
		}
		return ref, err
	}
	conjuncts := []ast.ASTHandle{body}
	bindings := make([]ast.ASTHandle, freeCount)
	for index := uint32(0); index < arity; index++ {
		argument, getErr := e.manager.GetArg(head, index)
		if getErr != nil {
			return ast.ASTHandle{}, result, getErr
		}
		sort, getErr := e.manager.GetSort(argument)
		if getErr != nil {
			return ast.ASTHandle{}, result, getErr
		}
		owned, getErr := reserve()
		if getErr != nil {
			return ast.ASTHandle{}, result, getErr
		}
		formal, getErr := e.manager.MkVar(freeCount+index, sort)
		if !formal.IsNull() {
			getErr = errors.Join(getErr, e.manager.SetRef(owned, formal))
		}
		if getErr != nil {
			return ast.ASTHandle{}, result, getErr
		}
		kind, getErr := e.manager.GetKind(argument)
		if getErr != nil {
			return ast.ASTHandle{}, result, getErr
		}
		if kind == ast.ASTVar {
			variable, getErr := e.manager.GetVarIndex(argument)
			if getErr != nil {
				return ast.ASTHandle{}, result, getErr
			}
			if uint64(variable) >= uint64(len(bindings)) {
				return ast.ASTHandle{}, result, ast.ErrASTDomain
			}
			if bindings[variable].IsNull() {
				bindings[variable] = formal
				continue
			}
			argument = bindings[variable]
		}
		owned, getErr = reserve()
		if getErr != nil {
			return ast.ASTHandle{}, result, getErr
		}
		equality, getErr := e.manager.MkEq(formal, argument)
		if !equality.IsNull() {
			getErr = errors.Join(getErr, e.manager.SetRef(owned, equality))
		}
		if getErr != nil {
			return ast.ASTHandle{}, result, getErr
		}
		conjuncts = append(conjuncts, equality)
	}
	owned, err := reserve()
	if err != nil {
		return ast.ASTHandle{}, result, err
	}
	formula, err := e.manager.MkAndArray(conjuncts)
	if !formula.IsNull() {
		err = errors.Join(err, e.manager.SetRef(owned, formula))
	}
	if err != nil {
		return ast.ASTHandle{}, result, err
	}
	if len(bindings) != 0 {
		attempt.substitution, err = astrewriter.NewVarSubstitution(e.manager, false)
		if err != nil {
			return ast.ASTHandle{}, result, err
		}
		rewritten, makeErr := reserve()
		if makeErr != nil {
			return ast.ASTHandle{}, result, makeErr
		}
		if makeErr = attempt.substitution.Substitute(formula, bindings, rewritten); makeErr != nil {
			return ast.ASTHandle{}, result, makeErr
		}
		formula, makeErr = e.manager.RefGet(rewritten)
		if makeErr != nil {
			return ast.ASTHandle{}, result, makeErr
		}
	}
	result, err = e.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, result, err
	}
	if freeCount != 0 {
		if err = free.Reverse(); err != nil {
			return ast.ASTHandle{}, result, err
		}
		sorts := free.Sorts()
		names := make([]usymbol.Symbol, freeCount)
		for i := range names {
			names[len(names)-i-1] = usymbol.Numerical(uint32(i))
		}
		formula, err = e.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: false, Sorts: sorts, Names: names, Body: formula})
	}
	if !formula.IsNull() {
		err = errors.Join(err, e.manager.SetRef(result, formula))
	}
	return declaration, result, err
}

func (e *PropositionalEngine) addInlineDefaultInterpretations(destination *model.ModelCore, expression ast.ASTHandle) error {
	boolean, err := e.manager.MkBoolSort()
	if err != nil {
		return err
	}
	falsity, err := e.manager.MkFalse()
	if err != nil {
		return err
	}
	visited := make(map[ast.ASTHandle]bool)
	stack := []ast.ASTHandle{expression}
	for len(stack) != 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if visited[current] {
			continue
		}
		visited[current] = true
		kind, getErr := e.manager.GetKind(current)
		if getErr != nil {
			return getErr
		}
		switch kind {
		case ast.ASTVar:
			continue
		case ast.ASTQuantifier:
			body, getErr := e.manager.GetQuantifierBody(current)
			if getErr != nil {
				return getErr
			}
			stack = append(stack, body)
			continue
		case ast.ASTApp:
		default:
			return ast.ErrASTDomain
		}
		declaration, getErr := e.manager.GetDecl(current)
		if getErr != nil {
			return getErr
		}
		family, getErr := e.manager.GetFamilyID(declaration)
		if getErr != nil {
			return getErr
		}
		sort, getErr := e.manager.GetSort(current)
		if getErr != nil {
			return getErr
		}
		arity, getErr := e.manager.GetArity(declaration)
		if getErr != nil {
			return getErr
		}
		if family == ast.NullFamilyID && sort == boolean {
			if arity == 0 {
				present, presentErr := destination.HasConstantInterpretation(declaration)
				if presentErr != nil {
					return presentErr
				}
				if !present {
					if presentErr = destination.RegisterConstant(declaration, falsity); presentErr != nil {
						return presentErr
					}
				}
			} else {
				interpretation, presentErr := destination.FunctionInterpretation(declaration)
				if presentErr != nil {
					return presentErr
				}
				if interpretation == nil {
					interpretation, presentErr = model.NewFunctionInterpretation(e.manager, arity)
					if presentErr == nil {
						presentErr = interpretation.SetElse(falsity)
					}
					if presentErr == nil {
						presentErr = destination.RegisterFunction(declaration, interpretation)
						interpretation = nil
					}
					if interpretation != nil {
						presentErr = errors.Join(presentErr, interpretation.Close())
					}
					if presentErr != nil {
						return presentErr
					}
				}
			}
		}
		arguments, getErr := e.manager.GetNumArgs(current)
		if getErr != nil {
			return getErr
		}
		for index := uint32(0); index < arguments; index++ {
			argument, argumentErr := e.manager.GetArg(current, index)
			if argumentErr != nil {
				return argumentErr
			}
			stack = append(stack, argument)
		}
	}
	return nil
}

func (e *PropositionalEngine) applyHornInlineModelRules(destination *model.Model, rules []transforms.InlineModelRule) (resultErr error) {
	if len(rules) == 0 {
		return nil
	}
	if e.params == nil || e.globals == nil || e.guard == nil {
		return ErrPropositionalEngine
	}
	cooperation, err := cooperate.NewCooperation(e.manager.Budget())
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, cooperation.Close()) }()
	configuration := model.ModelEvaluatorConfig{
		Context:     e.params,
		Globals:     e.globals,
		Memory:      e.manager.NumeralService().RuntimeOwner(),
		Cooperation: cooperation,
	}
	for index := len(rules) - 1; index >= 0; index-- {
		declaration, normalized, normalizeErr := e.normalizeInlineModelRule(rules[index])
		if normalizeErr != nil {
			return normalizeErr
		}
		normalizedHandle, normalizeErr := e.manager.RefGet(normalized)
		if normalizeErr == nil {
			normalizeErr = e.addInlineDefaultInterpretations(destination.Core(), normalizedHandle)
		}
		evaluated, makeErr := e.manager.NewRef(ast.ASTHandle{})
		if normalizeErr == nil && makeErr == nil {
			evaluator, evaluatorErr := model.NewModelEvaluator(destination.Core(), configuration)
			if evaluatorErr == nil {
				evaluatorErr = evaluator.Evaluate(normalizedHandle, evaluated)
			}
			if evaluator != nil {
				evaluatorErr = errors.Join(evaluatorErr, evaluator.Close())
			}
			normalizeErr = evaluatorErr
		} else if normalizeErr == nil {
			normalizeErr = makeErr
		}
		if normalizeErr != nil {
			return errors.Join(normalizeErr, e.manager.DelRef(&evaluated), e.manager.DelRef(&normalized))
		}
		value, normalizeErr := e.manager.RefGet(evaluated)
		arity, arityErr := e.manager.GetArity(declaration)
		if normalizeErr == nil {
			normalizeErr = arityErr
		}
		var interpretation *model.FunctionInterpretation
		if normalizeErr == nil && arity == 0 {
			present, presentErr := destination.Core().HasConstantInterpretation(declaration)
			if presentErr == nil && present {
				var prior ast.ASTHandle
				prior, presentErr = destination.Core().ConstantInterpretation(declaration)
				if presentErr == nil {
					value, presentErr = e.manager.MkOr(prior, value)
				}
			}
			normalizeErr = presentErr
		} else if normalizeErr == nil {
			var presentErr error
			interpretation, presentErr = destination.Core().FunctionInterpretation(declaration)
			if presentErr == nil && interpretation != nil {
				prior, priorErr := interpretation.Else()
				if priorErr == nil && !prior.IsNull() {
					value, priorErr = e.manager.MkOr(prior, value)
				}
				presentErr = priorErr
			}
			normalizeErr = presentErr
		}
		if normalizeErr == nil {
			rewritten, rewriteErr := e.manager.NewRef(ast.ASTHandle{})
			if rewriteErr == nil {
				rewriteErr = e.guard.Rewrite(value, rewritten)
				if rewriteErr != nil {
					// Source's converter member keeps an interrupted traversal.
					// The Go error-return boundary permits a new model export, so
					// discard that attempt before returning to the caller. The
					// engine continues to own the guard if resetting is refused.
					rewriteErr = errors.Join(rewriteErr, e.guard.Reset())
				}
			}
			if rewriteErr == nil {
				value, rewriteErr = e.manager.RefGet(rewritten)
				if arity == 0 {
					rewriteErr = destination.Core().RegisterConstant(declaration, value)
				} else {
					created := false
					if interpretation == nil {
						interpretation, rewriteErr = model.NewFunctionInterpretation(e.manager, arity)
						created = rewriteErr == nil
					}
					if rewriteErr == nil {
						rewriteErr = interpretation.SetElse(value)
					}
					if rewriteErr == nil && created {
						rewriteErr = destination.Core().RegisterFunction(declaration, interpretation)
						if rewriteErr == nil {
							interpretation = nil
						}
					}
					if rewriteErr != nil && created && interpretation != nil {
						rewriteErr = errors.Join(rewriteErr, interpretation.Close())
					}
				}
			}
			normalizeErr = errors.Join(rewriteErr, e.manager.DelRef(&rewritten))
		}
		normalizeErr = errors.Join(normalizeErr, e.manager.DelRef(&evaluated), e.manager.DelRef(&normalized))
		if normalizeErr != nil {
			return normalizeErr
		}
	}
	return nil
}

func (e *PropositionalEngine) hornInlineEliminated(conversions []hornModelConversion, declaration ast.ASTHandle) (bool, error) {
	for _, conversion := range conversions {
		if conversion.kind != hornConvertInline {
			continue
		}
		for _, rule := range conversion.inline {
			head, err := e.manager.RefGet(rule.Head)
			if err != nil {
				return false, err
			}
			headDeclaration, err := e.manager.GetDecl(head)
			if err != nil {
				return false, err
			}
			if headDeclaration == declaration {
				return true, nil
			}
		}
	}
	return false, nil
}
