package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"strings"
)

// HistoryProposer owns conjectures translated from an old unwinding into a
// new rule graph. The new graph/context are borrowed for subsequent proposals.
// Source: duality_solver.cpp:3156-3258.
type HistoryProposer struct {
	graph       *RPFP
	conjectures map[*Node][]*Transformer
	closed      bool
}

func historyKey(context *Context, decl ast.ASTHandle) (string, error) {
	name, err := context.manager.GetASTName(decl)
	if err != nil {
		return "", err
	}
	symbol, err := newSymbol(context, name)
	if err != nil {
		return "", err
	}
	text, err := symbol.String()
	if err != nil {
		return "", err
	}
	key, _, _ := strings.Cut(text, "@@")
	return key, nil
}
func NewHistoryProposer(old, newGraph *RPFP) (out *HistoryProposer, result error) {
	if err := old.open(); err != nil {
		return nil, err
	}
	if err := newGraph.open(); err != nil {
		return nil, err
	}
	proposer := &HistoryProposer{graph: newGraph, conjectures: make(map[*Node][]*Transformer)}
	m := newGraph.context.manager
	var translation *ast.ASTTranslation
	var pins []ast.ASTRef
	defer func() {
		if translation != nil {
			result = errors.Join(result, translation.Close())
		}
		for i := range pins {
			result = errors.Join(result, m.DelRef(&pins[i]))
		}
		if result != nil {
			result = errors.Join(result, proposer.Close())
			out = nil
		}
	}()
	if old.context.manager != m {
		var err error
		translation, err = ast.NewASTTranslation(old.context.manager, m, false)
		if err != nil {
			return nil, err
		}
	}
	translate := func(h ast.ASTHandle) (ast.ASTHandle, error) {
		var err error
		if translation != nil {
			h, err = translation.Translate(h)
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		ref, err := m.NewRef(h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		pins = append(pins, ref)
		return h, nil
	}
	newConstants := make(map[string]ast.ASTHandle)
	for h := range newGraph.vocabulary.constants {
		key, err := historyKey(newGraph.context, h)
		if err != nil {
			return nil, err
		}
		newConstants[key] = h
	}
	background := make(map[ast.ASTHandle]ast.ASTHandle)
	for h := range old.vocabulary.constants {
		translated, err := translate(h)
		if err != nil {
			return nil, err
		}
		key, err := historyKey(newGraph.context, translated)
		if err != nil {
			return nil, err
		}
		if matched, found := newConstants[key]; found {
			background[translated] = matched
		}
	}
	matches := make(map[string][]*Node)
	for _, node := range old.nodes {
		decl, err := old.context.manager.RefGet(node.Name)
		if err != nil {
			return nil, err
		}
		key, err := historyKey(old.context, decl)
		if err != nil {
			return nil, err
		}
		matches[key] = append(matches[key], node)
	}
	for _, node := range newGraph.nodes {
		decl, err := m.RefGet(node.Name)
		if err != nil {
			return nil, err
		}
		key, err := historyKey(newGraph.context, decl)
		if err != nil {
			return nil, err
		}
		for _, previous := range matches[key] {
			full, err := previous.Annotation.IsFull()
			if err != nil {
				return nil, err
			}
			if full {
				continue
			}
			parameters := make(map[string]ast.ASTHandle)
			for _, ref := range node.Annotation.individualParams {
				h, err := m.RefGet(ref)
				if err != nil {
					return nil, err
				}
				decl, err := m.GetDecl(h)
				if err != nil {
					return nil, err
				}
				key, err := historyKey(newGraph.context, decl)
				if err != nil {
					return nil, err
				}
				parameters[key] = h
			}
			substitutions := make(map[ast.ASTHandle]ast.ASTHandle)
			complete := true
			for _, ref := range previous.Annotation.individualParams {
				original, err := old.context.manager.RefGet(ref)
				if err != nil {
					return nil, err
				}
				translated, err := translate(original)
				if err != nil {
					return nil, err
				}
				decl, err := m.GetDecl(translated)
				if err != nil {
					return nil, err
				}
				key, err := historyKey(newGraph.context, decl)
				if err != nil {
					return nil, err
				}
				replacement, found := parameters[key]
				if !found {
					complete = false
					break
				}
				// Preserve source SubstParams(old.IndParams,...): keys are the original
				// terms, even though matching uses translated terms. Across managers
				// these keys do not match the translated formula's parameter nodes.
				substitutions[original] = replacement
			}
			if !complete {
				continue
			}
			previousFormula, err := old.context.manager.RefGet(previous.Annotation.formula)
			if err != nil {
				return nil, err
			}
			formula, err := translate(previousFormula)
			if err != nil {
				return nil, err
			}
			replaced, err := substituteExpression(newGraph.context, formula, substitutions)
			if err != nil {
				return nil, err
			}
			pins = append(pins, replaced)
			formula, err = m.RefGet(replaced)
			if err != nil {
				return nil, err
			}
			replaced, err = substituteExpressionWithDeclarations(newGraph.context, formula, make(map[ast.ASTHandle]ast.ASTHandle), background)
			if err != nil {
				return nil, err
			}
			pins = append(pins, replaced)
			formula, err = m.RefGet(replaced)
			if err != nil {
				return nil, err
			}
			fact, err := node.Annotation.Clone()
			if err != nil {
				return nil, err
			}
			proposer.conjectures[node] = append(proposer.conjectures[node], fact)
			if err = m.SetRef(fact.formula, formula); err != nil {
				return nil, err
			}
		}
	}
	return proposer, nil
}

// Propose returns owning copies; callers may use them as mutable local bounds.
func (p *HistoryProposer) Propose(node *Node) (out []*Transformer, result error) {
	if p == nil || p.closed {
		return nil, ErrWrapperOwner
	}
	if err := p.graph.open(); err != nil {
		return nil, err
	}
	if node == nil || node.closed || node.Owner.context != p.graph.context {
		return nil, ErrWrapperOwner
	}
	var owned []*Transformer
	defer func() {
		if result != nil {
			for _, fact := range owned {
				result = errors.Join(result, fact.Close())
			}
			out = nil
		}
	}()
	for _, fact := range p.conjectures[node.Map] {
		copy, err := fact.Clone()
		if err != nil {
			return nil, err
		}
		owned = append(owned, copy)
	}
	return owned, nil
}
func (p *HistoryProposer) Close() error {
	if p == nil || p.closed {
		return nil
	}
	for node, facts := range p.conjectures {
		for len(facts) > 0 {
			n := len(facts) - 1
			if err := facts[n].Close(); err != nil {
				p.conjectures[node] = facts
				return err
			}
			facts = facts[:n]
		}
		delete(p.conjectures, node)
	}
	p.conjectures = nil
	p.closed = true
	return nil
}
