package core

// Source: src/tactic/core/reduce_args_tactic.cpp mk_mc.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func (f *reduceArgsFrame) makeModelConverter(result *tactic.TacticResult) error {
	m := f.owner.manager
	var err error
	f.extension, err = tactic.NewExtensionModel(m, f.owner.services)
	if err != nil {
		return err
	}
	if err = f.extension.IncRef(); err != nil {
		return err
	}
	f.filter, err = tactic.NewFilterModel(m)
	if err != nil {
		return err
	}
	if err = f.filter.IncRef(); err != nil {
		return err
	}
	if err = f.filter.BindTrace(f.owner.trace, f.owner.globals); err != nil {
		return err
	}
	for d, group := range f.groups {
		candidate := f.candidates[d]
		var variables, arguments []ast.ASTHandle
		for i, v := range candidate.mask {
			sort, err := m.GetDomain(d, uint32(i))
			if err != nil {
				return err
			}
			variable, err := m.MkVar(uint32(i), sort)
			if err != nil {
				return err
			}
			if err = f.hold(variable, false); err != nil {
				return err
			}
			variables = append(variables, variable)
			if !v {
				arguments = append(arguments, variable)
			}
		}
		var definition ast.ASTHandle
		for _, entry := range group {
			if err = f.filter.Insert(entry.declaration); err != nil {
				return err
			}
			term, err := m.MkApp(entry.declaration, arguments)
			if err != nil {
				return err
			}
			if err = f.hold(term, false); err != nil {
				return err
			}
			if definition.IsNull() {
				definition = term
				continue
			}
			var equalities []ast.ASTHandle
			for i, v := range candidate.mask {
				if v {
					arg, err := m.GetArg(entry.application, uint32(i))
					if err != nil {
						return err
					}
					eq, err := m.MkEq(variables[i], arg)
					if err != nil {
						return err
					}
					if err = f.hold(eq, false); err != nil {
						return err
					}
					equalities = append(equalities, eq)
				}
			}
			condition, err := m.MkAndArgs(equalities)
			if err != nil {
				return err
			}
			if err = f.hold(condition, false); err != nil {
				return err
			}
			definition, err = m.MkIte(condition, term, definition)
			if err != nil {
				return err
			}
			if err = f.hold(definition, false); err != nil {
				return err
			}
		}
		if err = f.extension.Insert(d, definition); err != nil {
			return err
		}
	}
	converter, err := tactic.ConcatModels(m.Budget(), f.filter, f.extension)
	if err != nil {
		return err
	}
	if err = result.SetModelConverter(converter); err != nil {
		return errors.Join(err, converter.Close())
	}
	return nil
}
