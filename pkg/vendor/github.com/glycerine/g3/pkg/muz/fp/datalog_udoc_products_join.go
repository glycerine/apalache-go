package fp

// Source: dl_compiler.cpp joins two positive tail registers before head
// assembly. Product join matches their actual DOC and table components, or
// pairs one raw DOC input with one product input.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

type udocProductPairPlan struct {
	left, right               ast.ASTHandle
	leftColumns, rightColumns []uint32
	docTail                   int
}

func (e *PropositionalEngine) planUDocProductPair(program *udocProductProgram, rule *base.Rule, docTail int) (*udocProductRulePlan, error) {
	var declarations [2]ast.ASTHandle
	var arguments [2][]ast.ASTHandle
	ready := true
	for i := range declarations {
		tail, negated, err := rule.Tail(i)
		if err != nil {
			return nil, err
		}
		if negated {
			return nil, ErrUDocDatalogFragment
		}
		declarations[i], err = e.manager.GetDecl(tail)
		if err != nil {
			return nil, err
		}
		if i == docTail {
			if !program.docs[declarations[i]] {
				return nil, ErrUDocDatalogFragment
			}
		} else if !program.productHead(declarations[i]) {
			if program.rules[declarations[i]] != nil || program.docs[declarations[i]] || program.finite[declarations[i]] {
				return nil, ErrUDocDatalogFragment
			}
			ready = false
		}
		arguments[i], err = productArguments(e.manager, tail)
		if err != nil {
			return nil, err
		}
		seen := make(map[ast.ASTHandle]bool)
		for _, argument := range arguments[i] {
			kind, err := e.manager.GetKind(argument)
			if err != nil {
				return nil, err
			}
			if kind == ast.ASTVar {
				if seen[argument] {
					return nil, ErrUDocDatalogFragment
				}
				seen[argument] = true
				continue
			}
			value, err := e.manager.IsValue(argument)
			if err != nil {
				return nil, err
			}
			if !value {
				return nil, ErrUDocDatalogFragment
			}
		}
	}
	if !ready {
		return nil, nil
	}
	head, err := rule.Head()
	if err != nil {
		return nil, err
	}
	declaration, err := rule.HeadDeclaration()
	if err != nil {
		return nil, err
	}
	pair := &udocProductPairPlan{left: declarations[0], right: declarations[1], docTail: docTail}
	for i, left := range arguments[0] {
		kind, err := e.manager.GetKind(left)
		if err != nil {
			return nil, err
		}
		// Source variable_intersection only matches variables. Identical
		// constants in different tails are separate post-join filters.
		if kind != ast.ASTVar {
			continue
		}
		for j, right := range arguments[1] {
			if left == right {
				pair.leftColumns = append(pair.leftColumns, uint32(i))
				pair.rightColumns = append(pair.rightColumns, uint32(j))
			}
		}
	}
	plan := &udocProductRulePlan{head: declaration, pair: pair}
	joined := append(append([]ast.ASTHandle(nil), arguments[0]...), arguments[1]...)
	if err = planUDocProductAssembly(e.manager, plan, joined, head, true); err != nil {
		return nil, err
	}
	// Same-tail repeats remain outside this slice. Every remaining repeated
	// variable crosses the two tails, so source joins already enforce its
	// identity and the compiler skips the later FilterIdentical operation.
	plan.identical = nil
	return plan, nil
}

func (p *udocProductContext) productPairInput(declaration ast.ASTHandle, finite map[ast.ASTHandle]rel.FiniteTable) (*rel.UDocTableProduct, bool, error) {
	if product := p.result[declaration]; product != nil {
		empty, err := product.Empty()
		return product, empty, err
	}
	if ordinary := finite[declaration]; ordinary != nil {
		rows, err := ordinary.Len()
		return nil, rows == 0, err
	}
	return nil, true, nil
}

func (p *udocProductContext) multiInputEmpty(declaration ast.ASTHandle, kind uint8, docs map[ast.ASTHandle]*rel.TypedUDocRelation, finite map[ast.ASTHandle]rel.FiniteTable) (bool, error) {
	switch kind {
	case 0:
		product := p.result[declaration]
		if product == nil {
			return true, nil
		}
		return product.Empty()
	case 1:
		document := docs[declaration]
		if document == nil {
			return true, nil
		}
		return document.Empty()
	case 2:
		table := finite[declaration]
		if table == nil {
			return true, nil
		}
		rows, err := table.Len()
		return rows == 0, err
	default:
		return false, ErrUDocDatalogFragment
	}
}

func (p *udocProductContext) joinMultiProductInput(left *rel.UDocTableProduct, right udocProductMultiInput, leftColumns, rightColumns []uint32, docs map[ast.ASTHandle]*rel.TypedUDocRelation, finite map[ast.ASTHandle]rel.FiniteTable) (*rel.UDocTableProduct, error) {
	switch right.kind {
	case 0:
		product := p.result[right.declaration]
		if product == nil {
			return nil, nil
		}
		return left.Join(product, leftColumns, rightColumns)
	case 1:
		document := docs[right.declaration]
		if document == nil {
			return nil, nil
		}
		return rel.JoinProductUDoc(left, document, leftColumns, rightColumns)
	case 2:
		table := finite[right.declaration]
		if table == nil {
			return nil, nil
		}
		return rel.JoinProductFinite(left, right.sorts, table, leftColumns, rightColumns)
	default:
		return nil, ErrUDocDatalogFragment
	}
}

func (p *udocProductContext) joinMultiFirst(first ast.ASTHandle, firstKind uint8, second udocProductMultiInput, leftColumns, rightColumns []uint32, docs map[ast.ASTHandle]*rel.TypedUDocRelation, finite map[ast.ASTHandle]rel.FiniteTable) (*rel.UDocTableProduct, error) {
	switch firstKind {
	case 0:
		product := p.result[first]
		if product == nil {
			return nil, nil
		}
		return p.joinMultiProductInput(product, second, leftColumns, rightColumns, docs, finite)
	case 1:
		document := docs[first]
		if document == nil {
			return nil, nil
		}
		switch second.kind {
		case 0:
			product := p.result[second.declaration]
			if product == nil {
				return nil, nil
			}
			return rel.JoinUDocProduct(document, product, leftColumns, rightColumns)
		case 2:
			table := finite[second.declaration]
			if table == nil {
				return nil, nil
			}
			return rel.JoinUDocFinite(p.manager, document, second.sorts, table, leftColumns, rightColumns)
		case 1:
			other := docs[second.declaration]
			if other == nil {
				return nil, nil
			}
			return rel.JoinUDocRelations(p.manager, document, other, leftColumns, rightColumns)
		}
	case 2:
		table := finite[first]
		if table == nil {
			return nil, nil
		}
		sorts := p.program.finiteSorts[first]
		switch second.kind {
		case 0:
			product := p.result[second.declaration]
			if product == nil {
				return nil, nil
			}
			return rel.JoinFiniteProduct(p.manager, sorts, table, product, leftColumns, rightColumns)
		case 1:
			document := docs[second.declaration]
			if document == nil {
				return nil, nil
			}
			return rel.JoinFiniteUDoc(p.manager, sorts, table, document, leftColumns, rightColumns)
		case 2:
			other := finite[second.declaration]
			if other == nil {
				return nil, nil
			}
			return rel.JoinFiniteTables(p.manager, p.factory, sorts, table, second.sorts, other, leftColumns, rightColumns)
		}
	}
	return nil, ErrUDocDatalogFragment
}

func (p *udocProductContext) evaluateProductPair(pair *udocProductPairPlan, docs map[ast.ASTHandle]*rel.TypedUDocRelation, finite map[ast.ASTHandle]rel.FiniteTable) (*rel.UDocTableProduct, error) {
	if pair.docTail == 0 {
		document := docs[pair.left]
		product := p.result[pair.right]
		if document == nil || product == nil {
			return nil, nil
		}
		empty, err := document.Empty()
		if err != nil || empty {
			return nil, err
		}
		empty, err = product.Empty()
		if err != nil || empty {
			return nil, err
		}
		return rel.JoinUDocProduct(document, product, pair.leftColumns, pair.rightColumns)
	}
	if pair.docTail == 1 {
		product := p.result[pair.left]
		document := docs[pair.right]
		if document == nil || product == nil {
			return nil, nil
		}
		empty, err := document.Empty()
		if err != nil || empty {
			return nil, err
		}
		empty, err = product.Empty()
		if err != nil || empty {
			return nil, err
		}
		return rel.JoinProductUDoc(product, document, pair.leftColumns, pair.rightColumns)
	}
	left, leftEmpty, err := p.productPairInput(pair.left, finite)
	if err != nil {
		return nil, err
	}
	right, rightEmpty, err := p.productPairInput(pair.right, finite)
	if err != nil {
		return nil, err
	}
	// A source join with an absent/empty input register performs no join.
	if leftEmpty || rightEmpty {
		return nil, nil
	}
	// An earlier empty query did not infer product kind. A later API fact can
	// therefore populate ordinary finite storage. That is a real nonempty
	// input, but mixed table/product dispatch is outside this paired slice.
	if left == nil || right == nil {
		return nil, fmt.Errorf("%w: two-product join has a nonempty ordinary finite input", ErrUDocDatalogFragment)
	}
	return left.Join(right, pair.leftColumns, pair.rightColumns)
}
