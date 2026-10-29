package qe

// Source: src/qe/qsat.cpp qsat::maximize_model/maximize_core.

import (
	"io"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func copyQSATOptimum(dst *simplex.Optimum, src *simplex.Optimum) {
	dst.Infinite = src.Infinite
	dst.Value.Set(&src.Value)
	dst.Epsilon.Set(&src.Epsilon)
}
func qsatOptimumString(v *simplex.Optimum) string {
	s := v.Value.RatString()
	if v.Epsilon.Sign() != 0 {
		sign := " +e*"
		if v.Epsilon.Sign() < 0 {
			sign = " -e*"
		}
		s = "(" + s + sign + new(big.Rat).Abs(&v.Epsilon).RatString() + ")"
	}
	if v.Infinite {
		if v.Value.Sign() == 0 && v.Epsilon.Sign() == 0 {
			return "oo"
		}
		return "(oo + " + s + ")"
	}
	return s
}
func (s *qsatSearch) maximizeCore(core *[]ast.ASTRef) error {
	if s.value == nil || s.objective.IsNull() {
		return ast.ErrASTHandle
	}
	if err := s.preparation.abstraction.emit("maximize_core", 1363, func(w io.Writer) error {
		_, _ = io.WriteString(w, "maximize: ")
		if e := writeExpressionVector(s.manager, s.config, w, *core); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	}); err != nil {
		return err
	}
	s.wasSat = s.wasSat || len(*core) > 0
	copyQSATOptimum(s.value, &s.valueSaved)
	if err := s.verbose("(maximize " + qsatOptimumString(s.value) + ")\n"); err != nil {
		return err
	}
	h, err := s.manager.RefGet(s.improvement)
	if err != nil {
		return err
	}
	return s.assertBoth(h)
}
func (s *qsatSearch) maximizeModel(c *qsatSearchCall) error {
	if s.level != 1 || s.mode != qsatMaximize || s.objective.IsNull() {
		return ast.ErrASTDomain
	}
	md, err := s.current.Get()
	if err != nil {
		return err
	}
	if md == nil {
		return ast.ErrASTHandle
	}
	ge, err := c.ref(ast.ASTHandle{})
	if err != nil {
		return err
	}
	asms, defs := c.vector(), c.vector()
	if err = s.preparation.abstraction.GetAssumptions(md, asms); err != nil {
		return err
	}
	if err = s.preparation.abstraction.PredicatesToLiterals(asms); err != nil {
		return err
	}
	if err = s.requireDefinitions("maximize_model1"); err != nil {
		return err
	}
	v, err := s.mbp.Maximize(md, *asms, s.objective, ge, s.improvement)
	if err != nil {
		return err
	}
	copyQSATOptimum(&s.valueSaved, &v)
	if err = s.requireDefinitions("maximize_model2"); err != nil {
		return err
	}
	if err = s.verbose("(qsat-maximize-bound: " + qsatOptimumString(&s.valueSaved) + ")\n"); err != nil {
		return err
	}
	h, err := s.manager.RefGet(ge)
	if err != nil {
		return err
	}
	level := EmptyMaxLevel()
	if err = s.preparation.abstraction.AbstractAtoms(h, &level, defs); err != nil {
		return err
	}
	d, err := c.conjunction(*defs)
	if err != nil {
		return err
	}
	if err = s.assertBoth(d); err != nil {
		return err
	}
	if err = s.preparation.abstraction.MakeAbstract(h, ge); err != nil {
		return err
	}
	h, err = s.manager.RefGet(ge)
	if err != nil {
		return err
	}
	n, err := s.manager.GetNumArgs(h)
	if err != nil {
		return err
	}
	fid, err := s.manager.GetFamilyID(h)
	if err != nil {
		return err
	}
	isConstant := n == 0 && fid == ast.NullFamilyID
	if ast.ASTSourceDebug && !isConstant {
		return ast.ErrASTDomain
	}
	if isConstant {
		decl, err := s.manager.GetDecl(h)
		if err != nil {
			return err
		}
		truth, err := c.hold(s.manager.MkTrue())
		if err != nil {
			return err
		}
		if err = md.RegisterConstant(decl, truth); err != nil {
			return err
		}
	}
	return s.requireDefinitions("maximize_model3")
}

// maximize follows the retained qsat optimization entry; qmax/factory and
// command/API integration remain separate consumers.
func (s *qsatSearch) maximize(forms []ast.ASTRef, objective ast.ASTHandle, md *tactic.ModelRef, value *simplex.Optimum) (status lbool.LBool, err error) {
	c, err := s.begin()
	if err != nil {
		return lbool.Undef, err
	}
	defer s.finish(c, &err)
	if s.mode != qsatMaximize || md == nil || value == nil {
		return lbool.Undef, ast.ErrASTHandle
	}
	fml, err := c.conjunction(forms)
	if err != nil {
		return lbool.Undef, err
	}
	r, err := c.ref(fml)
	if err != nil {
		return lbool.Undef, err
	}
	if err = s.preparation.hoist(s.mode, r); err != nil {
		return lbool.Undef, err
	}
	s.objective = objective
	s.value = value
	s.wasSat = false
	if err = s.saved.Set(nil); err != nil {
		return lbool.Undef, err
	}
	if err = s.prepareKernels(c, r); err != nil {
		return lbool.Undef, err
	}
	status, err = s.checkLoop()
	if err != nil {
		return lbool.Undef, err
	}
	if err = md.SetRef(&s.current); err != nil {
		return lbool.Undef, err
	}
	switch status {
	case lbool.False:
		if !s.wasSat {
			return lbool.False, nil
		}
		if err = md.SetRef(&s.saved); err != nil {
			return lbool.Undef, err
		}
	case lbool.True:
		return lbool.Undef, ast.ErrASTDomain
	case lbool.Undef:
		ex, e := s.exists.solver()
		if e != nil {
			return lbool.Undef, e
		}
		reason, e := ex.ReasonUnknown()
		if e != nil {
			return lbool.Undef, e
		}
		if reason == "ok" {
			fa, e := s.forall.solver()
			if e != nil {
				return lbool.Undef, e
			}
			reason, e = fa.ReasonUnknown()
			if e != nil {
				return lbool.Undef, e
			}
		}
		return lbool.Undef, tactic.NewTacticException(reason)
	}
	return lbool.True, nil
}
