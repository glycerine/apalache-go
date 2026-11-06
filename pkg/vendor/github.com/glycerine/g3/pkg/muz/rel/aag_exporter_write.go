package rel

import (
	"github.com/glycerine/g3/pkg/ast"
	"io"
)

func (x *AAGExporter) Write(out io.Writer) error {
	if e := x.open(); e != nil {
		return e
	}
	for _, v := range append(append([]ast.ASTHandle{}, x.current...), x.argCurrent...) {
		if _, e := x.builder.internal(v); e != nil {
			return e
		}
	}
	for _, v := range append(append([]ast.ASTHandle{}, x.next...), x.argNext...) {
		if _, e := x.builder.Input(v); e != nil {
			return e
		}
	}
	transitions := []uint32{}
	n, e := x.rules.Len()
	if e != nil {
		return e
	}
	for i := 0; i < n; i++ {
		r, e := x.rules.Rule(i)
		if e != nil {
			return e
		}
		id, e := x.lowerRule(r)
		closeErr := r.Close()
		if e != nil {
			return e
		}
		if closeErr != nil {
			return closeErr
		}
		transitions = append(transitions, id)
	}
	for _, f := range x.facts {
		id, e := x.lowerFact(f)
		if e != nil {
			return e
		}
		transitions = append(transitions, id)
	}
	tr, e := x.builder.OrMany(transitions)
	if e != nil {
		return e
	}
	// Argument latches may have been allocated while lowering; register them now.
	for _, v := range x.argCurrent {
		if _, e = x.builder.internal(v); e != nil {
			return e
		}
	}
	for _, v := range x.argNext {
		if _, e = x.builder.Input(v); e != nil {
			return e
		}
	}
	for i := range x.current {
		if e = x.builder.AddTransitionLatch(x.current[i], x.next[i], tr); e != nil {
			return e
		}
	}
	for i := range x.argCurrent {
		if e = x.builder.AddTransitionLatch(x.argCurrent[i], x.argNext[i], tr); e != nil {
			return e
		}
	}
	if e = x.lowerOutputs(); e != nil {
		return e
	}
	return x.builder.Write(out)
}
