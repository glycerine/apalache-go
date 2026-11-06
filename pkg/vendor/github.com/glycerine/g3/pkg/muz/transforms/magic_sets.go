package transforms

// Source: src/muz/transforms/dl_mk_magic_sets.{h,cpp}.

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type magicAdornment []bool
type magicDescription struct {
	predicate ast.ASTHandle
	pattern   string
}
type magicTask struct {
	description magicDescription
	adornment   magicAdornment
}

// MagicSetsFactProbe reports whether a predicate has materialized facts in a
// relation backend that is not published through RelationManager.
type MagicSetsFactProbe func(predicate ast.ASTHandle) (bool, error)

type MagicSetsPlugin struct {
	manager     *ast.ASTManager
	rules       *base.RuleManager
	goal        ast.ASTHandle
	relations   *rel.RelationManager
	factProbe   MagicSetsFactProbe
	priority    uint32
	enabled     bool
	adorned     map[magicDescription]ast.ASTHandle
	patterns    map[ast.ASTHandle]magicAdornment
	magic       map[ast.ASTHandle]ast.ASTHandle
	freshOrigin map[ast.ASTHandle]ast.ASTHandle
	extensional map[ast.ASTHandle]bool
	todo        []magicTask
	fresh       []ast.ASTHandle
	closed      bool
}

func NewMagicSetsPlugin(manager *ast.ASTManager, rules *base.RuleManager, goal ast.ASTHandle, enabled bool, priority uint32) *MagicSetsPlugin {
	return NewMagicSetsPluginWithFactProbe(manager, rules, goal, enabled, priority, nil, nil)
}

// NewMagicSetsPluginWithRelations also transfers already materialized facts
// into adorned predicates, matching the source query transform.
func NewMagicSetsPluginWithRelations(manager *ast.ASTManager, rules *base.RuleManager, goal ast.ASTHandle, enabled bool, priority uint32, relations *rel.RelationManager) *MagicSetsPlugin {
	return NewMagicSetsPluginWithFactProbe(manager, rules, goal, enabled, priority, relations, nil)
}

// NewMagicSetsPluginWithFactProbe also consults factProbe before the relation
// manager. A false result falls through to the relation manager so specialized
// and manager-published relation backends can coexist.
func NewMagicSetsPluginWithFactProbe(manager *ast.ASTManager, rules *base.RuleManager, goal ast.ASTHandle, enabled bool, priority uint32, relations *rel.RelationManager, factProbe MagicSetsFactProbe) *MagicSetsPlugin {
	if priority == 0 {
		priority = 10000
	}
	return &MagicSetsPlugin{manager: manager, rules: rules, goal: goal, enabled: enabled, priority: priority, relations: relations, factProbe: factProbe}
}
func (p *MagicSetsPlugin) Priority() uint32            { return p.priority }
func (p *MagicSetsPlugin) CanDestratifyNegation() bool { return true }
func (p *MagicSetsPlugin) Cancel()                     {}
func (p *MagicSetsPlugin) FreshPredicates() []ast.ASTHandle {
	return append([]ast.ASTHandle(nil), p.fresh...)
}

// FreshPredicateOrigin returns the direct source predicate supplied to
// context::mk_fresh_head_predicate. Magic predicates therefore point to their
// adorned predicate, preserving the source's representation-inheritance chain.
func (p *MagicSetsPlugin) FreshPredicateOrigin(predicate ast.ASTHandle) (ast.ASTHandle, bool) {
	origin, found := p.freshOrigin[predicate]
	return origin, found
}

func adornmentKey(a magicAdornment) string {
	var b strings.Builder
	for _, bound := range a {
		if bound {
			b.WriteByte('b')
		} else {
			b.WriteByte('f')
		}
	}
	return b.String()
}
func (p *MagicSetsPlugin) adornment(literal ast.ASTHandle, bound map[uint32]bool) (magicAdornment, error) {
	n, err := p.manager.GetNumArgs(literal)
	if err != nil {
		return nil, err
	}
	a := make(magicAdornment, n)
	for i := uint32(0); i < n; i++ {
		arg, e := p.manager.GetArg(literal, i)
		if e != nil {
			return nil, e
		}
		kind, e := p.manager.GetKind(arg)
		if e != nil {
			return nil, e
		}
		if kind != ast.ASTVar {
			a[i] = true
			continue
		}
		v, e := p.manager.GetVarIndex(arg)
		if e != nil {
			return nil, e
		}
		a[i] = bound[v]
	}
	return a, nil
}
func (p *MagicSetsPlugin) freshPredicate(old ast.ASTHandle, suffix string, domain []ast.ASTHandle) (ast.ASTHandle, error) {
	name, err := p.manager.GetASTName(old)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	suffixSymbol, err := p.manager.Symbols.Intern(suffix)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	b, err := p.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	d, err := p.manager.MkFreshFuncDecl(name, suffixSymbol, domain, b)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = p.rules.RegisterPredicate(d); err != nil {
		return ast.ASTHandle{}, err
	}
	p.freshOrigin[d] = old
	p.fresh = append(p.fresh, d)
	return d, nil
}
func (p *MagicSetsPlugin) adornLiteral(literal ast.ASTHandle, bound map[uint32]bool) (ast.ASTHandle, error) {
	old, err := p.manager.GetDecl(literal)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	a, err := p.adornment(literal, bound)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	desc := magicDescription{predicate: old, pattern: adornmentKey(a)}
	decl := p.adorned[desc]
	if decl.IsNull() {
		arity, e := p.manager.GetArity(old)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		domain := make([]ast.ASTHandle, arity)
		for i := uint32(0); i < arity; i++ {
			domain[i], e = p.manager.GetDomain(old, i)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
		decl, err = p.freshPredicate(old, "ad_"+desc.pattern, domain)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		p.adorned[desc], p.patterns[decl] = decl, a
		p.todo = append(p.todo, magicTask{description: desc, adornment: a})
	}
	n, err := p.manager.GetNumArgs(literal)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	args := make([]ast.ASTHandle, n)
	for i := uint32(0); i < n; i++ {
		args[i], err = p.manager.GetArg(literal, i)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return p.manager.MkApp(decl, args)
}
func (p *MagicSetsPlugin) magicLiteral(adorned ast.ASTHandle) (ast.ASTHandle, error) {
	decl, err := p.manager.GetDecl(adorned)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	pattern, ok := p.patterns[decl]
	if !ok {
		return ast.ASTHandle{}, fmt.Errorf("muz/transforms: missing magic adornment")
	}
	magicDecl := p.magic[decl]
	var args, domain []ast.ASTHandle
	for i, bound := range pattern {
		if !bound {
			continue
		}
		arg, e := p.manager.GetArg(adorned, uint32(i))
		if e != nil {
			return ast.ASTHandle{}, e
		}
		sort, e := p.manager.GetDomain(decl, uint32(i))
		if e != nil {
			return ast.ASTHandle{}, e
		}
		args, domain = append(args, arg), append(domain, sort)
	}
	if magicDecl.IsNull() {
		magicDecl, err = p.freshPredicate(decl, "ms", domain)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		p.magic[decl] = magicDecl
	}
	return p.manager.MkApp(magicDecl, args)
}
func (p *MagicSetsPlugin) addRule(result *base.RuleSet, head ast.ASTHandle, tails []ast.ASTHandle, neg []bool, name usymbol.Symbol, parent *base.Rule) error {
	r, err := p.rules.NewRuleWithSymbol(head, tails, neg, name)
	if err != nil {
		return err
	}
	if parent != nil {
		err = r.SetAccountingParentRule(parent)
	}
	if err == nil {
		err = result.Add(r)
	}
	return errors.Join(err, r.Close())
}
func collectLiteralVars(m *ast.ASTManager, literal ast.ASTHandle, bound map[uint32]bool) error {
	n, err := m.GetNumArgs(literal)
	if err != nil {
		return err
	}
	for i := uint32(0); i < n; i++ {
		arg, e := m.GetArg(literal, i)
		if e != nil {
			return e
		}
		if k, e := m.GetKind(arg); e != nil {
			return e
		} else if k == ast.ASTVar {
			v, e := m.GetVarIndex(arg)
			if e != nil {
				return e
			}
			bound[v] = true
		}
	}
	return nil
}
func (p *MagicSetsPlugin) boundCount(literal ast.ASTHandle, bound map[uint32]bool) (int, error) {
	a, err := p.adornment(literal, bound)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, x := range a {
		if x {
			n++
		}
	}
	return n, nil
}

func (p *MagicSetsPlugin) unboundCost(literal ast.ASTHandle, bound map[uint32]bool) (float32, error) {
	declaration, err := p.manager.GetDecl(literal)
	if err != nil {
		return 0, err
	}
	count, err := p.manager.GetNumArgs(literal)
	if err != nil {
		return 0, err
	}
	cost := float32(1)
	for index := uint32(0); index < count; index++ {
		argument, getErr := p.manager.GetArg(literal, index)
		if getErr != nil {
			return 0, getErr
		}
		kind, getErr := p.manager.GetKind(argument)
		if getErr != nil {
			return 0, getErr
		}
		if kind != ast.ASTVar {
			continue
		}
		variable, getErr := p.manager.GetVarIndex(argument)
		if getErr != nil {
			return 0, getErr
		}
		if bound[variable] {
			continue
		}
		sort, getErr := p.manager.GetDomain(declaration, index)
		if getErr != nil {
			return 0, getErr
		}
		ruleSort, getErr := p.manager.IsDatalogRuleSort(sort)
		if getErr != nil {
			return 0, getErr
		}
		size := uint64(1)
		if !ruleSort {
			cardinality, sizeErr := p.manager.GetSortSize(sort)
			if sizeErr != nil {
				return 0, sizeErr
			}
			if cardinality.IsFinite() {
				size, sizeErr = cardinality.Size()
				if sizeErr != nil {
					return 0, sizeErr
				}
			} else {
				size = math.MaxUint64
			}
		}
		cost *= float32(size)
	}
	return cost, nil
}

func (p *MagicSetsPlugin) popBound(indices *[]int, rule *base.Rule, bound map[uint32]bool) (int, error) {
	best := -1
	var bestCost float32
	for pos, index := range *indices {
		lit, _, e := rule.Tail(index)
		if e != nil {
			return -1, e
		}
		count, e := p.boundCount(lit, bound)
		if e != nil {
			return -1, e
		}
		if count == 0 {
			continue
		}
		cost, e := p.unboundCost(lit, bound)
		if e != nil {
			return -1, e
		}
		if best < 0 || cost < bestCost {
			best, bestCost = pos, cost
		}
	}
	if best < 0 {
		return -1, nil
	}
	last := len(*indices) - 1
	(*indices)[best], (*indices)[last] = (*indices)[last], (*indices)[best]
	result := (*indices)[last]
	*indices = (*indices)[:last]
	return result, nil
}
func (p *MagicSetsPlugin) transformRule(task magicTask, rule *base.Rule, result *base.RuleSet) error {
	head, err := rule.Head()
	if err != nil {
		return err
	}
	bound := make(map[uint32]bool)
	for i, flag := range task.adornment {
		if !flag {
			continue
		}
		arg, e := p.manager.GetArg(head, uint32(i))
		if e != nil {
			return e
		}
		if k, e := p.manager.GetKind(arg); e != nil {
			return e
		} else if k == ast.ASTVar {
			v, e := p.manager.GetVarIndex(arg)
			if e != nil {
				return e
			}
			bound[v] = true
		}
	}
	ut, err := rule.UninterpretedTailSize()
	if err != nil {
		return err
	}
	var ext, intentional []int
	for i := 0; i < ut; i++ {
		d, e := rule.TailDeclaration(i)
		if e != nil {
			return e
		}
		if p.extensional[d] {
			ext = append(ext, i)
		} else {
			intentional = append(intentional, i)
		}
	}
	var tails []ast.ASTHandle
	var neg []bool
	for len(tails) < ut {
		index, e := p.popBound(&ext, rule, bound)
		isIntentional := false
		if e != nil {
			return e
		}
		if index < 0 {
			index, e = p.popBound(&intentional, rule, bound)
			isIntentional = index >= 0
			if e != nil {
				return e
			}
		}
		if index < 0 {
			if len(ext) > 0 {
				index = ext[len(ext)-1]
				ext = ext[:len(ext)-1]
			} else {
				index = intentional[len(intentional)-1]
				intentional = intentional[:len(intentional)-1]
				isIntentional = true
			}
		}
		lit, negative, e := rule.Tail(index)
		if e != nil {
			return e
		}
		if isIntentional {
			lit, e = p.adornLiteral(lit, bound)
			if e != nil {
				return e
			}
		}
		tails, neg = append(tails, lit), append(neg, negative)
		if e = collectLiteralVars(p.manager, lit, bound); e != nil {
			return e
		}
	}
	newHead, err := p.adornLiteral(head, func() map[uint32]bool {
		b := make(map[uint32]bool)
		for i, x := range task.adornment {
			if x {
				arg, _ := p.manager.GetArg(head, uint32(i))
				if k, _ := p.manager.GetKind(arg); k == ast.ASTVar {
					v, _ := p.manager.GetVarIndex(arg)
					b[v] = true
				}
			}
		}
		return b
	}())
	if err != nil {
		return err
	}
	headMagic, err := p.magicLiteral(newHead)
	if err != nil {
		return err
	}
	prefixTails := append([]ast.ASTHandle{headMagic}, tails...)
	prefixNeg := append([]bool{false}, neg...)
	for i, lit := range tails {
		d, e := p.manager.GetDecl(lit)
		if e != nil {
			return e
		}
		if p.extensional[d] {
			continue
		}
		mh, e := p.magicLiteral(lit)
		if e != nil {
			return e
		}
		if e = p.addRule(result, mh, prefixTails[:i+1], prefixNeg[:i+1], usymbol.Symbol{}, rule); e != nil {
			return e
		}
	}
	total, err := rule.TailSize()
	if err != nil {
		return err
	}
	for i := ut; i < total; i++ {
		lit, n, e := rule.Tail(i)
		if e != nil {
			return e
		}
		tails, neg = append(tails, lit), append(neg, n)
	}
	tails, neg = append(tails, headMagic), append(neg, false)
	name, err := rule.NameSymbol()
	if err != nil {
		return err
	}
	return p.addRule(result, newHead, tails, neg, name, rule)
}

// hasTransferFacts uses the concrete stored relation, including products with
// explanation columns, as source create_transfer_rule's generic empty check.
func (p *MagicSetsPlugin) hasTransferFacts(predicate ast.ASTHandle) (bool, error) {
	if p.factProbe != nil {
		populated, err := p.factProbe(predicate)
		if err != nil || populated {
			return populated, err
		}
	}
	if p.relations == nil {
		return false, nil
	}
	whole, found, err := p.relations.TryGetRelationExplanation(predicate)
	if err != nil {
		return false, err
	}
	if found {
		empty, err := whole.Empty()
		return !empty, err
	}
	product, found, err := p.relations.TryGetExplanationRelation(predicate)
	if err != nil {
		return false, err
	}
	if found {
		rows, err := product.Len()
		return rows != 0, err
	}
	table, found, err := p.relations.TryGetFiniteRelation(predicate)
	if err != nil || !found {
		return false, err
	}
	rows, err := table.Len()
	return rows != 0, err
}

func (p *MagicSetsPlugin) addTransferRule(task magicTask, result *base.RuleSet) error {
	populated, err := p.hasTransferFacts(task.description.predicate)
	if err != nil || !populated {
		return err
	}
	arity, err := p.manager.GetArity(task.description.predicate)
	if err != nil {
		return err
	}
	arguments := make([]ast.ASTHandle, arity)
	for i := uint32(0); i < arity; i++ {
		sort, getErr := p.manager.GetDomain(task.description.predicate, i)
		if getErr != nil {
			return getErr
		}
		arguments[i], err = p.manager.MkVar(i, sort)
		if err != nil {
			return err
		}
	}
	original, err := p.manager.MkApp(task.description.predicate, arguments)
	if err != nil {
		return err
	}
	adorned, err := p.manager.MkApp(p.adorned[task.description], arguments)
	if err != nil {
		return err
	}
	magic, err := p.magicLiteral(adorned)
	if err != nil {
		return err
	}
	return p.addRule(result, adorned, []ast.ASTHandle{original, magic}, nil, usymbol.Symbol{}, nil)
}

func (p *MagicSetsPlugin) Transform(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	if p == nil || p.closed || p.manager == nil || p.rules == nil {
		return nil, base.ErrRuleTransformerOwner
	}
	if !p.enabled {
		return nil, nil
	}
	definitions, err := source.PredicateRules(p.goal)
	if err != nil {
		return nil, err
	}
	if len(definitions) != 1 {
		_ = closeRules(definitions)
		return nil, fmt.Errorf("muz/transforms: magic goal must have exactly one rule")
	}
	goalHead, err := definitions[0].Head()
	err = errors.Join(err, closeRules(definitions))
	if err != nil {
		return nil, err
	}
	p.adorned = make(map[magicDescription]ast.ASTHandle)
	p.patterns = make(map[ast.ASTHandle]magicAdornment)
	p.magic = make(map[ast.ASTHandle]ast.ASTHandle)
	p.freshOrigin = make(map[ast.ASTHandle]ast.ASTHandle)
	p.extensional = make(map[ast.ASTHandle]bool)
	p.todo = nil
	p.fresh = nil
	heads := make(map[ast.ASTHandle]bool)
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	for i := 0; i < count; i++ {
		r, e := source.Rule(i)
		if e != nil {
			return nil, e
		}
		d, e := r.HeadDeclaration()
		e = errors.Join(e, r.Close())
		if e != nil {
			return nil, e
		}
		heads[d] = true
	}
	for i := 0; i < count; i++ {
		r, e := source.Rule(i)
		if e != nil {
			return nil, e
		}
		ut, e := r.UninterpretedTailSize()
		for j := 0; e == nil && j < ut; j++ {
			d, x := r.TailDeclaration(j)
			e = x
			if !heads[d] {
				p.extensional[d] = true
			}
		}
		e = errors.Join(e, r.Close())
		if e != nil {
			return nil, e
		}
	}
	empty := map[uint32]bool{}
	_, err = p.adornLiteral(goalHead, empty)
	if err != nil {
		return nil, err
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
	if err = result.InheritPredicates(source); err != nil {
		return nil, err
	}
	for len(p.todo) > 0 {
		last := len(p.todo) - 1
		task := p.todo[last]
		p.todo = p.todo[:last]
		rules, e := source.PredicateRules(task.description.predicate)
		if e != nil {
			return nil, e
		}
		for _, r := range rules {
			e = p.transformRule(task, r, result)
			e = errors.Join(e, r.Close())
			if e != nil {
				return nil, e
			}
		}
		if e = p.addTransferRule(task, result); e != nil {
			return nil, e
		}
	}
	adornedGoal, err := p.adornLiteral(goalHead, empty)
	if err != nil {
		return nil, err
	}
	magicGoal, err := p.magicLiteral(adornedGoal)
	if err != nil {
		return nil, err
	}
	if err = p.addRule(result, magicGoal, nil, nil, usymbol.Symbol{}, nil); err != nil {
		return nil, err
	}
	if err = p.addRule(result, goalHead, []ast.ASTHandle{adornedGoal}, nil, usymbol.Symbol{}, nil); err != nil {
		return nil, err
	}
	keep = true
	return result, nil
}
func (p *MagicSetsPlugin) Close() error {
	if p == nil || p.closed {
		return nil
	}
	p.closed = true
	p.adorned = nil
	p.patterns = nil
	p.magic = nil
	p.freshOrigin = nil
	p.extensional = nil
	p.todo = nil
	p.fresh = nil
	p.factProbe = nil
	return nil
}
