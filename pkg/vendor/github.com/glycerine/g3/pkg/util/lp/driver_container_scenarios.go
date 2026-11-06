package lp

// Source: src/test/lp.cpp:1302-1368,2968-2987.

import (
	"io"
	"strconv"

	z3rand "github.com/glycerine/g3/pkg/util/random"
)

// RunDriverIntSetTest executes the retained --test_int_set scenario, including
// its three reports and source debug assertions. The set uses native storage.
func RunDriverIntSetTest(out io.Writer) error {
	out = driverReportOutput(out)
	s, err := NewIntSet(4)
	if err != nil {
		return err
	}
	if err = s.Insert(2); err != nil {
		return err
	}
	if err = s.Print(out); err != nil {
		return err
	}
	if err = s.Insert(1); err != nil {
		return err
	}
	if err = s.Insert(2); err != nil {
		return err
	}
	if err = s.Print(out); err != nil {
		return err
	}
	if !s.Contains(2) || s.Size() != 2 {
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
	}
	s.Erase(2)
	if s.Size() != 1 {
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
	}
	s.Erase(2)
	if s.Size() != 1 {
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
	}
	if err = s.Print(out); err != nil {
		return err
	}
	if err = s.Insert(3); err != nil {
		return err
	}
	if err = s.Insert(2); err != nil {
		return err
	}
	if err = s.Clear(); err != nil {
		return err
	}
	if s.Size() != 0 {
		return sourceAssertionFailure()
	}
	return nil
}

// RunDriverPairQueueTest binds the source's shared g_rand to a concrete borrowed
// generator. It consumes exactly 300 draws, including draws for duplicate pairs.
// There is no pair report or source assertion in this retained helper.
func RunDriverPairQueueTest(random *z3rand.Generator) error {
	if random == nil {
		return ErrBounds
	}
	q := NewIntPairPriorityQueue(2)
	for k := 0; k < 100; k++ {
		i, j := random.Next()%10, random.Next()%10
		if err := q.Enqueue(i, j, int32(random.Next()%10)); err != nil {
			return err
		}
	}
	if err := q.Remove(5, 5); err != nil {
		return err
	}
	for !q.IsEmpty() {
		if _, err := q.Dequeue(); err != nil {
			return err
		}
	}
	return nil
}

// RunDriverPriorityQueueTest executes -tbq, including its debug-only second
// drain and the actual random pair-queue helper. Random state is borrowed and
// remains untouched if an earlier report fails. It is shared across source calls.
func RunDriverPriorityQueueTest(out io.Writer, random *z3rand.Generator) error {
	if random == nil {
		return ErrBounds
	}
	out = driverReportOutput(out)
	if _, err := io.WriteString(out, "testing binary_heap_priority_queue..."); err != nil {
		return err
	}
	q := NewIntPriorityQueue(10)
	for _, p := range [][2]int32{{2, 2}, {1, 1}, {9, 9}, {8, 8}, {5, 25}, {3, 3}, {4, 4}, {7, 30}, {6, 6}, {0, 0}, {5, 5}, {7, 7}} {
		if err := q.Enqueue(uint32(p[0]), p[1]); err != nil {
			return err
		}
	}
	for i := uint32(0); i < 10; i++ {
		d, err := q.Dequeue()
		if err != nil {
			return err
		}
		if i != d {
			if err = sourceAssertionFailure(); err != nil {
				return err
			}
		}
		if err = writeDriverLine(out, strconv.FormatUint(uint64(d), 10)); err != nil {
			return err
		}
	}
	for _, p := range [][2]int32{{2, 2}, {1, 1}, {9, 9}, {8, 8}, {5, 5}, {3, 3}, {4, 4}, {7, 2}, {0, 1}, {6, 6}, {7, 7}, {33, 1000}, {20, 0}} {
		if err := q.Enqueue(uint32(p[0]), p[1]); err != nil {
			return err
		}
	}
	if _, err := q.Dequeue(); err != nil {
		return err
	}
	if err := q.Remove(33); err != nil {
		return err
	}
	if err := q.Enqueue(0, 0); err != nil {
		return err
	}
	if sourceDebugEnabled {
		for t := uint32(0); !q.IsEmpty(); t++ {
			d, err := q.Dequeue()
			if err != nil {
				return err
			}
			if t != d {
				return ErrAssertion
			}
			if err = writeDriverLine(out, strconv.FormatUint(uint64(d), 10)); err != nil {
				return err
			}
		}
	}
	if err := RunDriverPairQueueTest(random); err != nil {
		return err
	}
	return writeDriverLine(out, " done")
}
