package opt

// Source: src/opt/maxsmt.cpp maxsmt_solver_base::process_mutex.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (d *maxSMTMutexDecomposition) transform(mutex []ast.ASTHandle) error {
	active := make([]ast.ASTHandle, 0, len(mutex))
	for _, expression := range mutex {
		if weight := d.weights[expression]; weight != nil && weight.Sign() > 0 {
			active = append(active, expression)
		}
	}
	if len(active) <= 1 {
		return nil
	}
	maxSMTSortMutex(active, d.weights)
	values := make([]big.Int, len(active))
	for i, expression := range active {
		values[i].Set(d.weights[expression])
		delete(d.weights, expression)
	}
	var previous big.Int
	for i := len(active) - 1; i >= 0; i-- {
		var delta big.Int
		delta.Sub(&values[i], &previous)
		previous.Set(&values[i])
		if delta.Sign() == 0 {
			continue
		}
		prefix, err := d.owner.hold(d.owner.manager.MkOrArgs(active[:i+1]))
		if err != nil {
			return err
		}
		if old := d.weights[prefix]; old != nil {
			old.Add(old, &delta)
		} else {
			d.weights[prefix] = new(big.Int).Set(&delta)
			d.order = append(d.order, prefix)
		}
		d.offset.Add(&d.offset, new(big.Int).Mul(&delta, big.NewInt(int64(i))))
	}
	return nil
}
