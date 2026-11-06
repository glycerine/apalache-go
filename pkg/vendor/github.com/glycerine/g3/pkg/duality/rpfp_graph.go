package duality

// Source: src/duality/duality.h:481-616 (RPFP graph primitives).

import (
	"errors"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/solver"
	"math"

	"github.com/glycerine/g3/pkg/ast"
)

var ErrConnectedNode = errors.New("cannot delete RPFP node")

// RPFP owns the graph and its AST references; the context is borrowed.
type RPFP struct {
	streamReporter        *StreamReporter
	conjectureReporter    *ConjectureReporter
	pendingProofWitnesses []*proofWitnessCall
	sourceClauses         []ast.ASTRef
	clauseIndices         map[*Edge]int
	evaluatorConfig       model.ModelEvaluatorConfig
	pendingEvaluators     []*model.ModelEvaluator
	proofCore             map[ast.ASTHandle]ast.ASTRef
	dualModel             *model.ModelCore
	solver                solver.Solver
	scopes                []graphScope
	HornClauses           bool
	underapproxFlags      map[ast.ASTHandle]*Node
	underapproxFlagRefs   []ast.ASTRef
	vocabulary            *BackgroundVocabulary
	ownsVocabulary        bool
	context               *Context
	nodes                 []*Node
	edges                 []*Edge
	nodeCount, edgeCount  int
	closed                bool
}

type Node struct {
	dual                           ast.ASTRef
	Name                           ast.ASTRef
	Annotation, Bound, Underapprox *Transformer
	Owner                          *RPFP
	Number                         int
	Outgoing                       *Edge
	Incoming                       []*Edge
	Map                            *Node
	RecursionBound                 uint32
	closed                         bool
}

type Edge struct {
	labeled          ast.ASTRef
	dual             ast.ASTRef
	constraints      []ast.ASTRef
	relMap           map[ast.ASTHandle]int
	varMap           map[ast.ASTHandle]ast.ASTRef
	mapsReady        bool
	F                *Transformer
	Parent           *Node
	Children         []*Node
	Owner            *RPFP
	Number           int
	Map              *Edge
	quantifiedTheory bool
	closed           bool
}

func NewRPFP(context *Context) (*RPFP, error) {
	if err := context.open(); err != nil {
		return nil, err
	}
	v, err := NewBackgroundVocabulary(context)
	if err != nil {
		return nil, err
	}
	return &RPFP{context: context, vocabulary: v, ownsVocabulary: true}, nil
}
func (g *RPFP) open() error {
	if g == nil || g.closed {
		return ErrWrapperOwner
	}
	return g.context.open()
}

func (g *RPFP) CreateNode(term *Expr) (_ *Node, result error) {
	if err := g.open(); err != nil {
		return nil, err
	}
	if err := term.open(); err != nil {
		return nil, err
	}
	if term.value.object.context != g.context {
		return nil, ErrContextMismatch
	}
	count, err := term.NumArgs()
	if err != nil {
		return nil, err
	}
	args := make([]*Expr, 0, count)
	defer func() {
		for _, a := range args {
			result = errors.Join(result, a.Close())
		}
	}()
	for n := uint32(0); n < count; n++ {
		a, err := term.Arg(n)
		if err != nil {
			return nil, err
		}
		args = append(args, a)
	}
	decl, err := term.Decl()
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, decl.Close()) }()
	node := &Node{Owner: g, RecursionBound: math.MaxUint32}
	defer func() {
		if result != nil {
			result = errors.Join(result, node.close())
		}
	}()
	node.Name, err = g.context.manager.CopyRef(decl.value.ref)
	if err != nil {
		return nil, err
	}
	truth, err := g.context.BoolVal(true)
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, truth.Close()) }()
	node.Annotation, err = NewTransformer(g.context, nil, args, truth)
	if err != nil {
		return nil, err
	}
	node.Bound, err = NewTransformer(g.context, nil, args, truth)
	if err != nil {
		return nil, err
	}
	falsity, err := g.context.BoolVal(false)
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, falsity.Close()) }()
	node.Underapprox, err = NewTransformer(g.context, nil, args, falsity)
	if err != nil {
		return nil, err
	}
	g.nodeCount++
	node.Number = g.nodeCount
	g.nodes = append(g.nodes, node)
	return node, nil
}

func (g *RPFP) CloneNode(old *Node) (_ *Node, result error) {
	if err := g.open(); err != nil {
		return nil, err
	}
	if old == nil || old.closed {
		return nil, ErrWrapperOwner
	}
	if old.Owner.context != g.context {
		return nil, ErrContextMismatch
	}
	node := &Node{Owner: g, Map: old, RecursionBound: math.MaxUint32}
	defer func() {
		if result != nil {
			result = errors.Join(result, node.close())
		}
	}()
	var err error
	node.Name, err = g.context.manager.CopyRef(old.Name)
	if err != nil {
		return nil, err
	}
	node.Annotation, err = old.Annotation.Clone()
	if err != nil {
		return nil, err
	}
	node.Bound, err = old.Bound.Clone()
	if err != nil {
		return nil, err
	}
	node.Underapprox, err = old.Underapprox.Clone()
	if err != nil {
		return nil, err
	}
	g.nodeCount++
	node.Number = g.nodeCount
	g.nodes = append(g.nodes, node)
	return node, nil
}
func (n *Node) close() error {
	if n.closed {
		return nil
	}
	for _, t := range []*Transformer{n.Annotation, n.Bound, n.Underapprox} {
		if t != nil {
			if err := t.Close(); err != nil {
				return err
			}
		}
	}
	if n.dual != (ast.ASTRef{}) {
		if err := n.Owner.context.manager.DelRef(&n.dual); err != nil {
			return err
		}
	}
	if n.Name != (ast.ASTRef{}) {
		if err := n.Owner.context.manager.DelRef(&n.Name); err != nil {
			return err
		}
	}
	n.closed = true
	return nil
}
func (g *RPFP) DeleteNode(node *Node) error {
	if err := g.open(); err != nil {
		return err
	}
	if node == nil || node.Owner != g || node.closed {
		return ErrWrapperOwner
	}
	if node.Outgoing != nil || len(node.Incoming) != 0 {
		return ErrConnectedNode
	}
	if err := node.close(); err != nil {
		return err
	}
	for i := len(g.nodes) - 1; i >= 0; i-- {
		if g.nodes[i] == node {
			g.nodes = append(g.nodes[:i], g.nodes[i+1:]...)
			break
		}
	}
	return nil
}
func (g *RPFP) CreateEdge(parent *Node, formula *Transformer, children []*Node) (*Edge, error) {
	if err := g.open(); err != nil {
		return nil, err
	}
	if parent == nil || parent.Owner != g || parent.closed {
		return nil, ErrWrapperOwner
	}
	if err := formula.open(); err != nil {
		return nil, err
	}
	if formula.context != g.context {
		return nil, ErrContextMismatch
	}
	for _, child := range children {
		if child == nil || child.Owner != g || child.closed {
			return nil, ErrWrapperOwner
		}
	}
	copied, err := formula.Clone()
	if err != nil {
		return nil, err
	}
	g.edgeCount++
	edge := &Edge{F: copied, Parent: parent, Children: append([]*Node(nil), children...), Owner: g, Number: g.edgeCount}
	parent.Outgoing = edge
	for _, child := range children {
		child.Incoming = append(child.Incoming, edge)
	}
	g.edges = append(g.edges, edge)
	return edge, nil
}
func (g *RPFP) CreateLowerBoundEdge(parent *Node) (*Edge, error) {
	if parent == nil {
		return nil, ErrWrapperOwner
	}
	return g.CreateEdge(parent, parent.Annotation, nil)
}
func (g *RPFP) DeleteEdge(edge *Edge) error {
	if err := g.open(); err != nil {
		return err
	}
	if edge == nil || edge.Owner != g || edge.closed {
		return ErrWrapperOwner
	}
	if edge.labeled != (ast.ASTRef{}) {
		if err := g.context.manager.DelRef(&edge.labeled); err != nil {
			return err
		}
	}
	if err := edge.closeConstraints(); err != nil {
		return err
	}
	if err := edge.clearMaps(); err != nil {
		return err
	}
	if err := edge.F.Close(); err != nil {
		return err
	}
	if edge.Parent != nil {
		edge.Parent.Outgoing = nil
	}
	for _, child := range edge.Children {
		for i, e := range child.Incoming {
			if e == edge {
				child.Incoming = append(child.Incoming[:i], child.Incoming[i+1:]...)
				break
			}
		}
	}
	for i := len(g.edges) - 1; i >= 0; i-- {
		if g.edges[i] == edge {
			g.edges = append(g.edges[:i], g.edges[i+1:]...)
			break
		}
	}
	edge.closed = true
	delete(g.clauseIndices, edge)
	return nil
}
func (g *RPFP) Close() error {
	if g == nil || g.closed {
		return nil
	}
	for len(g.edges) > 0 {
		if err := g.DeleteEdge(g.edges[len(g.edges)-1]); err != nil {
			return err
		}
	}
	for len(g.nodes) > 0 {
		if err := g.DeleteNode(g.nodes[len(g.nodes)-1]); err != nil {
			return err
		}
	}
	for len(g.sourceClauses) > 0 {
		n := len(g.sourceClauses) - 1
		err := g.context.manager.DelRef(&g.sourceClauses[n])
		if g.sourceClauses[n] == (ast.ASTRef{}) {
			g.sourceClauses = g.sourceClauses[:n]
		}
		if err != nil {
			return err
		}
	}
	g.clauseIndices = nil
	if err := g.cleanupProofWitnesses(); err != nil {
		return err
	}
	for len(g.underapproxFlagRefs) > 0 {
		n := len(g.underapproxFlagRefs) - 1
		err := g.context.manager.DelRef(&g.underapproxFlagRefs[n])
		if g.underapproxFlagRefs[n] == (ast.ASTRef{}) {
			g.underapproxFlagRefs = g.underapproxFlagRefs[:n]
		}
		if err != nil {
			return err
		}
	}
	g.underapproxFlags = nil
	for len(g.pendingEvaluators) > 0 {
		ev := g.pendingEvaluators[0]
		if err := ev.Close(); err != nil {
			return err
		}
		g.pendingEvaluators = g.pendingEvaluators[1:]
	}
	if err := g.ClearProofCore(); err != nil {
		return err
	}
	if err := g.replaceDualModel(nil, false); err != nil {
		return err
	}
	if err := g.closeScopeRefs(); err != nil {
		return err
	}
	if g.ownsVocabulary {
		if err := g.vocabulary.Close(); err != nil {
			return err
		}
	}
	g.closed = true
	return nil
}
