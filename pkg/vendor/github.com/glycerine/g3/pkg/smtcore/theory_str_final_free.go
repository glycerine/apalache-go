package smtcore

// Source: src/smt/theory_str.cpp final_check_eh free-variable/unroll suffix
// and process_free_var's retained tester-emission loops.

import "github.com/glycerine/g3/pkg/ast"

func (t *SMTStringTheory) assignStringFreeVariables(analysis *smtStringDependencyAnalysis) error {
	free := analysis.freeVariables
	boundedByUnroll := make(map[ast.ASTHandle][]ast.ASTHandle)
	for _, word := range stringOrderedCounts(free) {
		_, unrolls, err := t.stringClassUnrolls(word)
		if err != nil {
			return err
		}
		if len(unrolls) > 0 {
			boundedByUnroll[word] = unrolls
		}
	}
	for word := range boundedByUnroll {
		delete(free, word)
	}
	// This stage intentionally walks core enode cycles, not private String
	// classes used above and by simple-unroll collection below.
	concatUnrolls := make(map[ast.ASTHandle]map[ast.ASTHandle]int)
	m := t.state.manager
	for _, head := range stringOrderedRelations(analysis.unrollGroups) {
		start := t.state.context.enodes[head]
		if start == nil {
			return ast.ErrASTDomain
		}
		current := start
		for {
			term := current.Owner()
			_, _, concat, err := m.SequenceConcatParts(term)
			if err != nil {
				return err
			}
			if concat {
				stringPutRelation(concatUnrolls, term, head, 1)
				for member := range analysis.unrollGroups[head] {
					stringPutRelation(concatUnrolls, term, member, 1)
				}
			}
			current = current.Next()
			if current == start {
				break
			}
			if current == nil {
				return ast.ErrASTDomain
			}
		}
	}
	selected := make(map[ast.ASTHandle]map[ast.ASTHandle]int)
	unrollArguments := make(map[ast.ASTHandle]int)
	for _, concat := range stringOrderedRelations(concatUnrolls) {
		left, right, _, err := m.SequenceConcatParts(concat)
		if err != nil {
			return err
		}
		bounded := [2]bool{}
		for index, arg := range []ast.ASTHandle{left, right} {
			if _, variable := t.variableSet[arg]; variable {
				if _, available := free[arg]; !available {
					bounded[index] = true
				} else {
					unrollArguments[arg] = 1
				}
			} else {
				_, _, nested, err := m.SequenceConcatParts(arg)
				if err != nil {
					return err
				}
				if nested {
					if _, present := concatUnrolls[arg]; !present {
						bounded[index] = true
					}
				}
			}
		}
		if !bounded[0] && !bounded[1] {
			selected[concat] = concatUnrolls[concat]
		}
	}
	// Source removes every collected free argument, even when the other
	// argument prevented that concat from entering the selected map.
	for word := range unrollArguments {
		delete(free, word)
	}
	for _, concat := range stringOrderedRelations(selected) {
		for _, unroll := range stringOrderedCounts(selected[concat]) {
			if err := t.processStringConcatUnroll(concat, unroll); err != nil {
				return err
			}
		}
	}
	ordered, err := t.stringFreeVariableOrder(free)
	if err != nil {
		return err
	}
	for _, word := range ordered {
		root, err := t.generateStringLengthValueOptions(word, ast.ASTHandle{}, "")
		if err != nil {
			return err
		}
		if err = t.assertStringTesterOptions(root); err != nil {
			return err
		}
	}
	var removed []ast.ASTHandle
	for word := range boundedByUnroll {
		removed = append(removed, word)
	}
	stringSortHandles(removed)
	for _, word := range removed {
		_, simple, err := t.stringClassSimpleUnrolls(word)
		if err != nil {
			return err
		}
		if len(simple) == 0 {
			if err = t.assignStringRegexUnrolls(boundedByUnroll[word]); err != nil {
				return err
			}
		} else {
			root, err := t.generateStringLiteralUnrollAssignment(word, simple)
			if err != nil {
				return err
			}
			if err = t.assertStringTesterOptions(root); err != nil {
				return err
			}
		}
	}
	return nil
}
