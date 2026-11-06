package diagnostic

// Source: src/util/trace.cpp:25-55; src/util/trace.h:40-46.

import (
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/glycerine/g3/pkg/util/memory"
)

// TraceTags is the caller-owned, single-goroutine trace-tag control state.
// The retained shell and API callers can bind one shared instance.
type TraceTags struct {
	outputDepth uint32 // actual synchronous host Write/WriteString/Flush callbacks
	all         bool
	tags        map[string]struct{}
	writer      io.Writer
	openErr     error
	// Source tout keeps badbit across records until an explicit reopen.
	streamErr     error
	memoryProcess *memory.ProcessAccounting
}

// OutputCallbackActive lets a caller reject reentry into a lock held across
// this concrete trace stream. Ordinary trace-body computation is excluded.
func (t *TraceTags) OutputCallbackActive() bool {
	return t != nil && atomic.LoadUint32(&t.outputDepth) != 0
}

func NewTraceTags() *TraceTags { return &TraceTags{} }

// The source tag table and trace stream are process globals. An explicit
// instance may borrow a writer for a concrete caller or a focused test.
var processTraceTags = newProcessTraceTags()

func BorrowProcessTraceTags() *TraceTags { return processTraceTags }

func NewTraceTagsWithWriter(w io.Writer) *TraceTags { return &TraceTags{writer: w} }

func traceTagCString(tag string) string {
	if i := strings.IndexByte(tag, 0); i >= 0 {
		return tag[:i]
	}
	return tag
}

func (t *TraceTags) Enable(tag string) {
	if t.tags == nil {
		fresh := make(map[string]struct{})
		t.traceTagTableAllocated()
		t.tags = fresh
	}
	t.tags[traceTagCString(tag)] = struct{}{}
}

func (t *TraceTags) Disable(tag string) {
	if t.tags == nil {
		fresh := make(map[string]struct{})
		t.traceTagTableAllocated()
		t.tags = fresh
	}
	delete(t.tags, traceTagCString(tag))
}

func (t *TraceTags) EnableAll(flag bool) { t.all = flag }

func (t *TraceTags) IsEnabled(tag string) bool {
	if t.all {
		return true
	}
	_, found := t.tags[traceTagCString(tag)]
	return found
}

// ShouldTrace includes the source _TRACE compile gate before the tag test.
// Callers use it before constructing the trace body, matching TRACE_CODE.
func (t *TraceTags) ShouldTrace(tag string) bool {
	return traceCompiled && t != nil && t.IsEnabled(tag)
}

// IsStreamFailure identifies an ostream refusal returned by an emit call.
// Callers with source side effects can continue after the stream turns bad,
// while still propagating errors from computing a trace operand.
func (t *TraceTags) IsStreamFailure(err error) bool {
	if t == nil || err == nil {
		return false
	}
	return t.streamErr != nil && errors.Is(err, t.streamErr) ||
		t.openErr != nil && errors.Is(err, t.openErr)
}

func (t *TraceTags) traceWriter() (io.Writer, error) {
	if t.openErr != nil {
		return nil, t.openErr
	}
	if t.writer == nil {
		f, err := os.OpenFile(".z3-trace", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
		if err != nil {
			t.openErr = err
			return nil, err
		}
		t.writer = f
	}
	return t.writer, nil
}

func writeTracePart(w io.Writer, part string) error {
	n, err := io.WriteString(w, part)
	if err != nil {
		return err
	}
	if n != len(part) {
		return io.ErrShortWrite
	}
	return nil
}

func writeTraceInt32(w io.Writer, value int32) error {
	if out, ok := w.(interface{ AppendInt32(int32) error }); ok {
		return out.AppendInt32(value)
	}
	return writeTracePart(w, strconv.FormatInt(int64(value), 10))
}

func flushTrace(w io.Writer) error {
	if f, ok := w.(interface{ Flush() error }); ok {
		return f.Flush()
	}
	return nil
}

// The source ostream keeps its first failure while later insertion statements
// and the final flush still execute. Suppress later physical writes but allow
// a TRACE/CTRACE body to run after a refused header insertion.
type traceStickyWriter struct {
	owner     *TraceTags
	out       io.Writer
	first     error
	streamErr error
}

func (w *traceStickyWriter) enterOutput() {
	if w.owner != nil {
		atomic.AddUint32(&w.owner.outputDepth, 1)
	}
}
func (w *traceStickyWriter) leaveOutput() {
	if w.owner != nil {
		atomic.AddUint32(&w.owner.outputDepth, ^uint32(0))
	}
}

func (w *traceStickyWriter) capture(err error) {
	if err == nil {
		return
	}
	if w.first == nil {
		w.first = err
	}
	if w.streamErr == nil {
		w.streamErr = err
	}
	// Nested TRACE records share one actual ostream state immediately.
	if w.owner != nil && w.owner.streamErr == nil {
		w.owner.streamErr = w.streamErr
	}
}
func (w *traceStickyWriter) syncFailure() {
	if w.owner != nil && w.owner.streamErr != nil {
		w.capture(w.owner.streamErr)
	}
}
func (w *traceStickyWriter) remember(n, want int, err error) (int, error) {
	if err == nil && n != want {
		err = io.ErrShortWrite
	}
	w.capture(err)
	if err != nil && w.streamErr == nil {
		w.streamErr = err
	}
	return n, err
}
func (w *traceStickyWriter) Write(p []byte) (int, error) {
	w.syncFailure()
	if w.first != nil {
		return 0, w.first
	}
	w.enterOutput()
	defer w.leaveOutput()
	n, err := w.out.Write(p)
	return w.remember(n, len(p), err)
}
func (w *traceStickyWriter) WriteString(s string) (int, error) {
	w.syncFailure()
	if w.first != nil {
		return 0, w.first
	}
	w.enterOutput()
	defer w.leaveOutput()
	if out, ok := w.out.(interface{ AppendCString(string) error }); ok {
		err := out.AppendCString(s)
		if err != nil {
			w.capture(err)
			if w.streamErr == nil {
				w.streamErr = err
			}
			return 0, err
		}
		return len(s), nil
	}
	n, err := io.WriteString(w.out, s)
	return w.remember(n, len(s), err)
}

// AppendCString, AppendInt32 and AppendUint32 retain the receiving ostream's
// formatting state while keeping the same sticky failure and callback scope as
// ordinary trace writes. These small concrete insertion shapes are the ones
// used by retained source trace consumers.
func (w *traceStickyWriter) AppendCString(s string) error {
	_, err := w.WriteString(s)
	return err
}
func (w *traceStickyWriter) AppendInt32(v int32) error {
	w.syncFailure()
	if w.first != nil {
		return w.first
	}
	w.enterOutput()
	defer w.leaveOutput()
	if out, ok := w.out.(interface{ AppendInt32(int32) error }); ok {
		err := out.AppendInt32(v)
		w.capture(err)
		if err != nil && w.streamErr == nil {
			w.streamErr = err
		}
		return err
	}
	s := strconv.FormatInt(int64(v), 10)
	n, err := io.WriteString(w.out, s)
	_, err = w.remember(n, len(s), err)
	return err
}
func (w *traceStickyWriter) AppendUint32(v uint32) error {
	w.syncFailure()
	if w.first != nil {
		return w.first
	}
	w.enterOutput()
	defer w.leaveOutput()
	if out, ok := w.out.(interface{ AppendUint32(uint32) error }); ok {
		err := out.AppendUint32(v)
		w.capture(err)
		if err != nil && w.streamErr == nil {
			w.streamErr = err
		}
		return err
	}
	s := strconv.FormatUint(uint64(v), 10)
	n, err := io.WriteString(w.out, s)
	_, err = w.remember(n, len(s), err)
	return err
}
func (w *traceStickyWriter) Flush() error {
	w.syncFailure()
	if w.first != nil {
		return w.first
	}
	w.enterOutput()
	defer w.leaveOutput()
	err := flushTrace(w.out)
	w.capture(err)
	if err != nil && w.streamErr == nil {
		w.streamErr = err
	}
	return err
}

// WriteTraceEndl preserves a reached std::endl insertion: it writes a newline
// and flushes before the next source insertion in the same trace body.
func WriteTraceEndl(w io.Writer) error {
	if err := writeTracePart(w, "\n"); err != nil {
		return err
	}
	return flushTrace(w)
}

func (w *traceStickyWriter) finish() error {
	w.syncFailure()
	w.enterOutput()
	defer w.leaveOutput()
	if err := flushTrace(w.out); err != nil {
		w.capture(err)
		if w.streamErr == nil {
			w.streamErr = err
		}
	}
	return w.first
}

// A failed ostream insertion sets the stream state and source execution
// continues to the TRACE footer. An error from an operand computation unwinds
// the macro body before its footer or final flush.
func (w *traceStickyWriter) bodyComputationFailed(err error) bool {
	return err != nil && (w.streamErr == nil || !errors.Is(err, w.streamErr))
}

// TRACE and CTRACE insert each header component separately before CODE runs.
func writeTraceHeader(w io.Writer, tag, function, file string, line int) error {
	for _, part := range [...]string{"-------- [", traceTagCString(tag), "] ", function, " ", file, ":"} {
		if err := writeTracePart(w, part); err != nil {
			return err
		}
	}
	if err := writeTraceInt32(w, int32(line)); err != nil {
		return err
	}
	return writeTracePart(w, " ---------\n")
}

// Emit writes one TRACE record to the source-order stream. The caller
// supplies its static source site and already-gated body. An explicitly
// constructed unbound stream opens .z3-trace on its first
// emitted record. The process stream opens it during package initialization.
func (t *TraceTags) Emit(tag, function, file string, line int, body string) error {
	if !t.ShouldTrace(tag) {
		return nil
	}
	w, err := t.traceWriter()
	if err != nil {
		return err
	}
	sticky := &traceStickyWriter{owner: t, out: w, first: t.streamErr, streamErr: t.streamErr}
	sticky.capture(writeTraceHeader(sticky, tag, function, file, line))
	sticky.capture(writeTracePart(sticky, body))
	sticky.capture(writeTracePart(sticky, "------------------------------------------------\n"))
	err = sticky.finish()
	t.streamErr = sticky.streamErr
	return err
}

// EmitShort implements STRACE: no header or footer, but each reached body is
// flushed. The body is evaluated only after the compile and tag gates.
func (t *TraceTags) EmitShort(tag string, body func(io.Writer) error) error {
	if !t.ShouldTrace(tag) {
		return nil
	}
	w, err := t.traceWriter()
	if err != nil {
		return err
	}
	sticky := &traceStickyWriter{owner: t, out: w, first: t.streamErr, streamErr: t.streamErr}
	if body != nil {
		bodyErr := body(sticky)
		sticky.syncFailure()
		if sticky.bodyComputationFailed(bodyErr) {
			t.streamErr = sticky.streamErr
			return bodyErr
		}
		sticky.capture(bodyErr)
	}
	err = sticky.finish()
	t.streamErr = sticky.streamErr
	return err
}

// EmitConditional implements CTRACE's compile, tag, then condition order.
// Neither the condition nor the body is evaluated when an earlier gate fails.
// The header is written before the body, just as the source macro expands.
func (t *TraceTags) EmitConditional(tag, function, file string, line int, condition func() bool, body func(io.Writer) error) error {
	if !t.ShouldTrace(tag) || condition == nil || !condition() {
		return nil
	}
	w, err := t.traceWriter()
	if err != nil {
		return err
	}
	sticky := &traceStickyWriter{owner: t, out: w, first: t.streamErr, streamErr: t.streamErr}
	sticky.capture(writeTraceHeader(sticky, tag, function, file, line))
	if body != nil {
		bodyErr := body(sticky)
		sticky.syncFailure()
		if sticky.bodyComputationFailed(bodyErr) {
			t.streamErr = sticky.streamErr
			return bodyErr
		}
		sticky.capture(bodyErr)
	}
	sticky.capture(writeTracePart(sticky, "------------------------------------------------\n"))
	err = sticky.finish()
	t.streamErr = sticky.streamErr
	return err
}

// Finalize releases the tag table. Source finalize_trace leaves the all flag.
func (t *TraceTags) Finalize() {
	if t.tags == nil {
		return
	}
	t.tags = nil
	t.traceTagTableDeallocated()
}
