package rel

// Source: dl_mk_similarity_compressor.cpp, with the concrete UDoc relation
// used for generated auxiliary facts.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// UDocSimilarityProgram owns the transformed rule set and its query-local
// auxiliary UDoc relations. The factory is borrowed from the active UDoc
// query owner.
type UDocSimilarityProgram struct {
	set       *base.RuleSet
	factory   *UDocRelationFactory
	seeds     map[ast.ASTHandle]*TypedUDocRelation
	products  map[ast.ASTHandle]*UDocTableProduct
	seedSorts map[ast.ASTHandle][]ast.ASTHandle
	order     []ast.ASTHandle
	modified  bool
	closed    bool
}

func NewUDocSimilarityProgram(manager *ast.ASTManager, relationManager *RelationManager, productHeads map[ast.ASTHandle]bool, rules *base.RuleManager, factory *UDocRelationFactory, source *base.RuleSet, threshold uint32, outputs []ast.ASTHandle) (_ *UDocSimilarityProgram, resultErr error) {
	if manager == nil || relationManager == nil || relationManager.manager != manager || rules == nil || factory == nil || factory.manager != manager || source == nil || threshold <= 1 {
		return nil, ast.ErrASTDomain
	}
	owner, err := source.RuleManager()
	if err != nil || owner != rules {
		return nil, errors.Join(err, ast.ErrASTDomain)
	}
	p := &UDocSimilarityProgram{factory: factory, seeds: make(map[ast.ASTHandle]*TypedUDocRelation), products: make(map[ast.ASTHandle]*UDocTableProduct), seedSorts: make(map[ast.ASTHandle][]ast.ASTHandle)}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, p.Close())
		}
	}()

	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	sourceRules := make([]*base.Rule, 0, count)
	defer func() {
		for i := len(sourceRules) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, sourceRules[i].Close())
		}
	}()
	for index := 0; index < count; index++ {
		rule, err := source.Rule(index)
		if err != nil {
			return nil, err
		}
		sourceRules = append(sourceRules, rule)
	}
	protected := make(map[ast.ASTHandle]bool, len(outputs))
	for _, output := range outputs {
		protected[output] = true
	}
	classes, duplicates, err := similarityClassesExcept(source, sourceRules, threshold, protected)
	if err != nil {
		return nil, err
	}
	p.modified = duplicates
	for _, class := range classes {
		p.modified = p.modified || class.merge
	}
	if !p.modified {
		return nil, nil
	}
	p.set, err = base.NewRuleSet(rules)
	if err != nil {
		return nil, err
	}
	for _, class := range classes {
		if !class.merge {
			for _, rule := range class.rules {
				if err = p.set.Add(rule); err != nil {
					return nil, err
				}
			}
			continue
		}
		head, err := class.rules[0].HeadDeclaration()
		if err != nil {
			return nil, err
		}
		_, productKind := productHeads[head]
		auxiliary, sorts, relation, product, err := mergeSimilarityClassUDoc(manager, relationManager, rules, factory, class, productKind)
		if err != nil {
			return nil, err
		}
		if relation != nil {
			p.seeds[auxiliary] = relation
		}
		if product != nil {
			p.products[auxiliary] = product
		}
		p.seedSorts[auxiliary] = sorts
		p.order = append(p.order, auxiliary)
		merged, err := similarityRewriteRule(rules, class.rules[0], class.constants, auxiliary)
		if err != nil {
			return nil, err
		}
		err = p.set.Add(merged)
		err = errors.Join(err, merged.Close())
		if err != nil {
			return nil, err
		}
	}
	if err = p.set.InheritPredicates(source); err != nil {
		return nil, err
	}
	return p, nil
}

func mergeSimilarityClassUDoc(manager *ast.ASTManager, relationManager *RelationManager, rules *base.RuleManager, factory *UDocRelationFactory, class similarityClass, productKind bool) (ast.ASTHandle, []ast.ASTHandle, *TypedUDocRelation, *UDocTableProduct, error) {
	if manager == nil || rules == nil || factory == nil || !class.merge || len(class.rules) < 2 || len(class.constants) == 0 {
		return ast.ASTHandle{}, nil, nil, nil, ast.ErrASTDomain
	}
	domains, rows, err := similarityOrphanRows(class.rules, class.constants)
	if err != nil {
		return ast.ASTHandle{}, nil, nil, nil, err
	}
	if productKind {
		if relationManager == nil {
			return ast.ASTHandle{}, nil, nil, nil, ast.ErrASTDomain
		}
		if _, err = tableRelationSignature(relationManager, domains); err != nil {
			return ast.ASTHandle{}, nil, nil, nil, err
		}
	} else {
		supported, cause := factory.CanHandleSignature(domains)
		if cause != nil || !supported {
			return ast.ASTHandle{}, nil, nil, nil, errors.Join(cause, ErrTableRelation)
		}
	}
	head, err := class.rules[0].HeadDeclaration()
	if err != nil {
		return ast.ASTHandle{}, nil, nil, nil, err
	}
	prefix, err := manager.GetASTName(head)
	if err != nil {
		return ast.ASTHandle{}, nil, nil, nil, err
	}
	suffix, err := manager.Symbols.Intern(fmt.Sprintf("sc_%d", len(class.constants)))
	if err != nil {
		return ast.ASTHandle{}, nil, nil, nil, err
	}
	boolean, err := manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, nil, nil, nil, err
	}
	auxiliary, err := manager.MkFreshFuncDecl(prefix, suffix, domains, boolean)
	if err != nil {
		return ast.ASTHandle{}, nil, nil, nil, err
	}
	if err = rules.RegisterPredicate(auxiliary); err != nil {
		return ast.ASTHandle{}, nil, nil, nil, err
	}
	if productKind {
		if relationManager == nil {
			return ast.ASTHandle{}, nil, nil, nil, ast.ErrASTDomain
		}
		product, err := NewEmptyUDocTableProduct(relationManager, factory, domains)
		if err != nil {
			return ast.ASTHandle{}, nil, nil, nil, err
		}
		for _, row := range rows {
			if err = product.AddFact(row); err != nil {
				return ast.ASTHandle{}, nil, nil, nil, errors.Join(err, product.Close())
			}
		}
		if err = relationManager.MarkSaturated(auxiliary); err != nil {
			return ast.ASTHandle{}, nil, nil, nil, errors.Join(err, product.Close())
		}
		return auxiliary, append([]ast.ASTHandle(nil), domains...), nil, product, nil
	}
	relation, err := factory.NewEmpty(domains)
	if err != nil {
		return ast.ASTHandle{}, nil, nil, nil, err
	}
	for _, row := range rows {
		if _, err = relation.AddFact(row); err != nil {
			return ast.ASTHandle{}, nil, nil, nil, errors.Join(err, relation.Close())
		}
	}
	if err = relationManager.MarkSaturated(auxiliary); err != nil {
		return ast.ASTHandle{}, nil, nil, nil, errors.Join(err, relation.Close())
	}
	return auxiliary, append([]ast.ASTHandle(nil), domains...), relation, nil, nil
}

func (p *UDocSimilarityProgram) RuleSet() (*base.RuleSet, error) {
	if p == nil || p.closed || p.set == nil {
		return nil, ast.ErrASTHandle
	}
	return p.set, nil
}

func (p *UDocSimilarityProgram) Modified() bool { return p != nil && p.modified }

func (p *UDocSimilarityProgram) Seeds() map[ast.ASTHandle]*TypedUDocRelation {
	result := make(map[ast.ASTHandle]*TypedUDocRelation, len(p.seeds))
	for declaration, relation := range p.seeds {
		result[declaration] = relation
	}
	return result
}

func (p *UDocSimilarityProgram) ProductSeeds() map[ast.ASTHandle]*UDocTableProduct {
	result := make(map[ast.ASTHandle]*UDocTableProduct, len(p.products))
	for declaration, product := range p.products {
		result[declaration] = product
	}
	return result
}

func (p *UDocSimilarityProgram) SeedOrder() []ast.ASTHandle {
	return append([]ast.ASTHandle(nil), p.order...)
}

func (p *UDocSimilarityProgram) SeedSorts() map[ast.ASTHandle][]ast.ASTHandle {
	result := make(map[ast.ASTHandle][]ast.ASTHandle, len(p.seedSorts))
	for declaration, sorts := range p.seedSorts {
		result[declaration] = append([]ast.ASTHandle(nil), sorts...)
	}
	return result
}

func (p *UDocSimilarityProgram) Close() (result error) {
	if p == nil || p.closed {
		return nil
	}
	for i := len(p.order) - 1; i >= 0; i-- {
		declaration := p.order[i]
		if relation := p.seeds[declaration]; relation != nil {
			result = errors.Join(result, relation.Close())
			if result == nil {
				delete(p.seeds, declaration)
			}
		}
		if product := p.products[declaration]; product != nil {
			result = errors.Join(result, product.Close())
			if result == nil {
				delete(p.products, declaration)
			}
		}
	}
	if p.set != nil {
		result = errors.Join(result, p.set.Close())
		if result == nil {
			p.set = nil
		}
	}
	if result == nil {
		p.factory, p.seeds, p.products, p.seedSorts, p.order, p.closed = nil, nil, nil, nil, nil, true
	}
	return result
}
