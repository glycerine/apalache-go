package fp

// Source: dl_compiler.cpp assembles zero-tail ground heads from constant
// singleton registers before unioning them into their predicate. The singleton
// cache is keyed by (sort, value), not predicate kind. Unary DOC and all-Bool
// table heads cannot share a singleton across different relation kinds, so
// their exact singleton unions reduce to concrete insertions. Mixed Bool/BV
// heads execute the separate singleton-product assembly before component union.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

type udocProductGroundRule struct {
	head   ast.ASTHandle
	values []ast.ASTHandle // Borrowed from the query's owning rule set.
}

func udocProductGroundFact(m *ast.ASTManager, rule *base.Rule) (*udocProductGroundRule, error) {
	tails, err := rule.TailSize()
	if err != nil || tails != 0 {
		return nil, err
	}
	head, err := rule.Head()
	if err != nil {
		return nil, err
	}
	args, err := productArguments(m, head)
	if err != nil {
		return nil, err
	}
	for _, arg := range args {
		value, err := m.IsValue(arg)
		if err != nil || !value {
			return nil, err
		}
	}
	declaration, err := rule.HeadDeclaration()
	if err != nil {
		return nil, err
	}
	return &udocProductGroundRule{head: declaration, values: args}, nil
}

func (c *datalogUDocContext) evaluateProductGroundFacts(facts []*udocProductGroundRule) error {
	for _, fact := range facts {
		if err := c.checkpoint(); err != nil {
			return err
		}
		if column, mixed := c.products.program.groundProducts[fact.head]; mixed {
			if err := c.evaluateProductMixedGround(fact, column); err != nil {
				return err
			}
			continue
		}
		if c.products.program.docs[fact.head] {
			if _, err := c.result[fact.head].AddFact(fact.values); err != nil {
				return err
			}
			continue
		}
		values, err := c.relations.EncodeFact(c.products.program.finiteSorts[fact.head], fact.values)
		if err != nil {
			return err
		}
		if _, err = c.finiteResult[fact.head].Add(values); err != nil {
			return err
		}
	}
	return nil
}
