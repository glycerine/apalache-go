package ast

// Source: src/ast/ast.cpp:1297-1479; src/ast/ast.h:1454-1490.

import (
	"reflect"
)

func (m *ASTManager) Stats() ASTManagerStats {
	if m == nil || m.Self != m {
		return ASTManagerStats{}
	}
	if m.traceExecuting || m.formatExecuting {
		panic(ErrASTPending)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Closed {
		return ASTManagerStats{}
	}
	s := ASTManagerStats{LiveNodes: m.liveNodes, LiveParameters: m.liveParameters, LiveInfos: m.liveInfos, LiveReferences: m.liveReferences, OwnerBytes: m.ownerBytes, PageBytes: m.pageBytes, DirectoryBytes: m.directoryBytes}
	s.AllocatedBytes = m.datatypePluginBytes() + m.auxiliaryPluginBytes() + m.floatingPointPluginBytes() + m.sequencePluginBytes() + m.arrayCardinalityBytes() + m.arrayPluginBytes() + m.arithmeticPluginBytes() + m.bitVectorPluginBytes() + m.formatPluginBytes() + m.formatFramesBytes() + m.ppFramesBytes() + s.OwnerBytes + s.PageBytes + s.DirectoryBytes
	for _, state := range m.scopedExprSubstitutions {
		s.AllocatedBytes += state.header
	}
	for _, state := range m.exprSubstitutions {
		s.AllocatedBytes += state.header
	}
	s.AllocatedBytes += m.Limit.StorageBytes()
	for f := m.traceAcquisitionHistory; f != nil; f = f.historyNext {
		s.AllocatedBytes += f.bytes
	}
	for f := m.traceAcquisitions; f != nil; f = f.previous {
		s.AllocatedBytes += f.bytes
	}
	if f := m.traceAcquisitionExecuting; f != nil {
		s.AllocatedBytes += f.bytes
	}
	s.AllocatedBytes += m.exprVisit.bytes + m.exprVisit.localFast.heapBytes
	for h := m.exprVisitHistory; h != nil; h = h.next {
		s.AllocatedBytes += h.bytes
	}
	width := uint64(reflect.TypeOf(astNodeRef{}).Size())
	s.AllocatedBytes += uint64(cap(m.basic.eq)+cap(m.basic.oeq)+cap(m.basic.ite)) * width
	for _, v := range m.basic.proofCaches {
		s.AllocatedBytes += uint64(cap(v)) * width
	}
	if m.allocator != nil {
		a := m.allocator.Stats()
		s.AllocatedBytes += a.OwnerBytes + a.PayloadBytes + a.MetadataBytes + a.DirectoryBytes
	}
	if m.families != nil {
		f := m.families.Stats()
		s.AllocatedBytes += f.OwnerBytes
	}
	if m.Format != nil {
		s.AllocatedBytes += m.Format.Stats().AllocatedBytes
	}
	if m.constructorPendingFormat != nil {
		s.AllocatedBytes += m.constructorPendingFormat.Stats().AllocatedBytes
	}
	for _, file := range []*astTraceFile{m.TraceFile, m.constructorPendingFile} {
		if file != nil {
			x := file.stream.Stats()
			s.AllocatedBytes += file.header + x.ScratchBytes + x.OutputOwnerBytes + x.OutputOverflowBytes
		}
	}
	return s
}
