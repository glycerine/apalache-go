package fp

// Source: src/muz/rel/dl_compiler.cpp positive rule evaluation specialized
// to the concrete tr_sparse + interval_relation product instantiation.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

// SparseIntervalDatalogEvaluator borrows the relation manager and rule set,
// pins declarations, and owns cloned product state.
type SparseIntervalDatalogEvaluator struct {
	manager      *ast.ASTManager
	relations    *rel.RelationManager
	rules        *base.RuleSet
	guard        *astrewriter.TheoryRewriter // borrowed from the owning engine
	sorts        map[ast.ASTHandle][]ast.ASTHandle
	products     map[ast.ASTHandle]*rel.SparseIntervalSieveProduct
	declarations []ast.ASTRef
	closed       bool
}

func NewSparseIntervalDatalogEvaluator(manager *ast.ASTManager, relations *rel.RelationManager, rules *base.RuleSet, guard *astrewriter.TheoryRewriter, declarations []ast.ASTHandle, seeds map[ast.ASTHandle]*rel.SparseIntervalSieveProduct) (_ *SparseIntervalDatalogEvaluator, resultErr error) {
	if manager == nil || relations == nil || rules == nil || guard == nil {
		return nil, ErrSparseIntervalDatalogFragment
	}
	e := &SparseIntervalDatalogEvaluator{manager: manager, relations: relations, rules: rules, guard: guard, sorts: make(map[ast.ASTHandle][]ast.ASTHandle, len(declarations)), products: make(map[ast.ASTHandle]*rel.SparseIntervalSieveProduct, len(declarations))}
	defer func() {
		if resultErr != nil {
			resultErr = finishSparseIntervalEvaluatorConstruction(resultErr, e)
		}
	}()
	boolean, err := manager.MkBoolSort()
	if err != nil {
		return nil, err
	}
	for _, declaration := range declarations {
		if e.products[declaration] != nil {
			return nil, fmt.Errorf("%w: duplicate predicate", ErrSparseIntervalDatalogFragment)
		}
		rangeSort, err := manager.GetRange(declaration)
		if err != nil || rangeSort != boolean {
			return nil, errors.Join(err, ErrSparseIntervalDatalogFragment)
		}
		arity, err := manager.GetArity(declaration)
		if err != nil {
			return nil, err
		}
		sorts := make([]ast.ASTHandle, arity)
		for i := uint32(0); i < arity; i++ {
			sorts[i], err = manager.GetDomain(declaration, i)
			if err != nil {
				return nil, err
			}
		}
		seed := seeds[declaration]
		if seed == nil {
			return nil, fmt.Errorf("%w: missing product seed", ErrSparseIntervalDatalogFragment)
		}
		product, err := seed.CloneTo(relations)
		if err != nil {
			return nil, err
		}
		e.sorts[declaration] = sorts
		e.products[declaration] = product
		root, err := manager.NewRef(declaration)
		if err != nil {
			return nil, err
		}
		e.declarations = append(e.declarations, root)
	}
	for declaration := range seeds {
		if e.products[declaration] == nil {
			return nil, fmt.Errorf("%w: seed predicate was not declared", ErrSparseIntervalDatalogFragment)
		}
	}
	return e, nil
}

func (e *SparseIntervalDatalogEvaluator) open() error {
	if e == nil || e.closed || e.manager == nil || e.relations == nil || e.rules == nil || e.guard == nil || e.sorts == nil || e.products == nil {
		return ErrSparseIntervalDatalogFragment
	}
	return nil
}

func (e *SparseIntervalDatalogEvaluator) checkpoint() error {
	canceled, err := e.manager.Canceled()
	if err != nil {
		return err
	}
	if canceled {
		return z3exception.NewDefaultException(e.manager.ResourceLimit().CancelMessage())
	}
	return nil
}

func (e *SparseIntervalDatalogEvaluator) applyFactRule(rule *base.Rule) (resultErr error) {
	defer func() { resultErr = errors.Join(resultErr, rule.Close()) }()
	tails, err := rule.TailSize()
	if err != nil || tails != 0 {
		return errors.Join(err, fmt.Errorf("%w: non-fact rule", ErrSparseIntervalDatalogFragment))
	}
	declaration, err := rule.HeadDeclaration()
	if err != nil || e.products[declaration] == nil {
		return errors.Join(err, fmt.Errorf("%w: undeclared rule head", ErrSparseIntervalDatalogFragment))
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
	if kind, cause := e.manager.GetKind(head); cause != nil {
		return cause
	} else if kind == ast.ASTQuantifier {
		head, err = e.manager.GetQuantifierBody(head)
		if err != nil {
			return err
		}
	}
	if actual, cause := e.manager.GetDecl(head); cause != nil || actual != declaration {
		return errors.Join(cause, ErrSparseIntervalDatalogFragment)
	}
	count, err := e.manager.GetNumArgs(head)
	if err != nil || int(count) != len(e.sorts[declaration]) {
		return errors.Join(err, ErrSparseIntervalDatalogFragment)
	}
	values := make([]ast.ASTHandle, count)
	for i := uint32(0); i < count; i++ {
		values[i], err = e.manager.GetArg(head, i)
		if err != nil {
			return err
		}
	}
	if err = e.products[declaration].AddFact(values); err != nil {
		return err
	}
	return nil
}

func (e *SparseIntervalDatalogEvaluator) Query(declarations []ast.ASTHandle) (lbool.LBool, error) {
	if err := e.open(); err != nil {
		return lbool.Undef, err
	}
	if len(declarations) == 0 {
		return lbool.True, nil
	}
	precise := false
	for _, declaration := range declarations {
		product := e.products[declaration]
		if product == nil {
			return lbool.Undef, fmt.Errorf("%w: undeclared query predicate", ErrSparseIntervalDatalogFragment)
		}
		empty, err := product.Empty()
		if err != nil {
			return lbool.Undef, err
		}
		if !empty {
			exact, cause := product.IsPrecise()
			if cause != nil {
				return lbool.Undef, cause
			}
			if !exact {
				return lbool.Undef, nil
			}
			precise = true
		}
	}
	if precise {
		return lbool.True, nil
	}
	return lbool.False, nil
}

func (e *SparseIntervalDatalogEvaluator) Relation(declaration ast.ASTHandle) (*rel.SparseIntervalSieveProduct, error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	product := e.products[declaration]
	if product == nil {
		return nil, ErrSparseIntervalDatalogFragment
	}
	return product.Clone()
}

func (e *SparseIntervalDatalogEvaluator) RelationFormula(declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := e.open(); err != nil {
		return ast.ASTRef{}, err
	}
	product := e.products[declaration]
	if product == nil {
		return ast.ASTRef{}, ErrSparseIntervalDatalogFragment
	}
	return product.ToFormula()
}

func (e *SparseIntervalDatalogEvaluator) Close() (result error) {
	if e == nil || e.closed {
		return nil
	}
	for declaration, product := range e.products {
		if err := product.Close(); err != nil {
			result = errors.Join(result, err)
			continue
		}
		delete(e.products, declaration)
	}
	for i := len(e.declarations) - 1; i >= 0; i-- {
		result = errors.Join(result, e.manager.DelRef(&e.declarations[i]))
	}
	if result != nil {
		return result
	}
	e.closed = true
	e.manager, e.relations, e.rules, e.guard, e.sorts, e.products, e.declarations = nil, nil, nil, nil, nil, nil, nil
	return nil
}
