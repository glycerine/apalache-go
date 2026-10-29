package sat_solver

// Source: src/sat/sat_solver/inc_sat_solver.cpp:89-108; src/solver/solver.cpp.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

var _ solver.Solver = (*Solver)(nil)

type solverTranslation struct {
	translator *ast.ASTTranslation
	target     *Solver
}

func (s *Solver) drainTranslations() error {
	for len(s.translations) > 0 {
		i := len(s.translations) - 1
		w := s.translations[i]
		if w.translator != nil {
			if err := w.translator.Close(); err != nil {
				return err
			}
			w.translator = nil
		}
		if w.target != nil {
			if err := w.target.Close(); err != nil {
				return err
			}
			w.target = nil
		}
		s.translations = s.translations[:i]
	}
	return nil
}

// Translate copies original assertion and tracker roots, rebuilding all SAT
// state lazily. Independent parameter/numeral owners use TargetServices; the
// target services are borrowed and must outlive the returned solver.
func (s *Solver) Translate(m *ast.ASTManager, p *uparams.ParamsRef) (result solver.Solver, err error) {
	if err = s.open(); err != nil {
		return nil, err
	}
	if len(s.scopes) > 0 {
		return nil, z3exception.NewDefaultException("Cannot translate sat solver at non-base level")
	}
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	defer func() { s.retainFailure(err) }()
	services := s.services
	if check := services.Context.CheckServices(m.Symbols, m.NumeralService()); check != nil {
		if s.targetServices == nil {
			return nil, check
		}
		services, err = s.targetServices(m)
		if err != nil {
			return nil, err
		}
	}
	w := &solverTranslation{}
	s.translations = append(s.translations, w)
	w.target, err = NewIncrementalSolverWithConfig(m, p, IncrementalSolverConfig{Services: services, TargetServices: s.targetServices, Output: s.output})
	if err != nil {
		return nil, err
	}
	if m != s.manager {
		w.translator, err = ast.NewASTTranslation(s.manager, m, false)
		if err != nil {
			return nil, err
		}
	}
	for _, group := range []struct {
		from []ast.ASTHandle
		to   *[]ast.ASTHandle
	}{{s.formulas, &w.target.formulas}, {s.tracked, &w.target.tracked}} {
		for _, h := range group.from {
			translated := h
			if w.translator != nil {
				translated, err = w.translator.Translate(h)
				if err != nil {
					return nil, err
				}
			}
			if err = w.target.hold(group.to, translated); err != nil {
				return nil, err
			}
		}
	}
	if w.translator != nil {
		if err = w.translator.Close(); err != nil {
			return nil, err
		}
		w.translator = nil
	}
	result = w.target
	w.target = nil
	s.translations = s.translations[:len(s.translations)-1]
	return result, nil
}

func (s *Solver) PreferredSAT(_ []ast.ASTHandle) (lbool.LBool, [][]ast.ASTHandle, error) {
	return solver.DefaultPreferredSAT(s)
}

func (s *Solver) Display(out io.Writer, assumptions []ast.ASTHandle) error {
	return solver.DefaultDisplay(s, out, assumptions)
}
