package sat_solver

// Source: src/sat/sat_solver/inc_sat_solver.cpp live TRACE/CTRACE/IF_VERBOSE sites.

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func incrementalTrace(services model.ModelEvaluatorConfig, function string, line int, body func(io.Writer) error) error {
	tags := services.Trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional("sat", function, "src/sat/sat_solver/inc_sat_solver.cpp", line, func() bool { return true }, body)
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
func incrementalVerbose(level uint32, text string) error {
	enabled, err := diagnostic.ProcessVerboseEnabled(level)
	if err != nil || !enabled {
		return err
	}
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	err = out.AppendCString(text)
	if errors.Is(err, arena.ErrClosed) {
		return nil
	}
	return err
}
func (s *Solver) emit(function string, line int, body func(io.Writer) error) error {
	return incrementalTrace(s.services, function, line, body)
}
func (s *Solver) writeExpression(out io.Writer, h ast.ASTHandle) error {
	p, err := ast.NewSimpleASTPrinterContext(s.manager, nil, nil)
	if err != nil {
		return err
	}
	return p.DisplayExpr(out, h, 0)
}
func (s *Solver) emitVector(function string, line int, values []ast.ASTHandle) error {
	return s.emit(function, line, func(out io.Writer) error {
		text, err := s.formatExpressions(values)
		if err != nil {
			return err
		}
		_, err = io.WriteString(out, text+"\n")
		return err
	})
}
func (s *Solver) emitUnknown() error {
	if err := s.emit("internalize_goal", 422, func(out io.Writer) error { _, err := io.WriteString(out, s.unknown+"\n"); return err }); err != nil {
		return err
	}
	return incrementalVerbose(1, s.unknown+"\n")
}
func (s *Solver) emitVariable(v ast.ASTHandle, variables []sat.BoolVar, complete bool, bitvector bool) error {
	if complete && bitvector {
		return s.emit("internalize_var", 484, func(out io.Writer) error {
			if _, err := io.WriteString(out, "var: "); err != nil {
				return err
			}
			for _, v := range variables {
				if _, err := fmt.Fprintf(out, "%d ", v); err != nil {
					return err
				}
			}
			_, err := io.WriteString(out, "\n")
			return err
		})
	}
	if !complete {
		return s.emit("internalize_var", 489, func(out io.Writer) error {
			if _, err := io.WriteString(out, "unhandled variable "); err != nil {
				return err
			}
			if err := s.writeExpression(out, v); err != nil {
				return err
			}
			_, err := io.WriteString(out, "\n")
			return err
		})
	}
	return nil
}
func (s *Solver) emitCore(deps map[ast.ASTHandle]sat.Literal, originals map[ast.ASTHandle]ast.ASTHandle) error {
	return s.emit("extract_core", 606, func(out io.Writer) error {
		for f, l := range deps {
			if err := s.writeExpression(out, f); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(out, " |-> %s\n", l.String()); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(out, "asm2fml: "); err != nil {
			return err
		}
		for a, f := range originals {
			if err := s.writeExpression(out, a); err != nil {
				return err
			}
			if _, err := io.WriteString(out, " |-> "); err != nil {
				return err
			}
			if err := s.writeExpression(out, f); err != nil {
				return err
			}
			if _, err := io.WriteString(out, "\n"); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(out, "core: "); err != nil {
			return err
		}
		core, err := s.core.BorrowCore()
		if err != nil {
			return err
		}
		count, err := core.Size()
		if err != nil {
			return err
		}
		for i := uint32(0); i < count; i++ {
			l, e := core.Read(i)
			if e != nil {
				return e
			}
			if _, e = fmt.Fprintf(out, "%s ", l.String()); e != nil {
				return e
			}
		}
		_, err = io.WriteString(out, "\n")
		return err
	})
}
func (s *Solver) badAssumption(f ast.ASTHandle) error {
	var text strings.Builder
	if err := s.writeExpression(&text, f); err != nil {
		return err
	}
	text.WriteString(" does not evaluate to true\n")
	for i, l := range s.assumptions {
		if i > 0 {
			text.WriteByte(' ')
		}
		text.WriteString(l.String())
	}
	text.WriteByte('\n')
	if err := incrementalVerbose(0, text.String()); err != nil {
		return err
	}
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	if err = out.CheckOpen(); errors.Is(err, arena.ErrClosed) {
		return nil
	} else if err != nil {
		return err
	}
	if err = s.core.DisplayAssignment(out); err != nil {
		return err
	}
	return s.core.DisplayInto(out)
}

func (s *Solver) prepareDIMACS() (*modelConversion, error) {
	enabled, err := s.core.DIMACSDisplayEnabled()
	if err != nil || !enabled {
		return nil, err
	}
	w := &modelConversion{}
	if s.dimacs == nil {
		s.dimacs, err = utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: s.manager.Budget()})
		if err != nil {
			return nil, err
		}
	} else {
		if err = s.dimacs.ResetText(); err != nil {
			return nil, err
		}
	}
	w.stream = s.dimacs
	if err = s.core.BindDIMACS(w.stream); err != nil {
		return nil, err
	}
	return w, nil
}
func (s *Solver) finishDIMACS(w *modelConversion, unknown bool) error {
	if w == nil {
		return nil
	}
	text, err := w.stream.String()
	if err != nil {
		return err
	}
	if _, err = io.WriteString(s.output, text); err != nil {
		return err
	}
	if !unknown {
		return nil
	}
	entries, err := s.encoder.atoms.Entries()
	if err != nil {
		return err
	}
	sort.Slice(entries, func(left, right int) bool { return entries[left].Variable < entries[right].Variable })
	for _, entry := range entries {
		if _, err = fmt.Fprintf(s.output, "c %d ", entry.Variable); err != nil {
			return err
		}
		if err = s.writeExpression(s.output, entry.Expr); err != nil {
			return err
		}
		if _, err = io.WriteString(s.output, "\n"); err != nil {
			return err
		}
	}
	return nil
}
