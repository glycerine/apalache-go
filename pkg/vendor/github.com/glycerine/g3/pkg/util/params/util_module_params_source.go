package params

// Source: scripts/mk_genfile_common.py:895-939; src/util/gparams.cpp:598-605;
// src/util/params.cpp:325-339,639-648,902-928; src/smt/params/smt_params_helper.pyg:59-66.

import (
	"errors"
	"math"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// This concrete owner is embedded in the actual parameter facade header. No
// second allocation/charge is made for it. A frame represents only the reached
// lexical helper: borrowed local reference and one acquired module reference.
type ModuleParamsOwner struct {
	Self            *ModuleParamsOwner
	Context         *ParamsContext
	Globals         *GlobalParamsService
	Budget          *arena.Budget
	Helpers         *ModuleParamsFrame
	Serial          uint64
	Running         bool
	DestructorCause error
}
type ModuleParamsFrame struct {
	Self                                     *ModuleParamsFrame
	Owner                                    *ModuleParamsOwner
	Next                                     *ModuleParamsFrame
	Local                                    *ParamsRef
	Value                                    ParamsRef
	Receipt                                  ParamsDecrementReceipt
	Header, Invocation                       uint64
	Acquired, DestructorStarted, SourceDone  bool
	BodyCause, DestructorCause, CleanupCause error
}

func (m *ModuleParamsOwner) Bind(c *ParamsContext, g *GlobalParamsService) error {
	if m == nil || m.Self != m || m.Running || m.Helpers != nil || m.DestructorCause != nil {
		return ErrParamsPrecondition
	}
	if c == nil || c.Self != c || g == nil || g.self != g || g.Context != c || g.Config.Budget != m.Budget {
		return ErrParamsOwner
	}
	// Borrow only configuration identities. Actual symbol/global/local lifetimes
	// are read by the reached generated constructor and getter engines.
	m.Context = c
	m.Globals = g
	return nil
}
func (m *ModuleParamsOwner) BeginHelper(p *ParamsRef, module string) (*ModuleParamsFrame, error) {
	if m == nil || m.Self != m || !m.Running || m.DestructorCause != nil {
		return nil, ErrParamsPrecondition
	}
	if m.Serial == math.MaxUint64 {
		return nil, arena.ErrBounds
	}
	n := uint64(reflect.TypeOf(ModuleParamsFrame{}).Size())
	if e := m.Budget.Reserve(n); e != nil {
		return nil, e
	}
	m.Serial++
	f := &ModuleParamsFrame{Owner: m, Local: p, Header: n, Invocation: m.Serial, Next: m.Helpers}
	f.Self = f
	m.Helpers = f
	var e error
	f.Value, e = m.Globals.SourceGetModuleName(module)
	if e == nil {
		f.Acquired = true
	} // the actual returned reference is the sole authority.
	return f, e
}
func (m *ModuleParamsOwner) ExecutingFrame(f *ModuleParamsFrame) error {
	if m == nil || m.Self != m || !m.Running || f == nil || f.Self != f || f.Owner != m || f.Invocation == 0 || f.Invocation > m.Serial || f.Header == 0 {
		return ErrParamsPrecondition
	}
	// Concrete active declaration links identify this exact retained frame. Older
	// live helpers stay readable while later source declarations are alive.
	for live := m.Helpers; live != nil; live = live.Next {
		if live == f {
			return nil
		}
	}
	return ErrParamsPrecondition
}
func (m *ModuleParamsOwner) FrameBool(f *ModuleParamsFrame, key string, def bool) (bool, error) {
	if e := m.ExecutingFrame(f); e != nil {
		return false, e
	}
	if !f.Acquired || f.DestructorStarted {
		return false, ErrParamsPrecondition
	}
	return m.Context.GetBoolFallbackName(f.Local, &key, &f.Value, def)
}
func (m *ModuleParamsOwner) FrameUInt(f *ModuleParamsFrame, key string, def uint32) (uint32, error) {
	if e := m.ExecutingFrame(f); e != nil {
		return 0, e
	}
	if !f.Acquired || f.DestructorStarted {
		return 0, ErrParamsPrecondition
	}
	return m.Context.GetUIntFallbackName(f.Local, &key, &f.Value, def)
}
func (m *ModuleParamsOwner) FrameDouble(f *ModuleParamsFrame, key string, def float64) (float64, error) {
	if e := m.ExecutingFrame(f); e != nil {
		return 0, e
	}
	if !f.Acquired || f.DestructorStarted {
		return 0, ErrParamsPrecondition
	}
	return m.Context.GetDoubleFallbackName(f.Local, &key, &f.Value, def)
}
func (m *ModuleParamsOwner) FinishHelper(f *ModuleParamsFrame, body error) error {
	if e := m.ExecutingFrame(f); e != nil {
		return errors.Join(body, e)
	}
	if f.BodyCause == nil {
		f.BodyCause = body
	}
	if m.DestructorCause != nil {
		return errors.Join(body, m.DestructorCause)
	}
	if m.Helpers != f {
		return errors.Join(body, ErrParamsPrecondition)
	}
	if f.Acquired && !f.SourceDone {
		f.DestructorStarted = true
		if e := m.Context.DelTracked(&f.Value, &f.Receipt); e != nil {
			f.DestructorCause = e
			m.DestructorCause = e
			return errors.Join(body, e)
		}
		f.SourceDone = true
	}
	m.Helpers = f.Next
	m.Budget.Release(f.Header)
	f.Header = 0
	return body
}
func (m *ModuleParamsOwner) CloseHelpers() error {
	if m == nil || m.Self != m || m.Running {
		return ErrParamsPrecondition
	}
	m.Running = true
	defer func() { m.Running = false }()
	for m.Helpers != nil {
		f := m.Helpers
		if e := m.ExecutingFrame(f); e != nil {
			return e
		}
		if f.Acquired && !f.SourceDone {
			var e error
			if f.DestructorStarted {
				if f.Receipt.InvocationField == 0 {
					return f.DestructorCause
				}
				e = m.Context.CompleteDecrement(&f.Value, f.Receipt)
			} else {
				f.DestructorStarted = true
				e = m.Context.DelTracked(&f.Value, &f.Receipt)
			}
			if e != nil {
				if f.CleanupCause == nil {
					f.CleanupCause = e
				}
				return e
			}
			f.SourceDone = true
		}
		m.Helpers = f.Next
		m.Budget.Release(f.Header)
		f.Header = 0
	}
	return nil
}

func (m *ModuleParamsOwner) FrameStringView(f *ModuleParamsFrame, key string, def string) (ParamsStringView, error) {
	if e := m.ExecutingFrame(f); e != nil {
		return ParamsStringView{}, e
	}
	if !f.Acquired || f.DestructorStarted {
		return ParamsStringView{}, ErrParamsPrecondition
	}
	return m.Context.GetStringViewFallbackName(f.Local, &key, &f.Value, &def)
}

// Source: scripts/mk_genfile_common.py:842-851,895-921.
// The symbol default is a real constructor expression before the Params getter.
func (m *ModuleParamsOwner) FrameSymbol(f *ModuleParamsFrame, key, def string) (usymbol.Symbol, error) {
	if e := m.ExecutingFrame(f); e != nil {
		return usymbol.Symbol{}, e
	}
	if !f.Acquired || f.DestructorStarted {
		return usymbol.Symbol{}, ErrParamsPrecondition
	}
	d, e := m.Context.SymbolsField.Intern(def)
	if e != nil {
		return usymbol.Symbol{}, e
	}
	return m.Context.GetSymbolFallbackName(f.Local, &key, &f.Value, d)
}
