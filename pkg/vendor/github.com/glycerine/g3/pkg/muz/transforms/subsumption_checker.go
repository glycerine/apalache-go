package transforms

// Source: src/muz/transforms/dl_mk_subsumption_checker.cpp/.h retained
// rule pruning. Relation-table cardinality discovery and model conversion are
// outside the fixedpoint preprocessing consumer.

import (
	"errors"
	"fmt"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

type SubsumptionCheckerPlugin struct {
	priority uint32
	enabled  bool
	closed   bool
}

func NewSubsumptionCheckerPlugin(enabled bool, priority uint32) *SubsumptionCheckerPlugin {
	if priority == 0 {
		priority = 31000
	}
	return &SubsumptionCheckerPlugin{priority: priority, enabled: enabled}
}

func (p *SubsumptionCheckerPlugin) Priority() uint32 { return p.priority }

func (p *SubsumptionCheckerPlugin) CanDestratifyNegation() bool { return false }

func (p *SubsumptionCheckerPlugin) Cancel() {}

func (p *SubsumptionCheckerPlugin) Close() error {
	if p == nil {
		return base.ErrRuleTransformerOwner
	}
	p.closed = true
	return nil
}

type subsumptionIndex struct {
	rules []*base.Rule
	facts map[ast.ASTHandle]bool
}

func newSubsumptionIndex() *subsumptionIndex {
	return &subsumptionIndex{facts: make(map[ast.ASTHandle]bool)}
}

func (i *subsumptionIndex) add(rule *base.Rule) error {
	tail, err := rule.TailSize()
	if err != nil {
		return err
	}
	if tail == 0 {
		head, e := rule.Head()
		if e != nil {
			return e
		}
		i.facts[head] = true
	}
	owned, err := rule.Retain()
	if err != nil {
		return err
	}
	i.rules = append(i.rules, owned)
	return nil
}

func (i *subsumptionIndex) close() (result error) {
	for position := len(i.rules) - 1; position >= 0; position-- {
		result = errors.Join(result, i.rules[position].Close())
	}
	i.rules = nil
	return result
}

func (i *subsumptionIndex) atomSubsumed(atom ast.ASTHandle) bool { return i.facts[atom] }

func (i *subsumptionIndex) ruleSubsumed(rule *base.Rule) (bool, error) {
	head, err := rule.Head()
	if err != nil {
		return false, err
	}
	if i.atomSubsumed(head) {
		return true, nil
	}
	for _, prior := range i.rules {
		same, e := prior.Same(rule)
		if e != nil {
			return false, e
		}
		if same {
			return true, nil
		}
	}
	return false, nil
}

func totalRule(rule *base.Rule) (bool, error) {
	tail, err := rule.TailSize()
	if err != nil || tail != 0 {
		return false, err
	}
	head, err := rule.Head()
	if err != nil {
		return false, err
	}
	// Nullary facts are total. Non-nullary totality requires a head containing
	// each variable exactly once; NewRule has already normalized their indices.
	manager, err := rule.ASTManager()
	if err != nil {
		return false, err
	}
	arity, err := manager.GetNumArgs(head)
	if err != nil {
		return false, err
	}
	seen := make(map[uint32]bool, arity)
	for index := uint32(0); index < arity; index++ {
		argument, e := manager.GetArg(head, index)
		if e != nil {
			return false, e
		}
		kind, e := manager.GetKind(argument)
		if e != nil || kind != ast.ASTVar {
			return false, e
		}
		variable, e := manager.GetVarIndex(argument)
		if e != nil || seen[variable] {
			return false, e
		}
		seen[variable] = true
	}
	return true, nil
}

func discoverTotalRules(source *base.RuleSet) (map[ast.ASTHandle]*base.Rule, error) {
	total := make(map[ast.ASTHandle]*base.Rule)
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	for index := 0; index < count; index++ {
		rule, e := source.Rule(index)
		if e != nil {
			return nil, e
		}
		isTotal, e := totalRule(rule)
		if e == nil && isTotal {
			declaration, x := rule.HeadDeclaration()
			if x != nil {
				e = x
			} else if total[declaration] == nil {
				total[declaration] = rule
				rule = nil
			}
		}
		if rule != nil {
			e = errors.Join(e, rule.Close())
		}
		if e != nil {
			closeTotalRules(total)
			return nil, e
		}
	}
	return total, nil
}

func closeTotalRules(total map[ast.ASTHandle]*base.Rule) (result error) {
	for declaration, rule := range total {
		result = errors.Join(result, rule.Close())
		delete(total, declaration)
	}
	return result
}

func transformSubsumedRule(manager *base.RuleManager, rule *base.Rule, total map[ast.ASTHandle]*base.Rule, index *subsumptionIndex) (_ *base.Rule, retain, changed bool, resultErr error) {
	head, err := rule.Head()
	if err != nil {
		return nil, false, false, err
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return nil, false, false, err
	}
	tailSize, err := rule.TailSize()
	if err != nil {
		return nil, false, false, err
	}
	tail := make([]ast.ASTHandle, 0, tailSize)
	negated := make([]bool, 0, tailSize)
	for position := 0; position < tailSize; position++ {
		atom, negative, e := rule.Tail(position)
		if e != nil {
			return nil, false, false, e
		}
		if position < uninterpreted {
			declaration, x := rule.TailDeclaration(position)
			if x != nil {
				return nil, false, false, x
			}
			if total[declaration] != nil || index.atomSubsumed(atom) {
				if negative {
					return nil, false, true, nil
				}
				changed = true
				continue
			}
			if !negative && atom == head {
				return nil, false, true, nil
			}
		}
		tail = append(tail, atom)
		negated = append(negated, negative)
	}
	if !changed {
		return nil, true, false, nil
	}
	name, err := rule.Name()
	if err != nil {
		return nil, false, false, err
	}
	replacement, err := manager.NewRule(head, tail, negated, name)
	return replacement, true, true, err
}

func subsumptionPass(source *base.RuleSet) (_ *base.RuleSet, changed bool, resultErr error) {
	manager, err := source.RuleManager()
	if err != nil {
		return nil, false, err
	}
	total, err := discoverTotalRules(source)
	if err != nil {
		return nil, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, closeTotalRules(total)) }()
	result, err := base.NewRuleSet(manager)
	if err != nil {
		return nil, false, err
	}
	keep := false
	defer func() {
		if !keep {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	if err = result.InheritPredicates(source); err != nil {
		return nil, false, err
	}
	count, err := source.Len()
	if err != nil {
		return nil, false, err
	}
	rules := make([]*base.Rule, 0, count)
	for position := 0; position < count; position++ {
		rule, e := source.Rule(position)
		if e != nil {
			for _, owned := range rules {
				e = errors.Join(e, owned.Close())
			}
			return nil, false, e
		}
		rules = append(rules, rule)
	}
	defer func() {
		for _, rule := range rules {
			resultErr = errors.Join(resultErr, rule.Close())
		}
	}()
	sort.SliceStable(rules, func(left, right int) bool {
		leftSize, _ := rules[left].TailSize()
		rightSize, _ := rules[right].TailSize()
		return leftSize < rightSize
	})
	includedTotal := make(map[ast.ASTHandle]bool)
	index := newSubsumptionIndex()
	defer func() { resultErr = errors.Join(resultErr, index.close()) }()
	for _, rule := range rules {
		declaration, e := rule.HeadDeclaration()
		if e != nil {
			return nil, false, e
		}
		candidate := rule
		var replacement *base.Rule
		if defining := total[declaration]; defining != nil {
			output, x := source.IsOutputPredicate(declaration)
			if x != nil {
				return nil, false, x
			}
			if !output || includedTotal[declaration] {
				changed = true
				continue
			}
			candidate = defining
			includedTotal[declaration] = true
			if same, x := candidate.Same(rule); x != nil {
				return nil, false, x
			} else if !same {
				changed = true
			}
		}
		replacement, retain, ruleChanged, x := transformSubsumedRule(manager, candidate, total, index)
		if x != nil {
			return nil, false, x
		}
		changed = changed || ruleChanged
		if !retain {
			continue
		}
		if replacement != nil {
			candidate = replacement
		}
		subsumed, x := index.ruleSubsumed(candidate)
		if x == nil && !subsumed {
			x = result.Add(candidate)
			if x == nil {
				x = index.add(candidate)
			}
		} else if x == nil {
			changed = true
		}
		if replacement != nil {
			x = errors.Join(x, replacement.Close())
		}
		if x != nil {
			return nil, false, x
		}
	}
	if !changed {
		return nil, false, nil
	}
	valid, err := result.Finalize()
	if err != nil {
		return nil, false, err
	}
	if !valid {
		return nil, false, fmt.Errorf("muz/transforms: subsumption checker produced unstratified negation")
	}
	keep = true
	return result, true, nil
}

func (p *SubsumptionCheckerPlugin) Transform(source *base.RuleSet) (*base.RuleSet, error) {
	if p == nil || p.closed {
		return nil, base.ErrRuleTransformerOwner
	}
	if !p.enabled {
		return nil, nil
	}
	result, _, err := subsumptionPass(source)
	return result, err
}
