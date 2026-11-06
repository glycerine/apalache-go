package rel

// Source: src/muz/rel/dl_sieve_relation.cpp
// sieve_relation_plugin::mk_from_inner ownership transfer.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

// TakeSieveRelationFromTable transfers the ordinary relation's concrete table
// into an all-inner sieve. The source is consumed only after validation and
// successful result construction.
func TakeSieveRelationFromTable(source *TableRelation) (*SieveRelation, error) {
	if source == nil || source.open() != nil {
		return nil, ErrSieve
	}
	signature, err := source.table.Signature()
	if err != nil {
		return nil, err
	}
	mask := make([]bool, signature.Len())
	for i := range mask {
		mask[i] = true
	}
	result, err := AdoptSieveRelation(signature, mask, source.table)
	if err != nil {
		return nil, err
	}
	source.closed = true
	source.manager = nil
	source.sorts = nil
	source.table = nil
	return result, nil
}

// TakeTableRelation transfers an all-inner sieve's concrete table into an
// ordinary relation. Partial sieves are refused without consuming the source.
func (r *SieveRelation) TakeTableRelation(manager *RelationManager, sorts []ast.ASTHandle) (*TableRelation, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	if manager == nil || manager.open() != nil || !r.NoSievedColumns() {
		return nil, ErrSieve
	}
	signature, err := tableRelationSignature(manager, sorts)
	if err != nil {
		return nil, err
	}
	if !r.signature.equal(signature) {
		return nil, fmt.Errorf("%w: transfer signatures differ", ErrSieve)
	}
	result, err := AdoptTableRelation(manager, sorts, r.inner)
	if err != nil {
		return nil, err
	}
	r.closed = true
	r.inner = nil
	r.innerCols = nil
	r.sigToInner = nil
	r.innerToSig = nil
	r.ignored = nil
	return result, nil
}
