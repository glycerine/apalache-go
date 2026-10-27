package apalache

import (
	"fmt"
	"sort"
	"strings"
)

// Binding is an immutable mapping from TLA+ names to arena cells.
type Binding struct{ cells map[string]ArenaCell }

func EmptyBinding() Binding { return Binding{cells: make(map[string]ArenaCell)} }

func NewBinding(entries map[string]ArenaCell) Binding {
	return Binding{cells: cloneMap(entries)}
}

func BindingOf(entries ...BindingEntry) Binding {
	cells := make(map[string]ArenaCell, len(entries))
	for _, entry := range entries {
		cells[entry.Name] = entry.Cell
	}
	return Binding{cells: cells}
}

type BindingEntry struct {
	Name string
	Cell ArenaCell
}

func (binding Binding) Map() map[string]ArenaCell { return cloneMap(binding.cells) }

func (binding Binding) Get(name string) (ArenaCell, bool) {
	cell, found := binding.cells[name]
	return cell, found
}

func (binding Binding) MustGet(name string) ArenaCell {
	cell, found := binding.Get(name)
	if !found {
		panic(fmt.Sprintf("name %s is not present in the binding", name))
	}
	return cell
}

func (binding Binding) Contains(name string) bool {
	_, found := binding.cells[name]
	return found
}

// Merge returns a binding in which entries from other override entries from
// the receiver, matching Scala Map.++.
func (binding Binding) Merge(other Binding) Binding {
	result := binding.Map()
	for name, cell := range other.cells {
		result[name] = cell
	}
	return Binding{cells: result}
}

func (binding Binding) With(name string, cell ArenaCell) Binding {
	return binding.Merge(BindingOf(BindingEntry{Name: name, Cell: cell}))
}

// ForgetPrimed removes all names ending in a prime.
func (binding Binding) ForgetPrimed() Binding {
	result := make(map[string]ArenaCell)
	for name, cell := range binding.cells {
		if !strings.HasSuffix(name, "'") {
			result[name] = cell
		}
	}
	return Binding{cells: result}
}

// ForgetNonPrimed retains primed names and the named constants.
func (binding Binding) ForgetNonPrimed(constants map[string]struct{}) Binding {
	result := make(map[string]ArenaCell)
	for name, cell := range binding.cells {
		_, constant := constants[name]
		if strings.HasSuffix(name, "'") || constant {
			result[name] = cell
		}
	}
	return Binding{cells: result}
}

// Shift removes non-primed variables, retains constants, and strips one prime
// suffix from the remaining variable names.
func (binding Binding) Shift(constants map[string]struct{}) Binding {
	retained := binding.ForgetNonPrimed(constants)
	result := make(map[string]ArenaCell, len(retained.cells))
	for name, cell := range retained.cells {
		result[strings.TrimSuffix(name, "'")] = cell
	}
	return Binding{cells: result}
}

// SymbolicState is one expression together with its solver-backed arena and
// current variable binding.
type SymbolicState struct {
	Expression Expr
	Arena      *ArenaSolverAdapter
	Binding    Binding
}

func NewSymbolicState(expression Expr, arena *ArenaSolverAdapter, binding Binding) (*SymbolicState, error) {
	if expression == nil {
		return nil, fmt.Errorf("symbolic state requires an expression")
	}
	if arena == nil {
		return nil, fmt.Errorf("symbolic state requires an arena")
	}
	return &SymbolicState{Expression: expression, Arena: arena, Binding: binding}, nil
}

func (state *SymbolicState) SetExpression(expression Expr) (*SymbolicState, error) {
	return NewSymbolicState(expression, state.Arena, state.Binding)
}

func (state *SymbolicState) SetArena(arena *ArenaSolverAdapter) (*SymbolicState, error) {
	return NewSymbolicState(state.Expression, arena, state.Binding)
}

func (state *SymbolicState) SetBinding(binding Binding) *SymbolicState {
	return &SymbolicState{Expression: state.Expression, Arena: state.Arena, Binding: binding}
}

func (state *SymbolicState) AsCell() (ArenaCell, error) {
	return state.Arena.PureArena().FindCellByNameExpr(state.Expression)
}

func (state *SymbolicState) UpdateArena(update func(*ArenaSolverAdapter) (*ArenaSolverAdapter, error)) (*SymbolicState, error) {
	if update == nil {
		return nil, fmt.Errorf("arena update cannot be nil")
	}
	arena, err := update(state.Arena)
	if err != nil {
		return nil, err
	}
	return state.SetArena(arena)
}

// ChangedNames returns sorted primed names whose cells differ from their
// unprimed version or whose unprimed version is absent.
func (state *SymbolicState) ChangedNames() []string {
	changed := make([]string, 0)
	for name, primedCell := range state.Binding.cells {
		if !strings.HasSuffix(name, "'") {
			continue
		}
		unprimedCell, found := state.Binding.cells[strings.TrimSuffix(name, "'")]
		if !found || !unprimedCell.Equal(primedCell) {
			changed = append(changed, name)
		}
	}
	sort.Strings(changed)
	return changed
}

// RewriterScope is the solver-free state threaded through stratified rules.
type RewriterScope struct {
	Arena   *PureArena
	Binding Binding
}

func InitialRewriterScope() RewriterScope {
	return RewriterScope{Arena: InitialPureArena(), Binding: EmptyBinding()}
}
