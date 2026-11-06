package spacer

// Source: unsat_core_plugin_min_cut in
// src/muz/spacer/spacer_unsat_core_plugin.cpp and .h.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/mincut"
)

// UnsatCoreMinCutPlugin converts the retained B-pure proof frontier into the
// source split-node graph and publishes a minimum proof cut as the core.
type UnsatCoreMinCutPlugin struct {
	learner       *UnsatCoreLearner
	visited       map[ast.ASTHandle]bool
	proofToMinus  map[ast.ASTHandle]uint32
	proofToPlus   map[ast.ASTHandle]uint32
	nodeToFormula []ast.ASTHandle
	cut           *mincut.MinCut
	closed        bool
}

func NewUnsatCoreMinCutPlugin(learner *UnsatCoreLearner) (*UnsatCoreMinCutPlugin, error) {
	if learner == nil || learner.open() != nil {
		return nil, ErrUnsatCoreLearnerOwner
	}
	return &UnsatCoreMinCutPlugin{
		learner: learner, visited: make(map[ast.ASTHandle]bool),
		proofToMinus: make(map[ast.ASTHandle]uint32), proofToPlus: make(map[ast.ASTHandle]uint32),
		cut: mincut.New(),
	}, nil
}

func (p *UnsatCoreMinCutPlugin) open() error {
	if p == nil || p.closed || p.learner == nil || p.cut == nil {
		return ErrUnsatCoreLearnerOwner
	}
	return p.learner.open()
}

func (p *UnsatCoreMinCutPlugin) rememberNode(node uint32, formula ast.ASTHandle) {
	for uint32(len(p.nodeToFormula)) <= node {
		p.nodeToFormula = append(p.nodeToFormula, ast.ASTHandle{})
	}
	p.nodeToFormula[node] = formula
}

func (p *UnsatCoreMinCutPlugin) proofNodes(proof ast.ASTHandle) (uint32, uint32, error) {
	if minus, found := p.proofToMinus[proof]; found {
		return minus, p.proofToPlus[proof], nil
	}
	minus := p.cut.NewNode()
	plus := p.cut.NewNode()
	fact, err := p.learner.manager.GetProofFact(proof)
	if err != nil {
		return 0, 0, err
	}
	p.proofToMinus[proof] = minus
	p.proofToPlus[proof] = plus
	p.rememberNode(minus, fact)
	p.rememberNode(plus, fact)
	if err = p.cut.AddEdge(minus, plus); err != nil {
		return 0, 0, err
	}
	return minus, plus, nil
}

func (p *UnsatCoreMinCutPlugin) addEdge(from, to ast.ASTHandle) error {
	fromNode := uint32(0)
	if !from.IsNull() {
		_, plus, err := p.proofNodes(from)
		if err != nil {
			return err
		}
		fromNode = plus
	}
	toNode := uint32(1)
	if !to.IsNull() {
		minus, _, err := p.proofNodes(to)
		if err != nil {
			return err
		}
		toNode = minus
	}
	return p.cut.AddEdge(fromNode, toNode)
}

func (p *UnsatCoreMinCutPlugin) advanceToLowestPartialCut(step ast.ASTHandle, todo *[]ast.ASTHandle) error {
	isSink := true
	var subproof []ast.ASTHandle
	count, err := p.learner.manager.GetProofNumParents(step)
	if err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		parent, err := p.learner.manager.GetProofParent(step, i)
		if err != nil {
			return err
		}
		if p.learner.IsBMarked(parent) {
			subproof = append(subproof, parent)
		}
	}
	for len(subproof) != 0 {
		last := len(subproof) - 1
		current := subproof[last]
		subproof = subproof[:last]
		if p.learner.IsClosed(current) || !p.learner.IsBMarked(current) {
			continue
		}
		pure, err := p.learner.IsBPure(current)
		if err != nil {
			return err
		}
		fact, err := p.learner.manager.GetProofFact(current)
		if err != nil {
			return err
		}
		asserted, err := p.learner.manager.IsProofKind(current, ast.ASTPrAsserted)
		if err != nil {
			return err
		}
		literal, err := p.learner.manager.IsLiteral(fact)
		if err != nil {
			return err
		}
		if pure && (asserted || literal) {
			from := step
			if p.learner.IsAMarked(step) {
				from = ast.ASTHandle{}
			}
			if err = p.addEdge(from, current); err != nil {
				return err
			}
			*todo = append(*todo, current)
			isSink = false
			continue
		}
		parents, err := p.learner.manager.GetProofNumParents(current)
		if err != nil {
			return err
		}
		for i := uint32(0); i < parents; i++ {
			parent, err := p.learner.manager.GetProofParent(current, i)
			if err != nil {
				return err
			}
			subproof = append(subproof, parent)
		}
	}
	if isSink {
		return p.addEdge(step, ast.ASTHandle{})
	}
	return nil
}

func (p *UnsatCoreMinCutPlugin) ComputePartialCore(step ast.ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	todo := []ast.ASTHandle{step}
	for len(todo) != 0 {
		last := len(todo) - 1
		current := todo[last]
		todo = todo[:last]
		if p.learner.IsClosed(current) || p.visited[current] {
			continue
		}
		if err := p.advanceToLowestPartialCut(current, &todo); err != nil {
			return err
		}
		p.visited[current] = true
	}
	return p.learner.SetClosed(step, true)
}

func (p *UnsatCoreMinCutPlugin) Finalize() error {
	if err := p.open(); err != nil {
		return err
	}
	nodes, err := p.cut.ComputeMinCut(nil)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		if uint64(node) >= uint64(len(p.nodeToFormula)) || p.nodeToFormula[node].IsNull() {
			return ErrUnsatCoreLearnerOwner
		}
		if err = p.learner.AddLemmaToCore(p.nodeToFormula[node]); err != nil {
			return err
		}
	}
	return nil
}

func (p *UnsatCoreMinCutPlugin) Close() error {
	if p == nil {
		return ErrUnsatCoreLearnerOwner
	}
	if p.closed {
		return nil
	}
	p.learner = nil
	p.visited = nil
	p.proofToMinus = nil
	p.proofToPlus = nil
	p.nodeToFormula = nil
	p.cut = nil
	p.closed = true
	return nil
}
