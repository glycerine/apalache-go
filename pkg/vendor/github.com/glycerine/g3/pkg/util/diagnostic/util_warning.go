package diagnostic

// Source: src/util/warning.h:23-53; src/util/warning.cpp:49-85,106-168.

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"

	"github.com/glycerine/g3/pkg/arena"
)

// WarningService is the concrete single-goroutine owner of warning.cpp's
// process-wide scalar settings. Streams are borrowed from the caller.
type WarningService struct {
	stderr  io.Writer
	stream  io.Writer
	enabled bool
	// A selected ostream retains its failed state across warning_msg calls.
	streamFailed bool
	// Rebinding a borrowed ostream does not clear its own failed state.
	failedStreams map[io.Writer]struct{}
}

// The source warning settings are process globals. Retained callers share this
// one service; tests and explicit owners may still construct isolated services.
var processWarnings = NewWarningService(os.Stderr)

func BorrowProcessWarningService() *WarningService { return processWarnings }

func NewWarningService(stderr io.Writer) *WarningService {
	if warningWriterNil(stderr) {
		stderr = os.Stderr
	}
	return &WarningService{stderr: stderr, enabled: true}
}

func warningWriterNil(w io.Writer) bool {
	if w == nil {
		return true
	}
	v := reflect.ValueOf(w)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	}
	return false
}

func (s *WarningService) Enable(flag bool) error {
	if s == nil {
		return arena.ErrConfig
	}
	s.enabled = flag
	return nil
}
func (s *WarningService) Enabled() bool { return s != nil && s.enabled }

func (s *WarningService) SetWarningStream(stream io.Writer) error {
	if s == nil {
		return arena.ErrConfig
	}
	if warningWriterNil(stream) {
		stream = nil
	}
	if stream != nil && s.stream != nil {
		t := reflect.TypeOf(stream)
		if t == reflect.TypeOf(s.stream) && t.Comparable() && stream == s.stream {
			return nil
		}
	}
	s.stream = stream
	s.streamFailed = false
	if stream != nil && reflect.TypeOf(stream).Comparable() {
		_, s.streamFailed = s.failedStreams[stream]
	}
	return nil
}

// Warningf covers retained warning_msg calls using Go's %s and %d formatting,
// including the datalog parser's 64-bit unsigned-decimal %I64u spelling.
// String arguments are C-string views at those source sites.
func (s *WarningService) Warningf(format string, args ...any) error {
	if s == nil {
		return arena.ErrConfig
	}
	if !s.enabled {
		return nil
	}
	if i := strings.IndexByte(format, 0); i >= 0 {
		format = format[:i]
	}
	if strings.Contains(format, "%I64u") {
		var converted strings.Builder
		converted.Grow(len(format))
		for i := 0; i < len(format); i++ {
			if format[i] == '%' && i+1 < len(format) {
				if format[i+1] == '%' {
					converted.WriteString("%%")
					i++
					continue
				}
				if strings.HasPrefix(format[i+1:], "I64u") {
					converted.WriteString("%d")
					i += 4
					continue
				}
			}
			converted.WriteByte(format[i])
		}
		format = converted.String()
	}
	copied := false
	for i, arg := range args {
		if value, ok := arg.(string); ok {
			if end := strings.IndexByte(value, 0); end >= 0 {
				if !copied {
					args = append([]any(nil), args...)
					copied = true
				}
				args[i] = value[:end]
			}
		}
	}
	return s.Warning(fmt.Sprintf(format, args...))
}

// Warning writes one literal C-string message. On the selected Linux AMD64
// source profile, the optional ostream route truncates formatted content to
// 1023 bytes; the default FILE route does not truncate.
func (s *WarningService) Warning(message string) error {
	if s == nil {
		return arena.ErrConfig
	}
	if !s.enabled {
		return nil
	}
	if i := strings.IndexByte(message, 0); i >= 0 {
		message = message[:i]
	}
	w := s.stream
	if w != nil {
		if len(message) > 1023 {
			message = message[:1023]
		}
	} else {
		w = s.stderr
	}
	// The source performs all three insertions and flushes even after the
	// stream enters a failed state. A Go writer has no sticky state, so retain
	// the first insertion failure to skip later underlying writes until the
	// selected stream changes.
	var failure error
	if s.streamFailed {
		failure = io.ErrClosedPipe
	}
	for _, part := range [...]string{"WARNING: ", message, "\n"} {
		if failure == nil {
			failure = writeWarningPart(w, part)
		}
	}
	if s.stream != nil && failure != nil {
		s.streamFailed = true
	}
	// std::ostream::flush and fflush publish the completed warning. A borrowed
	// buffered Go writer supplies the corresponding Flush method.
	if f, ok := w.(interface{ Flush() error }); ok {
		if e := f.Flush(); s.stream != nil && e != nil {
			s.streamFailed = true
		}
	}
	if s.streamFailed && s.stream != nil && reflect.TypeOf(s.stream).Comparable() {
		if s.failedStreams == nil {
			s.failedStreams = make(map[io.Writer]struct{})
		}
		s.failedStreams[s.stream] = struct{}{}
	}
	// warning_msg returns void: an ostream/FILE failure never unwinds its
	// caller. The failed state belongs to the borrowed writer, not arithmetic
	// or parser control flow.
	return nil
}

func writeWarningPart(w io.Writer, part string) error {
	n, err := io.WriteString(w, part)
	if err != nil {
		return err
	}
	if n != len(part) {
		return io.ErrShortWrite
	}
	return nil
}
