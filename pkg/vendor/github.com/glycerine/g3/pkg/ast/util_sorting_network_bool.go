package ast

// Source: src/util/sorting_network.h:20-114; src/test/sorting_network.cpp:20-35,133-147.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

// ASTBooleanSortingNetwork is the concrete Boolean-expression specialization
// used by the registered C++ test. It keeps the source's index buffers and
// current/next orientation across calls. Returned references belong to the
// caller, like the source expr_ref_vector output.
type ASTBooleanSortingNetwork struct {
	manager      *ASTManager
	currentv     []uint32
	nextv        []uint32
	currentFirst bool
}

func NewASTBooleanSortingNetwork(m *ASTManager) (*ASTBooleanSortingNetwork, error) {
	if m == nil {
		return nil, ErrASTHandle
	}
	return &ASTBooleanSortingNetwork{manager: m, currentFirst: true}, nil
}

func (n *ASTBooleanSortingNetwork) columns() (current, next []uint32) {
	if n.currentFirst {
		return n.currentv, n.nextv
	}
	return n.nextv, n.currentv
}

func (n *ASTBooleanSortingNetwork) swapColumns() { n.currentFirst = !n.currentFirst }

func (n *ASTBooleanSortingNetwork) expression(a, b, thenExpr, elseExpr ASTHandle) (ASTRef, error) {
	condition, err := n.manager.MkImplies(a, b)
	if err != nil {
		return ASTRef{}, err
	}
	conditionRef, err := n.manager.NewRef(condition)
	if err != nil {
		return ASTRef{}, err
	}
	result, resultErr := n.manager.MkIte(condition, thenExpr, elseExpr)
	closeErr := n.manager.DelRef(&conditionRef)
	if resultErr != nil {
		return ASTRef{}, resultErr
	}
	if closeErr != nil {
		return ASTRef{}, closeErr
	}
	return n.manager.NewRef(result)
}

func (n *ASTBooleanSortingNetwork) exchange(i, j uint32, out []ASTRef) error {
	if i > j || uint64(j) >= uint64(len(out)) {
		return arenaBounds()
	}
	if i == j {
		return nil
	}
	ei, err := n.manager.RefGet(out[i])
	if err != nil {
		return err
	}
	ej, err := n.manager.RefGet(out[j])
	if err != nil {
		return err
	}
	left, err := n.expression(ei, ej, ei, ej)
	if err != nil {
		return err
	}
	right, err := n.expression(ej, ei, ei, ej)
	if err != nil {
		_ = n.manager.DelRef(&left)
		return err
	}
	oldLeft := out[i]
	out[i] = left
	if err = n.manager.DelRef(&oldLeft); err != nil {
		_ = n.manager.DelRef(&right)
		return err
	}
	oldRight := out[j]
	out[j] = right
	return n.manager.DelRef(&oldRight)
}

func (n *ASTBooleanSortingNetwork) sort(k uint32, out []ASTRef) error {
	current, next := n.columns()
	if k == 2 {
		for i := uint32(0); uint64(2*i+1) < uint64(len(out)); i++ {
			left, right := current[2*i], current[2*i+1]
			if err := n.exchange(left, right, out); err != nil {
				return err
			}
			next[2*i] = left
			next[2*i+1] = right
		}
		n.swapColumns()
		return nil
	}
	length := uint32(len(out))
	for i := uint32(0); i < length/k; i++ {
		base := k * i
		for j := uint32(0); j < k/2; j++ {
			next[base+j] = current[base+2*j]
			next[base+k/2+j] = current[base+2*j+1]
		}
	}
	n.swapColumns()
	if err := n.sort(k/2, out); err != nil {
		return err
	}
	current, next = n.columns()
	for i := uint32(0); i < length/k; i++ {
		base := k * i
		for j := uint32(0); j < k/2; j++ {
			next[base+2*j] = current[base+j]
			next[base+2*j+1] = current[base+k/2+j]
		}
		for j := uint32(0); j < k/2-1; j++ {
			if err := n.exchange(next[base+2*j+1], next[base+2*(j+1)], out); err != nil {
				return err
			}
		}
	}
	n.swapColumns()
	return nil
}

// Sort returns an owned expression reference for each output entry. It
// releases any partial output if an operation fails.
func (n *ASTBooleanSortingNetwork) Sort(input []ASTHandle) (output []ASTRef, result error) {
	if n == nil || n.manager == nil {
		return nil, ErrASTHandle
	}
	defer func() {
		if result != nil {
			for i := len(output) - 1; i >= 0; i-- {
				if err := n.manager.DelRef(&output[i]); err != nil {
					result = errors.Join(result, err)
				}
			}
			output = nil
		}
	}()
	for _, h := range input {
		ref, err := n.manager.NewRef(h)
		if err != nil {
			return output, err
		}
		output = append(output, ref)
	}
	if len(input) <= 1 {
		return output, nil
	}
	falseValue, err := n.manager.MkFalse()
	if err != nil {
		return output, err
	}
	for len(output)&(len(output)-1) != 0 {
		ref, err := n.manager.NewRef(falseValue)
		if err != nil {
			return output, err
		}
		output = append(output, ref)
	}
	if uint64(len(output)) > uint64(^uint32(0)) {
		return output, arenaBounds()
	}
	for i := uint32(0); i < uint32(len(output)); i++ {
		n.currentv = append(n.currentv, i)
		n.nextv = append(n.nextv, i)
	}
	for k := uint32(2); k <= uint32(len(output)); k *= 2 {
		if err := n.sort(k, output); err != nil {
			return output, err
		}
		if k > ^uint32(0)/2 {
			break
		}
	}
	return output, nil
}

func arenaBounds() error { return arena.ErrBounds }
