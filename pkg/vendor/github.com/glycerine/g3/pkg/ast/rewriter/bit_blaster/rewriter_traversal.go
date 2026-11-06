package bit_blaster

// Source: src/ast/rewriter/bit_blaster/bit_blaster_rewriter.cpp:95-144,565-684;
// src/ast/rewriter/bit_blaster/bit_blaster_rewriter.h:22-41.

import (
	"errors"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	parent "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/messages"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type bitBlasterTraversalConfiguration struct {
	parent.DefaultConfiguration
	owner *BitBlasterRewriter
}

func (c *bitBlasterTraversalConfiguration) RewritePatterns() bool { return true }

func (c *bitBlasterTraversalConfiguration) MaxStepsExceeded(steps uint32) (bool, error) {
	if err := c.owner.open(); err != nil {
		return false, err
	}
	if c.owner.blaster.cooperation != nil {
		if err := c.owner.blaster.cooperation.Cooperate("bit blaster"); err != nil {
			return false, err
		}
	}
	used, err := c.owner.manager.NumeralService().RuntimeOwner().AllocationSize()
	if err != nil {
		return false, err
	}
	if used > memory.MegabytesToBytes(c.owner.params.MaxMemoryMB) {
		return false, parent.NewRewriterException(messages.MaxMemory)
	}
	return steps > c.owner.params.MaxSteps, nil
}

func (r *BitBlasterRewriter) releaseBindings(limit int) (result error) {
	if limit < 0 || limit > len(r.bindings) {
		return ast.ErrASTDomain
	}
	for len(r.bindings) > limit {
		i := len(r.bindings) - 1
		if err := r.manager.DelRef(&r.bindings[i]); err != nil {
			return err
		}
		r.bindings = r.bindings[:i]
	}
	return result
}

func (c *bitBlasterTraversalConfiguration) PreVisit(term ast.ASTHandle) (allowed bool, result error) {
	if !c.owner.params.BlastQuant {
		return true, nil
	}
	kind, err := c.owner.manager.GetKind(term)
	if err != nil || kind != ast.ASTQuantifier {
		return err == nil, err
	}
	m := c.owner.manager
	count, err := m.GetNumDeclarations(term)
	if err != nil {
		return false, err
	}
	boolSort, err := m.MkBoolSort()
	if err != nil {
		return false, err
	}
	start := len(c.owner.bindings)
	var reverse []ast.ASTRef
	defer func() {
		if result != nil {
			result = errors.Join(result, releaseRefs(m, reverse))
			result = errors.Join(result, c.owner.releaseBindings(start))
		}
	}()
	var variableIndex uint32
	for i := count; i > 0; {
		i--
		sort, err := m.GetDeclarationSort(term, i)
		if err != nil {
			return false, err
		}
		isBV, err := m.IsBitVectorSort(sort)
		if err != nil {
			return false, err
		}
		var replacement ast.ASTHandle
		if isBV {
			width, err := m.BitVectorSize(sort)
			if err != nil {
				return false, err
			}
			bits := make([]ast.ASTHandle, width)
			for j := range bits {
				bits[j], err = m.MkVar(variableIndex, boolSort)
				if err != nil {
					return false, err
				}
				variableIndex++
			}
			replacement, err = m.MkBitVectorFromBits(bits)
			if err != nil {
				return false, err
			}
		} else {
			replacement, err = m.MkVar(variableIndex, sort)
			if err != nil {
				return false, err
			}
			variableIndex++
		}
		ref, err := m.NewRef(replacement)
		if err != nil {
			return false, err
		}
		reverse = append(reverse, ref)
	}
	for i := len(reverse); i > 0; {
		i--
		c.owner.bindings = append(c.owner.bindings, reverse[i])
	}
	reverse = nil
	return true, nil
}

func (c *bitBlasterTraversalConfiguration) ReduceApp(decl ast.ASTHandle, args ast.AppArguments, out, proof ast.ASTRef) (parent.RewriteStatus, error) {
	if err := c.owner.manager.SetRef(proof, ast.ASTHandle{}); err != nil {
		return parent.RewriteFailed, err
	}
	return c.owner.reduceApp(decl, args, out)
}

func (c *bitBlasterTraversalConfiguration) ReduceVar(variable ast.ASTHandle, out, proof ast.ASTRef) (bool, error) {
	if err := c.owner.manager.SetRef(proof, ast.ASTHandle{}); err != nil {
		return false, err
	}
	if c.owner.params.BlastQuant {
		index, err := c.owner.manager.GetVarIndex(variable)
		if err != nil {
			return false, err
		}
		if index >= uint32(len(c.owner.bindings)) {
			return false, nil
		}
		binding := c.owner.bindings[len(c.owner.bindings)-int(index)-1]
		return true, c.owner.manager.AssignRef(out, binding)
	}
	if !c.owner.params.BlastFull {
		return false, nil
	}
	isBV, err := c.owner.manager.IsBitVector(variable)
	if err != nil || !isBV {
		return false, err
	}
	return true, c.owner.blastBVTerm(variable, out)
}

func argumentsToHandles(args ast.AppArguments) ([]ast.ASTHandle, error) {
	values := make([]ast.ASTHandle, args.Count())
	for i := range values {
		value, err := args.At(uint32(i))
		if err != nil {
			return nil, err
		}
		values[i] = value
	}
	return values, nil
}

func (c *bitBlasterTraversalConfiguration) ReduceQuantifier(old, body ast.ASTHandle, patterns, noPatterns ast.AppArguments, out, proof ast.ASTRef) (done bool, result error) {
	if !c.owner.params.BlastQuant {
		return false, nil
	}
	m := c.owner.manager
	count, err := m.GetNumDeclarations(old)
	if err != nil {
		return false, err
	}
	if uint64(count) > uint64(len(c.owner.bindings)) {
		return false, ast.ErrASTDomain
	}
	oldSize := len(c.owner.bindings) - int(count)
	defer func() { result = errors.Join(result, c.owner.releaseBindings(oldSize)) }()
	boolSort, err := m.MkBoolSort()
	if err != nil {
		return false, err
	}
	var sorts []ast.ASTHandle
	var names []usymbol.Symbol
	for i := uint32(0); i < count; i++ {
		sort, err := m.GetDeclarationSort(old, i)
		if err != nil {
			return false, err
		}
		name, err := m.GetDeclarationName(old, i)
		if err != nil {
			return false, err
		}
		isBV, err := m.IsBitVectorSort(sort)
		if err != nil {
			return false, err
		}
		if !isBV {
			sorts = append(sorts, sort)
			names = append(names, name)
			continue
		}
		width, err := m.BitVectorSize(sort)
		if err != nil {
			return false, err
		}
		text, err := m.Symbols.Display(name)
		if err != nil {
			return false, err
		}
		for j := uint32(0); j < width; j++ {
			bitName, err := m.Symbols.Intern(text + "." + strconv.FormatUint(uint64(j), 10))
			if err != nil {
				return false, err
			}
			sorts = append(sorts, boolSort)
			names = append(names, bitName)
		}
	}
	forall, err := m.IsForall(old)
	if err != nil {
		return false, err
	}
	weight, err := m.GetQuantifierWeight(old)
	if err != nil {
		return false, err
	}
	qid, skid, err := m.GetQuantifierIDs(old)
	if err != nil {
		return false, err
	}
	patternValues, err := argumentsToHandles(patterns)
	if err != nil {
		return false, err
	}
	noPatternValues, err := argumentsToHandles(noPatterns)
	if err != nil {
		return false, err
	}
	value, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: forall, Sorts: sorts, Names: names, Body: body, Weight: weight, QID: qid, SkolemID: skid, Patterns: patternValues, NoPatterns: noPatternValues})
	if err != nil {
		return false, err
	}
	if err = m.SetRef(out, value); err != nil {
		return false, err
	}
	if err = m.SetRef(proof, ast.ASTHandle{}); err != nil {
		return false, err
	}
	return true, nil
}

// Rewrite applies the retained traversal while discarding its proof output.
func (r *BitBlasterRewriter) Rewrite(root ast.ASTHandle, out ast.ASTRef) error {
	if err := r.open(); err != nil {
		return err
	}
	return r.traversal.Rewrite(root, out)
}

func (r *BitBlasterRewriter) RewriteWithProof(root ast.ASTHandle, out, proof ast.ASTRef) error {
	if err := r.open(); err != nil {
		return err
	}
	return r.traversal.RewriteWithProof(root, out, proof)
}

func (r *BitBlasterRewriter) NumSteps() (uint32, error) {
	if err := r.open(); err != nil {
		return 0, err
	}
	return r.traversal.NumSteps()
}

func (r *BitBlasterRewriter) Cleanup() error {
	if err := r.open(); err != nil {
		return err
	}
	return errors.Join(r.traversal.Cleanup(), r.releaseBindings(0))
}
