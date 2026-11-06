package api

// Source: src/api/api_config_params.cpp:73,79; src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"reflect"
	"strconv"

	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func traceConfigAllocated(tags *diagnostic.TraceTags, p *smtcore.ContextParams) {
	if !uparams.GlobalParamsSourceDebug || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			for _, part := range []string{"alloc ", fmt.Sprintf("%p", p), " src/api/api_config_params.cpp:73 context_params ", strconv.FormatUint(uint64(reflect.TypeOf(smtcore.ContextParams{}).Size()), 10), "\n"} {
				_, _ = io.WriteString(w, part)
			}
			return nil
		})
}

func (c *Config) traceConfigDeallocated(p *smtcore.ContextParams) {
	if !uparams.GlobalParamsSourceDebug || !c.trace.ShouldTrace("memory") {
		return
	}
	_ = c.trace.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return c.process == nil || !c.process.Finalizing() }, func(w io.Writer) error {
			for _, part := range []string{"dealloc ", fmt.Sprintf("%p", p), " src/api/api_config_params.cpp:79\n"} {
				_, _ = io.WriteString(w, part)
			}
			return nil
		})
}
