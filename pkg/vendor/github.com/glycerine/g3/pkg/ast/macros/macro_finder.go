package macros

// Source: src/ast/macros/macro_finder.cpp/.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type Formula struct {
	Expression ast.ASTHandle
	Proof      ast.ASTHandle
	Dependency ast.ASTDependency
}

// FormulaBatch owns every expression, optional proof and dependency row.
type FormulaBatch struct {
	self    *FormulaBatch
	manager *ast.ASTManager
	rows    []Formula
	header  uint64
	closed  bool
}

func NewFormulaBatch(m *ast.ASTManager) (*FormulaBatch, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	n := uint64(reflect.TypeOf(FormulaBatch{}).Size())
	if err := m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	b := &FormulaBatch{manager: m, header: n}
	b.self = b
	return b, nil
}

func (b *FormulaBatch) open() error {
	if b == nil || b.self != b || b.manager == nil || b.closed {
		return ast.ErrASTHandle
	}
	return nil
}
func (b *FormulaBatch) Len() int {
	if b == nil || b.self != b || b.closed {
		return 0
	}
	return len(b.rows)
}
func (b *FormulaBatch) At(i int) (Formula, error) {
	if err := b.open(); err != nil {
		return Formula{}, err
	}
	if i < 0 || i >= len(b.rows) {
		return Formula{}, ast.ErrASTDomain
	}
	return b.rows[i], nil
}
func (b *FormulaBatch) Insert(row Formula) error {
	if err := b.open(); err != nil {
		return err
	}
	if row.Expression.IsNull() {
		return ast.ErrASTHandle
	}
	if err := b.manager.IncRef(row.Expression); err != nil {
		return err
	}
	if !row.Proof.IsNull() {
		if err := b.manager.IncRef(row.Proof); err != nil {
			_ = b.manager.DecRef(row.Expression)
			return err
		}
	}
	if !row.Dependency.IsNull() {
		if err := b.manager.DependencyInc(row.Dependency); err != nil {
			if !row.Proof.IsNull() {
				_ = b.manager.DecRef(row.Proof)
			}
			_ = b.manager.DecRef(row.Expression)
			return err
		}
	}
	b.rows = append(b.rows, row)
	return nil
}
func (b *FormulaBatch) Reset() (result error) {
	if err := b.open(); err != nil {
		return err
	}
	for i := len(b.rows) - 1; i >= 0; i-- {
		row := &b.rows[i]
		if !row.Dependency.IsNull() {
			result = errors.Join(result, b.manager.DependencyDec(row.Dependency))
		}
		if !row.Proof.IsNull() {
			result = errors.Join(result, b.manager.DecRef(row.Proof))
		}
		result = errors.Join(result, b.manager.DecRef(row.Expression))
		*row = Formula{}
	}
	b.rows = b.rows[:0]
	return result
}
func (b *FormulaBatch) Close() error {
	if b == nil || b.self != b {
		return ast.ErrASTHandle
	}
	if b.closed {
		return nil
	}
	if err := b.Reset(); err != nil {
		return err
	}
	b.manager.Budget().Release(b.header)
	b.header, b.rows, b.closed = 0, nil, true
	return nil
}

// Finder discovers macros and repeatedly expands the remaining formulas.
type Finder struct {
	manager *ast.ASTManager
	macros  *Manager
	util    *Util
}

func NewFinder(m *ast.ASTManager, macros *Manager) (*Finder, error) {
	if m == nil || macros == nil || macros.manager != m {
		return nil, ast.ErrASTHandle
	}
	if err := macros.open(); err != nil {
		return nil, err
	}
	return &Finder{manager: m, macros: macros, util: macros.Util()}, nil
}

func (f *Finder) simpleMacro(term ast.ASTHandle) (declaration ast.ASTHandle, matched bool, err error) {
	kind, err := f.manager.GetKind(term)
	if err != nil || kind != ast.ASTQuantifier {
		return declaration, false, err
	}
	forall, err := f.manager.IsForall(term)
	if err != nil || !forall {
		return declaration, false, err
	}
	body, err := f.manager.GetQuantifierBody(term)
	if err != nil {
		return declaration, false, err
	}
	n, err := f.manager.GetNumDeclarations(term)
	if err != nil {
		return declaration, false, err
	}
	head, _, yes, err := f.util.IsSimpleMacro(body, n)
	if err != nil || !yes {
		return declaration, false, err
	}
	declaration, err = f.manager.GetDecl(head)
	return declaration, err == nil, err
}

func (f *Finder) freshLike(head ast.ASTHandle) (declaration ast.ASTHandle, arguments []ast.ASTHandle, err error) {
	original, err := f.manager.GetDecl(head)
	if err != nil {
		return declaration, nil, err
	}
	name, err := f.manager.GetASTName(original)
	if err != nil {
		return declaration, nil, err
	}
	arity, err := f.manager.GetArity(original)
	if err != nil {
		return declaration, nil, err
	}
	domain := make([]ast.ASTHandle, arity)
	for i := range domain {
		domain[i], err = f.manager.GetDomain(original, uint32(i))
		if err != nil {
			return declaration, nil, err
		}
	}
	rangeSort, err := f.manager.GetRange(original)
	if err != nil {
		return declaration, nil, err
	}
	declaration, err = f.manager.MkFreshFuncDecl(name, usymbol.Symbol{}, domain, rangeSort)
	if err != nil {
		return declaration, nil, err
	}
	n, err := f.manager.GetNumArgs(head)
	if err != nil {
		return declaration, nil, err
	}
	arguments = make([]ast.ASTHandle, n)
	for i := range arguments {
		arguments[i], err = f.manager.GetArg(head, uint32(i))
		if err != nil {
			return declaration, nil, err
		}
	}
	return declaration, arguments, nil
}

func (f *Finder) splitPseudoPredicate(q, proof ast.ASTHandle, dependency ast.ASTDependency, head, value, definition ast.ASTHandle, output *FormulaBatch) error {
	declaration, arguments, err := f.freshLike(head)
	if err != nil {
		return err
	}
	otherwise, err := f.manager.MkApp(declaration, arguments)
	if err != nil {
		return err
	}
	choice, err := f.manager.MkIte(definition, value, otherwise)
	if err != nil {
		return err
	}
	body1, err := f.manager.MkEq(head, choice)
	if err != nil {
		return err
	}
	equal, err := f.manager.MkEq(otherwise, value)
	if err != nil {
		return err
	}
	body2, err := f.manager.MkNot(equal)
	if err != nil {
		return err
	}
	q1, err := f.manager.UpdateQuantifierBody(q, body1)
	if err != nil {
		return err
	}
	pattern, err := f.manager.MkPattern([]ast.ASTHandle{otherwise})
	if err != nil {
		return err
	}
	q2, err := f.manager.UpdateQuantifierPatterns(q, []ast.ASTHandle{pattern}, body2)
	if err != nil {
		return err
	}
	var proof1, proof2 ast.ASTHandle
	proofs, err := f.manager.ProofsEnabled()
	if err != nil {
		return err
	}
	if proofs {
		conjunction, e := f.manager.MkAnd(q1, q2)
		if e != nil {
			return e
		}
		rewrite, e := f.manager.MkProofOEqRewrite(q, conjunction)
		if e != nil {
			return e
		}
		combined, e := f.manager.MkProofModusPonens(proof, rewrite)
		if e != nil {
			return e
		}
		proof1, e = f.manager.MkProofAndElim(combined, 0)
		if e != nil {
			return e
		}
		proof2, e = f.manager.MkProofAndElim(combined, 1)
		if e != nil {
			return e
		}
	}
	if err = output.Insert(Formula{Expression: q1, Proof: proof1, Dependency: dependency}); err != nil {
		return err
	}
	return output.Insert(Formula{Expression: q2, Proof: proof2, Dependency: dependency})
}

func (f *Finder) arithmeticMacro(q, proof ast.ASTHandle, dependency ast.ASTDependency, output *FormulaBatch) (bool, error) {
	kind, err := f.manager.GetKind(q)
	if err != nil || kind != ast.ASTQuantifier {
		return false, err
	}
	forall, err := f.manager.IsForall(q)
	if err != nil || !forall {
		return false, err
	}
	body, err := f.manager.GetQuantifierBody(q)
	if err != nil {
		return false, err
	}
	lhs, err := f.manager.GetArg(body, 0)
	if err != nil {
		return false, nil
	}
	add, err := f.util.IsAdd(lhs)
	if err != nil || !add {
		return false, err
	}
	declarations, err := f.manager.GetNumDeclarations(q)
	if err != nil {
		return false, err
	}
	head, definition, inverse, matched, err := f.util.IsArithmeticMacro(body, declarations)
	if err != nil || !matched {
		return false, err
	}
	family, err := f.manager.GetFamilyID(body)
	if err != nil {
		return false, err
	}
	op, err := f.manager.GetDeclKind(body)
	if err != nil {
		return false, err
	}
	equality := family == f.manager.BasicFamilyID() && op == ast.ASTOpEq
	newOperation := op
	if inverse && !equality {
		if op == macroArithLE {
			newOperation = macroArithGE
		} else {
			newOperation = macroArithLE
		}
	}
	newBody, err := f.manager.MkFamilyApp(family, newOperation, nil, []ast.ASTHandle{head, definition})
	if err != nil {
		return false, err
	}
	newQ, err := f.manager.UpdateQuantifierBody(q, newBody)
	if err != nil {
		return false, err
	}
	newProof := ast.ASTHandle{}
	proofs, err := f.manager.ProofsEnabled()
	if err != nil {
		return false, err
	}
	if proofs {
		rewrite, e := f.manager.MkProofRewrite(q, newQ)
		if e != nil {
			return false, e
		}
		newProof, e = f.manager.MkProofModusPonens(proof, rewrite)
		if e != nil {
			return false, e
		}
	}
	if equality {
		declaration, e := f.manager.GetDecl(head)
		if e != nil {
			return false, e
		}
		return f.macros.Insert(declaration, newQ, newProof, dependency)
	}
	declaration, arguments, err := f.freshLike(head)
	if err != nil {
		return false, err
	}
	slack, err := f.manager.MkApp(declaration, arguments)
	if err != nil {
		return false, err
	}
	right, err := f.util.MkAdd(definition, slack)
	if err != nil {
		return false, err
	}
	body1, err := f.manager.MkEq(head, right)
	if err != nil {
		return false, err
	}
	sort, err := f.manager.GetSort(slack)
	if err != nil {
		return false, err
	}
	zero, err := f.util.MkZero(sort)
	if err != nil {
		return false, err
	}
	body2, err := f.manager.MkFamilyApp(family, newOperation, nil, []ast.ASTHandle{slack, zero})
	if err != nil {
		return false, err
	}
	q1, err := f.manager.UpdateQuantifierBody(newQ, body1)
	if err != nil {
		return false, err
	}
	pattern, err := f.manager.MkPattern([]ast.ASTHandle{slack})
	if err != nil {
		return false, err
	}
	q2, err := f.manager.UpdateQuantifierPatterns(newQ, []ast.ASTHandle{pattern}, body2)
	if err != nil {
		return false, err
	}
	var proof1, proof2 ast.ASTHandle
	if proofs {
		conjunction, e := f.manager.MkAnd(q1, q2)
		if e != nil {
			return false, e
		}
		rewrite, e := f.manager.MkProofOEqRewrite(newQ, conjunction)
		if e != nil {
			return false, e
		}
		combined, e := f.manager.MkProofModusPonens(newProof, rewrite)
		if e != nil {
			return false, e
		}
		proof1, e = f.manager.MkProofAndElim(combined, 0)
		if e != nil {
			return false, e
		}
		proof2, e = f.manager.MkProofAndElim(combined, 1)
		if e != nil {
			return false, e
		}
	}
	if err = output.Insert(Formula{Expression: q1, Proof: proof1, Dependency: dependency}); err != nil {
		return false, err
	}
	if err = output.Insert(Formula{Expression: q2, Proof: proof2, Dependency: dependency}); err != nil {
		return false, err
	}
	return true, nil
}

func (f *Finder) pass(input *FormulaBatch) (output *FormulaBatch, found bool, result error) {
	output, err := NewFormulaBatch(f.manager)
	if err != nil {
		return nil, false, err
	}
	fail := func(e error) (*FormulaBatch, bool, error) { return nil, false, errors.Join(e, output.Close()) }
	proofs, err := f.manager.ProofsEnabled()
	if err != nil {
		return fail(err)
	}
	for _, row := range input.rows {
		out, e := f.manager.NewRef(ast.ASTHandle{})
		if e != nil {
			return fail(e)
		}
		outProof, e := f.manager.NewRef(ast.ASTHandle{})
		if e != nil {
			_ = f.manager.DelRef(&out)
			return fail(e)
		}
		proof := row.Proof
		if !proofs {
			proof = ast.ASTHandle{}
		}
		dependency, e := f.macros.ExpandMacros(row.Expression, proof, row.Dependency, out, outProof)
		if e != nil {
			e = errors.Join(e, f.manager.DelRef(&outProof), f.manager.DelRef(&out))
			return fail(e)
		}
		expression, e := f.manager.RefGet(out)
		if e != nil {
			if !dependency.IsNull() {
				e = errors.Join(e, f.manager.DependencyDec(dependency))
			}
			e = errors.Join(e, f.manager.DelRef(&outProof), f.manager.DelRef(&out))
			return fail(e)
		}
		newProof, e := f.manager.RefGet(outProof)
		if e != nil {
			if !dependency.IsNull() {
				e = errors.Join(e, f.manager.DependencyDec(dependency))
			}
			e = errors.Join(e, f.manager.DelRef(&outProof), f.manager.DelRef(&out))
			return fail(e)
		}
		declaration, simple, e := f.simpleMacro(expression)
		if e == nil && simple {
			var inserted bool
			inserted, e = f.macros.Insert(declaration, expression, newProof, dependency)
			found = found || inserted
			if inserted {
				e = errors.Join(e, f.manager.DependencyDec(dependency), f.manager.DelRef(&outProof), f.manager.DelRef(&out))
				if e != nil {
					return fail(e)
				}
				continue
			}
		}
		if e == nil {
			arithmetic, x := f.arithmeticMacro(expression, newProof, dependency, output)
			e = x
			if e == nil && arithmetic {
				found = true
				if !dependency.IsNull() {
					e = errors.Join(e, f.manager.DependencyDec(dependency))
				}
				e = errors.Join(e, f.manager.DelRef(&outProof), f.manager.DelRef(&out))
				if e != nil {
					return fail(e)
				}
				continue
			}
		}
		if e == nil {
			head, value, definition, pseudo, x := f.util.IsPseudoPredicateMacro(expression)
			e = x
			if e == nil && pseudo {
				e = f.splitPseudoPredicate(expression, newProof, dependency, head, value, definition, output)
				found = found || e == nil
				if !dependency.IsNull() {
					e = errors.Join(e, f.manager.DependencyDec(dependency))
				}
				e = errors.Join(e, f.manager.DelRef(&outProof), f.manager.DelRef(&out))
				if e != nil {
					return fail(e)
				}
				continue
			}
		}
		if e == nil {
			e = output.Insert(Formula{Expression: expression, Proof: newProof, Dependency: dependency})
		}
		if !dependency.IsNull() {
			e = errors.Join(e, f.manager.DependencyDec(dependency))
		}
		e = errors.Join(e, f.manager.DelRef(&outProof), f.manager.DelRef(&out))
		if e != nil {
			return fail(e)
		}
	}
	return output, found, nil
}

// Apply borrows inputs and returns an owning batch. It repeats until a pass
// installs no new simple macro, matching the source fixed-point driver.
func (f *Finder) Apply(inputs []Formula) (current *FormulaBatch, result error) {
	if f == nil || f.manager == nil || f.macros == nil {
		return nil, ast.ErrASTHandle
	}
	current, err := NewFormulaBatch(f.manager)
	if err != nil {
		return nil, err
	}
	for _, input := range inputs {
		if err = current.Insert(input); err != nil {
			return nil, errors.Join(err, current.Close())
		}
	}
	for {
		next, found, err := f.pass(current)
		if err != nil {
			return nil, errors.Join(err, current.Close())
		}
		if err = current.Close(); err != nil {
			return nil, errors.Join(err, next.Close())
		}
		current = next
		if !found {
			return current, nil
		}
	}
}
