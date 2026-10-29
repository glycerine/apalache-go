package ast

// Source: src/util/sorting_network.h:120-1046; src/test/sorting_network.cpp:150-181.

import (
	"errors"
	"math"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"

	udiag "github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

var ErrPsortUninitializedComparison = errors.New("g3: reached uninitialized psort comparison mode")

const (
	psortLE uint8 = iota
	psortGE
	psortEQ
	psortGEFull
	psortLEFull
)

type PsortASTStats struct{ CompiledVariables, CompiledClauses, ClauseVariables uint32 }
type PsortASTNetwork struct {
	self            *PsortASTNetwork
	context         *PsortASTContext
	runtime         *umemory.Runtime
	budget          *arena.Budget
	debug           *udiag.DebugContext
	trace           *udiag.TraceTags
	header          uint64
	mode            uint8
	modeInitialized bool
	stats           PsortASTStats
	frames          *psortASTVectorFrame
	serial          uint64
	running, closed bool
	sourceDone      bool
}
type psortASTVectorFrame struct {
	next               *psortASTVectorFrame
	owner              *PsortASTNetwork
	invocation, header uint64
	vectors            []ASTLiteralVector
	cursor             uint32
	sourceDone         bool
}

// psortASTPointer denotes a call-local source pointer. Small literal arrays
// remain inline values; dynamic pointers borrow a captured native slice.
type psortASTPointer struct {
	span   ASTLiteralVectorSpan
	inline *[3]ASTHandle
	offset uint64
}

func (p psortASTPointer) at(i uint32) (ASTHandle, error) {
	index := p.offset + uint64(i)
	if index < p.offset {
		return ASTHandle{}, arena.ErrBounds
	}
	if p.inline != nil {
		if index >= 3 {
			return ASTHandle{}, arena.ErrBounds
		}
		return p.inline[index], nil
	}
	if index > math.MaxUint32 {
		return ASTHandle{}, arena.ErrBounds
	}
	return p.span.At(uint32(index))
}
func (p psortASTPointer) add(n uint32) psortASTPointer { p.offset += uint64(n); return p }
func psortVectorPointer(v *ASTLiteralVector) (psortASTPointer, error) {
	p, e := v.Span()
	return psortASTPointer{span: p}, e
}
func NewPsortASTNetwork(c *PsortASTContext, r *umemory.Runtime, b *arena.Budget, d *udiag.DebugContext) (*PsortASTNetwork, error) {
	if c == nil || c.self != c || r == nil || b == nil || c.budget != b || r.Budget() != b || d != nil && (!d.ValidOwner() || d.Budget() != b) {
		return nil, arena.ErrConfig
	}
	if e := c.open(); e != nil {
		return nil, e
	}
	if e := r.CheckOpen(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(PsortASTNetwork{}).Size())
	if e := b.Reserve(bytes); e != nil {
		return nil, e
	}
	s := &PsortASTNetwork{context: c, runtime: r, budget: b, debug: d, header: bytes}
	s.self = s
	return s, nil
}

// BindTrace borrows the retained TRACE tag/output owner before an operation.
// A nil binding selects the source process trace stream.
func (s *PsortASTNetwork) BindTrace(t *udiag.TraceTags) error {
	if e := s.open(); e != nil {
		return e
	}
	s.trace = t
	return nil
}
func (s *PsortASTNetwork) open() error {
	if s == nil || s.self != s {
		return ErrASTHandle
	}
	if s.closed || s.sourceDone {
		return arena.ErrClosed
	}
	if s.running {
		return ErrASTPending
	}
	return s.context.open()
}
func (s *PsortASTNetwork) assertions() (bool, error) {
	if !udiag.DebugBuild {
		return false, nil
	}
	if s.debug == nil {
		return true, nil
	}
	return s.debug.AssertionsEnabled()
}
func (s *PsortASTNetwork) assertion(line uint32, text string) error {
	return udiag.DebugAssertionFailure(s.debug, "src/util/sorting_network.h", int32(line), text)
}
func (s *PsortASTNetwork) comparison() (uint8, error) {
	if !s.modeInitialized {
		return 0, ErrPsortUninitializedComparison
	}
	return s.mode, nil
}
func (s *PsortASTNetwork) beginFrame(n uint32) (*psortASTVectorFrame, error) {
	if s.serial == math.MaxUint64 {
		return nil, arena.ErrSlotLimit
	}
	bytes := uint64(reflect.TypeOf(psortASTVectorFrame{}).Size()) + uint64(n)*uint64(reflect.TypeOf(ASTLiteralVector{}).Size())
	if e := s.budget.Reserve(bytes); e != nil {
		return nil, e
	}
	s.serial++
	f := &psortASTVectorFrame{owner: s, invocation: s.serial, header: bytes, vectors: make([]ASTLiteralVector, n), next: s.frames}
	s.frames = f
	for i := range f.vectors {
		v := &f.vectors[i]
		v.self = v
		v.budget = s.budget
	}
	return f, nil
}
func (s *PsortASTNetwork) sourceFrame(f *psortASTVectorFrame) error {
	if f.owner != s || f.invocation == 0 || f.invocation > s.serial {
		return ErrASTHandle
	}
	for f.cursor != 0 {
		v := &f.vectors[f.cursor-1]
		if e := v.SourceFinalize(); e != nil {
			return e
		}
		f.cursor--
	}
	f.sourceDone = true
	return nil
}
func (s *PsortASTNetwork) ownFrame(f *psortASTVectorFrame) error {
	if f.owner != s || f.invocation == 0 || f.invocation > s.serial {
		return ErrASTHandle
	}
	for i := len(f.vectors) - 1; i >= 0; i-- {
		v := &f.vectors[i]
		if e := v.Close(); e != nil {
			return e
		}
	}
	link := &s.frames
	for *link != nil && *link != f {
		link = &(*link).next
	}
	if *link != f {
		return ErrASTHandle
	}
	*link = f.next
	s.budget.Release(f.header)
	f.header = 0
	return nil
}
func (s *PsortASTNetwork) finishFrame(f *psortASTVectorFrame, body error) error {
	if e := s.sourceFrame(f); e != nil {
		return errors.Join(body, e)
	}
	if e := s.ownFrame(f); e != nil {
		return errors.Join(body, e)
	}
	return body
}
func (s *PsortASTNetwork) Close() error {
	if s == nil || s.self != s {
		return ErrASTHandle
	}
	if s.closed {
		return nil
	}
	if s.running {
		return ErrASTPending
	}
	s.running = true
	defer func() { s.running = false }()
	for s.frames != nil {
		f := s.frames
		if !f.sourceDone {
			if e := s.sourceFrame(f); e != nil {
				return e
			}
		}
		if e := s.ownFrame(f); e != nil {
			return e
		}
	}
	s.budget.Release(s.header)
	s.header = 0
	s.closed = true
	return nil
}
func (s *PsortASTNetwork) Stats() (PsortASTStats, error) {
	if s == nil || s.self != s || s.closed {
		return PsortASTStats{}, ErrASTHandle
	}
	return s.stats, nil
}
func (s *PsortASTNetwork) fresh() (ASTHandle, error) {
	s.stats.CompiledVariables++
	return s.context.Fresh()
}
func (s *PsortASTNetwork) maximum(a, b ASTHandle) (ASTHandle, error) {
	if a == b {
		return a, nil
	}
	s.stats.CompiledVariables++
	return s.context.Max(a, b)
}
func (s *PsortASTNetwork) minimum(a, b ASTHandle) (ASTHandle, error) {
	if a == b {
		return a, nil
	}
	s.stats.CompiledVariables++
	return s.context.Min(a, b)
}
func (s *PsortASTNetwork) not(a ASTHandle) (ASTHandle, error) { return s.context.Not(a) }
func (s *PsortASTNetwork) clause(n uint32, p psortASTPointer) (result error) {
	s.stats.CompiledClauses++
	s.stats.ClauseVariables += n
	f, e := s.beginFrame(1)
	if e != nil {
		return e
	}
	defer func() { result = s.finishFrame(f, result) }()
	tmp := &f.vectors[0]
	if e = s.rangeVector(f, 0, n, p); e != nil {
		return e
	}
	span, e := tmp.Span()
	if e != nil {
		return e
	}
	return s.context.Clause(n, span)
}
func (s *PsortASTNetwork) clause2(a, b ASTHandle) error {
	ls := [3]ASTHandle{a, b}
	return s.clause(2, psortASTPointer{inline: &ls})
}
func (s *PsortASTNetwork) clause3(a, b, c ASTHandle) error {
	ls := [3]ASTHandle{a, b, c}
	return s.clause(3, psortASTPointer{inline: &ls})
}
func (s *PsortASTNetwork) clauseVector(v *ASTLiteralVector) error {
	n, e := v.Size()
	if e != nil {
		return e
	}
	p, e := psortVectorPointer(v)
	if e != nil {
		return e
	}
	return s.clause(n, p)
}
func (s *PsortASTNetwork) appendPointer(v *ASTLiteralVector, n uint32, p psortASTPointer) error {
	for i := uint32(0); i < n; i++ {
		h, e := p.at(i)
		if e != nil {
			return e
		}
		if e = v.Push(h); e != nil {
			return e
		}
	}
	return nil
}
func (s *PsortASTNetwork) cmpGE(x1, x2, y1, y2 ASTHandle) error {
	n, e := s.not(y2)
	if e != nil {
		return e
	}
	if e = s.clause2(n, x1); e != nil {
		return e
	}
	n, e = s.not(y2)
	if e != nil {
		return e
	}
	if e = s.clause2(n, x2); e != nil {
		return e
	}
	n, e = s.not(y1)
	if e != nil {
		return e
	}
	return s.clause3(n, x1, x2)
}
func (s *PsortASTNetwork) cmpLE(x1, x2, y1, y2 ASTHandle) error {
	n, e := s.not(x1)
	if e != nil {
		return e
	}
	if e = s.clause2(n, y1); e != nil {
		return e
	}
	n, e = s.not(x2)
	if e != nil {
		return e
	}
	if e = s.clause2(n, y1); e != nil {
		return e
	}
	n, e = s.not(x1)
	if e != nil {
		return e
	}
	m, e := s.not(x2)
	if e != nil {
		return e
	}
	return s.clause3(n, m, y2)
}
func (s *PsortASTNetwork) compare(x1, x2, y1, y2 ASTHandle) error {
	mode, e := s.comparison()
	if e != nil {
		return e
	}
	switch mode {
	case psortLE, psortLEFull:
		return s.cmpLE(x1, x2, y1, y2)
	case psortGE, psortGEFull:
		return s.cmpGE(x1, x2, y1, y2)
	case psortEQ:
		if e = s.cmpGE(x1, x2, y1, y2); e != nil {
			return e
		}
		return s.cmpLE(x1, x2, y1, y2)
	}
	return nil
}
func (s *PsortASTNetwork) Ge(full bool, k, n uint32, p ASTLiteralVectorSpan) (ASTHandle, error) {
	if e := s.open(); e != nil {
		return ASTHandle{}, e
	}
	s.running = true
	defer func() { s.running = false }()
	return s.ge(full, k, n, psortASTPointer{span: p})
}
func (s *PsortASTNetwork) Le(full bool, k, n uint32, p ASTLiteralVectorSpan) (ASTHandle, error) {
	if e := s.open(); e != nil {
		return ASTHandle{}, e
	}
	s.running = true
	defer func() { s.running = false }()
	return s.le(full, k, n, psortASTPointer{span: p})
}
func (s *PsortASTNetwork) Eq(full bool, k, n uint32, p ASTLiteralVectorSpan) (ASTHandle, error) {
	if e := s.open(); e != nil {
		return ASTHandle{}, e
	}
	s.running = true
	defer func() { s.running = false }()
	return s.eq(full, k, n, psortASTPointer{span: p})
}
func (s *PsortASTNetwork) Sorting(n uint32, p ASTLiteralVectorSpan, out *ASTLiteralVector) error {
	if e := s.open(); e != nil {
		return e
	}
	if out == nil || out.self != out {
		return ErrASTHandle
	}
	s.running = true
	defer func() { s.running = false }()
	return s.sorting(n, psortASTPointer{span: p}, out)
}

func (s *PsortASTNetwork) defaultVector(f *psortASTVectorFrame, i uint32) error {
	if f.owner != s || f.invocation == 0 || f.invocation > s.serial || i != f.cursor || int(i) >= len(f.vectors) {
		return ErrASTHandle
	}
	f.vectors[i].constructed = true
	f.cursor++
	return nil
}
func (s *PsortASTNetwork) defaultVectors(f *psortASTVectorFrame) error {
	for f.cursor < uint32(len(f.vectors)) {
		if e := s.defaultVector(f, f.cursor); e != nil {
			return e
		}
	}
	return nil
}
func (s *PsortASTNetwork) rangeVector(f *psortASTVectorFrame, i, n uint32, p psortASTPointer) error {
	if f.owner != s || f.invocation == 0 || f.invocation > s.serial || i != f.cursor || int(i) >= len(f.vectors) {
		return ErrASTHandle
	}
	v := &f.vectors[i]
	for j := uint32(0); j < n; j++ {
		h, e := p.at(j)
		if e != nil {
			return e
		}
		v.values = append(v.values, h)
	}
	v.constructed = true
	f.cursor++
	return nil
}

func (s *PsortASTStats) Reset() { *s = PsortASTStats{} }
func (s *PsortASTNetwork) ResetStats() error {
	if e := s.open(); e != nil {
		return e
	}
	s.stats.Reset()
	return nil
}

// SourceFinalize invalidates captured spans from live recursive scratch.
// Close also releases the charged network and frame owner headers.
func (s *PsortASTNetwork) SourceFinalize() error {
	if s == nil || s.self != s {
		return ErrASTHandle
	}
	if s.running {
		return ErrASTPending
	}
	if s.closed || s.sourceDone {
		return nil
	}
	s.running = true
	defer func() { s.running = false }()
	for f := s.frames; f != nil; f = f.next {
		if e := s.sourceFrame(f); e != nil {
			return e
		}
	}
	s.sourceDone = true
	return nil
}
