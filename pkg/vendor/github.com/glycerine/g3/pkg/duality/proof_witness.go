package duality

// Reconstruct original Horn-rule witnesses after FromClauses' QE-lite phase.
// A projected variable has no meaningful value in the counterexample model;
// solve its original rule against the already proved child/head arguments.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

var ErrProofWitness = errors.New("duality: original rule witness reconstruction failed")

type proofWitnessCall struct {
	manager      *ast.ASTManager
	proofScope   *ast.ScopedProofMode
	refs         []ast.ASTRef
	handles      []ast.ASTHandle
	terms        []*Expr
	substitution *rewriter.VarSubstitution
	native       solver.Solver
	model        *model.ModelCore
	evaluator    *model.ModelEvaluator
}

func (g *RPFP) cleanupProofWitnesses() error {
	for len(g.pendingProofWitnesses) > 0 {
		if err := g.pendingProofWitnesses[0].close(); err != nil {
			return err
		}
		g.pendingProofWitnesses = g.pendingProofWitnesses[1:]
	}
	return nil
}

func (g *RPFP) insertPendingProofWitness(index int, call *proofWitnessCall) {
	g.pendingProofWitnesses = append(g.pendingProofWitnesses, nil)
	copy(g.pendingProofWitnesses[index+1:], g.pendingProofWitnesses[index:])
	g.pendingProofWitnesses[index] = call
}

func (c *proofWitnessCall) close() error {
	if c.proofScope != nil {
		if err := c.proofScope.Close(); err != nil {
			return err
		}
		c.proofScope = nil
	}
	if c.evaluator != nil {
		if err := c.evaluator.Close(); err != nil {
			return err
		}
		c.evaluator = nil
	}
	if c.model != nil {
		consumed, err := c.model.DecRefConsumed()
		if consumed {
			c.model = nil
		}
		if err != nil {
			return err
		}
	}
	if c.native != nil {
		if err := c.native.Close(); err != nil {
			return err
		}
		c.native = nil
	}
	if c.substitution != nil {
		if err := c.substitution.Close(); err != nil {
			return err
		}
		c.substitution = nil
	}
	for len(c.terms) > 0 {
		i := len(c.terms) - 1
		err := c.terms[i].Close()
		if c.terms[i] == nil || c.terms[i].value == nil || c.terms[i].value.closed {
			c.terms = c.terms[:i]
		}
		if err != nil {
			return err
		}
	}
	if err := c.closeHandles(); err != nil {
		return err
	}
	for len(c.refs) > 0 {
		i := len(c.refs) - 1
		err := c.manager.DelRef(&c.refs[i])
		if c.refs[i] == (ast.ASTRef{}) {
			c.refs = c.refs[:i]
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *proofWitnessCall) closeHandles() error {
	for len(c.handles) > 0 {
		i := len(c.handles) - 1
		consumed, err := c.manager.DecRefConsumed(c.handles[i])
		if consumed {
			c.handles[i] = ast.ASTHandle{}
			c.handles = c.handles[:i]
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *proofWitnessCall) pin(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ref, err := c.manager.NewRef(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	c.refs = append(c.refs, ref)
	return h, nil
}

func (g *RPFP) recoverProofWitness(factory solver.Factory, quantified, clause ast.ASTHandle, positions [][]uint32, resolved []ast.ASTProofPosition, premises []ast.ASTHandle, conclusion ast.ASTHandle, directStructuralWitness bool, background ...*Expr) (out []ast.ASTRef, result error) {
	if factory == nil {
		return nil, ErrWrapperOwner
	}
	m := g.context.manager
	c := &proofWitnessCall{manager: m}
	var values []ast.ASTRef
	defer func() {
		pending := false
		if err := c.close(); err != nil {
			pending = true
			result = errors.Join(result, err)
		}
		if result != nil {
			for i := range values {
				result = errors.Join(result, m.DelRef(&values[i]))
				if values[i] != (ast.ASTRef{}) {
					c.refs = append(c.refs, values[i])
					pending = true
				}
			}
			out = nil
		}
		if pending {
			g.pendingProofWitnesses = append(g.pendingProofWitnesses, c)
		}
	}()
	n, err := m.GetNumDeclarations(quantified)
	if err != nil {
		return nil, err
	}
	skolem := make([]ast.ASTHandle, n)
	for j := uint32(0); j < n; j++ {
		sort, err := m.GetDeclarationSort(quantified, j)
		if err != nil {
			return nil, err
		}
		skolem[n-1-j], err = c.pin(m.MkFreshConst("@proof", sort))
		if err != nil {
			return nil, err
		}
	}
	c.substitution, err = rewriter.NewVarSubstitution(m, false)
	if err != nil {
		return nil, err
	}
	instantiated, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, err
	}
	c.refs = append(c.refs, instantiated)
	if err := c.substitution.Substitute(clause, skolem, instantiated); err != nil {
		return nil, err
	}
	inst, err := m.RefGet(instantiated)
	if err != nil {
		return nil, err
	}
	body, head := ast.ASTHandle{}, inst
	family, err := m.GetFamilyID(inst)
	if err != nil {
		return nil, err
	}
	op, err := m.GetDeclKind(inst)
	if err != nil {
		return nil, err
	}
	if family == m.BasicFamilyID() && op == ast.ASTOpImplies {
		body, err = m.GetArg(inst, 0)
		if err != nil {
			return nil, err
		}
		head, err = m.GetArg(inst, 1)
		if err != nil {
			return nil, err
		}
	} else {
		body, err = m.MkTrue()
		if err != nil {
			return nil, err
		}
	}
	atoms := []ast.ASTHandle{body}
	family, err = m.GetFamilyID(body)
	if err != nil {
		return nil, err
	}
	op, err = m.GetDeclKind(body)
	if err != nil {
		return nil, err
	}
	if family == m.BasicFamilyID() && op == ast.ASTOpAnd {
		count, err := m.GetNumArgs(body)
		if err != nil {
			return nil, err
		}
		atoms = make([]ast.ASTHandle, count)
		for i := range atoms {
			atoms[i], err = m.GetArg(body, uint32(i))
			if err != nil {
				return nil, err
			}
		}
	}
	truth, err := m.MkTrue()
	if err != nil {
		return nil, err
	}
	falsity, err := m.MkFalse()
	if err != nil {
		return nil, err
	}
	replacements := make(map[ast.ASTHandle]ast.ASTHandle)
	for _, group := range positions {
		for _, position := range group {
			if position == 0 || int(position) > len(atoms) {
				return nil, ErrProofWitness
			}
			replacements[atoms[position-1]] = falsity
		}
	}
	var constraints []ast.ASTHandle
	direct := make(map[ast.ASTHandle]ast.ASTHandle, len(skolem))
	match := func(application, fact ast.ASTHandle, capture bool) error {
		d1, err := m.GetDecl(application)
		if err != nil {
			return err
		}
		d2, err := m.GetDecl(fact)
		if err != nil {
			return err
		}
		if d1 != d2 {
			return ErrProofWitness
		}
		n, err := m.GetNumArgs(application)
		if err != nil {
			return err
		}
		for j := uint32(0); j < n; j++ {
			a, err := m.GetArg(application, j)
			if err != nil {
				return err
			}
			b, err := m.GetArg(fact, j)
			if err != nil {
				return err
			}
			eq, err := c.pin(m.MkEq(a, b))
			if err != nil {
				return err
			}
			constraints = append(constraints, eq)
			if capture && directStructuralWitness {
				direct[a] = b
			}
		}
		return nil
	}
	for i, position := range resolved {
		if position.First == 0 || int(position.First) > len(atoms) || i+1 >= len(premises) {
			return nil, ErrProofWitness
		}
		atom := atoms[position.First-1]
		fact, err := m.GetProofFact(premises[i+1])
		if err != nil {
			return nil, err
		}
		if err := match(atom, fact, directStructuralWitness); err != nil {
			return nil, err
		}
		replacements[atom] = truth
	}
	if err := match(head, conclusion, true); err != nil {
		return nil, err
	}
	condition, err := substituteExpression(g.context, body, replacements)
	if err != nil {
		return nil, err
	}
	c.refs = append(c.refs, condition)
	constraint, err := m.RefGet(condition)
	if err != nil {
		return nil, err
	}
	constraints = append(constraints, constraint)
	if directStructuralWitness {
		skolemSet := make(map[ast.ASTHandle]bool, len(skolem))
		for _, constant := range skolem {
			skolemSet[constant] = true
		}
		var known func(ast.ASTHandle) (ast.ASTHandle, bool, error)
		known = func(term ast.ASTHandle) (ast.ASTHandle, bool, error) {
			if skolemSet[term] {
				value := direct[term]
				return value, !value.IsNull(), nil
			}
			kind, err := m.GetKind(term)
			if err != nil {
				return ast.ASTHandle{}, false, err
			}
			if kind != ast.ASTApp {
				return term, true, nil
			}
			count, err := m.GetNumArgs(term)
			if err != nil {
				return ast.ASTHandle{}, false, err
			}
			args := make([]ast.ASTHandle, count)
			changed := false
			for i := range args {
				original, err := m.GetArg(term, uint32(i))
				if err != nil {
					return ast.ASTHandle{}, false, err
				}
				value, complete, err := known(original)
				if err != nil || !complete {
					return ast.ASTHandle{}, false, err
				}
				args[i] = value
				changed = changed || value != original
			}
			if !changed {
				return term, true, nil
			}
			decl, err := m.GetDecl(term)
			if err != nil {
				return ast.ASTHandle{}, false, err
			}
			value, err := c.pin(m.MkApp(decl, args))
			return value, err == nil, err
		}
		var bind func(ast.ASTHandle, ast.ASTHandle) (bool, bool, error)
		bind = func(pattern, value ast.ASTHandle) (bool, bool, error) {
			if skolemSet[pattern] {
				if assigned := direct[pattern]; !assigned.IsNull() {
					return assigned == value, false, nil
				}
				patternSort, err := m.GetSort(pattern)
				if err != nil {
					return false, false, err
				}
				valueSort, err := m.GetSort(value)
				if err != nil || patternSort != valueSort {
					return false, false, err
				}
				direct[pattern] = value
				return true, true, nil
			}
			kind, err := m.GetKind(pattern)
			if err != nil {
				return false, false, err
			}
			if kind != ast.ASTApp {
				return pattern == value, false, nil
			}
			valueKind, err := m.GetKind(value)
			if err != nil || valueKind != ast.ASTApp {
				return false, false, err
			}
			patternDecl, err := m.GetDecl(pattern)
			if err != nil {
				return false, false, err
			}
			valueDecl, err := m.GetDecl(value)
			if err != nil || patternDecl != valueDecl {
				return false, false, err
			}
			count, err := m.GetNumArgs(pattern)
			if err != nil {
				return false, false, err
			}
			valueCount, err := m.GetNumArgs(value)
			if err != nil || count != valueCount {
				return false, false, err
			}
			changed := false
			for i := uint32(0); i < count; i++ {
				patternArg, err := m.GetArg(pattern, i)
				if err != nil {
					return false, false, err
				}
				valueArg, err := m.GetArg(value, i)
				if err != nil {
					return false, false, err
				}
				matched, added, err := bind(patternArg, valueArg)
				if err != nil || !matched {
					return false, false, err
				}
				changed = changed || added
			}
			return true, changed, nil
		}
		tryBind := func(pattern, value ast.ASTHandle) (bool, error) {
			before := make(map[ast.ASTHandle]ast.ASTHandle, len(direct))
			for key, assigned := range direct {
				before[key] = assigned
			}
			matched, changed, err := bind(pattern, value)
			if err != nil || !matched {
				direct = before
				return false, err
			}
			return changed, nil
		}
		headArgs, err := m.GetNumArgs(head)
		if err != nil {
			return nil, err
		}
		for i := uint32(0); i < headArgs; i++ {
			pattern, err := m.GetArg(head, i)
			if err != nil {
				return nil, err
			}
			value, err := m.GetArg(conclusion, i)
			if err != nil {
				return nil, err
			}
			if _, err := tryBind(pattern, value); err != nil {
				return nil, err
			}
		}
		progress := true
		for progress {
			progress = false
			for _, atom := range atoms {
				family, err := m.GetFamilyID(atom)
				if err != nil {
					return nil, err
				}
				op, err := m.GetDeclKind(atom)
				if err != nil || family != m.BasicFamilyID() || op != ast.ASTOpEq {
					continue
				}
				left, err := m.GetArg(atom, 0)
				if err != nil {
					return nil, err
				}
				right, err := m.GetArg(atom, 1)
				if err != nil {
					return nil, err
				}
				if value, complete, err := known(left); err != nil {
					return nil, err
				} else if complete {
					added, err := tryBind(right, value)
					if err != nil {
						return nil, err
					}
					progress = progress || added
				}
				if value, complete, err := known(right); err != nil {
					return nil, err
				} else if complete {
					added, err := tryBind(left, value)
					if err != nil {
						return nil, err
					}
					progress = progress || added
				}
			}
		}
		complete := true
		for _, constant := range skolem {
			complete = complete && !direct[constant].IsNull()
		}
		if complete {
			for _, constant := range skolem {
				value, err := m.NewRef(direct[constant])
				if err != nil {
					return nil, err
				}
				values = append(values, value)
			}
			return values, nil
		}
	}
	c.native, err = factory.New(m, nil, false, true, false, "")
	if err != nil {
		return nil, err
	}
	for _, axiom := range background {
		if err := axiom.open(); err != nil {
			return nil, err
		}
		if axiom.value.object.context != g.context {
			return nil, ErrContextMismatch
		}
		h, err := axiom.Raw()
		if err != nil {
			return nil, err
		}
		if err := c.native.Assert(h); err != nil {
			return nil, err
		}
	}
	for _, constraint := range constraints {
		if err := c.native.Assert(constraint); err != nil {
			return nil, err
		}
	}
	status, err := c.native.CheckSat(nil)
	if err != nil {
		return nil, err
	}
	if status != lbool.True {
		return nil, ErrProofWitness
	}
	c.model, err = c.native.AcquireModel()
	if err != nil {
		return nil, err
	}
	if c.model == nil {
		return nil, ErrProofWitness
	}
	c.evaluator, err = model.NewModelEvaluator(c.model, g.evaluatorConfig)
	if err != nil {
		return nil, err
	}
	if err := c.evaluator.SetModelCompletion(true); err != nil {
		return nil, err
	}
	for _, constant := range skolem {
		value, err := c.evaluator.EvaluateValue(constant)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}
