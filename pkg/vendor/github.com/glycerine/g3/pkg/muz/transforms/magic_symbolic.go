package transforms

// Source: src/muz/transforms/dl_mk_magic_symbolic.cpp/.h symbolic magic-set
// query and answer rule construction.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

type MagicSymbolicPlugin struct {
	manager  *ast.ASTManager
	rules    *base.RuleManager
	priority uint32
	queries  map[ast.ASTHandle]ast.ASTHandle
	answers  map[ast.ASTHandle]ast.ASTHandle
	closed   bool
}

func NewMagicSymbolicPlugin(manager *ast.ASTManager, rules *base.RuleManager, priority uint32) *MagicSymbolicPlugin {
	return &MagicSymbolicPlugin{manager: manager, rules: rules, priority: priority, queries: make(map[ast.ASTHandle]ast.ASTHandle), answers: make(map[ast.ASTHandle]ast.ASTHandle)}
}

func (p *MagicSymbolicPlugin) Priority() uint32            { return p.priority }
func (p *MagicSymbolicPlugin) CanDestratifyNegation() bool { return false }
func (p *MagicSymbolicPlugin) Cancel()                     {}

func (p *MagicSymbolicPlugin) AnswerPredicates() map[ast.ASTHandle]ast.ASTHandle {
	result := make(map[ast.ASTHandle]ast.ASTHandle, len(p.answers))
	for old, answer := range p.answers {
		result[old] = answer
	}
	return result
}

func (p *MagicSymbolicPlugin) declaration(old ast.ASTHandle, suffix string, cache map[ast.ASTHandle]ast.ASTHandle) (ast.ASTHandle, error) {
	if declaration := cache[old]; !declaration.IsNull() {
		return declaration, nil
	}
	name, err := p.manager.GetASTName(old)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	text, err := p.manager.Symbols.String(name)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	name, err = p.manager.Symbols.Intern(text + suffix)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	arity, err := p.manager.GetArity(old)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	domain := make([]ast.ASTHandle, arity)
	for i := uint32(0); i < arity; i++ {
		domain[i], err = p.manager.GetDomain(old, i)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	rangeSort, err := p.manager.GetRange(old)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	declaration, err := p.manager.MkFuncDecl(name, domain, rangeSort, ast.ASTFuncDeclInfo{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = p.rules.RegisterPredicate(declaration); err != nil {
		return ast.ASTHandle{}, err
	}
	cache[old] = declaration
	return declaration, nil
}

func (p *MagicSymbolicPlugin) application(source ast.ASTHandle, suffix string, cache map[ast.ASTHandle]ast.ASTHandle) (ast.ASTHandle, error) {
	old, err := p.manager.GetDecl(source)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	declaration, err := p.declaration(old, suffix, cache)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	arguments, err := p.manager.AppArgumentView(source)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.manager.RewriterMakeApp(declaration, arguments)
}

func (p *MagicSymbolicPlugin) Transform(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	if p == nil || p.closed || p.manager == nil || p.rules == nil {
		return nil, base.ErrRuleTransformerOwner
	}
	result, err := base.NewRuleSet(p.rules)
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		if !keep {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	for i := 0; i < count; i++ {
		rule, getErr := source.Rule(i)
		if getErr != nil {
			return nil, getErr
		}
		var pins []ast.ASTRef
		pin := func(handle ast.ASTHandle) (ast.ASTHandle, error) {
			reference, pinErr := p.manager.NewRef(handle)
			if pinErr == nil {
				pins = append(pins, reference)
			}
			return handle, pinErr
		}
		release := func() (releaseErr error) {
			for j := len(pins) - 1; j >= 0; j-- {
				releaseErr = errors.Join(releaseErr, p.manager.DelRef(&pins[j]))
			}
			return releaseErr
		}
		uninterpreted, sizeErr := rule.UninterpretedTailSize()
		total, totalErr := rule.TailSize()
		getErr = errors.Join(sizeErr, totalErr)
		tails := make([]ast.ASTHandle, 0, total+1)
		for j := uninterpreted; getErr == nil && j < total; j++ {
			tail, _, tailErr := rule.Tail(j)
			getErr = tailErr
			tails = append(tails, tail)
		}
		var head ast.ASTHandle
		if getErr == nil {
			head, getErr = rule.Head()
		}
		var headQuery ast.ASTHandle
		if getErr == nil {
			headQuery, getErr = p.application(head, "!query", p.queries)
		}
		if getErr == nil {
			headQuery, getErr = pin(headQuery)
		}
		tails = append(tails, headQuery)
		for j := 0; getErr == nil && j < uninterpreted; j++ {
			tail, _, tailErr := rule.Tail(j)
			if tailErr == nil {
				tail, tailErr = p.application(tail, "!ans", p.answers)
			}
			if tailErr == nil {
				tail, tailErr = pin(tail)
			}
			getErr = tailErr
			tails = append(tails, tail)
		}
		var answerHead ast.ASTHandle
		if getErr == nil {
			answerHead, getErr = p.application(head, "!ans", p.answers)
		}
		if getErr == nil {
			answerHead, getErr = pin(answerHead)
		}
		name, nameErr := rule.Name()
		getErr = errors.Join(getErr, nameErr)
		var replacement *base.Rule
		if getErr == nil {
			replacement, getErr = p.rules.NewRule(answerHead, tails, make([]bool, len(tails)), name)
		}
		if getErr == nil {
			getErr = result.Add(replacement)
		}
		if replacement != nil {
			getErr = errors.Join(getErr, replacement.Close())
			replacement = nil
		}
		output := false
		if getErr == nil {
			declaration, declarationErr := rule.HeadDeclaration()
			if declarationErr == nil {
				output, declarationErr = source.IsOutputPredicate(declaration)
			}
			getErr = declarationErr
		}
		if getErr == nil && output {
			old, declarationErr := rule.HeadDeclaration()
			if declarationErr == nil {
				declarationErr = result.InheritPredicate(source, old, p.answers[old])
			}
			if declarationErr == nil {
				replacement, declarationErr = p.rules.NewRule(headQuery, nil, nil, name)
			}
			if declarationErr == nil {
				declarationErr = result.Add(replacement)
			}
			if replacement != nil {
				declarationErr = errors.Join(declarationErr, replacement.Close())
				replacement = nil
			}
			getErr = declarationErr
		}
		for j := 0; getErr == nil && j < uninterpreted; j++ {
			original, _, tailErr := rule.Tail(j)
			var queryHead ast.ASTHandle
			if tailErr == nil {
				queryHead, tailErr = p.application(original, "!query", p.queries)
			}
			if tailErr == nil {
				queryHead, tailErr = pin(queryHead)
			}
			prefix := total - uninterpreted + 1 + j
			if tailErr == nil {
				replacement, tailErr = p.rules.NewRule(queryHead, tails[:prefix], make([]bool, prefix), name)
			}
			if tailErr == nil {
				tailErr = result.Add(replacement)
			}
			if replacement != nil {
				tailErr = errors.Join(tailErr, replacement.Close())
				replacement = nil
			}
			getErr = tailErr
		}
		getErr = errors.Join(getErr, release(), rule.Close())
		if getErr != nil {
			return nil, getErr
		}
	}
	keep = true
	return result, nil
}

func (p *MagicSymbolicPlugin) Close() error {
	if p == nil || p.closed {
		return nil
	}
	p.closed = true
	p.queries, p.answers = nil, nil
	return nil
}
