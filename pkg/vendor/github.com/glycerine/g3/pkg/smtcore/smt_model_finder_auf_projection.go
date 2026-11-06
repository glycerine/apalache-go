package smtcore

// Source: src/smt/smt_model_finder.cpp get_instantiation_set_values,
// sort_values and mk_mono_proj.

import (
	"errors"
	"math/big"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type smtModelFinderProjectionValue struct {
	value   ast.ASTHandle
	numeral *big.Rat
	id      uint32
}

func (g *smtModelFinderAUFGraph) instantiationValues(node *smtModelFinderAUFNode) ([]ast.ASTHandle, error) {
	seen := make(map[ast.ASTHandle]bool)
	var values []ast.ASTHandle
	for term := range node.root().set.elements {
		value, err := g.eval(term)
		if err != nil {
			return nil, err
		}
		if !seen[value] {
			seen[value] = true
			values = append(values, value)
		}
	}
	return values, nil
}

func (g *smtModelFinderAUFGraph) sortedProjectionValues(node *smtModelFinderAUFNode) ([]ast.ASTHandle, error) {
	values, err := g.instantiationValues(node)
	if err != nil {
		return nil, err
	}
	m := g.finder.manager
	bv, err := m.IsBitVectorSort(node.sort)
	if err != nil {
		return nil, err
	}
	var width uint32
	if bv && node.root().signed {
		width, err = m.BitVectorSize(node.sort)
		if err != nil {
			return nil, err
		}
	}
	keys := make([]smtModelFinderProjectionValue, len(values))
	for i, value := range values {
		id, err := m.GetASTID(value)
		if err != nil {
			return nil, err
		}
		var text string
		var numeral bool
		if bv {
			text, numeral, err = m.BitVectorNumeralText(value)
		} else {
			text, _, numeral, err = m.ArithmeticNumeralText(value)
		}
		if err != nil {
			return nil, err
		}
		key := smtModelFinderProjectionValue{value: value, id: id}
		if numeral {
			key.numeral = new(big.Rat)
			if _, ok := key.numeral.SetString(text); !ok {
				return nil, ast.ErrASTDomain
			}
			if bv && node.root().signed {
				modulus := new(big.Int).Lsh(big.NewInt(1), uint(width))
				signBit := new(big.Int).Rsh(new(big.Int).Set(modulus), 1)
				if key.numeral.Num().Cmp(signBit) >= 0 {
					key.numeral.Sub(key.numeral, new(big.Rat).SetInt(modulus))
				}
			}
		}
		keys[i] = key
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].numeral != nil && keys[j].numeral != nil {
			return keys[i].numeral.Cmp(keys[j].numeral) < 0
		}
		return keys[i].id < keys[j].id
	})
	for i := range keys {
		values[i] = keys[i].value
	}
	return values, nil
}

func (g *smtModelFinderAUFGraph) makeMonotoneProjection(node *smtModelFinderAUFNode) error {
	if node == nil || node.parent != nil || !node.mono || node.set == nil || g.model == nil {
		return ast.ErrASTDomain
	}
	values, err := g.sortedProjectionValues(node)
	if err != nil || len(values) == 0 {
		return err
	}
	m := g.finder.manager
	bv, err := m.IsBitVectorSort(node.sort)
	if err != nil {
		return err
	}
	variable, err := m.MkVar(0, node.sort)
	if err != nil {
		return err
	}
	definition := values[len(values)-1]
	for i := len(values) - 1; i >= 1; i-- {
		var condition ast.ASTHandle
		if bv {
			if node.signed {
				condition, err = m.MkBitVectorSLE(values[i], variable)
			} else {
				condition, err = m.MkBitVectorULE(values[i], variable)
			}
			if err == nil {
				condition, err = m.MkNot(condition)
			}
		} else {
			family, e := m.ArithmeticFamilyID()
			if e != nil {
				return e
			}
			condition, err = m.MkFamilyApp(family, 4, nil, []ast.ASTHandle{variable, values[i]})
		}
		if err != nil {
			return err
		}
		definition, err = m.MkIte(condition, values[i-1], definition)
		if err != nil {
			return err
		}
	}
	function, err := m.MkFreshFuncDecl(symbol.Symbol{}, symbol.Symbol{}, []ast.ASTHandle{node.sort}, node.sort)
	if err != nil {
		return err
	}
	interpretation, err := model.NewFunctionInterpretation(m, 1)
	if err != nil {
		return err
	}
	if err := interpretation.SetElse(definition); err != nil {
		return errors.Join(err, interpretation.Close())
	}
	if err := g.model.RegisterAuxFunction(function, interpretation); err != nil {
		return errors.Join(err, interpretation.Close())
	}
	return node.setProjection(function)
}
