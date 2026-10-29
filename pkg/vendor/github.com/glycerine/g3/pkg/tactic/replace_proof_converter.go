package tactic

// Source: src/tactic/replace_proof_converter.cpp; src/tactic/replace_proof_converter.h.

import (
	"errors"
	"io"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
)

type replaceProofConverter struct {
	proofConverterBase
	manager *ast.ASTManager
	proofs  []ast.ASTRef
}

func NewReplaceProofConverter(m *ast.ASTManager) (*replaceProofConverter, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	c := &replaceProofConverter{manager: m}
	if err := c.init(c, m.Budget(), uint64(reflect.TypeOf(*c).Size())); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *replaceProofConverter) Insert(proof ast.ASTHandle) error {
	if err := c.open(); err != nil {
		return err
	}
	ref, err := c.manager.NewRef(proof)
	if err != nil {
		return err
	}
	c.proofs = append(c.proofs, ref)
	return nil
}

func (c *replaceProofConverter) Invert() error {
	if err := c.open(); err != nil {
		return err
	}
	for left, right := 0, len(c.proofs)-1; left < right; left, right = left+1, right-1 {
		c.proofs[left], c.proofs[right] = c.proofs[right], c.proofs[left]
	}
	return nil
}

func (c *replaceProofConverter) Convert(m *ast.ASTManager, sources []ast.ASTHandle, out ast.ASTRef) (result error) {
	if err := c.open(); err != nil {
		return err
	}
	if m != c.manager || len(sources) != 1 {
		return ast.ErrASTDomain
	}
	replacements := make(map[ast.ASTHandle]ast.ASTHandle, len(c.proofs))
	cache := make(map[ast.ASTHandle]ast.ASTHandle)
	for _, proofRef := range c.proofs {
		proof, e := m.RefGet(proofRef)
		if e != nil {
			return e
		}
		value, e := replaceProofExpressions(m, proof, replacements, cache)
		if e != nil {
			return e
		}
		fact, e := m.GetProofFact(proof)
		if e != nil {
			return e
		}
		asserted, e := m.MkProofAsserted(fact)
		if e != nil {
			return e
		}
		replacements[asserted] = value
		cache[asserted] = value
	}
	rewritten, err := replaceProofExpressions(m, sources[0], replacements, cache)
	if err != nil {
		return err
	}
	return m.SetRef(out, rewritten)
}

// replaceProofExpressions is the concrete map_proc reconstruction used by the
// source converter. Quantifiers and variables are cached as leaves; proof
// structure and asserted leaves are applications.
func replaceProofExpressions(m *ast.ASTManager, root ast.ASTHandle, replacements, cache map[ast.ASTHandle]ast.ASTHandle) (ast.ASTHandle, error) {
	if value, found := replacements[root]; found {
		return value, nil
	}
	if value, found := cache[root]; found {
		return value, nil
	}
	kind, err := m.GetKind(root)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if kind != ast.ASTApp {
		cache[root] = root
		return root, nil
	}
	n, err := m.GetNumArgs(root)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	args := make([]ast.ASTHandle, n)
	changed := false
	for i := uint32(0); i < n; i++ {
		arg, e := m.GetArg(root, i)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		args[i], e = replaceProofExpressions(m, arg, replacements, cache)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		changed = changed || args[i] != arg
	}
	value := root
	if changed {
		decl, e := m.GetDecl(root)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		value, e = m.RewriterMakeApp(decl, ast.SliceArguments(args))
		if e != nil {
			return ast.ASTHandle{}, e
		}
	}
	cache[root] = value
	return value, nil
}

func (c *replaceProofConverter) Translate(tr *ast.ASTTranslation) (ProofConverter, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if tr == nil || tr.From() != c.manager {
		return nil, ast.ErrASTHandle
	}
	translated, err := NewReplaceProofConverter(tr.To())
	if err != nil {
		return nil, err
	}
	for _, proofRef := range c.proofs {
		proof, e := c.manager.RefGet(proofRef)
		if e != nil {
			return nil, errors.Join(e, translated.Close())
		}
		proof, e = tr.Translate(proof)
		if e != nil {
			return nil, errors.Join(e, translated.Close())
		}
		if e = translated.Insert(proof); e != nil {
			return nil, errors.Join(e, translated.Close())
		}
	}
	return translated, nil
}

func (c *replaceProofConverter) Cancel() error { return c.open() }

func (c *replaceProofConverter) Display(io.Writer) error { return c.open() }

func (c *replaceProofConverter) Manager() *ast.ASTManager {
	if c.open() != nil {
		return nil
	}
	return c.manager
}

func (c *replaceProofConverter) Close() (result error) {
	if c == nil || c.self != &c.proofConverterBase {
		return ast.ErrASTHandle
	}
	if c.closed {
		return nil
	}
	for len(c.proofs) != 0 {
		last := len(c.proofs) - 1
		if err := c.manager.DelRef(&c.proofs[last]); err != nil {
			return err
		}
		c.proofs = c.proofs[:last]
	}
	c.proofs = nil
	c.manager = nil
	c.release()
	return nil
}
