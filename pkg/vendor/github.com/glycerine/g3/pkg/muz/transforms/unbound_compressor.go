package transforms

// Source: src/muz/transforms/dl_mk_unbound_compressor.{h,cpp}.

import (
	"errors"
	"fmt"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type compressionKey struct {
	predicate ast.ASTHandle
	argument  uint32
}

type UnboundCompressorPlugin struct {
	manager  *ast.ASTManager
	rules    *base.RuleManager
	enabled  bool
	priority uint32
	mapping  map[compressionKey]ast.ASTHandle
	closed   bool
}

func NewUnboundCompressorPlugin(manager *ast.ASTManager, rules *base.RuleManager, enabled bool, priority uint32) *UnboundCompressorPlugin {
	if priority == 0 {
		priority = 500
	}
	return &UnboundCompressorPlugin{manager: manager, rules: rules, enabled: enabled, priority: priority, mapping: make(map[compressionKey]ast.ASTHandle)}
}

func (p *UnboundCompressorPlugin) Priority() uint32            { return p.priority }
func (p *UnboundCompressorPlugin) CanDestratifyNegation() bool { return false }
func (p *UnboundCompressorPlugin) Cancel()                     {}

func (p *UnboundCompressorPlugin) CompressedPredicates() map[ast.ASTHandle]ast.ASTHandle {
	result := make(map[ast.ASTHandle]ast.ASTHandle)
	for key, declaration := range p.mapping {
		result[key.predicate] = declaration
	}
	return result
}

func (p *UnboundCompressorPlugin) headTask(source *base.RuleSet, rule *base.Rule) (compressionKey, bool, error) {
	declaration, err := rule.HeadDeclaration()
	if err != nil {
		return compressionKey{}, false, err
	}
	output, err := source.IsOutputPredicate(declaration)
	if err != nil || output {
		return compressionKey{}, false, err
	}
	head, err := rule.Head()
	if err != nil {
		return compressionKey{}, false, err
	}
	tailVariables, err := p.rules.TailVariables(rule)
	if err != nil {
		return compressionKey{}, false, err
	}
	count, err := p.manager.GetNumArgs(head)
	if err != nil {
		return compressionKey{}, false, err
	}
	occurrences := make(map[uint32]uint32)
	for index := uint32(0); index < count; index++ {
		argument, getErr := p.manager.GetArg(head, index)
		if getErr != nil {
			return compressionKey{}, false, getErr
		}
		if kind, x := p.manager.GetKind(argument); x == nil && kind == ast.ASTVar {
			variable, x := p.manager.GetVarIndex(argument)
			if x != nil {
				return compressionKey{}, false, x
			}
			occurrences[variable]++
		}
	}
	for index := uint32(0); index < count; index++ {
		argument, getErr := p.manager.GetArg(head, index)
		if getErr != nil {
			return compressionKey{}, false, getErr
		}
		kind, getErr := p.manager.GetKind(argument)
		if getErr != nil || kind != ast.ASTVar {
			continue
		}
		variable, getErr := p.manager.GetVarIndex(argument)
		if getErr != nil {
			return compressionKey{}, false, getErr
		}
		if tailVariables[variable].IsNull() && occurrences[variable] == 1 {
			return compressionKey{predicate: declaration, argument: index}, true, nil
		}
	}
	return compressionKey{}, false, nil
}

func (p *UnboundCompressorPlugin) declare(key compressionKey) (ast.ASTHandle, error) {
	if declaration := p.mapping[key]; !declaration.IsNull() {
		return declaration, nil
	}
	arity, err := p.manager.GetArity(key.predicate)
	if err != nil || arity == 0 || key.argument >= arity {
		return ast.ASTHandle{}, errors.Join(err, ast.ErrASTDomain)
	}
	domain := make([]ast.ASTHandle, 0, arity-1)
	for index := uint32(0); index < arity; index++ {
		if index == key.argument {
			continue
		}
		sort, getErr := p.manager.GetDomain(key.predicate, index)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		domain = append(domain, sort)
	}
	name, err := p.manager.GetASTName(key.predicate)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	suffixText := fmt.Sprintf("compr_arg_%d", key.argument)
	suffix, err := p.manager.Symbols.Intern(suffixText)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	boolean, err := p.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	declaration, err := p.manager.MkFreshFuncDecl(name, suffix, domain, boolean)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = p.rules.RegisterPredicate(declaration); err != nil {
		return ast.ASTHandle{}, err
	}
	p.mapping[key] = declaration
	return declaration, nil
}

func (p *UnboundCompressorPlugin) compressedApplication(application ast.ASTHandle, key compressionKey) (ast.ASTHandle, error) {
	declaration := p.mapping[key]
	if declaration.IsNull() {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	count, err := p.manager.GetNumArgs(application)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	arguments := make([]ast.ASTHandle, 0, count-1)
	for index := uint32(0); index < count; index++ {
		if index == key.argument {
			continue
		}
		argument, getErr := p.manager.GetArg(application, index)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		arguments = append(arguments, argument)
	}
	return p.manager.MkApp(declaration, arguments)
}

func (p *UnboundCompressorPlugin) rewriteRule(source *base.RuleSet, rule *base.Rule, inProgress map[compressionKey]bool) ([]*base.Rule, bool, error) {
	head, err := rule.Head()
	if err != nil {
		return nil, false, err
	}
	headDeclaration, err := rule.HeadDeclaration()
	if err != nil {
		return nil, false, err
	}
	changed := false
	key, found, err := p.headTask(source, rule)
	if err != nil {
		return nil, false, err
	}
	if found && inProgress[key] && !p.mapping[key].IsNull() && key.predicate == headDeclaration {
		head, err = p.compressedApplication(head, key)
		if err != nil {
			return nil, false, err
		}
		changed = true
	}
	total, err := rule.TailSize()
	if err != nil {
		return nil, false, err
	}
	type alternative struct {
		tails          []ast.ASTHandle
		neg            []bool
		extra          []ast.ASTHandle
		decompressed   bool
		negativeChange bool
	}
	alternatives := []alternative{{}}
	changedByTail := false
	for index := 0; index < total; index++ {
		tail, negative, getErr := rule.Tail(index)
		if getErr != nil {
			return nil, false, getErr
		}
		declaration, getErr := p.manager.GetDecl(tail)
		if getErr != nil {
			return nil, false, getErr
		}
		var keys []compressionKey
		for key := range inProgress {
			if key.predicate == declaration {
				keys = append(keys, key)
			}
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i].argument < keys[j].argument })
		if len(keys) == 0 {
			for i := range alternatives {
				alternatives[i].tails = append(alternatives[i].tails, tail)
				alternatives[i].neg = append(alternatives[i].neg, negative)
			}
			continue
		}
		changedByTail = true
		if negative {
			compressed, x := p.compressedApplication(tail, keys[0])
			if x != nil {
				return nil, false, x
			}
			for i := range alternatives {
				alternatives[i].tails = append(alternatives[i].tails, tail)
				alternatives[i].neg = append(alternatives[i].neg, true)
				alternatives[i].extra = append(alternatives[i].extra, compressed)
				alternatives[i].negativeChange = true
			}
			continue
		}
		var branched []alternative
		for _, existing := range alternatives {
			original := alternative{tails: append(append([]ast.ASTHandle(nil), existing.tails...), tail), neg: append(append([]bool(nil), existing.neg...), false), extra: append([]ast.ASTHandle(nil), existing.extra...), decompressed: existing.decompressed, negativeChange: existing.negativeChange}
			branched = append(branched, original)
			for _, key := range keys {
				compressed, x := p.compressedApplication(tail, key)
				if x != nil {
					return nil, false, x
				}
				variant := alternative{tails: append(append([]ast.ASTHandle(nil), existing.tails...), compressed), neg: append(append([]bool(nil), existing.neg...), false), extra: append([]ast.ASTHandle(nil), existing.extra...), decompressed: true, negativeChange: existing.negativeChange}
				branched = append(branched, variant)
			}
		}
		alternatives = branched
	}
	if !changed && !changedByTail {
		retained, retainErr := rule.Retain()
		return []*base.Rule{retained}, false, retainErr
	}
	name, err := rule.NameSymbol()
	if err != nil {
		return nil, false, err
	}
	result := make([]*base.Rule, 0, len(alternatives))
	for _, variant := range alternatives {
		if !changed && !variant.decompressed && !variant.negativeChange {
			retained, retainErr := rule.Retain()
			if retainErr != nil {
				for i := len(result) - 1; i >= 0; i-- {
					retainErr = errors.Join(retainErr, result[i].Close())
				}
				return nil, false, retainErr
			}
			result = append(result, retained)
			continue
		}
		variant.tails = append(variant.tails, variant.extra...)
		for range variant.extra {
			variant.neg = append(variant.neg, true)
		}
		// Source mk_decompression_rule omits a name for either tail rewrite;
		// only pure head compression inherits the parent symbol.
		label := name
		if variant.decompressed || variant.negativeChange {
			label = usymbol.Symbol{}
		}
		replacement, makeErr := p.rules.NewRuleWithSymbol(head, variant.tails, variant.neg, label)
		if makeErr == nil {
			makeErr = replacement.SetAccountingParentRule(rule)
		}
		if makeErr == nil && variant.decompressed {
			makeErr = p.rules.MkRuleRewriteProof(rule, replacement)
		}
		if makeErr != nil {
			if replacement != nil {
				makeErr = errors.Join(makeErr, replacement.Close())
			}
			for i := len(result) - 1; i >= 0; i-- {
				makeErr = errors.Join(makeErr, result[i].Close())
			}
			return nil, false, makeErr
		}
		result = append(result, replacement)
	}
	return result, true, nil
}

func (p *UnboundCompressorPlugin) Transform(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	if p == nil || p.closed || p.manager == nil || p.rules == nil {
		return nil, base.ErrRuleTransformerOwner
	}
	if !p.enabled {
		return nil, nil
	}
	p.mapping = make(map[compressionKey]ast.ASTHandle)
	current := source
	var owned *base.RuleSet
	defer func() {
		if owned != nil {
			resultErr = errors.Join(resultErr, owned.Close())
		}
	}()
	for {
		before := len(p.mapping)
		inProgress := make(map[compressionKey]bool)
		count, err := current.Len()
		if err != nil {
			return nil, err
		}
		for index := 0; index < count; index++ {
			rule, getErr := current.Rule(index)
			if getErr != nil {
				return nil, getErr
			}
			key, found, getErr := p.headTask(source, rule)
			getErr = errors.Join(getErr, rule.Close())
			if getErr != nil {
				return nil, getErr
			}
			if found {
				if p.mapping[key].IsNull() {
					if _, getErr = p.declare(key); getErr != nil {
						return nil, getErr
					}
					inProgress[key] = true
				}
			}
		}
		if len(p.mapping) == before {
			if owned == nil {
				return nil, nil
			}
			result := owned
			owned = nil
			return result, nil
		}
		result, err := base.NewRuleSet(p.rules)
		if err != nil {
			return nil, err
		}
		for index := 0; index < count; index++ {
			rule, getErr := current.Rule(index)
			if getErr != nil {
				return nil, errors.Join(getErr, result.Close())
			}
			replacements, _, getErr := p.rewriteRule(source, rule, inProgress)
			for _, replacement := range replacements {
				if getErr == nil {
					getErr = result.Add(replacement)
				}
				getErr = errors.Join(getErr, replacement.Close())
			}
			getErr = errors.Join(getErr, rule.Close())
			if getErr != nil {
				return nil, errors.Join(getErr, result.Close())
			}
		}
		if err = result.InheritPredicates(current); err != nil {
			return nil, errors.Join(err, result.Close())
		}
		for key, declaration := range p.mapping {
			if err = result.InheritPredicate(source, key.predicate, declaration); err != nil {
				return nil, errors.Join(err, result.Close())
			}
		}
		if owned != nil {
			if err = owned.Close(); err != nil {
				return nil, errors.Join(err, result.Close())
			}
		}
		owned, current = result, result
	}
}

func (p *UnboundCompressorPlugin) Close() error {
	if p == nil || p.closed {
		return nil
	}
	p.closed, p.mapping = true, nil
	return nil
}
