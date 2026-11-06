package qe

// Source: src/qe/qe.cpp, quant_elim_plugin::find_min_weight.

import (
	"errors"
	"math"
	"math/big"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func exprQEAtomHandles(m *ast.ASTManager, entries map[ast.ASTHandle]ast.ASTRef) ([]ast.ASTHandle, error) {
	type item struct {
		h  ast.ASTHandle
		id uint32
	}
	items := make([]item, 0, len(entries))
	for h := range entries {
		id, err := m.GetASTID(h)
		if err != nil {
			return nil, err
		}
		items = append(items, item{h, id})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].id < items[j].id })
	values := make([]ast.ASTHandle, len(items))
	for i := range items {
		values[i] = items[i].h
	}
	return values, nil
}

func exprQEPluginWeight(plugin SolverPlugin, variable *ast.ContainsApp, formula ast.ASTHandle, atoms *BoolAtomIndex) (uint32, error) {
	switch p := plugin.(type) {
	case *BoolPlugin:
		return p.Weight(variable, atoms)
	case *BitVectorPlugin:
		return p.Weight(variable, formula)
	case *ArithmeticPlugin:
		return p.Weight(variable, formula)
	case *NonlinearArithmeticPlugin:
		return p.Weight(variable, formula)
	default:
		return math.MaxUint32, nil
	}
}

func exprQEPluginBranches(plugin SolverPlugin, variable *ast.ContainsApp, formula ast.ASTHandle, positive, negative []ast.ASTHandle, out *numeral.OwnedRational) (bool, error) {
	switch p := plugin.(type) {
	case *BoolPlugin:
		return p.GetNumBranches(variable, formula, out)
	case *BitVectorPlugin:
		return p.GetNumBranches(variable, formula, out)
	case *DatatypeSolverPlugin:
		ok, err := p.GetNumBranches(variable, formula, out)
		if err != nil || ok {
			return ok, err
		}
		ok, err = p.Prepare(variable, formula, positive, negative)
		if err != nil || !ok {
			return false, err
		}
		return p.GetNumBranches(variable, formula, out)
	case *ArithmeticPlugin:
		ok, err := p.Prepare(variable, formula, positive, negative)
		if err != nil || !ok {
			return false, err
		}
		return p.GetNumBranches(variable, formula, out)
	case *NonlinearArithmeticPlugin:
		ok, err := p.Prepare(variable, formula, positive, negative)
		if err != nil || !ok {
			return false, err
		}
		return p.GetNumBranches(variable, formula, out)
	case *DatalogPlugin:
		ok, err := p.Prepare(variable, formula, positive, negative)
		if err != nil || !ok {
			return false, err
		}
		return p.GetNumBranches(variable, formula, out)
	default:
		return false, nil
	}
}

// selectExprQEVariable borrows plugins and appends owned unsupported variables
// to freeVariables. It preserves the source scan's last eligible index.
func selectExprQEVariable(node *exprQETree, variables *exprQEBranchVariables, context *SolverContext, atoms *BoolAtomIndex, freeVariables *[]ast.ASTRef) (selected ast.ASTHandle, count big.Rat, found bool, err error) {
	if node == nil || node.manager == nil || variables == nil || variables.manager != node.manager || context == nil || atoms == nil || freeVariables == nil || node.selected != (ast.ASTRef{}) {
		return selected, count, false, ast.ErrASTHandle
	}
	m := node.manager
	if atoms.manager != m {
		return selected, count, false, ast.ErrASTHandle
	}
	positive, err := exprQEAtomHandles(m, atoms.positiveAtoms())
	if err != nil {
		return selected, count, false, err
	}
	negative, err := exprQEAtomHandles(m, atoms.negativeAtoms())
	if err != nil {
		return selected, count, false, err
	}
	formula, err := m.RefGet(node.formula)
	if err != nil {
		return selected, count, false, err
	}
	rational, err := m.NumeralService().NewValue()
	if err != nil {
		return selected, count, false, err
	}
	defer func() { err = errors.Join(err, m.NumeralService().Del(&rational)) }()
	for len(node.variables) > 0 {
		index := 0
		for i, ref := range node.variables {
			v, e := m.RefGet(ref)
			if e != nil {
				return selected, count, false, e
			}
			has, e := context.HasPlugin(v)
			if e != nil {
				return selected, count, false, e
			}
			if !has {
				break
			}
			plugin, e := context.PluginFor(v)
			if e != nil {
				return selected, count, false, e
			}
			entry, ok := variables.index[v]
			if !ok {
				return selected, count, false, ast.ErrASTDomain
			}
			weight, e := exprQEPluginWeight(plugin, variables.entries[entry].contains, formula, atoms)
			if e != nil {
				return selected, count, false, e
			}
			if weight < math.MaxUint32 {
				index = i
			}
		}
		v, e := m.RefGet(node.variables[index])
		if e != nil {
			return selected, count, false, e
		}
		has, e := context.HasPlugin(v)
		if e != nil {
			return selected, count, false, e
		}
		if has {
			plugin, e := context.PluginFor(v)
			if e != nil {
				return selected, count, false, e
			}
			entry, ok := variables.index[v]
			if !ok {
				return selected, count, false, ast.ErrASTDomain
			}
			ok, e = exprQEPluginBranches(plugin, variables.entries[entry].contains, formula, positive, negative, &rational)
			if e != nil {
				return selected, count, false, e
			}
			if ok {
				text, e := m.NumeralService().String(&rational)
				if e != nil {
					return selected, count, false, e
				}
				if _, ok = count.SetString(text); !ok {
					return selected, count, false, ast.ErrASTDomain
				}
				return v, count, true, nil
			}
		}
		ref, e := m.NewRef(v)
		if e != nil {
			return selected, count, false, e
		}
		if e = node.deleteVariable(v); e != nil {
			return selected, count, false, errors.Join(e, m.DelRef(&ref))
		}
		*freeVariables = append(*freeVariables, ref)
	}
	return selected, count, false, nil
}
