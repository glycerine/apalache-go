package smtcore

// Source: src/smt/smt_model_checker.cpp check_rec_fun;
// src/smt/smt_context.cpp add_rec_funs_to_model.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
)

func smtRecursiveDefinitionParts(m *ast.ASTManager, q ast.ASTHandle) (function, body ast.ASTHandle, err error) {
	count, err := m.GetNumPatterns(q)
	if err != nil {
		return function, body, err
	}
	if count != 2 {
		return function, body, ast.ErrASTDomain
	}
	values := [2]ast.ASTHandle{}
	for i := uint32(0); i < 2; i++ {
		pattern, err := m.GetPattern(q, i)
		if err != nil {
			return function, body, err
		}
		values[i], err = m.GetArg(pattern, 0)
		if err != nil {
			return function, body, err
		}
	}
	return values[0], values[1], nil
}

func smtRecursiveCallArguments(m *ast.ASTManager, application ast.ASTHandle) ([]ast.ASTHandle, error) {
	count, err := m.GetNumArgs(application)
	if err != nil {
		return nil, err
	}
	arguments := make([]ast.ASTHandle, count)
	for i := range arguments {
		arguments[i], err = m.GetArg(application, uint32(i))
		if err != nil {
			return nil, err
		}
	}
	return arguments, nil
}

func (c *SMTModelChecker) checkRecursiveFunction(md *protomodel.ProtoModel, q ast.ASTHandle, strict bool) (satisfied bool, result error) {
	function, _, err := smtRecursiveDefinitionParts(c.manager, q)
	if err != nil {
		return false, err
	}
	declaration, err := c.manager.GetDecl(function)
	if err != nil {
		return false, err
	}
	count, err := c.manager.GetNumDeclarations(q)
	if err != nil {
		return false, err
	}
	instance, err := c.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, c.manager.DelRef(&instance)) }()
	evaluated, err := c.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, c.manager.DelRef(&evaluated)) }()
	for _, node := range c.state.context.byDeclaration[declaration] {
		if !c.state.isRelevant(node.owner) {
			continue
		}
		arguments, err := smtRecursiveCallArguments(c.manager, node.owner)
		if err != nil {
			return false, err
		}
		if len(arguments) != int(count) {
			return false, ast.ErrASTDomain
		}
		if err = rewriter.InstantiateQuantifier(c.manager, q, arguments, instance); err != nil {
			return false, err
		}
		term, err := c.manager.RefGet(instance)
		if err != nil {
			return false, err
		}
		if err = c.manager.ResetRef(evaluated); err != nil {
			return false, err
		}
		if _, err = md.Eval(term, evaluated, true); err != nil {
			return false, err
		}
		value, err := c.manager.RefGet(evaluated)
		if err != nil {
			return false, err
		}
		if strict && !c.manager.IsTrue(value) || !strict && c.manager.IsFalse(value) {
			_, err := c.pinInstance(q, arguments, 0)
			return false, err
		}
	}
	return true, nil
}

func (k *SMTKernel) addRecursiveFunctionsToModel(md *model.ModelCore) error {
	for i := 0; ; i++ {
		canceled, err := k.manager.Canceled()
		if err != nil {
			return err
		}
		if canceled || i >= len(k.assertions) {
			return nil
		}
		q := k.assertions[i].Formula
		kind, err := k.manager.GetKind(q)
		if err != nil {
			return err
		}
		if kind != ast.ASTQuantifier {
			continue
		}
		recursive, err := k.manager.IsRecursiveFunctionDefinition(q)
		if err != nil {
			return err
		}
		if recursive {
			if err := k.registerRecursiveFunctionModel(md, q); err != nil {
				return err
			}
		}
	}
}

func (k *SMTKernel) registerRecursiveFunctionModel(md *model.ModelCore, q ast.ASTHandle) (result error) {
	function, body, err := smtRecursiveDefinitionParts(k.manager, q)
	if err != nil {
		return err
	}
	declaration, err := k.manager.GetDecl(function)
	if err != nil {
		return err
	}
	arguments, err := smtRecursiveCallArguments(k.manager, function)
	if err != nil {
		return err
	}
	substitutions := make([]ast.ASTHandle, len(arguments))
	var refs []ast.ASTRef
	defer func() {
		for i := range refs {
			result = errors.Join(result, k.manager.DelRef(&refs[i]))
		}
	}()
	for i, argument := range arguments {
		sort, err := k.manager.GetSort(argument)
		if err != nil {
			return err
		}
		substitutions[i], err = k.manager.MkVar(uint32(i), sort)
		if err != nil {
			return err
		}
		ref, err := k.manager.NewRef(substitutions[i])
		if err != nil {
			return err
		}
		refs = append(refs, ref)
	}
	substitution, err := rewriter.NewVarSubstitution(k.manager, true)
	if substitution != nil {
		defer func() { result = errors.Join(result, substitution.Close()) }()
	}
	if err != nil {
		return err
	}
	out, err := k.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, k.manager.DelRef(&out)) }()
	if err = substitution.Substitute(body, substitutions, out); err != nil {
		return err
	}
	value, err := k.manager.RefGet(out)
	if err != nil {
		return err
	}
	interpretation, err := model.NewFunctionInterpretation(k.manager, uint32(len(arguments)))
	if err != nil {
		return err
	}
	if err = interpretation.SetElse(value); err == nil {
		err = md.RegisterFunction(declaration, interpretation)
	}
	if err != nil {
		return errors.Join(err, interpretation.Close())
	}
	return nil
}
