package fp

// Source: src/muz/rel/dl_interval_relation.cpp interval_relation_plugin and
// interval_relation::add_fact. This is the typed storage boundary needed by a
// future Datalog-engine hook; it deliberately does not alter engine dispatch.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

type intervalDatalogStoreEntry struct {
	sorts    []ast.ASTHandle
	relation *rel.IntervalRelation
	root     ast.ASTRef
}

// IntervalDatalogStore owns arithmetic predicate declarations and their
// abstract interval facts. Evaluators created from it receive seed snapshots.
type IntervalDatalogStore struct {
	manager *ast.ASTManager
	entries map[ast.ASTHandle]*intervalDatalogStoreEntry
	order   []ast.ASTHandle
	closed  bool
}

func NewIntervalDatalogStore(manager *ast.ASTManager) (*IntervalDatalogStore, error) {
	if manager == nil {
		return nil, ErrIntervalDatalogFragment
	}
	return &IntervalDatalogStore{manager: manager, entries: make(map[ast.ASTHandle]*intervalDatalogStoreEntry)}, nil
}

func (s *IntervalDatalogStore) open() error {
	if s == nil || s.closed || s.manager == nil || s.entries == nil {
		return ErrIntervalDatalogFragment
	}
	return nil
}

// Register installs one Boolean-valued predicate whose columns are all Int or
// Real, matching interval_relation_plugin::can_handle_signature.
func (s *IntervalDatalogStore) Register(declaration ast.ASTHandle) error {
	if err := s.open(); err != nil {
		return err
	}
	if _, found := s.entries[declaration]; found {
		return fmt.Errorf("%w: duplicate predicate", ErrIntervalDatalogFragment)
	}
	rangeSort, err := s.manager.GetRange(declaration)
	if err != nil {
		return err
	}
	boolean, err := s.manager.MkBoolSort()
	if err != nil {
		return err
	}
	if rangeSort != boolean {
		return fmt.Errorf("%w: predicate range is not Boolean", ErrIntervalDatalogFragment)
	}
	arity, err := s.manager.GetArity(declaration)
	if err != nil {
		return err
	}
	sorts := make([]ast.ASTHandle, arity)
	for i := uint32(0); i < arity; i++ {
		sorts[i], err = s.manager.GetDomain(declaration, i)
		if err != nil {
			return err
		}
	}
	if !rel.CanHandleIntervalSorts(s.manager, sorts) {
		return fmt.Errorf("%w: predicate signature is not entirely arithmetic", ErrIntervalDatalogFragment)
	}
	root, err := s.manager.NewRef(declaration)
	if err != nil {
		return err
	}
	s.entries[declaration] = &intervalDatalogStoreEntry{sorts: sorts, relation: rel.NewIntervalEmpty(arity), root: root}
	s.order = append(s.order, declaration)
	return nil
}

func (s *IntervalDatalogStore) AddFact(declaration ast.ASTHandle, values []ast.ASTHandle) error {
	if err := s.open(); err != nil {
		return err
	}
	entry := s.entries[declaration]
	if entry == nil {
		return fmt.Errorf("%w: unregistered predicate", ErrIntervalDatalogFragment)
	}
	return entry.relation.AddFactExpressions(s.manager, entry.sorts, values)
}

func (s *IntervalDatalogStore) Relation(declaration ast.ASTHandle) (*rel.IntervalRelation, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	entry := s.entries[declaration]
	if entry == nil {
		return nil, fmt.Errorf("%w: unregistered predicate", ErrIntervalDatalogFragment)
	}
	return entry.relation.Clone()
}

// Evaluator snapshots store facts so later fact insertion cannot mutate an
// already-running evaluation.
func (s *IntervalDatalogStore) Evaluator(rules *base.RuleSet) (*IntervalDatalogEvaluator, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	seeds := make(map[ast.ASTHandle]*rel.IntervalRelation, len(s.entries))
	for declaration, entry := range s.entries {
		seeds[declaration] = entry.relation
	}
	return NewIntervalDatalogEvaluator(s.manager, rules, append([]ast.ASTHandle(nil), s.order...), seeds)
}

func (s *IntervalDatalogStore) Reset() error {
	if err := s.open(); err != nil {
		return err
	}
	for _, entry := range s.entries {
		entry.relation = rel.NewIntervalEmpty(uint32(len(entry.sorts)))
	}
	return nil
}

func (s *IntervalDatalogStore) Close() (result error) {
	if s == nil || s.closed {
		return nil
	}
	for i := len(s.order) - 1; i >= 0; i-- {
		entry := s.entries[s.order[i]]
		if entry != nil && entry.root != (ast.ASTRef{}) {
			result = errors.Join(result, s.manager.DelRef(&entry.root))
		}
	}
	if result != nil {
		return result
	}
	s.closed = true
	s.manager, s.entries, s.order = nil, nil, nil
	return nil
}
