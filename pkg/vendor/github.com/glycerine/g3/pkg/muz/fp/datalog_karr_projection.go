package fp

// Source: dl_compiler.cpp unary tail selection, identical filters, projection
// and head renaming, executed using the concrete Karr relation operations.
import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

type karrUnaryRule struct {
	body           ast.ASTHandle
	constants      []uint32
	constantValues []ast.ASTHandle
	identical      [][]uint32
	karrHeadPlan
}

type karrHeadPlan struct {
	removed       []uint32
	headConstants []*big.Rat
	duplicates    []uint32
	cycles        [][]uint32
}

func (r karrUnaryRule) apply(source *rel.KarrRelation) (*rel.KarrRelation, error) {
	result := source
	var err error
	// The unary compiler selects and removes each constant immediately. The
	// column index therefore refers to the current, already shortened relation.
	if len(r.constants) != 0 || len(r.identical) != 0 {
		result = source.Clone()
	}
	for i, column := range r.constants {
		if !result.Empty() {
			before := result.Clone()
			if err = result.FilterEqual(column, true); err != nil {
				return nil, err
			}
			if checker := result.FormulaChecker(); checker != nil {
				if i >= len(r.constantValues) {
					return nil, ErrKarrDatalogFragment
				}
				if err = checker.VerifyFilterEqual(before, column, r.constantValues[i], result); err != nil {
					return nil, err
				}
			}
		}
		result, err = rel.KarrSourceProject(result, []uint32{column})
		if err != nil {
			return nil, err
		}
	}
	if !result.Empty() {
		for _, group := range r.identical {
			if err = result.FilterIdentical(group); err != nil {
				return nil, err
			}
		}
	}
	return r.karrHeadPlan.apply(result)
}

// apply follows make_assembling_code: remove, add columns, duplicate, rename.
func (r karrHeadPlan) apply(source *rel.KarrRelation) (*rel.KarrRelation, error) {
	result := source
	var err error
	if len(r.removed) != 0 {
		result, err = rel.KarrSourceProject(result, r.removed)
		if err != nil {
			return nil, err
		}
	}
	for _, value := range r.headConstants {
		singleton := rel.NewKarrFull(1, nil)
		if value != nil {
			singleton = rel.NewKarrEmpty(1, nil)
			if err = singleton.AddFact([]*big.Rat{value}); err != nil {
				return nil, err
			}
		}
		result, err = rel.KarrJoin(result, singleton, nil, nil)
		if err != nil {
			return nil, err
		}
	}
	for _, column := range r.duplicates {
		single := result
		if result.Dimension() != 1 {
			var remove []uint32
			for i := uint32(0); i < result.Dimension(); i++ {
				if i != column {
					remove = append(remove, i)
				}
			}
			single, err = rel.KarrSourceProject(result, remove)
			if err != nil {
				return nil, err
			}
		}
		result, err = rel.KarrJoin(result, single, []uint32{column}, []uint32{0})
		if err != nil {
			return nil, err
		}
	}
	for _, cycle := range r.cycles {
		result, err = result.Rename(cycle)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (a *karrDatalogActivation) compileUnaryRule(rule *base.Rule) (ast.ASTHandle, karrUnaryRule, error) {
	var none ast.ASTHandle
	var out karrUnaryRule
	n, err := rule.TailSize()
	if err != nil {
		return none, out, err
	}
	if n != 1 {
		return none, out, ErrKarrDatalogFragment
	}
	n, err = rule.UninterpretedTailSize()
	if err != nil {
		return none, out, err
	}
	if n != 1 {
		return none, out, ErrKarrDatalogFragment
	}
	head, err := rule.Head()
	if err != nil {
		return none, out, err
	}
	tail, negated, err := rule.Tail(0)
	if err != nil {
		return none, out, err
	}
	if negated {
		return none, out, ErrKarrDatalogFragment
	}
	hd, err := a.manager.GetDecl(head)
	if err != nil {
		return none, out, err
	}
	out.body, err = a.manager.GetDecl(tail)
	if err != nil {
		return none, out, err
	}
	if a.seeds[hd] == nil || a.seeds[out.body] == nil {
		return none, out, ErrKarrDatalogFragment
	}
	tailArity, err := a.manager.GetNumArgs(tail)
	if err != nil {
		return none, out, err
	}
	// Constants are selected and projected in left-to-right order. Repeated
	// variable columns refer to the shortened signature after those removals.
	positions := make(map[ast.ASTHandle][]uint32, tailArity)
	order := make([]ast.ASTHandle, 0, tailArity)
	var column uint32
	columns := make([]ast.ASTHandle, 0, tailArity)
	for i := uint32(0); i < tailArity; i++ {
		arg, err := a.manager.GetArg(tail, i)
		if err != nil {
			return none, out, err
		}
		kind, err := a.manager.GetKind(arg)
		if err != nil {
			return none, out, err
		}
		if kind == ast.ASTVar {
			if len(positions[arg]) == 0 {
				order = append(order, arg)
			}
			positions[arg] = append(positions[arg], column)
			columns = append(columns, arg)
			column++
			continue
		}
		_, _, matched, err := a.manager.ArithmeticNumeralText(arg)
		if err != nil {
			return none, out, err
		}
		if !matched {
			return none, out, ErrKarrDatalogFragment
		}
		out.constants = append(out.constants, column)
		out.constantValues = append(out.constantValues, arg)
	}
	for _, arg := range order {
		if len(positions[arg]) > 1 {
			out.identical = append(out.identical, positions[arg])
		}
	}
	out.karrHeadPlan, err = a.compileKarrHead(head, columns)
	if err != nil {
		return none, out, err
	}
	return hd, out, nil
}

// compileKarrHead receives the relation columns after tail filtering and
// projection. Only variable expressions bind head variables; constants are
// assembled through singleton relations even when a tail contains that value.
func (a *karrDatalogActivation) compileKarrHead(head ast.ASTHandle, columns []ast.ASTHandle) (karrHeadPlan, error) {
	var out karrHeadPlan
	headArity, err := a.manager.GetNumArgs(head)
	if err != nil {
		return out, err
	}
	positions := make(map[ast.ASTHandle][]uint32, len(columns))
	for i, arg := range columns {
		kind, err := a.manager.GetKind(arg)
		if err != nil {
			return out, err
		}
		if kind == ast.ASTVar {
			positions[arg] = append(positions[arg], uint32(i))
		}
	}
	column := uint32(len(columns))
	selected := make([]uint32, headArity)
	used := make([]bool, column)
	// Entries in headConstants are added in head order; nil is a full
	// relation for an unbound variable. The map is used only for lookup.
	unbound := make(map[ast.ASTHandle]int)
	constantHeads := make([]int, headArity)
	for i := range constantHeads {
		constantHeads[i] = -1
	}
	for i := uint32(0); i < headArity; i++ {
		arg, err := a.manager.GetArg(head, i)
		if err != nil {
			return out, err
		}
		indexes := positions[arg]
		if len(indexes) == 0 {
			kind, err := a.manager.GetKind(arg)
			if err != nil {
				return out, err
			}
			if kind == ast.ASTVar {
				index, ok := unbound[arg]
				if !ok {
					index = len(out.headConstants)
					unbound[arg] = index
					out.headConstants = append(out.headConstants, nil)
				}
				constantHeads[i] = index
				continue
			}
			text, _, matched, err := a.manager.ArithmeticNumeralText(arg)
			if err != nil {
				return out, err
			}
			value, ok := new(big.Rat).SetString(text)
			if !matched || !ok {
				return out, ErrKarrDatalogFragment
			}
			constantHeads[i] = len(out.headConstants)
			out.headConstants = append(out.headConstants, value)
			continue
		}
		position := indexes[len(indexes)-1]
		if len(indexes) > 1 {
			positions[arg] = indexes[:len(indexes)-1]
		}
		used[position] = true
		selected[i] = position
	}
	// The source projection preserves tail order; identify each head column's
	// index in that projected relation, then decompose its permutation in cycles.
	projected := make([]uint32, column)
	next := uint32(0)
	for i := uint32(0); i < column; i++ {
		if !used[i] {
			out.removed = append(out.removed, i)
		} else {
			projected[i] = next
			next++
		}
	}
	permutation := make([]uint32, headArity)
	for i, position := range selected {
		if constantHeads[i] >= 0 {
			permutation[i] = next + uint32(constantHeads[i])
		} else {
			permutation[i] = projected[position]
		}
	}
	next += uint32(len(out.headConstants))
	usedHead := make(map[uint32]bool)
	for i, position := range permutation {
		if usedHead[position] {
			out.duplicates = append(out.duplicates, position)
			permutation[i] = next
			next++
		} else {
			usedHead[position] = true
		}
	}
	visited := make([]bool, headArity)
	for i := uint32(0); i < headArity; i++ {
		if visited[i] {
			continue
		}
		var cycle []uint32
		for j := i; !visited[j]; j = permutation[j] {
			visited[j] = true
			cycle = append(cycle, j)
		}
		if len(cycle) > 1 {
			out.cycles = append(out.cycles, cycle)
		}
	}
	return out, nil
}
