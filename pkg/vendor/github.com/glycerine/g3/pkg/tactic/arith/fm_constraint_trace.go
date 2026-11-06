package arith

// Source: src/tactic/arith/fm_tactic.cpp:618-648,749,1173,1366-1514,1577,1611-1633.

import (
	"fmt"
	"io"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type fmConstraintDiagnostics struct {
	manager     *ast.ASTManager
	services    model.ModelEvaluatorConfig
	expressions *FMExpressionTable
}

func (t *FMTactic) BindTrace(tags *diagnostic.TraceTags) error {
	if err := t.open(); err != nil {
		return err
	}
	t.services.Trace = tags
	return nil
}
func (d *fmConstraintDiagnostics) emit(tag, function string, line int, body func(io.Writer) error) error {
	if d == nil {
		return nil
	}
	return emitFMTrace(d.manager, d.services, tag, function, line, body)
}
func (d *fmConstraintDiagnostics) variable(out io.Writer, x uint32) error {
	term, err := d.expressions.Variable(x)
	if err != nil {
		return err
	}
	return writeFMExpression(d.manager, d.services, out, term)
}
func (d *fmConstraintDiagnostics) constraint(out io.Writer, c *FMConstraint) error {
	for _, literal := range c.Literals {
		id := literal
		if id < 0 {
			id = -id
			if err := fmDisplayWrite(out, "~"); err != nil {
				return err
			}
		}
		term, err := d.expressions.Boolean(uint32(id))
		if err != nil {
			return err
		}
		if err = writeFMExpression(d.manager, d.services, out, term); err != nil {
			return err
		}
		if err = fmDisplayWrite(out, " "); err != nil {
			return err
		}
	}
	if err := fmDisplayWrite(out, "("); err != nil {
		return err
	}
	if len(c.Variables) == 0 {
		if err := fmDisplayWrite(out, "0"); err != nil {
			return err
		}
	}
	for i, x := range c.Variables {
		if i > 0 {
			if err := fmDisplayWrite(out, " + "); err != nil {
				return err
			}
		}
		if !c.Coefficients[i].IsInt() || c.Coefficients[i].Num().Cmp(big.NewInt(1)) != 0 {
			if err := fmDisplayWrite(out, c.Coefficients[i].RatString()+"*"); err != nil {
				return err
			}
		}
		if err := d.variable(out, x); err != nil {
			return err
		}
	}
	comparison := " <= "
	if c.Strict {
		comparison = " < "
	}
	return fmDisplayWrite(out, comparison+c.Constant.RatString()+")")
}
func (d *fmConstraintDiagnostics) constraints(out io.Writer, cs []*FMConstraint) error {
	for _, c := range cs {
		if err := fmDisplayWrite(out, "  "); err != nil {
			return err
		}
		if err := d.constraint(out, c); err != nil {
			return err
		}
		if err := fmDisplayWrite(out, "\n"); err != nil {
			return err
		}
	}
	return nil
}
func (d *fmConstraintDiagnostics) state() error {
	return d.emit("fm", "operator()", 1577, func(out io.Writer) error {
		v := d.expressions.variables
		for x := range v.Integer {
			if v.IsForbidden(uint32(x)) {
				continue
			}
			if err := d.variable(out, uint32(x)); err != nil {
				return err
			}
			if err := fmDisplayWrite(out, "\n"); err != nil {
				return err
			}
			if err := d.constraints(out, v.Index.Lower[uint32(x)]); err != nil {
				return err
			}
			if err := d.constraints(out, v.Index.Upper[uint32(x)]); err != nil {
				return err
			}
		}
		return nil
	})
}
func (d *fmConstraintDiagnostics) candidates(xs []uint32) error {
	return d.emit("fm", "sort_candidates", 1173, func(out io.Writer) error {
		for _, x := range xs {
			if err := fmDisplayWrite(out, "("); err != nil {
				return err
			}
			if err := d.variable(out, x); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(out, " %d) ", d.expressions.variables.Cost(x)); err != nil {
				return err
			}
		}
		return fmDisplayWrite(out, "\n")
	})
}
func (d *fmConstraintDiagnostics) goal(goal *tactic.Goal) error {
	return d.emit("fm", "operator()", 1611, func(out io.Writer) error {
		if err := fmDisplayWrite(out, "(goal"); err != nil {
			return err
		}
		size, err := goal.Size()
		if err != nil {
			return err
		}
		for i := uint32(0); i < size; i++ {
			form, err := goal.Form(i)
			if err != nil {
				return err
			}
			if err = fmDisplayWrite(out, "\n  "); err != nil {
				return err
			}
			body, err := fmExpressionText(d.manager, d.services, form, 2)
			if err != nil {
				return err
			}
			if err = fmDisplayWrite(out, body); err != nil {
				return err
			}
		}
		return fmDisplayWrite(out, ")\n")
	})
}
func (d *fmConstraintDiagnostics) subsumed(c, other *FMConstraint) error {
	return d.emit("fm_subsumption", "backward_subsumption", 749, func(out io.Writer) error {
		if err := d.constraint(out, c); err != nil {
			return err
		}
		if err := fmDisplayWrite(out, "\nsubsumed:\n"); err != nil {
			return err
		}
		if err := d.constraint(out, other); err != nil {
			return err
		}
		return fmDisplayWrite(out, "\n")
	})
}
func (d *fmConstraintDiagnostics) resolution(x uint32, lower, upper, result *FMConstraint, reason string, line int) error {
	return d.emit("fm", "resolve", line, func(out io.Writer) error {
		if _, err := fmt.Fprintf(out, "resolution %d%s\n", x, reason); err != nil {
			return err
		}
		if err := d.constraint(out, lower); err != nil {
			return err
		}
		if err := fmDisplayWrite(out, "\n"); err != nil {
			return err
		}
		if err := d.constraint(out, upper); err != nil {
			return err
		}
		if result != nil {
			if err := fmDisplayWrite(out, "\n---->\n"); err != nil {
				return err
			}
			if err := d.constraint(out, result); err != nil {
				return err
			}
			address, err := d.manager.DependencyAddressText(result.Dependency)
			if err != nil {
				return err
			}
			return fmDisplayWrite(out, "\nnew_dep: "+address+"\n")
		}
		return fmDisplayWrite(out, "\n")
	})
}
func (d *fmConstraintDiagnostics) elimination(x uint32, lower, upper []*FMConstraint) error {
	return d.emit("fm_bug", "try_eliminate", 1479, func(out io.Writer) error {
		if err := fmDisplayWrite(out, "eliminating "); err != nil {
			return err
		}
		if err := d.variable(out, x); err != nil {
			return err
		}
		if err := fmDisplayWrite(out, "\nlowers:\n"); err != nil {
			return err
		}
		if err := d.constraints(out, lower); err != nil {
			return err
		}
		if err := fmDisplayWrite(out, "uppers:\n"); err != nil {
			return err
		}
		return d.constraints(out, upper)
	})
}
func (d *fmConstraintDiagnostics) eliminationEnd(old, created uint32, trivial, overflow bool) error {
	line := 1514
	if trivial {
		line = 1461
	} else if overflow {
		line = 1489
	}
	return d.emit("fm", "try_eliminate", line, func(out io.Writer) error {
		if trivial {
			return fmDisplayWrite(out, "variables was eliminated (trivial case)\n")
		}
		if overflow {
			_, err := fmt.Fprintf(out, "too many new constraints: %d\n", created)
			return err
		}
		_, err := fmt.Fprintf(out, "variables was eliminated old: %d new_constraints: %d\n", old, created)
		return err
	})
}
