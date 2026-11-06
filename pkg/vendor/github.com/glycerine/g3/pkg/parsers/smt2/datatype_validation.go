package smt2

// Source: src/parsers/smt2/smt2parser.cpp:885-920,930-954,981-1001.

import (
	"github.com/glycerine/g3/pkg/smtcore"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// validateDatatypeNames is the selected name-checking part of the parser's
// declaration branch. AST datatype commit and command-context publication are
// later phases; they still need the full parser and pdecl manager.
func validateDatatypeNames(symbols *usymbol.Manager, group *smtcore.ParamDatatypeGroup, line, pos int32) error {
	if len(group.Datatypes) == 0 {
		return nil
	}
	if len(group.Datatypes) == 1 {
		if missing, found := group.Datatypes[0].HasMissingRef(); found {
			return datatypeNameError(symbols, "invalid datatype declaration, unknown sort '", missing, line, pos)
		}
	} else if missing, ok := group.FixMissingRefs(); !ok {
		return datatypeNameError(symbols, "invalid datatype declaration, unknown sort '", missing, line, pos)
	}
	for i := range group.Datatypes {
		if repeated, found := group.Datatypes[i].HasDuplicateAccessors(); found {
			return datatypeNameError(symbols, "invalid datatype declaration, repeated accessor identifier '", repeated, line, pos)
		}
	}
	return nil
}

func datatypeNameError(symbols *usymbol.Manager, prefix string, name usymbol.Symbol, line, pos int32) error {
	spelling, err := symbols.String(name)
	if err != nil {
		return err
	}
	return smtcore.NewCmdExceptionAt(prefix+spelling+"'", line, pos)
}
