package smtcore

// Source: src/smt/smt_case_split_queue.cpp live TRACE and AST trace-stream records.

import (
	"fmt"
	"io"
	"math"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Preserve the native endl flush while keeping ostream failures non-fatal.
type smtCaseSplitTraceWriter struct{ assertedTraceWriter }

func (w smtCaseSplitTraceWriter) Flush() error {
	if f, ok := w.out.(interface{ Flush() error }); ok {
		_ = f.Flush()
	}
	return nil
}

func (s *SMTInternalizerState) emitCaseSplitTrace(tag, function string, line int, body func(io.Writer) error) error {
	tags := s.queueTrace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	var operandErr error
	err := tags.EmitConditional(tag, function, "src/smt/smt_case_split_queue.cpp", line, func() bool { return true }, func(out io.Writer) error {
		operandErr = body(smtCaseSplitTraceWriter{assertedTraceWriter{out}})
		return operandErr
	})
	if operandErr != nil {
		return operandErr
	}
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func (s *SMTInternalizerState) traceRandomSplit(line int, variable int32, value lbool.LBool) error {
	return s.emitCaseSplitTrace("random_split", "next_case_split", line, func(out io.Writer) error {
		return smtWriteTheoryText(out, fmt.Sprintf("next: %d get_assignment(next): %s\n", variable, value))
	})
}

func (s *SMTInternalizerState) displayCaseSplitQueue(out io.Writer, strategy uint32) error {
	switch strategy {
	case 0, 1, 2:
		return s.DisplayBasicActivityQueue(out)
	case 3:
		return s.DisplayRelevanceCaseSplitQueue(out)
	case 4:
		return s.DisplayRelevanceActivityQueue(out)
	case 5:
		return s.DisplayRelevanceGoalQueue(out)
	case 6:
		if s.theoryAwareBranching == nil {
			return nil
		}
		return s.theoryAwareBranching.display(out)
	}
	return nil
}

func (s *SMTInternalizerState) traceCaseSplitScope(pop bool) error {
	var line, head int
	switch s.caseSplitStrategy {
	case 3:
		line, head = 387, s.relevanceFIFO.firstHead
	case 4:
		line, head = 581, s.relevanceActivity.head
	case 5:
		line, head = 827, s.relevanceGoal.head
	default:
		return nil
	}
	function := "push_scope"
	if pop {
		function = "pop_scope"
		switch s.caseSplitStrategy {
		case 3:
			line = 400
		case 4:
			line = 592
		case 5:
			line = 860
		}
	}
	return s.emitCaseSplitTrace("case_split", function, line, func(out io.Writer) error {
		if pop {
			if err := s.displayCaseSplitQueue(out, s.caseSplitStrategy); err != nil {
				return err
			}
		}
		return smtWriteTheoryText(out, fmt.Sprintf("head: %d\n", head))
	})
}

func (q *SMTActivityCaseSplitQueue) traceCaseSplitCandidate(expression, child ast.ASTHandle) error {
	line := 437
	if q.strategy == 4 {
		line = 626
	} else if q.strategy == 5 {
		line = 893
	}
	if !child.IsNull() {
		if q.strategy == 3 {
			line = 427
		} else if q.strategy == 4 {
			line = 616
		} else {
			line = 883
		}
	}
	return q.state.emitCaseSplitTrace("case_split", "next_case_split_core", line, func(out io.Writer) error {
		id, err := q.state.manager.GetASTID(expression)
		if err != nil {
			return err
		}
		if child.IsNull() {
			return smtWriteTheoryText(out, fmt.Sprintf("found candidate: #%d\n", id))
		}
		childID, err := q.state.manager.GetASTID(child)
		if err != nil {
			return err
		}
		return smtWriteTheoryText(out, fmt.Sprintf("found AND/OR candidate: #%d #%d\n", id, childID))
	})
}

func (q *SMTActivityCaseSplitQueue) traceCaseSplitQueue(child bool) error {
	line := 439
	if q.strategy == 4 {
		line = 628
	} else if q.strategy == 5 {
		line = 895
	}
	if child {
		if q.strategy == 3 {
			line = 431
		} else if q.strategy == 4 {
			line = 620
		} else {
			line = 887
		}
	}
	return q.state.emitCaseSplitTrace("case_split", "next_case_split_core", line, func(out io.Writer) error { return q.state.displayCaseSplitQueue(out, q.strategy) })
}

// Profiles 3 and 5 also emit an unconditional record to the AST manager's
// separate stream. Profile 4 has no such source record.
func (q *SMTActivityCaseSplitQueue) traceDecideAndOr(expression, child ast.ASTHandle) error {
	if q.strategy != 3 && q.strategy != 5 || q.state.manager.TraceStream == nil {
		return nil
	}
	out := q.state.manager.TraceStream
	if err := out.AppendCString("[decide-and-or] #"); err != nil {
		return err
	}
	id, err := q.state.manager.GetASTID(expression)
	if err != nil {
		return err
	}
	if err = out.AppendUint32(id); err != nil {
		return err
	}
	if err = out.AppendCString(" #"); err != nil {
		return err
	}
	id, err = q.state.manager.GetASTID(child)
	if err != nil {
		return err
	}
	if err = out.AppendUint32(id); err != nil {
		return err
	}
	return out.AppendCString("\n")
}

func (s *SMTInternalizerState) traceTheoryPriority(variable int32, priority float64) error {
	return s.emitCaseSplitTrace("theory_aware_branching", "add_theory_aware_branching_info", 1191, func(out io.Writer) error {
		if err := smtWriteTheoryText(out, fmt.Sprintf("Add theory-aware branching information for l#%d: priority=", variable)); err != nil {
			return err
		}
		text := strconv.FormatFloat(priority, 'g', 6, 64)
		if math.IsInf(priority, 0) {
			text = "inf"
			if math.Signbit(priority) {
				text = "-inf"
			}
		} else if math.IsNaN(priority) {
			text = "nan"
			if math.Signbit(priority) {
				text = "-nan"
			}
		}
		if err := smtWriteTheoryText(out, text); err != nil {
			return err
		}
		return diagnostic.WriteTraceEndl(out)
	})
}

func (s *SMTInternalizerState) traceFoundGoal(expression ast.ASTHandle) error {
	return s.emitCaseSplitTrace("case_split", "assign_lit_eh", 975, func(out io.Writer) error {
		if err := smtWriteTheoryText(out, "Found goal\n"); err != nil {
			return err
		}
		printer, err := ast.NewSimpleASTPrinterContext(s.manager, out, out)
		if err != nil {
			return err
		}
		if err = printer.DisplayExpr(out, expression, 0); err != nil {
			return err
		}
		return smtWriteTheoryText(out, "\n")
	})
}
