package base

// Source: src/muz/base/dl_rule_subsumption_index.h and
// src/muz/base/dl_rule_subsumption_index.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

var ErrRuleSubsumptionIndex = errors.New("muz/base: invalid or closed rule subsumption index")

// RuleSubsumptionIndex is the source under-approximation: it recognizes exact
// structural rules and exact unconditional heads. It owns every added rule.
type RuleSubsumptionIndex struct {
	self          *RuleSubsumptionIndex
	manager       *RuleManager
	rules         []*Rule
	buckets       map[uint32][]*Rule
	unconditioned map[ast.ASTHandle]map[ast.ASTHandle]bool
	closed        bool
}

func NewRuleSubsumptionIndex(manager *RuleManager) (*RuleSubsumptionIndex, error) {
	if manager == nil {
		return nil, ErrRuleSubsumptionIndex
	}
	if err := manager.open(); err != nil {
		return nil, err
	}
	index := &RuleSubsumptionIndex{
		manager:       manager,
		buckets:       make(map[uint32][]*Rule),
		unconditioned: make(map[ast.ASTHandle]map[ast.ASTHandle]bool),
	}
	index.self = index
	return index, nil
}

func (i *RuleSubsumptionIndex) open() error {
	if i == nil || i.self != i || i.closed || i.manager == nil {
		return ErrRuleSubsumptionIndex
	}
	return i.manager.open()
}

func (i *RuleSubsumptionIndex) Add(rule *Rule) error {
	if err := i.open(); err != nil {
		return err
	}
	state, err := rule.open()
	if err != nil {
		return err
	}
	if state.manager != i.manager.manager {
		return ErrRuleSubsumptionIndex
	}
	hash, err := rule.Hash()
	if err != nil {
		return err
	}
	owned, err := rule.Retain()
	if err != nil {
		return err
	}
	i.rules = append(i.rules, owned)
	i.buckets[hash] = append(i.buckets[hash], owned)
	if len(state.tail) == 0 {
		declaration, err := state.manager.GetDecl(state.head)
		if err != nil {
			return err
		}
		set := i.unconditioned[declaration]
		if set == nil {
			set = make(map[ast.ASTHandle]bool)
			i.unconditioned[declaration] = set
		}
		set[state.head] = true
	}
	return nil
}

func (i *RuleSubsumptionIndex) IsSubsumedApplication(query ast.ASTHandle) (bool, error) {
	if err := i.open(); err != nil {
		return false, err
	}
	kind, err := i.manager.manager.GetKind(query)
	if err != nil {
		return false, err
	}
	if kind != ast.ASTApp {
		return false, ast.ErrASTDomain
	}
	declaration, err := i.manager.manager.GetDecl(query)
	if err != nil {
		return false, err
	}
	return i.unconditioned[declaration][query], nil
}

func (i *RuleSubsumptionIndex) IsSubsumedRule(rule *Rule) (bool, error) {
	if err := i.open(); err != nil {
		return false, err
	}
	head, err := rule.Head()
	if err != nil {
		return false, err
	}
	if found, err := i.IsSubsumedApplication(head); err != nil || found {
		return found, err
	}
	hash, err := rule.Hash()
	if err != nil {
		return false, err
	}
	for _, candidate := range i.buckets[hash] {
		equal, err := candidate.Equal(rule)
		if err != nil {
			return false, err
		}
		if equal {
			return true, nil
		}
	}
	return false, nil
}

func (i *RuleSubsumptionIndex) Reset() (result error) {
	if err := i.open(); err != nil {
		return err
	}
	for position := len(i.rules) - 1; position >= 0; position-- {
		result = errors.Join(result, i.rules[position].Close())
	}
	i.rules = nil
	i.buckets = make(map[uint32][]*Rule)
	i.unconditioned = make(map[ast.ASTHandle]map[ast.ASTHandle]bool)
	return result
}

func (i *RuleSubsumptionIndex) Close() error {
	if i == nil || i.self != i {
		return ErrRuleSubsumptionIndex
	}
	if i.closed {
		return nil
	}
	if err := i.Reset(); err != nil {
		return err
	}
	i.closed = true
	i.manager = nil
	return nil
}
