package apalache

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSMTQueryContextAppliesFreshTimeout(t *testing.T) {
	configured := WithSMTQueryTimeout(context.Background(), time.Minute)
	if _, hasDeadline := configured.Deadline(); hasDeadline {
		t.Fatal("query timeout unexpectedly became a whole-operation deadline")
	}
	first, cancelFirst := smtQueryContext(configured)
	defer cancelFirst()
	second, cancelSecond := smtQueryContext(configured)
	defer cancelSecond()
	firstDeadline, firstOK := first.Deadline()
	secondDeadline, secondOK := second.Deadline()
	if !firstOK || !secondOK {
		t.Fatal("query context has no deadline")
	}
	if secondDeadline.Before(firstDeadline) {
		t.Fatalf("second query deadline %s is before first deadline %s", secondDeadline, firstDeadline)
	}
}

func TestG3CheckHonorsSMTQueryTimeout(t *testing.T) {
	solver, err := NewG3SolverContext()
	if err != nil {
		t.Fatal(err)
	}
	defer solver.Close()
	_, err = solver.Check(WithSMTQueryTimeout(context.Background(), time.Nanosecond))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timed query error = %v, want context deadline exceeded", err)
	}
}

func TestG3CheckRecoversAfterRepeatedCancellation(t *testing.T) {
	solver := newTestG3Solver(t)
	truth, err := solver.Bool(true)
	if err != nil {
		t.Fatal(err)
	}
	if err = solver.Assert(truth); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 64; index++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if status, checkErr := solver.Check(ctx); !errors.Is(checkErr, context.Canceled) || status != SMTUnknown {
			t.Fatalf("canceled check %d = %v, %v", index, status, checkErr)
		}
	}
	if status, err := solver.Check(context.Background()); err != nil || status != SMTSatisfiable {
		t.Fatalf("post-cancellation check = %v, %v", status, err)
	}
}

func TestSMTQueryContextPreservesEarlierParentDeadline(t *testing.T) {
	parent, cancelParent := context.WithTimeout(context.Background(), time.Minute)
	defer cancelParent()
	parentDeadline, _ := parent.Deadline()
	query, cancelQuery := smtQueryContext(WithSMTQueryTimeout(parent, 2*time.Minute))
	defer cancelQuery()
	queryDeadline, ok := query.Deadline()
	if !ok || !queryDeadline.Equal(parentDeadline) {
		t.Fatalf("query deadline = %s, want parent deadline %s", queryDeadline, parentDeadline)
	}
}
