package cmd_context

// Source: src/cmd_context/cmd_context.cpp init_manager, mk_app and
// find_func_decl. Builtin names come from the retained declaration plugins.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (c *Context) addBuiltinNames(family ast.FamilyID, names *ast.ASTBuiltinNames) error {
	for _, item := range names.Items() {
		name, _, err := c.manager.Symbols.BareString(item.Name)
		if err != nil {
			return err
		}
		c.builtins[name] = append(c.builtins[name], builtin{family, item.Kind})
	}
	names.Reset()
	return nil
}

func (c *Context) initBuiltins() error {
	m := c.manager
	for _, item := range []struct {
		name string
		kind int32
	}{{"true", ast.ASTOpTrue}, {"false", ast.ASTOpFalse}, {"and", ast.ASTOpAnd}, {"or", ast.ASTOpOr}, {"not", ast.ASTOpNot}, {"=>", ast.ASTOpImplies}, {"xor", ast.ASTOpXor}, {"=", ast.ASTOpEq}, {"distinct", ast.ASTOpDistinct}, {"ite", ast.ASTOpIte}} {
		c.builtins[item.name] = []builtin{{m.BasicFamilyID(), item.kind}}
	}
	var logic usymbol.Symbol
	var err error
	if c.logic != "" {
		logic, err = m.Symbols.Intern(c.logic)
		if err != nil {
			return err
		}
	}
	var names ast.ASTBuiltinNames
	defer names.Close()
	family, err := m.ArithmeticFamilyID()
	if err != nil {
		return err
	}
	if c.logic == "" || solver.LogicHasArithmetic(c.logic) {
		if err = m.GetArithmeticOpNames(&names, logic); err != nil {
			return err
		}
		if err = c.addBuiltinNames(family, &names); err != nil {
			return err
		}
	}
	family, err = m.RegisterBitVectorPlugin()
	if err != nil {
		return err
	}
	if c.logic == "" || solver.LogicHasBitVector(c.logic) {
		if err = m.GetBitVectorOpNames(&names, logic); err != nil {
			return err
		}
		if err = c.addBuiltinNames(family, &names); err != nil {
			return err
		}
	}
	family, err = m.RegisterFloatingPointPlugin()
	if err != nil {
		return err
	}
	if c.logic == "" || solver.LogicHasFPA(c.logic) {
		if err = m.GetFloatingPointOpNames(&names, logic); err != nil {
			return err
		}
		if err = c.addBuiltinNames(family, &names); err != nil {
			return err
		}
	}
	family, err = m.RegisterPseudoBooleanPlugin()
	if err != nil {
		return err
	}
	// The plugin itself gates these names to unspecified logic, ALL and QF_FD.
	if err = m.GetPseudoBooleanOpNames(&names, logic); err != nil {
		return err
	}
	if err = c.addBuiltinNames(family, &names); err != nil {
		return err
	}
	family, err = m.DatatypeFamilyID()
	if err != nil {
		return err
	}
	if c.logic == "" || solver.LogicHasDatatype(c.logic) {
		if err = m.GetDatatypeOpNames(&names, logic); err != nil {
			return err
		}
		if err = c.addBuiltinNames(family, &names); err != nil {
			return err
		}
	}
	family, err = m.ArrayFamilyID()
	if err != nil {
		return err
	}
	if c.logic == "" || solver.LogicHasArray(c.logic) {
		if err = m.GetArrayOpNames(&names, logic); err != nil {
			return err
		}
		if err = c.addBuiltinNames(family, &names); err != nil {
			return err
		}
	}
	family, err = m.SequenceFamilyID()
	if err != nil {
		return err
	}
	if c.logic == "" || solver.LogicHasSequence(c.logic) {
		if err = m.GetSequenceOpNames(&names, logic); err != nil {
			return err
		}
		if err = c.addBuiltinNames(family, &names); err != nil {
			return err
		}
	}
	return nil
}

// BuiltinSortAvailable follows the logic-gated source sort-name installation.
// Plugins remain installed in the AST manager even when their names are hidden
// from the command language.
func (c *Context) BuiltinSortAvailable(name string) bool {
	switch name {
	case "Bool":
		return true
	case "Int":
		return (c.logic == "" || solver.LogicHasArithmetic(c.logic)) && c.logic != "NRA" && c.logic != "QF_NRA" && c.logic != "QF_UFNRA"
	case "Real":
		return c.logic == "" || solver.LogicHasArithmetic(c.logic)
	case "BitVec":
		return c.logic == "" || solver.LogicHasBitVector(c.logic)
	case "FloatingPoint", "RoundingMode", "Float16", "Float32", "Float64", "Float128":
		return c.logic == "" || solver.LogicHasFPA(c.logic)
	case "Array":
		return c.logic == "" || solver.LogicHasArray(c.logic)
	case "String", "StringSequence", "Seq", "RegEx":
		return c.logic == "" || solver.LogicHasSequence(c.logic)
	}
	return false
}

func (c *Context) MakeApp(name string, args []ast.ASTHandle, indices []ast.ASTParameter, rangeSort ast.ASTHandle) (ast.ASTHandle, error) {
	m, err := c.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if values := c.builtins[name]; len(values) != 0 {
		if name == "const" && !rangeSort.IsNull() && len(args) == 1 {
			return m.MkConstArray(rangeSort, args[0])
		}
		op := values[0]
		if len(values) > 1 && len(args) > 0 {
			sort, err := m.GetSort(args[0])
			if err != nil {
				return ast.ASTHandle{}, err
			}
			family, err := m.GetFamilyID(sort)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			for _, candidate := range values {
				if candidate.family == family {
					op = candidate
					break
				}
			}
		}
		if name == "=" && len(args) == 2 {
			return m.MkEq(args[0], args[1])
		}
		if name == "xor" {
			return m.MkXorArgs(args)
		}
		if name == "distinct" {
			return m.MkDistinct(args)
		}
		return m.MkFamilyApp(op.family, op.kind, indices, args)
	}
	if len(indices) != 0 {
		return ast.ASTHandle{}, fmt.Errorf("invalid use of indexed identifier, unknown builtin function %s", name)
	}
	if len(c.functions[name]) == 0 {
		if len(args) == 0 {
			return ast.ASTHandle{}, fmt.Errorf("unknown constant %s", name)
		}
		return ast.ASTHandle{}, fmt.Errorf("unknown function/constant %s", name)
	}
	// Source func_decls::find returns the sole declaration directly. MkApp
	// then validates arity and performs the manager's arithmetic coercions.
	if declarations := c.functions[name]; len(declarations) == 1 {
		return m.MkApp(declarations[0], args)
	}
	if len(args) == 0 && rangeSort.IsNull() && len(c.functions[name]) > 1 {
		return ast.ASTHandle{}, fmt.Errorf("ambiguous constant reference %s; use (as <symbol> <sort>)", name)
	}
	var match ast.ASTHandle
	for _, decl := range c.functions[name] {
		arity, err := m.GetArity(decl)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if int(arity) != len(args) {
			continue
		}
		if !rangeSort.IsNull() {
			r, err := m.GetRange(decl)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if r != rangeSort {
				continue
			}
		}
		matches := true
		for i, arg := range args {
			sort, err := m.GetSort(arg)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			domain, err := m.GetDomain(decl, uint32(i))
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if sort != domain {
				matches = false
				break
			}
		}
		if matches {
			if !match.IsNull() {
				return ast.ASTHandle{}, fmt.Errorf("ambiguous function reference %s", name)
			}
			match = decl
		}
	}
	if match.IsNull() {
		return ast.ASTHandle{}, fmt.Errorf("unknown constant %s or no matching function signature", name)
	}
	return m.MkApp(match, args)
}
