package rel

import "github.com/glycerine/g3/pkg/ast"

func (b *AAGBuilder) variable(expression ast.ASTHandle, input bool) (uint32, error) {
	if e := b.open(); e != nil {
		return 0, e
	}
	if id, ok := b.expressions[expression]; ok {
		return id, nil
	}
	id, e := b.newLiteral()
	if e != nil {
		return 0, e
	}
	b.expressions[expression] = id
	if input {
		b.inputs = append(b.inputs, id)
	}
	return id, nil
}
func (b *AAGBuilder) Input(expression ast.ASTHandle) (uint32, error) {
	return b.variable(expression, true)
}
func (b *AAGBuilder) internal(expression ast.ASTHandle) (uint32, error) {
	return b.variable(expression, false)
}
func (b *AAGBuilder) FreshInput() (uint32, error) {
	id, e := b.newLiteral()
	if e == nil {
		b.inputs = append(b.inputs, id)
	}
	return id, e
}
