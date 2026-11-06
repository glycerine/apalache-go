package statistics

// Source: src/util/statistics.h; src/util/statistics.cpp.

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/insdict"
)

// Key is the spelling used to identify a statistic.
type Key string

type uintContribution struct {
	key   Key
	value uint32
}

type doubleContribution struct {
	key   Key
	value float64
}

// Statistics retains contributions in source order. Repeated keys remain
// separate until rendering, as in the source statistics class.
type Statistics struct {
	uints   []uintContribution
	doubles []doubleContribution
}

func New() *Statistics { return &Statistics{} }

func (s *Statistics) UpdateUint(key Key, value uint32) {
	if value != 0 {
		s.uints = append(s.uints, uintContribution{key, value})
	}
}

func (s *Statistics) UpdateDouble(key Key, value float64) {
	if value != 0 {
		s.doubles = append(s.doubles, doubleContribution{key, value})
	}
}

// UpdateUint64 follows get_uint64_stats: values wider than uint32 are exposed
// as doubles because the source statistics container has no uint64 domain.
func (s *Statistics) UpdateUint64(key Key, value uint64) {
	if value <= uint64(^uint32(0)) {
		s.UpdateUint(key, uint32(value))
	} else {
		s.UpdateDouble(key, float64(value))
	}
}

// Copy appends src's contributions. It takes a value to make self-copy finite.
func (s *Statistics) Copy(src Statistics) {
	s.uints = append(s.uints, src.uints...)
	s.doubles = append(s.doubles, src.doubles...)
}

func (s *Statistics) Reset() {
	s.uints = s.uints[:0]
	s.doubles = s.doubles[:0]
}

func (s *Statistics) Size() int { return len(s.uints) + len(s.doubles) }

func (s *Statistics) IsUint(index int) bool { return index >= 0 && index < len(s.uints) }

func (s *Statistics) Key(index int) Key {
	if index < len(s.uints) {
		return s.uints[index].key
	}
	return s.doubles[index-len(s.uints)].key
}

func (s *Statistics) UintValue(index int) uint32 { return s.uints[index].value }

func (s *Statistics) DoubleValue(index int) float64 {
	return s.doubles[index-len(s.uints)].value
}

type uintValue struct {
	key   Key
	value uint32
}

type doubleValue struct {
	key   Key
	value float64
}

func (s *Statistics) aggregate() (*insdict.Dict[string, uintValue], *insdict.Dict[string, doubleValue]) {
	uints := insdict.NewDictSize[string, uintValue](len(s.uints))
	doubles := insdict.NewDictSize[string, doubleValue](len(s.doubles))
	for _, contribution := range s.uints {
		name := string(contribution.key)
		value := contribution.value
		if old, found := uints.Get2(name); found {
			value += old.value
		}
		uints.Put(name, uintValue{contribution.key, value})
	}
	for _, contribution := range s.doubles {
		name := string(contribution.key)
		value := contribution.value
		if old, found := doubles.Get2(name); found {
			value += old.value
		}
		doubles.Put(name, doubleValue{contribution.key, value})
	}
	return uints, doubles
}

func visibleKey(key string) string {
	return strings.TrimPrefix(key, ":")
}

func smt2Key(key string) string {
	key = visibleKey(key)
	var out strings.Builder
	out.Grow(len(key) + 1)
	out.WriteByte(':')
	for i := 0; i < len(key); i++ {
		if isSMT2SimpleSymbolChar(key[i]) {
			out.WriteByte(key[i])
		} else {
			out.WriteByte('-')
		}
	}
	return out.String()
}

func isSMT2SimpleSymbolChar(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' ||
		strings.ContainsRune("~!@$%^&*_-+=<>.?/", rune(c))
}

func writeValue(out io.Writer, uints *insdict.Dict[string, uintValue], name string, doubles *insdict.Dict[string, doubleValue]) error {
	if value, found := uints.Get2(name); found {
		_, err := io.WriteString(out, strconv.FormatUint(uint64(value.value), 10))
		return err
	}
	value, _ := doubles.Get2(name)
	_, err := io.WriteString(out, strconv.FormatFloat(value.value, 'f', 2, 64))
	return err
}

func (s *Statistics) sortedKeys(uints *insdict.Dict[string, uintValue], doubles *insdict.Dict[string, doubleValue]) []string {
	keys := make([]string, 0, uints.Len()+doubles.Len())
	for _, value := range uints.All() {
		keys = append(keys, string(value.key))
	}
	for _, value := range doubles.All() {
		keys = append(keys, string(value.key))
	}
	sort.Strings(keys)
	return keys
}

func maxVisibleKeyLength(keys []string) int {
	max := 0
	for _, key := range keys {
		if n := len(visibleKey(key)); n > max {
			max = n
		}
	}
	return max
}

func padding(width, length int) string {
	if length >= width {
		return ""
	}
	return strings.Repeat(" ", width-length)
}

func (s *Statistics) Display(out io.Writer) error {
	uints, doubles := s.aggregate()
	keys := s.sortedKeys(uints, doubles)
	width := maxVisibleKeyLength(keys)
	for _, key := range keys {
		visible := visibleKey(key)
		if _, err := fmt.Fprintf(out, "%s:%s ", visible, padding(width, len(visible))); err != nil {
			return err
		}
		if err := writeValue(out, uints, key, doubles); err != nil {
			return err
		}
		if _, err := io.WriteString(out, "\n"); err != nil {
			return err
		}
	}
	return nil
}

func (s *Statistics) DisplaySMT2(out io.Writer) error {
	uints, doubles := s.aggregate()
	keys := s.sortedKeys(uints, doubles)
	width := maxVisibleKeyLength(keys)
	if _, err := io.WriteString(out, "("); err != nil {
		return err
	}
	for i, key := range keys {
		if i != 0 {
			if _, err := io.WriteString(out, "\n "); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(out, "%s%s ", smt2Key(key), padding(width, len(key))); err != nil {
			return err
		}
		if err := writeValue(out, uints, key, doubles); err != nil {
			return err
		}
	}
	_, err := io.WriteString(out, ")\n")
	return err
}

func internalKey(key string) string {
	key = visibleKey(key)
	var out strings.Builder
	out.Grow(len(key))
	for i := 0; i < len(key); i++ {
		c := key[i]
		switch {
		case c >= 'a' && c <= 'z':
			out.WriteByte('A' + c - 'a')
		case c == ' ':
			out.WriteByte('_')
		default:
			out.WriteByte(c)
		}
	}
	return out.String()
}

func (s *Statistics) DisplayInternal(out io.Writer) error {
	uints, doubles := s.aggregate()
	for _, value := range uints.All() {
		if _, err := fmt.Fprintf(out, "%s %d\n", internalKey(string(value.key)), value.value); err != nil {
			return err
		}
	}
	for _, value := range doubles.All() {
		if _, err := fmt.Fprintf(out, "%s %v\n", internalKey(string(value.key)), value.value); err != nil {
			return err
		}
	}
	return nil
}

func memoryMegabytes(value uint64) float64 {
	// The source multiplies unsigned long long before dividing; that product
	// wraps in its 64-bit domain for very large byte counts.
	centi := (100 * value) / (1024 * 1024)
	return float64(centi) / 100
}

// AddResourceLimit records the count from a resource-limit owner without
// coupling this leaf package to that owner's package.
func AddResourceLimit(s *Statistics, count uint64) {
	s.UpdateUint64("rlimit count", count)
}

func addMemory(s *Statistics, current, peak, count uint64) {
	s.UpdateDouble("max memory", memoryMegabytes(peak))
	s.UpdateDouble("memory", memoryMegabytes(current))
	s.UpdateUint64("num allocs", count)
}

// AddMemory follows get_memory_statistics using the source process accounting
// values. Callers must supply the same process owner as their raw allocators.
func AddMemory(s *Statistics, process *memory.ProcessAccounting) error {
	if s == nil || !process.Valid() {
		return arena.ErrConfig
	}
	addMemory(s, process.AllocationSize(), process.MaximumUsed(), process.AllocationCount())
	return nil
}
