package rel

// Source: dl_mk_similarity_compressor.cpp initial_comparator, operator(),
// and the enabled portion of process_class.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"sort"
)

type similarityClass struct {
	rules     []*base.Rule
	merge     bool
	constants []similarityConstant
}

// similarityClasses borrows rules and the source set. It reports groups in
// source comparator order; rules within each group are copied as borrowed
// pointers. It does not create facts or mutate the source rule set.
func similarityClasses(source *base.RuleSet, input []*base.Rule, threshold uint32) (classes []similarityClass, duplicates bool, err error) {
	return similarityClassesExcept(source, input, threshold, nil)
}

func similarityClassesExcept(source *base.RuleSet, input []*base.Rule, threshold uint32, protected map[ast.ASTHandle]bool) (classes []similarityClass, duplicates bool, err error) {
	if source == nil {
		return nil, false, base.ErrRuleSetOwner
	}
	if threshold <= 1 {
		return nil, false, ast.ErrASTDomain
	}
	if _, err = source.Len(); err != nil {
		return nil, false, err
	}
	rules := append([]*base.Rule(nil), input...)
	sort.Slice(rules, func(i, j int) bool {
		if err != nil {
			return false
		}
		var cmp int
		cmp, err = similarityRoughCompare(rules[i], rules[j])
		if err != nil {
			return false
		}
		if cmp == 0 {
			cmp, err = similarityTotalCompare(rules[i], rules[j], -1)
		}
		return err == nil && cmp > 0
	})
	if err != nil {
		return nil, false, err
	}
	for start := 0; start < len(rules); {
		end := start + 1
		for end < len(rules) {
			cmp, e := similarityRoughCompare(rules[end-1], rules[end])
			if e != nil {
				return nil, false, e
			}
			if cmp != 0 {
				break
			}
			end++
		}
		group := append([]*base.Rule(nil), rules[start:end]...)
		// Match the source's swap-with-last deletion, including its scan order.
		for i := 1; i < len(group); {
			cmp, e := similarityTotalCompare(group[i-1], group[i], -1)
			if e != nil {
				return nil, false, e
			}
			if cmp == 0 {
				group[i] = group[len(group)-1]
				group = group[:len(group)-1]
				duplicates = true
			} else {
				i++
			}
		}
		positions, e := similarityConstantPositions(group[0])
		if e != nil {
			return nil, false, e
		}
		head, e := group[0].HeadDeclaration()
		if e != nil {
			return nil, false, e
		}
		output, e := source.IsOutputPredicate(head)
		if e != nil {
			return nil, false, e
		}
		class := similarityClass{rules: group}
		if len(positions) > 0 && !output && !protected[head] && uint64(len(group)) > uint64(threshold) {
			class.constants, e = similarityVaryingConstants(group)
			if e != nil {
				return nil, false, e
			}
			if len(class.constants) == 0 {
				return nil, false, ast.ErrASTDomain
			}
			class.merge = true
		}
		classes = append(classes, class)
		start = end
	}
	return classes, duplicates, nil
}
