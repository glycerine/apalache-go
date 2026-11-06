package rel

// Source: dl_mk_similarity_compressor.cpp, using the active external relation
// plugin to store each generated auxiliary fact.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// ExternalSimilarityProgram owns transformed rules and their generated fact
// rows. The caller inserts those rows into its context's callback relations.
type ExternalSimilarityProgram struct {
	manager     *ast.ASTManager
	rules       *base.RuleManager
	set         *base.RuleSet
	sourceRules []*base.Rule
	pending     []*base.Rule
	refs        []ast.ASTRef
	auxiliary   []ast.ASTHandle
	rows        map[ast.ASTHandle][][]ast.ASTHandle
	modified    bool
	closed      bool
}

func NewExternalSimilarityProgram(manager *ast.ASTManager, source *base.RuleSet, threshold uint32, outputs []ast.ASTHandle) (_ *ExternalSimilarityProgram, resultErr error) {
	if manager == nil || source == nil || threshold <= 1 {
		return nil, ast.ErrASTDomain
	}
	p := &ExternalSimilarityProgram{manager: manager, rows: make(map[ast.ASTHandle][][]ast.ASTHandle)}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, p.Close())
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
		rule, cause := source.Rule(index)
		if cause != nil {
			return nil, cause
		}
		p.sourceRules = append(p.sourceRules, rule)
		head, cause := rule.HeadDeclaration()
		if cause != nil {
			return nil, cause
		}
		if cause = p.rules.RegisterPredicate(head); cause != nil {
			return nil, cause
		}
		uninterpreted, cause := rule.UninterpretedTailSize()
		if cause != nil {
			return nil, cause
		}
		for tail := 0; tail < uninterpreted; tail++ {
			declaration, cause := rule.TailDeclaration(tail)
			if cause == nil {
				cause = p.rules.RegisterPredicate(declaration)
			}
			if cause != nil {
				return nil, cause
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
	classes, duplicates, err := similarityClassesExcept(source, p.sourceRules, threshold, protected)
	if err != nil {
		return nil, err
	}
	p.modified = duplicates
	for _, class := range classes {
		p.modified = p.modified || class.merge
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
		auxiliary, rows, cause := p.merge(class)
		if cause != nil {
			return nil, cause
		}
		p.rows[auxiliary] = rows
		p.auxiliary = append(p.auxiliary, auxiliary)
		merged, cause := similarityRewriteRule(p.rules, class.rules[0], class.constants, auxiliary)
		if cause != nil {
			return nil, cause
		}
		p.pending = append(p.pending, merged)
		cause = p.set.Add(merged)
		cause = errors.Join(cause, merged.Close())
		if cause == nil {
			p.pending = p.pending[:len(p.pending)-1]
		}
		if cause != nil {
			return nil, cause
		}
	}
	return p, nil
}

func (p *ExternalSimilarityProgram) merge(class similarityClass) (ast.ASTHandle, [][]ast.ASTHandle, error) {
	domains, rows, err := similarityOrphanRows(class.rules, class.constants)
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	head, err := class.rules[0].HeadDeclaration()
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	prefix, err := p.manager.GetASTName(head)
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	suffix, err := p.manager.Symbols.Intern(fmt.Sprintf("sc_%d", len(class.constants)))
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	boolean, err := p.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	auxiliary, err := p.manager.MkFreshFuncDecl(prefix, suffix, domains, boolean)
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	ref, err := p.manager.NewRef(auxiliary)
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	p.refs = append(p.refs, ref)
	if err = p.rules.RegisterPredicate(auxiliary); err != nil {
		return ast.ASTHandle{}, nil, err
	}
	return auxiliary, rows, nil
}

func (p *ExternalSimilarityProgram) Rules() (*base.RuleSet, error) {
	if p == nil || p.closed || p.set == nil {
		return nil, ast.ErrASTHandle
	}
	return p.set, nil
}

func (p *ExternalSimilarityProgram) AuxiliaryPredicates() []ast.ASTHandle {
	if p == nil || p.closed {
		return nil
	}
	return append([]ast.ASTHandle(nil), p.auxiliary...)
}

func (p *ExternalSimilarityProgram) SeedRows(predicate ast.ASTHandle) ([][]ast.ASTHandle, bool) {
	if p == nil || p.closed {
		return nil, false
	}
	rows, found := p.rows[predicate]
	result := make([][]ast.ASTHandle, len(rows))
	for i := range rows {
		result[i] = append([]ast.ASTHandle(nil), rows[i]...)
	}
	return result, found
}

func (p *ExternalSimilarityProgram) Modified() bool { return p != nil && !p.closed && p.modified }

func (p *ExternalSimilarityProgram) Close() (result error) {
	if p == nil || p.closed {
		return nil
	}
	for len(p.pending) != 0 {
		last := len(p.pending) - 1
		if err := p.pending[last].Close(); err != nil {
			return errors.Join(result, err)
		}
		p.pending = p.pending[:last]
	}
	if p.set != nil {
		if err := p.set.Close(); err != nil {
			return errors.Join(result, err)
		}
		p.set = nil
	}
	for len(p.sourceRules) != 0 {
		last := len(p.sourceRules) - 1
		if err := p.sourceRules[last].Close(); err != nil {
			return errors.Join(result, err)
		}
		p.sourceRules = p.sourceRules[:last]
	}
	if p.rules != nil {
		if err := p.rules.Close(); err != nil {
			return errors.Join(result, err)
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
			return errors.Join(result, err)
		}
	}
	p.rows, p.auxiliary, p.manager, p.closed = nil, nil, nil, true
	return nil
}
