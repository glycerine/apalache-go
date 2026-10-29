package qe

// Source: src/qe/qe_lite.cpp fm::fm candidate ordering and bound analysis.

import (
	"io"
	"sort"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
)

func fmCost(lowers, uppers uint64) uint32 {
	if lowers != 0 && uppers > uint64(^uint32(0))/lowers {
		return ^uint32(0)
	}
	return uint32(lowers * uppers)
}

func (s *fmStore) candidates() (result []uint32, err error) {
	scope, err := s.begin()
	if err != nil {
		return nil, err
	}
	defer s.finish(scope, &err)
	type candidate struct{ index, cost uint32 }
	var rows []candidate
	for i, v := range s.variables {
		if !v.forbidden {
			rows = append(rows, candidate{uint32(i), fmCost(uint64(len(v.lowers)), uint64(len(v.uppers)))})
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.cost == 0 {
			if b.cost > 0 {
				return true
			}
			return a.index < b.index
		}
		if b.cost == 0 {
			return false
		}
		ai, bi := s.variables[a.index].integer, s.variables[b.index].integer
		return (!ai && bi) || (ai == bi && a.cost < b.cost)
	})
	if err = s.recognizer.emit("sort_candidates", 1878, func(out io.Writer) error {
		for _, row := range rows {
			h, err := s.recognizer.manager.RefGet(s.variables[row.index].expression)
			if err != nil {
				return err
			}
			_, _ = io.WriteString(out, "(")
			if err = writeExpression(s.recognizer.manager, s.recognizer.config, out, h); err != nil {
				return err
			}
			_, _ = io.WriteString(out, " "+strconv.FormatUint(uint64(row.cost), 10)+") ")
		}
		_, _ = io.WriteString(out, "\n")
		return nil
	}); err != nil {
		return nil, err
	}
	for _, row := range rows {
		result = append(result, row.index)
	}
	return result, nil
}

func fmLiveConstraints(cs []*fmConstraint) []*fmConstraint {
	j := 0
	for _, c := range cs {
		if !c.dead {
			cs[j] = c
			j++
		}
	}
	return cs[:j]
}

func (s *fmStore) analyze(cs []*fmConstraint, x uint32) (allInt, unit bool, err error) {
	allInt, unit = true, true
	n := s.recognizer.manager.NumeralService()
	for _, c := range cs {
		for i, index := range c.variables {
			if !s.variables[index].integer {
				return false, unit, nil
			}
			if index == x {
				one, err := n.IsOne(&c.coefficients[i])
				if err != nil {
					return false, false, err
				}
				minusOne, err := n.IsMinusOne(&c.coefficients[i])
				if err != nil {
					return false, false, err
				}
				if !one && !minusOne {
					unit = false
				}
			}
		}
	}
	return allInt, unit, nil
}
func (s *fmStore) canEliminate(x uint32) (bool, error) {
	if uint64(x) >= uint64(len(s.variables)) {
		return false, ast.ErrASTDomain
	}
	v := &s.variables[x]
	if !v.integer {
		return true, nil
	}
	allInt, lower, err := s.analyze(v.lowers, x)
	if err != nil || !allInt {
		return false, err
	}
	allInt, upper, err := s.analyze(v.uppers, x)
	return allInt && (lower || upper), err
}
func (s *fmStore) markDead(x uint32) {
	for _, c := range s.variables[x].lowers {
		c.dead = true
	}
	for _, c := range s.variables[x].uppers {
		c.dead = true
	}
}
