package apalache

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// SMTListener observes the SMT-level objects introduced by rewriting rules.
type SMTListener interface {
	OnIntroduceSMTConstant(name string)
	OnIntroduceCell(id int64)
	OnSMTAssert(expression Expr, smtNodes int64)
}

// IdleSMTListener is the zero-overhead listener used when profiling is off.
type IdleSMTListener struct{}

func (IdleSMTListener) OnIntroduceSMTConstant(string) {}
func (IdleSMTListener) OnIntroduceCell(int64)         {}
func (IdleSMTListener) OnSMTAssert(Expr, int64)       {}

// RuleStat contains the counters attributed directly to one rewriting rule.
type RuleStat struct {
	RuleName            string
	Calls               int64
	SMTConstantsSelf    int64
	CellsSelf           int64
	SMTAssertionsSelf   int64
	SMTAssertionSizeSum int64
}

func NewRuleStat(ruleName string) *RuleStat { return &RuleStat{RuleName: ruleName} }

func (stat *RuleStat) SMTAssertionSizeAverage() int64 {
	if stat.SMTAssertionsSelf == 0 {
		return 0
	}
	return stat.SMTAssertionSizeSum / stat.SMTAssertionsSelf
}

// RuleStatLocator owns exactly one mutable statistics record per rule name.
type RuleStatLocator struct{ stats map[string]*RuleStat }

func NewRuleStatLocator() *RuleStatLocator {
	return &RuleStatLocator{stats: make(map[string]*RuleStat)}
}

func (locator *RuleStatLocator) RuleStat(ruleName string) *RuleStat {
	if locator.stats == nil {
		locator.stats = make(map[string]*RuleStat)
	}
	if stat, found := locator.stats[ruleName]; found {
		return stat
	}
	stat := NewRuleStat(ruleName)
	locator.stats[ruleName] = stat
	return stat
}

// Stats returns the records in Scala SortedMap key order.
func (locator *RuleStatLocator) Stats() []*RuleStat {
	result := make([]*RuleStat, 0, len(locator.stats))
	for _, stat := range locator.stats {
		result = append(result, stat)
	}
	sort.Slice(result, func(left, right int) bool { return result[left].RuleName < result[right].RuleName })
	return result
}

func (locator *RuleStatLocator) WriteStats(writer io.Writer) error {
	if writer == nil {
		return fmt.Errorf("profiling writer is nil")
	}
	if _, err := fmt.Fprintln(writer, "Rule profiling statistics"); err != nil {
		return err
	}
	horizontalRule := strings.Repeat("-", 80)
	if _, err := fmt.Fprintln(writer, horizontalRule); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "%20s %9s %9s %9s %9s %9s\n",
		"name", "calls", "cells", "smt-consts", "smt-asserts", "smt-avg-size"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer, horizontalRule); err != nil {
		return err
	}
	stats := locator.Stats()
	sort.SliceStable(stats, func(left, right int) bool {
		if stats[left].Calls != stats[right].Calls {
			return stats[left].Calls > stats[right].Calls
		}
		return stats[left].RuleName < stats[right].RuleName
	})
	for _, stat := range stats {
		if _, err := fmt.Fprintf(writer, "%-20s %9d %9d %9d %9d %9d\n",
			stat.RuleName, stat.Calls, stat.CellsSelf, stat.SMTConstantsSelf,
			stat.SMTAssertionsSelf, stat.SMTAssertionSizeAverage()); err != nil {
			return err
		}
	}
	return nil
}

// RuleStatListener attributes callbacks only to the innermost active rule.
type RuleStatListener struct {
	locator *RuleStatLocator
	stack   []*RuleStat
}

func NewRuleStatListener() *RuleStatListener {
	return &RuleStatListener{locator: NewRuleStatLocator()}
}

func (listener *RuleStatListener) Locator() *RuleStatLocator { return listener.locator }

func (listener *RuleStatListener) EnterRule(ruleName string) {
	stat := listener.locator.RuleStat(ruleName)
	stat.Calls++
	listener.stack = append(listener.stack, stat)
}

func (listener *RuleStatListener) ExitRule() {
	if len(listener.stack) == 0 {
		panic("RuleStatListener.ExitRule called with an empty stack")
	}
	listener.stack = listener.stack[:len(listener.stack)-1]
}

func (listener *RuleStatListener) current() *RuleStat {
	if len(listener.stack) == 0 {
		return nil
	}
	return listener.stack[len(listener.stack)-1]
}

func (listener *RuleStatListener) OnIntroduceSMTConstant(string) {
	if stat := listener.current(); stat != nil {
		stat.SMTConstantsSelf++
	}
}

func (listener *RuleStatListener) OnIntroduceCell(int64) {
	if stat := listener.current(); stat != nil {
		stat.CellsSelf++
	}
}

func (listener *RuleStatListener) OnSMTAssert(expression Expr, _ int64) {
	if stat := listener.current(); stat != nil {
		stat.SMTAssertionsSelf++
		stat.SMTAssertionSizeSum += int64(smtProfileExpressionSize(expression))
	}
}

func smtProfileExpressionSize(expression Expr) int {
	operation, ok := expression.(*OperExpr)
	if !ok {
		return 1
	}
	size := 1
	for _, argument := range operation.Arguments() {
		size += smtProfileExpressionSize(argument)
	}
	return size
}
