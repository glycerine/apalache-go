package fp

// Source: rel_context relation-plugin selection plus interval_relation::add_fact.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/smtcore"
)

var ErrIntervalDatalogFragment = errors.New("muz/fp: unsupported arithmetic interval Datalog fragment")

// IntervalDatalogEvaluator is the separate typed arithmetic relation lane.
// It borrows the rule set and owns declaration pins plus cloned relation state.
type IntervalDatalogEvaluator struct {
	manager      *ast.ASTManager
	rules        *base.RuleSet
	sorts        map[ast.ASTHandle][]ast.ASTHandle
	relations    map[ast.ASTHandle]*rel.IntervalRelation
	declarations []ast.ASTRef
	closed       bool
	formulaCheck *smtcore.SMTSolverConfig
}

func NewIntervalDatalogEvaluator(manager *ast.ASTManager, rules *base.RuleSet, declarations []ast.ASTHandle, seeds map[ast.ASTHandle]*rel.IntervalRelation) (_ *IntervalDatalogEvaluator, resultErr error) {
	if manager == nil || rules == nil {
		return nil, ErrIntervalDatalogFragment
	}
	e := &IntervalDatalogEvaluator{manager: manager, rules: rules, sorts: make(map[ast.ASTHandle][]ast.ASTHandle, len(declarations)), relations: make(map[ast.ASTHandle]*rel.IntervalRelation, len(declarations))}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, e.Close())
		}
	}()
	for _, declaration := range declarations {
		if _, duplicate := e.sorts[declaration]; duplicate {
			return nil, fmt.Errorf("%w: duplicate predicate", ErrIntervalDatalogFragment)
		}
		arity, err := manager.GetArity(declaration)
		if err != nil {
			return nil, err
		}
		rangeSort, err := manager.GetRange(declaration)
		if err != nil {
			return nil, err
		}
		boolean, err := manager.MkBoolSort()
		if err != nil {
			return nil, err
		}
		if rangeSort != boolean {
			return nil, fmt.Errorf("%w: predicate range is not Boolean", ErrIntervalDatalogFragment)
		}
		sorts := make([]ast.ASTHandle, arity)
		for i := uint32(0); i < arity; i++ {
			sorts[i], err = manager.GetDomain(declaration, i)
			if err != nil {
				return nil, err
			}
		}
		if !rel.CanHandleIntervalSorts(manager, sorts) {
			return nil, fmt.Errorf("%w: predicate signature is not entirely arithmetic", ErrIntervalDatalogFragment)
		}
		root, err := manager.NewRef(declaration)
		if err != nil {
			return nil, err
		}
		e.declarations = append(e.declarations, root)
		e.sorts[declaration] = sorts
		relation := rel.NewIntervalEmpty(arity)
		if seed := seeds[declaration]; seed != nil {
			seedArity, cause := seed.Arity()
			if cause != nil || seedArity != arity {
				return nil, errors.Join(cause, fmt.Errorf("%w: seed arity mismatch", ErrIntervalDatalogFragment))
			}
			relation, err = seed.Clone()
			if err != nil {
				return nil, err
			}
		}
		e.relations[declaration] = relation
	}
	for declaration := range seeds {
		if _, found := e.relations[declaration]; !found {
			return nil, fmt.Errorf("%w: seed predicate was not declared", ErrIntervalDatalogFragment)
		}
	}
	return e, nil
}

func (e *IntervalDatalogEvaluator) open() error {
	if e == nil || e.closed || e.manager == nil || e.rules == nil || e.sorts == nil || e.relations == nil {
		return ErrIntervalDatalogFragment
	}
	return nil
}

func (e *IntervalDatalogEvaluator) applyFactRule(rule *base.Rule) (resultErr error) {
	defer func() { resultErr = errors.Join(resultErr, rule.Close()) }()
	tails, err := rule.TailSize()
	if err != nil {
		return err
	}
	if tails != 0 {
		return fmt.Errorf("%w: non-fact rule", ErrIntervalDatalogFragment)
	}
	declaration, err := rule.HeadDeclaration()
	if err != nil {
		return err
	}
	relation := e.relations[declaration]
	if relation == nil {
		return fmt.Errorf("%w: undeclared rule head", ErrIntervalDatalogFragment)
	}
	formula, err := rule.ToFormula()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, e.manager.DelRef(&formula)) }()
	head, err := e.manager.RefGet(formula)
	if err != nil {
		return err
	}
	kind, err := e.manager.GetKind(head)
	if err != nil {
		return err
	}
	if kind == ast.ASTQuantifier {
		head, err = e.manager.GetQuantifierBody(head)
		if err != nil {
			return err
		}
	}
	if kind, err = e.manager.GetKind(head); err != nil || kind != ast.ASTApp {
		return errors.Join(err, fmt.Errorf("%w: fact head is not an application", ErrIntervalDatalogFragment))
	}
	actual, err := e.manager.GetDecl(head)
	if err != nil || actual != declaration {
		return errors.Join(err, fmt.Errorf("%w: fact head declaration mismatch", ErrIntervalDatalogFragment))
	}
	count, err := e.manager.GetNumArgs(head)
	if err != nil || int(count) != len(e.sorts[declaration]) {
		return errors.Join(err, fmt.Errorf("%w: fact head arity mismatch", ErrIntervalDatalogFragment))
	}
	values := make([]ast.ASTHandle, count)
	for i := uint32(0); i < count; i++ {
		values[i], err = e.manager.GetArg(head, i)
		if err != nil {
			return err
		}
	}
	if err = e.addFact(relation, e.sorts[declaration], values); err != nil {
		return fmt.Errorf("interval add_fact: %w", err)
	}
	return nil
}

func (e *IntervalDatalogEvaluator) addFact(relation *rel.IntervalRelation, sorts, values []ast.ASTHandle) (resultErr error) {
	if e.formulaCheck == nil {
		return relation.AddFactExpressions(e.manager, sorts, values)
	}
	beforeFormula, err := relation.ToFormula(e.manager, sorts)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, e.manager.DelRef(&beforeFormula)) }()
	before, err := e.manager.RefGet(beforeFormula)
	if err != nil {
		return err
	}
	if err = relation.AddFactExpressions(e.manager, sorts, values); err != nil {
		return err
	}
	return rel.VerifyIntervalRelationFactAdd(e.manager, sorts, *e.formulaCheck, before, values, relation)
}

// SaturateFacts evaluates zero-tail source rules using interval abstraction.
// Other rule shapes fail explicitly until their lowering is implemented.
func (e *IntervalDatalogEvaluator) SaturateFacts() (resultErr error) {
	if err := e.open(); err != nil {
		return err
	}
	previous := make(map[ast.ASTHandle]*rel.IntervalRelation, len(e.relations))
	for declaration, relation := range e.relations {
		clone, err := relation.Clone()
		if err != nil {
			return err
		}
		previous[declaration] = clone
	}
	defer func() {
		if resultErr != nil {
			e.relations = previous
		}
	}()
	count, err := e.rules.Len()
	if err != nil {
		return err
	}
	for i := 0; i < count; i++ {
		rule, err := e.rules.Rule(i)
		if err != nil {
			return err
		}
		if resultErr = e.applyFactRule(rule); resultErr != nil {
			return resultErr
		}
	}
	return nil
}

// Relation returns an independent snapshot of the current abstract relation.
func (e *IntervalDatalogEvaluator) Relation(declaration ast.ASTHandle) (*rel.IntervalRelation, error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	r := e.relations[declaration]
	if r == nil {
		return nil, ErrIntervalDatalogFragment
	}
	clone, err := r.Clone()
	if err != nil {
		return nil, err
	}
	if e.formulaCheck != nil {
		if err = rel.VerifyIntervalRelationClone(e.manager, e.sorts[declaration], *e.formulaCheck, r, clone); err != nil {
			return nil, err
		}
	}
	return clone, nil
}

func (e *IntervalDatalogEvaluator) Close() (result error) {
	if e == nil || e.closed {
		return nil
	}
	for i := len(e.declarations) - 1; i >= 0; i-- {
		result = errors.Join(result, e.manager.DelRef(&e.declarations[i]))
	}
	if result != nil {
		return result
	}
	e.closed = true
	e.manager, e.rules, e.sorts, e.relations, e.declarations = nil, nil, nil, nil, nil
	return nil
}
