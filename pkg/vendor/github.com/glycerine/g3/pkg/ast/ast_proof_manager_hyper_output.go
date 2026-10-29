package ast

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

// ASTProofHyperOutput owns substitution AST references in native slices.
// Positions are plain values. The output is confined to its manager's caller.
type ASTProofHyperOutput struct {
	self            *ASTProofHyperOutput
	manager         *ASTManager
	positions       []ASTProofPosition
	substitutions   [][]ASTHandle
	ownerBytes      uint64
	closed, closing bool
}

func NewASTProofHyperOutput(m *ASTManager) (*ASTProofHyperOutput, error) {
	if e := m.lock(); e != nil {
		return nil, e
	}
	defer m.mu.Unlock()
	n := uint64(reflect.TypeOf(ASTProofHyperOutput{}).Size())
	if e := m.BudgetField.Reserve(n); e != nil {
		return nil, e
	}
	v := &ASTProofHyperOutput{manager: m, ownerBytes: n}
	v.self = v
	return v, nil
}
func (v *ASTProofHyperOutput) check(m *ASTManager) error {
	if v == nil || v.self != v || v.manager != m {
		return ErrASTHandle
	}
	if v.closed {
		return arena.ErrClosed
	}
	if v.closing {
		return ErrASTPending
	}
	return nil
}
func (v *ASTProofHyperOutput) PositionsLen() uint32 {
	if v == nil || v.closed || v.self != v {
		return 0
	}
	return uint32(len(v.positions))
}
func (v *ASTProofHyperOutput) SubstitutionsLen() uint32 {
	if v == nil || v.closed || v.self != v {
		return 0
	}
	return uint32(len(v.substitutions))
}
func (v *ASTProofHyperOutput) Position(i uint32) (ASTProofPosition, error) {
	if v == nil {
		return ASTProofPosition{}, ErrASTHandle
	}
	if e := v.check(v.manager); e != nil {
		return ASTProofPosition{}, e
	}
	if e := v.manager.lock(); e != nil {
		return ASTProofPosition{}, e
	}
	defer v.manager.mu.Unlock()
	if uint64(i) >= uint64(len(v.positions)) {
		return ASTProofPosition{}, arena.ErrBounds
	}
	return v.positions[i], nil
}

// Substitution returns a borrowed, read-only handle slice. Its references
// remain owned by the output until Close; callers must not mutate the slice.
func (v *ASTProofHyperOutput) Substitution(i uint32) ([]ASTHandle, error) {
	if v == nil {
		return nil, ErrASTHandle
	}
	if e := v.check(v.manager); e != nil {
		return nil, e
	}
	if e := v.manager.lock(); e != nil {
		return nil, e
	}
	defer v.manager.mu.Unlock()
	if uint64(i) >= uint64(len(v.substitutions)) {
		return nil, arena.ErrBounds
	}
	return v.substitutions[i], nil
}
func (v *ASTProofHyperOutput) Close() error {
	if v == nil || v.self != v {
		return ErrASTHandle
	}
	if v.closed {
		return nil
	}
	m := v.manager
	if m == nil || m.Self != m {
		return ErrASTHandle
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	v.closing = true
	if !m.Closed {
		for _, inner := range v.substitutions {
			for j, h := range inner {
				if h.IsNull() {
					continue
				}
				r, e := m.internal(h)
				if e != nil {
					return e
				}
				consumed, e := m.decNode(r)
				if consumed {
					inner[j] = ASTHandle{}
				}
				if e != nil {
					return e
				}
				if !consumed {
					return ErrASTPending
				}
			}
		}
	}
	// Forced manager teardown already ended the AST graph. Native backing and
	// this external owner can still be released without node callbacks.
	m.BudgetField.Release(v.ownerBytes)
	v.positions = nil
	v.substitutions = nil
	v.ownerBytes = 0
	v.closed = true
	return nil
}
