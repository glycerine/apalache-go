package fp

// Source: src/muz/fp/horn_tactic.cpp retained finite verification path.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type HornTactic struct {
	manager     *ast.ASTManager
	context     *uparams.ParamsContext
	globals     *uparams.GlobalParamsService
	params      uparams.ParamsRef
	paramsReady bool
	simplify    bool
	statistics  map[string]float64
	progress    func(uint32)
	logic       usymbol.Symbol
	references  uint32
	closed      bool
}

func NewHornTactic(manager *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService) (*HornTactic, error) {
	return newHornTactic(manager, context, globals, false)
}

func newHornTactic(manager *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, simplify bool) (*HornTactic, error) {
	if manager == nil || context == nil || globals == nil {
		return nil, tactic.NewTacticException("horn tactic services are unavailable")
	}
	params, err := context.NewValue()
	if err != nil {
		return nil, err
	}
	return &HornTactic{manager: manager, context: context, globals: globals, params: params, paramsReady: true, simplify: simplify, statistics: make(map[string]float64)}, nil
}

func (t *HornTactic) open() error {
	if t == nil || t.closed || t.manager == nil || t.context == nil || t.globals == nil {
		return tactic.NewTacticException("horn tactic is closed")
	}
	return nil
}

func (t *HornTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.references++
	return nil
}

func (t *HornTactic) DecRef() error {
	if err := t.open(); err != nil {
		return err
	}
	if t.references == 0 {
		return tactic.NewTacticException("horn tactic reference underflow")
	}
	t.references--
	if t.references == 0 {
		var result error
		if t.paramsReady {
			result = t.context.Del(&t.params)
			t.paramsReady = false
		}
		t.closed = true
		t.manager, t.context, t.globals = nil, nil, nil
		return result
	}
	return nil
}

func (t *HornTactic) predicate(expression ast.ASTHandle) (ast.ASTHandle, bool, error) {
	kind, err := t.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return ast.ASTHandle{}, false, err
	}
	declaration, err := t.manager.GetDecl(expression)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	family, err := t.manager.GetFamilyID(declaration)
	if err != nil || family != ast.NullFamilyID {
		return ast.ASTHandle{}, false, err
	}
	sort, err := t.manager.GetSort(expression)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	boolean, err := t.manager.MkBoolSort()
	return declaration, err == nil && sort == boolean, err
}

func (t *HornTactic) registerPredicates(engine *PropositionalEngine, root ast.ASTHandle, seen map[ast.ASTHandle]bool) error {
	if declaration, yes, err := t.predicate(root); err != nil {
		return err
	} else if yes {
		if !seen[declaration] {
			if err = engine.RegisterPredicate(declaration); err != nil {
				return err
			}
			seen[declaration] = true
		}
		return nil
	}
	kind, err := t.manager.GetKind(root)
	if err != nil {
		return err
	}
	if kind == ast.ASTQuantifier {
		body, x := t.manager.GetQuantifierBody(root)
		if x != nil {
			return x
		}
		return t.registerPredicates(engine, body, seen)
	}
	if kind != ast.ASTApp {
		return nil
	}
	family, err := t.manager.GetFamilyID(root)
	if err != nil || family != t.manager.BasicFamilyID() {
		return err
	}
	operation, err := t.manager.GetDeclKind(root)
	if err != nil {
		return err
	}
	first := uint32(0)
	switch operation {
	case ast.ASTOpNot, ast.ASTOpAnd, ast.ASTOpOr, ast.ASTOpImplies:
	case ast.ASTOpIte:
		first = 1 // The condition is a theory constraint, not a relation.
	default:
		return nil
	}
	count, err := t.manager.GetNumArgs(root)
	if err != nil {
		return err
	}
	for index := count; index > first; {
		index-- // Source check_predicate processes its explicit stack last-first.
		child, x := t.manager.GetArg(root, index)
		if x != nil {
			return x
		}
		if x = t.registerPredicates(engine, child, seen); x != nil {
			return x
		}
	}
	return nil
}

func (t *HornTactic) pin(temporaries *[]ast.ASTRef, handle ast.ASTHandle) error {
	reference, err := t.manager.NewRef(handle)
	if err == nil {
		*temporaries = append(*temporaries, reference)
	}
	return err
}

func (t *HornTactic) stripForall(formula ast.ASTHandle) (ast.ASTHandle, error) {
	for {
		kind, err := t.manager.GetKind(formula)
		if err != nil || kind != ast.ASTQuantifier {
			return formula, err
		}
		forall, err := t.manager.IsForall(formula)
		if err != nil || !forall {
			return formula, err
		}
		formula, err = t.manager.GetQuantifierBody(formula)
		if err != nil {
			return formula, err
		}
	}
}

// bindQueryRule mirrors imp::bind_variables independently of the context's
// fix_unbound_vars option, including default Boolean sorts for index gaps.
func (t *HornTactic) bindQueryRule(rule ast.ASTHandle, temporaries *[]ast.ASTRef) (ast.ASTHandle, error) {
	free, err := rewriter.NewExprFreeVars(t.manager)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = free.Collect(rule); err != nil {
		return ast.ASTHandle{}, err
	}
	if free.Empty() {
		return rule, nil
	}
	boolean, err := t.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = free.SetDefaultSort(boolean); err != nil {
		return ast.ASTHandle{}, err
	}
	if err = free.Reverse(); err != nil {
		return ast.ASTHandle{}, err
	}
	sorts := free.Sorts()
	names := make([]usymbol.Symbol, len(sorts))
	for i := range names {
		names[i] = usymbol.Numerical(uint32(len(names) - i - 1))
	}
	bound, err := t.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: rule})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = t.pin(temporaries, bound); err != nil {
		return ast.ASTHandle{}, err
	}
	return bound, nil
}

func (t *HornTactic) isImplication(formula ast.ASTHandle) (bool, error) {
	current, err := t.stripForall(formula)
	if err != nil {
		return false, err
	}
	for {
		operator, err := t.manager.GetDeclKind(current)
		if err != nil {
			return false, err
		}
		if operator != ast.ASTOpImplies {
			_, predicate, err := t.predicate(current)
			return predicate, err
		}
		current, err = t.manager.GetArg(current, 1)
		if err != nil {
			return false, err
		}
	}
}

func (t *HornTactic) normalizePolarity(formula ast.ASTHandle, temporaries *[]ast.ASTRef) (ast.ASTHandle, error) {
	positive := true
	for {
		kind, err := t.manager.GetKind(formula)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if kind == ast.ASTQuantifier {
			forall, x := t.manager.IsForall(formula)
			if x != nil {
				return ast.ASTHandle{}, x
			}
			if (forall && positive) || (!forall && !positive) {
				formula, x = t.manager.GetQuantifierBody(formula)
				if x != nil {
					return ast.ASTHandle{}, x
				}
				continue
			}
		}
		if kind == ast.ASTApp {
			operator, x := t.manager.GetDeclKind(formula)
			if x != nil {
				return ast.ASTHandle{}, x
			}
			if operator == ast.ASTOpNot {
				formula, x = t.manager.GetArg(formula, 0)
				if x != nil {
					return ast.ASTHandle{}, x
				}
				positive = !positive
				continue
			}
		}
		break
	}
	if positive {
		return formula, nil
	}
	negated, err := t.manager.MkNot(formula)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = t.pin(temporaries, negated); err != nil {
		return ast.ASTHandle{}, err
	}
	return negated, nil
}

// classify applies the source tactic's disjunctive Horn-clause view. A clause
// has at most one positive predicate; clauses without one are query bodies.
func (t *HornTactic) classify(formula ast.ASTHandle, temporaries *[]ast.ASTRef) (rule bool, normalized ast.ASTHandle, err error) {
	analysis, err := t.normalizePolarity(formula, temporaries)
	if err != nil {
		return false, normalized, err
	}
	arguments, err := t.manager.FlattenOr([]ast.ASTHandle{analysis})
	if err != nil {
		return false, normalized, err
	}
	body := make([]ast.ASTHandle, 0, len(arguments))
	var head ast.ASTHandle
	for _, argument := range arguments {
		operator, x := t.manager.GetDeclKind(argument)
		if x != nil {
			return false, normalized, x
		}
		if operator == ast.ASTOpNot {
			child, y := t.manager.GetArg(argument, 0)
			if y != nil {
				return false, normalized, y
			}
			body = append(body, child)
			continue
		}
		if _, predicate, y := t.predicate(argument); y != nil {
			return false, normalized, y
		} else if predicate {
			if !head.IsNull() {
				return false, normalized, tactic.NewTacticException("formula is not in Horn fragment")
			}
			head = argument
			continue
		}
		negated, y := t.manager.MkNot(argument)
		if y != nil {
			return false, normalized, y
		}
		if y = t.pin(temporaries, negated); y != nil {
			return false, normalized, y
		}
		body = append(body, negated)
	}
	antecedent, err := t.manager.MkAndArray(body)
	if err != nil {
		return false, normalized, err
	}
	if err = t.pin(temporaries, antecedent); err != nil {
		return false, normalized, err
	}
	if head.IsNull() {
		return false, antecedent, nil
	}
	implication, err := t.isImplication(formula)
	if err != nil {
		return false, normalized, err
	}
	if implication {
		return true, formula, nil
	}
	if len(body) == 0 {
		return true, head, nil
	}
	normalized, err = t.manager.MkImplies(antecedent, head)
	if err != nil {
		return false, normalized, err
	}
	if err = t.pin(temporaries, normalized); err != nil {
		return false, normalized, err
	}
	return true, normalized, nil
}

func (t *HornTactic) Apply(goal *tactic.Goal, out *tactic.TacticResult) (resultErr error) {
	if err := t.open(); err != nil || goal == nil || goal.Manager() != t.manager {
		return errors.Join(err, tactic.NewTacticException("invalid Horn goal"))
	}
	if err := out.Prepare(t.manager); err != nil {
		return err
	}
	proofsEnabled, err := goal.ProofsEnabled()
	if err != nil {
		return err
	}
	modelsEnabled, err := goal.ModelsEnabled()
	if err != nil {
		return err
	}
	engine, err := NewPropositionalEngineWithServices(t.manager, t.context, t.globals)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, engine.Close()) }()
	if t.paramsReady {
		if err = engine.SetQueryParameters(&t.params); err != nil {
			return err
		}
	}
	defer func() { t.recordEngineStatistics(engine) }()
	engine.forceProofTrace = proofsEnabled
	var temporaries []ast.ASTRef
	defer func() {
		for index := len(temporaries) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, t.manager.DelRef(&temporaries[index]))
		}
	}()
	seen := make(map[ast.ASTHandle]bool)
	var queries []ast.ASTHandle
	size, err := goal.Size()
	if err != nil {
		return err
	}
	for index := uint32(0); index < size; index++ {
		formula, x := goal.Form(index)
		if x != nil {
			return x
		}
		if x = t.registerPredicates(engine, formula, seen); x != nil {
			return x
		}
		isRule, normalized, x := t.classify(formula, &temporaries)
		if x != nil {
			return x
		}
		if isRule {
			if x = engine.AddRule(normalized, "horn-tactic"); x != nil {
				return x
			}
		} else {
			queries = append(queries, normalized)
		}
	}
	var query, declaration ast.ASTHandle
	if len(queries) == 1 && !t.simplify {
		query = queries[0]
	} else {
		boolean, x := t.manager.MkBoolSort()
		if x != nil {
			return x
		}
		query, x = t.manager.MkFreshConst("query", boolean)
		if x != nil {
			return x
		}
		if x = t.pin(&temporaries, query); x != nil {
			return x
		}
		var yes bool
		declaration, yes, x = t.predicate(query)
		if x != nil || !yes {
			return errors.Join(x, tactic.NewTacticException("fresh Horn query is not a predicate"))
		}
		if x = engine.RegisterPredicate(declaration); x != nil {
			return x
		}
		for _, body := range queries {
			rule, y := t.manager.MkImplies(body, query)
			if y != nil {
				return y
			}
			if y = t.pin(&temporaries, rule); y != nil {
				return y
			}
			rule, y = t.bindQueryRule(rule, &temporaries)
			if y != nil {
				return y
			}
			if y = engine.AddRule(rule, "horn-query"); y != nil {
				return y
			}
		}
		filter, x := tactic.NewFilterModel(t.manager)
		if x != nil {
			return x
		}
		if x = filter.Insert(declaration); x != nil {
			return errors.Join(x, filter.Close())
		}
		if x = out.SetModelConverter(filter); x != nil {
			return errors.Join(x, filter.Close())
		}
	}
	if t.simplify {
		return t.simplifyGoal(goal, out, engine, query, declaration)
	}
	// The backend receives the expression and constructs its own query rule.
	if err = engine.SelectEngine("auto-config"); err != nil {
		return err
	}
	status, err := engine.QueryExpression(query)
	if err != nil {
		return err
	}
	resultEngine, resultDeclaration := engine, declaration
	if engine.queryClone != nil {
		resultEngine, resultDeclaration = engine.queryClone, engine.queryClonePredicate
	}
	if err = goal.IncDepth(); err != nil {
		return err
	}
	if status == lbool.True {
		falsity, x := t.manager.MkFalse()
		if x != nil {
			return x
		}
		if proofsEnabled {
			proof, y := resultEngine.Proof(resultDeclaration)
			if y != nil {
				return y
			}
			proofHandle, y := t.manager.RefGet(proof)
			if y == nil {
				var converter tactic.ProofConverter
				converter, y = tactic.NewStoredProofConverter(t.manager, proofHandle)
				if y == nil && converter != nil {
					if y = out.SetProofConverter(converter); y != nil {
						y = errors.Join(y, converter.Close())
					}
				}
			}
			if y == nil {
				y = goal.AssertWithProof(falsity, proofHandle, ast.ASTDependency{})
			}
			y = errors.Join(y, t.manager.DelRef(&proof))
			if y != nil {
				return y
			}
		} else if x = goal.Assert(falsity); x != nil {
			return x
		}
	} else if status == lbool.False {
		if modelsEnabled {
			inductive, x := resultEngine.Model(resultDeclaration)
			if x != nil {
				return x
			}
			converter, x := tactic.NewStoredModel(inductive.Core())
			if x != nil {
				return errors.Join(x, inductive.Close())
			}
			// NewStoredModel establishes the model's sole counted reference.
			// The raw zero-reference result is now owned by the converter and
			// must not be closed directly.
			combined, y := tactic.ConcatModels(t.manager.Budget(), out.ModelConverter, converter)
			if y != nil {
				return errors.Join(y, converter.Close())
			}
			if x = out.SetModelConverter(combined); x != nil {
				return errors.Join(x, combined.Close())
			}
		}
		if err = goal.Reset(); err != nil {
			return err
		}
	}
	if t.progress != nil {
		t.progress(1)
	}
	return out.AddGoal(goal)
}
