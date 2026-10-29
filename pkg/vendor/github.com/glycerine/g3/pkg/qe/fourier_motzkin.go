package qe

// Source: src/qe/qe_lite.cpp fm::fm retained owning entry point.

import (
	"errors"
	"io"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/memory"
)

// FourierMotzkin eliminates selected de-Bruijn arithmetic variables from a
// caller-owned conjunction vector. Manager, selector and Config are borrowed.
type FourierMotzkin struct {
	self       *FourierMotzkin
	recognizer fmRecognizer
	store      *fmStore
	failures   []memory.ConstructionFailure
	bytes      uint64
	closed     bool
}

func NewFourierMotzkin(m *ast.ASTManager, selection *VariableTest, config Config) (*FourierMotzkin, error) {
	if m == nil || selection == nil || config.Context == nil || config.Globals == nil || config.Globals.Context != config.Context || config.Cooperation == nil {
		return nil, ast.ErrASTHandle
	}
	if err := selection.check(); err != nil {
		return nil, err
	}
	if selection.manager != m {
		return nil, ast.ErrASTDomain
	}
	if err := config.Context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	if _, err := config.Cooperation.Enabled(); err != nil {
		return nil, err
	}
	family, err := m.ArithmeticFamilyID()
	if err != nil {
		return nil, err
	}
	bytes := uint64(reflect.TypeOf(FourierMotzkin{}).Size())
	if err = m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	f := &FourierMotzkin{recognizer: fmRecognizer{manager: m, selection: selection, config: config, arithmetic: family, occ: true}, bytes: bytes}
	f.self = f
	return f, nil
}
func (f *FourierMotzkin) check() error {
	if f == nil || f.self != f || f.closed {
		return ast.ErrASTHandle
	}
	return nil
}
func (f *FourierMotzkin) SetVariableTest(v *VariableTest) error {
	if err := f.check(); err != nil {
		return err
	}
	if err := v.check(); err != nil {
		return err
	}
	if v.manager != f.recognizer.manager {
		return ast.ErrASTDomain
	}
	f.recognizer.selection = v
	return nil
}
func (f *FourierMotzkin) checkpoint() error {
	if err := f.recognizer.config.Cooperation.Cooperate("fm"); err != nil {
		return err
	}
	canceled, err := f.recognizer.manager.Canceled()
	if err != nil {
		return err
	}
	if canceled {
		return tactic.NewTacticException(f.recognizer.manager.ResourceLimit().CancelMessage())
	}
	return nil
}
func (f *FourierMotzkin) drain() error {
	for len(f.failures) != 0 {
		i := len(f.failures) - 1
		if err := f.failures[i].Cleanup(); err != nil {
			return err
		}
		f.failures = f.failures[:i]
	}
	return nil
}

type FourierMotzkinFailure struct {
	owner          *FourierMotzkin
	cause, cleanup error
}

var _ memory.ConstructionFailure = (*FourierMotzkinFailure)(nil)

func (e *FourierMotzkinFailure) Error() string        { return e.Unwrap().Error() }
func (e *FourierMotzkinFailure) Unwrap() error        { return errors.Join(e.cause, e.cleanup) }
func (e *FourierMotzkinFailure) OriginalCause() error { return e.cause }
func (e *FourierMotzkinFailure) CleanupCause() error  { return e.cleanup }
func (e *FourierMotzkinFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.recognizer.manager.NumeralService().RuntimeOwner()
}
func (e *FourierMotzkinFailure) Cleanup() error { return e.owner.drain() }

func (f *FourierMotzkin) finish(err *error) {
	var failure memory.ConstructionFailure
	if errors.As(*err, &failure) {
		f.failures = append(f.failures, failure)
	}
	if cleanup := f.drain(); cleanup != nil {
		*err = &FourierMotzkinFailure{owner: f, cause: *err, cleanup: cleanup}
	}
}

func (f *FourierMotzkin) Apply(forms *[]ast.ASTRef) (err error) {
	if err = f.check(); err != nil {
		return err
	}
	if forms == nil {
		return ast.ErrASTHandle
	}
	if err = f.recognizer.selection.check(); err != nil {
		return err
	}
	if err = f.drain(); err != nil {
		return err
	}
	defer f.finish(&err)
	if f.store != nil {
		if err = f.store.Close(); err != nil {
			return err
		}
		f.store = nil
	}
	// Source init resets all expression vectors. A fresh native run also
	// clears Boolean indices: C++ accidentally resets m_expr2var twice and
	// leaves m_expr2bvar pointing into the emptied Boolean vector.
	f.store, err = newFMStore(&f.recognizer)
	if err != nil {
		return err
	}
	s := f.store
	m := f.recognizer.manager
	var input []ast.ASTHandle
	for _, ref := range *forms {
		h, err := m.RefGet(ref)
		if err != nil {
			return err
		}
		input = append(input, h)
	}
	if err = f.recognizer.initForbidden(input); err != nil {
		return err
	}
	for _, h := range input {
		if s.inconsistent {
			break
		}
		occ, err := f.recognizer.clause(h)
		if err != nil {
			return err
		}
		if occ {
			c, err := s.parse(h)
			if err != nil {
				return err
			}
			if _, err = s.register(c); err != nil {
				return err
			}
		} else {
			ref, err := m.NewRef(h)
			if err != nil {
				return err
			}
			s.outputs = append(s.outputs, ref)
		}
	}
	// Source checks for quantified input after initialization, leaving both
	// the caller vector and retained run state intact on this refusal.
	for _, h := range input {
		quantified, err := m.HasQuantifiers(h)
		if err != nil {
			return err
		}
		if quantified {
			return nil
		}
	}
	if s.inconsistent {
		if err = s.setFalse(); err != nil {
			return err
		}
	} else {
		if err = f.recognizer.emit("operator()", 2262, func(out io.Writer) error { return s.display(out) }); err != nil {
			return err
		}
		if err = s.subsume(); err != nil {
			return err
		}
		candidates, err := s.candidates()
		if err != nil {
			return err
		}
		for _, x := range candidates {
			if err = f.checkpoint(); err != nil {
				return err
			}
			if s.counter > fmLimit {
				break
			}
			s.counter++
			if _, err = s.tryEliminate(x); err != nil {
				return err
			}
			if s.inconsistent {
				if err = s.setFalse(); err != nil {
					return err
				}
				break
			}
		}
		if !s.inconsistent {
			if err = s.copyRemaining(); err != nil {
				return err
			}
		}
	}
	// Delete constraint storage before replacing the caller's vector, as in
	// source reset_constraints followed by expr_ref_vector::reset/append.
	for _, c := range s.constraints {
		if err = s.deleteConstraint(c); err != nil {
			return err
		}
	}
	for len(*forms) != 0 {
		last := len(*forms) - 1
		if (*forms)[last] != (ast.ASTRef{}) {
			if err = m.DelRef(&(*forms)[last]); err != nil {
				return err
			}
		}
		*forms = (*forms)[:last]
	}
	for _, ref := range s.outputs {
		h, err := m.RefGet(ref)
		if err != nil {
			return err
		}
		r, err := m.NewRef(h)
		if err != nil {
			return err
		}
		*forms = append(*forms, r)
	}
	return nil
}

func (s *fmStore) setFalse() error {
	m := s.recognizer.manager
	for len(s.outputs) != 0 {
		i := len(s.outputs) - 1
		if s.outputs[i] != (ast.ASTRef{}) {
			if err := m.DelRef(&s.outputs[i]); err != nil {
				return err
			}
		}
		s.outputs = s.outputs[:i]
	}
	h, err := m.MkFalse()
	if err != nil {
		return err
	}
	ref, err := m.NewRef(h)
	if err != nil {
		return err
	}
	s.outputs = append(s.outputs, ref)
	return nil
}

func (f *FourierMotzkin) Close() error {
	if f == nil || f.self != f {
		return ast.ErrASTHandle
	}
	if f.closed {
		return nil
	}
	if err := f.drain(); err != nil {
		return err
	}
	if f.store != nil {
		if err := f.store.Close(); err != nil {
			return err
		}
		f.store = nil
	}
	f.recognizer.manager.Budget().Release(f.bytes)
	f.bytes = 0
	f.closed = true
	return nil
}
