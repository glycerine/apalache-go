package rel

// Source: dl_mk_similarity_compressor.cpp operator()/process_class and
// dl_mk_similarity_compressor.h. This first integration uses finite tables.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// FiniteSimilarityCompressor is the finite-table specialization of the
// similarity transformation. It borrows its DatalogContext and relation
// manager. Unsupported fact domains return an error rather than being skipped.
type FiniteSimilarityCompressor struct {
	self      *FiniteSimilarityCompressor
	context   *base.DatalogContext
	rules     *base.RuleManager
	relations *RelationManager
	fresh     []ast.ASTHandle
	origins   map[ast.ASTHandle]ast.ASTHandle
	requested map[ast.ASTHandle]bool
	protected map[ast.ASTHandle]bool
	threshold uint32
	closed    bool
}

func NewFiniteSimilarityCompressor(context *base.DatalogContext, relations *RelationManager, threshold uint32) (*FiniteSimilarityCompressor, error) {
	if context == nil || relations == nil || threshold <= 1 {
		return nil, ast.ErrASTDomain
	}
	m, e := context.Manager()
	if e != nil {
		return nil, e
	}
	if e = relations.open(); e != nil {
		return nil, e
	}
	if relations.manager != m {
		return nil, ast.ErrASTDomain
	}
	rm, e := context.RuleManager()
	if e != nil {
		return nil, e
	}
	p := &FiniteSimilarityCompressor{context: context, rules: rm, relations: relations, threshold: threshold}
	p.self = p
	return p, nil
}

// NewFiniteSimilarityCompressorForRules binds the finite relational engine's
// concrete rule manager directly; the relation manager owns generated facts.
func NewFiniteSimilarityCompressorForRules(rules *base.RuleManager, relations *RelationManager, threshold uint32, outputs []ast.ASTHandle) (*FiniteSimilarityCompressor, error) {
	return NewFiniteSimilarityCompressorForRulesWithRequestedKinds(rules, relations, threshold, outputs, nil)
}

// NewFiniteSimilarityCompressorForRulesWithRequestedKinds inherits the actual
// finite plugin of explicitly selected source predicates. Unselected heads
// continue to use the current favourite for their generated auxiliary table.
func NewFiniteSimilarityCompressorForRulesWithRequestedKinds(rules *base.RuleManager, relations *RelationManager, threshold uint32, outputs, requested []ast.ASTHandle) (*FiniteSimilarityCompressor, error) {
	if rules == nil || relations == nil || threshold <= 1 {
		return nil, ast.ErrASTDomain
	}
	if err := relations.open(); err != nil {
		return nil, err
	}
	p := &FiniteSimilarityCompressor{rules: rules, relations: relations, threshold: threshold}
	p.requested = make(map[ast.ASTHandle]bool, len(requested))
	for _, declaration := range requested {
		p.requested[declaration] = true
	}
	p.protected = make(map[ast.ASTHandle]bool, len(outputs))
	for _, output := range outputs {
		p.protected[output] = true
	}
	p.self = p
	return p, nil
}

// FreshPredicates returns borrowed declarations owned by the relation manager.
func (p *FiniteSimilarityCompressor) FreshPredicates() []ast.ASTHandle {
	if p == nil {
		return nil
	}
	return append([]ast.ASTHandle(nil), p.fresh...)
}

// FreshPredicateOrigin returns the source head whose requested relation kind
// a generated predicate inherits. Both declarations are borrowed.
func (p *FiniteSimilarityCompressor) FreshPredicateOrigin(predicate ast.ASTHandle) (ast.ASTHandle, bool) {
	if p == nil {
		return ast.ASTHandle{}, false
	}
	origin, found := p.origins[predicate]
	return origin, found
}
func (p *FiniteSimilarityCompressor) open() error {
	if p == nil || p.self != p || p.closed {
		return ast.ErrASTHandle
	}
	if p.context != nil {
		if _, e := p.context.Manager(); e != nil {
			return e
		}
	}
	return p.relations.open()
}
func (p *FiniteSimilarityCompressor) Priority() uint32            { return 5000 }
func (p *FiniteSimilarityCompressor) CanDestratifyNegation() bool { return false }
func (p *FiniteSimilarityCompressor) Cancel()                     {}
func (p *FiniteSimilarityCompressor) Close() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	p.closed = true
	return nil
}

// Transform returns a new rule set only when duplicates are removed or a
// class is compressed. Side effects on generated predicates and relations
// occur in the same class order as the source.
func (p *FiniteSimilarityCompressor) Transform(source *base.RuleSet) (result *base.RuleSet, err error) {
	if err = p.open(); err != nil {
		return nil, err
	}
	sourceRM, e := source.RuleManager()
	if e != nil {
		return nil, e
	}
	rm := p.rules
	if sourceRM != rm {
		return nil, ast.ErrASTDomain
	}
	n, e := source.Len()
	if e != nil {
		return nil, e
	}
	var rules []*base.Rule
	defer func() {
		for i := len(rules) - 1; i >= 0; i-- {
			err = errors.Join(err, rules[i].Close())
		}
		if err != nil && result != nil {
			err = errors.Join(err, result.Close())
			result = nil
		}
	}()
	for i := 0; i < n; i++ {
		r, e := source.Rule(i)
		if e != nil {
			return nil, e
		}
		rules = append(rules, r)
	}
	classes, changed, e := similarityClassesExcept(source, rules, p.threshold, p.protected)
	if e != nil {
		return nil, e
	}
	for _, class := range classes {
		if class.merge {
			changed = true
			break
		}
	}
	if !changed {
		return nil, nil
	}
	result, e = base.NewRuleSet(rm)
	if e != nil {
		return nil, e
	}
	for _, class := range classes {
		if class.merge {
			head, e := class.rules[0].HeadDeclaration()
			if e != nil {
				return result, e
			}
			var template FiniteTable
			if p.requested[head] {
				template, e = p.relations.GetFiniteRelation(head)
				if e != nil {
					return result, e
				}
			}
			aux, merged, e := similarityMergeFiniteCoreWithTemplate(p.context, rm, p.relations, class, template)
			if e != nil {
				return result, e
			}
			p.fresh = append(p.fresh, aux)
			if p.origins == nil {
				p.origins = make(map[ast.ASTHandle]ast.ASTHandle)
			}
			p.origins[aux] = head
			e = result.Add(merged)
			e = errors.Join(e, merged.Close())
			if e != nil {
				return result, e
			}
		} else {
			for _, r := range class.rules {
				if e = result.Add(r); e != nil {
					return result, e
				}
			}
		}
	}
	if e = result.InheritPredicates(source); e != nil {
		return result, e
	}
	return result, nil
}
