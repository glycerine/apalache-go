package fp

// Source: duality_dl_interface.cpp, query's quantify_arrays branch.
import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/duality"
)

func (e *PropositionalEngine) prepareDualityTransformedQuery(query ast.ASTHandle, bound uint32) (out *duality.PreparedQuery, result error) {
	clone, err := e.newQueryClone()
	if err != nil {
		return nil, err
	}
	defer func() {
		result = errors.Join(result, e.discardQueryClone(clone))
		if result != nil && out != nil {
			result = errors.Join(result, out.Close())
			out = nil
		}
	}()
	declaration, headRef, err := clone.makeQueryHead(query)
	if err != nil {
		return nil, err
	}
	queryRoots := []ast.ASTRef{headRef}
	defer func() { result = errors.Join(result, closeASTRefs(e.manager, queryRoots)) }()
	head, err := e.manager.RefGet(headRef)
	if err != nil {
		return nil, err
	}
	if err := clone.RegisterPredicate(declaration); err != nil {
		return nil, err
	}
	implicationRef, err := e.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, err
	}
	queryRoots = append(queryRoots, implicationRef)
	implication, err := e.manager.MkImplies(query, head)
	if !implication.IsNull() {
		err = errors.Join(err, e.manager.SetRef(implicationRef, implication))
	}
	if err != nil {
		return nil, err
	}
	if err := clone.AddRule(implication, ""); err != nil {
		return nil, err
	}
	prepared, err := clone.preprocessHornRulesWithQueryRule(declaration, nil, true, true, false, true)
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, prepared.Close()) }()
	var argumentRefs []ast.ASTRef
	defer func() { result = errors.Join(result, closeASTRefs(e.manager, argumentRefs)) }()
	outputs, err := prepared.rules.OutputPredicates()
	if err != nil {
		return nil, err
	}
	derivedQuery, err := e.manager.MkFalse()
	if err != nil {
		return nil, err
	}
	if len(outputs) > 0 {
		pred, err := prepared.rules.OutputPredicate()
		if err != nil {
			return nil, err
		}
		arity, err := e.manager.GetArity(pred)
		if err != nil {
			return nil, err
		}
		args := make([]ast.ASTHandle, arity)
		for i := range args {
			sort, err := e.manager.GetDomain(pred, uint32(i))
			if err != nil {
				return nil, err
			}
			args[i], err = e.manager.MkVar(arity-1-uint32(i), sort)
			if err != nil {
				return nil, err
			}
			ref, err := e.manager.NewRef(args[i])
			if err != nil {
				return nil, err
			}
			argumentRefs = append(argumentRefs, ref)
		}
		derivedQuery, err = e.manager.MkApp(pred, args)
		if err != nil {
			return nil, err
		}
	}
	queryRef, err := e.manager.NewRef(derivedQuery)
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, e.manager.DelRef(&queryRef)) }()
	var refs []ast.ASTRef
	defer func() { result = errors.Join(result, closeASTRefs(e.manager, refs)) }()
	count, err := prepared.rules.Len()
	if err != nil {
		return nil, err
	}
	forms := make([]ast.ASTHandle, 0, count)
	bounds := make([]uint32, 0, count)
	predicates := append([]ast.ASTHandle(nil), clone.predicateOrder...)
	predicates = append(predicates, outputs...)
	for i := 0; i < count; i++ {
		rule, err := prepared.rules.Rule(i)
		if err != nil {
			return nil, err
		}
		ref, err := rule.ToFormula()
		if err == nil {
			refs = append(refs, ref)
			var h ast.ASTHandle
			h, err = e.manager.RefGet(ref)
			if err == nil {
				forms = append(forms, h)
				bounds = append(bounds, ^uint32(0))
			}
		}
		if err == nil {
			var pred ast.ASTHandle
			pred, err = rule.HeadDeclaration()
			if err == nil {
				predicates = append(predicates, pred)
			}
		}
		err = errors.Join(err, rule.Close())
		if err != nil {
			return nil, err
		}
	}
	return duality.PrepareQuery(e.manager, forms, bounds, derivedQuery, predicates, bound)
}
