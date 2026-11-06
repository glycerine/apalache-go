package arith

// Source: src/tactic/arith/fm_tactic.cpp:481,557,820,971,1070,1081,1117,1530.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

func (d *fmConstraintDiagnostics) recognition(term ast.ASTHandle) error {
	return d.emit("is_occ_bug", "is_linear_ineq", 481, func(out io.Writer) error {
		// ast_pp.h defines mk_pp as the SMT2 printer used by mk_ismt2_pp.
		if err := writeFMExpression(d.manager, d.services, out, term); err != nil {
			return err
		}
		return fmDisplayWrite(out, "\n")
	})
}
func (d *fmConstraintDiagnostics) notOCC(term ast.ASTHandle) error {
	return d.emit("is_occ_bug", "init_forbidden_set", 820, func(out io.Writer) error {
		if err := fmDisplayWrite(out, "not OCC:\n"); err != nil {
			return err
		}
		if err := writeFMExpression(d.manager, d.services, out, term); err != nil {
			return err
		}
		return fmDisplayWrite(out, "\n")
	})
}
func (d *fmConstraintDiagnostics) toVariable(term ast.ASTHandle, id uint32) error {
	return d.emit("to_var_bug", "to_var", 971, func(out io.Writer) error {
		if err := writeFMExpression(d.manager, d.services, out, term); err != nil {
			return err
		}
		_, err := fmt.Fprintf(out, " --> %d\n", id)
		return err
	})
}
func (d *fmConstraintDiagnostics) inputVariables(xs []uint32) error {
	return d.emit("to_var_bug", "add_constraint", 1070, func(out io.Writer) error {
		if err := fmDisplayWrite(out, "before mk_constraint: "); err != nil {
			return err
		}
		for _, x := range xs {
			if _, err := fmt.Fprintf(out, " %d", x); err != nil {
				return err
			}
		}
		return fmDisplayWrite(out, "\n")
	})
}
func (d *fmConstraintDiagnostics) allocatedVariable(i int, x uint32) error {
	return d.emit("mk_constraint_bug", "mk_constraint", 557, func(out io.Writer) error {
		_, err := fmt.Fprintf(out, "xs[%d]: %d\n", i, x)
		return err
	})
}
func (d *fmConstraintDiagnostics) inputConstraint(c *FMConstraint) error {
	return d.emit("to_var_bug", "add_constraint", 1081, func(out io.Writer) error {
		if err := fmDisplayWrite(out, "add_constraint: "); err != nil {
			return err
		}
		if err := d.constraint(out, c); err != nil {
			return err
		}
		return fmDisplayWrite(out, "\n")
	})
}
func (d *fmConstraintDiagnostics) deferred(c *FMConstraint) error {
	return d.emit("add_constraint_bug", "register_constraint", 1117, func(out io.Writer) error {
		if err := fmDisplayWrite(out, "all variables are forbidden "); err != nil {
			return err
		}
		if err := d.constraint(out, c); err != nil {
			return err
		}
		return fmDisplayWrite(out, "\n")
	})
}
func (d *fmConstraintDiagnostics) copied(form ast.ASTHandle, dep ast.ASTDependency) error {
	return d.emit("fm_bug", "copy_remaining", 1530, func(out io.Writer) error {
		if err := fmDisplayWrite(out, "asserting...\n"); err != nil {
			return err
		}
		if err := writeFMExpression(d.manager, d.services, out, form); err != nil {
			return err
		}
		address, err := d.manager.DependencyAddressText(dep)
		if err != nil {
			return err
		}
		return fmDisplayWrite(out, "\nnew_dep: "+address+"\n")
	})
}
