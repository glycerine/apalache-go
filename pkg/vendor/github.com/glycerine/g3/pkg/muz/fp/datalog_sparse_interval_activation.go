package fp

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// sparseIntervalDatalogActivation owns seed storage and replaces its
// evaluator snapshot for each query. Engine selection remains in the caller.
type sparseIntervalDatalogActivation struct {
	manager            *ast.ASTManager
	rules              *base.RuleSet
	guard              *astrewriter.TheoryRewriter // borrowed
	store              *SparseIntervalDatalogStore
	evaluator          *SparseIntervalDatalogEvaluator
	predicates         []ast.ASTHandle
	migratedFinite     map[ast.ASTHandle]bool
	migratedArithmetic map[ast.ASTHandle]bool
	closed             bool
}

func newSparseIntervalDatalogActivation(manager *ast.ASTManager, rules *base.RuleSet, guard *astrewriter.TheoryRewriter, declarations []ast.ASTHandle) (_ *sparseIntervalDatalogActivation, selected bool, resultErr error) {
	originals := make([]sparseIntervalOriginal, len(declarations))
	for i, declaration := range declarations {
		originals[i] = sparseIntervalOriginal{declaration: declaration, class: sparseIntervalMixedOriginal}
	}
	return newClassifiedSparseIntervalDatalogActivation(manager, rules, guard, originals)
}

func newClassifiedSparseIntervalDatalogActivation(manager *ast.ASTManager, rules *base.RuleSet, guard *astrewriter.TheoryRewriter, originals []sparseIntervalOriginal) (_ *sparseIntervalDatalogActivation, selected bool, resultErr error) {
	if manager == nil || rules == nil || guard == nil || len(originals) == 0 {
		return nil, false, nil
	}
	a := &sparseIntervalDatalogActivation{manager: manager, rules: rules, guard: guard, predicates: make([]ast.ASTHandle, len(originals)), migratedFinite: make(map[ast.ASTHandle]bool), migratedArithmetic: make(map[ast.ASTHandle]bool)}
	for i := range originals {
		a.predicates[i] = originals[i].declaration
	}
	defer func() {
		if resultErr != nil {
			resultErr = finishSparseIntervalActivationConstruction(resultErr, a)
		}
	}()
	var err error
	a.store, err = NewSparseIntervalDatalogStore(manager)
	if err != nil {
		return nil, true, err
	}
	seen := make(map[ast.ASTHandle]bool, len(originals))
	for _, original := range originals {
		declaration := original.declaration
		if seen[declaration] {
			return nil, true, ErrSparseIntervalDatalogFragment
		}
		seen[declaration] = true
		if err = a.store.RegisterOriginal(declaration, original.class); err != nil {
			if errors.Is(err, rel.ErrSparseIntervalProduct) || errors.Is(err, ErrSparseIntervalDatalogFragment) {
				cleanup := finishSparseIntervalActivationConstruction(nil, a)
				a = nil
				return nil, false, cleanup
			}
			return nil, true, err
		}
	}
	return a, true, nil
}

func (a *sparseIntervalDatalogActivation) open() error {
	if a == nil || a.closed || a.manager == nil || a.rules == nil || a.guard == nil || a.store == nil {
		return ErrSparseIntervalDatalogFragment
	}
	return nil
}

func (a *sparseIntervalDatalogActivation) discardEvaluator() error {
	if a.evaluator == nil {
		return nil
	}
	err := a.evaluator.Close()
	if err == nil {
		a.evaluator = nil
	}
	return err
}

func (a *sparseIntervalDatalogActivation) addFact(declaration ast.ASTHandle, values []ast.ASTHandle) error {
	if err := a.open(); err != nil {
		return err
	}
	if err := a.discardEvaluator(); err != nil {
		return err
	}
	return a.store.AddFact(declaration, values)
}

// cloneSeed returns an owned copy for failure-atomic fact insertion.
func (a *sparseIntervalDatalogActivation) cloneSeed(declaration ast.ASTHandle) (*rel.SparseIntervalSieveProduct, error) {
	if err := a.open(); err != nil {
		return nil, err
	}
	entry := a.store.entries[declaration]
	if entry == nil {
		return nil, ErrSparseIntervalDatalogFragment
	}
	return entry.relation.Clone()
}

// arithmeticSeedReplacement constructs an owned arithmetic-original product
// from a borrowed interval snapshot without enumerating it.
func (a *sparseIntervalDatalogActivation) arithmeticSeedReplacement(declaration ast.ASTHandle, seed *rel.IntervalRelation) (*rel.SparseIntervalSieveProduct, error) {
	if err := a.open(); err != nil {
		return nil, err
	}
	entry := a.store.entries[declaration]
	if entry == nil || entry.class != sparseIntervalArithmeticOriginal {
		return nil, ErrSparseIntervalDatalogFragment
	}
	return rel.NewSparseIntervalArithmeticOriginalFrom(a.store.relations, entry.sorts, seed)
}

// exchangeSeed transfers replacement into the store and the prior seed to the
// caller. It performs no closes, and on failure replacement remains owned by
// the caller and store state is unchanged.
func (a *sparseIntervalDatalogActivation) exchangeSeed(declaration ast.ASTHandle, replacement *rel.SparseIntervalSieveProduct) (*rel.SparseIntervalSieveProduct, error) {
	if err := a.open(); err != nil {
		return nil, err
	}
	entry := a.store.entries[declaration]
	if entry == nil || replacement == nil {
		return nil, ErrSparseIntervalDatalogFragment
	}
	if !replacement.OwnedBy(a.store.relations) {
		return nil, ErrSparseIntervalDatalogFragment
	}
	if err := replacement.MatchesSorts(entry.sorts); err != nil {
		return nil, errors.Join(err, ErrSparseIntervalDatalogFragment)
	}
	if err := a.discardEvaluator(); err != nil {
		return nil, err
	}
	prior := entry.relation
	entry.relation = replacement
	return prior, nil
}

func (a *sparseIntervalDatalogActivation) syncPredicates(declarations []ast.ASTHandle) error {
	originals := make([]sparseIntervalOriginal, len(declarations))
	for i, declaration := range declarations {
		originals[i] = sparseIntervalOriginal{declaration: declaration, class: sparseIntervalMixedOriginal}
	}
	return a.syncOriginals(originals)
}

// syncOriginals transactionally extends original predicates while preserving
// each declaration's sparse, interval, or mixed concrete representation.
func (a *sparseIntervalDatalogActivation) syncOriginals(originals []sparseIntervalOriginal) error {
	if err := a.open(); err != nil {
		return err
	}
	current := make(map[ast.ASTHandle]sparseIntervalPredicateClass, len(originals))
	for _, original := range originals {
		if original.declaration.IsNull() || current[original.declaration] != 0 || original.class == sparseIntervalGeneratedAuxiliary {
			return ErrSparseIntervalDatalogFragment
		}
		current[original.declaration] = original.class
	}
	for declaration, entry := range a.store.entries {
		if !entry.auxiliary && current[declaration] != entry.class {
			return ErrSparseIntervalDatalogFragment
		}
	}
	type stagedOriginal struct {
		declaration ast.ASTHandle
		entry       *sparseIntervalDatalogStoreEntry
	}
	staged := make([]stagedOriginal, 0, len(originals))
	for _, original := range originals {
		if a.store.entries[original.declaration] != nil {
			continue
		}
		entry, err := a.store.prepareRegistration(original.declaration, original.class)
		if err != nil {
			owners := make([]*sparseIntervalDatalogStoreEntry, len(staged))
			for i := range staged {
				owners[i] = staged[i].entry
			}
			return finishSparseIntervalEntriesConstruction(errors.Join(err, ErrSparseIntervalDatalogFragment), a.manager, owners...)
		}
		staged = append(staged, stagedOriginal{declaration: original.declaration, entry: entry})
	}
	if err := a.discardEvaluator(); err != nil {
		owners := make([]*sparseIntervalDatalogStoreEntry, len(staged))
		for i := range staged {
			owners[i] = staged[i].entry
		}
		return finishSparseIntervalEntriesConstruction(err, a.manager, owners...)
	}
	for _, original := range staged {
		a.store.entries[original.declaration] = original.entry
		a.store.order = append(a.store.order, original.declaration)
		a.predicates = append(a.predicates, original.declaration)
	}
	return nil
}

// syncAuxiliaryPredicates admits generated pure finite, pure arithmetic, and
// nullary predicates without widening original-program backend selection.
func (a *sparseIntervalDatalogActivation) syncAuxiliaryPredicates(declarations []ast.ASTHandle) error {
	return a.sync(declarations, true)
}

func (a *sparseIntervalDatalogActivation) sync(declarations []ast.ASTHandle, auxiliary bool) error {
	if err := a.open(); err != nil {
		return err
	}
	current := make(map[ast.ASTHandle]bool, len(declarations))
	for _, declaration := range declarations {
		if current[declaration] {
			return ErrSparseIntervalDatalogFragment
		}
		current[declaration] = true
	}
	for _, declaration := range a.predicates {
		if !current[declaration] {
			return ErrSparseIntervalDatalogFragment
		}
	}
	type stagedRegistration struct {
		declaration ast.ASTHandle
		entry       *sparseIntervalDatalogStoreEntry
	}
	staged := make([]stagedRegistration, 0, len(declarations)-len(a.predicates))
	class := sparseIntervalMixedOriginal
	if auxiliary {
		class = sparseIntervalGeneratedAuxiliary
	}
	for _, declaration := range declarations {
		if a.store.entries[declaration] != nil {
			continue
		}
		entry, err := a.store.prepareRegistration(declaration, class)
		if err != nil {
			owners := make([]*sparseIntervalDatalogStoreEntry, len(staged))
			for i := range staged {
				owners[i] = staged[i].entry
			}
			return finishSparseIntervalEntriesConstruction(errors.Join(err, ErrSparseIntervalDatalogFragment), a.manager, owners...)
		}
		staged = append(staged, stagedRegistration{declaration: declaration, entry: entry})
	}
	if err := a.discardEvaluator(); err != nil {
		owners := make([]*sparseIntervalDatalogStoreEntry, len(staged))
		for i := range staged {
			owners[i] = staged[i].entry
		}
		return finishSparseIntervalEntriesConstruction(err, a.manager, owners...)
	}
	for _, registration := range staged {
		a.store.entries[registration.declaration] = registration.entry
		a.store.order = append(a.store.order, registration.declaration)
		a.predicates = append(a.predicates, registration.declaration)
	}
	return nil
}

func (a *sparseIntervalDatalogActivation) clone(rules *base.RuleSet, guard *astrewriter.TheoryRewriter) (_ *sparseIntervalDatalogActivation, resultErr error) {
	if err := a.open(); err != nil || rules == nil || guard == nil {
		if err == nil {
			err = ErrSparseIntervalDatalogFragment
		}
		return nil, err
	}
	copyActivation := &sparseIntervalDatalogActivation{manager: a.manager, rules: rules, guard: guard, predicates: append([]ast.ASTHandle(nil), a.predicates...), migratedFinite: make(map[ast.ASTHandle]bool, len(a.migratedFinite)), migratedArithmetic: make(map[ast.ASTHandle]bool, len(a.migratedArithmetic))}
	for declaration, migrated := range a.migratedFinite {
		copyActivation.migratedFinite[declaration] = migrated
	}
	for declaration, migrated := range a.migratedArithmetic {
		copyActivation.migratedArithmetic[declaration] = migrated
	}
	defer func() {
		if resultErr != nil {
			resultErr = finishSparseIntervalActivationConstruction(resultErr, copyActivation)
		}
	}()
	var err error
	copyActivation.store, err = NewSparseIntervalDatalogStore(a.manager)
	if err != nil {
		return nil, err
	}
	for _, declaration := range a.store.order {
		source := a.store.entries[declaration]
		// Preserve the concrete auxiliary precision class as well as the
		// original representation class. RegisterAuxiliary alone would turn a
		// precise finite query head back into an approximate generated product.
		err = copyActivation.store.register(declaration, source.class)
		if err != nil {
			return nil, err
		}
		clone, cause := source.relation.CloneTo(copyActivation.store.relations)
		if cause != nil {
			return nil, cause
		}
		destination := copyActivation.store.entries[declaration]
		if cause = destination.relation.Close(); cause != nil {
			return nil, finishSparseIntervalProductConstruction(cause, a.manager, clone)
		}
		destination.relation = clone
	}
	return copyActivation, nil
}

func (a *sparseIntervalDatalogActivation) query(declarations []ast.ASTHandle, widen bool) (lbool.LBool, error) {
	if err := a.open(); err != nil {
		return lbool.Undef, err
	}
	if err := a.discardEvaluator(); err != nil {
		return lbool.Undef, err
	}
	evaluator, err := a.store.Evaluator(a.rules, a.guard)
	if err != nil {
		return lbool.Undef, err
	}
	if err = evaluator.SaturatePositive(widen); err != nil {
		return lbool.Undef, finishSparseIntervalEvaluatorConstruction(err, evaluator)
	}
	a.evaluator = evaluator
	return evaluator.Query(declarations)
}

func (a *sparseIntervalDatalogActivation) relation(declaration ast.ASTHandle) (*rel.SparseIntervalSieveProduct, error) {
	if err := a.open(); err != nil || a.evaluator == nil {
		if err == nil {
			err = ErrSparseIntervalDatalogFragment
		}
		return nil, err
	}
	return a.evaluator.Relation(declaration)
}

func (a *sparseIntervalDatalogActivation) relationFormula(declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := a.open(); err != nil || a.evaluator == nil {
		if err == nil {
			err = ErrSparseIntervalDatalogFragment
		}
		return ast.ASTRef{}, err
	}
	return a.evaluator.RelationFormula(declaration)
}

// finiteSnapshot publishes an owned exact table only for a sparse original.
// Mixed products and arithmetic relations remain non-enumerable.
func (a *sparseIntervalDatalogActivation) finiteSnapshot(declaration ast.ASTHandle) (rel.FiniteTable, bool, error) {
	if err := a.open(); err != nil || a.evaluator == nil {
		if err == nil {
			err = ErrSparseIntervalDatalogFragment
		}
		return nil, false, err
	}
	entry := a.store.entries[declaration]
	if entry == nil || (entry.class != sparseIntervalFiniteOriginal && entry.class != sparseIntervalHashOriginal) {
		return nil, false, nil
	}
	product := a.evaluator.products[declaration]
	if product == nil {
		return nil, false, ErrSparseIntervalDatalogFragment
	}
	var table rel.FiniteTable
	var err error
	if entry.class == sparseIntervalHashOriginal {
		table, err = product.HashSnapshot()
	} else {
		table, err = product.FiniteSnapshot()
	}
	return table, err == nil, err
}

func (a *sparseIntervalDatalogActivation) reset() error {
	if err := a.open(); err != nil {
		return err
	}
	if err := a.discardEvaluator(); err != nil {
		return err
	}
	return a.store.Reset()
}

func (a *sparseIntervalDatalogActivation) close() (result error) {
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
	a.manager, a.rules, a.guard, a.store, a.predicates, a.migratedFinite, a.migratedArithmetic = nil, nil, nil, nil, nil, nil, nil
	return nil
}
