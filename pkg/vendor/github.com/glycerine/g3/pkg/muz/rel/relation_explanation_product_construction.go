package rel

// Source: src/muz/rel/dl_product_relation.cpp mk_full, converting joins and
// transform_fn, specialized to the finite and whole-explanation sieves used
// by relation-level explanations.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func newRelationExplanationFiniteBacking(manager *RelationManager, signature Signature, source FiniteTable) (FiniteTable, error) {
	if manager == nil || manager.open() != nil {
		return nil, ErrTableRelation
	}
	if source != nil {
		switch source.(type) {
		case *FormulaCheckedTable, *FormulaCheckedFinitePair:
			return emptyFiniteLike(source)
		}
		if manager.checkedFinite == nil {
			return emptyFiniteLike(source)
		}
		// Checked favourites participate only when both concrete plugins handle
		// the signature. Sparse and lazy_sparse reject the nullary signature, so
		// preserve the source-selected ordinary Hash fallback for that product.
		if !manager.checkedFinite.CanHandleSignature(signature) {
			return emptyFiniteLike(source)
		}
		if _, checked := source.(*FiniteCheckedPair); checked {
			return emptyFiniteLike(source)
		}
		// Source transfer preserves the actual selected primary (including the
		// context-owned lazy_sparse engine) while constructing an empty backing;
		// the caller copies the snapshot exactly once into both checked sides.
		primary, err := source.CloneFinite()
		if err != nil {
			return nil, err
		}
		if err = primary.Reset(); err != nil {
			return nil, errors.Join(err, primary.Close())
		}
		checker, err := manager.checkedFinite.table(manager.checkedFinite.checkerKind, signature, 0)
		if err != nil {
			return nil, errors.Join(err, primary.Close())
		}
		return adoptFiniteCheckedPair(primary, checker)
	}
	return manager.NewDefaultFiniteTable(signature)
}

// newRelationExplanationFiniteBackingSelected builds the ordinary relation
// plugin selected as the check_relation favourite. Sparse and LazySparse
// reject nullary signatures in C++, so the source relation manager falls back
// to Hash for that shape.
func newRelationExplanationFiniteBackingSelected(manager *RelationManager, signature Signature, kind string) (FiniteTable, error) {
	switch kind {
	case "":
		return manager.NewDefaultFiniteTable(signature)
	case "hashtable":
		return NewHashTable(signature), nil
	case "sparse":
		if signature.Len() == 0 {
			return NewHashTable(signature), nil
		}
		return NewSparseTable(signature, 0)
	case "lazy_sparse":
		if signature.Len() == 0 {
			return NewHashTable(signature), nil
		}
		if manager.lazyFinite == nil || !manager.lazyFinite.CanHandleSignature(signature) {
			return nil, ErrTable
		}
		return manager.lazyFinite.New(signature)
	default:
		return nil, ErrTable
	}
}

func newRelationExplanationProductWithBacking(manager *RelationManager, sorts []ast.ASTHandle, source FiniteTable) (_ *RelationExplanationProduct, resultErr error) {
	if manager != nil && manager.explanationUDoc != nil {
		handles, err := manager.explanationUDoc.CanHandleSignature(sorts)
		if err != nil {
			return nil, err
		}
		if handles {
			return NewUDocRelationExplanationProduct(manager, sorts, manager.explanationUDoc)
		}
	}
	product, err := NewRelationExplanationProduct(manager, sorts)
	if err != nil || source == nil {
		return product, err
	}
	if product.numeric != nil {
		// Relation-manager finite seeds are only convertible to a numeric
		// backing for the nullary truth relation; numeric tuples are stored in
		// their typed activation owners, never as finite table elements.
		count, countErr := source.Len()
		if countErr != nil {
			return nil, errors.Join(countErr, product.Close())
		}
		if count != 0 {
			if len(sorts) != 0 {
				return nil, errors.Join(ErrTableRelation, product.Close())
			}
			seed := RelationExplanationNumericSeed{Kind: product.numeric.kind}
			switch seed.Kind {
			case "bound_relation":
				seed.Bound = NewBoundRelation(0, false)
			case "interval_relation":
				seed.Interval = NewIntervalFull(0)
			case "karr_relation":
				seed.Karr = NewKarrFull(0, nil, product.numeric.karr.formulaChecker)
			}
			if err = product.UnionNumericSeed(seed); err != nil {
				return nil, errors.Join(err, product.Close())
			}
		}
		return product, nil
	}
	signature, err := product.table.Signature()
	if err != nil {
		return nil, errors.Join(err, product.Close())
	}
	backing, err := newRelationExplanationFiniteBacking(manager, signature, source)
	if err != nil {
		return nil, errors.Join(err, product.Close())
	}
	if err = product.table.Close(); err != nil {
		return nil, errors.Join(err, backing.Close(), product.Close())
	}
	product.table = backing
	return product, nil
}

func newRelationExplanationProductWithSelectedBacking(manager *RelationManager, sorts []ast.ASTHandle, kind string) (_ *RelationExplanationProduct, resultErr error) {
	if kind == "doc" {
		if manager == nil || manager.explanationUDoc == nil {
			return nil, ErrCompilerUnsupported
		}
		handles, err := manager.explanationUDoc.CanHandleSignature(sorts)
		if err != nil {
			return nil, err
		}
		if handles {
			return NewUDocRelationExplanationProduct(manager, sorts, manager.explanationUDoc)
		}
		// check_relation delegates capability to its base. The relation manager
		// therefore falls through to the ordinary Hash plugin for signatures
		// the selected DOC base rejects.
		kind = "hashtable"
	}
	if kind == "bound_relation" || kind == "interval_relation" || kind == "karr_relation" {
		accepted, err := relationExplanationNumericCanHandle(manager, kind, sorts)
		if err != nil {
			return nil, err
		}
		if accepted {
			return NewNumericRelationExplanationProduct(manager, sorts, kind, manager.explanationNumericCheck)
		}
		kind = "hashtable"
	}
	product, err := NewRelationExplanationProduct(manager, sorts)
	if err != nil || kind == "" {
		return product, err
	}
	signature, err := product.table.Signature()
	if err != nil {
		return nil, errors.Join(err, product.Close())
	}
	backing, err := newRelationExplanationFiniteBackingSelected(manager, signature, kind)
	if err != nil {
		return nil, errors.Join(err, product.Close())
	}
	if err = product.table.Close(); err != nil {
		return nil, errors.Join(err, backing.Close(), product.Close())
	}
	product.table = backing
	return product, nil
}

// EmptyLike preserves the selected finite representation for union targets
// and deltas while constructing an empty proof component of identical width.
func (p *RelationExplanationProduct) EmptyLike() (_ *RelationExplanationProduct, resultErr error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	var table FiniteTable
	var doc *UDocTableProduct
	var numeric *relationExplanationNumeric
	var err error
	if p.doc != nil {
		doc, err = p.doc.EmptyLike()
	} else if p.numeric != nil {
		numeric, err = newRelationExplanationNumeric(p.manager, p.finiteSorts, p.numeric.kind, p.numeric.checker)
	} else {
		table, err = emptyFiniteLike(p.table)
	}
	if err != nil {
		return nil, err
	}
	proofSorts, err := p.explanation.signature()
	if err != nil {
		if doc != nil {
			return nil, errors.Join(err, doc.Close())
		}
		if numeric != nil {
			return nil, errors.Join(err, numeric.Close())
		}
		return nil, errors.Join(err, table.Close())
	}
	explanation, err := NewExplanationRelation(p.manager.manager, proofSorts)
	if err != nil {
		if doc != nil {
			return nil, errors.Join(err, doc.Close())
		}
		if numeric != nil {
			return nil, errors.Join(err, numeric.Close())
		}
		return nil, errors.Join(err, table.Close())
	}
	return &RelationExplanationProduct{manager: p.manager, finiteSorts: append([]ast.ASTHandle(nil), p.finiteSorts...), table: table, doc: doc, numeric: numeric, explanation: explanation}, nil
}

// NewRelationExplanationTotal constructs the join identity with one nullary
// finite tuple and one undefined whole-explanation row. A zero-column proof
// row is valid and remains nonempty.
func NewRelationExplanationTotal(manager *RelationManager, proofColumns int) (_ *RelationExplanationProduct, resultErr error) {
	if manager == nil || manager.open() != nil || proofColumns < 0 {
		return nil, ErrTableRelation
	}
	if manager.explanationNumericKind != "" {
		product, err := newNumericRelationExplanationProductWithProofColumns(manager, nil, manager.explanationNumericKind, manager.explanationNumericCheck, proofColumns)
		if err != nil {
			return nil, err
		}
		seed := RelationExplanationNumericSeed{Kind: product.numeric.kind}
		switch seed.Kind {
		case "bound_relation":
			seed.Bound = NewBoundRelation(0, false)
		case "interval_relation":
			seed.Interval = NewIntervalFull(0)
		case "karr_relation":
			seed.Karr = NewKarrFull(0, nil, product.numeric.karr.formulaChecker)
		default:
			return nil, errors.Join(ErrTableRelation, product.Close())
		}
		if err = product.UnionNumericSeed(seed); err == nil {
			err = product.explanation.AddFact(make([]ast.ASTHandle, proofColumns))
		}
		if err != nil {
			return nil, errors.Join(err, product.Close())
		}
		return product, nil
	}
	signature, err := NewSignature(nil)
	if err != nil {
		return nil, err
	}
	table, err := manager.NewDefaultFiniteTable(signature)
	if err != nil {
		return nil, err
	}
	if _, err = table.Add(Fact{}); err != nil {
		return nil, errors.Join(err, table.Close())
	}
	ruleSort, err := manager.manager.MkDatalogRuleSort()
	if err != nil {
		return nil, errors.Join(err, table.Close())
	}
	proofSorts := make([]ast.ASTHandle, proofColumns)
	for index := range proofSorts {
		proofSorts[index] = ruleSort
	}
	explanation, err := NewExplanationRelation(manager.manager, proofSorts)
	if err != nil {
		return nil, errors.Join(err, table.Close())
	}
	if err = explanation.AddFact(make([]ast.ASTHandle, proofColumns)); err != nil {
		return nil, errors.Join(err, explanation.Close(), table.Close())
	}
	return &RelationExplanationProduct{manager: manager, table: table, explanation: explanation}, nil
}

// RelationExplanationFromFinite copies the actual finite rows into the
// concrete Hash sieve and pairs them with the full zero-proof identity.
func RelationExplanationFromFinite(manager *RelationManager, finiteSorts []ast.ASTHandle, source FiniteTable) (_ *RelationExplanationProduct, resultErr error) {
	if manager == nil || manager.open() != nil || source == nil {
		return nil, ErrTableRelation
	}
	expected, err := tableRelationSignature(manager, finiteSorts)
	if err != nil {
		return nil, err
	}
	actual, err := source.Signature()
	if err != nil || !expected.equal(actual) {
		return nil, errors.Join(err, ErrTableRelation)
	}
	table, err := newRelationExplanationFiniteBacking(manager, expected, source)
	if err != nil {
		return nil, err
	}
	rows, err := source.Facts()
	if err != nil {
		return nil, errors.Join(err, table.Close())
	}
	for _, row := range rows {
		if _, err = table.Add(row); err != nil {
			return nil, errors.Join(err, table.Close())
		}
	}
	explanation, err := NewExplanationRelation(manager.manager, nil)
	if err != nil {
		return nil, errors.Join(err, table.Close())
	}
	if err = explanation.AddFact(nil); err != nil {
		return nil, errors.Join(err, explanation.Close(), table.Close())
	}
	return &RelationExplanationProduct{manager: manager, finiteSorts: append([]ast.ASTHandle(nil), finiteSorts...), table: table, explanation: explanation}, nil
}

// RenameFinite applies a finite-column cycle while cloning the independent
// whole-explanation sieve.
func (p *RelationExplanationProduct) RenameFinite(cycle []int) (_ *RelationExplanationProduct, resultErr error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	permutation, err := tableRelationCyclePermutation(len(p.finiteSorts), cycle)
	if err != nil {
		return nil, err
	}
	var table FiniteTable
	var doc *UDocTableProduct
	var numeric *relationExplanationNumeric
	if p.doc != nil {
		columns := make([]uint32, len(cycle))
		for i, column := range cycle {
			columns[i] = uint32(column)
		}
		doc, err = p.doc.Rename(columns)
	} else if p.numeric != nil {
		columns := make([]uint32, len(cycle))
		for i, column := range cycle {
			columns[i] = uint32(column)
		}
		numeric, err = p.numeric.Rename(columns)
	} else {
		table, err = finiteTableRename(p.table, cycle)
	}
	if err != nil {
		return nil, err
	}
	explanation, err := p.explanation.Clone()
	if err != nil {
		if doc != nil {
			return nil, errors.Join(err, doc.Close())
		}
		if numeric != nil {
			return nil, errors.Join(err, numeric.Close())
		}
		return nil, errors.Join(err, table.Close())
	}
	sorts := make([]ast.ASTHandle, len(p.finiteSorts))
	for column, source := range permutation {
		sorts[column] = p.finiteSorts[source]
	}
	return &RelationExplanationProduct{manager: p.manager, finiteSorts: sorts, table: table, doc: doc, numeric: numeric, explanation: explanation}, nil
}

func (p *RelationExplanationProduct) FilterEqual(value Element, column int) error {
	if err := p.open(); err != nil {
		return err
	}
	if p.numeric != nil {
		return ErrCompilerUnsupported
	}
	if p.doc != nil {
		if column < 0 || column >= len(p.finiteSorts) {
			return ErrUDocTableProduct
		}
		decoded, err := p.manager.DecodeValue(p.finiteSorts[column], value)
		if err != nil {
			return err
		}
		handle, err := p.manager.manager.RefGet(decoded)
		if err != nil {
			return errors.Join(err, p.manager.manager.DelRef(&decoded))
		}
		return errors.Join(p.doc.FilterEqual(uint32(column), handle), p.manager.manager.DelRef(&decoded))
	}
	return finiteTableFilterEqual(p.table, value, column)
}

// FilterEqualAST preserves unbounded arithmetic constants until the selected
// numeric relation plugin interprets them.
func (p *RelationExplanationProduct) FilterEqualAST(value ast.ASTHandle, column int) error {
	if err := p.open(); err != nil {
		return err
	}
	if p.numeric == nil {
		return ErrCompilerUnsupported
	}
	if column < 0 || column >= len(p.finiteSorts) {
		return ast.ErrASTDomain
	}
	return p.numeric.FilterEqual(uint32(column), value)
}

func (p *RelationExplanationProduct) FilterIdentical(columns []int) error {
	if err := p.open(); err != nil {
		return err
	}
	if p.doc != nil {
		mapped := make([]uint32, len(columns))
		for i, column := range columns {
			if column < 0 || column >= len(p.finiteSorts) {
				return ErrUDocTableProduct
			}
			mapped[i] = uint32(column)
		}
		return p.doc.FilterIdentical(mapped)
	}
	if p.numeric != nil {
		mapped := make([]uint32, len(columns))
		for i, column := range columns {
			if column < 0 || column >= len(p.finiteSorts) {
				return ast.ErrASTDomain
			}
			mapped[i] = uint32(column)
		}
		return p.numeric.FilterIdentical(mapped)
	}
	return finiteTableFilterIdentical(p.table, columns)
}

// Len reports logical rows. A product with an empty finite or explanation
// component is empty, matching product_relation::empty.
func (p *RelationExplanationProduct) Len() (int, error) {
	if err := p.open(); err != nil {
		return 0, err
	}
	empty, err := p.explanation.Empty()
	if err != nil || empty {
		return 0, err
	}
	if p.doc != nil {
		empty, err := p.doc.Empty()
		if err != nil || empty {
			return 0, err
		}
		return 1, nil
	}
	if p.numeric != nil {
		empty, err := p.numeric.Empty()
		if err != nil || empty {
			return 0, err
		}
		return 1, nil
	}
	return p.table.Len()
}

func (p *RelationExplanationProduct) Empty() (bool, error) {
	rows, err := p.Len()
	return rows == 0, err
}

// CheckedConcreteKinds reports the concrete primary/checker pair used by the
// finite component. Ordinary products return checked=false.
func (p *RelationExplanationProduct) CheckedConcreteKinds() (primary, checker string, checked bool, err error) {
	if err = p.open(); err != nil {
		return "", "", false, err
	}
	switch table := p.table.(type) {
	case *FiniteCheckedPair:
		primary, checker, err = table.ConcreteKinds()
		return primary, checker, true, err
	case *FormulaCheckedTable:
		return "hashtable", "formula", true, nil
	case *FormulaCheckedFinitePair:
		primary, checker, _, err = table.ConcreteKinds()
		return primary, checker, true, err
	case *HashTable:
		return "hashtable", "", false, nil
	case *SparseTable:
		return "sparse", "", false, nil
	case *LazySparseTable:
		return "lazy_sparse", "", false, nil
	default:
		if p.numeric != nil {
			return p.numeric.kind, map[bool]string{true: "formula", false: ""}[p.numeric.checker != nil], p.numeric.checker != nil, nil
		}
		if p.doc != nil {
			checked, checkErr := p.doc.Doc().Inner().FormulaChecked(), error(nil)
			if checkErr != nil {
				return "", "", false, checkErr
			}
			return "doc", map[bool]string{true: "formula", false: ""}[checked], checked, nil
		}
		return "", "", false, ErrCompilerUnsupported
	}
}

// FormulaChecked reports whether the finite sieve is guarded by the concrete
// source check_relation wrapper. The whole proof relation is never part of
// this checker.
func (p *RelationExplanationProduct) FormulaChecked() (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	switch p.table.(type) {
	case *FormulaCheckedTable, *FormulaCheckedFinitePair:
		return true, nil
	default:
		if p.numeric != nil {
			return p.numeric.checker != nil, nil
		}
		if p.doc != nil {
			return p.doc.Doc().Inner().FormulaChecked(), nil
		}
		return false, nil
	}
}
