package ast

// Source: src/ast/ast.cpp:2114-2134; src/ast/ast.cpp:1714-1733.

import (
	"errors"
	"math"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

// Cold Go acquisition metadata, prepared before source placement publication.
// Tokens are stamped only at successful INC leaves; this is not a second app
// argument payload. Source catch never consults them to undo registration.
type astTraceAcquisition struct {
	tableSourceEnded, referencesSourceEnded  bool
	historyNext                              *astTraceAcquisition
	retired                                  bool
	owner                                    *ASTManager
	previous                                 *astTraceAcquisition
	invocation                               uint64
	node                                     astNodeRef
	hash                                     uint32
	references                               []astNodeRef
	acquired, ownIndex                       uint32
	caught, placementConsumed, tableConsumed bool
	bodyCause, sourceCause, ownCause         error
	bytes                                    uint64
}

func (m *ASTManager) prepareTraceAcquisition(count uint32) (*astTraceAcquisition, error) {
	if m.TraceStream == nil {
		return nil, nil
	}
	if m.traceAcquisitionExecuting != nil || m.traceAcquisitionSerial == math.MaxUint64 {
		return nil, ErrASTPending
	}
	n := uint64(count) + 1
	width := uint64(reflect.TypeOf(astNodeRef{}).Size())
	if n > uint64(^uint(0)>>1)/width {
		return nil, arena.ErrBounds
	}
	bytes := uint64(reflect.TypeOf(astTraceAcquisition{}).Size()) + n*width
	if e := m.BudgetField.Reserve(bytes); e != nil {
		return nil, e
	}
	m.traceAcquisitionSerial++
	f := &astTraceAcquisition{owner: m, invocation: m.traceAcquisitionSerial, references: make([]astNodeRef, int(n)), bytes: bytes}
	m.traceAcquisitionExecuting = f
	return f, nil
}
func (m *ASTManager) retireTraceAcquisition(f *astTraceAcquisition) {
	if f != nil {
		m.BudgetField.Release(f.bytes)
		f.bytes = 0
		f.references = nil
	}
}
func (m *ASTManager) finishTraceAcquisition(f *astTraceAcquisition) {
	if f == nil {
		return
	}
	m.traceAcquisitionExecuting = nil
	if !f.caught {
		m.retireTraceAcquisition(f)
		return
	}
	f.previous = m.traceAcquisitions
	m.traceAcquisitions = f
}
func (m *ASTManager) catchTraceAllocation(f *astTraceAcquisition, e error) error {
	if f == nil || f.owner != m || f.node.slot == 0 {
		return e
	}
	f.caught = true
	f.bodyCause = e
	// The source catch frees placement, and leaves table/child references intact.
	x := m.freeNodeStorage(f.node)
	if x == nil {
		f.placementConsumed = true
	} else {
		f.sourceCause = x
	}
	return errors.Join(e, x)
}
func (m *ASTManager) cleanupTraceAcquisitions() error {
	for f := m.traceAcquisitions; f != nil; f = m.traceAcquisitions {
		if f.owner != m || f.invocation == 0 || !f.caught {
			return ErrASTHandle
		}
		stop := func(e error) error {
			if e != nil && f.ownCause == nil {
				f.ownCause = e
			}
			return e
		}
		if !f.placementConsumed {
			if e := m.freeNodeStorage(f.node); e != nil {
				return stop(e)
			}
			f.placementConsumed = true
		}
		if !f.tableConsumed {
			if e := m.tableEraseCaptured(f.node, f.hash); e != nil {
				return stop(e)
			}
			f.tableConsumed = true
		}
		for f.ownIndex < f.acquired {
			consumed, e := m.decNode(f.references[f.ownIndex])
			if consumed {
				f.ownIndex++
			}
			if e != nil {
				return stop(e)
			}
		}
		m.traceAcquisitions = f.previous
		f.retired = true
		f.historyNext = m.traceAcquisitionHistory
		m.traceAcquisitionHistory = f
	}
	return nil
}

// Only the end of the concrete owning context releases retired receipt storage.
// Before that point every exact child token/cause remains available, with
// retirement separate from acquisition authority and charged in Stats.
func (m *ASTManager) releaseTraceAcquisitionHistory() {
	for f := m.traceAcquisitionHistory; f != nil; f = f.historyNext {
		m.retireTraceAcquisition(f)
	}
	m.traceAcquisitionHistory = nil
}

// Actual constructor member actions end their own table/allocator contexts.
// These action stamps do not erase source membership or replay reference DEC.
func (m *ASTManager) traceConstructorTableEnded() {
	for f := m.traceAcquisitions; f != nil; f = f.previous {
		f.tableSourceEnded = true
	}
	for f := m.traceAcquisitionHistory; f != nil; f = f.historyNext {
		f.tableSourceEnded = true
	}
}
func (m *ASTManager) traceConstructorNodesEnded() {
	for f := m.traceAcquisitions; f != nil; f = f.previous {
		f.referencesSourceEnded = true
	}
	for f := m.traceAcquisitionHistory; f != nil; f = f.historyNext {
		f.referencesSourceEnded = true
	}
}
func (m *ASTManager) retireConstructorTraceAcquisitions() error {
	for f := m.traceAcquisitions; f != nil; f = m.traceAcquisitions {
		if f.owner != m || f.invocation == 0 || !f.caught || !f.placementConsumed || !f.tableSourceEnded || !f.referencesSourceEnded {
			return ErrASTPending
		}
		m.traceAcquisitions = f.previous
		f.retired = true
		f.historyNext = m.traceAcquisitionHistory
		m.traceAcquisitionHistory = f
	}
	return nil
}
