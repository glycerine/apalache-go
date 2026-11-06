package fp

// Source: rel_context plugin selection and query lifecycle, specialized to
// the live interval_relation instantiation. Main engine dispatch deliberately
// remains outside this helper.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// intervalDatalogEligible recognizes the complete predicate-signature
// boundary of the typed interval evaluator. Zero-arity Boolean predicates are
// eligible: they are the synthetic truth relations used by expression query
// clones and need no arithmetic columns.
func intervalDatalogEligible(manager *ast.ASTManager, declarations []ast.ASTHandle) (bool, error) {
	if manager == nil || len(declarations) == 0 {
		return false, nil
	}
	boolean, err := manager.MkBoolSort()
	if err != nil {
		return false, err
	}
	seen := make(map[ast.ASTHandle]bool, len(declarations))
	for _, declaration := range declarations {
		kind, err := manager.GetKind(declaration)
		if err != nil {
			return false, err
		}
		if kind != ast.ASTFuncDecl || seen[declaration] {
			return false, ErrIntervalDatalogFragment
		}
		seen[declaration] = true
		rangeSort, err := manager.GetRange(declaration)
		if err != nil {
			return false, err
		}
		if rangeSort != boolean {
			return false, nil
		}
		arity, err := manager.GetArity(declaration)
		if err != nil {
			return false, err
		}
		sorts := make([]ast.ASTHandle, arity)
		for i := uint32(0); i < arity; i++ {
			sorts[i], err = manager.GetDomain(declaration, i)
			if err != nil {
				return false, err
			}
		}
		if !rel.CanHandleIntervalSorts(manager, sorts) {
			return false, nil
		}
	}
	return true, nil
}

// intervalDatalogActivation owns interval fact storage and the most recent
// saturated evaluator snapshot. It is installed only after eligibility and
// representation gates select the interval relation plugin.
type intervalDatalogActivation struct {
	manager      *ast.ASTManager
	rules        *base.RuleSet
	store        *IntervalDatalogStore
	evaluator    *IntervalDatalogEvaluator
	similarity   *rel.IntervalSimilarityProgram
	predicates   []ast.ASTHandle
	closed       bool
	formulaCheck *smtcore.SMTSolverConfig
}

func newIntervalDatalogActivation(manager *ast.ASTManager, rules *base.RuleSet, declarations []ast.ASTHandle) (_ *intervalDatalogActivation, selected bool, resultErr error) {
	eligible, err := intervalDatalogEligible(manager, declarations)
	if err != nil || !eligible {
		return nil, false, err
	}
	activation := &intervalDatalogActivation{manager: manager, rules: rules, predicates: append([]ast.ASTHandle(nil), declarations...)}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, activation.close())
		}
	}()
	activation.store, err = NewIntervalDatalogStore(manager)
	if err != nil {
		return nil, true, err
	}
	for _, declaration := range declarations {
		if err = activation.store.Register(declaration); err != nil {
			return nil, true, err
		}
	}
	return activation, true, nil
}

func (a *intervalDatalogActivation) open() error {
	if a == nil || a.closed || a.manager == nil || a.rules == nil || a.store == nil {
		return ErrIntervalDatalogFragment
	}
	return nil
}

func (a *intervalDatalogActivation) verifyEmptySeeds() error {
	if a.formulaCheck == nil {
		return nil
	}
	for _, entry := range a.store.entries {
		if err := rel.VerifyIntervalRelationConstant(a.manager, entry.sorts, *a.formulaCheck, entry.relation, false); err != nil {
			return err
		}
	}
	return nil
}

func (a *intervalDatalogActivation) discardEvaluator() error {
	if a.evaluator != nil {
		if err := a.evaluator.Close(); err != nil {
			return err
		}
		a.evaluator = nil
	}
	if a.similarity != nil {
		if err := a.similarity.Close(); err != nil {
			return err
		}
		a.similarity = nil
	}
	return nil
}

func (a *intervalDatalogActivation) addFact(declaration ast.ASTHandle, values []ast.ASTHandle) error {
	if err := a.open(); err != nil {
		return err
	}
	if err := a.discardEvaluator(); err != nil {
		return err
	}
	entry := a.store.entries[declaration]
	if entry == nil || a.formulaCheck == nil {
		return a.store.AddFact(declaration, values)
	}
	beforeFormula, err := entry.relation.ToFormula(a.manager, entry.sorts)
	if err != nil {
		return err
	}
	defer a.manager.DelRef(&beforeFormula)
	before, err := a.manager.RefGet(beforeFormula)
	if err != nil {
		return err
	}
	if err = a.store.AddFact(declaration, values); err != nil {
		return err
	}
	return rel.VerifyIntervalRelationFactAdd(a.manager, entry.sorts, *a.formulaCheck, before, values, entry.relation)
}

// syncPredicates monotonically extends a selected activation to the engine's
// current declaration order. This covers predicates registered after an early
// fact and synthetic zero-arity query heads installed by expression clones.
// Once interval execution is selected, a later mixed signature is an error,
// never a reason to discard seeds and fall back to a different relation kind.
func (a *intervalDatalogActivation) syncPredicates(declarations []ast.ASTHandle) error {
	if err := a.open(); err != nil {
		return err
	}
	eligible, err := intervalDatalogEligible(a.manager, declarations)
	if err != nil {
		return err
	}
	if !eligible {
		return ErrIntervalDatalogFragment
	}
	current := make(map[ast.ASTHandle]bool, len(declarations))
	for _, declaration := range declarations {
		current[declaration] = true
	}
	for _, declaration := range a.predicates {
		if !current[declaration] {
			return ErrIntervalDatalogFragment
		}
	}
	if err = a.discardEvaluator(); err != nil {
		return err
	}
	for _, declaration := range declarations {
		if a.store.entries[declaration] != nil {
			continue
		}
		if err = a.store.Register(declaration); err != nil {
			return err
		}
		if a.formulaCheck != nil {
			entry := a.store.entries[declaration]
			if err = rel.VerifyIntervalRelationConstant(a.manager, entry.sorts, *a.formulaCheck, entry.relation, false); err != nil {
				return err
			}
		}
		a.predicates = append(a.predicates, declaration)
	}
	return nil
}

// clone copies seed relations while borrowing the destination clone's rule
// set. Saturated evaluator state is intentionally rebuilt from those sources.
func (a *intervalDatalogActivation) clone(rules *base.RuleSet) (_ *intervalDatalogActivation, resultErr error) {
	if err := a.open(); err != nil || rules == nil {
		if err == nil {
			err = ErrIntervalDatalogFragment
		}
		return nil, err
	}
	copyActivation, selected, err := newIntervalDatalogActivation(a.manager, rules, a.predicates)
	if err != nil || !selected {
		return nil, errors.Join(err, ErrIntervalDatalogFragment)
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, copyActivation.close())
		}
	}()
	for declaration, entry := range a.store.entries {
		relation, err := entry.relation.Clone()
		if err != nil {
			return nil, err
		}
		copyActivation.store.entries[declaration].relation = relation
		if a.formulaCheck != nil {
			if err = rel.VerifyIntervalRelationClone(a.manager, entry.sorts, *a.formulaCheck, entry.relation, relation); err != nil {
				return nil, err
			}
		}
	}
	if a.formulaCheck != nil {
		config := *a.formulaCheck
		copyActivation.formulaCheck = &config
	}
	return copyActivation, nil
}

// query snapshots facts and rules for each call, so rule mutation cannot
// accidentally reuse a stale result. Unsupported selected fragments return an
// error instead of silently falling back to finite execution.
func (a *intervalDatalogActivation) query(declarations []ast.ASTHandle, widen bool) (lbool.LBool, error) {
	if err := a.open(); err != nil {
		return lbool.Undef, err
	}
	if err := a.discardEvaluator(); err != nil {
		return lbool.Undef, err
	}
	evaluator, err := a.store.Evaluator(a.rules)
	if err != nil {
		return lbool.Undef, err
	}
	evaluator.formulaCheck = a.formulaCheck
	if a.formulaCheck != nil {
		for declaration, entry := range a.store.entries {
			if err = rel.VerifyIntervalRelationClone(a.manager, entry.sorts, *a.formulaCheck, entry.relation, evaluator.relations[declaration]); err != nil {
				return lbool.Undef, errors.Join(err, evaluator.Close())
			}
		}
	}
	if err = evaluator.SaturatePositive(widen); err != nil {
		return lbool.Undef, errors.Join(err, evaluator.Close())
	}
	a.evaluator = evaluator
	return evaluator.Query(declarations)
}

func (a *intervalDatalogActivation) relation(declaration ast.ASTHandle) (*rel.IntervalRelation, error) {
	if err := a.open(); err != nil || a.evaluator == nil {
		if err == nil {
			err = ErrIntervalDatalogFragment
		}
		return nil, err
	}
	return a.evaluator.Relation(declaration)
}

func (a *intervalDatalogActivation) relationFormula(declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := a.open(); err != nil || a.evaluator == nil {
		if err == nil {
			err = ErrIntervalDatalogFragment
		}
		return ast.ASTRef{}, err
	}
	return a.evaluator.RelationFormula(declaration)
}

func (a *intervalDatalogActivation) reset() error {
	if err := a.open(); err != nil {
		return err
	}
	if err := a.discardEvaluator(); err != nil {
		return err
	}
	if err := a.store.Reset(); err != nil {
		return err
	}
	return a.verifyEmptySeeds()
}

func (a *intervalDatalogActivation) close() (result error) {
	if a == nil || a.closed {
		return nil
	}
	result = errors.Join(result, a.discardEvaluator())
	if a.store != nil {
		result = errors.Join(result, a.store.Close())
	}
	if result != nil {
		return result
	}
	a.closed = true
	a.manager, a.rules, a.store, a.predicates = nil, nil, nil, nil
	return nil
}
