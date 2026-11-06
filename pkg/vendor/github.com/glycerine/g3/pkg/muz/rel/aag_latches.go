package rel

import "github.com/glycerine/g3/pkg/ast"

func (b *AAGBuilder) AddLatch(current ast.ASTHandle, next uint32) error {
	id, e := b.internal(current)
	if e != nil {
		return e
	}
	b.latches = append(b.latches, aagLatch{id, next})
	return nil
}
func (b *AAGBuilder) AddTransitionLatch(current, next ast.ASTHandle, transition uint32) error {
	currentID, e := b.internal(current)
	if e != nil {
		return e
	}
	nextID, e := b.Input(next)
	if e != nil {
		return e
	}
	take, e := b.And(transition, nextID)
	if e != nil {
		return e
	}
	keep, e := b.And(AAGNegate(transition), currentID)
	if e != nil {
		return e
	}
	value, e := b.Or(take, keep)
	if e != nil {
		return e
	}
	b.latches = append(b.latches, aagLatch{currentID, value})
	return nil
}
func (b *AAGBuilder) AddOutput(literal uint32) error {
	if e := b.open(); e != nil {
		return e
	}
	b.outputs = append(b.outputs, literal)
	return nil
}
