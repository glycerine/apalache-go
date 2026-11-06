package lp

// Source: src/test/lp.cpp:1933-2058, retained --smap scenario.

import (
	"io"
	"strconv"
)

// StackedTestDriver owns the source scenario's native container values.
// Completed counts finished map/value/vector/set tests. A report failure keeps
// the completed prefix for inspection. Results must not be mutated during Run.
type StackedTestDriver struct {
	Map       StackedInt32FloatMap
	Value     StackedUint32
	Vector    StackedIntVector
	Set       *StackedInt32Set
	Completed uint32
	started   bool
}

func (d *StackedTestDriver) Run(out io.Writer) error {
	if d.started {
		return ErrExactVectorPending
	}
	d.started = true
	out = driverReportOutput(out)
	if err := writeDriverLine(out, "test_stacked_map()"); err != nil {
		return err
	}
	if err := d.runMap(out); err != nil {
		return err
	}
	d.Completed++
	if err := d.runValue(out); err != nil {
		return err
	}
	d.Completed++
	if err := d.runVector(out); err != nil {
		return err
	}
	d.Completed++
	if err := d.runSet(out); err != nil {
		return err
	}
	d.Completed++
	return nil
}

func (d *StackedTestDriver) runMap(out io.Writer) error {
	for _, v := range []int32{0, 0, 0} {
		if err := writeDriverLine(out, strconv.FormatInt(int64(v), 10)); err != nil {
			return err
		}
	}
	// Native concrete lookup replaces the source unordered_map<int,fff>.
	foo := map[int32]struct{ a, b int32 }{1: {0, 1}}
	if err := writeDriverLine(out, strconv.FormatInt(int64(foo[1].a), 10)); err != nil {
		return err
	}
	m := &d.Map
	m.Set(0, 3)
	m.Set(1, 4)
	m.Push()
	m.Set(1, 5)
	m.Set(2, 2)
	m.Pop(1)
	if err := m.Erase(2); err != nil {
		return err
	}
	m.Set(2, 3)
	if err := m.Erase(1); err != nil {
		return err
	}
	m.Push()
	m.Set(3, 100)
	m.Set(4, 200)
	if err := m.Erase(1); err != nil {
		return err
	}
	m.Push()
	m.Set(5, 300)
	m.Set(6, 400)
	m.Set(5, 301)
	if err := m.Erase(5); err != nil {
		return err
	}
	m.Set(3, 122)
	m.Pop(2)
	m.Pop(1)
	return nil
}

func (d *StackedTestDriver) runValue(out io.Writer) error {
	if err := writeDriverLine(out, "test stacked unsigned"); err != nil {
		return err
	}
	v := &d.Value
	*v = NewStackedUint32(0)
	v.Value = 1
	v.Value = 2
	v.Push()
	v.Value = 3
	v.Value = 4
	v.Pop(1)
	if v.Value != 2 {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	v.Value++
	v.Value++
	if err := writeDriverLine(out, "before push v="+strconv.FormatUint(uint64(v.Value), 10)); err != nil {
		return err
	}
	v.Push()
	v.Value++
	v.Push()
	v.Value += 1
	if err := writeDriverLine(out, "v = "+strconv.FormatUint(uint64(v.Value), 10)); err != nil {
		return err
	}
	v.Pop(2)
	if v.Value != 4 {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	return writeDriverLine(out, strconv.FormatUint(uint64(v.Value), 10))
}

func (d *StackedTestDriver) runVector(out io.Writer) error {
	if err := writeDriverLine(out, "test_stacked_vector"); err != nil {
		return err
	}
	v := &d.Vector
	v.Push()
	v.Append(0)
	v.Append(1)
	v.Push()
	if err := v.Set(0, 3); err != nil {
		return err
	}
	if err := v.Set(0, 0); err != nil {
		return err
	}
	v.Append(2)
	v.Append(3)
	v.Append(34)
	v.Push()
	if err := v.Set(1, 3); err != nil {
		return err
	}
	if err := v.Set(2, 3); err != nil {
		return err
	}
	v.Push()
	if err := v.Set(0, 7); err != nil {
		return err
	}
	if err := v.Set(1, 9); err != nil {
		return err
	}
	if err := v.Pop(2); err != nil {
		return err
	}
	if v.Size() != 0 {
		if err := v.Set(v.Size()-1, 7); err != nil {
			return err
		}
	}
	v.Push()
	v.Append(33)
	if err := v.Set(0, 13); err != nil {
		return err
	}
	return v.Pop(1)
}

func (d *StackedTestDriver) runSet(out io.Writer) error {
	if !sourceDebugEnabled {
		return nil
	}
	if err := writeDriverLine(out, "test_stacked_set"); err != nil {
		return err
	}
	d.Set = &StackedInt32Set{}
	s := d.Set
	s.Insert(1)
	s.Insert(2)
	s.Insert(3)
	copy := make(map[int32]struct{}, len(s.Values()))
	for key := range s.Values() {
		copy[key] = struct{}{}
	}
	s.Push()
	s.Insert(4)
	if err := s.Pop(1); err != nil {
		return err
	}
	if !driverStackedSetsEqual(s.Values(), copy) {
		return ErrAssertion
	}
	s.Push()
	s.Push()
	s.Insert(4)
	s.Insert(5)
	s.Push()
	s.Insert(4)
	if err := s.Pop(3); err != nil {
		return err
	}
	if !driverStackedSetsEqual(s.Values(), copy) {
		return ErrAssertion
	}
	return nil
}

func driverStackedSetsEqual(a, b map[int32]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for key := range a {
		if _, ok := b[key]; !ok {
			return false
		}
	}
	return true
}
