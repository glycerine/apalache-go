package tactic

// Source: src/tactic/sine_filter.cpp; src/tactic/sine_filter.h.

import (
	"errors"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type sineTactic struct {
	self       *sineTactic
	manager    *ast.ASTManager
	references uint32
	closed     bool
}

func NewSineTactic(manager *ast.ASTManager, _ *uparams.ParamsRef) (Tactic, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	if _, err := manager.ProofsEnabled(); err != nil {
		return nil, err
	}
	t := &sineTactic{manager: manager}
	t.self = t
	return t, nil
}

func (t *sineTactic) open() error {
	if t == nil || t.self != t || t.closed {
		return arena.ErrClosed
	}
	return nil
}
func (t *sineTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.references++
	return nil
}
func (t *sineTactic) DecRef() error {
	if err := t.open(); err != nil {
		return err
	}
	if t.references == 0 {
		return arena.ErrBounds
	}
	t.references--
	if t.references == 0 {
		t.closed = true
		t.manager = nil
	}
	return nil
}

type sineWorkItem struct{ expression, root ast.ASTHandle }

func (t *sineTactic) quantifierMatches(q ast.ASTHandle, constants map[ast.ASTHandle]bool) (bool, []ast.ASTHandle, error) {
	n, err := t.manager.GetNumPatterns(q)
	if err != nil {
		return false, nil, err
	}
	var next []ast.ASTHandle
	for i := uint32(0); i < n; i++ {
		pattern, err := t.manager.GetPattern(q, i)
		if err != nil {
			return false, nil, err
		}
		isPattern, err := t.manager.IsPattern(pattern)
		if err != nil {
			return false, nil, err
		}
		if !isPattern {
			continue
		}
		args, err := t.manager.GetNumArgs(pattern)
		if err != nil {
			return false, nil, err
		}
		stack := make([]ast.ASTHandle, 0, args)
		for j := uint32(0); j < args; j++ {
			h, err := t.manager.GetArg(pattern, j)
			if err != nil {
				return false, nil, err
			}
			stack = append(stack, h)
		}
		matched := true
		for len(stack) != 0 {
			last := len(stack) - 1
			current := stack[last]
			stack = stack[:last]
			kind, err := t.manager.GetKind(current)
			if err != nil {
				return false, nil, err
			}
			if kind != ast.ASTApp {
				continue
			}
			declaration, err := t.manager.GetDecl(current)
			if err != nil {
				return false, nil, err
			}
			if !constants[declaration] {
				matched = false
				next = append(next, declaration)
				break
			}
			count, err := t.manager.GetNumArgs(current)
			if err != nil {
				return false, nil, err
			}
			for j := uint32(0); j < count; j++ {
				arg, err := t.manager.GetArg(current, j)
				if err != nil {
					return false, nil, err
				}
				stack = append(stack, arg)
			}
		}
		if matched {
			return true, next, nil
		}
	}
	return false, next, nil
}

func (t *sineTactic) filterExpressions(g *Goal) ([]ast.ASTHandle, error) {
	constantExpressions := make(map[ast.ASTHandle]map[ast.ASTHandle]bool)
	expressionConstants := make(map[ast.ASTHandle]map[ast.ASTHandle]bool)
	waiting := make(map[ast.ASTHandle]map[sineWorkItem]bool)
	constants := make(map[ast.ASTHandle]bool)
	stack := make([]sineWorkItem, 0, len(g.forms))
	for _, form := range g.forms {
		stack = append(stack, sineWorkItem{form, form})
	}
	for len(stack) != 0 {
		last := len(stack) - 1
		item := stack[last]
		stack = stack[:last]
		kind, err := t.manager.GetKind(item.expression)
		if err != nil {
			return nil, err
		}
		switch kind {
		case ast.ASTApp:
			declaration, err := t.manager.GetDecl(item.expression)
			if err != nil {
				return nil, err
			}
			family, err := t.manager.GetFamilyID(declaration)
			if err != nil {
				return nil, err
			}
			if family != ast.NullFamilyID {
				continue
			}
			if !constants[declaration] {
				constants[declaration] = true
				for pending := range waiting[declaration] {
					stack = append(stack, pending)
				}
				delete(waiting, declaration)
			}
			if constantExpressions[declaration] == nil {
				constantExpressions[declaration] = make(map[ast.ASTHandle]bool)
			}
			constantExpressions[declaration][item.root] = true
			if expressionConstants[item.root] == nil {
				expressionConstants[item.root] = make(map[ast.ASTHandle]bool)
			}
			expressionConstants[item.root][declaration] = true
			count, err := t.manager.GetNumArgs(item.expression)
			if err != nil {
				return nil, err
			}
			for i := uint32(0); i < count; i++ {
				arg, err := t.manager.GetArg(item.expression, i)
				if err != nil {
					return nil, err
				}
				stack = append(stack, sineWorkItem{arg, item.root})
			}
		case ast.ASTQuantifier:
			forall, err := t.manager.IsForall(item.expression)
			if err != nil {
				return nil, err
			}
			body, err := t.manager.GetQuantifierBody(item.expression)
			if err != nil {
				return nil, err
			}
			patterns, err := t.manager.GetNumPatterns(item.expression)
			if err != nil {
				return nil, err
			}
			if !forall || patterns == 0 {
				stack = append(stack, sineWorkItem{body, item.root})
				continue
			}
			matched, next, err := t.quantifierMatches(item.expression, constants)
			if err != nil {
				return nil, err
			}
			if matched {
				stack = append(stack, sineWorkItem{body, item.root})
				continue
			}
			for _, declaration := range next {
				if waiting[declaration] == nil {
					waiting[declaration] = make(map[sineWorkItem]bool)
				}
				waiting[declaration][item] = true
			}
		}
	}
	if len(g.forms) == 0 {
		return nil, ast.ErrASTDomain
	}
	visited := make(map[ast.ASTHandle]bool)
	toVisit := []ast.ASTHandle{g.forms[len(g.forms)-1]}
	for len(toVisit) != 0 {
		last := len(toVisit) - 1
		current := toVisit[last]
		toVisit = toVisit[:last]
		visited[current] = true
		for declaration := range expressionConstants[current] {
			for expression := range constantExpressions[declaration] {
				if !visited[expression] {
					toVisit = append(toVisit, expression)
				}
			}
		}
	}
	result := make([]ast.ASTHandle, 0, len(visited))
	for _, form := range g.forms {
		if visited[form] {
			result = append(result, form)
		}
	}
	return result, nil
}

func (t *sineTactic) trace(line int, body func(io.Writer) error) error {
	tags := t.manager.BorrowMemoryTraceTags()
	if !tags.ShouldTrace("sine") {
		return nil
	}
	return tags.EmitConditional("sine", "operator()", "src/tactic/sine_filter.cpp", line, func() bool { return true }, body)
}

func (t *sineTactic) Apply(g *Goal, out *TacticResult) (result error) {
	if err := t.open(); err != nil {
		return err
	}
	if g == nil || out == nil || g.manager != t.manager {
		return ast.ErrASTHandle
	}
	if err := out.prepare(t.manager); err != nil {
		return err
	}
	if err := t.trace(58, g.Display); err != nil {
		return err
	}
	if err := t.trace(59, func(w io.Writer) error { _, e := io.WriteString(w, strconv.Itoa(len(g.forms))); return e }); err != nil {
		return err
	}
	selected, err := t.filterExpressions(g)
	if err != nil {
		return err
	}
	if err = t.trace(62, func(w io.Writer) error { _, e := io.WriteString(w, strconv.Itoa(len(selected))); return e }); err != nil {
		return err
	}
	owned := 0
	defer func() {
		for owned != 0 {
			owned--
			result = errors.Join(result, t.manager.DecRef(selected[owned]))
		}
	}()
	for _, form := range selected {
		if err = t.manager.IncRef(form); err != nil {
			return err
		}
		owned++
	}
	if err = g.Reset(); err != nil {
		return err
	}
	for _, form := range selected {
		if err = g.AssertWithProof(form, ast.ASTHandle{}, ast.ASTDependency{}); err != nil {
			return err
		}
	}
	if err = g.IncDepth(); err != nil {
		return err
	}
	if err = g.UpdatePrecision(GoalOver); err != nil {
		return err
	}
	if err = out.AddGoal(g); err != nil {
		return err
	}
	if err = t.trace(70, g.Display); err != nil {
		return err
	}
	if tacticSourceDebug {
		wellSorted, e := g.IsWellSorted()
		if e != nil {
			return e
		}
		if !wellSorted {
			return ast.ErrASTDomain
		}
	}
	converter, err := NewFilterModel(t.manager)
	if err != nil {
		return err
	}
	if err = out.SetModelConverter(converter); err != nil {
		return errors.Join(err, converter.Close())
	}
	return nil
}

func (t *sineTactic) UpdateParams(*uparams.ParamsRef) error         { return t.open() }
func (t *sineTactic) CollectParamDescrs(*uparams.ParamDescrs) error { return t.open() }
func (t *sineTactic) CollectStatistics() map[string]float64         { return nil }
func (t *sineTactic) ResetStatistics() error                        { return t.open() }
func (t *sineTactic) Cleanup() error                                { return t.open() }
func (t *sineTactic) Reset() error                                  { return t.open() }
func (t *sineTactic) SetLogic(usymbol.Symbol) error                 { return t.open() }
func (t *sineTactic) SetProgressCallback(func(uint32)) error        { return t.open() }
func (t *sineTactic) Translate(manager *ast.ASTManager) (Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return NewSineTactic(manager, nil)
}
