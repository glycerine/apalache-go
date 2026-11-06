package params

// Source: src/util/gparams.cpp:23,93; generated gparams_register_modules.cpp.

import (
	"fmt"
	"sync"

	"github.com/glycerine/g3/pkg/arena"
)

// Module packages supply the collectors implemented by their own C++ sources.
// This replaces the generated C++ dispatcher's linkage to those translation units.
var globalParamsCollectors = struct {
	sync.RWMutex
	byName map[string]func(*ParamDescrs) error
}{byName: make(map[string]func(*ParamDescrs) error)}

// RegisterGlobalParamsCollector binds a source collector during package initialization.
// Names identify source collector functions rather than modules: a module may have
// several collectors, whose invocation order is retained by the dispatcher.
func RegisterGlobalParamsCollector(name string, collect func(*ParamDescrs) error) {
	if name == "" || collect == nil {
		panic(arena.ErrConfig)
	}
	globalParamsCollectors.Lock()
	defer globalParamsCollectors.Unlock()
	if _, exists := globalParamsCollectors.byName[name]; exists {
		panic("duplicate parameter collector: " + name)
	}
	globalParamsCollectors.byName[name] = collect
}

func catalogCollector(name string) func(*ParamDescrs) error {
	return func(d *ParamDescrs) error {
		globalParamsCollectors.RLock()
		collect := globalParamsCollectors.byName[name]
		globalParamsCollectors.RUnlock()
		if collect == nil {
			return fmt.Errorf("%w: parameter collector %s is not linked", arena.ErrConfig, name)
		}
		return collect(d)
	}
}
