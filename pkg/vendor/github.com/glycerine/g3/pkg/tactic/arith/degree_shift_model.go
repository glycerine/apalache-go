package arith

// Source: src/tactic/arith/degree_shift_tactic.cpp, prepare_substitution.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
)

func degreeShiftPublishModel(plan *degreeShiftPlan, services model.ModelEvaluatorConfig, out *tactic.TacticResult) (err error) {
	if plan == nil || plan.manager == nil || out == nil {
		return ast.ErrASTHandle
	}
	if len(plan.entries) == 0 {
		return nil
	}
	m := plan.manager
	filter, err := tactic.NewFilterModel(m)
	if err != nil {
		return err
	}
	if err = filter.IncRef(); err != nil {
		return errors.Join(err, filter.Close())
	}
	defer func() { err = errors.Join(err, filter.DecRef()) }()
	extension, err := tactic.NewExtensionModel(m, services)
	if err != nil {
		return err
	}
	if err = extension.IncRef(); err != nil {
		return errors.Join(err, extension.Close())
	}
	defer func() { err = errors.Join(err, extension.DecRef()) }()
	family, err := m.ArithmeticFamilyID()
	if err != nil {
		return err
	}
	for _, entry := range plan.entries {
		fresh, e := m.RefGet(entry.fresh)
		if e != nil {
			return e
		}
		freshDecl, e := m.GetDecl(fresh)
		if e != nil {
			return e
		}
		if e = filter.Insert(freshDecl); e != nil {
			return e
		}
		original, e := m.RefGet(entry.original)
		if e != nil {
			return e
		}
		originalDecl, e := m.GetDecl(original)
		if e != nil {
			return e
		}
		exponent, e := m.ArithmeticRealNumeralText("1/" + entry.degree.String())
		if e != nil {
			return e
		}
		exponentRef, e := m.NewRef(exponent)
		if e != nil {
			return e
		}
		definition, e := m.MkFamilyApp(family, 18, nil, []ast.ASTHandle{fresh, exponent})
		if e == nil {
			e = extension.Insert(originalDecl, definition)
		}
		e = errors.Join(e, m.DelRef(&exponentRef))
		if e != nil {
			return e
		}
	}
	combined, err := tactic.ConcatModels(m.Budget(), filter, extension)
	if err != nil {
		return err
	}
	if err = out.SetModelConverter(combined); err != nil {
		return errors.Join(err, combined.Close())
	}
	return nil
}
