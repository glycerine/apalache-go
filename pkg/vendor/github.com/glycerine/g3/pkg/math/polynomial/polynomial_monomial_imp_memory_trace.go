package polynomial

// Source: src/math/polynomial/polynomial.cpp:814,2330,2341;
// src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"reflect"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (m *MonomialManager) tracePolynomialImpAllocated() {
	tags := m.monomialTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "alloc "+fmt.Sprintf("%p", m)+
				" src/math/polynomial/polynomial.cpp:"+strconv.Itoa(m.config.polynomialImpAllocationLine)+
				" monomial_manager "+strconv.FormatUint(uint64(reflect.TypeOf(MonomialManager{}).Size()), 10)+"\n")
			return nil
		})
}

func (m *MonomialManager) tracePolynomialImpDeallocated() {
	tags := m.monomialTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool {
			if tags.MemoryFinalizing() {
				return false
			}
			process := m.memory.ProcessOwner()
			return process == nil || !process.Finalizing()
		}, func(w io.Writer) error {
			_, _ = io.WriteString(w, "dealloc "+fmt.Sprintf("%p", m)+" src/math/polynomial/polynomial.cpp:814\n")
			return nil
		})
}
