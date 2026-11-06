package rel

// Source: dl_mk_similarity_compressor.cpp merge_class/operator, with concrete
// interval, bound, and Karr auxiliary relation implementations.

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// arithmeticSimilarityProgram owns the common rule-construction lifetime of
// the two live arithmetic relation specializations. Its private rule manager
// keeps generated predicates out of the original registration state.
type arithmeticSimilarityProgram struct {
	manager     *ast.ASTManager
	rules       *base.RuleManager
	set         *base.RuleSet
	sourceRules []*base.Rule
	pending     []*base.Rule
	refs        []ast.ASTRef
	auxiliary   []ast.ASTHandle
	seeds       map[ast.ASTHandle]*IntervalRelation
	boundSeeds  map[ast.ASTHandle]*BoundRelation
	karrSeeds   map[ast.ASTHandle]*KarrRelation
	origins     map[ast.ASTHandle]ast.ASTHandle
	modified    bool
	closed      bool
}

type IntervalSimilarityProgram struct{ *arithmeticSimilarityProgram }
type BoundSimilarityProgram struct{ *arithmeticSimilarityProgram }
type KarrSimilarityProgram struct{ *arithmeticSimilarityProgram }

type similarityBackend uint8

const (
	similarityIntervalBackend similarityBackend = iota
	similarityBoundBackend
	similarityKarrBackend
)

func (p *IntervalSimilarityProgram) Rules() (*base.RuleSet, error) {
	if p == nil {
		return nil, ast.ErrASTHandle
	}
	return p.arithmeticSimilarityProgram.Rules()
}
func (p *BoundSimilarityProgram) Rules() (*base.RuleSet, error) {
	if p == nil {
		return nil, ast.ErrASTHandle
	}
	return p.arithmeticSimilarityProgram.Rules()
}
func (p *IntervalSimilarityProgram) AuxiliaryPredicates() []ast.ASTHandle {
	if p == nil {
		return nil
	}
	return p.arithmeticSimilarityProgram.AuxiliaryPredicates()
}
func (p *BoundSimilarityProgram) AuxiliaryPredicates() []ast.ASTHandle {
	if p == nil {
		return nil
	}
	return p.arithmeticSimilarityProgram.AuxiliaryPredicates()
}
func (p *IntervalSimilarityProgram) Modified() bool {
	return p != nil && p.arithmeticSimilarityProgram.Modified()
}
func (p *BoundSimilarityProgram) Modified() bool {
	return p != nil && p.arithmeticSimilarityProgram.Modified()
}
func (p *IntervalSimilarityProgram) Close() error {
	if p == nil {
		return nil
	}
	return p.arithmeticSimilarityProgram.Close()
}
func (p *BoundSimilarityProgram) Close() error {
	if p == nil {
		return nil
	}
	return p.arithmeticSimilarityProgram.Close()
}

func (p *KarrSimilarityProgram) Rules() (*base.RuleSet, error) {
	if p == nil {
		return nil, ast.ErrASTHandle
	}
	return p.arithmeticSimilarityProgram.Rules()
}
func (p *KarrSimilarityProgram) AuxiliaryPredicates() []ast.ASTHandle {
	if p == nil {
		return nil
	}
	return p.arithmeticSimilarityProgram.AuxiliaryPredicates()
}
func (p *KarrSimilarityProgram) Modified() bool {
	return p != nil && p.arithmeticSimilarityProgram.Modified()
}
func (p *KarrSimilarityProgram) Close() error {
	if p == nil {
		return nil
	}
	return p.arithmeticSimilarityProgram.Close()
}

func NewIntervalSimilarityProgram(manager *ast.ASTManager, source *base.RuleSet, threshold uint32, outputs []ast.ASTHandle) (*IntervalSimilarityProgram, error) {
	p, err := newArithmeticSimilarityProgram(manager, source, threshold, outputs, similarityIntervalBackend)
	if err != nil {
		return nil, err
	}
	return &IntervalSimilarityProgram{p}, nil
}

func NewBoundSimilarityProgram(manager *ast.ASTManager, source *base.RuleSet, threshold uint32, outputs []ast.ASTHandle) (*BoundSimilarityProgram, error) {
	p, err := newArithmeticSimilarityProgram(manager, source, threshold, outputs, similarityBoundBackend)
	if err != nil {
		return nil, err
	}
	return &BoundSimilarityProgram{p}, nil
}

// NewKarrSimilarityProgram applies the source similarity rewrite while
// building its auxiliary fact relation with Karr's concrete source semantics.
func NewKarrSimilarityProgram(manager *ast.ASTManager, source *base.RuleSet, threshold uint32, outputs []ast.ASTHandle) (*KarrSimilarityProgram, error) {
	p, err := newArithmeticSimilarityProgram(manager, source, threshold, outputs, similarityKarrBackend)
	if err != nil {
		return nil, err
	}
	return &KarrSimilarityProgram{p}, nil
}

func newArithmeticSimilarityProgram(manager *ast.ASTManager, source *base.RuleSet, threshold uint32, outputs []ast.ASTHandle, backend similarityBackend) (_ *arithmeticSimilarityProgram, resultErr error) {
	if manager == nil || source == nil || threshold <= 1 {
		return nil, ast.ErrASTDomain
	}
	p := &arithmeticSimilarityProgram{manager: manager, origins: make(map[ast.ASTHandle]ast.ASTHandle)}
	if backend == similarityBoundBackend {
		p.boundSeeds = make(map[ast.ASTHandle]*BoundRelation)
	} else if backend == similarityIntervalBackend {
		p.seeds = make(map[ast.ASTHandle]*IntervalRelation)
	} else {
		p.karrSeeds = make(map[ast.ASTHandle]*KarrRelation)
	}
	defer func() {
		if resultErr != nil {
			resultErr = finishIntervalSimilarityConstruction(resultErr, p)
		}
	}()
	var err error
	p.rules, err = base.NewRuleManager(manager)
	if err != nil {
		return nil, err
	}
	p.set, err = base.NewRuleSet(p.rules)
	if err != nil {
		return nil, err
	}
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	for index := 0; index < count; index++ {
		rule, err := source.Rule(index)
		if err != nil {
			return nil, err
		}
		p.sourceRules = append(p.sourceRules, rule)
		owner, err := rule.ASTManager()
		if err != nil || owner != manager {
			return nil, errors.Join(err, ast.ErrASTDomain)
		}
		head, err := rule.HeadDeclaration()
		if err != nil {
			return nil, err
		}
		if err = p.rules.RegisterPredicate(head); err != nil {
			return nil, err
		}
		tails, err := rule.UninterpretedTailSize()
		if err != nil {
			return nil, err
		}
		for column := 0; column < tails; column++ {
			declaration, err := rule.TailDeclaration(column)
			if err != nil {
				return nil, err
			}
			if err = p.rules.RegisterPredicate(declaration); err != nil {
				return nil, err
			}
		}
	}
	protected := make(map[ast.ASTHandle]bool, len(outputs))
	for _, output := range outputs {
		protected[output] = true
		if err = p.rules.RegisterPredicate(output); err != nil {
			return nil, err
		}
		if err = p.set.SetOutputPredicate(output); err != nil {
			return nil, err
		}
	}
	originalOutputs, err := source.OutputPredicates()
	if err != nil {
		return nil, err
	}
	for _, output := range originalOutputs {
		if err = p.rules.RegisterPredicate(output); err != nil {
			return nil, err
		}
		if err = p.set.SetOutputPredicate(output); err != nil {
			return nil, err
		}
	}
	classes, modified, err := similarityClassesExcept(source, p.sourceRules, threshold, protected)
	if err != nil {
		return nil, err
	}
	p.modified = modified
	for _, class := range classes {
		p.modified = p.modified || class.merge
	}
	if !p.modified {
		// The source returns nullptr when no transformation occurred. Its
		// comparator order must not affect subsequent interval widening.
		for _, rule := range p.sourceRules {
			if err = p.set.Add(rule); err != nil {
				return nil, err
			}
		}
		return p, nil
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
		if err = p.merge(class, backend); err != nil {
			return nil, err
		}
		p.modified = true
	}
	return p, nil
}

func (p *arithmeticSimilarityProgram) merge(class similarityClass, backend similarityBackend) error {
	domains, rows, err := similarityOrphanRows(class.rules, class.constants)
	if err != nil {
		return err
	}
	if backend == similarityKarrBackend {
		family, err := p.manager.RegisterArithmeticPlugin()
		if err != nil {
			return err
		}
		integer, err := p.manager.MkFamilySort(family, 1, nil)
		if err != nil {
			return err
		}
		for _, sort := range domains {
			if sort != integer {
				return ast.ErrASTDomain
			}
		}
	} else if !CanHandleIntervalSorts(p.manager, domains) {
		return ast.ErrASTDomain
	}
	var intervalSeed *IntervalRelation
	var boundSeed *BoundRelation
	var karrSeed *KarrRelation
	switch backend {
	case similarityBoundBackend:
		boundSeed = NewBoundRelation(uint32(len(domains)), true)
		for _, row := range rows {
			if err = boundSeed.AddFact(row); err != nil {
				return err
			}
		}
	case similarityIntervalBackend:
		intervalSeed = NewIntervalEmpty(uint32(len(domains)))
		for _, row := range rows {
			if err = intervalSeed.AddFactExpressions(p.manager, domains, row); err != nil {
				return err
			}
		}
	case similarityKarrBackend:
		karrSeed = NewKarrEmpty(uint32(len(domains)), nil)
		for _, row := range rows {
			values := make([]*big.Rat, len(row))
			for i, expression := range row {
				text, _, matched, cause := p.manager.ArithmeticNumeralText(expression)
				if cause != nil {
					return cause
				}
				if matched {
					value, ok := new(big.Rat).SetString(text)
					if !ok {
						return ast.ErrASTDomain
					}
					if value.IsInt() {
						values[i] = value
					}
				}
			}
			if err = karrSeed.AddFact(values); err != nil {
				return err
			}
		}
	default:
		return ast.ErrASTDomain
	}
	head, err := class.rules[0].HeadDeclaration()
	if err != nil {
		return err
	}
	prefix, err := p.manager.GetASTName(head)
	if err != nil {
		return err
	}
	suffix, err := p.manager.Symbols.Intern(fmt.Sprintf("sc_%d", len(class.constants)))
	if err != nil {
		return err
	}
	boolean, err := p.manager.MkBoolSort()
	if err != nil {
		return err
	}
	auxiliary, err := p.manager.MkFreshFuncDecl(prefix, suffix, domains, boolean)
	if err != nil {
		return err
	}
	ref, err := p.manager.NewRef(auxiliary)
	if err != nil {
		return err
	}
	p.refs = append(p.refs, ref)
	if err = p.rules.RegisterPredicate(auxiliary); err != nil {
		return err
	}
	p.auxiliary = append(p.auxiliary, auxiliary)
	switch backend {
	case similarityBoundBackend:
		p.boundSeeds[auxiliary] = boundSeed
	case similarityIntervalBackend:
		p.seeds[auxiliary] = intervalSeed
	case similarityKarrBackend:
		p.karrSeeds[auxiliary] = karrSeed
	}
	p.origins[auxiliary] = head
	merged, err := similarityRewriteRule(p.rules, class.rules[0], class.constants, auxiliary)
	if err != nil {
		return err
	}
	p.pending = append(p.pending, merged)
	if err = p.set.Add(merged); err != nil {
		return err
	}
	if err = merged.Close(); err != nil {
		return err
	}
	p.pending = p.pending[:len(p.pending)-1]
	return nil
}

func (p *arithmeticSimilarityProgram) Rules() (*base.RuleSet, error) {
	if p == nil || p.closed || p.set == nil {
		return nil, ast.ErrASTHandle
	}
	return p.set, nil
}

func (p *arithmeticSimilarityProgram) AuxiliaryPredicates() []ast.ASTHandle {
	if p == nil || p.closed {
		return nil
	}
	return append([]ast.ASTHandle(nil), p.auxiliary...)
}

// AuxiliaryRelation returns a borrowed numeric seed; evaluators clone it.
func (p *IntervalSimilarityProgram) AuxiliaryRelation(predicate ast.ASTHandle) (*IntervalRelation, bool) {
	if p == nil || p.arithmeticSimilarityProgram == nil || p.closed {
		return nil, false
	}
	relation, found := p.seeds[predicate]
	return relation, found
}

// AuxiliaryRelation returns a borrowed bound seed; query runtimes clone it.
func (p *BoundSimilarityProgram) AuxiliaryRelation(predicate ast.ASTHandle) (*BoundRelation, bool) {
	if p == nil || p.arithmeticSimilarityProgram == nil || p.closed {
		return nil, false
	}
	relation, found := p.boundSeeds[predicate]
	return relation, found
}

func (p *KarrSimilarityProgram) AuxiliaryRelation(predicate ast.ASTHandle) (*KarrRelation, bool) {
	if p == nil || p.arithmeticSimilarityProgram == nil || p.closed {
		return nil, false
	}
	relation, found := p.karrSeeds[predicate]
	return relation, found
}

func (p *arithmeticSimilarityProgram) Modified() bool { return p != nil && !p.closed && p.modified }

func (p *arithmeticSimilarityProgram) Close() error {
	if p == nil || p.closed {
		return nil
	}
	for len(p.pending) != 0 {
		last := len(p.pending) - 1
		if err := p.pending[last].Close(); err != nil {
			return err
		}
		p.pending = p.pending[:last]
	}
	if p.set != nil {
		if err := p.set.Close(); err != nil {
			return err
		}
		p.set = nil
	}
	for len(p.sourceRules) != 0 {
		last := len(p.sourceRules) - 1
		if err := p.sourceRules[last].Close(); err != nil {
			return err
		}
		p.sourceRules = p.sourceRules[:last]
	}
	if p.rules != nil {
		if err := p.rules.Close(); err != nil {
			return err
		}
		p.rules = nil
	}
	for len(p.refs) != 0 {
		last := len(p.refs) - 1
		err := p.manager.DelRef(&p.refs[last])
		if p.refs[last] == (ast.ASTRef{}) {
			p.refs = p.refs[:last]
		}
		if err != nil {
			return err
		}
	}
	p.seeds, p.boundSeeds, p.karrSeeds, p.origins, p.auxiliary, p.closed = nil, nil, nil, nil, nil, true
	return nil
}
