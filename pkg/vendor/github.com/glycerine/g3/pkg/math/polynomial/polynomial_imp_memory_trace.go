package polynomial

// Source: src/math/polynomial/polynomial.cpp:6931,6935,6939;
// src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"reflect"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (m *MultivariatePolynomialManager) polynomialImpTraceTags() *diagnostic.TraceTags {
	if m.memoryTraceTags != nil {
		return m.memoryTraceTags
	}
	return diagnostic.BorrowProcessTraceTags()
}

func (m *MultivariatePolynomialManager) tracePolynomialImpAllocated(line int) {
	tags := m.polynomialImpTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "alloc "+fmt.Sprintf("%p", m)+
				" src/math/polynomial/polynomial.cpp:"+strconv.Itoa(line)+
				" imp "+strconv.FormatUint(uint64(reflect.TypeOf(MultivariatePolynomialManager{}).Size()), 10)+"\n")
			return nil
		})
}

func (m *MultivariatePolynomialManager) tracePolynomialImpDeallocated() {
	tags := m.polynomialImpTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool {
			if tags.MemoryFinalizing() {
				return false
			}
			runtime := m.core.integer.RuntimeOwner()
			return runtime == nil || runtime.ProcessOwner() == nil || !runtime.ProcessOwner().Finalizing()
		}, func(w io.Writer) error {
			_, _ = io.WriteString(w, "dealloc "+fmt.Sprintf("%p", m)+" src/math/polynomial/polynomial.cpp:6939\n")
			return nil
		})
}
