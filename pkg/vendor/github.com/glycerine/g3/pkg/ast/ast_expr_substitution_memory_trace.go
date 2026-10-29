package ast

// Source: src/ast/expr_substitution.cpp:29,31;
// src/util/util.h:223-225; src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"reflect"
	"strconv"

	udiag "github.com/glycerine/g3/pkg/util/diagnostic"
)

func (m *ASTManager) newExprSubstOptionalTable(line int, name string) (*astExprSubstTable, error) {
	header := uint64(reflect.TypeOf(astExprSubstTable{}).Size())
	if e := m.BudgetField.Reserve(header); e != nil {
		return nil, e
	}
	table := &astExprSubstTable{header: header}
	m.traceExprSubstTableAllocated(table, line, name)
	return table, nil
}

func (m *ASTManager) releaseExprSubstOptionalTables(state *astExprSubstState) {
	if state.dep != nil {
		table := state.dep
		m.BudgetField.Release(table.header)
		table.header = 0
		m.traceExprSubstTableDeallocated(table)
		state.dep = nil
	}
	if state.proof != nil {
		table := state.proof
		m.BudgetField.Release(table.header)
		table.header = 0
		m.traceExprSubstTableDeallocated(table)
		state.proof = nil
	}
}

func (m *ASTManager) traceExprSubstTableAllocated(table *astExprSubstTable, line int, name string) {
	tags := m.astMemoryTraceTags()
	if !udiag.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "alloc "+fmt.Sprintf("%p", table)+
				" src/ast/expr_substitution.cpp:"+strconv.Itoa(line)+" "+name+" "+
				strconv.FormatUint(uint64(reflect.TypeOf(astExprSubstTable{}).Size()), 10)+"\n")
			return nil
		})
}

func (m *ASTManager) traceExprSubstTableDeallocated(table *astExprSubstTable) {
	tags := m.astMemoryTraceTags()
	if !udiag.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return !m.astMemoryFinalizing(tags) }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "dealloc "+fmt.Sprintf("%p", table)+" src/util/util.h:224\n")
			return nil
		})
}
