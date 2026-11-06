package sat

// Source: src/sat/sat_integrity_checker.cpp:162-181; src/util/trace.h:40-46.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (c satIntegrityChecker) watchListText(list satSolverWatchListView) (body string, result error) {
	out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: c.lifecycle.budget})
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, out.Close()) }()
	ref, err := list.ref()
	if err != nil {
		return "", err
	}
	if err = list.owner.store.DisplayInto(ref, c.lifecycle.propagation.clauses, out); err != nil {
		return "", err
	}
	return out.String()
}

func (c satIntegrityChecker) eliminatedWatchDisplayText(watches bool) (body string, result error) {
	s := c.gc.conflict.solver
	out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: c.lifecycle.budget})
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, out.Close()) }()
	if watches {
		err = s.displayWatches(out)
	} else {
		err = s.displayInto(out)
	}
	if err != nil {
		return "", err
	}
	return out.String()
}

func (c satIntegrityChecker) traceEliminatedWatch(l Literal) {
	tags := c.lifecycle.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("sat_bug") || c.gc.conflict.solver == nil {
		return
	}
	_ = tags.EmitConditional("sat_bug", "check_watches", "src/sat/sat_integrity_checker.cpp", 162,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "l: "+l.String()+"\n")
			body, err := c.eliminatedWatchDisplayText(true)
			if err != nil {
				return err
			}
			_, _ = io.WriteString(w, body)
			body, err = c.eliminatedWatchDisplayText(false)
			if err != nil {
				return err
			}
			_, _ = io.WriteString(w, body)
			return nil
		})
}

func (c satIntegrityChecker) traceMissingBinaryWatch(literal, other Literal, learned, ownGone, otherGone bool, current, reciprocal satSolverWatchListView) {
	tags := c.lifecycle.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("sat_watched_bug") {
		return
	}
	_ = tags.EmitConditional("sat_watched_bug", "check_watches", "src/sat/sat_integrity_checker.cpp", 172,
		func() bool { return true }, func(w io.Writer) error {
			boolDigit := func(v bool) string {
				if v {
					return "1"
				}
				return "0"
			}
			_, _ = io.WriteString(w, "l: "+literal.String()+" l2: "+other.String()+"\n")
			_, _ = io.WriteString(w, "was_eliminated1: "+boolDigit(ownGone)+" was_eliminated2: "+boolDigit(otherGone)+" learned: "+boolDigit(learned)+"\n")
			currentText, err := c.watchListText(current)
			if err != nil {
				return err
			}
			_, _ = io.WriteString(w, currentText)
			_, _ = io.WriteString(w, "\n")
			reciprocalText, err := c.watchListText(reciprocal)
			if err != nil {
				return err
			}
			_, _ = io.WriteString(w, reciprocalText)
			_, _ = io.WriteString(w, "\n")
			return nil
		})
}
