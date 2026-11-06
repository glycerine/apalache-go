package rel

// Source: src/muz/rel/dl_check_table.cpp query-reached check_table operations.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
)

// FiniteCheckedPair owns independently instantiated concrete primary and
// checker tables and compares their observable results after every operation.
type FiniteCheckedPair struct {
	primary FiniteTable
	checker FiniteTable
	closed  bool
}

// FiniteCheckedFactory is the concrete check_table plugin selection used for
// compiler temporaries during one configured checked execution.
type FiniteCheckedFactory struct {
	primaryKind string
	checkerKind string
	lazy        *LazySparseFiniteFactory
}

func NewFiniteCheckedFactory(primaryKind, checkerKind string) (*FiniteCheckedFactory, error) {
	return NewFiniteCheckedFactoryWithLazy(primaryKind, checkerKind, nil)
}

// NewFiniteCheckedFactoryWithLazy binds the context-owned lazy_sparse plugin
// used by either concrete side. The factory borrows lazy and never closes it.
func NewFiniteCheckedFactoryWithLazy(primaryKind, checkerKind string, lazy *LazySparseFiniteFactory) (*FiniteCheckedFactory, error) {
	valid := func(kind string) bool {
		switch kind {
		case "hashtable", "tr_hashtable", "sparse", "tr_sparse":
			return true
		case "lazy_sparse", "tr_lazy_sparse":
			return lazy != nil && lazy.open() == nil
		default:
			return false
		}
	}
	if !valid(primaryKind) || !valid(checkerKind) {
		return nil, ErrCheckTable
	}
	return &FiniteCheckedFactory{primaryKind: primaryKind, checkerKind: checkerKind, lazy: lazy}, nil
}

func checkedKindHandles(kind string, signature Signature) bool {
	switch kind {
	case "hashtable", "tr_hashtable":
		return true
	case "sparse", "tr_sparse", "lazy_sparse", "tr_lazy_sparse":
		return signature.Len() != 0
	default:
		return false
	}
}

// CanHandleSignature mirrors check_table_plugin::can_handle_signature: both
// selected concrete plugins must accept the signature.
func (f *FiniteCheckedFactory) CanHandleSignature(signature Signature) bool {
	return f != nil && checkedKindHandles(f.primaryKind, signature) && checkedKindHandles(f.checkerKind, signature)
}

func checkedKindHandlesFunctional(kind string, signature Signature, functional int) bool {
	if functional == 0 {
		return checkedKindHandles(kind, signature)
	}
	switch kind {
	case "sparse", "tr_sparse", "lazy_sparse", "tr_lazy_sparse":
		return signature.Len() != 0 && functional > 0 && functional <= signature.Len()
	default:
		return false
	}
}

// CanHandleExplanationIndex mirrors the functional-column capability of both
// wrapped plugins. Hash rejects functional signatures; Sparse and LazySparse
// accept the live nonempty explanation index.
func (f *FiniteCheckedFactory) CanHandleExplanationIndex(signature Signature, functional int) bool {
	return f != nil && checkedKindHandlesFunctional(f.primaryKind, signature, functional) && checkedKindHandlesFunctional(f.checkerKind, signature, functional)
}

// AllowsUncheckedExplanationIndexFallback reports the exact source boundary
// where the checked favourite rejects a valid functional signature and manager
// plugin selection must fall through to ordinary Sparse.
func (f *FiniteCheckedFactory) AllowsUncheckedExplanationIndexFallback(signature Signature, functional int) bool {
	return f != nil && functional == 1 && signature.Len() >= functional && !f.CanHandleExplanationIndex(signature, functional)
}

// AllowsUncheckedNullaryFallback reports whether source plugin selection must
// bypass this checked favourite for the empty signature.
func (f *FiniteCheckedFactory) AllowsUncheckedNullaryFallback() bool {
	if f == nil {
		return false
	}
	signature, err := NewSignature(nil)
	return err == nil && !f.CanHandleSignature(signature)
}

func (f *FiniteCheckedFactory) table(kind string, signature Signature, functional int) (FiniteTable, error) {
	if !checkedKindHandles(kind, signature) {
		return nil, ErrCheckTable
	}
	switch kind {
	case "hashtable", "tr_hashtable":
		return NewHashTable(signature), nil
	case "sparse", "tr_sparse":
		return NewSparseTable(signature, functional)
	case "lazy_sparse", "tr_lazy_sparse":
		if f.lazy == nil {
			return nil, ErrCheckTable
		}
		if functional == 0 {
			return f.lazy.New(signature)
		}
		return f.lazy.NewFunctional(signature, functional)
	default:
		return nil, fmt.Errorf("%w: finite checker plugin %s", ErrTable, kind)
	}
}

func (f *FiniteCheckedFactory) New(signature Signature) (*FiniteCheckedPair, error) {
	if !f.CanHandleSignature(signature) {
		return nil, ErrCheckTable
	}
	primary, err := f.table(f.primaryKind, signature, 0)
	if err != nil {
		return nil, err
	}
	checker, err := f.table(f.checkerKind, signature, 0)
	if err != nil {
		return nil, errors.Join(err, primary.Close())
	}
	return adoptFiniteCheckedPair(primary, checker)
}

func adoptFiniteCheckedPair(primary, checker FiniteTable) (_ *FiniteCheckedPair, resultErr error) {
	if primary == nil || checker == nil {
		return nil, ErrCheckTable
	}
	result := &FiniteCheckedPair{primary: primary, checker: checker}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	primarySignature, err := primary.Signature()
	if err != nil {
		return nil, err
	}
	checkerSignature, err := checker.Signature()
	if err != nil || !primarySignature.equal(checkerSignature) {
		return nil, errors.Join(err, ErrCheckTable)
	}
	if err = result.verify("construction"); err != nil {
		return nil, err
	}
	return result, nil
}

func newFiniteTableKind(kind string, signature Signature) (FiniteTable, error) {
	switch kind {
	case "hashtable", "tr_hashtable":
		return NewHashTable(signature), nil
	case "sparse", "tr_sparse":
		return NewSparseTable(signature, 0)
	default:
		return nil, fmt.Errorf("%w: finite checker plugin %s", ErrTable, kind)
	}
}

func newFiniteTableLike(template FiniteTable, signature Signature) (FiniteTable, error) {
	switch concrete := template.(type) {
	case *HashTable:
		return NewHashTable(signature), nil
	case *SparseTable:
		if signature.Len() == 0 {
			return NewHashTable(signature), nil
		}
		return NewSparseTable(signature, 0)
	case *LazySparseTable:
		return lazySparseInstructionResult(concrete, signature, 0)
	default:
		return nil, fmt.Errorf("%w: unsupported checked component %T", ErrCheckTable, template)
	}
}

func copyFiniteFacts(target, source FiniteTable) error {
	facts, err := source.Facts()
	if err != nil {
		return err
	}
	for _, fact := range facts {
		if _, err = target.Add(fact); err != nil {
			return err
		}
	}
	return nil
}

// NewFiniteCheckedPair clones source as the selected implementation and
// creates an independent configured checker seeded with the same rows.
func NewFiniteCheckedPair(source FiniteTable, checkerKind string) (_ *FiniteCheckedPair, resultErr error) {
	if source == nil {
		return nil, ErrTable
	}
	primary, err := source.CloneFinite()
	if err != nil {
		return nil, err
	}
	signature, err := source.Signature()
	if err != nil {
		return nil, errors.Join(err, primary.Close())
	}
	checker, err := newFiniteTableKind(checkerKind, signature)
	if err != nil {
		return nil, errors.Join(err, primary.Close())
	}
	result := &FiniteCheckedPair{primary: primary, checker: checker}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	facts, err := source.Facts()
	if err != nil {
		return nil, err
	}
	for _, fact := range facts {
		if _, err = checker.Add(fact); err != nil {
			return nil, err
		}
	}
	if err = result.verify("construction"); err != nil {
		return nil, err
	}
	return result, nil
}

// NewFiniteCheckedPairFromFactory clones source as the primary implementation
// and constructs the configured checker, including lazy_sparse, through the
// context-bound factory. A signature rejected by the configured checked
// favourite must be handled by the relation manager's ordinary fallback.
func NewFiniteCheckedPairFromFactory(source FiniteTable, factory *FiniteCheckedFactory) (_ *FiniteCheckedPair, resultErr error) {
	if source == nil || factory == nil {
		return nil, ErrCheckTable
	}
	signature, err := source.Signature()
	if err != nil {
		return nil, err
	}
	if !factory.CanHandleSignature(signature) {
		return nil, ErrCheckTable
	}
	primary, err := source.CloneFinite()
	if err != nil {
		return nil, err
	}
	checker, err := factory.table(factory.checkerKind, signature, 0)
	if err != nil {
		return nil, errors.Join(err, primary.Close())
	}
	result := &FiniteCheckedPair{primary: primary, checker: checker}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	if err = copyFiniteFacts(checker, source); err != nil {
		return nil, err
	}
	if err = result.verify("seed"); err != nil {
		return nil, err
	}
	return result, nil
}

// Lift materializes a plain compiler operand into this pair's exact concrete
// primary/checker kinds. Nullary Sparse components use the source fallback.
func (p *FiniteCheckedPair) Lift(source FiniteTable) (_ *FiniteCheckedPair, resultErr error) {
	if err := p.open(); err != nil || source == nil {
		return nil, errors.Join(err, ErrCheckTable)
	}
	signature, err := source.Signature()
	if err != nil {
		return nil, err
	}
	primary, err := newFiniteTableLike(p.primary, signature)
	if err != nil {
		return nil, err
	}
	checker, err := newFiniteTableLike(p.checker, signature)
	if err != nil {
		return nil, errors.Join(err, primary.Close())
	}
	result := &FiniteCheckedPair{primary: primary, checker: checker}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	if err = copyFiniteFacts(primary, source); err != nil {
		return nil, err
	}
	if err = copyFiniteFacts(checker, source); err != nil {
		return nil, err
	}
	if err = result.verify("lift"); err != nil {
		return nil, err
	}
	return result, nil
}

func (p *FiniteCheckedPair) open() error {
	if p == nil || p.closed || p.primary == nil || p.checker == nil {
		return ErrCheckTable
	}
	return nil
}

func (p *FiniteCheckedPair) verify(objective string) error {
	if err := p.open(); err != nil {
		return err
	}
	a, err := p.primary.Facts()
	if err != nil {
		return err
	}
	b, err := p.checker.Facts()
	if err != nil {
		return err
	}
	if len(a) != len(b) {
		return fmt.Errorf("%w: %s row counts differ", ErrCheckTable, objective)
	}
	for _, fact := range a {
		found, err := p.checker.Contains(fact)
		if err != nil || !found {
			if err != nil {
				return err
			}
			return fmt.Errorf("%w: %s checker misses %v", ErrCheckTable, objective, fact)
		}
	}
	for _, fact := range b {
		found, err := p.primary.Contains(fact)
		if err != nil || !found {
			if err != nil {
				return err
			}
			return fmt.Errorf("%w: %s primary misses %v", ErrCheckTable, objective, fact)
		}
	}
	return nil
}

func (p *FiniteCheckedPair) Signature() (Signature, error) {
	if err := p.open(); err != nil {
		return Signature{}, err
	}
	return p.primary.Signature()
}
func (p *FiniteCheckedPair) Facts() ([]Fact, error) {
	if err := p.verify("facts"); err != nil {
		return nil, err
	}
	return p.primary.Facts()
}
func (p *FiniteCheckedPair) Add(fact Fact) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	a, err := p.primary.Add(fact)
	if err != nil {
		return false, err
	}
	b, err := p.checker.Add(fact)
	if err != nil {
		return false, err
	}
	if a != b {
		return false, fmt.Errorf("%w: insertion results differ", ErrCheckTable)
	}
	return a, p.verify("add")
}
func (p *FiniteCheckedPair) Contains(fact Fact) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	a, err := p.primary.Contains(fact)
	if err != nil {
		return false, err
	}
	b, err := p.checker.Contains(fact)
	if err != nil {
		return false, err
	}
	if a != b {
		return false, fmt.Errorf("%w: membership results differ", ErrCheckTable)
	}
	return a, nil
}
func (p *FiniteCheckedPair) Len() (int, error) {
	if err := p.verify("size"); err != nil {
		return 0, err
	}
	return p.primary.Len()
}
func (p *FiniteCheckedPair) SizeEstimateRows() (uint32, error) {
	if err := p.open(); err != nil {
		return 0, err
	}
	// Source check_table forwards metadata without iterating either side.
	// In particular, lazy_table's estimate remains 1 after materialization.
	switch primary := p.primary.(type) {
	case *HashTable:
		return primary.SizeEstimateRows()
	case *SparseTable:
		return primary.SizeEstimateRows()
	case *LazySparseTable:
		return primary.SizeEstimateRows()
	default:
		return 0, ErrCheckTable
	}
}
func (p *FiniteCheckedPair) SizeEstimateBytes() (uint32, error) {
	if err := p.open(); err != nil {
		return 0, err
	}
	switch primary := p.primary.(type) {
	case *HashTable:
		return primary.SizeEstimateBytes()
	case *SparseTable:
		return primary.SizeEstimateBytes()
	case *LazySparseTable:
		return primary.SizeEstimateBytes()
	default:
		return 0, ErrCheckTable
	}
}
func (p *FiniteCheckedPair) Reset() error {
	if err := p.open(); err != nil {
		return err
	}
	return errors.Join(p.primary.Reset(), p.checker.Reset())
}

// EmptyLike creates empty primary and checker instances of the same concrete
// kinds. It is the checked equivalent of table_plugin::mk_empty.
func (p *FiniteCheckedPair) EmptyLike() (*FiniteCheckedPair, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	primary, err := emptyFiniteLike(p.primary)
	if err != nil {
		return nil, err
	}
	checker, err := emptyFiniteLike(p.checker)
	if err != nil {
		return nil, errors.Join(err, primary.Close())
	}
	return adoptFiniteCheckedPair(primary, checker)
}

// Complement implements the live finite complement boundary used by checked
// full construction. Wider complements remain compiler-decomposed.
func (p *FiniteCheckedPair) Complement() (*FiniteCheckedPair, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	signature, err := p.Signature()
	if err != nil {
		return nil, err
	}
	if signature.Len() > 1 {
		return nil, fmt.Errorf("%w: complement requires compiler decomposition", ErrCheckTable)
	}
	result, err := p.EmptyLike()
	if err != nil {
		return nil, err
	}
	failed := true
	defer func() {
		if failed {
			_ = result.Close()
		}
	}()
	if signature.Len() == 0 {
		found, err := p.Contains(Fact{})
		if err != nil {
			return nil, err
		}
		if !found {
			if _, err = result.Add(Fact{}); err != nil {
				return nil, err
			}
		}
	} else {
		domain, err := signature.Domain(0)
		if err != nil {
			return nil, err
		}
		for value := uint64(0); value < domain; value++ {
			fact := Fact{Element(value)}
			found, err := p.Contains(fact)
			if err != nil {
				return nil, err
			}
			if !found {
				if _, err = result.Add(fact); err != nil {
					return nil, err
				}
			}
		}
	}
	failed = false
	return result, nil
}

// FullLike creates a checked full relation of the same concrete pair kinds.
func (p *FiniteCheckedPair) FullLike() (*FiniteCheckedPair, error) {
	empty, err := p.EmptyLike()
	if err != nil {
		return nil, err
	}
	full, fullErr := empty.Complement()
	closeErr := empty.Close()
	if fullErr != nil || closeErr != nil {
		if full != nil {
			closeErr = errors.Join(closeErr, full.Close())
		}
		return nil, errors.Join(fullErr, closeErr)
	}
	return full, nil
}

func (p *FiniteCheckedPair) Join(other *FiniteCheckedPair, left, right []int) (*FiniteCheckedPair, error) {
	if err := p.open(); err != nil || other == nil || other.open() != nil {
		return nil, ErrCheckTable
	}
	primary, err := finiteTableJoin(p.primary, other.primary, left, right)
	if err != nil {
		return nil, err
	}
	checker, err := finiteTableJoin(p.checker, other.checker, left, right)
	if err != nil {
		return nil, errors.Join(err, primary.Close())
	}
	return adoptFiniteCheckedPair(primary, checker)
}

func (p *FiniteCheckedPair) JoinProject(other *FiniteCheckedPair, left, right, removed []int) (*FiniteCheckedPair, error) {
	if err := p.open(); err != nil || other == nil || other.open() != nil {
		return nil, ErrCheckTable
	}
	primary, err := finiteTableJoinProject(p.primary, other.primary, left, right, removed)
	if err != nil {
		return nil, err
	}
	checker, err := finiteTableJoinProject(p.checker, other.checker, left, right, removed)
	if err != nil {
		return nil, errors.Join(err, primary.Close())
	}
	return adoptFiniteCheckedPair(primary, checker)
}

func (p *FiniteCheckedPair) Project(removed []int) (*FiniteCheckedPair, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	primary, err := finiteTableProject(p.primary, removed)
	if err != nil {
		return nil, err
	}
	checker, err := finiteTableProject(p.checker, removed)
	if err != nil {
		return nil, errors.Join(err, primary.Close())
	}
	return adoptFiniteCheckedPair(primary, checker)
}

func (p *FiniteCheckedPair) RenameCycle(cycle []int) (*FiniteCheckedPair, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	primary, err := finiteTableRename(p.primary, cycle)
	if err != nil {
		return nil, err
	}
	checker, err := finiteTableRename(p.checker, cycle)
	if err != nil {
		return nil, errors.Join(err, primary.Close())
	}
	return adoptFiniteCheckedPair(primary, checker)
}

func (p *FiniteCheckedPair) SelectEqualAndProject(value Element, column int) (*FiniteCheckedPair, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	primary, err := finiteTableSelectProject(p.primary, value, column)
	if err != nil {
		return nil, err
	}
	checker, err := finiteTableSelectProject(p.checker, value, column)
	if err != nil {
		return nil, errors.Join(err, primary.Close())
	}
	return adoptFiniteCheckedPair(primary, checker)
}

func (p *FiniteCheckedPair) FilterEqual(value Element, column int) error {
	if err := p.open(); err != nil {
		return err
	}
	if err := finiteTableFilterEqual(p.primary, value, column); err != nil {
		return err
	}
	if err := finiteTableFilterEqual(p.checker, value, column); err != nil {
		return err
	}
	return p.verify("filter equal")
}

func (p *FiniteCheckedPair) FilterIdentical(columns []int) error {
	if err := p.open(); err != nil {
		return err
	}
	if err := finiteTableFilterIdentical(p.primary, columns); err != nil {
		return err
	}
	if err := finiteTableFilterIdentical(p.checker, columns); err != nil {
		return err
	}
	return p.verify("filter identical")
}

func (p *FiniteCheckedPair) FilterByNegation(negative *FiniteCheckedPair, targetColumns, negativeColumns []int) error {
	if err := p.open(); err != nil || negative == nil || negative.open() != nil {
		return ErrCheckTable
	}
	if err := finiteTableFilterNegation(p.primary, negative.primary, targetColumns, negativeColumns); err != nil {
		return err
	}
	if err := finiteTableFilterNegation(p.checker, negative.checker, targetColumns, negativeColumns); err != nil {
		return err
	}
	return p.verify("filter negation")
}

func filterCheckedPairInterpreted(table FiniteTable, manager *RelationManager, relationSorts []ast.ASTHandle, theory *astrewriter.TheoryRewriter, conditions []ast.ASTHandle, columns map[uint32]int, sorts map[uint32]ast.ASTHandle) error {
	switch concrete := table.(type) {
	case *HashTable:
		relation := &TableRelation{manager: manager, sorts: append([]ast.ASTHandle(nil), relationSorts...), table: concrete}
		return relation.FilterInterpreted(theory, conditions, columns, sorts)
	case *SparseTable:
		return filterSparseInterpretedBound(concrete, manager, theory, conditions, columns, sorts)
	case *LazySparseTable:
		return concrete.filterInterpretedCompiled(manager, theory, conditions, columns, sorts)
	default:
		return ErrCheckTable
	}
}

// FilterInterpreted executes the source checker first and selected primary
// second, then verifies the concrete results remain extensionally equal.
func (p *FiniteCheckedPair) FilterInterpreted(manager *RelationManager, relationSorts []ast.ASTHandle, theory *astrewriter.TheoryRewriter, conditions []ast.ASTHandle, columns map[uint32]int, sorts map[uint32]ast.ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	if err := filterCheckedPairInterpreted(p.checker, manager, relationSorts, theory, conditions, columns, sorts); err != nil {
		return err
	}
	if err := filterCheckedPairInterpreted(p.primary, manager, relationSorts, theory, conditions, columns, sorts); err != nil {
		return err
	}
	return p.verify("filter interpreted")
}

// UnionFrom executes the reached check_table union semantics and accumulates
// only newly inserted rows in delta. Existing delta rows are preserved.
func (p *FiniteCheckedPair) UnionFrom(source, delta *FiniteCheckedPair) error {
	return p.unionFromConcrete(source, delta)
}

func (p *FiniteCheckedPair) CloneFinite() (FiniteTable, error) {
	if err := p.verify("clone"); err != nil {
		return nil, err
	}
	primary, err := p.primary.CloneFinite()
	if err != nil {
		return nil, err
	}
	checker, err := p.checker.CloneFinite()
	if err != nil {
		return nil, errors.Join(err, primary.Close())
	}
	return &FiniteCheckedPair{primary: primary, checker: checker}, nil
}
func (p *FiniteCheckedPair) Close() error {
	if p == nil {
		return ErrCheckTable
	}
	if p.closed {
		return nil
	}
	// Source destruction releases the primary before the checker. A lazy
	// side may refuse AST-condition cleanup; keep that owner for retry.
	if p.primary != nil {
		if err := p.primary.Close(); err != nil {
			return err
		}
		p.primary = nil
	}
	if p.checker != nil {
		if err := p.checker.Close(); err != nil {
			return err
		}
		p.checker = nil
	}
	p.closed = true
	return nil
}

func (p *FiniteCheckedPair) ConcreteKinds() (primary, checker string, err error) {
	if err = p.open(); err != nil {
		return "", "", err
	}
	kind := func(table FiniteTable) string {
		switch table.(type) {
		case *HashTable:
			return "hashtable"
		case *SparseTable:
			return "sparse"
		case *LazySparseTable:
			return "lazy_sparse"
		default:
			return fmt.Sprintf("%T", table)
		}
	}
	return kind(p.primary), kind(p.checker), nil
}

var _ FiniteTable = (*FiniteCheckedPair)(nil)
