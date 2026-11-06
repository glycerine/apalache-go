package sat

import (
	"github.com/glycerine/g3/pkg/arena"
	"sort"
)

func (a *satAIGCuts) AddVar(v uint32) error {
	if err := a.reserve(v); err != nil {
		return err
	}
	if len(a.nodes[v]) == 0 {
		a.nodes[v] = append(a.nodes[v], satAIGVarNode(v))
		if err := a.initCutSet(v); err != nil {
			a.nodes[v] = nil
			return err
		}
		a.touch(v)
	}
	return nil
}

func (a *satAIGCuts) AddNode(head Literal, op satAIGBoolOp, args []Literal) error {
	if op != satAIGAnd && op != satAIGXor && op != satAIGITE {
		return ErrSATKind
	}
	if op == satAIGITE && len(args) != 3 {
		return arena.ErrBounds
	}
	v := uint32(head.variable())
	if err := a.reserve(v); err != nil {
		return err
	}
	off := uint32(len(a.literals))
	a.literals = append(a.literals, args...)
	if op == satAIGAnd || op == satAIGXor {
		sort.Slice(a.literals[off:], func(i, j int) bool { return a.literals[int(off)+i] < a.literals[int(off)+j] })
	}
	for _, lit := range args {
		if err := a.AddVar(uint32(lit.variable())); err != nil {
			a.literals = a.literals[:off]
			return err
		}
	}
	return a.addNode(v, satAIGOpNode(head.sign(), op, uint32(len(args)), off))
}

func (a *satAIGCuts) AddLUT(v uint32, table uint64, args []BoolVar) error {
	if len(args) > satCutMaxSize {
		return arena.ErrBounds
	}
	if err := a.reserve(v); err != nil {
		return err
	}
	off := uint32(len(a.literals))
	for _, w := range args {
		if err := a.AddVar(uint32(w)); err != nil {
			a.literals = a.literals[:off]
			return err
		}
		a.literals = append(a.literals, MakeLiteral(w, false))
	}
	return a.addNode(v, satAIGLUTNode(table, uint32(len(args)), off))
}

func (a *satAIGCuts) AddCut(v uint32, table uint64, args []BoolVar) error {
	if len(args) > satCutMaxSize {
		return arena.ErrBounds
	}
	if err := a.AddVar(v); err != nil {
		return err
	}
	var c satCut
	for _, w := range args {
		if err := a.AddVar(uint32(w)); err != nil {
			return err
		}
		if !c.Add(uint32(w)) {
			return arena.ErrBounds
		}
	}
	c.SetTable(table)
	_, err := a.insertCut(v, c, a.cuts[v])
	return err
}

func (a *satAIGCuts) SetRoot(v BoolVar, root Literal) error {
	if err := a.reserve(uint32(v)); err != nil {
		return err
	}
	if err := a.reserve(uint32(root.variable())); err != nil {
		return err
	}
	a.roots = append(a.roots, satAIGRoot{v, root})
	return nil
}
