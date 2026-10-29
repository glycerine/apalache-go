package text

// Source: src/util/memory_manager.cpp:55-72; src/util/small_object_allocator.cpp:172-242.
// Numeric stream platform adapter for repository C++ ostream consumers.
// C-locale/libstdc++ AMD64 dynamic-int precision and nearest/even IEEE64 profile.
// Go strconv is the explicit external runtime dependency, not a solver algorithm.

import (
	"bytes"
	"errors"
	"math"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/glycerine/g3/pkg/arena"
)

var ErrNumericStreamOwner = errors.New("g3: invalid numeric stream owner")
var ErrNumericStreamConfig = errors.New("g3: unsupported numeric stream profile/state")

type NumericFloatMode uint32

const (
	NumericDefaultFloat NumericFloatMode = iota
	NumericFixedFloat
	NumericScientificFloat
)

type NumericAlignment uint32

const (
	NumericRight NumericAlignment = iota
	NumericLeft
	NumericInternal
)

type NumericStreamProfile uint32

const NumericLibStdCPPAMD64 NumericStreamProfile = 0

// Profile0 explicitly denotes the selected narrow C-locale, nearest/even,
// libstdc++ AMD64 precision ABI. It does not select arbitrary host CRT behavior.
type NumericTextStreamConfig struct {
	Budget       *arena.Budget
	Profile      NumericStreamProfile
	BorrowedFile *os.File
}
type NumericStreamState struct {
	Precision, Width                        int64
	FloatMode                               NumericFloatMode
	Alignment                               NumericAlignment
	IntegerBase                             uint32
	Fill                                    byte
	Uppercase, ShowPoint, ShowPos, ShowBase bool
}
type NumericStreamStats struct {
	OwnerBytes, ScratchBytes, Position, OutputCapacity, OutputOwnerBytes, OutputOverflowBytes uint64
	State                                                                                     NumericStreamState
}

// NumericTextStream owns private output and scratch. Never shallow-copy it.
// Budget is borrowed and remains externally confined; the mutex does not make
// a Budget shared with unrelated owners globally thread-safe.
type NumericTextStream struct {
	mu           sync.Mutex
	self         *NumericTextStream
	Budget       *arena.Budget
	Buffer       *bytes.Buffer
	File         *os.File
	fileByte     [1]byte
	FilePosition uint64
	FileBad      bool
	fileFail     bool // failbit is distinct from badbit (actual fstream open failure)
	fileMode     bool // actual file provider even when a failed open has no fd
	scratch      []byte
	OwnerBytes   uint64
	state        NumericStreamState
	Closed       bool
}

func NewNumericTextStream(c NumericTextStreamConfig) (*NumericTextStream, error) {
	if c.Profile != NumericLibStdCPPAMD64 {
		return nil, ErrNumericStreamConfig
	}
	header := uint64(reflect.TypeOf(NumericTextStream{}).Size())
	if e := c.Budget.Reserve(header); e != nil {
		return nil, e
	}
	var b *bytes.Buffer
	if c.BorrowedFile == nil {
		b = new(bytes.Buffer)
	}
	s := &NumericTextStream{Budget: c.Budget, Buffer: b, File: c.BorrowedFile, OwnerBytes: header, state: NumericStreamState{Precision: 6, IntegerBase: 10, Fill: ' '}}
	s.self = s
	return s, nil
}
func (s *NumericTextStream) lock() error {
	if s == nil || s.self != s {
		return ErrNumericStreamOwner
	}
	s.mu.Lock()
	if s.Closed {
		s.mu.Unlock()
		return arena.ErrClosed
	}
	return nil
}
func (s *NumericTextStream) State() NumericStreamState {
	if s.lock() != nil {
		return NumericStreamState{}
	}
	defer s.mu.Unlock()
	return s.state
}
func (s *NumericTextStream) Stats() NumericStreamStats {
	if s.lock() != nil {
		return NumericStreamStats{}
	}
	defer s.mu.Unlock()
	if s.fileMode || s.File != nil {
		return NumericStreamStats{OwnerBytes: s.OwnerBytes, ScratchBytes: uint64(len(s.scratch)), Position: s.FilePosition, State: s.state}
	}
	return NumericStreamStats{OwnerBytes: s.OwnerBytes, ScratchBytes: uint64(len(s.scratch)), Position: uint64(s.Buffer.Len()), OutputCapacity: uint64(s.Buffer.Cap()), State: s.state}
}
func (s *NumericTextStream) SetFloatMode(v NumericFloatMode) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if v > NumericScientificFloat {
		return ErrNumericStreamConfig
	}
	s.state.FloatMode = v
	return nil
}
func (s *NumericTextStream) SetAlignment(v NumericAlignment) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if v > NumericInternal {
		return ErrNumericStreamConfig
	}
	s.state.Alignment = v
	return nil
}
func (s *NumericTextStream) SetIntegerBase(v uint32) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if v != 8 && v != 10 && v != 16 {
		return ErrNumericStreamConfig
	}
	s.state.IntegerBase = v
	return nil
}
func (s *NumericTextStream) SetPrecision(v int64) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	s.state.Precision = v
	return nil
}
func (s *NumericTextStream) SetWidth(v int64) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	s.state.Width = v
	return nil
}
func (s *NumericTextStream) SetFill(v byte) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	s.state.Fill = v
	return nil
}
func (s *NumericTextStream) setFlag(which uint32, v bool) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	switch which {
	case 0:
		s.state.Uppercase = v
	case 1:
		s.state.ShowPoint = v
	case 2:
		s.state.ShowPos = v
	case 3:
		s.state.ShowBase = v
	}
	return nil
}
func (s *NumericTextStream) SetUppercase(v bool) error { return s.setFlag(0, v) }
func (s *NumericTextStream) SetShowPoint(v bool) error { return s.setFlag(1, v) }
func (s *NumericTextStream) SetShowPos(v bool) error   { return s.setFlag(2, v) }
func (s *NumericTextStream) SetShowBase(v bool) error  { return s.setFlag(3, v) }
func (s *NumericTextStream) ensureScratch(n uint64) error {
	if n > uint64(^uint(0)>>1) {
		return arena.ErrBounds
	}
	if n <= uint64(len(s.scratch)) {
		return nil
	}
	if e := s.Budget.Reserve(n); e != nil {
		return e
	}
	next := make([]byte, int(n))
	s.Budget.Release(uint64(len(s.scratch)))
	s.scratch = next
	return nil
}
func (s *NumericTextStream) fieldPadding(n uint64) (uint64, error) {
	size := n
	if s.state.Width > 0 && uint64(s.state.Width) > size {
		size = uint64(s.state.Width)
	}
	s.state.Width = 0
	return size - n, nil
}
func (s *NumericTextStream) emitByte(c byte) error {
	if s.fileMode || s.File != nil {
		if s.FileBad || s.fileFail {
			return nil
		}
		if s.File == nil {
			return ErrNumericStreamFile
		}
		s.fileByte[0] = c
		n, e := s.File.Write(s.fileByte[:])
		s.FilePosition += uint64(n)
		if e != nil || n != 1 {
			s.FileBad = true
		}
		return nil
	}
	return s.Buffer.WriteByte(c)
}
func (s *NumericTextStream) padding(n uint64) error {
	for i := uint64(0); i < n; i++ {
		if e := s.emitByte(s.state.Fill); e != nil {
			return e
		}
	}
	return nil
}
func (s *NumericTextStream) emitBytes(v []byte, numeric bool) error {
	if s.FileBad || s.fileFail {
		return nil
	}
	pad, e := s.fieldPadding(uint64(len(v)))
	if e != nil {
		return e
	}
	prefix := 0
	if numeric && s.state.Alignment == NumericInternal && len(v) > 0 {
		if v[0] == '+' || v[0] == '-' {
			prefix = 1
		} else if len(v) > 1 && v[0] == '0' && (v[1] == 'x' || v[1] == 'X') {
			prefix = 2
		}
	}
	if s.state.Alignment == NumericRight || (s.state.Alignment == NumericInternal && prefix == 0) {
		if e = s.padding(pad); e != nil {
			return e
		}
	}
	for i, c := range v {
		if prefix > 0 && i == prefix {
			if e = s.padding(pad); e != nil {
				return e
			}
		}
		if e = s.emitByte(c); e != nil {
			return e
		}
	}
	if s.state.Alignment == NumericLeft {
		return s.padding(pad)
	}
	return nil
}
func (s *NumericTextStream) AppendCString(v string) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	return s.appendCString(v)
}

// AppendString inserts a counted native Go string, including embedded NUL bytes.
func (s *NumericTextStream) AppendString(v string) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	return s.appendString(v)
}

// AppendNullableCString accepts a native optional string. A missing input
// marks the selected file transport bad without emitting or consuming width.
func (s *NumericTextStream) AppendNullableCString(v *string) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if v == nil {
		s.FileBad = true
		return nil
	}
	return s.appendCString(*v)
}
func (s *NumericTextStream) appendCString(v string) error {
	n := len(v)
	if i := strings.IndexByte(v, 0); i >= 0 {
		n = i
	}
	return s.appendString(v[:n])
}
func (s *NumericTextStream) appendString(v string) error {
	n := len(v)
	if s.FileBad || s.fileFail {
		return nil
	}
	pad, e := s.fieldPadding(uint64(n))
	if e != nil {
		return e
	}
	if s.state.Alignment != NumericLeft {
		if e = s.padding(pad); e != nil {
			return e
		}
	}
	for i := 0; i < n; i++ {
		if e = s.emitByte(v[i]); e != nil {
			return e
		}
	}
	if s.state.Alignment == NumericLeft {
		return s.padding(pad)
	}
	return nil
}
func (s *NumericTextStream) AppendByte(v byte) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	one := [1]byte{v}
	return s.emitBytes(one[:], false)
}
func (s *NumericTextStream) integer(v uint64, negative, signed bool) error {
	var raw [67]byte
	at := len(raw)
	base := uint64(s.state.IntegerBase)
	original := v
	for {
		at--
		d := byte(v % base)
		if d < 10 {
			raw[at] = '0' + d
		} else if s.state.Uppercase {
			raw[at] = 'A' + d - 10
		} else {
			raw[at] = 'a' + d - 10
		}
		v /= base
		if v == 0 {
			break
		}
	}
	if original != 0 && s.state.ShowBase {
		if base == 16 {
			at--
			raw[at] = 'x'
			if s.state.Uppercase {
				raw[at] = 'X'
			}
			at--
			raw[at] = '0'
		} else if base == 8 {
			at--
			raw[at] = '0'
		}
	}
	if base == 10 && signed {
		if negative {
			at--
			raw[at] = '-'
		} else if s.state.ShowPos {
			at--
			raw[at] = '+'
		}
	}
	return s.emitBytes(raw[at:], true)
}
func (s *NumericTextStream) AppendUint32(v uint32) error { return s.AppendUint64(uint64(v)) }
func (s *NumericTextStream) AppendUint64(v uint64) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	return s.integer(v, false, false)
}
func (s *NumericTextStream) AppendInt32(v int32) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	n := uint64(uint32(v))
	if s.state.IntegerBase == 10 {
		n = uint64(int64(v))
		if v < 0 {
			n = ^n + 1
		}
	}
	return s.integer(n, v < 0, true)
}
func (s *NumericTextStream) AppendInt64(v int64) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	n := uint64(v)
	if s.state.IntegerBase == 10 && v < 0 {
		n = ^n + 1
	}
	return s.integer(n, v < 0, true)
}
func (s *NumericTextStream) effectivePrecision() int {
	if s.state.Precision < 0 {
		return 6
	}
	p := int32(uint32(uint64(s.state.Precision)))
	if p < 0 {
		return 6
	}
	return int(p)
}
func (s *NumericTextStream) AppendFloat64(v float64) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if s.FileBad || s.fileFail {
		return nil
	}
	bits := math.Float64bits(v)
	negative := bits>>63 != 0
	if bits&0x7ff0000000000000 == 0x7ff0000000000000 {
		var token [4]byte
		at := 0
		if negative {
			token[at] = '-'
			at++
		} else if s.state.ShowPos {
			token[at] = '+'
			at++
		}
		word := "inf"
		if bits&0xfffffffffffff != 0 {
			word = "nan"
		}
		for i := 0; i < 3; i++ {
			c := word[i]
			if s.state.Uppercase {
				c -= 32
			}
			token[at] = c
			at++
		}
		return s.emitBytes(token[:at], true)
	}
	p := s.effectivePrecision()
	mode := byte('g')
	bound := uint64(p) + 7
	if s.state.FloatMode == NumericFixedFloat {
		mode = 'f'
		bound = uint64(p) + 311
	} else if s.state.FloatMode == NumericScientificFloat {
		mode = 'e'
		bound = uint64(p) + 8
	} else if p == 0 {
		p = 1
		bound = 8
	}
	if e := s.ensureScratch(bound); e != nil {
		return e
	}
	if s.state.Uppercase && mode != 'f' {
		mode -= 32
	}
	raw := strconv.AppendFloat(s.scratch[:0], v, mode, p, 64)
	// Bounds preflight guarantees AppendFloat never grows/replaces this backing.
	if len(raw) > len(s.scratch) || len(raw) > 0 && &raw[0] != &s.scratch[0] {
		panic("numeric stream preflight bound violated")
	}
	if !negative && s.state.ShowPos {
		copy(s.scratch[1:], raw)
		s.scratch[0] = '+'
		raw = s.scratch[:len(raw)+1]
	}
	if s.state.ShowPoint {
		end := len(raw)
		start := 0
		if raw[0] == '-' || raw[0] == '+' {
			start = 1
		}
		for i := start; i < len(raw); i++ {
			if raw[i] == 'e' || raw[i] == 'E' {
				end = i
				break
			}
		}
		dot := -1
		significant := 0
		seen := false
		for i := start; i < end; i++ {
			if raw[i] == '.' {
				dot = i
				continue
			}
			if raw[i] != '0' {
				seen = true
			}
			if seen {
				significant++
			}
		}
		if !seen {
			significant = 1
		}
		extra := 0
		if dot < 0 {
			extra++
		}
		if s.state.FloatMode == NumericDefaultFloat && p > significant {
			extra += p - significant
		}
		if extra > 0 {
			old := len(raw)
			copy(s.scratch[end+extra:], raw[end:])
			at := end
			if dot < 0 {
				s.scratch[at] = '.'
				at++
			}
			for at < end+extra {
				s.scratch[at] = '0'
				at++
			}
			raw = s.scratch[:old+extra]
		}
	}
	return s.emitBytes(raw, true)
}
func (s *NumericTextStream) String() (string, error) {
	if e := s.lock(); e != nil {
		return "", e
	}
	defer s.mu.Unlock()
	if s.fileMode || s.File != nil {
		return "", ErrNumericStreamFile
	}
	return s.Buffer.String(), nil
}
func (s *NumericTextStream) ResetText() error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if s.fileMode || s.File != nil {
		return ErrNumericStreamFile
	}
	s.Buffer.Reset()
	return nil
}
func (s *NumericTextStream) Close() error {
	if s == nil || s.self != s {
		return ErrNumericStreamOwner
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Closed {
		return nil
	}
	s.Buffer = nil
	s.Budget.Release(uint64(len(s.scratch)) + s.OwnerBytes)
	s.scratch = nil
	s.Closed = true
	return nil
}
