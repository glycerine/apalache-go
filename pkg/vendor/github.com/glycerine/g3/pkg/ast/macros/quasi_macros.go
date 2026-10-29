package macros

// Source: src/ast/macros/quasi_macros.cpp/.h.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type QuasiMacros struct {
	manager     *ast.ASTManager
	macros      *Manager
	occurrences map[ast.ASTHandle]uint32
}

func NewQuasiMacros(m *ast.ASTManager, macros *Manager) (*QuasiMacros, error) {
	if m == nil || macros == nil || macros.manager != m {
		return nil, ast.ErrASTHandle
	}
	if err := macros.open(); err != nil {
		return nil, err
	}
	return &QuasiMacros{manager: m, macros: macros}, nil
}

func (q *QuasiMacros) nonGroundUninterpreted(term ast.ASTHandle) (bool, error) {
	kind, err := q.manager.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	ground, err := q.manager.IsGround(term)
	if err != nil || ground {
		return false, err
	}
	declaration, err := q.manager.GetDecl(term)
	if err != nil {
		return false, err
	}
	family, err := q.manager.GetFamilyID(declaration)
	return family == ast.NullFamilyID, err
}

func (q *QuasiMacros) findOccurrences(root ast.ASTHandle) error {
	once := make(map[ast.ASTHandle]struct{})
	more := make(map[ast.ASTHandle]struct{})
	stack := []ast.ASTHandle{root}
	for len(stack) != 0 {
		i := len(stack) - 1
		current := stack[i]
		stack = stack[:i]
		if _, yes := more[current]; yes {
			continue
		}
		if _, yes := once[current]; yes {
			more[current] = struct{}{}
		}
		once[current] = struct{}{}
		kind, err := q.manager.GetKind(current)
		if err != nil {
			return err
		}
		switch kind {
		case ast.ASTVar:
		case ast.ASTQuantifier:
			body, e := q.manager.GetQuantifierBody(current)
			if e != nil {
				return e
			}
			stack = append(stack, body)
		case ast.ASTApp:
			yes, e := q.nonGroundUninterpreted(current)
			if e != nil {
				return e
			}
			if yes {
				declaration, e := q.manager.GetDecl(current)
				if e != nil {
					return e
				}
				q.occurrences[declaration]++
			}
			n, e := q.manager.GetNumArgs(current)
			if e != nil {
				return e
			}
			for j := n; j > 0; j-- {
				arg, e := q.manager.GetArg(current, j-1)
				if e != nil {
					return e
				}
				stack = append(stack, arg)
			}
		default:
			return ast.ErrASTDomain
		}
	}
	return nil
}

func (q *QuasiMacros) unique(head ast.ASTHandle) (bool, error) {
	declaration, err := q.manager.GetDecl(head)
	if err != nil {
		return false, err
	}
	return q.occurrences[declaration] == 1, nil
}

func (q *QuasiMacros) fullyDepends(head, quantifier ast.ASTHandle) (bool, error) {
	n, err := q.manager.GetNumDeclarations(quantifier)
	if err != nil {
		return false, err
	}
	found := make([]bool, n)
	arguments, err := q.manager.GetNumArgs(head)
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < arguments; i++ {
		arg, e := q.manager.GetArg(head, i)
		if e != nil {
			return false, e
		}
		kind, e := q.manager.GetKind(arg)
		if e != nil {
			return false, e
		}
		if kind == ast.ASTVar {
			index, e := q.manager.GetVarIndex(arg)
			if e != nil || index >= n {
				return false, e
			}
			found[index] = true
		}
	}
	for _, yes := range found {
		if !yes {
			return false, nil
		}
	}
	return true, nil
}

func (q *QuasiMacros) depends(term, declaration ast.ASTHandle) (bool, error) {
	seen := make(map[ast.ASTHandle]struct{})
	stack := []ast.ASTHandle{term}
	for len(stack) != 0 {
		i := len(stack) - 1
		current := stack[i]
		stack = stack[:i]
		if _, yes := seen[current]; yes {
			continue
		}
		seen[current] = struct{}{}
		kind, err := q.manager.GetKind(current)
		if err != nil {
			return false, err
		}
		if kind != ast.ASTApp {
			continue
		}
		d, err := q.manager.GetDecl(current)
		if err != nil {
			return false, err
		}
		if d == declaration {
			return true, nil
		}
		n, err := q.manager.GetNumArgs(current)
		if err != nil {
			return false, err
		}
		for j := n; j > 0; j-- {
			arg, e := q.manager.GetArg(current, j-1)
			if e != nil {
				return false, e
			}
			stack = append(stack, arg)
		}
	}
	return false, nil
}

func (q *QuasiMacros) candidate(term ast.ASTHandle) (head, definition ast.ASTHandle, matched bool, err error) {
	kind, err := q.manager.GetKind(term)
	if err != nil || kind != ast.ASTQuantifier {
		return head, definition, false, err
	}
	forall, err := q.manager.IsForall(term)
	if err != nil || !forall {
		return head, definition, false, err
	}
	body, err := q.manager.GetQuantifierBody(term)
	if err != nil {
		return head, definition, false, err
	}
	family, familyErr := q.manager.GetFamilyID(body)
	op, opErr := q.manager.GetDeclKind(body)
	if familyErr == nil && opErr == nil && family == q.manager.BasicFamilyID() && (op == ast.ASTOpEq || op == ast.ASTOpIff) {
		lhs, e := q.manager.GetArg(body, 0)
		if e != nil {
			return head, definition, false, e
		}
		rhs, e := q.manager.GetArg(body, 1)
		if e != nil {
			return head, definition, false, e
		}
		for _, pair := range [][2]ast.ASTHandle{{lhs, rhs}, {rhs, lhs}} {
			yes, e := q.nonGroundUninterpreted(pair[0])
			if e != nil || !yes {
				if e != nil {
					return head, definition, false, e
				}
				continue
			}
			unique, e := q.unique(pair[0])
			if e != nil || !unique {
				if e != nil {
					return head, definition, false, e
				}
				continue
			}
			declaration, e := q.manager.GetDecl(pair[0])
			if e != nil {
				return head, definition, false, e
			}
			depends, e := q.depends(pair[1], declaration)
			if e != nil || depends {
				if e != nil {
					return head, definition, false, e
				}
				continue
			}
			full, e := q.fullyDepends(pair[0], term)
			if e != nil || !full {
				if e != nil {
					return head, definition, false, e
				}
				continue
			}
			return pair[0], pair[1], true, nil
		}
	}
	if familyErr == nil && opErr == nil && family == q.manager.BasicFamilyID() && op == ast.ASTOpNot {
		argument, e := q.manager.GetArg(body, 0)
		if e != nil {
			return head, definition, false, e
		}
		yes, e := q.nonGroundUninterpreted(argument)
		if e != nil || !yes {
			return head, definition, false, e
		}
		unique, e := q.unique(argument)
		if e != nil || !unique {
			return head, definition, false, e
		}
		value, e := q.manager.MkFalse()
		return argument, value, e == nil, e
	}
	yes, err := q.nonGroundUninterpreted(body)
	if err != nil || !yes {
		return head, definition, false, err
	}
	unique, err := q.unique(body)
	if err != nil || !unique {
		return head, definition, false, err
	}
	value, err := q.manager.MkTrue()
	return body, value, err == nil, err
}

func (q *QuasiMacros) convert(quantifier, head, definition ast.ASTHandle) (ast.ASTHandle, error) {
	declaration, err := q.manager.GetDecl(head)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	oldCount, err := q.manager.GetNumDeclarations(quantifier)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	argumentCount, err := q.manager.GetNumArgs(head)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	seen := make([]bool, oldCount)
	arguments := make([]ast.ASTHandle, argumentCount)
	extraNames := make([]usymbol.Symbol, 0, argumentCount)
	extraSorts := make([]ast.ASTHandle, 0, argumentCount)
	conditions := make([]ast.ASTHandle, 0, argumentCount)
	for i := uint32(0); i < argumentCount; i++ {
		argument, e := q.manager.GetArg(head, i)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		kind, e := q.manager.GetKind(argument)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if kind == ast.ASTVar {
			index, e := q.manager.GetVarIndex(argument)
			if e != nil || index >= oldCount {
				return ast.ASTHandle{}, errors.Join(e, ast.ErrASTDomain)
			}
			if !seen[index] {
				seen[index], arguments[i] = true, argument
				continue
			}
		}
		sort, e := q.manager.GetDomain(declaration, i)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		name, e := q.manager.Symbols.Intern(fmt.Sprintf("X%d", len(extraNames)))
		if e != nil {
			return ast.ASTHandle{}, e
		}
		variable, e := q.manager.MkVar(oldCount+uint32(len(extraNames)), sort)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		extraNames = append(extraNames, name)
		extraSorts = append(extraSorts, sort)
		arguments[i] = variable
		equal, e := q.manager.MkEq(variable, argument)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		conditions = append(conditions, equal)
	}
	names := make([]usymbol.Symbol, 0, len(extraNames)+int(oldCount))
	sorts := make([]ast.ASTHandle, 0, len(extraSorts)+int(oldCount))
	for i := len(extraNames) - 1; i >= 0; i-- {
		names, sorts = append(names, extraNames[i]), append(sorts, extraSorts[i])
	}
	for i := uint32(0); i < oldCount; i++ {
		name, e := q.manager.GetDeclarationName(quantifier, i)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		sort, e := q.manager.GetDeclarationSort(quantifier, i)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		names, sorts = append(names, name), append(sorts, sort)
	}
	application, err := q.manager.MkApp(declaration, arguments)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	name, err := q.manager.GetASTName(declaration)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	domain := make([]ast.ASTHandle, argumentCount)
	for i := range domain {
		domain[i], err = q.manager.GetDomain(declaration, uint32(i))
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	rangeSort, err := q.manager.GetRange(declaration)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	suffix, err := q.manager.Symbols.Intern("else")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	otherwiseDecl, err := q.manager.MkFreshFuncDecl(name, suffix, domain, rangeSort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	otherwise, err := q.manager.MkApp(otherwiseDecl, arguments)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	condition, err := q.manager.MkAndArray(conditions)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	choice, err := q.manager.MkIte(condition, definition, otherwise)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	body, err := q.manager.MkEq(application, choice)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return q.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: body})
}

func (q *QuasiMacros) find(inputs []Formula) (bool, error) {
	q.occurrences = make(map[ast.ASTHandle]uint32)
	for _, row := range inputs {
		if err := q.findOccurrences(row.Expression); err != nil {
			return false, err
		}
	}
	found := false
	proofs, err := q.manager.ProofsEnabled()
	if err != nil {
		return false, err
	}
	for _, row := range inputs {
		head, definition, yes, e := q.candidate(row.Expression)
		if e != nil {
			return false, e
		}
		if !yes {
			continue
		}
		macro, e := q.convert(row.Expression, head, definition)
		if e != nil {
			return false, e
		}
		var proof ast.ASTHandle
		if proofs {
			proof, e = q.manager.MkProofDefAxiom(macro)
			if e != nil {
				return false, e
			}
		}
		declaration, e := q.manager.GetDecl(head)
		if e != nil {
			return false, e
		}
		inserted, e := q.macros.Insert(declaration, macro, proof, ast.ASTDependency{})
		if e != nil {
			return false, e
		}
		found = found || inserted
	}
	return found, nil
}

// Apply finds unique quasi macros, installs them, and expands every input.
// The returned batch is owned by the caller.
func (q *QuasiMacros) Apply(inputs []Formula) (result *FormulaBatch, changed bool, body error) {
	if q == nil || q.manager == nil || q.macros == nil {
		return nil, false, ast.ErrASTHandle
	}
	found, err := q.find(inputs)
	if err != nil {
		return nil, false, err
	}
	result, err = NewFormulaBatch(q.manager)
	if err != nil {
		return nil, false, err
	}
	fail := func(e error) (*FormulaBatch, bool, error) { return nil, false, errors.Join(e, result.Close()) }
	proofs, err := q.manager.ProofsEnabled()
	if err != nil {
		return fail(err)
	}
	for _, row := range inputs {
		if !found {
			if err = result.Insert(row); err != nil {
				return fail(err)
			}
			continue
		}
		out, e := q.manager.NewRef(ast.ASTHandle{})
		if e != nil {
			return fail(e)
		}
		outProof, e := q.manager.NewRef(ast.ASTHandle{})
		if e != nil {
			_ = q.manager.DelRef(&out)
			return fail(e)
		}
		proof := row.Proof
		if !proofs {
			proof = ast.ASTHandle{}
		}
		dependency, e := q.macros.ExpandMacros(row.Expression, proof, row.Dependency, out, outProof)
		if e == nil {
			expression, x := q.manager.RefGet(out)
			e = x
			newProof := ast.ASTHandle{}
			if e == nil {
				newProof, e = q.manager.RefGet(outProof)
			}
			if e == nil {
				e = result.Insert(Formula{Expression: expression, Proof: newProof, Dependency: dependency})
			}
		}
		if !dependency.IsNull() {
			e = errors.Join(e, q.manager.DependencyDec(dependency))
		}
		e = errors.Join(e, q.manager.DelRef(&outProof), q.manager.DelRef(&out))
		if e != nil {
			return fail(e)
		}
	}
	return result, found, nil
}
