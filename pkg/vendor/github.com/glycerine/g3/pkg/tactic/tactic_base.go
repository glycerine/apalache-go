package tactic

// Source: src/tactic/tactic.h; src/tactic/tactic.cpp.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// ProofConverter is the retained proof-converter ownership and application
// boundary. Concrete proof converters can be added without weakening Tactic's
// typed result contract.
type ProofConverter interface {
	IncRef() error
	DecRef() error
	Convert(*ast.ASTManager, []ast.ASTHandle, ast.ASTRef) error
	Translate(*ast.ASTTranslation) (ProofConverter, error)
	Cancel() error
	Display(io.Writer) error
	Close() error
}

// Tactic is the source tactic virtual surface used by execution and tacticals.
type Tactic interface {
	IncRef() error
	DecRef() error
	Apply(*Goal, *TacticResult) error
	UpdateParams(*uparams.ParamsRef) error
	CollectParamDescrs(*uparams.ParamDescrs) error
	CollectStatistics() map[string]float64
	ResetStatistics() error
	Cleanup() error
	Reset() error
	SetLogic(usymbol.Symbol) error
	SetProgressCallback(func(uint32)) error
	Translate(*ast.ASTManager) (Tactic, error)
}

// TacticFactory is the typed Go replacement for tactic_factory. Function
// adapters replace the C++ factory-declaration macros.
type TacticFactory interface {
	New(*ast.ASTManager, *uparams.ParamsRef) (Tactic, error)
	Close() error
}

type TacticFactoryFunc func(*ast.ASTManager, *uparams.ParamsRef) (Tactic, error)

func (f TacticFactoryFunc) New(m *ast.ASTManager, p *uparams.ParamsRef) (Tactic, error) {
	if f == nil || m == nil {
		return nil, ast.ErrASTHandle
	}
	return f(m, p)
}
func (TacticFactoryFunc) Close() error { return nil }

// TacticResult owns the subgoal, converter and dependency references returned
// by one tactic application.
type TacticResult struct {
	manager        *ast.ASTManager
	Goals          []*Goal
	goalOwned      []bool
	ModelConverter ModelConverter
	ProofConverter ProofConverter
	Core           ast.ASTDependency
	modelOwned     bool
	proofOwned     bool
	coreOwned      bool
	modelCleanup   bool
	closed         bool
}

func (r *TacticResult) prepare(m *ast.ASTManager) error {
	if r == nil {
		return ast.ErrASTHandle
	}
	if r.closed {
		*r = TacticResult{}
	}
	if r.manager != nil && r.manager != m {
		return ast.ErrASTHandle
	}
	r.manager = m
	return r.Reset()
}

// Prepare resets a result for a tactic implemented in another package. It is
// the package boundary counterpart of tactic's result-buffer initialization.
func (r *TacticResult) Prepare(m *ast.ASTManager) error { return r.prepare(m) }

// BindManager preserves existing buffers for source tactics that append goals
// rather than resetting them. Prepare remains the resetting entry point.
func (r *TacticResult) BindManager(m *ast.ASTManager) error {
	if r == nil || r.closed || m == nil || (r.manager != nil && r.manager != m) {
		return ast.ErrASTHandle
	}
	r.manager = m
	return nil
}
func (r *TacticResult) AddGoal(g *Goal) error {
	if r == nil || r.closed || g == nil || g.manager != r.manager {
		return ast.ErrASTHandle
	}
	r.Goals = append(r.Goals, g)
	r.goalOwned = append(r.goalOwned, false)
	return nil
}

func (r *TacticResult) moveGoalsFrom(source *TacticResult) error {
	if r == nil || source == nil || source.manager == nil {
		return ast.ErrASTHandle
	}
	if err := r.prepare(source.manager); err != nil {
		return err
	}
	r.Goals = append(r.Goals, source.Goals...)
	r.goalOwned = append(r.goalOwned, source.goalOwned...)
	for i := range source.Goals {
		source.Goals[i] = nil
		source.goalOwned[i] = false
	}
	source.Goals = source.Goals[:0]
	source.goalOwned = source.goalOwned[:0]
	return nil
}

func (r *TacticResult) appendMovedGoalsFrom(source *TacticResult) error {
	if r == nil || source == nil || source.manager == nil || r.closed {
		return ast.ErrASTHandle
	}
	if r.manager == nil {
		r.manager = source.manager
	}
	if r.manager != source.manager {
		return ast.ErrASTHandle
	}
	r.Goals = append(r.Goals, source.Goals...)
	r.goalOwned = append(r.goalOwned, source.goalOwned...)
	for i := range source.Goals {
		source.Goals[i] = nil
		source.goalOwned[i] = false
	}
	source.Goals = source.Goals[:0]
	source.goalOwned = source.goalOwned[:0]
	return nil
}

func (r *TacticResult) transferBorrowedGoalOwnershipTo(target *TacticResult) {
	if r == nil || target == nil {
		return
	}
	for i, goal := range r.Goals {
		if !r.goalOwned[i] {
			continue
		}
		for j, candidate := range target.Goals {
			if candidate == goal && !target.goalOwned[j] {
				r.goalOwned[i] = false
				target.goalOwned[j] = true
				break
			}
		}
	}
}

// TakeGoal transfers a directly owned goal to the result. AddGoal records a
// borrowed goal, which is the correct operation for skip and similar tactics.
func (r *TacticResult) TakeGoal(g *Goal) error {
	if err := r.AddGoal(g); err != nil {
		return err
	}
	r.goalOwned[len(r.goalOwned)-1] = true
	return nil
}
func (r *TacticResult) SetModelConverter(c ModelConverter) error {
	if r == nil || r.closed {
		return ErrModelConverterOwner
	}
	if c != nil {
		if err := c.IncRef(); err != nil {
			return err
		}
	}
	if r.ModelConverter != nil && r.modelOwned {
		consumed, err := r.ModelConverter.DecRefConsumed()
		if consumed {
			r.modelOwned = false
			r.modelCleanup = err != nil
		}
		if err != nil || !consumed {
			if c != nil {
				_ = c.DecRef()
			}
			if err != nil {
				return err
			}
			return ErrModelConverterOwner
		}
	}
	r.ModelConverter, r.modelOwned, r.modelCleanup = c, c != nil, false
	return nil
}
func (r *TacticResult) SetProofConverter(c ProofConverter) error {
	if r == nil || r.closed {
		return ast.ErrASTHandle
	}
	if c != nil {
		if err := c.IncRef(); err != nil {
			return err
		}
	}
	if r.ProofConverter != nil && r.proofOwned {
		if err := r.ProofConverter.DecRef(); err != nil {
			if c != nil {
				_ = c.DecRef()
			}
			return err
		}
	}
	r.ProofConverter, r.proofOwned = c, c != nil
	return nil
}
func (r *TacticResult) SetCore(d ast.ASTDependency) error {
	if r == nil || r.closed || r.manager == nil {
		return ast.ErrASTHandle
	}
	if err := r.manager.DependencyInc(d); err != nil {
		return err
	}
	if r.coreOwned {
		if err := r.manager.DependencyDec(r.Core); err != nil {
			_ = r.manager.DependencyDec(d)
			return err
		}
	}
	r.Core, r.coreOwned = d, !d.IsNull()
	return nil
}
func (r *TacticResult) Reset() (err error) {
	if r == nil {
		return ast.ErrASTHandle
	}
	for len(r.Goals) != 0 {
		i := len(r.Goals) - 1
		if r.goalOwned[i] {
			if e := r.Goals[i].Close(); e != nil {
				return errors.Join(err, e)
			}
		}
		r.Goals[i] = nil
		r.Goals = r.Goals[:i]
		r.goalOwned = r.goalOwned[:i]
	}
	if r.ModelConverter != nil {
		if r.modelCleanup {
			err = r.ModelConverter.Close()
		} else if r.modelOwned {
			var consumed bool
			consumed, err = r.ModelConverter.DecRefConsumed()
			if consumed {
				r.modelOwned = false
				r.modelCleanup = err != nil
			}
			if err == nil && !consumed {
				err = ErrModelConverterOwner
			}
		}
		if err != nil {
			return err
		}
		r.ModelConverter, r.modelCleanup = nil, false
	}
	if r.ProofConverter != nil && r.proofOwned {
		if err = r.ProofConverter.DecRef(); err != nil {
			return err
		}
		r.ProofConverter, r.proofOwned = nil, false
	}
	if r.coreOwned {
		if err = r.manager.DependencyDec(r.Core); err != nil {
			return err
		}
		r.Core, r.coreOwned = ast.ASTDependency{}, false
	}
	return nil
}
func (r *TacticResult) Close() error {
	if r == nil || r.closed {
		return nil
	}
	if err := r.Reset(); err != nil {
		return err
	}
	r.manager = nil
	r.closed = true
	return nil
}
func (r *TacticResult) DecidedSat() bool {
	return len(r.Goals) == 1 && r.Goals[0].IsDecidedSat()
}
func (r *TacticResult) DecidedUnsat() bool {
	return len(r.Goals) == 1 && r.Goals[0].IsDecidedUnsat()
}

type primitiveTacticKind uint8

const (
	primitiveSkip primitiveTacticKind = iota
	primitiveFail
	primitiveFailUndecided
	primitiveReport
	primitiveTrace
)

type primitiveTactic struct {
	self       *primitiveTactic
	kind       primitiveTacticKind
	message    string
	level      uint32
	references uint32
	closed     bool
}

func newPrimitiveTactic(kind primitiveTacticKind, message string, level uint32) *primitiveTactic {
	t := &primitiveTactic{kind: kind, message: message, level: level}
	t.self = t
	return t
}
func NewSkipTactic() Tactic { return newPrimitiveTactic(primitiveSkip, "", 0) }
func NewFailTactic() Tactic { return newPrimitiveTactic(primitiveFail, "fail tactic", 0) }
func NewFailIfUndecidedTactic() Tactic {
	return newPrimitiveTactic(primitiveFailUndecided, "undecided", 0)
}
func NewReportVerboseTactic(s string, l uint32) Tactic {
	return newPrimitiveTactic(primitiveReport, s, l)
}
func NewTraceTactic(tag string) Tactic { return newPrimitiveTactic(primitiveTrace, tag, 0) }

func (t *primitiveTactic) open() error {
	if t == nil || t.self != t || t.closed {
		return arena.ErrClosed
	}
	return nil
}
func (t *primitiveTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.references++
	return nil
}
func (t *primitiveTactic) DecRef() error {
	if err := t.open(); err != nil {
		return err
	}
	if t.references == 0 {
		return arena.ErrBounds
	}
	t.references--
	if t.references == 0 {
		t.closed = true
	}
	return nil
}
func (t *primitiveTactic) Apply(in *Goal, result *TacticResult) error {
	if err := t.open(); err != nil {
		return err
	}
	if in == nil {
		return ast.ErrASTHandle
	}
	if err := in.open(); err != nil {
		return err
	}
	if err := result.prepare(in.manager); err != nil {
		return err
	}
	switch t.kind {
	case primitiveFail:
		return NewTacticException(t.message)
	case primitiveFailUndecided:
		if !in.IsDecided() {
			return NewTacticException(t.message)
		}
	case primitiveReport:
		enabled, err := diagnostic.ProcessVerboseEnabled(t.level)
		if err != nil {
			return err
		}
		if enabled {
			stream, err := diagnostic.BorrowProcessVerboseStream()
			if err != nil {
				return err
			}
			if err = stream.AppendCString(t.message + "\n"); err != nil {
				return err
			}
			if err = stream.Flush(); err != nil {
				return err
			}
		}
	case primitiveTrace:
		err := diagnostic.BorrowProcessTraceTags().EmitShort(t.message, func(w io.Writer) error { return in.Display(w) })
		if err != nil {
			return err
		}
	}
	return result.AddGoal(in)
}
func (t *primitiveTactic) UpdateParams(*uparams.ParamsRef) error         { return t.open() }
func (t *primitiveTactic) CollectParamDescrs(*uparams.ParamDescrs) error { return t.open() }
func (t *primitiveTactic) CollectStatistics() map[string]float64         { return nil }
func (t *primitiveTactic) ResetStatistics() error                        { return t.open() }
func (t *primitiveTactic) Cleanup() error                                { return t.open() }
func (t *primitiveTactic) Reset() error                                  { return t.Cleanup() }
func (t *primitiveTactic) SetLogic(usymbol.Symbol) error                 { return t.open() }
func (t *primitiveTactic) SetProgressCallback(func(uint32)) error        { return t.open() }
func (t *primitiveTactic) Translate(*ast.ASTManager) (Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return t, nil
}

func Exec(t Tactic, in *Goal, result *TacticResult) (err error) {
	if t == nil || in == nil || result == nil {
		return ast.ErrASTHandle
	}
	if err = t.ResetStatistics(); err != nil {
		return err
	}
	err = t.Apply(in, result)
	return errors.Join(err, t.Cleanup())
}

type DependencyRef struct {
	manager *ast.ASTManager
	value   ast.ASTDependency
	owned   bool
}

func (r *DependencyRef) Set(m *ast.ASTManager, value ast.ASTDependency) error {
	if r == nil || m == nil || r.manager != nil && r.manager != m {
		return ast.ErrASTHandle
	}
	if err := m.DependencyInc(value); err != nil {
		return err
	}
	if r.owned {
		if err := m.DependencyDec(r.value); err != nil {
			_ = m.DependencyDec(value)
			return err
		}
	}
	r.manager, r.value, r.owned = m, value, !value.IsNull()
	return nil
}
func (r *DependencyRef) Get() ast.ASTDependency { return r.value }
func (r *DependencyRef) Close() error {
	if r == nil || !r.owned {
		return nil
	}
	if err := r.manager.DependencyDec(r.value); err != nil {
		return err
	}
	r.value, r.owned, r.manager = ast.ASTDependency{}, false, nil
	return nil
}

func CheckSat(t Tactic, g *Goal, md *ModelRef, labels *ModelLabels, proof ast.ASTRef, core *DependencyRef) (status lbool.LBool, reason string, err error) {
	if t == nil || g == nil || md == nil || labels == nil || core == nil {
		return lbool.Undef, "", ast.ErrASTHandle
	}
	models, err := g.ModelsEnabled()
	if err != nil {
		return lbool.Undef, "", err
	}
	proofs, err := g.ProofsEnabled()
	if err != nil {
		return lbool.Undef, "", err
	}
	cores, err := g.UnsatCoresEnabled()
	if err != nil {
		return lbool.Undef, "", err
	}
	if err = md.Set(nil); err != nil {
		return lbool.Undef, "", err
	}
	if err = g.manager.SetRef(proof, ast.ASTHandle{}); err != nil {
		return lbool.Undef, "", err
	}
	if err = core.Set(g.manager, ast.ASTDependency{}); err != nil {
		return lbool.Undef, "", err
	}
	result := &TacticResult{}
	defer func() { err = errors.Join(err, result.Close()) }()
	if err = Exec(t, g, result); err != nil {
		var tacticErr TacticException
		if errors.As(err, &tacticErr) {
			return lbool.Undef, tacticErr.Message(), nil
		}
		return lbool.Undef, "", err
	}
	if result.DecidedSat() {
		if models {
			if result.ModelConverter != nil {
				if err = result.ModelConverter.ApplyLabels(labels, 0); err != nil {
					return lbool.Undef, "", err
				}
			}
			if err = ModelConverterToModel(g.manager, result.ModelConverter, md); err != nil {
				return lbool.Undef, "", err
			}
			if value, e := md.Get(); e != nil {
				return lbool.Undef, "", e
			} else if value == nil {
				empty, e := model.NewModel(g.manager)
				if e != nil {
					return lbool.Undef, "", e
				}
				if e = md.Set(empty.Core()); e != nil {
					return lbool.Undef, "", errors.Join(e, empty.Close())
				}
			}
		}
		return lbool.True, "", nil
	}
	if result.DecidedUnsat() {
		final := result.Goals[0]
		if proofs {
			value, e := final.Proof(0)
			if e != nil {
				return lbool.Undef, "", e
			}
			if e = g.manager.SetRef(proof, value); e != nil {
				return lbool.Undef, "", e
			}
		}
		if cores {
			value, e := final.Dependency(0)
			if e != nil {
				return lbool.Undef, "", e
			}
			if e = core.Set(g.manager, value); e != nil {
				return lbool.Undef, "", e
			}
		}
		return lbool.False, "", nil
	}
	if models {
		if err = ModelConverterToModel(g.manager, result.ModelConverter, md); err != nil {
			return lbool.Undef, "", err
		}
		if result.ModelConverter != nil {
			if err = result.ModelConverter.ApplyLabels(labels, 0); err != nil {
				return lbool.Undef, "", err
			}
		}
	}
	return lbool.Undef, "incomplete", nil
}

func FailIfProofGeneration(name string, g *Goal) error {
	if enabled, err := g.ProofsEnabled(); err != nil {
		return err
	} else if enabled {
		return NewTacticException(fmt.Sprintf("%s does not support proof production", name))
	}
	return nil
}
func FailIfUnsatCoreGeneration(name string, g *Goal) error {
	if enabled, err := g.UnsatCoresEnabled(); err != nil {
		return err
	} else if enabled {
		return NewTacticException(fmt.Sprintf("%s does not support unsat core production", name))
	}
	return nil
}
func FailIfModelGeneration(name string, g *Goal) error {
	if enabled, err := g.ModelsEnabled(); err != nil {
		return err
	} else if enabled {
		return NewTacticException(fmt.Sprintf("%s does not generate models", name))
	}
	return nil
}
