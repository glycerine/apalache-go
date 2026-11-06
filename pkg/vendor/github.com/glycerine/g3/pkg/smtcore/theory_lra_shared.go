package smtcore

// Source: src/smt/theory_lra.cpp:1012-1145,1200-1243.

import (
	"fmt"
	"github.com/glycerine/g3/pkg/ast"
	"io"
)

type smtLRASharedValueKey struct {
	main, epsilon string
	integer       bool
}

func (t *SMTLRATheory) relevantShared(node *SMTEnode) (bool, error) {
	if t.state.relevancy != nil && !t.state.relevancy.IsRelevant(node.owner) {
		return false, nil
	}
	return t.sharedNode(node)
}

// Sharing is independent of relevancy in context::is_shared. Equality search
// filters relevancy first, while the diagnostic display reports both values.
func (t *SMTLRATheory) sharedNode(node *SMTEnode) (bool, error) {
	shared, consult, err := t.state.sharedTheoryNode(node)
	if err != nil || !consult || shared || len(t.underspecified) == 0 {
		return shared, err
	}
	root := node.Root()
	if err := t.emitLRATag("shared", "is_shared", 1220, func(out io.Writer) error {
		return smtWriteTheoryText(out, fmt.Sprintf("%d %d %d\n", t.state.scopeLevel, node.TheoryVar(t.FamilyID()), len(root.parents)))
	}); err != nil {
		return false, err
	}
	if len(root.parents) > 2*len(t.underspecified) {
		for _, term := range t.underspecified {
			n, err := t.state.manager.GetNumArgs(term)
			if err != nil {
				return false, err
			}
			for i := uint32(0); i < n; i++ {
				arg, err := t.state.manager.GetArg(term, i)
				if err != nil {
					return false, err
				}
				enode := t.state.context.enodes[arg]
				if enode == nil {
					return false, ast.ErrASTDomain
				}
				if enode.Root() == root {
					return true, nil
				}
			}
		}
	} else {
		for _, parent := range root.parents {
			underspecified, err := t.underspecifiedTerm(parent.owner)
			if err != nil || underspecified {
				return underspecified, err
			}
		}
	}
	return false, nil
}

func (t *SMTLRATheory) delayedAssumeEqualities() (bool, error) {
	for t.assumeHead < len(t.assumeCandidates) {
		pair := t.assumeCandidates[t.assumeHead]
		left, err := t.Enode(pair.left)
		if err != nil {
			return false, err
		}
		right, err := t.Enode(pair.right)
		if err != nil {
			return false, err
		}
		t.assumeHead++ // Source consumes a candidate before checking current values.
		for _, v := range []SMTTheoryVar{pair.left, pair.right} {
			index, ok := t.numeric.index(v)
			if !ok || !t.numeric.solver.VarIsRegistered(index) {
				return false, ast.ErrASTDomain
			}
		}
		if t.lraTraceTags().ShouldTrace("arith") {
			a, err := t.rawLRAValue(pair.left)
			if err != nil {
				return false, err
			}
			b, err := t.rawLRAValue(pair.right)
			if err != nil {
				return false, err
			}
			if a.Cmp(&b) == 0 && left.Root() != right.Root() {
				if err = t.emitLRA("delayed_assume_eqs", 1138, func(out io.Writer) error {
					return smtWriteTheoryText(out, fmt.Sprintf("assuming eq: v%d = v%d\n", pair.left, pair.right))
				}); err != nil {
					return false, err
				}
			}
		}
		a, err := t.rawLRAValue(pair.left)
		if err != nil {
			return false, err
		}
		b, err := t.rawLRAValue(pair.right)
		if err != nil {
			return false, err
		}
		if a.Cmp(&b) == 0 && left.Root() != right.Root() {
			if progress, err := t.state.assumeEquality(left, right); err != nil || progress {
				return progress, err
			}
		}
	}
	return false, nil
}

func (t *SMTLRATheory) assumeSharedEqualities() (progress bool, err error) {
	if _, err = t.State(); err != nil {
		return false, err
	}
	if err = t.numeric.ready(); err != nil {
		return false, err
	}
	var indices []uint32
	for v, node := range t.variables {
		shared, err := t.relevantShared(node)
		if err != nil {
			return false, err
		}
		if !shared {
			continue
		}
		index, ok := t.numeric.index(SMTTheoryVar(v))
		// The native direct directory access requires a mapped variable here.
		// Do not invent a new column or silently substitute zero.
		if !ok {
			return false, ast.ErrASTDomain
		}
		indices = append(indices, index)
	}
	if len(indices) == 0 {
		return false, nil
	}
	if err = t.emitLRA("assume_eqs", 1081, func(out io.Writer) error {
		for v, node := range t.variables {
			shared, err := t.relevantShared(node)
			if err != nil {
				return err
			}
			if !shared {
				continue
			}
			index, ok := t.numeric.index(SMTTheoryVar(v))
			if !ok {
				return ast.ErrASTDomain
			}
			if err = smtWriteTheoryText(out, fmt.Sprintf("v%d %d ", v, index)); err != nil {
				return err
			}
		}
		return smtWriteTheoryText(out, "\n")
	}); err != nil {
		return false, err
	}
	if err = t.numeric.solver.RandomUpdate(indices); err != nil {
		t.numeric.stopped = true
		return false, err
	}
	values := make(map[smtLRASharedValueKey]SMTTheoryVar)
	if err = t.emitLRA("assume_eqs", 1091, t.Display); err != nil {
		return false, err
	}
	size := len(t.variables)
	start := int(t.state.random.Next())
	for i := 0; i < size; i++ {
		v := SMTTheoryVar((i + start) % size)
		node := t.variables[v]
		shared, err := t.relevantShared(node)
		if err != nil {
			return false, err
		}
		if !shared {
			continue
		}
		index, ok := t.numeric.index(v)
		if !ok || !t.numeric.solver.VarIsRegistered(index) {
			continue
		}
		value, err := t.rawLRAValue(v)
		if err != nil {
			return false, err
		}
		sort, err := t.fixedKey(v, &value.Main)
		if err != nil {
			return false, err
		}
		key := smtLRASharedValueKey{value.Main.RatString(), value.Epsilon.RatString(), sort.integer}
		other, found := values[key]
		if !found {
			values[key] = v
			continue
		}
		if node.Root() != t.variables[other].Root() {
			if err = t.emitLRA("assume_eqs", 1111, func(out io.Writer) error {
				// Preserve the duplicated source equality line.
				for i := 0; i < 2; i++ {
					if err := t.displayLRAExpression(out, node.owner); err != nil {
						return err
					}
					if err := smtWriteTheoryText(out, " = "); err != nil {
						return err
					}
					if err := t.displayLRAExpression(out, t.variables[other].owner); err != nil {
						return err
					}
					if err := smtWriteTheoryText(out, "\n"); err != nil {
						return err
					}
				}
				return smtWriteTheoryText(out, fmt.Sprintf("v%d = v%d\n", v, other))
			}); err != nil {
				return false, err
			}
			t.assumeCandidates = append(t.assumeCandidates, smtArithmeticEqualityCandidate{v, other})
		}
	}
	return t.delayedAssumeEqualities()
}
