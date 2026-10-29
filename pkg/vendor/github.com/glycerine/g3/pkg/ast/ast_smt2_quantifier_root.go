package ast

// Source: src/ast/ast_smt2_pp.cpp:872-902,904-917,922-989;
// src/ast/format.h:133-157.

import (
	"strconv"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// smt2PPQuantifierRoot prints source binder names, children and annotations.
func (m *ASTManager) smt2PPQuantifierRootWithInfo(q ASTHandle, outer []usymbol.Symbol, info *smt2PPInfo) (ASTHandle, error) {
	return m.smt2PPQuantifierRootWithInfoAt(q, outer, info, 0, true, SMT2PPExprConfig{DecimalPrecision: 10})
}

func (m *ASTManager) smt2PPQuantifierRootWithInfoAt(q ASTHandle, outer []usymbol.Symbol, info *smt2PPInfo, enclosingAliases int, allowAliases bool, config SMT2PPExprConfig) (ASTHandle, error) {
	count, err := m.GetNumDeclarations(q)
	if err != nil || count == 0 {
		return ASTHandle{}, ErrASTDomain
	}
	patterns, err := m.GetNumPatterns(q)
	if err != nil {
		return ASTHandle{}, err
	}
	noPatterns, err := m.GetNumNoPatterns(q)
	if err != nil {
		return ASTHandle{}, err
	}
	weight, err := m.GetQuantifierWeight(q)
	if err != nil {
		return ASTHandle{}, err
	}
	qid, skid, err := m.GetQuantifierIDs(q)
	if err != nil {
		return ASTHandle{}, err
	}
	used := make(map[string]bool, len(outer)+int(count))
	for _, name := range outer {
		spelling, e := m.smt2EnsureQuote(name)
		if e != nil {
			return ASTHandle{}, e
		}
		used[spelling] = true
	}
	binders := make([]usymbol.Symbol, 0, count)
	spellings := make([]string, 0, count)
	sorts := make([]ASTHandle, 0, count)
	for i := uint32(0); i < count; i++ {
		name, e := m.GetDeclarationName(q, i)
		if e != nil {
			return ASTHandle{}, e
		}
		if name.IsNull() {
			return ASTHandle{}, ErrASTDomain
		}
		spelling := ""
		prefix := "x"
		if !name.IsNumerical() {
			spelling, e = m.smt2EnsureQuote(name)
			if e != nil {
				return ASTHandle{}, e
			}
			prefix = spelling
		}
		if name.IsNumerical() || used[spelling] {
			for suffix := uint32(1); ; suffix++ {
				candidate := prefix + "!" + strconv.FormatUint(uint64(suffix), 10)
				if used[candidate] {
					if suffix == ^uint32(0) {
						return ASTHandle{}, ErrASTDomain
					}
					continue
				}
				name, e = m.Symbols.Intern(candidate)
				if e != nil {
					return ASTHandle{}, e
				}
				printed, quoteErr := m.smt2EnsureQuote(name)
				if quoteErr != nil {
					return ASTHandle{}, quoteErr
				}
				if printed != candidate {
					return ASTHandle{}, ErrASTDomain
				}
				spelling = candidate
				break
			}
		}
		used[spelling] = true
		binders = append(binders, name)
		sort, e := m.GetDeclarationSort(q, i)
		if e != nil {
			return ASTHandle{}, e
		}
		spellings = append(spellings, spelling)
		sorts = append(sorts, sort)
	}
	scopedNames := append(append([]usymbol.Symbol{}, outer...), binders...)
	printedPatterns := make([]ASTHandle, 0, patterns)
	for i := uint32(0); i < patterns; i++ {
		pattern, e := m.GetPattern(q, i)
		if e != nil {
			return ASTHandle{}, e
		}
		printed, e := m.smt2PPExprTreeWithInfoAt(pattern, scopedNames, false, nil, enclosingAliases, false, config)
		if e != nil {
			return ASTHandle{}, e
		}
		printedPatterns = append(printedPatterns, printed)
	}
	printedNoPatterns := make([]ASTHandle, 0, noPatterns)
	for i := uint32(0); i < noPatterns; i++ {
		pattern, e := m.GetNoPattern(q, i)
		if e != nil {
			return ASTHandle{}, e
		}
		printed, e := m.smt2PPExprTreeWithInfoAt(pattern, scopedNames, false, nil, enclosingAliases, false, config)
		if e != nil {
			return ASTHandle{}, e
		}
		printedNoPatterns = append(printedNoPatterns, printed)
	}
	body, err := m.GetQuantifierBody(q)
	if err != nil {
		return ASTHandle{}, err
	}
	var bodyInfo smt2PPInfo
	printedBody, err := m.smt2PPExprTreeWithInfoAt(body, scopedNames, false, &bodyInfo, enclosingAliases, allowAliases, config)
	if err != nil {
		return ASTHandle{}, err
	}
	if !allowAliases {
		// A pattern suppresses new aliases, but a quantifier encountered in
		// that pattern still executes its own pp_let checkpoint.
		printedBody, err = m.smt2PPLetAt(printedBody, nil, enclosingAliases)
		if err != nil {
			return ASTHandle{}, err
		}
	}
	if patterns != 0 || noPatterns != 0 || weight > 1 || !skid.IsNull() || !qid.IsNull() && !qid.IsNumerical() {
		parts := []ASTHandle{printedBody}
		for _, printed := range printedPatterns {
			attr, e := m.SMT2PPAttribute(":pattern ", printed)
			if e != nil {
				return ASTHandle{}, e
			}
			parts = append(parts, attr)
		}
		for _, printed := range printedNoPatterns {
			attr, e := m.SMT2PPAttribute(":no-pattern ", printed)
			if e != nil {
				return ASTHandle{}, e
			}
			parts = append(parts, attr)
		}
		if weight > 1 {
			attr, e := m.SMT2PPSimpleAttributeInt(":weight ", weight)
			if e != nil {
				return ASTHandle{}, e
			}
			parts = append(parts, attr)
		}
		if !skid.IsNull() {
			attr, e := m.SMT2PPSimpleAttributeSymbol(":skolemid ", skid)
			if e != nil {
				return ASTHandle{}, e
			}
			parts = append(parts, attr)
		}
		if !qid.IsNull() && !qid.IsNumerical() {
			attr, e := m.SMT2PPSimpleAttributeSymbol(":qid ", qid)
			if e != nil {
				return ASTHandle{}, e
			}
			parts = append(parts, attr)
		}
		printedBody, err = m.FormatSeq1("!", parts)
		if err != nil {
			return ASTHandle{}, err
		}
	}
	decls := make([]ASTHandle, 0, count)
	for i, sort := range sorts {
		printedSort, e := m.SMT2PPSort(sort)
		if e != nil {
			return ASTHandle{}, e
		}
		decl, e := m.FormatSeq1(spellings[i], []ASTHandle{printedSort})
		if e != nil {
			return ASTHandle{}, e
		}
		decls = append(decls, decl)
	}
	printedDecls, err := m.FormatSeq5(decls)
	if err != nil {
		return ASTHandle{}, err
	}
	forall, err := m.IsForall(q)
	if err != nil {
		return ASTHandle{}, err
	}
	header := "exists"
	if forall {
		header = "forall"
	}
	whole, err := m.FormatSeq3(header, []ASTHandle{printedDecls, printedBody}, 1, 2)
	if err != nil {
		return ASTHandle{}, err
	}
	if info != nil {
		*info = smt2PPInfo{weight: bodyInfo.weight + 2*count + 8*bodyInfo.lets, depth: bodyInfo.depth + 1}
	}
	return whole, nil
}
