package macros

// Source: src/ast/macros/macro_manager.cpp:216-348.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// macroExpanderConfig is the concrete macro_manager::macro_expander_cfg
// specialization. Replacement references remain pinned until its traversal is
// closed, and usedDependency owns its accumulated dependency root.
type macroExpanderConfig struct {
	rewriter.DefaultConfiguration
	owner          *Manager
	substitution   *rewriter.VarSubstitution
	trail          []ast.ASTRef
	usedDependency ast.ASTDependency
}

func (c *macroExpanderConfig) RewritePatterns() bool { return false }
func (c *macroExpanderConfig) FlatAssoc(ast.ASTHandle) (bool, error) {
	return false, nil
}

func (c *macroExpanderConfig) ReduceQuantifier(q, body ast.ASTHandle, patterns, noPatterns ast.AppArguments, out, proof ast.ASTRef) (bool, error) {
	m := c.owner.manager
	erase := false
	n, err := m.GetNumPatterns(q)
	if err != nil {
		return false, err
	}
	if n != patterns.Count() {
		erase = true
	}
	for i := uint32(0); !erase && i < n; i++ {
		old, e := m.GetPattern(q, i)
		if e != nil {
			return false, e
		}
		current, e := patterns.At(i)
		if e != nil {
			return false, e
		}
		erase = old != current
	}
	n, err = m.GetNumNoPatterns(q)
	if err != nil {
		return false, err
	}
	if n != noPatterns.Count() {
		erase = true
	}
	for i := uint32(0); !erase && i < n; i++ {
		old, e := m.GetNoPattern(q, i)
		if e != nil {
			return false, e
		}
		current, e := noPatterns.At(i)
		if e != nil {
			return false, e
		}
		erase = old != current
	}
	if !erase {
		return false, nil
	}
	updated, err := m.UpdateQuantifierAnnotations(q, nil, nil, body)
	if err != nil {
		return false, err
	}
	return true, m.SetRef(out, updated)
}

func (c *macroExpanderConfig) joinDependency(dependency ast.ASTDependency) error {
	if dependency.IsNull() {
		return nil
	}
	m := c.owner.manager
	joined, err := m.DependencyJoin(c.usedDependency, dependency)
	if err != nil {
		return err
	}
	if err = m.DependencyInc(joined); err != nil {
		return err
	}
	if !c.usedDependency.IsNull() {
		if err = m.DependencyDec(c.usedDependency); err != nil {
			_ = m.DependencyDec(joined)
			return err
		}
	}
	c.usedDependency = joined
	return nil
}

func (c *macroExpanderConfig) GetSubst(term ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	m := c.owner.manager
	kind, err := m.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	declaration, err := m.GetDecl(term)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	i, found := c.owner.byDeclaration[declaration]
	if !found {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, nil
	}
	entry := c.owner.entries[i]
	head, definition, err := c.owner.GetHeadDefinition(entry.quantifier, declaration)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	n, err := m.GetNumArgs(term)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	hn, err := m.GetNumArgs(head)
	if err != nil || hn != n {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, errors.Join(err, ast.ErrASTDomain)
	}
	arguments := make([]ast.ASTHandle, n)
	for j := uint32(0); j < n; j++ {
		variable, e := m.GetArg(head, j)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		index, e := m.GetVarIndex(variable)
		if e != nil || index >= n {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, errors.Join(e, ast.ErrASTDomain)
		}
		position := n - index - 1
		if !arguments[position].IsNull() {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, ast.ErrASTDomain
		}
		arguments[position], e = m.GetArg(term, j)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
	}
	replacement, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	if err = c.substitution.Substitute(definition, arguments, replacement); err != nil {
		_ = m.DelRef(&replacement)
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	c.trail = append(c.trail, replacement)
	result, err := m.RefGet(replacement)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	var proof ast.ASTHandle
	proofs, err := m.ProofsEnabled()
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	if proofs {
		body, e := m.GetQuantifierBody(entry.quantifier)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		instanceRef, e := m.NewRef(ast.ASTHandle{})
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		defer m.DelRef(&instanceRef)
		if e = c.substitution.Substitute(body, arguments, instanceRef); e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		instance, e := m.RefGet(instanceRef)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		notQuantifier, e := m.MkNot(entry.quantifier)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		clause, e := m.MkOr(notQuantifier, instance)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		quantInstantiation, e := m.MkProofQuantInstantiation(clause, arguments)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		if entry.proof.IsNull() {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, ast.ErrASTDomain
		}
		proof, e = m.MkProofUnitResolution([]ast.ASTHandle{quantInstantiation, entry.proof})
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
	}
	if err = c.joinDependency(entry.dependency); err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	return result, proof, true, nil
}

func (c *macroExpanderConfig) close() (result error) {
	for i := len(c.trail) - 1; i >= 0; i-- {
		result = errors.Join(result, c.owner.manager.DelRef(&c.trail[i]))
	}
	c.trail = nil
	if c.substitution != nil {
		result = errors.Join(result, c.substitution.Close())
		c.substitution = nil
	}
	if !c.usedDependency.IsNull() {
		result = errors.Join(result, c.owner.manager.DependencyDec(c.usedDependency))
		c.usedDependency = ast.ASTDependency{}
	}
	return result
}

type macroTheoryNormalizer struct {
	context *uparams.ParamsContext
	catalog *uparams.GlobalParamsCatalog
	globals *uparams.GlobalParamsService
	rewrite *rewriter.TheoryRewriter
}

func newMacroTheoryNormalizer(m *ast.ASTManager) (n *macroTheoryNormalizer, result error) {
	n = &macroTheoryNormalizer{}
	fail := func(err error) (*macroTheoryNormalizer, error) { return n, errors.Join(err, n.close()) }
	var err error
	n.context, err = uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget(), Text: arena.VectorConfig{Descriptors: arena.Config{Budget: m.Budget()}}})
	if err != nil {
		return fail(err)
	}
	n.catalog, err = uparams.NewGlobalParamsCatalog(m.Symbols, uparams.GlobalParamsCatalogConfig{Budget: m.Budget()})
	if err != nil {
		return fail(err)
	}
	n.globals, err = uparams.NewGlobalParamsService(m.Symbols, n.context, uparams.GlobalParamsConfig{Budget: m.Budget(), Catalog: n.catalog})
	if err != nil {
		return fail(err)
	}
	if err = n.globals.InitializeSource(); err != nil {
		return fail(err)
	}
	n.rewrite, err = rewriter.NewTheoryRewriter(m, n.context, n.globals, nil)
	if err != nil {
		return fail(err)
	}
	return n, nil
}

func (n *macroTheoryNormalizer) close() (result error) {
	if n == nil {
		return nil
	}
	if n.rewrite != nil {
		result = errors.Join(result, n.rewrite.Close())
		n.rewrite = nil
	}
	if n.globals != nil {
		result = errors.Join(result, n.globals.Close())
		n.globals = nil
	}
	if n.catalog != nil {
		result = errors.Join(result, n.catalog.Close())
		n.catalog = nil
	}
	if n.context != nil {
		result = errors.Join(result, n.context.Close())
		n.context = nil
	}
	return result
}

func replaceOwnedDependency(m *ast.ASTManager, current *ast.ASTDependency, added ast.ASTDependency) error {
	joined, err := m.DependencyJoin(*current, added)
	if err != nil {
		return err
	}
	if err = m.DependencyInc(joined); err != nil {
		return err
	}
	if !current.IsNull() {
		if err = m.DependencyDec(*current); err != nil {
			_ = m.DependencyDec(joined)
			return err
		}
	}
	*current = joined
	return nil
}

// ExpandMacros expands installed macros to a fixed point, then applies the
// retained theory rewriter when expansion changed the term. The returned
// dependency is owned by the caller and must be released with DependencyDec.
func (m *Manager) ExpandMacros(term, proof ast.ASTHandle, dependency ast.ASTDependency, out, outProof ast.ASTRef) (resultDependency ast.ASTDependency, result error) {
	if err := m.open(); err != nil {
		return resultDependency, err
	}
	if err := m.manager.SetRef(out, term); err != nil {
		return resultDependency, err
	}
	if err := m.manager.SetRef(outProof, proof); err != nil {
		return resultDependency, err
	}
	if !dependency.IsNull() {
		if err := m.manager.DependencyInc(dependency); err != nil {
			return resultDependency, err
		}
		resultDependency = dependency
	}
	defer func() {
		if result != nil && !resultDependency.IsNull() {
			result = errors.Join(result, m.manager.DependencyDec(resultDependency))
			resultDependency = ast.ASTDependency{}
		}
	}()
	if !m.HasMacros() {
		return resultDependency, nil
	}
	oldTerm, err := m.manager.NewRef(term)
	if err != nil {
		return resultDependency, err
	}
	defer func() { result = errors.Join(result, m.manager.DelRef(&oldTerm)) }()
	oldProof, err := m.manager.NewRef(proof)
	if err != nil {
		return resultDependency, err
	}
	defer func() { result = errors.Join(result, m.manager.DelRef(&oldProof)) }()
	proofs, err := m.manager.ProofsEnabled()
	if err != nil {
		return resultDependency, err
	}
	changed := false
	for {
		cfg := &macroExpanderConfig{owner: m}
		cfg.substitution, err = rewriter.NewVarSubstitution(m.manager, true)
		if err != nil {
			return resultDependency, errors.Join(err, cfg.close())
		}
		var traversal *rewriter.Traversal
		if proofs {
			traversal, err = rewriter.NewProofTraversal(m.manager, cfg)
		} else {
			traversal, err = rewriter.NewTraversal(m.manager, cfg)
		}
		if err != nil {
			return resultDependency, errors.Join(err, cfg.close())
		}
		old, e := m.manager.RefGet(oldTerm)
		if e == nil {
			if proofs {
				e = traversal.RewriteWithProof(old, out, outProof)
			} else {
				e = traversal.Rewrite(old, out)
			}
		}
		used := cfg.usedDependency
		if !used.IsNull() && e == nil {
			e = replaceOwnedDependency(m.manager, &resultDependency, used)
		}
		e = errors.Join(e, traversal.Close(), cfg.close())
		if e != nil {
			return resultDependency, e
		}
		if proofs {
			previous, x := m.manager.RefGet(oldProof)
			if x != nil {
				return resultDependency, x
			}
			equalityProof, x := m.manager.RefGet(outProof)
			if x != nil {
				return resultDependency, x
			}
			combined, x := m.manager.MkProofModusPonens(previous, equalityProof)
			if x != nil {
				return resultDependency, x
			}
			if x = m.manager.SetRef(outProof, combined); x != nil {
				return resultDependency, x
			}
		}
		current, e := m.manager.RefGet(out)
		if e != nil {
			return resultDependency, e
		}
		if current == old {
			break
		}
		changed = true
		if e = m.manager.AssignRef(oldTerm, out); e != nil {
			return resultDependency, e
		}
		if proofs {
			if x := m.manager.AssignRef(oldProof, outProof); x != nil {
				return resultDependency, x
			}
		}
	}
	if !changed {
		return resultDependency, nil
	}
	normalizer, err := newMacroTheoryNormalizer(m.manager)
	if err != nil {
		return resultDependency, err
	}
	defer func() { result = errors.Join(result, normalizer.close()) }()
	before, err := m.manager.RefGet(out)
	if err != nil {
		return resultDependency, err
	}
	if proofs {
		rewriteProof, e := m.manager.NewRef(ast.ASTHandle{})
		if e != nil {
			return resultDependency, e
		}
		defer func() { result = errors.Join(result, m.manager.DelRef(&rewriteProof)) }()
		if e = normalizer.rewrite.RewriteWithProof(before, out, rewriteProof); e != nil {
			return resultDependency, e
		}
		old, e := m.manager.RefGet(oldProof)
		if e != nil {
			return resultDependency, e
		}
		rwProof, e := m.manager.RefGet(rewriteProof)
		if e != nil {
			return resultDependency, e
		}
		combined, e := m.manager.MkProofModusPonens(old, rwProof)
		if e != nil {
			return resultDependency, e
		}
		if e = m.manager.SetRef(outProof, combined); e != nil {
			return resultDependency, e
		}
	} else if err = normalizer.rewrite.Rewrite(before, out); err != nil {
		return resultDependency, err
	}
	return resultDependency, nil
}
