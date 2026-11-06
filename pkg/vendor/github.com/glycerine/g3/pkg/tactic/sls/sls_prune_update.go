package sls

// Source: src/tactic/sls/sls_evaluator.h:668-795.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// UpdatePrune propagates non-Boolean values, then updates Boolean scores only
// as far as the source occurrence test permits. Boolean values are not
// committed by a speculative probe.
func (e *Evaluator) UpdatePrune(search *SearchTracker, decl ast.ASTHandle, value *numeral.Int) (benefit bool, result error) {
	if err := e.open(); err != nil {
		return false, err
	}
	if search == nil || search.open() != nil || search.base != e.tracker {
		return false, ErrSLSSearch
	}
	if err := e.tracker.SetDeclValue(decl, value); err != nil {
		return false, err
	}
	entry, err := e.tracker.EntryPoint(decl)
	if err != nil {
		return false, err
	}
	maxDepth, err := e.tracker.Distance(entry)
	if err != nil {
		return false, err
	}
	boolean, err := e.manager.MkBoolSort()
	if err != nil {
		return false, err
	}
	sort, err := e.manager.GetSort(entry)
	if err != nil {
		return false, err
	}
	values := make([][]ast.ASTHandle, int(maxDepth)+1)
	scores := make([][]ast.ASTHandle, int(maxDepth)+1)
	if sort == boolean {
		scores[maxDepth] = append(scores[maxDepth], entry)
	} else {
		values[maxDepth] = append(values[maxDepth], entry)
		visited := make(map[ast.ASTHandle]struct{})
		temp := numeral.NewInt(0)
		defer func() { result = errors.Join(result, e.numbers.Del(&temp)) }()
		for depth := int(maxDepth); depth >= 0; depth-- {
			for _, node := range values[depth] {
				if err = e.Evaluate(node, &temp); err != nil {
					return false, err
				}
				if err = e.tracker.SetValue(node, &temp); err != nil {
					return false, err
				}
				for _, parent := range e.tracker.uplinks[node] {
					if _, found := visited[parent]; found {
						continue
					}
					parentDepth, err := e.tracker.Distance(parent)
					if err != nil {
						return false, err
					}
					if parentDepth >= uint32(depth) {
						return false, ErrSLSSearch
					}
					parentSort, err := e.manager.GetSort(parent)
					if err != nil {
						return false, err
					}
					if parentSort == boolean {
						scores[maxDepth] = append(scores[maxDepth], parent)
					} else {
						values[parentDepth] = append(values[parentDepth], parent)
					}
					visited[parent] = struct{}{}
				}
			}
		}
	}
	visited := make(map[ast.ASTHandle]struct{})
	for depth := int(maxDepth); depth >= 0; depth-- {
		if depth != int(maxDepth) && !benefit {
			continue
		}
		for _, node := range scores[depth] {
			newScore, err := search.Score(node)
			if err != nil {
				return false, err
			}
			oldScore, err := e.tracker.Score(node)
			if err != nil {
				return false, err
			}
			isTop, err := e.tracker.IsTopExpr(node)
			if err != nil {
				return false, err
			}
			if isTop {
				if err = e.tracker.AdaptTopSum(node, newScore, oldScore); err != nil {
					return false, err
				}
			}
			if depth == int(maxDepth) {
				record, err := e.tracker.existingScore(node)
				if err != nil {
					return false, err
				}
				benefit = benefit || newScore > record.scorePrune && record.hasPosOcc != 0 || newScore <= record.scorePrune && record.hasNegOcc != 0
			}
			if err = e.tracker.SetScore(node, newScore); err != nil {
				return false, err
			}
			for _, parent := range e.tracker.uplinks[node] {
				if _, found := visited[parent]; found {
					continue
				}
				parentDepth, err := e.tracker.Distance(parent)
				if err != nil {
					return false, err
				}
				if parentDepth >= uint32(depth) {
					return false, ErrSLSSearch
				}
				scores[parentDepth] = append(scores[parentDepth], parent)
				visited[parent] = struct{}{}
			}
		}
	}
	return benefit, nil
}
