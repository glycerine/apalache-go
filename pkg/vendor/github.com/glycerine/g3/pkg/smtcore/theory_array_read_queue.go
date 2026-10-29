package smtcore

// Source: theory_array_base::assert_store_axiom2 and assert_store_axiom2_core.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

type smtArrayReadFingerprint struct {
	store   ast.ASTHandle
	indices []*SMTEnode
}

func (t *SMTArrayTheory) enqueueStoreReadAxiom(read, store ast.ASTHandle) error {
	_, err := t.queueStoreReadAxiom(read, store, false)
	return err
}

func (t *SMTArrayTheory) queueStoreReadAxiom(read, store ast.ASTHandle, upward bool) (bool, error) {
	s, err := t.State()
	if err != nil {
		return false, err
	}
	readNode, storeNode := s.context.enodes[read], s.context.enodes[store]
	if readNode == nil || storeNode == nil || len(readNode.arguments) < 2 || len(storeNode.arguments) != len(readNode.arguments)+1 {
		return false, ast.ErrASTDomain
	}
	readOp, readArray, err := t.arrayOperation(read)
	if err != nil {
		return false, err
	}
	storeOp, storeArray, err := t.arrayOperation(store)
	if err != nil {
		return false, err
	}
	readLine, storeLine := int32(208), int32(209)
	if upward {
		readLine, storeLine = 216, 217
	}
	if err = arraySourceAssert(readLine, "is_select(select)", func() bool { return readArray && readOp == ast.ASTArrayOpSelect }); err != nil {
		return false, err
	}
	if err = arraySourceAssert(storeLine, "is_store(store)", func() bool { return storeArray && storeOp == ast.ASTArrayOpStore }); err != nil {
		return false, err
	}
	if !readArray || readOp != ast.ASTArrayOpSelect || !storeArray || storeOp != ast.ASTArrayOpStore {
		return false, ast.ErrASTDomain
	}
	line, tag, label := 207, "array", "axiom 2a"
	if upward {
		line, tag, label = 215, "array_axiom2b", "axiom 2b"
	}
	if err = t.emitSimpleArrayTrace(tag, "instantiate_axiom2", line, nil, func(out io.Writer) error {
		readID, err := readNode.OwnerID()
		if err != nil {
			return err
		}
		storeID, err := storeNode.OwnerID()
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "%s: #%d #%d\n", label, readID, storeID)
		return err
	}); err != nil {
		return false, err
	}
	allEqual := true
	indices := make([]*SMTEnode, len(readNode.arguments)-1)
	for i := range indices {
		indices[i] = readNode.arguments[i+1]
		allEqual = allEqual && indices[i].root == storeNode.arguments[i+1].root
	}
	if allEqual {
		return false, nil
	}
	if !t.insertArrayReadFingerprint(store, indices) {
		if err = t.emitArrayTrace("array", "assert_store_axiom2", 205, nil, func(out io.Writer) error {
			storeID, err := storeNode.OwnerID()
			if err != nil {
				return err
			}
			readID, err := readNode.OwnerID()
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(out, "axiom already instantiated: #%d #%d\n", storeID, readID)
			return err
		}); err != nil {
			return false, err
		}
		return false, nil
	}
	if err = t.emitArrayTrace("array", "assert_store_axiom2", 201, nil, func(out io.Writer) error {
		_, err := io.WriteString(out, "adding axiom2 to todo queue\n")
		return err
	}); err != nil {
		return false, err
	}
	t.readTodo = append(t.readTodo, smtArrayAxiomPair{observer: read, array: store})
	if upward {
		t.statistics.expandedAxiom2++
	} else {
		t.statistics.axiom2++
	}
	return true, nil
}

// Source uses the same constructor/indices fingerprint shape for stores and
// maps. The concrete AST identity distinguishes those retained operations.
func (t *SMTArrayTheory) insertArrayReadFingerprint(constructor ast.ASTHandle, indices []*SMTEnode) bool {
	// fingerprint_set::insert checks the original tuple before replacing it
	// with current roots. An older fingerprint can still name a former root.
	for _, entry := range t.readFingerprints[constructor] {
		if smtFingerprintArgsEqual(entry.indices, indices) {
			return false
		}
	}
	roots := make([]*SMTEnode, len(indices))
	for i, node := range indices {
		roots[i] = node.root
	}
	for _, entry := range t.readFingerprints[constructor] {
		if smtFingerprintArgsEqual(entry.indices, roots) {
			return false
		}
	}
	entry := &smtArrayReadFingerprint{store: constructor, indices: roots}
	t.readFingerprints[constructor] = append(t.readFingerprints[constructor], entry)
	t.readFingerprintTrail = append(t.readFingerprintTrail, entry)
	return true
}

func (t *SMTArrayTheory) assertStoreReadAxiomCore(read, store ast.ASTHandle) error {
	s, err := t.State()
	if err != nil {
		return err
	}
	readNode, storeNode := s.context.enodes[read], s.context.enodes[store]
	if readNode == nil || storeNode == nil {
		return ast.ErrASTDomain
	}
	if err = t.emitArrayTrace("array", "assert_store_axiom2_core", 131, nil, func(out io.Writer) error {
		storeID, err := storeNode.OwnerID()
		if err != nil {
			return err
		}
		readID, err := readNode.OwnerID()
		if err != nil {
			return err
		}
		if _, err = fmt.Fprintf(out, "generating axiom2: #%d #%d\n", storeID, readID); err != nil {
			return err
		}
		if err := displayArrayTraceExpression(s.manager, out, store); err != nil {
			return err
		}
		if _, err := io.WriteString(out, "\n"); err != nil {
			return err
		}
		if err := displayArrayTraceExpression(s.manager, out, read); err != nil {
			return err
		}
		_, err = io.WriteString(out, "\n")
		return err
	}); err != nil {
		return err
	}
	var consequence SMTLiteral
	var consequenceExpr ast.ASTHandle
	initialized := false
	for i := 1; i < len(readNode.arguments); i++ {
		left, right := storeNode.arguments[i], readNode.arguments[i]
		if left.root == right.root {
			if err = t.emitArrayTrace("array_bug", "assert_store_axiom2_core", 162, nil, func(out io.Writer) error {
				_, err := io.WriteString(out, "indexes are equal... skipping...\n")
				return err
			}); err != nil {
				return err
			}
			continue
		}
		if !initialized {
			// Source uses select(store, indices), even when the observer is
			// a read of an equal alias or of the store's base (upward rule).
			args := []ast.ASTHandle{store}
			for _, index := range readNode.arguments[1:] {
				args = append(args, index.owner)
			}
			selected, err := t.makeArraySelect(args)
			if err != nil {
				return err
			}
			args[0] = storeNode.arguments[0].owner
			fallback, err := t.makeArraySelect(args)
			if err != nil {
				return err
			}
			if selected == fallback {
				if err = t.emitArrayTrace("array_bug", "assert_store_axiom2_core", 170, nil, func(out io.Writer) error {
					_, err := io.WriteString(out, "sel1 and sel2 are equal:\n")
					return err
				}); err != nil {
					return err
				}
				break
			}
			if err = t.traceArrayExpressions("array", "assert_store_axiom2_core", 174, "", selected, fallback); err != nil {
				return err
			}
			consequenceExpr, err = s.mkEqualityAtom(selected, fallback)
			if err != nil {
				return err
			}
			if err = s.Internalize(consequenceExpr, true); err != nil {
				return err
			}
			consequence, err = s.Literal(consequenceExpr)
			if err != nil {
				return err
			}
			initialized = true
		}
		antecedent, err := t.MkEqualityLiteral(left.owner, right.owner, true)
		if err != nil {
			return err
		}
		if antecedent.Var() >= SMTFirstBoolVar {
			atom := s.boolExpr[antecedent.Var()]
			if err = s.markRelevant(atom); err != nil {
				return err
			}
			if s.relevancy != nil {
				if err = s.relevancy.AddWatchTarget(atom, antecedent.Sign(), consequenceExpr); err != nil {
					return err
				}
			}
		} else if antecedent == SMTFalseLiteral {
			if err = s.markRelevant(consequenceExpr); err != nil {
				return err
			}
		}
		if err = t.emitArrayTrace("array", "assert_store_axiom2_core", 184, nil, func(out io.Writer) error {
			if _, err := io.WriteString(out, "asserting axiom2: "); err != nil {
				return err
			}
			if err := antecedent.DisplayCompact(out, s.manager, s.boolExpr); err != nil {
				return err
			}
			_, err := io.WriteString(out, "\n")
			return err
		}); err != nil {
			return err
		}
		if err = t.emitArrayTrace("array_map_bug", "assert_store_axiom2_core", 185, nil, func(out io.Writer) error {
			if _, err := io.WriteString(out, "axiom2:\n"); err != nil {
				return err
			}
			if err := displayArrayTraceExpression(s.manager, out, left.owner); err != nil {
				return err
			}
			if _, err := io.WriteString(out, "\n=\n"); err != nil {
				return err
			}
			if err := displayArrayTraceExpression(s.manager, out, right.owner); err != nil {
				return err
			}
			if _, err := io.WriteString(out, "\nimplies\n"); err != nil {
				return err
			}
			if err := displayArrayTraceExpression(s.manager, out, consequenceExpr); err != nil {
				return err
			}
			_, err := io.WriteString(out, "\n")
			return err
		}); err != nil {
			return err
		}
		if err = t.assertArrayAxiom([]SMTLiteral{antecedent, consequence}); err != nil {
			return err
		}
	}
	return nil
}
