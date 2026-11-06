package rel

// Source: dl_finite_product_relation.cpp converting_join_fn converts finite
// relations to products with a full zero-column inner explanation relation.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

type explanationFiniteLoadInstruction struct {
	*relationIOInstruction
	source uint32
}

// The optional source is a finite old-delta register; VoidRegister loads the
// predicate's complete manager-owned finite relation. Both paths copy rows.
func newExplanationFiniteLoadInstruction(relations *RelationManager, predicate ast.ASTHandle, source, target uint32) (Instruction, error) {
	owner, err := newRelationIOInstruction(relations, predicate, target, false)
	if err != nil {
		return nil, err
	}
	load := owner.(*relationIOInstruction)
	if load.explanation {
		return nil, errors.Join(ErrInstruction, load.Close())
	}
	return &explanationFiniteLoadInstruction{relationIOInstruction: load, source: source}, nil
}

func (i *explanationFiniteLoadInstruction) Perform(context *ExecutionContext) (bool, error) {
	if err := i.open(); err != nil {
		return false, err
	}
	relations, err := i.contextRelations(context)
	if err != nil {
		return false, err
	}
	sorts, _, err := relations.PredicateSignature(i.predicate)
	if err != nil {
		return false, err
	}
	var table FiniteTable
	if i.source == VoidRegister {
		table, err = relations.GetFiniteRelation(i.predicate)
	} else {
		table, err = context.FiniteRegister(i.source)
	}
	if err != nil {
		return false, err
	}
	if table == nil {
		return true, context.MakeEmpty(i.register)
	}
	count, err := table.Len()
	if err != nil {
		return false, err
	}
	if count == 0 {
		return true, context.MakeEmpty(i.register)
	}
	product, err := finiteExplanationProductFromFiniteTable(relations, sorts, table)
	if err != nil {
		return false, err
	}
	if err = context.SetExplanationRegister(i.register, product); err != nil {
		return false, errors.Join(err, product.Close())
	}
	return true, nil
}

func (i *explanationFiniteLoadInstruction) DisplayHead(out io.Writer) error {
	if err := i.open(); err != nil || out == nil {
		return errors.Join(ErrInstruction, err)
	}
	name, err := i.predicateName()
	if err != nil {
		return err
	}
	if i.source == VoidRegister {
		_, err = fmt.Fprintf(out, "load_finite_product %s into %d", name, i.register)
	} else {
		_, err = fmt.Fprintf(out, "copy_finite_product %d into %d", i.source, i.register)
	}
	return err
}
