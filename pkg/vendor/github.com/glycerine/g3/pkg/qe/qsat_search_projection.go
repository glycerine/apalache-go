package qe

// Source: src/qe/qsat.cpp qsat::get_core/add_assumption/project_qe/project.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

func (c *qsatSearchCall) conjunction(forms []ast.ASTRef) (ast.ASTHandle, error) {
	s := c.owner
	hs := make([]ast.ASTHandle, len(forms))
	for i, r := range forms {
		h, err := s.manager.RefGet(r)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		hs[i] = h
	}
	return c.hold(s.manager.MkAndArray(hs))
}
func (s *qsatSearch) assertBoth(h ast.ASTHandle) error {
	if err := s.exists.assertExpression(h); err != nil {
		return err
	}
	return s.forall.assertExpression(h)
}
func (s *qsatSearch) prepareKernels(c *qsatSearchCall, formula ast.ASTRef) error {
	h, err := s.manager.RefGet(formula)
	if err != nil {
		return err
	}
	defs := c.vector()
	level := EmptyMaxLevel()
	if err = s.preparation.abstraction.AbstractAtoms(h, &level, defs); err != nil {
		return err
	}
	if err = s.preparation.abstraction.MakeAbstract(h, formula); err != nil {
		return err
	}
	d, err := c.conjunction(*defs)
	if err != nil {
		return err
	}
	if err = s.assertBoth(d); err != nil {
		return err
	}
	h, err = s.manager.RefGet(formula)
	if err != nil {
		return err
	}
	if err = s.exists.assertExpression(h); err != nil {
		return err
	}
	negative, err := c.hold(s.manager.MkNot(h))
	if err != nil {
		return err
	}
	if err = s.forall.assertExpression(negative); err != nil {
		return err
	}
	return nil
}

// initialize is the retained top-level SAT/QE setup preceding check_sat.
// The goal/tactic caller keeps its original formula until the search finishes.
func (s *qsatSearch) initialize(formula ast.ASTRef) (err error) {
	if err = s.open(); err != nil {
		return err
	}
	if err = s.reset(); err != nil {
		return err
	}
	c, err := s.begin()
	if err != nil {
		return err
	}
	defer s.finish(c, &err)
	if err = s.preparation.abstraction.emit("operator()", 1247, func(w io.Writer) error {
		h, e := s.manager.RefGet(formula)
		if e != nil {
			return e
		}
		if e = writeExpression(s.manager, s.config, w, h); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	}); err != nil {
		return err
	}
	if s.mode != qsatSAT {
		h, e := s.manager.RefGet(formula)
		if e != nil {
			return e
		}
		h, err = c.hold(s.manager.PushNot(h))
		if err != nil {
			return err
		}
		if err = s.manager.SetRef(formula, h); err != nil {
			return err
		}
	}
	if err = s.preparation.hoist(s.mode, formula); err != nil {
		return err
	}
	if err = s.prepareKernels(c, formula); err != nil {
		return err
	}
	return s.preparation.abstraction.emit("operator()", 1258, func(w io.Writer) error {
		_, _ = io.WriteString(w, "ex: ")
		h, e := s.manager.RefGet(formula)
		if e != nil {
			return e
		}
		if e = writeExpression(s.manager, s.config, w, h); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	})
}
func (s *qsatSearch) getCore(core *[]ast.ASTRef, level uint32) error {
	if err := s.requireDefinitions("get_core"); err != nil {
		return err
	}
	if err := s.kernel(level).getCore(core); err != nil {
		return err
	}
	return s.preparation.abstraction.PredicatesToLiterals(core)
}
func (s *qsatSearch) negateCore(c *qsatSearchCall, core []ast.ASTRef) (ast.ASTHandle, error) {
	h, err := c.conjunction(core)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.hold(s.manager.PushNot(h))
}
func (s *qsatSearch) addAssumption(c *qsatSearchCall, formula ast.ASTHandle) error {
	r, err := c.ref(ast.ASTHandle{})
	if err != nil {
		return err
	}
	if err = s.preparation.abstraction.FreshBoolean("b", r); err != nil {
		return err
	}
	b, err := s.manager.RefGet(r)
	if err != nil {
		return err
	}
	if err = s.appendRef(&s.assumptions, b); err != nil {
		return err
	}
	eq, err := c.hold(s.manager.MkEq(b, formula))
	if err != nil {
		return err
	}
	if err = s.assertBoth(eq); err != nil {
		return err
	}
	if err = s.preparation.abstraction.AddPredicate(b, formula); err != nil {
		return err
	}
	return s.preparation.abstraction.SetExpressionLevel(b, EmptyMaxLevel())
}
func (s *qsatSearch) projectQE(c *qsatSearchCall, core *[]ast.ASTRef) error {
	if s.level != 1 {
		return ast.ErrASTDomain
	}
	md, err := s.current.Get()
	if err != nil {
		return err
	}
	if md == nil {
		return ast.ErrASTHandle
	}
	if err = s.getCore(core, s.level); err != nil {
		return err
	}
	if err = s.preparation.getVariables(s.level, &s.projectionVariables); err != nil {
		return err
	}
	if ast.ASTSourceDebug {
		valid, e := c.validateAssumptions(md, *core)
		if e != nil {
			return e
		}
		if !valid {
			return ast.ErrASTDomain
		}
	}
	if err = s.mbp.Project(s.mode != qsatRecursiveQE, &s.projectionVariables, md, core); err != nil {
		return err
	}
	if err = s.requireDefinitions("project_qe"); err != nil {
		return err
	}
	if s.mode == qsatMaximize {
		if err = s.maximizeCore(core); err != nil {
			return err
		}
	} else {
		fml, err := s.negateCore(c, *core)
		if err != nil {
			return err
		}
		if err = s.addAssumption(c, fml); err != nil {
			return err
		}
		if err = s.appendRef(&s.answers, fml); err != nil {
			return err
		}
		if err = s.appendRefs(&s.freeVariables, s.projectionVariables); err != nil {
			return err
		}
	}
	return s.pop(1)
}
func (s *qsatSearch) project(c *qsatSearchCall, core *[]ast.ASTRef) (bool, error) {
	if err := s.getCore(core, s.level); err != nil {
		return false, err
	}
	if err := s.preparation.abstraction.emit("project", 891, func(w io.Writer) error {
		if e := s.display(w); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "core\n")
		return s.preparation.abstraction.DisplayAssumptions(w, *core)
	}); err != nil {
		return false, err
	}
	if s.level < 2 {
		return false, ast.ErrASTDomain
	}
	md, err := s.current.Get()
	if err != nil {
		return false, err
	}
	if md == nil {
		return false, ast.ErrASTHandle
	}
	if err = s.preparation.getVariables(s.level-1, &s.projectionVariables); err != nil {
		return false, err
	}
	// Source validate_core/validate_project are true stubs with disabled bodies.
	if err = s.mbp.Project(s.mode != qsatRecursiveQE, &s.projectionVariables, md, core); err != nil {
		return false, err
	}
	if err = s.appendRefs(&s.freeVariables, s.projectionVariables); err != nil {
		return false, err
	}
	fml, err := s.negateCore(c, *core)
	if err != nil {
		return false, err
	}
	defs := c.vector()
	level := EmptyMaxLevel()
	if err = s.preparation.abstraction.AbstractAtoms(fml, &level, defs); err != nil {
		return false, err
	}
	d, err := c.conjunction(*defs)
	if err != nil {
		return false, err
	}
	if err = s.assertBoth(d); err != nil {
		return false, err
	}
	var scopes uint32
	if level.Max() == NoQuantifierLevel {
		scopes = 2 * (s.level / 2)
	} else if s.mode == qsatRecursiveQE {
		scopes = 2
	} else {
		if uint64(level.Max())+2 > uint64(s.level) {
			return false, nil
		}
		scopes = s.level - level.Max()
		if scopes%2 != 0 {
			scopes--
		}
	}
	if err = s.pop(scopes); err != nil {
		return false, err
	}
	if err = s.preparation.abstraction.emit("project", 925, func(w io.Writer) error {
		_, _ = fmt.Fprintf(w, "backtrack: %d new level: %d\nproject:\n", scopes, s.level)
		if e := writeExpressionVector(s.manager, s.config, w, *core); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n|->\n")
		if e := writeExpression(s.manager, s.config, w, fml); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	}); err != nil {
		return false, err
	}
	if s.level == 0 && s.mode != qsatSAT {
		err = s.addAssumption(c, fml)
	} else {
		r, e := c.ref(fml)
		if e != nil {
			return false, e
		}
		if err = s.preparation.abstraction.MakeAbstract(fml, r); err != nil {
			return false, err
		}
		h, e := s.manager.RefGet(r)
		if e != nil {
			return false, e
		}
		err = s.kernel(s.level).assertExpression(h)
	}
	return err == nil, err
}
