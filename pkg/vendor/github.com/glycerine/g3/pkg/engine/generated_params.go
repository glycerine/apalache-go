// Package engine links the generated parameter collectors at the process
// composition boundary above util.
package engine

// Source: mem_initializer links the same collector translation units through
// gparams_register_modules.cpp.

import (
	_ "github.com/glycerine/g3/pkg/ackermannization"
	_ "github.com/glycerine/g3/pkg/ast"
	_ "github.com/glycerine/g3/pkg/ast/fpa"
	_ "github.com/glycerine/g3/pkg/ast/normal_forms"
	_ "github.com/glycerine/g3/pkg/ast/rewriter"
	_ "github.com/glycerine/g3/pkg/math/polynomial"
	_ "github.com/glycerine/g3/pkg/math/realclosure"
	_ "github.com/glycerine/g3/pkg/model"
	_ "github.com/glycerine/g3/pkg/muz/base"
	_ "github.com/glycerine/g3/pkg/nlsat"
	_ "github.com/glycerine/g3/pkg/opt"
	_ "github.com/glycerine/g3/pkg/parsers/util"
	_ "github.com/glycerine/g3/pkg/sat"
	_ "github.com/glycerine/g3/pkg/smtcore"
	_ "github.com/glycerine/g3/pkg/solver"
	_ "github.com/glycerine/g3/pkg/tactic/sls"
	_ "github.com/glycerine/g3/pkg/tactic/smtlogics/params"
	_ "github.com/glycerine/g3/pkg/util/lp"
	"github.com/glycerine/g3/pkg/util/process"
)

// InitializeGenerated binds all retained generated collectors before the
// source-order process initializer is reached.
func InitializeGenerated(c process.InitializationConfig, maxSize uint64) (*process.Initialized, error) {
	return process.InitializeGenerated(c, maxSize)
}
