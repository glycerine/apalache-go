package polynomial

// Source: src/math/polynomial/polynomial.cpp:2933-2950;
// src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"reflect"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (v *PolynomialNewtonVector) traceNewtonAllocated(child *PolynomialNewtonInterpolator) {
	tags := v.storage.monomials.monomialTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "alloc "+fmt.Sprintf("%p", child)+
				" src/math/polynomial/polynomial.cpp:2948 newton_interpolator "+
				strconv.FormatUint(uint64(reflect.TypeOf(PolynomialNewtonInterpolator{}).Size()), 10)+"\n")
			return nil
		})
}

func (v *PolynomialNewtonVector) traceNewtonDeallocated(child *PolynomialNewtonInterpolator) {
	tags := v.storage.monomials.monomialTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool {
			if tags.MemoryFinalizing() {
				return false
			}
			runtime := v.storage.integer.RuntimeOwner()
			return runtime == nil || runtime.ProcessOwner() == nil || !runtime.ProcessOwner().Finalizing()
		}, func(w io.Writer) error {
			_, _ = io.WriteString(w, "dealloc "+fmt.Sprintf("%p", child)+" src/math/polynomial/polynomial.cpp:2936\n")
			return nil
		})
}
