package fp

// Source: src/muz/rel/rel_context.cpp set_predicate_representation/add_fact
// and dl_product_relation.cpp product_relation::add_fact. Go creates finite
// tables eagerly, so a finite original selected into a heterogeneous product
// needs an explicit, ownership-preserving seed transfer.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/memory"
)

type sparseIntervalDecodedFact struct {
	manager *ast.ASTManager
	refs    []ast.ASTRef
}

func (o *sparseIntervalDecodedFact) close() (result error) {
	if o == nil || o.manager == nil {
		return nil
	}
	for i := len(o.refs) - 1; i >= 0; i-- {
		if o.refs[i] == (ast.ASTRef{}) {
			continue
		}
		if err := o.manager.DelRef(&o.refs[i]); err != nil {
			result = errors.Join(result, err)
		}
	}
	if result == nil {
		o.manager, o.refs = nil, nil
	}
	return result
}

type sparseIntervalDecodedFactCleanupFailure struct {
	cause, cleanupErr error
	owner             *sparseIntervalDecodedFact
}

func (f *sparseIntervalDecodedFactCleanupFailure) Error() string { return f.cause.Error() }
func (f *sparseIntervalDecodedFactCleanupFailure) Unwrap() error { return f.cause }
func (f *sparseIntervalDecodedFactCleanupFailure) Cleanup() error {
	f.cleanupErr = f.owner.close()
	return f.cleanupErr
}
func (f *sparseIntervalDecodedFactCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.manager.NumeralService().RuntimeOwner()
}
func (f *sparseIntervalDecodedFactCleanupFailure) OriginalCause() error { return f.cause }
func (f *sparseIntervalDecodedFactCleanupFailure) CleanupCause() error  { return f.cleanupErr }

func (o *sparseIntervalDecodedFact) finish(cause error) error {
	cleanup := o.close()
	if cleanup == nil {
		return cause
	}
	return &sparseIntervalDecodedFactCleanupFailure{
		cause:      errors.Join(cause, cleanup),
		cleanupErr: cleanup,
		owner:      o,
	}
}

// migrateSparseIntervalFiniteSeeds copies exact pre-activation finite rows
// into newly classified product owners. It neither replaces nor mutates the
// engine's enumerable finite table. Callers pass only newly classified
// originals so derived rows are never replayed as seeds on a later ensure.
func (e *PropositionalEngine) migrateSparseIntervalFiniteSeeds(activation *sparseIntervalDatalogActivation, declarations []ast.ASTHandle) error {
	if err := e.open(); err != nil {
		return err
	}
	if activation == nil || activation.open() != nil {
		return ErrSparseIntervalDatalogFragment
	}
	for _, declaration := range declarations {
		entry := activation.store.entries[declaration]
		if entry == nil {
			return ErrSparseIntervalDatalogFragment
		}
		if entry.class != sparseIntervalFiniteOriginal && entry.class != sparseIntervalHashOriginal {
			continue
		}
		if activation.migratedFinite[declaration] {
			continue
		}
		state := e.predicates[declaration]
		if state == nil || state.table == nil {
			return ErrSparseIntervalDatalogFragment
		}
		facts, err := state.table.Facts()
		if err != nil {
			return err
		}
		for _, fact := range facts {
			refs, cause := e.relations.DecodeFact(state.sorts, fact)
			if cause != nil {
				return cause
			}
			owner := &sparseIntervalDecodedFact{manager: e.manager, refs: refs}
			values := make([]ast.ASTHandle, len(refs))
			for i := range refs {
				values[i], cause = e.manager.RefGet(refs[i])
				if cause != nil {
					break
				}
			}
			if cause == nil {
				cause = activation.addFact(declaration, values)
			}
			if cause = owner.finish(cause); cause != nil {
				return cause
			}
		}
		activation.migratedFinite[declaration] = true
	}
	return nil
}

// migrateSparseIntervalArithmeticSeeds transfers a preexisting explicit
// interval owner into the heterogeneous product without enumerating an
// unbounded relation. Each product replacement clones the borrowed interval
// seed and preserves the full zero-column finite identity.
func (e *PropositionalEngine) migrateSparseIntervalArithmeticSeeds(activation *sparseIntervalDatalogActivation, source *intervalDatalogActivation, declarations []ast.ASTHandle) error {
	if err := e.open(); err != nil {
		return err
	}
	if activation == nil || activation.open() != nil || source == nil || source.open() != nil {
		return ErrSparseIntervalDatalogFragment
	}
	for _, declaration := range declarations {
		entry := activation.store.entries[declaration]
		if entry == nil {
			return ErrSparseIntervalDatalogFragment
		}
		if entry.class != sparseIntervalArithmeticOriginal || activation.migratedArithmetic[declaration] {
			continue
		}
		if source.store.entries[declaration] == nil {
			activation.migratedArithmetic[declaration] = true
			continue
		}
		seed, err := source.store.Relation(declaration)
		if err != nil {
			return err
		}
		replacement, err := activation.arithmeticSeedReplacement(declaration, seed)
		if err != nil {
			return err
		}
		attempt := &sparseIntervalDirectFactAttempt{engine: e, activation: activation, declaration: declaration, seed: replacement}
		attempt.priorSeed, err = activation.exchangeSeed(declaration, replacement)
		if err != nil {
			return attempt.finish(err)
		}
		attempt.seedExchanged = true
		attempt.seed = nil
		attempt.committed = true
		activation.migratedArithmetic[declaration] = true
		if err = attempt.finish(nil); err != nil {
			return err
		}
	}
	return nil
}

type sparseIntervalDirectFactAttempt struct {
	engine                                    *PropositionalEngine
	activation                                *sparseIntervalDatalogActivation
	declaration                               ast.ASTHandle
	table, priorTable                         rel.FiniteTable
	seed, priorSeed                           *rel.SparseIntervalSieveProduct
	publicExchanged, seedExchanged, committed bool
}

func (a *sparseIntervalDirectFactAttempt) cleanup() (result error) {
	if !a.committed {
		if a.seedExchanged {
			replacement, err := a.activation.exchangeSeed(a.declaration, a.priorSeed)
			if err != nil {
				return err
			}
			a.seed, a.priorSeed, a.seedExchanged = replacement, nil, false
		}
		if a.publicExchanged {
			replacement, err := a.engine.relations.ExchangeFiniteRelation(a.declaration, a.priorTable)
			if err != nil {
				return err
			}
			a.table, a.priorTable, a.publicExchanged = replacement, nil, false
		}
	}
	if a.committed {
		if a.priorSeed != nil {
			if err := a.priorSeed.Close(); err != nil {
				result = errors.Join(result, err)
			} else {
				a.priorSeed = nil
			}
		}
		if a.priorTable != nil {
			if err := a.priorTable.Close(); err != nil {
				result = errors.Join(result, err)
			} else {
				a.priorTable = nil
			}
		}
		return result
	}
	if a.seed != nil {
		if err := a.seed.Close(); err != nil {
			result = errors.Join(result, err)
		} else {
			a.seed = nil
		}
	}
	if a.table != nil {
		if err := a.table.Close(); err != nil {
			result = errors.Join(result, err)
		} else {
			a.table = nil
		}
	}
	return result
}

type sparseIntervalDirectFactCleanupFailure struct {
	cause, cleanupErr error
	attempt           *sparseIntervalDirectFactAttempt
}

func (f *sparseIntervalDirectFactCleanupFailure) Error() string { return f.cause.Error() }
func (f *sparseIntervalDirectFactCleanupFailure) Unwrap() error { return f.cause }
func (f *sparseIntervalDirectFactCleanupFailure) Cleanup() error {
	f.cleanupErr = f.attempt.cleanup()
	return f.cleanupErr
}
func (f *sparseIntervalDirectFactCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.attempt.engine.manager.NumeralService().RuntimeOwner()
}
func (f *sparseIntervalDirectFactCleanupFailure) OriginalCause() error { return f.cause }
func (f *sparseIntervalDirectFactCleanupFailure) CleanupCause() error  { return f.cleanupErr }
func (a *sparseIntervalDirectFactAttempt) finish(cause error) error {
	cleanup := a.cleanup()
	if cleanup == nil {
		return cause
	}
	return &sparseIntervalDirectFactCleanupFailure{cause: errors.Join(cause, cleanup), cleanupErr: cleanup, attempt: a}
}

// addSparseIntervalFact mirrors a finite-original fact into both exact public
// storage and the private product seed without exposing a one-sided update.
// Other product classes have no separate enumerable public owner.
func (e *PropositionalEngine) addSparseIntervalFact(declaration ast.ASTHandle, values []ast.ASTHandle) error {
	a := e.sparseIntervalDatalog
	if a == nil || a.open() != nil {
		return ErrSparseIntervalDatalogFragment
	}
	entry := a.store.entries[declaration]
	if entry == nil {
		return ErrSparseIntervalDatalogFragment
	}
	if entry.class != sparseIntervalFiniteOriginal && entry.class != sparseIntervalHashOriginal {
		return a.addFact(declaration, values)
	}
	state := e.predicates[declaration]
	if state == nil || state.table == nil {
		return ErrSparseIntervalDatalogFragment
	}
	attempt := &sparseIntervalDirectFactAttempt{engine: e, activation: a, declaration: declaration}
	var err error
	attempt.seed, err = a.cloneSeed(declaration)
	if err != nil {
		return err
	}
	if err = attempt.seed.AddFact(values); err != nil {
		return attempt.finish(err)
	}
	attempt.table, err = state.table.CloneFinite()
	if err != nil {
		return attempt.finish(err)
	}
	fact, err := e.relations.EncodeFact(state.sorts, values)
	if err != nil {
		return attempt.finish(err)
	}
	if _, err = attempt.table.Add(fact); err != nil {
		return attempt.finish(err)
	}
	attempt.priorTable, err = e.relations.ExchangeFiniteRelation(declaration, attempt.table)
	if err != nil {
		return attempt.finish(err)
	}
	attempt.publicExchanged = true
	attempt.priorSeed, err = a.exchangeSeed(declaration, attempt.seed)
	if err != nil {
		return attempt.finish(err)
	}
	attempt.seedExchanged = true
	state.table = attempt.table
	attempt.table, attempt.seed = nil, nil
	attempt.committed = true
	return attempt.finish(nil)
}

type sparseIntervalFinitePublication struct {
	declaration ast.ASTHandle
	table       rel.FiniteTable // staged replacement, then manager-owned after exchange
	prior       rel.FiniteTable
}

type sparseIntervalPublicationAttempt struct {
	engine    *PropositionalEngine
	staged    []sparseIntervalFinitePublication
	exchanged int
	committed bool
}

func (a *sparseIntervalPublicationAttempt) cleanup() (result error) {
	if !a.committed {
		for a.exchanged > 0 {
			i := a.exchanged - 1
			replacement, err := a.engine.relations.ExchangeFiniteRelation(a.staged[i].declaration, a.staged[i].prior)
			if err != nil {
				return errors.Join(result, err)
			}
			a.staged[i].table = replacement
			a.staged[i].prior = nil
			a.exchanged--
		}
	}
	for i := range a.staged {
		owner := a.staged[i].prior
		if !a.committed {
			owner = a.staged[i].table
		}
		if owner == nil {
			continue
		}
		if err := owner.Close(); err != nil {
			result = errors.Join(result, err)
			continue
		}
		if a.committed {
			a.staged[i].prior = nil
		} else {
			a.staged[i].table = nil
		}
	}
	return result
}

type sparseIntervalPublicationCleanupFailure struct {
	cause, cleanupErr error
	attempt           *sparseIntervalPublicationAttempt
}

func (f *sparseIntervalPublicationCleanupFailure) Error() string { return f.cause.Error() }
func (f *sparseIntervalPublicationCleanupFailure) Unwrap() error { return f.cause }
func (f *sparseIntervalPublicationCleanupFailure) Cleanup() error {
	f.cleanupErr = f.attempt.cleanup()
	return f.cleanupErr
}
func (f *sparseIntervalPublicationCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.attempt.engine.manager.NumeralService().RuntimeOwner()
}
func (f *sparseIntervalPublicationCleanupFailure) OriginalCause() error { return f.cause }
func (f *sparseIntervalPublicationCleanupFailure) CleanupCause() error  { return f.cleanupErr }

func (a *sparseIntervalPublicationAttempt) finish(cause error) error {
	cleanup := a.cleanup()
	if cleanup == nil {
		return cause
	}
	return &sparseIntervalPublicationCleanupFailure{cause: errors.Join(cause, cleanup), cleanupErr: cleanup, attempt: a}
}

// publishSparseIntervalFiniteRelations installs exact finite-original results
// from source's most recent evaluator. Mixed and arithmetic products remain
// abstract and are deliberately not exposed as enumerable finite tables.
func (e *PropositionalEngine) publishSparseIntervalFiniteRelations(source *PropositionalEngine) error {
	if err := e.open(); err != nil {
		return err
	}
	if source == nil || source.sparseIntervalDatalog == nil {
		return ErrSparseIntervalDatalogFragment
	}
	attempt := &sparseIntervalPublicationAttempt{engine: e}
	for _, declaration := range e.predicateOrder {
		if e.auxiliary[declaration] {
			continue
		}
		table, exact, err := source.sparseIntervalDatalog.finiteSnapshot(declaration)
		if err != nil {
			return attempt.finish(err)
		}
		if exact {
			state := e.predicates[declaration]
			if state == nil || state.table == nil {
				attempt.staged = append(attempt.staged, sparseIntervalFinitePublication{table: table})
				return attempt.finish(ErrSparseIntervalDatalogFragment)
			}
			attempt.staged = append(attempt.staged, sparseIntervalFinitePublication{declaration: declaration, table: table})
		}
	}
	for i := range attempt.staged {
		prior, err := e.relations.ExchangeFiniteRelation(attempt.staged[i].declaration, attempt.staged[i].table)
		if err == nil {
			attempt.staged[i].prior = prior
			attempt.exchanged++
			continue
		}
		return attempt.finish(err)
	}
	// All exchanges succeeded. Only now publish state pointers and retire the
	// former exact owners; before this point every failure was reversible.
	for i := range attempt.staged {
		e.predicates[attempt.staged[i].declaration].table = attempt.staged[i].table
		attempt.staged[i].table = nil
	}
	attempt.committed = true
	return attempt.finish(nil)
}

var _ memory.ConstructionFailure = (*sparseIntervalDecodedFactCleanupFailure)(nil)
var _ memory.ConstructionFailure = (*sparseIntervalPublicationCleanupFailure)(nil)
var _ memory.ConstructionFailure = (*sparseIntervalDirectFactCleanupFailure)(nil)
