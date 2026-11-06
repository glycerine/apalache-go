package tactic

// Source: src/math/subpaving/tactic/subpaving_tactic.cpp:193-263,
// concrete MPF core tactic with source print_nodes support.

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

var ErrArbitraryCorePrintNodes = errors.New("subpaving tactic: SMT2 display for this mapped rational expression is not yet ported")

// ArbitraryCoreTactic owns an MPF engine, its expression converter and one
// parameter snapshot. The AST manager, rational manager and goals are borrowed.
type ArbitraryCoreTactic struct {
	self        *ArbitraryCoreTactic
	manager     *ast.ASTManager
	numbers     *numeral.ArbitraryFloatNumeralManager
	floatCore   *numeral.ArbitraryFloatManager
	rational    *numeral.RationalManager
	pc          *params.ParamsContext
	params      params.ParamsRef
	engine      *ArbitraryEngine
	converter   *ArbitraryExprConverter
	cooperation *cooperate.Cooperation
	stats       statistics.Statistics
	ownsNumbers bool
	closed      bool
}

func NewArbitraryCoreTacticOwned(m *ast.ASTManager, pc *params.ParamsContext, p *params.ParamsRef) (*ArbitraryCoreTactic, error) {
	if m == nil || m.NumeralService() == nil {
		return nil, arena.ErrInvalidSlot
	}
	rational, err := m.NumeralService().NewPrivateRationalManager()
	if err != nil {
		return nil, err
	}
	floatCore, err := m.NumeralService().NewArbitraryFloatManager()
	if err != nil {
		_ = rational.CloseChecked()
		return nil, err
	}
	numbers, err := numeral.NewArbitraryFloatNumeralManager(floatCore, 11, 53)
	if err != nil {
		_ = floatCore.Close()
		_ = rational.CloseChecked()
		return nil, err
	}
	t, err := NewArbitraryCoreTactic(m, numbers, rational, pc, p)
	if err != nil {
		_ = numbers.Close()
		_ = floatCore.Close()
		_ = rational.CloseChecked()
		return t, err
	}
	t.floatCore, t.ownsNumbers = floatCore, true
	return t, nil
}

func NewArbitraryCoreTactic(m *ast.ASTManager, numbers *numeral.ArbitraryFloatNumeralManager, rational *numeral.RationalManager, pc *params.ParamsContext, p *params.ParamsRef) (*ArbitraryCoreTactic, error) {
	if m == nil || numbers == nil || rational == nil {
		return nil, arena.ErrInvalidSlot
	}
	if pc == nil || p == nil {
		return nil, params.ErrParamsOwner
	}
	astNumerals, parameterNumerals := m.NumeralService(), pc.Numerals()
	if astNumerals == nil || parameterNumerals == nil {
		return nil, arena.ErrInvalidSlot
	}
	runtimes := []*memory.Runtime{rational.RuntimeOwner(), astNumerals.RuntimeOwner(), parameterNumerals.RuntimeOwner()}
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
	t := &ArbitraryCoreTactic{manager: m, numbers: numbers, rational: rational, pc: pc, params: owned}
	t.self = t
	if err := t.rebuild(); err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}

func (t *ArbitraryCoreTactic) rebuild() error {
	engine, err := NewArbitraryEngine(t.manager.ResourceLimit(), t.numbers, t.pc, &t.params)
	if err != nil {
		if engine != nil {
			err = errors.Join(err, engine.Close())
		}
		return err
	}
	converter, err := NewArbitraryExprConverter(t.manager, engine, t.rational)
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

func (t *ArbitraryCoreTactic) open() error {
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

func (t *ArbitraryCoreTactic) BindCooperation(owner *cooperate.Cooperation) error {
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
func (t *ArbitraryCoreTactic) Apply(g *goaltactic.Goal) (*goaltactic.Goal, error) {
	return t.ApplyWithOutput(g, os.Stdout)
}

// ApplyWithOutput routes optional print_nodes text to a borrowed writer.
func (t *ArbitraryCoreTactic) ApplyWithOutput(g *goaltactic.Goal, out io.Writer) (result *goaltactic.Goal, err error) {
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

func (t *ArbitraryCoreTactic) CollectStatistics(dst *statistics.Statistics) error {
	if err := t.open(); err != nil {
		return err
	}
	if dst == nil {
		return arena.ErrInvalidSlot
	}
	dst.Copy(t.stats)
	return nil
}

func (t *ArbitraryCoreTactic) ResetStatistics() error {
	if err := t.open(); err != nil {
		return err
	}
	t.stats.Reset()
	return nil
}

func (t *ArbitraryCoreTactic) UpdateParams(p *params.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	selected, err := ReadOptions(t.pc, p)
	if err != nil {
		return err
	}
	if selected.Engine != EngineMPF {
		return ErrArbitraryEngineSelection
	}
	if err := t.pc.Assign(&t.params, p); err != nil {
		return err
	}
	return t.engine.UpdateParamsRef(t.pc, &t.params)
}

// Cleanup releases the converter before its context, then rebuilds the
// selected MPF implementation from the retained parameter snapshot.
func (t *ArbitraryCoreTactic) Cleanup() error {
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

func (t *ArbitraryCoreTactic) Close() error {
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
	if t.ownsNumbers {
		if t.numbers != nil {
			if err := t.numbers.Close(); err != nil {
				return err
			}
			t.numbers = nil
		}
		if t.floatCore != nil {
			if err := t.floatCore.Close(); err != nil {
				return err
			}
			t.floatCore = nil
		}
		if t.rational != nil {
			if err := t.rational.CloseChecked(); err != nil {
				return err
			}
			t.rational = nil
		}
	}
	t.closed = true
	return nil
}
