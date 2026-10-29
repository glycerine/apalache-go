package polynomial

// Source: src/math/polynomial/polynomial_cache.cpp:207,211,232,233;
// src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"reflect"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// The private source imp is replaced on Reset independently of the public
// cache object. Its Go record carries that separate charged lifetime.
type polynomialCacheImpOwner struct{ bytes uint64 }

func (c *PolynomialCache) newImp(line int) error {
	size := uint64(reflect.TypeOf(polynomialCacheImpOwner{}).Size())
	if e := c.manager.core.integer.Budget().Reserve(size); e != nil {
		return e
	}
	owner := &polynomialCacheImpOwner{bytes: size}
	c.imp = owner
	tags := c.manager.core.monomials.monomialTraceTags()
	if diagnostic.DebugBuild && tags.ShouldTrace("memory") {
		_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
			func() bool { return true }, func(w io.Writer) error {
				_, _ = io.WriteString(w, "alloc "+fmt.Sprintf("%p", owner)+
					" src/math/polynomial/polynomial_cache.cpp:"+strconv.Itoa(line)+
					" imp "+strconv.FormatUint(size, 10)+"\n")
				return nil
			})
	}
	return nil
}

func (c *PolynomialCache) releaseImp(line int) {
	owner := c.imp
	if owner == nil {
		return
	}
	c.imp = nil
	c.manager.core.integer.Budget().Release(owner.bytes)
	owner.bytes = 0
	tags := c.manager.core.monomials.monomialTraceTags()
	if diagnostic.DebugBuild && tags.ShouldTrace("memory") {
		_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
			func() bool {
				if tags.MemoryFinalizing() {
					return false
				}
				runtime := c.manager.core.integer.RuntimeOwner()
				return runtime == nil || runtime.ProcessOwner() == nil || !runtime.ProcessOwner().Finalizing()
			}, func(w io.Writer) error {
				_, _ = io.WriteString(w, "dealloc "+fmt.Sprintf("%p", owner)+
					" src/math/polynomial/polynomial_cache.cpp:"+strconv.Itoa(line)+"\n")
				return nil
			})
	}
}
