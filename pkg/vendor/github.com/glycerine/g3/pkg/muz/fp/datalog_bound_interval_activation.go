package fp

// Source: rel_context::set_predicate_representation selects a product kind
// for the requested bound_relation plus interval_relation pair. This first
// activation supports exact-pair originals, inherited query heads, direct
// facts, ground rules and the one/two-positive-tail compiler fragment.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

var ErrBoundIntervalDatalogFragment = errors.New("muz/fp: unsupported bound/interval product Datalog fragment")

type boundIntervalDatalogPredicate struct {
	declaration ast.ASTHandle
	kinds       []string
}

type boundIntervalDatalogEntry struct {
	sorts []ast.ASTHandle
	kinds []string
	seed  *rel.IntervalBoundProduct
	root  ast.ASTRef
}

type boundIntervalDatalogActivation struct {
	manager   *ast.ASTManager
	rules     *base.RuleSet
	entries   map[ast.ASTHandle]*boundIntervalDatalogEntry
	order     []ast.ASTHandle
	evaluated map[ast.ASTHandle]*rel.IntervalBoundProduct
	closed    bool
}

type boundIntervalDatalogConstructionFailure struct {
	original   error
	cleanup    error
	reported   error
	text       string
	activation *boundIntervalDatalogActivation
}

func (f *boundIntervalDatalogConstructionFailure) Error() string { return f.text }
func (f *boundIntervalDatalogConstructionFailure) Unwrap() error { return f.reported }
func (f *boundIntervalDatalogConstructionFailure) RuntimeOwner() *memory.Runtime {
	return f.activation.manager.NumeralService().RuntimeOwner()
}
func (f *boundIntervalDatalogConstructionFailure) OriginalCause() error { return f.original }
func (f *boundIntervalDatalogConstructionFailure) CleanupCause() error  { return f.cleanup }
func (f *boundIntervalDatalogConstructionFailure) Cleanup() error {
	f.cleanup = f.activation.close()
	return f.cleanup
}

func finishBoundIntervalDatalogConstruction(cause error, activation *boundIntervalDatalogActivation) error {
	if activation == nil {
		return cause
	}
	cleanup := activation.close()
	if cleanup == nil {
		return cause
	}
	original := cause
	if original == nil {
		original = cleanup
	}
	reported := errors.Join(cause, cleanup)
	return &boundIntervalDatalogConstructionFailure{original: original, cleanup: cleanup, reported: reported, text: reported.Error(), activation: activation}
}

func newBoundIntervalDatalogActivation(manager *ast.ASTManager, rules *base.RuleSet, predicates []boundIntervalDatalogPredicate) (_ *boundIntervalDatalogActivation, selected bool, resultErr error) {
	if manager == nil || rules == nil || len(predicates) == 0 {
		return nil, false, nil
	}
	a := &boundIntervalDatalogActivation{manager: manager, rules: rules, entries: make(map[ast.ASTHandle]*boundIntervalDatalogEntry, len(predicates))}
	defer func() {
		if resultErr != nil {
			resultErr = finishBoundIntervalDatalogConstruction(resultErr, a)
		}
	}()
	boolean, err := manager.MkBoolSort()
	if err != nil {
		return nil, false, err
	}
	for _, predicate := range predicates {
		if a.entries[predicate.declaration] != nil {
			return nil, true, ErrBoundIntervalDatalogFragment
		}
		rangeSort, cause := manager.GetRange(predicate.declaration)
		if cause != nil || rangeSort != boolean {
			return nil, true, errors.Join(cause, ErrBoundIntervalDatalogFragment)
		}
		arity, cause := manager.GetArity(predicate.declaration)
		if cause != nil {
			return nil, true, cause
		}
		sorts := make([]ast.ASTHandle, arity)
		for i := uint32(0); i < arity; i++ {
			sorts[i], cause = manager.GetDomain(predicate.declaration, i)
			if cause != nil {
				return nil, true, cause
			}
		}
		product, productSelected, cause := rel.NewSelectedIntervalBoundProduct(manager, sorts, predicate.kinds, true)
		if cause != nil {
			return nil, selected || productSelected, errors.Join(cause, ErrBoundIntervalDatalogFragment)
		}
		if !productSelected {
			if !selected {
				return nil, false, nil
			}
			return nil, true, ErrBoundIntervalDatalogFragment
		}
		selected = true
		root, cause := manager.NewRef(predicate.declaration)
		if cause != nil {
			return nil, true, cause
		}
		a.entries[predicate.declaration] = &boundIntervalDatalogEntry{sorts: sorts, kinds: product.Kinds(), seed: product, root: root}
		a.order = append(a.order, predicate.declaration)
	}
	return a, true, nil
}

func (a *boundIntervalDatalogActivation) open() error {
	if a == nil || a.closed || a.manager == nil || a.rules == nil || a.entries == nil {
		return ErrBoundIntervalDatalogFragment
	}
	return nil
}

func (a *boundIntervalDatalogActivation) checkpoint() error {
	canceled, err := a.manager.Canceled()
	if err != nil {
		return err
	}
	if canceled {
		return z3exception.NewDefaultException(a.manager.ResourceLimit().CancelMessage())
	}
	return nil
}

func (a *boundIntervalDatalogActivation) addFact(declaration ast.ASTHandle, values []ast.ASTHandle) error {
	if err := a.open(); err != nil {
		return err
	}
	entry := a.entries[declaration]
	if entry == nil || len(values) != len(entry.sorts) {
		return ErrBoundIntervalDatalogFragment
	}
	for i, value := range values {
		sort, err := a.manager.GetSort(value)
		if err != nil {
			return err
		}
		if sort != entry.sorts[i] {
			return ast.ErrASTDomain
		}
	}
	if err := entry.seed.AddFactExpressions(a.manager, entry.sorts, values); err != nil {
		return err
	}
	a.evaluated = nil
	return nil
}

var _ memory.ConstructionFailure = (*boundIntervalDatalogConstructionFailure)(nil)

func boundIntervalKindsEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

// syncPredicates adds inherited synthetic query heads without changing any
// existing seed. The complete list must still describe the same aligned
// product program; direct bound-only or interval-only declarations are never
// normalized into this activation.
func (a *boundIntervalDatalogActivation) syncPredicates(predicates []boundIntervalDatalogPredicate) error {
	if err := a.open(); err != nil {
		return err
	}
	seen := make(map[ast.ASTHandle]bool, len(predicates))
	additions := make([]boundIntervalDatalogPredicate, 0, len(predicates)-len(a.entries))
	for _, predicate := range predicates {
		if seen[predicate.declaration] {
			return ErrBoundIntervalDatalogFragment
		}
		seen[predicate.declaration] = true
		if entry := a.entries[predicate.declaration]; entry != nil {
			if !boundIntervalKindsEqual(entry.kinds, predicate.kinds) {
				return ErrBoundIntervalDatalogFragment
			}
			continue
		}
		additions = append(additions, predicate)
	}
	for declaration := range a.entries {
		if !seen[declaration] {
			return ErrBoundIntervalDatalogFragment
		}
	}
	if len(additions) == 0 {
		return nil
	}
	staged, selected, err := newBoundIntervalDatalogActivation(a.manager, a.rules, additions)
	if err != nil || !selected {
		return errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	for _, declaration := range staged.order {
		a.entries[declaration] = staged.entries[declaration]
		a.order = append(a.order, declaration)
	}
	staged.entries = nil
	staged.order = nil
	staged.closed = true
	a.evaluated = nil
	return nil
}

func (a *boundIntervalDatalogActivation) clone(rules *base.RuleSet) (_ *boundIntervalDatalogActivation, resultErr error) {
	if err := a.open(); err != nil || rules == nil {
		return nil, errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	predicates := make([]boundIntervalDatalogPredicate, 0, len(a.order))
	for _, declaration := range a.order {
		predicates = append(predicates, boundIntervalDatalogPredicate{declaration: declaration, kinds: a.entries[declaration].kinds})
	}
	copyActivation, selected, err := newBoundIntervalDatalogActivation(a.manager, rules, predicates)
	if err != nil || !selected {
		return nil, errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, copyActivation.close())
		}
	}()
	for declaration, entry := range a.entries {
		copyActivation.entries[declaration].seed, err = entry.seed.Clone()
		if err != nil {
			return nil, err
		}
	}
	return copyActivation, nil
}

func (a *boundIntervalDatalogActivation) applyFactRule(rule *base.Rule, products map[ast.ASTHandle]*rel.IntervalBoundProduct) (resultErr error) {
	defer func() { resultErr = errors.Join(resultErr, rule.Close()) }()
	tails, err := rule.TailSize()
	if err != nil || tails != 0 {
		return errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	declaration, err := rule.HeadDeclaration()
	if err != nil || products[declaration] == nil {
		return errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	formula, err := rule.ToFormula()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, a.manager.DelRef(&formula)) }()
	head, err := a.manager.RefGet(formula)
	if err != nil {
		return err
	}
	if kind, cause := a.manager.GetKind(head); cause != nil {
		return cause
	} else if kind == ast.ASTQuantifier {
		head, err = a.manager.GetQuantifierBody(head)
		if err != nil {
			return err
		}
	}
	actual, err := a.manager.GetDecl(head)
	if err != nil || actual != declaration {
		return errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	count, err := a.manager.GetNumArgs(head)
	if err != nil || int(count) != len(a.entries[declaration].sorts) {
		return errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	values := make([]ast.ASTHandle, count)
	for i := uint32(0); i < count; i++ {
		values[i], err = a.manager.GetArg(head, i)
		if err != nil {
			return err
		}
	}
	temporary := &boundIntervalDatalogActivation{manager: a.manager, rules: a.rules, entries: map[ast.ASTHandle]*boundIntervalDatalogEntry{declaration: &boundIntervalDatalogEntry{sorts: a.entries[declaration].sorts, seed: products[declaration]}}}
	return temporary.addFact(declaration, values)
}

func (a *boundIntervalDatalogActivation) query(declarations []ast.ASTHandle) (lbool.LBool, error) {
	return a.queryWithWidening(declarations, false, false)
}

func (a *boundIntervalDatalogActivation) queryWithWidening(declarations []ast.ASTHandle, widen, allOrNothing bool) (lbool.LBool, error) {
	if err := a.open(); err != nil {
		return lbool.Undef, err
	}
	// A new execution owns a new snapshot. If construction or lowering fails,
	// no answer from an older rule set remains observable.
	a.evaluated = nil
	if err := a.checkpoint(); err != nil {
		return lbool.Undef, err
	}
	evaluated := make(map[ast.ASTHandle]*rel.IntervalBoundProduct, len(a.entries))
	for declaration, entry := range a.entries {
		copyProduct, err := entry.seed.Clone()
		if err != nil {
			return lbool.Undef, err
		}
		evaluated[declaration] = copyProduct
	}
	count, err := a.rules.Len()
	if err != nil {
		return lbool.Undef, err
	}
	for i := 0; i < count; i++ {
		if err = a.checkpoint(); err != nil {
			return lbool.Undef, err
		}
		rule, cause := a.rules.Rule(i)
		if cause != nil {
			return lbool.Undef, cause
		}
		tails, cause := rule.TailSize()
		if cause != nil {
			_ = rule.Close()
			return lbool.Undef, cause
		}
		if tails == 0 {
			cause = a.applyFactRule(rule, evaluated)
		} else {
			cause = rule.Close()
		}
		if cause != nil {
			return lbool.Undef, cause
		}
	}
	if widen {
		if err = a.saturateWithWidening(evaluated, allOrNothing); err != nil {
			return lbool.Undef, err
		}
	} else {
		for {
			changed := false
			for i := 0; i < count; i++ {
				if err = a.checkpoint(); err != nil {
					return lbool.Undef, err
				}
				rule, cause := a.rules.Rule(i)
				if cause != nil {
					return lbool.Undef, cause
				}
				tails, cause := rule.TailSize()
				if cause != nil {
					_ = rule.Close()
					return lbool.Undef, cause
				}
				if tails == 0 {
					if cause = rule.Close(); cause != nil {
						return lbool.Undef, cause
					}
					continue
				}
				ruleChanged, cause := a.evaluatePositiveRule(rule, evaluated)
				if cause != nil {
					return lbool.Undef, cause
				}
				changed = changed || ruleChanged
			}
			if !changed {
				break
			}
		}
	}
	for _, declaration := range declarations {
		if evaluated[declaration] == nil {
			return lbool.Undef, fmt.Errorf("%w: undeclared query predicate", ErrBoundIntervalDatalogFragment)
		}
	}
	a.evaluated = evaluated
	if len(declarations) == 0 {
		return lbool.True, nil
	}
	for _, declaration := range declarations {
		if !evaluated[declaration].Empty() {
			return lbool.Undef, nil
		}
	}
	return lbool.False, nil
}

func (a *boundIntervalDatalogActivation) relationFormula(declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := a.open(); err != nil || a.evaluated == nil || a.evaluated[declaration] == nil {
		return ast.ASTRef{}, errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	// rel_context::query tests emptiness before asking the relation plugin
	// for its formula; empty vector components alone describe no constraints.
	if a.evaluated[declaration].Empty() {
		falsity, err := a.manager.MkFalse()
		if err != nil {
			return ast.ASTRef{}, err
		}
		return a.manager.NewRef(falsity)
	}
	return a.evaluated[declaration].ToFormula(a.manager, a.entries[declaration].sorts)
}

func (a *boundIntervalDatalogActivation) reset() error {
	if err := a.open(); err != nil {
		return err
	}
	for _, entry := range a.entries {
		product, selected, err := rel.NewSelectedIntervalBoundProduct(a.manager, entry.sorts, entry.kinds, true)
		if err != nil || !selected {
			return errors.Join(err, ErrBoundIntervalDatalogFragment)
		}
		entry.seed = product
	}
	a.evaluated = nil
	return nil
}

func (a *boundIntervalDatalogActivation) close() (result error) {
	if a == nil || a.closed {
		return nil
	}
	for i := len(a.order) - 1; i >= 0; i-- {
		entry := a.entries[a.order[i]]
		if entry != nil && entry.root != (ast.ASTRef{}) {
			result = errors.Join(result, a.manager.DelRef(&entry.root))
		}
	}
	if result == nil {
		a.closed = true
		a.entries = nil
		a.evaluated = nil
		a.order = nil
	}
	return result
}
