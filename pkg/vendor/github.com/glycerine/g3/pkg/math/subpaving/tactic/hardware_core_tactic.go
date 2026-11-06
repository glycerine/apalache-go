package tactic

// Source: src/math/subpaving/tactic/subpaving_tactic.cpp:193-263,
// concrete HWF core tactic with source print_nodes support.

import (
	"errors"
	"io"
	"os"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/subpaving"
	goaltactic "github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

var ErrHardwareCorePrintNodes = errors.New("subpaving tactic: SMT2 display for this mapped rational expression is not yet ported")

// HardwareCoreTactic owns an HWF engine, its expression converter and one
// parameter snapshot. The AST manager, rational manager and goals are borrowed.
type HardwareCoreTactic struct {
	self        *HardwareCoreTactic
	manager     *ast.ASTManager
	numbers     *numeral.RationalManager
	pc          *params.ParamsContext
	params      params.ParamsRef
	engine      *HardwareEngine
	converter   *HardwareExprConverter
	cooperation *cooperate.Cooperation
	stats       statistics.Statistics
	ownsNumbers bool
	closed      bool
}

func NewHardwareCoreTacticOwned(m *ast.ASTManager, pc *params.ParamsContext, p *params.ParamsRef) (*HardwareCoreTactic, error) {
	if m == nil || m.NumeralService() == nil {
		return nil, arena.ErrInvalidSlot
	}
	numbers, err := m.NumeralService().NewPrivateRationalManager()
	if err != nil {
		return nil, err
	}
	t, err := NewHardwareCoreTactic(m, numbers, pc, p)
	if err != nil {
		_ = numbers.CloseChecked()
		return t, err
	}
	t.ownsNumbers = true
	return t, nil
}

func NewHardwareCoreTactic(m *ast.ASTManager, numbers *numeral.RationalManager, pc *params.ParamsContext, p *params.ParamsRef) (*HardwareCoreTactic, error) {
	if m == nil || numbers == nil {
		return nil, arena.ErrInvalidSlot
	}
	if pc == nil || p == nil {
		return nil, params.ErrParamsOwner
	}
	astNumerals, parameterNumerals := m.NumeralService(), pc.Numerals()
	if astNumerals == nil || parameterNumerals == nil {
		return nil, arena.ErrInvalidSlot
	}
	runtimes := []*memory.Runtime{numbers.RuntimeOwner(), astNumerals.RuntimeOwner(), parameterNumerals.RuntimeOwner()}
	var process *memory.ProcessAccounting
	for _, runtime := range runtimes {
		if runtime == nil {
			continue
		}
		other := runtime.ProcessOwner()
		if process != nil && other != nil && process != other {
			return nil, arena.ErrConfig
		}
		if other != nil {
			process = other
		}
	}
	owned, err := pc.NewValue()
	if err != nil {
		return nil, err
	}
	if err := pc.Assign(&owned, p); err != nil {
		return nil, errors.Join(err, pc.Del(&owned))
	}
	t := &HardwareCoreTactic{manager: m, numbers: numbers, pc: pc, params: owned}
	t.self = t
	if err := t.rebuild(); err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}

func (t *HardwareCoreTactic) rebuild() error {
	engine, err := NewHardwareEngine(t.manager.ResourceLimit(), t.numbers, t.pc, &t.params)
	if err != nil {
		if engine != nil {
			err = errors.Join(err, engine.Close())
		}
		return err
	}
	converter, err := NewHardwareExprConverter(t.manager, engine)
	if err != nil {
		return errors.Join(err, engine.Close())
	}
	t.engine, t.converter = engine, converter
	if t.cooperation != nil {
		if err := engine.BindCooperation(t.cooperation); err != nil {
			return err
		}
		if err := converter.BindCooperation(t.cooperation); err != nil {
			return err
		}
	}
	return nil
}

func (t *HardwareCoreTactic) open() error {
	if t == nil || t.self != t {
		return arena.ErrInvalidSlot
	}
	if t.closed {
		return arena.ErrClosed
	}
	if t.engine == nil || t.converter == nil {
		return arena.ErrInvalidSlot
	}
	return t.converter.check()
}

func (t *HardwareCoreTactic) BindCooperation(owner *cooperate.Cooperation) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.engine.BindCooperation(owner); err != nil {
		return err
	}
	if err := t.converter.BindCooperation(owner); err != nil {
		return err
	}
	t.cooperation = owner
	return nil
}

// Apply internalizes and searches the borrowed goal. The source core tactic
// returns the same goal and no model, proof or dependency converter.
func (t *HardwareCoreTactic) Apply(g *goaltactic.Goal) (*goaltactic.Goal, error) {
	return t.ApplyWithOutput(g, os.Stdout)
}

// ApplyWithOutput routes optional print_nodes text to a borrowed writer.
func (t *HardwareCoreTactic) ApplyWithOutput(g *goaltactic.Goal, out io.Writer) (result *goaltactic.Goal, err error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if g == nil {
		return nil, arena.ErrInvalidSlot
	}
	if err := g.CheckOwner(); err != nil {
		return nil, err
	}
	if g.Manager() != t.manager {
		return nil, ast.ErrASTHandle
	}
	options, err := t.engine.Options()
	if err != nil {
		return nil, err
	}
	if options.PrintNodes && out == nil {
		return nil, arena.ErrInvalidSlot
	}
	defer func() {
		var source z3exception.Z3Exception
		if errors.As(err, &source) {
			result, err = nil, goaltactic.NewTacticException(source.Message())
		}
	}()
	if err := t.converter.InternalizeGoal(g); err != nil {
		if errors.Is(err, subpaving.ErrSubpavingFailure) {
			return nil, goaltactic.NewTacticException("failed to internalize goal into subpaving module")
		}
		return nil, err
	}
	context, err := t.engine.Shell()
	if err != nil {
		return nil, err
	}
	display, err := t.converter.VariableDisplay(t.pc.SymbolsField)
	if err != nil {
		return nil, err
	}
	if err := context.SetVariableDisplay(display); err != nil {
		return nil, err
	}
	if err := context.Search(); err != nil {
		if errors.Is(err, subpaving.ErrSubpavingFailure) {
			return nil, goaltactic.NewTacticException("failed building subpaving tree...")
		}
		return nil, err
	}
	if options.PrintNodes {
		stream := &fastTacticOutput{out: out}
		if err := context.DisplayConstraints(stream, false); err != nil {
			return nil, err
		}
		if err := subpavingWriteDisplay(stream, "bounds at leaves: \n"); err != nil {
			return nil, err
		}
		if err := context.DisplayBounds(stream); err != nil {
			return nil, err
		}
	}
	var current statistics.Statistics
	if err := context.CollectStatistics(&current); err != nil {
		return nil, err
	}
	t.stats.Copy(current)
	return g, nil
}

func (t *HardwareCoreTactic) CollectStatistics(dst *statistics.Statistics) error {
	if err := t.open(); err != nil {
		return err
	}
	if dst == nil {
		return arena.ErrInvalidSlot
	}
	dst.Copy(t.stats)
	return nil
}

func (t *HardwareCoreTactic) ResetStatistics() error {
	if err := t.open(); err != nil {
		return err
	}
	t.stats.Reset()
	return nil
}

func (t *HardwareCoreTactic) UpdateParams(p *params.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	selected, err := ReadOptions(t.pc, p)
	if err != nil {
		return err
	}
	if selected.Engine != EngineHWF {
		return ErrHardwareEngineSelection
	}
	if err := t.pc.Assign(&t.params, p); err != nil {
		return err
	}
	return t.engine.UpdateParamsRef(t.pc, &t.params)
}

// Cleanup releases the converter before its context, then rebuilds the
// selected HWF implementation from the retained parameter snapshot.
func (t *HardwareCoreTactic) Cleanup() error {
	if t == nil || t.self != t {
		return arena.ErrInvalidSlot
	}
	if t.closed {
		return arena.ErrClosed
	}
	if t.converter != nil {
		if err := t.converter.Close(); err != nil {
			return err
		}
		t.converter = nil
	}
	if t.engine != nil {
		if err := t.engine.Close(); err != nil {
			return err
		}
		t.engine = nil
	}
	return t.rebuild()
}

func (t *HardwareCoreTactic) Close() error {
	if t == nil || t.self != t {
		return arena.ErrInvalidSlot
	}
	if t.closed {
		return nil
	}
	if t.converter != nil {
		if err := t.converter.Close(); err != nil {
			return err
		}
		t.converter = nil
	}
	if t.engine != nil {
		if err := t.engine.Close(); err != nil {
			return err
		}
		t.engine = nil
	}
	if err := t.pc.Del(&t.params); err != nil {
		return err
	}
	if t.ownsNumbers && t.numbers != nil {
		if err := t.numbers.CloseChecked(); err != nil {
			return err
		}
		t.numbers = nil
	}
	t.closed = true
	return nil
}
