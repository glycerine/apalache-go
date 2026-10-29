package arith

// Source: src/tactic/arith/diff_neq_tactic.cpp concrete int search.

import (
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func diffNeqIntText(n int32) string { return strconv.FormatInt(int64(n), 10) }
func (t *DiffNeqTactic) initDiffNeqForbidden() error {
	imp := &t.current
	max := int32(0)
	for _, v := range imp.variables {
		if v.upper > max {
			max = v.upper
		}
	}
	size := int(max) + 1
	if size > cap(imp.forbidden) {
		if err := t.manager.Budget().Reserve(uint64(size)); err != nil {
			return err
		}
		values := make([]bool, size)
		t.manager.Budget().Release(imp.forbiddenBytes)
		imp.forbidden, imp.forbiddenBytes = values, uint64(size)
	} else {
		imp.forbidden = imp.forbidden[:size]
		clear(imp.forbidden)
	}
	return nil
}
func (t *DiffNeqTactic) chooseDiffNeqValue(x uint32, starting int32) (int32, error) {
	imp := &t.current
	if int(x) >= len(imp.variables) {
		return -1, ast.ErrASTDomain
	}
	max, v, upper := starting-1, starting, imp.variables[x].upper
	if starting > upper {
		return -1, nil
	}
	for _, d := range imp.variables[x].disequalities {
		if int(d.variable) >= len(imp.stack) {
			return -1, ast.ErrASTDomain
		}
		bad := imp.stack[d.variable] + d.offset
		if bad < v || bad > upper {
			continue
		}
		if bad == v {
			for {
				v++
				if v > upper {
					return -1, nil
				}
				if int(v) >= len(imp.forbidden) || v < 0 {
					return -1, ast.ErrASTDomain
				}
				if !imp.forbidden[v] {
					break
				}
				imp.forbidden[v] = false
			}
			continue
		}
		if bad <= v || bad < 0 || int(bad) >= len(imp.forbidden) {
			return -1, ast.ErrASTDomain
		}
		imp.forbidden[bad] = true
		if bad > max {
			max = bad
		}
	}
	for i := starting + 1; i <= max; i++ {
		imp.forbidden[i] = false
	}
	if ast.ASTSourceDebug {
		for _, marked := range imp.forbidden {
			if marked {
				return -1, ast.ErrASTDomain
			}
		}
	}
	return v, nil
}
func (t *DiffNeqTactic) resolveDiffNeqConflict() (bool, error) {
	imp := &t.current
	imp.conflicts++
	for len(imp.stack) > 0 {
		last := imp.stack[len(imp.stack)-1]
		imp.stack = imp.stack[:len(imp.stack)-1]
		value, err := t.chooseDiffNeqValue(uint32(len(imp.stack)), last+1)
		if err != nil {
			return false, err
		}
		if value != -1 {
			imp.stack = append(imp.stack, value)
			return true, nil
		}
	}
	return false, nil
}
func (t *DiffNeqTactic) searchDiffNeq() (bool, error) {
	imp := &t.current
	imp.conflicts = 0
	if err := t.initDiffNeqForbidden(); err != nil {
		return false, err
	}
	for len(imp.stack) < len(imp.variables) {
		canceled, err := t.manager.Canceled()
		if err != nil {
			return false, err
		}
		if canceled {
			return false, tactic.NewTacticException(t.manager.ResourceLimit().CancelMessage())
		}
		if err = t.traceDiffNeqAssignment(293); err != nil {
			return false, err
		}
		value, err := t.chooseDiffNeqValue(uint32(len(imp.stack)), 0)
		if err != nil {
			return false, err
		}
		if value != -1 {
			imp.stack = append(imp.stack, value)
			continue
		}
		resolved, err := t.resolveDiffNeqConflict()
		if err != nil || !resolved {
			return false, err
		}
	}
	if err := t.traceDiffNeqAssignment(300); err != nil {
		return false, err
	}
	return true, nil
}
