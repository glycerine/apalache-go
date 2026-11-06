package rel

// The live mixed finite/doc interpreter only needs the source zero-column
// relation specialization here. Truth is stored in the concrete DOC owner;
// no finite shadow decides membership, additions or query results.

import "github.com/glycerine/g3/pkg/ast"

type UDocNullaryTable struct{ relation *TypedUDocRelation }

func NewUDocNullaryTable(factory *UDocRelationFactory) (*UDocNullaryTable, error) {
	if factory == nil {
		return nil, ErrDOC
	}
	r, err := factory.NewEmpty(nil)
	if err != nil {
		return nil, err
	}
	return &UDocNullaryTable{relation: r}, nil
}

func (t *UDocNullaryTable) Signature() (Signature, error) {
	if t == nil || t.relation == nil || t.relation.open() != nil {
		return Signature{}, ErrDOC
	}
	return Signature{}, nil
}
func (t *UDocNullaryTable) Facts() ([]Fact, error) {
	if _, err := t.Signature(); err != nil {
		return nil, err
	}
	empty, err := t.relation.Empty()
	if err != nil || empty {
		return nil, err
	}
	return []Fact{{}}, nil
}
func (t *UDocNullaryTable) Add(fact Fact) (bool, error) {
	if _, err := t.Signature(); err != nil {
		return false, err
	}
	if len(fact) != 0 {
		return false, ast.ErrASTDomain
	}
	return t.relation.AddFact(nil)
}
func (t *UDocNullaryTable) Contains(fact Fact) (bool, error) {
	if _, err := t.Signature(); err != nil {
		return false, err
	}
	if len(fact) != 0 {
		return false, ast.ErrASTDomain
	}
	return t.relation.ContainsFact(nil)
}
func (t *UDocNullaryTable) Len() (int, error) {
	if _, err := t.Signature(); err != nil {
		return 0, err
	}
	empty, err := t.relation.Empty()
	if err != nil || empty {
		return 0, err
	}
	return 1, nil
}
func (t *UDocNullaryTable) Reset() error {
	if _, err := t.Signature(); err != nil {
		return err
	}
	return t.relation.Reset()
}
func (t *UDocNullaryTable) Close() error {
	if t == nil || t.relation == nil {
		return nil
	}
	if err := t.relation.Close(); err != nil {
		return err
	}
	t.relation = nil
	return nil
}
func (t *UDocNullaryTable) CloneFinite() (FiniteTable, error) {
	if _, err := t.Signature(); err != nil {
		return nil, err
	}
	r, err := t.relation.Clone()
	if err != nil {
		return nil, err
	}
	return &UDocNullaryTable{relation: r}, nil
}

var _ FiniteTable = (*UDocNullaryTable)(nil)
