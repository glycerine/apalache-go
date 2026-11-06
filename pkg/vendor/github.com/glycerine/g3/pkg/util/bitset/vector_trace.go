package bitset

// Source: src/util/bit_vector.cpp:43-68, the two growth TRACE sites.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// BindTrace borrows an isolated trace stream. Without one, resize uses the
// source process-wide stream. A nil binding restores that process stream.
func (v *Vector) BindTrace(tags *diagnostic.TraceTags) { v.trace = tags }

func (v *Vector) resizeTraceTags() *diagnostic.TraceTags {
	if v.trace != nil {
		return v.trace
	}
	return diagnostic.BorrowProcessTraceTags()
}

func (v *Vector) traceResizeStart(n, words uint32) {
	tags := v.resizeTraceTags()
	_ = tags.EmitConditional("bit_vector", "resize", "src/util/bit_vector.cpp", 43, func() bool { return true }, func(w io.Writer) error {
		return writeResizeTraceParts(w, []string{
			"expanding: ", strconv.FormatUint(uint64(n), 10),
			" capacity: ", strconv.Itoa(len(v.words)),
			" num words: ", strconv.FormatUint(uint64(words), 10), "\n",
		})
	})
}

func (v *Vector) traceResizeFields(begin, end, pos, mask uint32, val bool) {
	tags := v.resizeTraceTags()
	cval := 0
	if val {
		cval = -1
	}
	_ = tags.EmitConditional("bit_vector", "resize", "src/util/bit_vector.cpp", 67, func() bool { return true }, func(w io.Writer) error {
		return writeResizeTraceParts(w, []string{
			"num_bits: ", strconv.FormatUint(uint64(v.size), 10), "\n",
			"bwidx:    ", strconv.FormatUint(uint64(begin), 10), "\n",
			"ewidx:    ", strconv.FormatUint(uint64(end), 10), "\n",
			"pos:      ", strconv.FormatUint(uint64(pos), 10), "\n",
			"mask:     ", strconv.FormatUint(uint64(mask), 16), "\n",
			"cval:     ", strconv.Itoa(cval), "\n",
		})
	})
}

func writeResizeTraceParts(w io.Writer, parts []string) error {
	for _, part := range parts {
		n, err := io.WriteString(w, part)
		if err != nil {
			return err
		}
		if n != len(part) {
			return io.ErrShortWrite
		}
	}
	return nil
}
