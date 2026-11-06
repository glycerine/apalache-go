package arith

// Source: src/tactic/arith/propagate_ineqs_tactic.cpp:99-116,492,509-510.

import (
	"errors"
	"io"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	goaltactic "github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (b *PolynomialBuilder) traceSMT2Term(term ast.ASTHandle) (body string, result error) {
	g := uparams.BorrowProcessGlobalParamsService()
	if g == nil || g.Context == nil {
		return "", uparams.ErrGlobalParamsOwner
	}
	format, err := b.manager.SMT2PPExprTree(term, nil)
	if err != nil {
		return "", err
	}
	out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: b.manager.BudgetField})
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, out.Close()) }()
	empty := g.Context.EmptyValue()
	if err = b.manager.PrintFormat(out, format, g.Context, &empty, g); err != nil {
		return "", err
	}
	return out.String()
}

// displayBounds follows the tactic's expression order, not the bound owner's
// internal variable display. One reusable rational receives each source lower
// and upper query before it is deleted at function exit.
func (b *PolynomialBuilder) displayBounds(out io.Writer) (result error) {
	value := numeral.NewRational()
	defer func() { result = errors.Join(result, b.store.manager.Del(&value)) }()
	for x, term := range b.var2expr {
		found, strict, _, err := b.store.Lower(uint32(x), &value)
		if err != nil {
			return err
		}
		if found {
			s, err := b.store.manager.String(&value)
			if err != nil {
				return err
			}
			operator := " <="
			if strict {
				operator = " <"
			}
			if err = writeBoundText(out, s+operator); err != nil {
				return err
			}
		} else if err = writeBoundText(out, "-oo <"); err != nil {
			return err
		}
		s, err := b.traceSMT2Term(term)
		if err != nil {
			return err
		}
		if err = writeBoundText(out, " "+s+" "); err != nil {
			return err
		}
		found, strict, _, err = b.store.Upper(uint32(x), &value)
		if err != nil {
			return err
		}
		if found {
			s, err = b.store.manager.String(&value)
			if err != nil {
				return err
			}
			operator := "<= "
			if strict {
				operator = "< "
			}
			if err = writeBoundText(out, operator+s); err != nil {
				return err
			}
		} else if err = writeBoundText(out, "< oo"); err != nil {
			return err
		}
		if err = writeBoundText(out, "\n"); err != nil {
			return err
		}
	}
	return nil
}

func (b *PolynomialBuilder) emitTacticTrace(line int, body string) {
	tags := b.store.boundTraceTags()
	_ = tags.EmitConditional("propagate_ineqs_tactic", "operator()", "src/tactic/arith/propagate_ineqs_tactic.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			_, err := io.WriteString(w, body)
			return err
		})
}

func (b *PolynomialBuilder) traceBeforePropagation(input *goaltactic.Goal) error {
	if !b.store.boundTraceTags().ShouldTrace("propagate_ineqs_tactic") {
		return nil
	}
	var body strings.Builder
	if err := input.Display(&body); err != nil {
		return err
	}
	if err := b.displayBounds(&body); err != nil {
		return err
	}
	body.WriteString("bound propagator:\n")
	if err := b.store.Display(&body); err != nil {
		return err
	}
	b.emitTacticTrace(492, body.String())
	return nil
}

func (b *PolynomialBuilder) traceAfterPropagation() error {
	if !b.store.boundTraceTags().ShouldTrace("propagate_ineqs_tactic") {
		return nil
	}
	var body strings.Builder
	body.WriteString("after propagation:\n")
	if err := b.displayBounds(&body); err != nil {
		return err
	}
	if err := b.store.Display(&body); err != nil {
		return err
	}
	b.emitTacticTrace(509, body.String())
	return nil
}

// The final TRACE reads the restored goal after all bound assertions. The
// process printer is reached only when the tag and compile gates are open.
func (b *PolynomialBuilder) traceResultGoal(result *goaltactic.Goal) error {
	if !b.store.boundTraceTags().ShouldTrace("propagate_ineqs_tactic") {
		return nil
	}
	var body strings.Builder
	if err := result.Display(&body); err != nil {
		return err
	}
	b.emitTacticTrace(510, body.String())
	return nil
}
