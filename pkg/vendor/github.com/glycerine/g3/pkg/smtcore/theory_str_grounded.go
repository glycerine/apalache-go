package smtcore

// Source: src/smt/theory_str.cpp dealias_node, get_grounded_concats,
// is_partial_in_grounded_concat, check_subsequence and compute_contains.

import (
	"errors"
	"sort"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
)

type smtStringGroundedConcat struct {
	parts      []ast.ASTHandle
	conditions []ast.ASTHandle
}

// These are the concrete String dependency-analysis maps, borrowed for one
// compute_contains call. New formula and folded-literal ASTs have local roots.
type smtStringGrounding struct {
	theory          *SMTStringTheory
	variableAliases map[ast.ASTHandle]ast.ASTHandle
	concatAliases   map[ast.ASTHandle]ast.ASTHandle
	variableValues  map[ast.ASTHandle]ast.ASTHandle
	concatValues    map[ast.ASTHandle]ast.ASTHandle
	variableConcats map[ast.ASTHandle]map[ast.ASTHandle]int
	grounded        map[ast.ASTHandle][]smtStringGroundedConcat
	locals          smtStringLengthPremises
}

func newStringGrounding(theory *SMTStringTheory) *smtStringGrounding {
	return &smtStringGrounding{
		theory:          theory,
		variableAliases: make(map[ast.ASTHandle]ast.ASTHandle), concatAliases: make(map[ast.ASTHandle]ast.ASTHandle),
		variableValues: make(map[ast.ASTHandle]ast.ASTHandle), concatValues: make(map[ast.ASTHandle]ast.ASTHandle),
		variableConcats: make(map[ast.ASTHandle]map[ast.ASTHandle]int), grounded: make(map[ast.ASTHandle][]smtStringGroundedConcat),
		locals: smtStringLengthPremises{theory: theory},
	}
}

func (g *smtStringGrounding) close() error { return g.locals.close() }

func (g *smtStringGrounding) pin(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return value, err
	}
	return value, g.locals.add(value)
}

func (g *smtStringGrounding) dealias(expression ast.ASTHandle) (ast.ASTHandle, error) {
	if _, variable := g.theory.variableSet[expression]; variable {
		if alias, found := g.variableAliases[expression]; found {
			return alias, nil
		}
		return expression, nil
	}
	_, _, concat, err := g.theory.state.manager.SequenceConcatParts(expression)
	if err != nil {
		return expression, err
	}
	if concat {
		if alias, found := g.concatAliases[expression]; found {
			return alias, nil
		}
	}
	return expression, nil
}

func (g *smtStringGrounding) entries(expression ast.ASTHandle) []smtStringGroundedConcat {
	values, found := g.grounded[expression]
	if !found {
		// Preserve source operator[] insertion, including empty unroll entries.
		g.grounded[expression] = nil
	}
	return values
}

func (g *smtStringGrounding) insert(expression ast.ASTHandle, parts, conditions []ast.ASTHandle) {
	for _, previous := range g.grounded[expression] {
		if len(parts) != len(previous.parts) {
			continue
		}
		same := true
		for i := range parts {
			same = same && parts[i] == previous.parts[i]
		}
		if same {
			return // Source keeps the first sufficient condition set.
		}
	}
	unique := make(map[ast.ASTHandle]struct{}, len(conditions))
	var ordered []ast.ASTHandle
	for _, condition := range conditions {
		if _, seen := unique[condition]; !seen {
			unique[condition] = struct{}{}
			ordered = append(ordered, condition)
		}
	}
	stringSortHandles(ordered)
	g.grounded[expression] = append(g.grounded[expression], smtStringGroundedConcat{append([]ast.ASTHandle(nil), parts...), ordered})
	sort.Slice(g.grounded[expression], func(i, j int) bool {
		left, right := g.grounded[expression][i].parts, g.grounded[expression][j].parts
		for k := 0; k < len(left) && k < len(right); k++ {
			if left[k].Slot() != right[k].Slot() {
				return left[k].Slot() < right[k].Slot()
			}
			if left[k].Generation() != right[k].Generation() {
				return left[k].Generation() < right[k].Generation()
			}
		}
		return len(left) < len(right)
	})
}

func (g *smtStringGrounding) ground(expression ast.ASTHandle) error {
	t := g.theory
	m := t.state.manager
	op, own, err := t.operation(expression)
	if err != nil || own && op == 43 {
		return err // Native unroll is not grounded.
	}
	expression, err = g.dealias(expression)
	if err != nil {
		return err
	}
	if _, found := g.grounded[expression]; found {
		return nil
	}
	if _, literal, err := m.SequenceStringValue(expression); err != nil {
		return err
	} else if literal {
		g.insert(expression, []ast.ASTHandle{expression}, nil)
		return nil
	}
	left, right, concat, err := m.SequenceConcatParts(expression)
	if err != nil {
		return err
	}
	if concat {
		if value, found := g.concatValues[expression]; found {
			condition, err := g.pin(m.MkEq(expression, value))
			if err != nil {
				return err
			}
			g.insert(expression, []ast.ASTHandle{value}, []ast.ASTHandle{condition})
			return nil
		}
		leftAlias, err := g.dealias(left)
		if err != nil {
			return err
		}
		rightAlias, err := g.dealias(right)
		if err != nil {
			return err
		}
		if err := g.ground(leftAlias); err != nil {
			return err
		}
		if err := g.ground(rightAlias); err != nil {
			return err
		}
		leftEntries, rightEntries := g.entries(leftAlias), g.entries(rightAlias)
		for _, l := range leftEntries {
			for _, r := range rightEntries {
				parts := append([]ast.ASTHandle(nil), l.parts...)
				merge := false
				if len(l.parts) > 0 && len(r.parts) > 0 {
					_, leftLiteral, err := m.SequenceStringValue(l.parts[len(l.parts)-1])
					if err != nil {
						return err
					}
					_, rightLiteral, err := m.SequenceStringValue(r.parts[0])
					if err != nil {
						return err
					}
					merge = leftLiteral && rightLiteral
				}
				if merge {
					folded, err := g.pin(t.concatTerm(l.parts[len(l.parts)-1], r.parts[0]))
					if err != nil {
						return err
					}
					parts[len(parts)-1] = folded
					parts = append(parts, r.parts[1:]...)
				} else {
					parts = append(parts, r.parts...)
				}
				conditions := append([]ast.ASTHandle(nil), l.conditions...)
				if left != leftAlias {
					condition, err := g.pin(m.MkEq(left, leftAlias))
					if err != nil {
						return err
					}
					conditions = append(conditions, condition)
				}
				if right != rightAlias {
					condition, err := g.pin(m.MkEq(right, rightAlias))
					if err != nil {
						return err
					}
					conditions = append(conditions, condition)
				}
				conditions = append(conditions, r.conditions...)
				g.insert(expression, parts, conditions)
			}
		}
		return nil
	}
	if _, variable := t.variableSet[expression]; !variable {
		return nil
	}
	if value, found := g.variableValues[expression]; found {
		condition, err := g.pin(m.MkEq(expression, value))
		if err != nil {
			return err
		}
		g.insert(expression, []ast.ASTHandle{value}, []ast.ASTHandle{condition})
		return nil
	}
	if concats, found := g.variableConcats[expression]; found {
		var choices []ast.ASTHandle
		for choice := range concats {
			choices = append(choices, choice)
		}
		stringSortHandles(choices)
		if len(choices) == 0 {
			return ast.ErrASTDomain // Source dereferences begin() here.
		}
		choice, err := g.dealias(choices[0])
		if err != nil {
			return err
		}
		if err := g.ground(choice); err != nil {
			return err
		}
		for _, child := range g.entries(choice) {
			condition, err := g.pin(m.MkEq(expression, choice))
			if err != nil {
				return err
			}
			conditions := append([]ast.ASTHandle{condition}, child.conditions...)
			g.insert(expression, child.parts, conditions)
		}
		return nil
	}
	g.insert(expression, []ast.ASTHandle{expression}, nil)
	return nil
}

func (t *SMTStringTheory) partialInGroundedConcat(word, needle []ast.ASTHandle) (bool, error) {
	if len(word) == 0 || len(needle) == 0 || len(word) < len(needle) {
		return false, nil
	}
	m := t.state.manager
	if len(needle) == 1 {
		find, literal, err := m.SequenceStringValue(needle[0])
		if err != nil {
			return false, err
		}
		for _, part := range word {
			if !literal {
				if part == needle[0] {
					return true, nil
				}
				continue
			}
			text, constant, err := m.SequenceStringValue(part)
			if err != nil {
				return false, err
			}
			if constant && strings.Contains(text, find) {
				return true, nil
			}
		}
		return false, nil
	}
	head, headLiteral, err := m.SequenceStringValue(needle[0])
	if err != nil {
		return false, err
	}
	tail, tailLiteral, err := m.SequenceStringValue(needle[len(needle)-1])
	if err != nil {
		return false, err
	}
	for start := 0; start <= len(word)-len(needle); start++ {
		if headLiteral {
			text, literal, err := m.SequenceStringValue(word[start])
			if err != nil {
				return false, err
			}
			if literal {
				if !strings.HasSuffix(text, head) {
					continue
				}
			} else if needle[0] != word[start] {
				continue
			}
		}
		// Native does not compare a variable needle head. Preserve that branch.
		middle := true
		for i := 1; i < len(needle)-1; i++ {
			if needle[i] != word[start+i] {
				middle = false
				break
			}
		}
		if !middle {
			continue
		}
		last := word[start+len(needle)-1]
		if tailLiteral {
			text, literal, err := m.SequenceStringValue(last)
			if err != nil {
				return false, err
			}
			if literal && strings.HasPrefix(text, tail) {
				return true, nil
			}
		} else if needle[len(needle)-1] == last {
			return true, nil
		}
	}
	return false, nil
}

func (g *smtStringGrounding) checkSubsequence(word, wordAlias, needle, needleAlias, contains ast.ASTHandle) (result error) {
	for _, haystack := range g.entries(wordAlias) {
		for _, find := range g.entries(needleAlias) {
			matches, err := g.theory.partialInGroundedConcat(haystack.parts, find.parts)
			if err != nil {
				return err
			}
			if !matches {
				continue
			}
			premises := smtStringLengthPremises{theory: g.theory}
			err = func() (result error) {
				defer func() { result = errors.Join(result, premises.close()) }()
				if word != wordAlias {
					if err := premises.equate(word, wordAlias); err != nil {
						return err
					}
				}
				if needle != needleAlias {
					if err := premises.equate(needle, needleAlias); err != nil {
						return err
					}
				}
				for _, condition := range haystack.conditions {
					if err := premises.add(condition); err != nil {
						return err
					}
				}
				for _, condition := range find.conditions {
					if err := premises.add(condition); err != nil {
						return err
					}
				}
				return premises.imply(contains)
			}()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *smtStringGrounding) computeContains() error {
	for _, expression := range g.theory.contains {
		word, needle, matched, err := g.theory.state.manager.SequenceBinaryOperationArguments(expression, 5)
		if err != nil {
			return err
		}
		if !matched {
			return ast.ErrASTDomain
		}
		wordAlias, err := g.dealias(word)
		if err != nil {
			return err
		}
		needleAlias, err := g.dealias(needle)
		if err != nil {
			return err
		}
		if err := g.ground(wordAlias); err != nil {
			return err
		}
		if err := g.ground(needleAlias); err != nil {
			return err
		}
		if err := g.checkSubsequence(word, wordAlias, needle, needleAlias, expression); err != nil {
			return err
		}
	}
	return nil
}
