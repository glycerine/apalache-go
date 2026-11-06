package fp

// Source: acyclic compiler components execute in dependency order. Missing
// target registers acquire the incoming product kind; original seed owners
// remain separate from each query's actual product result snapshot.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func (p *udocProductContext) evaluate(plan *udocProductRulePlan, docs map[ast.ASTHandle]*rel.TypedUDocRelation, finite map[ast.ASTHandle]rel.FiniteTable) (resultErr error) {
	a := p.attempt()
	defer func() { resultErr = p.finish(a, resultErr) }()
	var working *rel.UDocTableProduct
	var err error
	if !plan.multiStart.IsNull() {
		first, second := plan.multiStart, plan.multi[0]
		firstEmpty, cause := p.multiInputEmpty(first, plan.multiStartKind, docs, finite)
		if cause != nil || firstEmpty {
			return cause
		}
		secondEmpty, cause := p.multiInputEmpty(second.declaration, second.kind, docs, finite)
		if cause != nil || secondEmpty {
			return cause
		}
		working, err = p.joinMultiFirst(first, plan.multiStartKind, second, second.leftColumns, second.rightColumns, docs, finite)
		if working != nil {
			a.values = append(a.values, working)
		}
		if err != nil || working == nil {
			return err
		}
		for _, input := range plan.multi[1:] {
			empty, cause := working.Empty()
			if cause != nil || empty {
				return cause
			}
			empty, cause = p.multiInputEmpty(input.declaration, input.kind, docs, finite)
			if cause != nil || empty {
				return cause
			}
			next, cause := p.joinMultiProductInput(working, input, input.leftColumns, input.rightColumns, docs, finite)
			if next != nil {
				a.values = append(a.values, next)
			}
			if cause != nil {
				return cause
			}
			working = next
		}
	} else if plan.pair != nil {
		working, err = p.evaluateProductPair(plan.pair, docs, finite)
		if err == nil && working == nil {
			return nil
		}
	} else if !plan.source.IsNull() {
		source := p.result[plan.source]
		if source == nil {
			return nil
		}
		working, err = source.Clone()
	} else {
		doc, table := docs[plan.doc], finite[plan.finite]
		if doc == nil {
			return ErrUDocDatalogFragment
		}
		empty, emptyErr := doc.Empty()
		if emptyErr != nil || empty {
			return emptyErr
		}
		if _, mixed := p.program.groundProducts[plan.finite]; mixed {
			input := p.result[plan.finite]
			if input == nil {
				return nil
			}
			if plan.docFirst {
				working, err = rel.JoinUDocProduct(doc, input, plan.docColumns, plan.finiteColumns)
			} else {
				working, err = rel.JoinProductUDoc(input, doc, plan.finiteColumns, plan.docColumns)
			}
		} else {
			if table == nil {
				return ErrUDocDatalogFragment
			}
			rows, rowsErr := table.Len()
			if rowsErr != nil || rows == 0 {
				return rowsErr
			}
			inputSorts := p.program.finiteSorts[plan.finite]
			if plan.docFirst {
				working, err = rel.JoinUDocFinite(p.manager, doc, inputSorts, table, plan.docColumns, plan.finiteColumns)
			} else {
				working, err = rel.JoinFiniteUDoc(p.manager, inputSorts, table, doc, plan.finiteColumns, plan.docColumns)
			}
		}
	}
	if working != nil {
		a.values = append(a.values, working)
	}
	if err != nil {
		return err
	}
	for _, filter := range plan.filters {
		if err = working.FilterEqual(filter.column, filter.value); err != nil {
			return err
		}
	}
	for _, columns := range plan.identical {
		if err = working.FilterIdentical(columns); err != nil {
			return err
		}
	}
	for _, membership := range plan.docMemberships {
		var next *rel.UDocTableProduct
		if membership.product {
			auxiliary := p.result[membership.declaration]
			if auxiliary == nil {
				return ErrUDocDatalogFragment
			}
			next, err = working.Join(auxiliary, membership.productColumns, membership.docColumns)
		} else {
			document := docs[membership.declaration]
			if document == nil {
				return ErrUDocDatalogFragment
			}
			next, err = rel.JoinProductUDoc(working, document, membership.productColumns, membership.docColumns)
		}
		if next != nil {
			a.values = append(a.values, next)
		}
		if err != nil {
			return err
		}
		working = next
	}
	empty, err := working.Empty()
	if err != nil || empty {
		return err
	}
	if len(plan.removed) != 0 {
		next, err := working.Project(plan.removed)
		if next != nil {
			a.values = append(a.values, next)
		}
		if err != nil {
			return err
		}
		working = next
	}
	for _, cycle := range plan.cycles {
		next, err := working.Rename(cycle)
		if next != nil {
			a.values = append(a.values, next)
		}
		if err != nil {
			return err
		}
		working = next
	}
	return p.unionResult(plan.head, working, a, finite[plan.head])
}

// Source instr_union infers an absent target from the incoming concrete kind.
func (p *udocProductContext) unionResult(head ast.ASTHandle, working *rel.UDocTableProduct, a *udocProductAttempt, ordinary rel.FiniteTable) error {
	var err error
	target := p.result[head]
	if target == nil && ordinary != nil {
		count, cause := ordinary.Len()
		if cause != nil {
			return cause
		}
		// Source cannot union a two-component product into a nonempty raw
		// table. Never silently upgrade an ordinary head with seed facts.
		if count != 0 {
			return ErrUDocDatalogFragment
		}
	}
	if p.seeds[head] == nil {
		// Source relation kind survives invalidation caused by another
		// predicate's mutation, even before this head has explicit seed facts.
		seed, cause := working.EmptyLike()
		if seed != nil {
			p.seeds[head] = seed
		}
		if cause != nil {
			return cause
		}
	}
	if target == nil {
		target, err = working.EmptyLike()
		if target != nil {
			p.result[head] = target
		}
		if err != nil {
			return err
		}
	}
	delta, err := working.EmptyLike()
	if delta != nil {
		a.values = append(a.values, delta)
	}
	if err != nil {
		return err
	}
	// Keep the source componentwise delta, including its deliberately coarse
	// Empty result. Acyclic dependency order requires no fixed-point shortcut.
	return target.UnionFrom(working, delta)
}
