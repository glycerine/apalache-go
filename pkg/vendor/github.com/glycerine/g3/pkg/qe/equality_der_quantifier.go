package qe

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func (d *EqualityDER) quantifierArguments(q ast.ASTHandle) ([]ast.ASTHandle, bool, error) {
	body, err := d.manager.GetQuantifierBody(q)
	if err != nil {
		return nil, false, err
	}
	forall, err := d.manager.IsForall(q)
	if err != nil {
		return nil, false, err
	}
	op := ast.ASTOpAnd
	if forall {
		op = ast.ASTOpOr
	}
	flat, err := d.operation(body, d.manager.BasicFamilyID(), op)
	if err != nil {
		return nil, false, err
	}
	if flat {
		args, err := d.children(body)
		return args, !forall, err
	}
	return []ast.ASTHandle{body}, !forall, nil
}
func (d *EqualityDER) applyQuantifierSubstitution(c *equalityDERCall, q ast.ASTHandle, bindings []ast.ASTHandle) (ast.ASTHandle, error) {
	args, exists, err := d.quantifierArguments(q)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var remaining []ast.ASTHandle
	for i, h := range args {
		index := c.positions[i]
		if index < 0 || c.definitions[index].term.IsNull() {
			remaining = append(remaining, h)
		}
	}
	if len(remaining) == len(args) {
		return q, nil
	}
	rw, err := c.boolean()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	out, err := c.ref(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if exists {
		err = rw.MkAnd(remaining, out)
	} else {
		err = rw.MkOr(remaining, out)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	body, err := d.manager.RefGet(out)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	body, err = d.substitute(c, body, bindings)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	n, err := d.manager.GetNumPatterns(q)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	patterns := make([]ast.ASTHandle, n)
	for i := range patterns {
		h, err := d.manager.GetPattern(q, uint32(i))
		if err != nil {
			return ast.ASTHandle{}, err
		}
		patterns[i], err = d.substitute(c, h, bindings)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	n, err = d.manager.GetNumNoPatterns(q)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	noPatterns := make([]ast.ASTHandle, n)
	for i := range noPatterns {
		h, err := d.manager.GetNoPattern(q, uint32(i))
		if err != nil {
			return ast.ASTHandle{}, err
		}
		noPatterns[i], err = d.substitute(c, h, bindings)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	result, err := d.manager.UpdateQuantifierAnnotations(q, patterns, noPatterns, body)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.hold(result)
}
func (d *EqualityDER) reduceQuantifierOnce(c *equalityDERCall, q ast.ASTHandle) (ast.ASTHandle, error) {
	n, err := d.manager.GetNumDeclarations(q)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	selection, err := NewVariableDeclarationTest(d.manager, n)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	c.selectors = append(c.selectors, selection)
	d.selection = selection
	args, exists, err := d.quantifierArguments(q)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	count, err := d.findDefinitions(c, args, exists)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if count == 0 {
		err = d.emit("der_bug", "reduce_quantifier1", 542, func(out io.Writer) error {
			_, _ = io.WriteString(out, "Did not find any diseq\n")
			if err := writeExpression(d.manager, d.config, out, q); err != nil {
				return err
			}
			_, _ = io.WriteString(out, "\n")
			return nil
		})
		return q, err
	}
	if err = d.eliminationOrder(c); err != nil {
		return ast.ASTHandle{}, err
	}
	if len(c.order) == 0 {
		return q, nil
	}
	bindings, err := d.createSubstitution(c)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return d.applyQuantifierSubstitution(c, q, bindings)
}
func (d *EqualityDER) reduceQuantifier(c *equalityDERCall, q ast.ASTHandle, out, proof ast.ASTRef) error {
	if _, err := c.hold(q); err != nil {
		return err
	}
	if err := d.manager.SetRef(out, q); err != nil {
		return err
	}
	proofs, err := d.manager.ProofsEnabled()
	if err != nil {
		return err
	}
	current := q
	for {
		if err = d.checkpoint(); err != nil {
			return err
		}
		next, err := d.reduceQuantifierOnce(c, current)
		if err != nil {
			return err
		}
		if err = d.manager.SetRef(out, next); err != nil {
			return err
		}
		if next == current {
			return nil
		}
		if proofs {
			step, err := d.manager.MkProofDER(current, next)
			if err != nil {
				return err
			}
			if step, err = c.hold(step); err != nil {
				return err
			}
			previous, err := d.manager.RefGet(proof)
			if err != nil {
				return err
			}
			chain, err := d.manager.MkProofTransitivity(previous, step)
			if err != nil {
				return err
			}
			if err = d.manager.SetRef(proof, chain); err != nil {
				return err
			}
		}
		current = next
		kind, err := d.manager.GetKind(current)
		if err != nil {
			return err
		}
		if kind != ast.ASTQuantifier {
			return nil
		}
	}
}

// ReduceQuantifier preserves the declaration vector, as required by QE-lite's
// constant-abstraction overload. Proof is accumulated into the supplied ref.
func (d *EqualityDER) ReduceQuantifier(q ast.ASTHandle, out, proof ast.ASTRef) (err error) {
	if err = d.check(); err != nil {
		return err
	}
	if err = d.drain(); err != nil {
		return err
	}
	c := &equalityDERCall{owner: d}
	d.pending = append(d.pending, c)
	defer d.finish(c, &err)
	return d.reduceQuantifier(c, q, out, proof)
}

// ApplyQuantifier is eq::der::operator()(quantifier*,expr_ref&,proof_ref&):
// reduce to a fixed point, then remove unused declarations with constructor p.
func (d *EqualityDER) ApplyQuantifier(q ast.ASTHandle, out, proof ast.ASTRef) (err error) {
	if err = d.check(); err != nil {
		return err
	}
	if err = d.drain(); err != nil {
		return err
	}
	c := &equalityDERCall{owner: d}
	d.pending = append(d.pending, c)
	defer d.finish(c, &err)
	if err = d.traceExpression("operator()", 790, q); err != nil {
		return err
	}
	if err = d.manager.ResetRef(proof); err != nil {
		return err
	}
	if err = d.reduceQuantifier(c, q, out, proof); err != nil {
		return err
	}
	current, err := d.manager.RefGet(out)
	if err != nil {
		return err
	}
	if current == q {
		return nil
	}
	kind, err := d.manager.GetKind(current)
	if err != nil || kind != ast.ASTQuantifier {
		return err
	}
	removed, err := rewriter.ElimUnusedVarsWithParams(d.manager, current, d.config.Context, &d.params)
	if removed != (ast.ASTRef{}) {
		c.refs = append(c.refs, removed)
	}
	if err != nil {
		return err
	}
	result, err := d.manager.RefGet(removed)
	if err != nil {
		return err
	}
	if err = d.manager.SetRef(out, result); err != nil {
		return err
	}
	proofs, err := d.manager.ProofsEnabled()
	if err != nil || !proofs {
		return err
	}
	step, err := d.manager.MkProofElimUnusedVars(current, result)
	if err != nil {
		return err
	}
	if step, err = c.hold(step); err != nil {
		return err
	}
	previous, err := d.manager.RefGet(proof)
	if err != nil {
		return err
	}
	chain, err := d.manager.MkProofTransitivity(previous, step)
	if err != nil {
		return err
	}
	return d.manager.SetRef(proof, chain)
}
