package core

// Source: src/tactic/core/solve_eqs_tactic.cpp sort_vars.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"io"
)

type solveEquationFrame struct {
	manager *ast.ASTManager
	refs    []ast.ASTRef
}

func (f *solveEquationFrame) pin(value ast.ASTHandle) error {
	if value.IsNull() {
		return nil
	}
	ref, err := f.manager.NewRef(value)
	if err != nil {
		return err
	}
	f.refs = append(f.refs, ref)
	return nil
}
func (f *solveEquationFrame) Close() (err error) {
	for i := len(f.refs) - 1; i >= 0; i-- {
		err = errors.Join(err, f.manager.DelRef(&f.refs[i]))
	}
	f.refs = nil
	return err
}
func (t *SolveEquationsTactic) children(term ast.ASTHandle) ([]ast.ASTHandle, error) {
	m := t.manager
	kind, err := m.GetKind(term)
	if err != nil {
		return nil, err
	}
	var children []ast.ASTHandle
	if kind == ast.ASTApp {
		count, e := m.GetNumArgs(term)
		if e != nil {
			return nil, e
		}
		for i := uint32(0); i < count; i++ {
			child, e := m.GetArg(term, i)
			if e != nil {
				return nil, e
			}
			children = append(children, child)
		}
	} else if kind == ast.ASTQuantifier {
		body, e := m.GetQuantifierBody(term)
		if e != nil {
			return nil, e
		}
		children = append(children, body)
		count, e := m.GetNumPatterns(term)
		if e != nil {
			return nil, e
		}
		for i := uint32(0); i < count; i++ {
			child, e := m.GetPattern(term, i)
			if e != nil {
				return nil, e
			}
			children = append(children, child)
		}
		count, e = m.GetNumNoPatterns(term)
		if e != nil {
			return nil, e
		}
		for i := uint32(0); i < count; i++ {
			child, e := m.GetNoPattern(term, i)
			if e != nil {
				return nil, e
			}
			children = append(children, child)
		}
	}
	return children, nil
}

func (t *SolveEquationsTactic) sortVariables() (err error) {
	if err = t.emit("solve_eqs_bug", "sort_vars", 438, func(out io.Writer) error {
		_, e := io.WriteString(out, "sorting vars...\n")
		return e
	}); err != nil {
		return err
	}
	t.ordered = nil
	saved := solveEquationFrame{manager: t.manager}
	defer func() { err = errors.Join(err, saved.Close()) }()
	visiting, done := make(map[ast.ASTHandle]bool), make(map[ast.ASTHandle]bool)
	type frame struct {
		term ast.ASTHandle
		next uint32
	}
	for _, v := range t.variables {
		if err = t.checkpoint(); err != nil {
			return err
		}
		if !t.candidateVars[v] {
			continue
		}
		todo := []frame{{term: v}}
		for len(todo) > 0 {
			last := len(todo) - 1
			term := todo[last].term
			t.steps++
			if err = t.traceExpression("solve_eqs_bug", "sort_vars", 466, "processing:\n", term); err != nil {
				return err
			}
			count, e := t.manager.GetRefCount(term)
			if e != nil {
				return e
			}
			if count > 1 && done[term] {
				todo = todo[:last]
				continue
			}
			kind, e := t.manager.GetKind(term)
			if e != nil {
				return e
			}
			if kind == ast.ASTVar {
				todo = todo[:last]
				continue
			}
			children, e := t.children(term)
			if e != nil {
				return e
			}
			if kind == ast.ASTApp && len(children) == 0 {
				if todo[last].next == 0 {
					if t.candidateVars[term] {
						if visiting[term] {
							delete(visiting, term)
							delete(t.candidateVars, term)
							definition, _, _, e := t.find(t.substitution, term)
							if e != nil {
								return e
							}
							if err = saved.pin(term); err != nil {
								return err
							}
							if err = saved.pin(definition); err != nil {
								return err
							}
							if err = t.manager.ExprSubstitutionErase(t.substitution, term); err != nil {
								return err
							}
						} else {
							visiting[term] = true
							todo[last].next = 1
							definition, _, _, e := t.find(t.substitution, term)
							if e != nil {
								return e
							}
							todo = append(todo, frame{term: definition})
							continue
						}
					}
				} else if t.candidateVars[term] {
					delete(visiting, term)
					t.ordered = append(t.ordered, term)
				}
			} else {
				pushed := false
				for todo[last].next < uint32(len(children)) {
					child := children[todo[last].next]
					todo[last].next++
					refs, e := t.manager.GetRefCount(child)
					if e != nil {
						return e
					}
					if refs > 1 && done[child] {
						continue
					}
					todo = append(todo, frame{term: child})
					pushed = true
					break
				}
				if pushed {
					continue
				}
			}
			if count > 1 {
				done[term] = true
			}
			todo = todo[:last]
		}
	}
	for i, v := range t.variables {
		if !t.candidateVars[v] {
			delete(t.candidateSet, t.candidates[i])
		}
	}
	if err = t.traceVariables("sort_vars", 569, "ordered vars:\n", t.ordered); err != nil {
		return err
	}
	clear(t.candidateVars)
	return nil
}
