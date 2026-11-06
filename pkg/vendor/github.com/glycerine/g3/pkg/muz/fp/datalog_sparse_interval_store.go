package fp

// Source: src/muz/rel/rel_context.cpp multi-plugin representation selection
// and src/muz/rel/dl_product_relation.cpp independent product components.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

var ErrSparseIntervalDatalogFragment = errors.New("muz/fp: unsupported sparse/interval Datalog fragment")

type sparseIntervalPredicateClass uint8

const (
	sparseIntervalMixedOriginal sparseIntervalPredicateClass = iota + 1
	sparseIntervalFiniteOriginal
	sparseIntervalHashOriginal
	sparseIntervalArithmeticOriginal
	sparseIntervalGeneratedAuxiliary
)

type sparseIntervalOriginal struct {
	declaration ast.ASTHandle
	class       sparseIntervalPredicateClass
}

type sparseIntervalDatalogStoreEntry struct {
	sorts     []ast.ASTHandle
	relation  *rel.SparseIntervalSieveProduct
	root      ast.ASTRef
	auxiliary bool
	class     sparseIntervalPredicateClass
}

// Evaluator snapshots seed products; subsequent fact insertion cannot mutate
// the running fixed point.
func (s *SparseIntervalDatalogStore) Evaluator(rules *base.RuleSet, guard *astrewriter.TheoryRewriter) (*SparseIntervalDatalogEvaluator, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	seeds := make(map[ast.ASTHandle]*rel.SparseIntervalSieveProduct, len(s.entries))
	for declaration, entry := range s.entries {
		seeds[declaration] = entry.relation
	}
	return NewSparseIntervalDatalogEvaluator(s.manager, s.relations, rules, guard, append([]ast.ASTHandle(nil), s.order...), seeds)
}

// SparseIntervalDatalogStore owns mixed finite/arithmetic seed relations. The
// relation manager is private because these products are deliberately not
// FiniteTable instances and must not enter the finite compiler registers.
type SparseIntervalDatalogStore struct {
	manager   *ast.ASTManager
	relations *rel.RelationManager
	entries   map[ast.ASTHandle]*sparseIntervalDatalogStoreEntry
	order     []ast.ASTHandle
	closed    bool
}

func NewSparseIntervalDatalogStore(manager *ast.ASTManager) (*SparseIntervalDatalogStore, error) {
	if manager == nil {
		return nil, ErrSparseIntervalDatalogFragment
	}
	relations, err := rel.NewRelationManager(manager)
	if err != nil {
		return nil, err
	}
	return &SparseIntervalDatalogStore{manager: manager, relations: relations, entries: make(map[ast.ASTHandle]*sparseIntervalDatalogStoreEntry)}, nil
}

func (s *SparseIntervalDatalogStore) open() error {
	if s == nil || s.closed || s.manager == nil || s.relations == nil || s.entries == nil {
		return ErrSparseIntervalDatalogFragment
	}
	return nil
}

func (s *SparseIntervalDatalogStore) Register(declaration ast.ASTHandle) error {
	return s.RegisterOriginal(declaration, sparseIntervalMixedOriginal)
}

func (s *SparseIntervalDatalogStore) RegisterOriginal(declaration ast.ASTHandle, class sparseIntervalPredicateClass) error {
	if class != sparseIntervalMixedOriginal && class != sparseIntervalFiniteOriginal && class != sparseIntervalHashOriginal && class != sparseIntervalArithmeticOriginal {
		return ErrSparseIntervalDatalogFragment
	}
	return s.register(declaration, class)
}

// RegisterAuxiliary admits generated pure finite, pure arithmetic, and
// nullary predicates after a mixed program has selected this backend.
func (s *SparseIntervalDatalogStore) RegisterAuxiliary(declaration ast.ASTHandle) error {
	return s.register(declaration, sparseIntervalGeneratedAuxiliary)
}

func (s *SparseIntervalDatalogStore) register(declaration ast.ASTHandle, class sparseIntervalPredicateClass) (resultErr error) {
	if err := s.open(); err != nil {
		return err
	}
	if s.entries[declaration] != nil {
		return fmt.Errorf("%w: duplicate predicate", ErrSparseIntervalDatalogFragment)
	}
	entry, err := s.prepareRegistration(declaration, class)
	if err != nil {
		return err
	}
	s.entries[declaration] = entry
	s.order = append(s.order, declaration)
	return nil
}

// prepareRegistration validates and owns a complete entry without publishing
// it. Activation sync stages every new declaration before changing the store.
func (s *SparseIntervalDatalogStore) prepareRegistration(declaration ast.ASTHandle, class sparseIntervalPredicateClass) (*sparseIntervalDatalogStoreEntry, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	rangeSort, err := s.manager.GetRange(declaration)
	if err != nil {
		return nil, err
	}
	boolean, err := s.manager.MkBoolSort()
	if err != nil {
		return nil, err
	}
	if rangeSort != boolean {
		return nil, fmt.Errorf("%w: predicate range is not Boolean", ErrSparseIntervalDatalogFragment)
	}
	arity, err := s.manager.GetArity(declaration)
	if err != nil {
		return nil, err
	}
	sorts := make([]ast.ASTHandle, arity)
	for i := uint32(0); i < arity; i++ {
		sorts[i], err = s.manager.GetDomain(declaration, i)
		if err != nil {
			return nil, err
		}
	}
	var product *rel.SparseIntervalSieveProduct
	switch class {
	case sparseIntervalGeneratedAuxiliary:
		product, err = rel.NewSparseIntervalSieveProductAuxiliary(s.relations, sorts, false)
	case sparseIntervalMixedOriginal:
		product, err = rel.NewSparseIntervalSieveProduct(s.relations, sorts, []string{"tr_sparse", "interval_relation"}, false)
	case sparseIntervalFiniteOriginal:
		product, err = rel.NewSparseIntervalFiniteOriginal(s.relations, sorts, false)
	case sparseIntervalHashOriginal:
		product, err = rel.NewSparseIntervalHashOriginal(s.relations, sorts, false)
	case sparseIntervalArithmeticOriginal:
		product, err = rel.NewSparseIntervalArithmeticOriginal(s.relations, sorts, false)
	default:
		return nil, ErrSparseIntervalDatalogFragment
	}
	if err != nil {
		return nil, errors.Join(ErrSparseIntervalDatalogFragment, err)
	}
	if class == sparseIntervalGeneratedAuxiliary {
		err = product.MarkGeneratedRawShape()
	}
	if err != nil {
		return nil, errors.Join(ErrSparseIntervalDatalogFragment, err, product.Close())
	}
	declarationRef, err := s.manager.NewRef(declaration)
	if err != nil {
		return nil, finishSparseIntervalProductConstruction(err, s.manager, product)
	}
	auxiliary := class == sparseIntervalGeneratedAuxiliary
	return &sparseIntervalDatalogStoreEntry{sorts: sorts, relation: product, root: declarationRef, auxiliary: auxiliary, class: class}, nil
}

func (s *SparseIntervalDatalogStore) AddFact(declaration ast.ASTHandle, values []ast.ASTHandle) error {
	if err := s.open(); err != nil {
		return err
	}
	entry := s.entries[declaration]
	if entry == nil {
		return fmt.Errorf("%w: unregistered predicate", ErrSparseIntervalDatalogFragment)
	}
	return entry.relation.AddFact(values)
}

func (s *SparseIntervalDatalogStore) Reset() (result error) {
	if err := s.open(); err != nil {
		return err
	}
	for _, entry := range s.entries {
		result = errors.Join(result, entry.relation.Reset())
	}
	return result
}

func (s *SparseIntervalDatalogStore) Close() (result error) {
	if s == nil || s.closed {
		return nil
	}
	for i := len(s.order) - 1; i >= 0; i-- {
		entry := s.entries[s.order[i]]
		if entry == nil {
			continue
		}
		if entry.relation != nil {
			if err := entry.relation.Close(); err != nil {
				result = errors.Join(result, err)
				continue
			}
			entry.relation = nil
		}
		if entry.root != (ast.ASTRef{}) {
			result = errors.Join(result, s.manager.DelRef(&entry.root))
		}
	}
	if result != nil {
		return result
	}
	if err := s.relations.Close(); err != nil {
		return err
	}
	s.closed = true
	s.manager, s.relations, s.entries, s.order = nil, nil, nil, nil
	return nil
}
