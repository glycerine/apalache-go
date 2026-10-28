package apalache

import "sync"

// ChangeListener records predecessor UIDs across transformations. Descendants
// of a transformed expression inherit the transformed root's origin unless a
// more specific transformation has already been recorded for them.
type ChangeListener struct {
	mutex        sync.RWMutex
	predecessors map[UID]UID
}

func NewChangeListener() *ChangeListener {
	return &ChangeListener{predecessors: make(map[UID]UID)}
}

func (listener *ChangeListener) OnExpressionTransformation(original, transformed Expr) {
	if listener == nil || original == nil || transformed == nil || original.ID() == transformed.ID() {
		return
	}
	listener.mutex.Lock()
	defer listener.mutex.Unlock()
	listener.predecessors[transformed.ID()] = original.ID()
	listener.inheritExpression(transformed, original.ID())
}

func (listener *ChangeListener) inheritExpression(expression Expr, origin UID) {
	switch typed := expression.(type) {
	case *OperExpr:
		for _, argument := range typed.Arguments() {
			listener.inheritExpression(argument, origin)
		}
	case *LetInExpr:
		for _, declaration := range typed.Declarations() {
			listener.inheritExpression(declaration.Body(), origin)
		}
		listener.inheritExpression(typed.Body(), origin)
	}
	if expression.ID() == origin {
		return
	}
	if _, exists := listener.predecessors[expression.ID()]; !exists {
		listener.predecessors[expression.ID()] = origin
	}
}

func (listener *ChangeListener) OnDeclarationTransformation(original, transformed Decl) {
	if listener == nil || original == nil || transformed == nil || original.ID() == transformed.ID() {
		return
	}
	listener.mutex.Lock()
	defer listener.mutex.Unlock()
	listener.predecessors[transformed.ID()] = original.ID()
}

func (listener *ChangeListener) IsDefinedAt(id UID) bool {
	if listener == nil {
		return false
	}
	listener.mutex.RLock()
	defer listener.mutex.RUnlock()
	_, exists := listener.predecessors[id]
	return exists
}

func (listener *ChangeListener) TraceBack(id UID) UID {
	if listener == nil {
		return id
	}
	listener.mutex.RLock()
	defer listener.mutex.RUnlock()
	seen := make(map[UID]struct{})
	for {
		if _, cycle := seen[id]; cycle {
			return id
		}
		seen[id] = struct{}{}
		predecessor, exists := listener.predecessors[id]
		if !exists {
			return id
		}
		id = predecessor
	}
}
