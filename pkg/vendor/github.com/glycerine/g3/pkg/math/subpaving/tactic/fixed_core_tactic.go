package tactic

// Source: src/math/subpaving/tactic/subpaving_tactic.cpp:193-263;
// concrete MPFX core tactic, excluding the public simplify composition.

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

var ErrFixedCorePrintNodes = errors.New("subpaving tactic: SMT2 display for this mapped expression is not yet ported")

// The source print_nodes route writes to std::cout with its default
// nonthrowing failure state. Keep a borrowed Go writer's first failed prefix
// while allowing the tactic to publish statistics and its result.
type fixedTacticOutput struct {
	out    io.Writer
	failed bool
}

func (w *fixedTacticOutput) Write(p []byte) (int, error) {
	if !w.failed {
		n, err := w.out.Write(p)
		if err != nil || n != len(p) {
			w.failed = true
		}
	}
	return len(p), nil
}

// FixedCoreTactic owns the selected engine, expression converter and a shared
// parameter snapshot. Its numeral managers are owned by NewFixedCoreTacticOwned
// and borrowed by NewFixedCoreTactic; the AST manager and parameter service are
// borrowed in both cases. The returned goal is borrowed.
type FixedCoreTactic struct {
	self                       *FixedCoreTactic
	manager                    *ast.ASTManager
	numbers                    *numeral.FixedPointManager
	rational                   *numeral.RationalManager
	pc                         *params.ParamsContext
	params                     params.ParamsRef
	engine                     *FixedEngine
	cooperation                *cooperate.Cooperation
	converter                  *FixedExprConverter
	stats                      statistics.Statistics
	ownsNumerals, paramsClosed bool
	closed                     bool
}

// NewFixedCoreTacticOwned constructs the two numeral-manager members of the
// retained MPFX tactic from its AST manager's source memory process. The AST
// and parameter services remain borrowed; Close releases MPFX before rational.
func NewFixedCoreTacticOwned(m *ast.ASTManager, pc *params.ParamsContext, p *params.ParamsRef) (*FixedCoreTactic, error) {
	if m == nil || pc == nil || p == nil {
		return nil, arena.ErrInvalidSlot
	}
	q := m.NumeralService()
	if q == nil || pc.Numerals() == nil {
		return nil, arena.ErrConfig
	}
	var process *memory.ProcessAccounting
	if runtime := q.RuntimeOwner(); runtime != nil {
		process = runtime.ProcessOwner()
	}
	if other := pc.Numerals().RuntimeOwner(); other != nil {
		if otherProcess := other.ProcessOwner(); (process != nil || otherProcess != nil) && process != otherProcess {
			return nil, arena.ErrConfig
		}
	}
	budget := m.Budget()
	integers := numeral.IntegerConfig{Process: process, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}
	rational, err := numeral.NewRationalManager(numeral.RationalConfig{Integers: integers})
	if err != nil {
		return nil, err
	}
	numbers, err := numeral.NewFixedPointManager(2, 1)
	if err != nil {
		pending := &FixedCoreTactic{rational: rational, ownsNumerals: true, paramsClosed: true}
		pending.self = pending
		if cleanup := pending.Close(); cleanup != nil {
			return pending, errors.Join(err, cleanup)
		}
		return nil, err
	}
	t, err := NewFixedCoreTactic(m, numbers, rational, pc, p)
	if err != nil {
		if t == nil {
			t = &FixedCoreTactic{numbers: numbers, rational: rational, ownsNumerals: true, paramsClosed: true}
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

func NewFixedCoreTactic(m *ast.ASTManager, numbers *numeral.FixedPointManager, rational *numeral.RationalManager, pc *params.ParamsContext, p *params.ParamsRef) (*FixedCoreTactic, error) {
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
	owners := []*memory.Runtime{rational.RuntimeOwner(), astNumerals.RuntimeOwner(), parameterNumerals.RuntimeOwner()}
	var process *memory.ProcessAccounting
	processSet := false
	for _, owner := range owners {
		if owner == nil {
			continue
		}
		other := owner.ProcessOwner()
		if processSet && (process != nil || other != nil) && process != other {
			return nil, arena.ErrConfig
		}
		process, processSet = other, true
	}
	owned, err := pc.NewValue()
	if err != nil {
		return nil, err
	}
	if err := pc.Assign(&owned, p); err != nil {
		return nil, errors.Join(err, pc.Del(&owned))
	}
	t := &FixedCoreTactic{manager: m, numbers: numbers, rational: rational, pc: pc, params: owned}
	t.self = t
	if err := t.rebuild(); err != nil {
		if cleanup := t.Close(); cleanup != nil {
			return t, errors.Join(err, cleanup)
		}
		return nil, err
	}
	return t, nil
}

func (t *FixedCoreTactic) rebuild() error {
	engine, err := NewFixedEngine(t.manager.ResourceLimit(), t.numbers, t.rational, t.pc, &t.params)
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
	converter, err := NewFixedExprConverter(t.manager, engine, t.rational)
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
func (t *FixedCoreTactic) BindCooperation(owner *cooperate.Cooperation) error {
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

func (t *FixedCoreTactic) open() error {
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
func (t *FixedCoreTactic) Apply(g *goaltactic.Goal) (*goaltactic.Goal, error) {
	return t.ApplyWithOutput(g, os.Stdout)
}

// ApplyWithOutput routes the source optional print_nodes output to a caller
// writer. Nil is valid only when print_nodes is disabled.
func (t *FixedCoreTactic) ApplyWithOutput(g *goaltactic.Goal, out io.Writer) (result *goaltactic.Goal, err error) {
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
		stream := &fixedTacticOutput{out: out}
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

func (t *FixedCoreTactic) CollectStatistics(dst *statistics.Statistics) error {
	if err := t.open(); err != nil {
		return err
	}
	if dst == nil {
		return arena.ErrInvalidSlot
	}
	dst.Copy(t.stats)
	return nil
}

// CollectParamDescriptions follows the source core tactic's delegation to its
// selected context, followed by the numeral and print_nodes entries.
func (t *FixedCoreTactic) CollectParamDescriptions(dst *params.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	return CollectParamDescriptions(dst)
}

// Translate constructs a separate MPFX core tactic for another AST manager,
// retaining this tactic's parameter snapshot as the source does. The new
// tactic owns its numeral managers and borrows the destination AST manager.
func (t *FixedCoreTactic) Translate(m *ast.ASTManager) (*FixedCoreTactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return NewFixedCoreTacticOwned(m, t.pc, &t.params)
}

func (t *FixedCoreTactic) ResetStatistics() error {
	if err := t.open(); err != nil {
		return err
	}
	t.stats.Reset()
	return nil
}

// UpdateParams retains the caller's parameter value before updating the live
// selected MPFX context. Other numeral engines require their own concrete
// tactic owners and leave this MPFX owner unchanged.
func (t *FixedCoreTactic) UpdateParams(p *params.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	selected, err := ReadOptions(t.pc, p)
	if err != nil {
		return err
	}
	if selected.Engine != EngineMPFX {
		return ErrFixedEngineSelection
	}
	if err := t.pc.Assign(&t.params, p); err != nil {
		return err
	}
	return t.engine.UpdateParamsRef(t.pc, &t.params)
}

// Cleanup releases the old converter before its context, then constructs a
// fresh implementation from the tactic's retained parameter snapshot.
// Accumulated outer statistics survive, as in the source tactic.
func (t *FixedCoreTactic) Cleanup() error {
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

func (t *FixedCoreTactic) Close() error {
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
			if err := t.numbers.Close(); err != nil {
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
