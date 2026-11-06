package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type implicantLimit struct{ assumptions, namings int }

// ImplicantSolver owns assertion bookkeeping and store names over a borrowed
// native solver and RPFP. Extensionality is the source solver wrapper property.
// Source: duality_rpfp.cpp:2883-2954.
type ImplicantSolver struct {
	owner                *RPFP
	solver               solver.Solver
	extensional          bool
	stores               *StoreExtraction
	assumptions, namings []*Expr
	limits               []implicantLimit
	lastModel            *model.ModelCore
	closed               bool
}

func NewImplicantSolver(owner *RPFP, native solver.Solver, extensional bool) (*ImplicantSolver, error) {
	if err := owner.open(); err != nil {
		return nil, err
	}
	if native == nil {
		return nil, ErrWrapperOwner
	}
	s := &ImplicantSolver{owner: owner, solver: native, extensional: extensional}
	if !extensional {
		var err error
		s.stores, err = NewStoreExtraction(owner.context)
		if err != nil {
			return nil, err
		}
	}
	return s, nil
}
func (s *ImplicantSolver) open() error {
	if s == nil || s.closed {
		return ErrWrapperOwner
	}
	return s.owner.open()
}
func (s *ImplicantSolver) Add(formula *Expr) (result error) {
	if err := s.open(); err != nil {
		return err
	}
	if err := formula.open(); err != nil {
		return err
	}
	if formula.value.object.context != s.owner.context {
		return ErrContextMismatch
	}
	var term *Expr
	var err error
	if s.stores != nil {
		previous := len(s.stores.constraints)
		term, err = s.stores.Extract(formula)
		if err != nil {
			return err
		}
		defer func() {
			if term != nil {
				result = errors.Join(result, term.Close())
			}
		}()
		for _, constraint := range s.stores.constraints[previous:] {
			name, err := constraint.Clone()
			if err != nil {
				return err
			}
			s.namings = append(s.namings, name)
			h, err := name.Raw()
			if err != nil {
				return err
			}
			if err = s.solver.Assert(h); err != nil {
				return err
			}
		}
	} else {
		term, err = formula.Clone()
		if err != nil {
			return err
		}
		defer func() {
			if term != nil {
				result = errors.Join(result, term.Close())
			}
		}()
	}
	s.assumptions = append(s.assumptions, term)
	term = nil
	h, err := s.assumptions[len(s.assumptions)-1].Raw()
	if err != nil {
		return err
	}
	return s.solver.Assert(h)
}
func (s *ImplicantSolver) Push() error {
	if err := s.open(); err != nil {
		return err
	}
	if err := s.solver.Push(); err != nil {
		return err
	}
	s.limits = append(s.limits, implicantLimit{len(s.assumptions), len(s.namings)})
	return nil
}

// Pop implements the live source pop(1) specialization. Definitions introduced
// in the popped scope are asserted again in its parent before local truncation.
func (s *ImplicantSolver) Pop() error {
	if err := s.open(); err != nil {
		return err
	}
	if len(s.limits) == 0 {
		return ErrWrapperOwner
	}
	limit := s.limits[len(s.limits)-1]
	if err := s.solver.Pop(1); err != nil {
		return err
	}
	for _, name := range s.namings[limit.namings:] {
		h, err := name.Raw()
		if err != nil {
			return err
		}
		if err = s.solver.Assert(h); err != nil {
			return err
		}
	}
	for len(s.assumptions) > limit.assumptions {
		n := len(s.assumptions) - 1
		if err := s.assumptions[n].Close(); err != nil {
			return err
		}
		s.assumptions = s.assumptions[:n]
	}
	for len(s.namings) > limit.namings {
		n := len(s.namings) - 1
		if err := s.namings[n].Close(); err != nil {
			return err
		}
		s.namings = s.namings[:n]
	}
	s.limits = s.limits[:len(s.limits)-1]
	return nil
}

// The source solver wrapper keeps its last checked model across push/pop.
func (s *ImplicantSolver) clearModel() error {
	if s.lastModel == nil {
		return nil
	}
	consumed, err := s.lastModel.DecRefConsumed()
	if consumed {
		s.lastModel = nil
	}
	return err
}
func (s *ImplicantSolver) Check() (lbool.LBool, error) {
	if err := s.open(); err != nil {
		return lbool.Undef, err
	}
	if err := s.clearModel(); err != nil {
		return lbool.Undef, err
	}
	status, err := s.solver.CheckSat(nil)
	if err != nil {
		return status, err
	}
	s.lastModel, err = s.solver.AcquireModel()
	return status, err
}
func (s *ImplicantSolver) AcquireModel() (*model.ModelCore, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	if s.lastModel == nil {
		return nil, nil
	}
	if err := s.lastModel.IncRef(); err != nil {
		return nil, err
	}
	return s.lastModel, nil
}
func (s *ImplicantSolver) GetImplicant() (out *Expr, result error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	md, err := s.AcquireModel()
	if err != nil {
		return nil, err
	}
	if err = s.owner.replaceDualModel(md, false); err != nil {
		return nil, err
	}
	m := s.owner.context.manager
	handles := make([]ast.ASTHandle, len(s.assumptions))
	for i, term := range s.assumptions {
		handles[i], err = term.Raw()
		if err != nil {
			return nil, err
		}
	}
	h, err := m.MkAndArgs(handles)
	if err != nil {
		return nil, err
	}
	formula, err := NewExpr(s.owner.context, h)
	if err != nil {
		return nil, err
	}
	defer func() {
		result = errors.Join(result, formula.Close())
		if result != nil && out != nil {
			result = errors.Join(result, out.Close())
			out = nil
		}
	}()
	cube, err := s.owner.UnderapproxFullFormula(formula, s.extensional)
	if err != nil {
		return nil, err
	}
	if s.stores == nil {
		return cube, nil
	}
	defer func() { result = errors.Join(result, cube.Close()) }()
	return s.stores.Undo(cube)
}
func (s *ImplicantSolver) AddEdge(edge *Edge) (result error) {
	if err := s.open(); err != nil {
		return err
	}
	if edge == nil || edge.closed || edge.Owner != s.owner {
		return ErrWrapperOwner
	}
	refs := append([]ast.ASTRef{edge.dual}, edge.constraints...)
	for _, ref := range refs {
		if ref == (ast.ASTRef{}) {
			continue
		}
		h, err := s.owner.context.manager.RefGet(ref)
		if err != nil {
			return err
		}
		term, err := NewExpr(s.owner.context, h)
		if err != nil {
			return err
		}
		addErr := s.Add(term)
		closeErr := term.Close()
		if err = errors.Join(addErr, closeErr); err != nil {
			return err
		}
	}
	return nil
}
func (s *ImplicantSolver) Close() error {
	if s == nil || s.closed {
		return nil
	}
	if err := s.clearModel(); err != nil {
		return err
	}
	for len(s.assumptions) > 0 {
		n := len(s.assumptions) - 1
		if err := s.assumptions[n].Close(); err != nil {
			return err
		}
		s.assumptions = s.assumptions[:n]
	}
	for len(s.namings) > 0 {
		n := len(s.namings) - 1
		if err := s.namings[n].Close(); err != nil {
			return err
		}
		s.namings = s.namings[:n]
	}
	if s.stores != nil {
		if err := s.stores.Close(); err != nil {
			return err
		}
	}
	s.limits = nil
	s.closed = true
	return nil
}
