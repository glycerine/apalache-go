package arith

// Source: src/tactic/arith/fix_dl_var_tactic.cpp live diagnostics.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (t *FixDLVarTactic) BindTrace(tags *diagnostic.TraceTags) error {
	if err := t.open(); err != nil {
		return err
	}
	t.trace = tags
	return nil
}
func (t *FixDLVarTactic) emitDL(tag, function string, line int, body func(io.Writer) error) error {
	tags := t.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional(tag, function, "src/tactic/arith/fix_dl_var_tactic.cpp", line, func() bool { return true }, body)
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
func (t *FixDLVarTactic) writeDLExpr(out io.Writer, term ast.ASTHandle) (err error) {
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: t.manager.Budget()})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, stream.Close()) }()
	format, err := t.manager.SMT2PPExprTree(term, nil)
	if err != nil {
		return err
	}
	empty := t.context.EmptyValue()
	if err = t.manager.PrintFormat(stream, format, t.context, &empty, t.globals); err != nil {
		return err
	}
	body, err := stream.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, body)
	return err
}
func (t *FixDLVarTactic) traceDLFailure(first, second ast.ASTHandle) error {
	return t.emitDL("fix_dl_var", "throw_failed", 48, func(out io.Writer) error {
		for _, term := range []ast.ASTHandle{first, second} {
			if term.IsNull() {
				continue
			}
			if err := t.writeDLExpr(out, term); err != nil {
				return err
			}
			if _, err := io.WriteString(out, "\n"); err != nil {
				return err
			}
		}
		return nil
	})
}
func (t *FixDLVarTactic) traceDLAtomShape(left, negative ast.ASTHandle, minus, uninterpretedLeft, uninterpretedRight bool) error {
	return t.emitDL("fix_dl_var", "process_arith_atom", 110, func(out io.Writer) error {
		boolean := func(value bool) int {
			if value {
				return 1
			}
			return 0
		}
		family := func(term ast.ASTHandle) (int, error) {
			f, _, _, app, err := t.dlApp(term)
			if !app {
				return -1, err
			}
			return int(f), err
		}
		leftFamily, err := family(left)
		if err != nil {
			return err
		}
		if _, err = fmt.Fprintf(out, "is_times_minus_one: %d\nis_uninterp(t):     %d\nt.family_id():      %d\nutil.family_id:     %d\n", boolean(minus), boolean(uninterpretedLeft), leftFamily, t.family); err != nil {
			return err
		}
		if !negative.IsNull() {
			f, err := family(negative)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(out, "is_uninterp(s):     %d\ns.family_id():      %d\n", boolean(uninterpretedRight), f)
			return err
		}
		return nil
	})
}
func (t *FixDLVarTactic) traceDLChoice(unit, total ast.ASTHandle, bestUnit, bestTotal uint32) error {
	return t.emitDL("fix_dl_var_choice", "most_occs", 204, func(out io.Writer) error {
		for i, term := range []ast.ASTHandle{unit, total} {
			if term.IsNull() {
				continue
			}
			n := bestUnit
			if i == 1 {
				n = bestTotal
			}
			if _, err := fmt.Fprintf(out, "r%d occs: %d\n", i+1, n); err != nil {
				return err
			}
			if err := t.writeDLExpr(out, term); err != nil {
				return err
			}
			if _, err := io.WriteString(out, "\n"); err != nil {
				return err
			}
		}
		return nil
	})
}
func (t *FixDLVarTactic) traceDLGoal(goal *tactic.Goal) error {
	return t.emitDL("fix_dl_var", "operator()", 293, goal.Display)
}
func (t *FixDLVarTactic) verboseDLChoice(variable ast.ASTHandle) error {
	enabled, err := diagnostic.ProcessVerboseEnabled(10)
	if err != nil || !enabled {
		return err
	}
	decl, err := t.manager.GetDecl(variable)
	if err != nil {
		return err
	}
	name, err := t.manager.GetASTName(decl)
	if err != nil {
		return err
	}
	text, err := t.manager.Symbols.Display(name)
	if err != nil {
		return err
	}
	stream, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	if err = stream.AppendCString("(fixing-at-zero " + text + ")\n"); err != nil {
		return err
	}
	return stream.Flush()
}
