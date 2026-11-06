package tactic

// Source: src/math/subpaving/tactic/subpaving_tactic.cpp:193-263;
// concrete MPFF core tactic, excluding the public simplify composition.

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

var ErrFastCorePrintNodes = errors.New("subpaving tactic: SMT2 display for this mapped expression is not yet ported")

// The source print_nodes route writes to std::cout with its default
// nonthrowing failure state. Keep a borrowed Go writer's first failed prefix
// while allowing the tactic to publish statistics and its result.
type fastTacticOutput struct {
	out    io.Writer
	failed bool
}

func (w *fastTacticOutput) Write(p []byte) (int, error) {
	if !w.failed {
		n, err := w.out.Write(p)
		if err != nil || n != len(p) {
			w.failed = true
		}
	}
	return len(p), nil
}

// FastCoreTactic owns the selected engine, expression converter and a shared
// parameter snapshot. Its numeral managers are owned by NewFastCoreTacticOwned
// and borrowed by NewFastCoreTactic; the AST manager and parameter service are
// borrowed in both cases. The returned goal is borrowed.
type FastCoreTactic struct {
	self                       *FastCoreTactic
	manager                    *ast.ASTManager
	numbers                    *numeral.FastFloatManager
	rational                   *numeral.RationalManager
	pc                         *params.ParamsContext
	params                     params.ParamsRef
	engine                     *FastEngine
	cooperation                *cooperate.Cooperation
	converter                  *FastExprConverter
	stats                      statistics.Statistics
	ownsNumerals, paramsClosed bool
	closed                     bool
}

// NewFastCoreTacticOwned constructs the two numeral-manager members of the
// retained MPFF tactic from its AST manager's source memory process. The AST
// and parameter services remain borrowed; Close releases MPFF before rational.
func NewFastCoreTacticOwned(m *ast.ASTManager, pc *params.ParamsContext, p *params.ParamsRef) (*FastCoreTactic, error) {
	if m == nil || pc == nil || p == nil {
		return nil, arena.ErrInvalidSlot
	}
	q := m.NumeralService()
	if q == nil || q.RuntimeOwner() == nil || pc.Numerals() == nil || pc.Numerals().RuntimeOwner() == nil {
		return nil, arena.ErrConfig
	}
	runtime := q.RuntimeOwner()
	process := runtime.ProcessOwner()
	if process == nil || pc.Numerals().RuntimeOwner().ProcessOwner() != process {
		return nil, arena.ErrConfig
	}
	budget := runtime.Budget()
	integers := numeral.IntegerConfig{Memory: runtime, Process: process, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}
	rational, err := numeral.NewRationalManager(numeral.RationalConfig{Integers: integers})
	if err != nil {
		return nil, err
	}
	fastConfig := numeral.DefaultFastFloatConfig(arena.VectorConfig{Descriptors: arena.Config{Budget: budget}})
	fastConfig.Memory = runtime
	fastConfig.Process = process
	numbers, err := numeral.NewFastFloatManager(fastConfig)
	if err != nil {
		pending := &FastCoreTactic{rational: rational, ownsNumerals: true, paramsClosed: true}
		pending.self = pending
		if cleanup := pending.Close(); cleanup != nil {
			return pending, errors.Join(err, cleanup)
		}
		return nil, err
	}
	t, err := NewFastCoreTactic(m, numbers, rational, pc, p)
	if err != nil {
		if t == nil {
			t = &FastCoreTactic{numbers: numbers, rational: rational, ownsNumerals: true, paramsClosed: true}
			t.self = t
		} else {
			t.ownsNumerals = true
		}
		if cleanup := t.Close(); cleanup != nil {
			return t, errors.Join(err, cleanup)
		}
		return nil, err
	}
	t.ownsNumerals = true
	return t, nil
}

func NewFastCoreTactic(m *ast.ASTManager, numbers *numeral.FastFloatManager, rational *numeral.RationalManager, pc *params.ParamsContext, p *params.ParamsRef) (*FastCoreTactic, error) {
	if m == nil || numbers == nil || rational == nil {
		return nil, arena.ErrInvalidSlot
	}
	if pc == nil || p == nil {
		return nil, params.ErrParamsOwner
	}
	astNumerals := m.NumeralService()
	parameterNumerals := pc.Numerals()
	if astNumerals == nil || parameterNumerals == nil {
		return nil, arena.ErrInvalidSlot
	}
	owners := []*memory.Runtime{numbers.RuntimeOwner(), rational.RuntimeOwner(), astNumerals.RuntimeOwner(), parameterNumerals.RuntimeOwner()}
	for _, owner := range owners {
		if owner == nil {
			return nil, arena.ErrInvalidSlot
		}
	}
	process := owners[0].ProcessOwner()
	for _, owner := range owners[1:] {
		if other := owner.ProcessOwner(); (process != nil || other != nil) && process != other {
			return nil, arena.ErrConfig
		}
	}
	owned, err := pc.NewValue()
	if err != nil {
		return nil, err
	}
	if err := pc.Assign(&owned, p); err != nil {
		return nil, errors.Join(err, pc.Del(&owned))
	}
	t := &FastCoreTactic{manager: m, numbers: numbers, rational: rational, pc: pc, params: owned}
	t.self = t
	if err := t.rebuild(); err != nil {
		if cleanup := t.Close(); cleanup != nil {
			return t, errors.Join(err, cleanup)
		}
		return nil, err
	}
	return t, nil
}

func (t *FastCoreTactic) rebuild() error {
	engine, err := NewFastEngine(t.manager.ResourceLimit(), t.numbers, t.rational, t.pc, &t.params)
	if err != nil {
		if engine != nil {
			err = errors.Join(err, engine.Close())
		}
		return err
	}
	if t.cooperation != nil {
		if err := engine.BindCooperation(t.cooperation); err != nil {
			return errors.Join(err, engine.Close())
		}
	}
	converter, err := NewFastExprConverter(t.manager, engine, t.rational)
	if err != nil {
		return errors.Join(err, engine.Close())
	}
	if t.cooperation != nil {
		if err := converter.BindCooperation(t.cooperation); err != nil {
			return errors.Join(err, converter.Close(), engine.Close())
		}
	}
	t.engine, t.converter = engine, converter
	return nil
}

// BindCooperation retains the borrowed source process owner across Cleanup.
func (t *FastCoreTactic) BindCooperation(owner *cooperate.Cooperation) error {
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

func (t *FastCoreTactic) open() error {
	if t == nil || t.self != t {
		return arena.ErrInvalidSlot
	}
	if t.closed {
		return arena.ErrClosed
	}
	if t.converter == nil || t.engine == nil {
		return arena.ErrInvalidSlot
	}
	return t.converter.check()
}

// Apply internalizes and searches the input in place. As in the source core
// tactic, a successful result contains exactly the original goal; it creates
// no model, proof, or unsat-core converter.
func (t *FastCoreTactic) Apply(g *goaltactic.Goal) (*goaltactic.Goal, error) {
	return t.ApplyWithOutput(g, os.Stdout)
}

// ApplyWithOutput routes the source optional print_nodes output to a caller
// writer. Nil is valid only when print_nodes is disabled.
func (t *FastCoreTactic) ApplyWithOutput(g *goaltactic.Goal, out io.Writer) (result *goaltactic.Goal, err error) {
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
	// The source operator catches z3_exception around process, statistics and
	// result publication, then throws a fresh tactic_exception with ex.msg().
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
	shell, err := t.engine.Shell()
	if err != nil {
		return nil, err
	}
	display, err := t.converter.VariableDisplay(t.pc.SymbolsField)
	if err != nil {
		return nil, err
	}
	if err := shell.SetVariableDisplay(display); err != nil {
		return nil, err
	}
	if err := shell.Search(); err != nil {
		if errors.Is(err, subpaving.ErrSubpavingFailure) {
			return nil, goaltactic.NewTacticException("failed building subpaving tree...")
		}
		return nil, err
	}
	if options.PrintNodes {
		stream := &fastTacticOutput{out: out}
		if err := shell.DisplayConstraints(stream, true); err != nil {
			return nil, err
		}
		if err := subpavingWriteDisplay(stream, "bounds at leaves: \n"); err != nil {
			return nil, err
		}
		if err := shell.DisplayBounds(stream); err != nil {
			return nil, err
		}
	}
	var current statistics.Statistics
	if err := shell.CollectStatistics(&current); err != nil {
		return nil, err
	}
	t.stats.Copy(current)
	return g, nil
}

func (t *FastCoreTactic) CollectStatistics(dst *statistics.Statistics) error {
	if err := t.open(); err != nil {
		return err
	}
	if dst == nil {
		return arena.ErrInvalidSlot
	}
	dst.Copy(t.stats)
	return nil
}

func (t *FastCoreTactic) ResetStatistics() error {
	if err := t.open(); err != nil {
		return err
	}
	t.stats.Reset()
	return nil
}

// UpdateParams retains the caller's parameter value before updating the live
// selected MPFF context. Other numeral engines require their own concrete
// tactic owners and leave this MPFF owner unchanged.
func (t *FastCoreTactic) UpdateParams(p *params.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	selected, err := ReadOptions(t.pc, p)
	if err != nil {
		return err
	}
	if selected.Engine != EngineMPFF {
		return ErrFastEngineSelection
	}
	if err := t.pc.Assign(&t.params, p); err != nil {
		return err
	}
	return t.engine.UpdateParamsRef(t.pc, &t.params)
}

// Cleanup releases the old converter before its context, then constructs a
// fresh implementation from the tactic's retained parameter snapshot.
// Accumulated outer statistics survive, as in the source tactic.
func (t *FastCoreTactic) Cleanup() error {
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

func (t *FastCoreTactic) Close() error {
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
	if !t.paramsClosed {
		if err := t.pc.Del(&t.params); err != nil {
			return err
		}
		t.paramsClosed = true
	}
	if t.ownsNumerals {
		if t.numbers != nil {
			if err := t.numbers.CloseChecked(); err != nil {
				return err
			}
			t.numbers = nil
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
