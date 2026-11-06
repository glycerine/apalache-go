package smtcore

// Source: theory_datatype.cpp occurs_check and occurs_check_explain.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

type smtDatatypeOccursFrame struct {
	node *SMTEnode
	exit bool
}

// These marks belong to one final_check invocation, including all of its
// roots. Keeping them here provides the source final_check_st cleanup on
// every return without borrowing the context's enode mark bits.
type smtDatatypeOccursCheck struct {
	theory             *SMTDatatypeTheory
	onStack, cycleFree map[*SMTEnode]bool
	parent             map[*SMTEnode]*SMTEnode
	stack              []smtDatatypeOccursFrame
	equalities         []SMTEnodePair
}

func (c *smtDatatypeOccursCheck) push(node *SMTEnode) {
	c.stack = append(c.stack, smtDatatypeOccursFrame{node, true}, smtDatatypeOccursFrame{node, false})
}

func (c *smtDatatypeOccursCheck) explain(app, root *SMTEnode) error {
	if err := c.theory.emitDatatype("datatype", "occurs_check_explain", 461, func(out io.Writer) error {
		_, _ = io.WriteString(out, "occurs_check_explain ")
		if err := c.theory.displayDatatypeBounded(out, app.owner); err != nil {
			return err
		}
		_, _ = io.WriteString(out, " <-> ")
		if err := c.theory.displayDatatypeBounded(out, root.owner); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	}); err != nil {
		return err
	}
	constructor, err := c.theory.classConstructor(app)
	if err != nil {
		return err
	}
	if constructor == nil {
		return ast.ErrASTDomain
	}
	for _, arg := range constructor.arguments {
		if arg.root == root.root {
			if arg != root {
				c.equalities = append(c.equalities, SMTEnodePair{arg, root})
			}
			break
		}
	}
	for app.root != root.root {
		constructor, err = c.theory.classConstructor(app)
		if err != nil {
			return err
		}
		if constructor == nil {
			return ast.ErrASTDomain
		}
		if app != constructor {
			c.equalities = append(c.equalities, SMTEnodePair{app, constructor})
		}
		app = c.parent[app.root]
		if app == nil {
			return ast.ErrASTDomain
		}
	}
	// The source closing step can omit the constructor equality when the
	// datatype representative is a variable. Retain that necessary edge;
	// app=root alone does not explain app=C(...,root,...).
	constructor, err = c.theory.classConstructor(app)
	if err != nil {
		return err
	}
	if constructor == nil {
		return ast.ErrASTDomain
	}
	if app != constructor && root != constructor {
		c.equalities = append(c.equalities, SMTEnodePair{app, constructor})
	}
	if app != root {
		c.equalities = append(c.equalities, SMTEnodePair{app, root})
	}
	return nil
}

func (c *smtDatatypeOccursCheck) run(node *SMTEnode) (bool, error) {
	t := c.theory
	state, err := t.State()
	if err != nil {
		return false, err
	}
	if err = t.traceDatatypeOccursNode("occurs_check", 524, "occurs check: #", node, ""); err != nil {
		return false, err
	}
	t.statistics.occursChecks++
	c.push(node)
	for len(c.stack) > 0 {
		last := len(c.stack) - 1
		frame := c.stack[last]
		c.stack = c.stack[:last]
		app := frame.node
		if c.cycleFree[app.root] {
			continue
		}
		operation := " enter"
		if frame.exit {
			operation = " exit"
		}
		if err = t.traceDatatypeOccursNode("occurs_check", 538, "occurs check loop: #", app, operation); err != nil {
			return false, err
		}
		if frame.exit {
			c.cycleFree[app.root] = true
			continue
		}
		c.onStack[app.root] = true
		if app.root.TheoryVar(t.FamilyID()) == SMTNullTheoryVar {
			continue
		}
		constructor, err := t.classConstructor(app)
		if err != nil {
			return false, err
		}
		if state.Inconsistent() {
			return true, nil
		}
		if constructor == nil {
			continue
		}
		for _, arg := range constructor.arguments {
			if c.cycleFree[arg.root] {
				continue
			}
			if c.onStack[arg.root] {
				if err := c.explain(app, arg); err != nil {
					return false, err
				}
				if err = t.setDatatypeConflict(nil, c.equalities); err != nil {
					return true, err
				}
				return true, c.traceConflict()
			}
			sort, err := state.manager.GetSort(arg.owner)
			if err != nil {
				return false, err
			}
			family, err := state.manager.GetFamilyID(sort)
			if err != nil {
				return false, err
			}
			if family == t.FamilyID() {
				c.parent[arg.root] = app
				c.push(arg)
			}
		}
	}
	return false, nil
}
