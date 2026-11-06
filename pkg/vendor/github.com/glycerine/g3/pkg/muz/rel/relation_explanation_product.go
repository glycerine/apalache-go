package rel

// Source: src/muz/rel/dl_mk_explanations.cpp,
// translate_rel_level_relation's concrete finite-table and explanation sieves.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
)

// RelationExplanationProduct keeps one explanation for an entire finite
// relation, alongside a private copy of its table. It borrows the manager and
// finite sort handles and owns the copied table and explanation relation.
type RelationExplanationProduct struct {
	manager     *RelationManager
	finiteSorts []ast.ASTHandle
	table       FiniteTable
	doc         *UDocTableProduct
	numeric     *relationExplanationNumeric
	explanation *ExplanationRelation
}

// NewRelationExplanationProduct constructs an empty finite/whole-explanation
// product. Its external signature is the finite predicate signature; the
// single Datalog-rule explanation column is internal to the product.
func NewRelationExplanationProduct(manager *RelationManager, sorts []ast.ASTHandle) (_ *RelationExplanationProduct, resultErr error) {
	if manager == nil || manager.open() != nil {
		return nil, ast.ErrASTHandle
	}
	if manager.explanationUDoc != nil {
		handles, err := manager.explanationUDoc.CanHandleSignature(sorts)
		if err != nil {
			return nil, err
		}
		if handles {
			return NewUDocRelationExplanationProduct(manager, sorts, manager.explanationUDoc)
		}
	}
	if manager.explanationNumericKind != "" {
		accepted, err := relationExplanationNumericCanHandle(manager, manager.explanationNumericKind, sorts)
		if err != nil {
			return nil, err
		}
		if accepted {
			return NewNumericRelationExplanationProduct(manager, sorts, manager.explanationNumericKind, manager.explanationNumericCheck)
		}
	}
	domains := make([]uint64, len(sorts))
	for i, sort := range sorts {
		var err error
		domains[i], err = manager.sortDomain(sort)
		if err != nil {
			return nil, fmt.Errorf("relation explanation numeric backing %q did not accept signature: %w", manager.explanationNumericKind, errors.Join(err, ErrTableRelation))
		}
	}
	signature, err := NewSignature(domains)
	if err != nil {
		return nil, err
	}
	table, err := manager.NewDefaultFiniteTable(signature)
	if err != nil {
		return nil, err
	}
	p := &RelationExplanationProduct{manager: manager, finiteSorts: append([]ast.ASTHandle(nil), sorts...), table: table}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, p.Close())
		}
	}()
	ruleSort, err := manager.manager.MkDatalogRuleSort()
	if err != nil {
		return nil, err
	}
	p.explanation, err = NewExplanationRelation(manager.manager, []ast.ASTHandle{ruleSort})
	if err != nil {
		return nil, err
	}
	return p, nil
}

// NewNumericRelationExplanationProduct constructs a relation-level product
// whose ordinary component uses a selected numeric abstract relation.
func NewNumericRelationExplanationProduct(manager *RelationManager, sorts []ast.ASTHandle, kind string, checker *smtcore.SMTSolverConfig) (_ *RelationExplanationProduct, resultErr error) {
	return newNumericRelationExplanationProductWithProofColumns(manager, sorts, kind, checker, 1)
}

func newNumericRelationExplanationProductWithProofColumns(manager *RelationManager, sorts []ast.ASTHandle, kind string, checker *smtcore.SMTSolverConfig, proofColumns int) (_ *RelationExplanationProduct, resultErr error) {
	if manager == nil || manager.open() != nil || proofColumns < 0 {
		return nil, ErrTableRelation
	}
	numeric, err := newRelationExplanationNumeric(manager, sorts, kind, checker)
	if err != nil {
		return nil, err
	}
	p := &RelationExplanationProduct{manager: manager, finiteSorts: append([]ast.ASTHandle(nil), sorts...), numeric: numeric}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, p.Close())
		}
	}()
	ruleSort, err := manager.manager.MkDatalogRuleSort()
	if err != nil {
		return nil, err
	}
	proofSorts := make([]ast.ASTHandle, proofColumns)
	for index := range proofSorts {
		proofSorts[index] = ruleSort
	}
	p.explanation, err = NewExplanationRelation(manager.manager, proofSorts)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// NewUDocRelationExplanationProduct constructs the relation-level explanation
// product with the source-selected UDoc sieve as its ordinary relation
// component. The UDoc factory is borrowed and must outlive the product.
func NewUDocRelationExplanationProduct(manager *RelationManager, sorts []ast.ASTHandle, factory *UDocRelationFactory) (_ *RelationExplanationProduct, resultErr error) {
	if manager == nil || manager.open() != nil || factory == nil || factory.open() != nil || factory.manager != manager.manager {
		return nil, ErrTableRelation
	}
	sieve, err := NewEmptyUDocSieve(factory, sorts)
	if err != nil {
		return nil, err
	}
	doc := &UDocTableProduct{manager: manager, doc: sieve}
	p := &RelationExplanationProduct{manager: manager, finiteSorts: append([]ast.ASTHandle(nil), sorts...), doc: doc}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, p.Close())
		}
	}()
	ruleSort, err := manager.manager.MkDatalogRuleSort()
	if err != nil {
		return nil, err
	}
	p.explanation, err = NewExplanationRelation(manager.manager, []ast.ASTHandle{ruleSort})
	if err != nil {
		return nil, err
	}
	return p, nil
}

func newRelationExplanationProduct(manager *RelationManager, sorts []ast.ASTHandle, source FiniteTable, fact ast.ASTHandle) (_ *RelationExplanationProduct, resultErr error) {
	p, err := newRelationExplanationProductWithBacking(manager, sorts, source)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, p.Close())
		}
	}()
	if source != nil {
		if p.doc != nil {
			err = importFiniteFactsToExplanationUDoc(manager, p.finiteSorts, source, p.doc)
		} else {
			err = relationExplanationFiniteUnion(p.table, source, nil, p.finiteSorts)
		}
		if err != nil {
			return nil, err
		}
	}
	if err = p.explanation.AddFact([]ast.ASTHandle{fact}); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *RelationExplanationProduct) open() error {
	backings := 0
	if p != nil {
		if p.table != nil {
			backings++
		}
		if p.doc != nil {
			backings++
		}
		if p.numeric != nil {
			backings++
		}
	}
	if p == nil || p.manager == nil || p.manager.open() != nil || backings != 1 || p.doc != nil && p.doc.open() != nil || p.numeric != nil && p.numeric.open() != nil || p.explanation == nil {
		return ErrTableRelation
	}
	return nil
}

func (p *RelationExplanationProduct) FiniteSignature() (Signature, error) {
	if err := p.open(); err != nil {
		return Signature{}, err
	}
	if p.doc != nil {
		return tableRelationSignature(p.manager, p.finiteSorts)
	}
	if p.numeric != nil {
		return Signature{}, ErrTable
	}
	return p.table.Signature()
}

// UnionUDocSeed transfers a typed UDoc seed into the product's ordinary
// relation component without enumerating or expanding its stored cubes.
func (p *RelationExplanationProduct) UnionUDocSeed(source *TypedUDocRelation) error {
	if err := p.open(); err != nil {
		return err
	}
	if p.doc == nil || source == nil || source.factory != p.doc.Doc().Inner().factory {
		return ErrUDocTableProduct
	}
	return p.doc.Doc().Inner().UnionFrom(source, nil)
}

// Explanation returns the relation-level explanation for a present tuple.
// The returned AST handle is borrowed from the product.
func (p *RelationExplanationProduct) Explanation(values []ast.ASTHandle) (ast.ASTHandle, bool, error) {
	if err := p.open(); err != nil {
		return ast.ASTHandle{}, false, err
	}
	if len(values) != len(p.finiteSorts) {
		return ast.ASTHandle{}, false, ast.ErrASTDomain
	}
	var found bool
	var err error
	if p.doc != nil {
		found, err = p.doc.ContainsFact(values)
	} else if p.numeric != nil {
		found, err = p.numeric.Contains(values)
	} else {
		key := make(Fact, len(values))
		for i, value := range values {
			encoded, encodeErr := p.manager.EncodeValue(p.finiteSorts[i], value)
			if encodeErr != nil {
				return ast.ASTHandle{}, false, encodeErr
			}
			key[i] = encoded
		}
		found, err = p.table.Contains(key)
	}
	if err != nil || !found {
		return ast.ASTHandle{}, false, err
	}
	data, err := p.explanation.Data()
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if len(data) != 1 {
		return ast.ASTHandle{}, false, ErrTableRelation
	}
	return data[0], true, nil
}

// Clone copies both the finite sieve and the single whole-relation proof.
func (p *RelationExplanationProduct) Clone() (_ *RelationExplanationProduct, resultErr error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	var table FiniteTable
	var doc *UDocTableProduct
	var numeric *relationExplanationNumeric
	var err error
	if p.doc != nil {
		doc, err = p.doc.Clone()
	} else if p.numeric != nil {
		numeric, err = p.numeric.Clone()
	} else {
		table, err = p.table.CloneFinite()
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
	return &RelationExplanationProduct{manager: p.manager, finiteSorts: append([]ast.ASTHandle(nil), p.finiteSorts...), table: table, doc: doc, numeric: numeric, explanation: explanation}, nil
}

func (p *RelationExplanationProduct) compatible(other *RelationExplanationProduct) error {
	if err := p.open(); err != nil {
		return err
	}
	if err := other.open(); err != nil {
		return err
	}
	if p.manager != other.manager || len(p.finiteSorts) != len(other.finiteSorts) {
		return ast.ErrASTDomain
	}
	for i := range p.finiteSorts {
		if p.finiteSorts[i] != other.finiteSorts[i] {
			return ast.ErrASTDomain
		}
	}
	if (p.numeric == nil) != (other.numeric == nil) || p.numeric != nil && p.numeric.kind != other.numeric.kind {
		return ast.ErrASTDomain
	}
	return p.explanation.compatible(other.explanation)
}

func (p *RelationExplanationProduct) requireCoherent() error {
	if err := p.open(); err != nil {
		return err
	}
	if p.doc != nil {
		empty, err := p.doc.Empty()
		if err != nil || empty {
			return err
		}
	} else if p.numeric != nil {
		empty, err := p.numeric.Empty()
		if err != nil || empty {
			return err
		}
	} else {
		rows, err := p.table.Len()
		if err != nil || rows == 0 {
			return err
		}
	}
	empty, err := p.explanation.Empty()
	if err != nil {
		return err
	}
	if empty {
		return ErrTableRelation
	}
	return nil
}

// Union combines finite rows while aggregating the single whole-relation
// proof with e_union. Delta accumulates newly added rows and relation-level
// proof contributions independently of individual tuple keys.
func (p *RelationExplanationProduct) Union(source, delta *RelationExplanationProduct) error {
	if err := p.compatible(source); err != nil {
		return err
	}
	if delta != nil {
		if delta == p || delta == source {
			return ast.ErrASTDomain
		}
		if err := p.compatible(delta); err != nil {
			return err
		}
	}
	for _, product := range []*RelationExplanationProduct{p, source, delta} {
		if product == nil {
			continue
		}
		defined, err := product.explanation.NoUndefined()
		if err != nil {
			return err
		}
		if !defined {
			return ast.ErrASTDomain
		}
		if err = product.requireCoherent(); err != nil {
			return err
		}
	}
	if p.numeric != nil {
		if source.numeric == nil || delta != nil && delta.numeric == nil {
			return ErrTableRelation
		}
		var deltaNumeric *relationExplanationNumeric
		if delta != nil {
			deltaNumeric = delta.numeric
		}
		if err := p.numeric.UnionFrom(source.numeric, deltaNumeric); err != nil {
			return err
		}
	} else if p.doc != nil {
		if source.doc == nil || delta != nil && delta.doc == nil {
			return ErrTableRelation
		}
		var deltaDoc *UDocTableProduct
		if delta != nil {
			deltaDoc = delta.doc
		}
		if err := p.doc.UnionFrom(source.doc, deltaDoc); err != nil {
			return err
		}
	} else {
		if source.table == nil || delta != nil && delta.table == nil {
			return ErrTableRelation
		}
		var deltaTable FiniteTable
		if delta != nil {
			deltaTable = delta.table
		}
		if err := relationExplanationFiniteUnion(p.table, source.table, deltaTable, p.finiteSorts); err != nil {
			return err
		}
	}
	var deltaExplanation *ExplanationRelation
	if delta != nil {
		deltaExplanation = delta.explanation
	}
	return p.explanation.Union(source.explanation, deltaExplanation, true)
}

func (p *RelationExplanationProduct) Close() (result error) {
	if p == nil || p.table == nil && p.doc == nil && p.numeric == nil {
		return nil
	}
	if p.explanation != nil {
		if err := p.explanation.Close(); err != nil {
			return err
		}
		p.explanation = nil
	}
	if p.doc != nil {
		if err := p.doc.Close(); err != nil {
			return err
		}
		p.doc = nil
	} else if p.numeric != nil {
		if err := p.numeric.Close(); err != nil {
			return err
		}
		p.numeric = nil
	} else if p.table != nil {
		if err := p.table.Close(); err != nil {
			return err
		}
		p.table = nil
	}
	p.manager, p.finiteSorts = nil, nil
	return nil
}
