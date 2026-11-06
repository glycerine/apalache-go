package backtrack

// Source: src/util/union_find.h:25-241.

import (
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

var ErrUnionFindContext = errors.New("g3: union-find context has no trail stack")

// UnionFindContext supplies the source merge notifications and shared trail.
type UnionFindContext interface {
	GetTrailStack() *TrailStack
	UnmergeEH(root, child uint32)
	MergeEH(root, child, value2, value1 uint32)
	AfterMergeEH(root, child, value2, value1 uint32)
}

// UnionFindDefaultContext provides the source no-op notification policy.
type UnionFindDefaultContext struct {
	trail *TrailStack
}

func NewUnionFindDefaultContext() *UnionFindDefaultContext {
	return &UnionFindDefaultContext{trail: NewTrailStack()}
}

func (c *UnionFindDefaultContext) GetTrailStack() *TrailStack {
	if c != nil && c.trail == nil {
		c.trail = NewTrailStack()
	}
	if c == nil {
		return nil
	}
	return c.trail
}

func (*UnionFindDefaultContext) UnmergeEH(uint32, uint32)                    {}
func (*UnionFindDefaultContext) MergeEH(uint32, uint32, uint32, uint32)      {}
func (*UnionFindDefaultContext) AfterMergeEH(uint32, uint32, uint32, uint32) {}

// UnionFind is the backtrackable source union-find. It deliberately omits path
// compression so merge trails can restore the exact previous forest.
type UnionFind struct {
	context UnionFindContext
	trail   *TrailStack
	find    []uint32
	size    []uint32
	next    []uint32
	trace   *diagnostic.TraceTags
}

func NewUnionFind(context UnionFindContext) (*UnionFind, error) {
	if context == nil {
		return nil, ErrUnionFindContext
	}
	trail := context.GetTrailStack()
	if trail == nil {
		return nil, ErrUnionFindContext
	}
	return &UnionFind{context: context, trail: trail}, nil
}

func NewDefaultUnionFind() *UnionFind {
	context := NewUnionFindDefaultContext()
	result, _ := NewUnionFind(context)
	return result
}

// BindTrace borrows the source process trace state for this concrete owner.
// Nil restores the process-global trace tags.
func (u *UnionFind) BindTrace(tags *diagnostic.TraceTags) {
	u.trace = tags
}

func (u *UnionFind) traceRoots(function string, line int, verb string, root1, root2 uint32) {
	tags := u.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	// Source std::ofstream has no throwing exceptions by default. An output
	// failure leaves the union-find transition uninterrupted.
	_ = tags.EmitConditional("union_find", function, "src/util/union_find.h", line,
		func() bool { return true }, func(w io.Writer) error {
			for _, part := range [...]string{verb + " ", strconv.FormatUint(uint64(root1), 10), " ", strconv.FormatUint(uint64(root2), 10), "\n"} {
				n, err := io.WriteString(w, part)
				if err != nil {
					return err
				}
				if n != len(part) {
					return io.ErrShortWrite
				}
			}
			return nil
		})
}

func (u *UnionFind) MkVar() uint32 {
	value := uint32(len(u.find))
	u.find = append(u.find, value)
	u.size = append(u.size, 1)
	u.next = append(u.next, value)
	_ = u.trail.PushUndo(func() {
		u.find = u.find[:len(u.find)-1]
		u.size = u.size[:len(u.size)-1]
		u.next = u.next[:len(u.next)-1]
	})
	return value
}

func (u *UnionFind) GetNumVars() uint32 { return uint32(len(u.find)) }

func (u *UnionFind) Find(value uint32) uint32 {
	for {
		parent := u.find[value]
		if parent == value {
			return value
		}
		value = parent
	}
}

func (u *UnionFind) Next(value uint32) uint32 { return u.next[value] }

func (u *UnionFind) Size(value uint32) uint32 { return u.size[u.Find(value)] }

func (u *UnionFind) IsRoot(value uint32) bool { return u.find[value] == value }

func (u *UnionFind) Merge(value1, value2 uint32) {
	root1 := u.Find(value1)
	root2 := u.Find(value2)
	u.traceRoots("merge", 122, "merging", root1, root2)
	if root1 == root2 {
		return
	}
	if u.size[root1] > u.size[root2] {
		root1, root2 = root2, root1
	}
	u.context.MergeEH(root2, root1, value2, value1)
	u.find[root1] = root2
	u.size[root2] += u.size[root1]
	u.next[root1], u.next[root2] = u.next[root2], u.next[root1]
	_ = u.trail.PushUndo(func() { u.unmerge(root1) })
	u.context.AfterMergeEH(root2, root1, value2, value1)
}

func (u *UnionFind) unmerge(root1 uint32) {
	root2 := u.find[root1]
	u.traceRoots("unmerge", 79, "unmerging", root1, root2)
	u.size[root2] -= u.size[root1]
	u.find[root1] = root1
	u.next[root1], u.next[root2] = u.next[root2], u.next[root1]
	u.context.UnmergeEH(root2, root1)
}

// Dissolve separates every member of value's equivalence class. As in the
// source, callers must not use this operation with pending backtracking trails.
func (u *UnionFind) Dissolve(value uint32) {
	start := value
	for {
		next := u.Next(value)
		u.size[value] = 1
		u.find[value] = value
		u.next[value] = value
		value = next
		if value == start {
			return
		}
	}
}

func (u *UnionFind) Display(out io.Writer) error {
	for value := uint32(0); value < u.GetNumVars(); value++ {
		if _, err := fmt.Fprintf(out, "v%d --> v%d (%d)\n", value, u.find[value], u.Size(value)); err != nil {
			return err
		}
	}
	return nil
}

func (u *UnionFind) CheckInvariant() bool {
	for value := uint32(0); value < u.GetNumVars(); value++ {
		if !u.IsRoot(value) {
			continue
		}
		current := value
		var count uint32
		for {
			if u.Find(current) != value {
				return false
			}
			count++
			current = u.Next(current)
			if current == value {
				break
			}
			if count > u.GetNumVars() {
				return false
			}
		}
		if u.size[value] != count {
			return false
		}
	}
	return true
}

// BasicUnionFind is the non-backtrackable source variant. Unknown values are
// singleton roots until a merge grows the arrays to include them.
type BasicUnionFind struct {
	find []uint32
	size []uint32
	next []uint32
}

func NewBasicUnionFind() *BasicUnionFind { return &BasicUnionFind{} }

func (u *BasicUnionFind) MkVar() uint32 {
	value := uint32(len(u.find))
	u.find = append(u.find, value)
	u.size = append(u.size, 1)
	u.next = append(u.next, value)
	return value
}

func (u *BasicUnionFind) GetNumVars() uint32 { return uint32(len(u.find)) }

func (u *BasicUnionFind) ensureSize(value uint32) {
	for value >= u.GetNumVars() {
		u.MkVar()
	}
}

func (u *BasicUnionFind) Find(value uint32) uint32 {
	if value >= u.GetNumVars() {
		return value
	}
	for {
		parent := u.find[value]
		if parent == value {
			return value
		}
		value = parent
	}
}

func (u *BasicUnionFind) Next(value uint32) uint32 {
	if value >= u.GetNumVars() {
		return value
	}
	return u.next[value]
}

func (u *BasicUnionFind) IsRoot(value uint32) bool {
	return value >= u.GetNumVars() || u.find[value] == value
}

func (u *BasicUnionFind) Merge(value1, value2 uint32) {
	root1 := u.Find(value1)
	root2 := u.Find(value2)
	if root1 == root2 {
		return
	}
	u.ensureSize(value1)
	u.ensureSize(value2)
	if u.size[root1] > u.size[root2] {
		root1, root2 = root2, root1
	}
	u.find[root1] = root2
	u.size[root2] += u.size[root1]
	u.next[root1], u.next[root2] = u.next[root2], u.next[root1]
}

func (u *BasicUnionFind) Reset() {
	clear(u.find)
	clear(u.size)
	clear(u.next)
	u.find = u.find[:0]
	u.size = u.size[:0]
	u.next = u.next[:0]
}
