package rel

// Source: src/muz/rel/dl_finite_product_relation.cpp display_tuples,
// specialized to the non-table explanation inner relation.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// DisplayTuples follows the source's non-table-inner fallback: explanations
// cannot be enumerated as finite constants, so it writes the predicate heading
// and delegates to structural product display.
func (p *FiniteExplanationProduct) DisplayTuples(context *base.DatalogContext, predicate ast.ASTHandle, out io.Writer) error {
	if err := p.open(); err != nil {
		return err
	}
	if context == nil || out == nil {
		return ErrTableRelation
	}
	manager, err := context.Manager()
	if err != nil {
		return err
	}
	if manager != p.manager.manager {
		return ErrTableRelation
	}
	finite, explanations, err := p.manager.explanationPredicateSignature(predicate)
	if err != nil || explanations != p.explanationColumns || len(finite) != len(p.finiteSorts) {
		if err != nil {
			return err
		}
		return ErrTableRelation
	}
	for index := range finite {
		if finite[index] != p.finiteSorts[index] {
			return ErrTableRelation
		}
	}
	name, err := manager.GetASTName(predicate)
	if err != nil {
		return err
	}
	nameText, err := manager.Symbols.Display(name)
	if err != nil {
		return err
	}
	if _, err = fmt.Fprintf(out, "Tuples in %s: \n", nameText); err != nil {
		return err
	}
	return p.Display(out)
}
