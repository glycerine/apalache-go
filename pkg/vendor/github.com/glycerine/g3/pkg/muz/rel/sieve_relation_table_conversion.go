package rel

// Source: src/muz/rel/dl_sieve_relation.cpp mk_from_inner and mixed operation
// lifecycle, specialized to the retained concrete TableRelation/HashTable.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

// NewSieveRelationFromTable clones an ordinary relation into an independently
// owned all-inner sieve. No outer coordinate is approximated away.
func NewSieveRelationFromTable(source *TableRelation) (*SieveRelation, error) {
	if source == nil || source.open() != nil {
		return nil, ErrSieve
	}
	signature, err := source.table.Signature()
	if err != nil {
		return nil, err
	}
	inner, err := source.table.Clone()
	if err != nil {
		return nil, err
	}
	mask := make([]bool, signature.Len())
	for i := range mask {
		mask[i] = true
	}
	result, err := AdoptSieveRelation(signature, mask, inner)
	if err != nil {
		_ = inner.Close()
		return nil, err
	}
	return result, nil
}

// TableRelationSnapshot clones an all-inner sieve into an ordinary relation.
// A partial sieve cannot be converted because enumerating its ignored full
// columns is a different, potentially expansive source operation.
func (r *SieveRelation) TableRelationSnapshot(manager *RelationManager, sorts []ast.ASTHandle) (*TableRelation, error) {
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
		return nil, fmt.Errorf("%w: conversion signatures differ", ErrSieve)
	}
	table, err := r.inner.Clone()
	if err != nil {
		return nil, err
	}
	result, err := AdoptTableRelation(manager, sorts, table)
	if err != nil {
		_ = table.Close()
		return nil, err
	}
	return result, nil
}
