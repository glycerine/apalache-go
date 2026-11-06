package sat

import "math"

type satAIGNode struct {
	sign         bool
	op           satAIGBoolOp
	lut          uint64
	size, offset uint32
}

func invalidSATAIGNode() satAIGNode {
	return satAIGNode{op: satAIGNone, size: math.MaxUint32, offset: math.MaxUint32}
}
func satAIGVarNode(v uint32) satAIGNode { return satAIGNode{op: satAIGVar, offset: v} }
func satAIGOpNode(sign bool, op satAIGBoolOp, n, off uint32) satAIGNode {
	return satAIGNode{sign: sign, op: op, size: n, offset: off}
}
func satAIGLUTNode(table uint64, n, off uint32) satAIGNode {
	return satAIGNode{op: satAIGLUT, lut: table, size: n, offset: off}
}
func (n satAIGNode) valid() bool   { return n.offset != math.MaxUint32 }
func (n satAIGNode) isVar() bool   { return n.op == satAIGVar }
func (n satAIGNode) isAnd() bool   { return n.op == satAIGAnd }
func (n satAIGNode) isXor() bool   { return n.op == satAIGXor }
func (n satAIGNode) isITE() bool   { return n.op == satAIGITE }
func (n satAIGNode) isLUT() bool   { return n.op == satAIGLUT }
func (n satAIGNode) isConst() bool { return n.isAnd() && n.size == 0 }

type satAIGRoot struct {
	variable BoolVar
	root     Literal
}

func (a *satAIGCuts) child(n satAIGNode, i uint32) Literal { return a.literals[n.offset+i] }
