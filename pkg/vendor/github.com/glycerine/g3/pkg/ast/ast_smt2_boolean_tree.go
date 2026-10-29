package ast

// Source: src/ast/ast_smt2_pp.cpp:630-844,1037-1060.

import (
	uparams "github.com/glycerine/g3/pkg/util/params"
	"strconv"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (m *ASTManager) smt2FlatAssocCandidate(parent, child ASTHandle) (bool, error) {
	if err := m.lock(); err != nil {
		return false, err
	}
	defer m.mu.Unlock()
	p, err := m.requireKind(parent, ASTApp)
	if err != nil {
		return false, err
	}
	c, err := m.requireKind(child, ASTApp)
	if err != nil {
		return false, err
	}
	pd := m.appAt(p.slot).decl
	if pd != m.appAt(c.slot).decl {
		return false, nil
	}
	flags, err := m.funcFlags(pd)
	if err != nil {
		return false, err
	}
	return flags.LeftAssociative && flags.RightAssociative, nil
}

// SMT2PPBooleanTree builds the source child-before-parent documents for
// Boolean application trees, including flat_assoc and source-default aliases.
func (m *ASTManager) SMT2PPBooleanTree(root ASTHandle, names []usymbol.Symbol) (ASTHandle, error) {
	return m.smt2PPExprTree(root, names, true)
}

// SMT2PPExprProfile captures the source printer constructor's eight pp fields.
type SMT2PPExprProfile struct {
	Decimal                                bool
	DecimalPrecision                       uint32
	BVLiterals, FPRealLiterals, BVNegative bool
	MaxDepth, MinAliasSize                 uint32
	FlatAssoc                              bool
}

// SMT2PPExprConfig keeps legacy numeral options and an optional full profile.
type SMT2PPExprConfig struct {
	Decimal          bool
	DecimalPrecision uint32
	Profile          *SMT2PPExprProfile
	Globals          *uparams.GlobalParamsService
}

// SMT2PPBooleanTreeWithConfig prints a Boolean tree with the requested
// arithmetic numeral profile.
func (m *ASTManager) SMT2PPBooleanTreeWithConfig(root ASTHandle, names []usymbol.Symbol, config SMT2PPExprConfig) (ASTHandle, error) {
	return m.smt2PPExprTreeProfile(root, names, true, config)
}

// SMT2PPExprTree also accepts non-Boolean application roots reached by the
// well-sorted diagnostics. Pattern apps and quantifiers use their source
// document branches.
func (m *ASTManager) SMT2PPExprTree(root ASTHandle, names []usymbol.Symbol) (ASTHandle, error) {
	return m.smt2PPExprTree(root, names, false)
}

// SMT2PPExprTreeWithConfig prints an expression tree with the requested
// arithmetic numeral profile.
func (m *ASTManager) SMT2PPExprTreeWithConfig(root ASTHandle, names []usymbol.Symbol, config SMT2PPExprConfig) (ASTHandle, error) {
	return m.smt2PPExprTreeProfile(root, names, false, config)
}

type smt2PPInfo struct{ weight, depth, lets uint32 }

func (m *ASTManager) smt2PPExprTree(root ASTHandle, names []usymbol.Symbol, requireBool bool) (ASTHandle, error) {
	return m.smt2PPExprTreeProfile(root, names, requireBool, SMT2PPExprConfig{DecimalPrecision: 10})
}

func (m *ASTManager) smt2PPExprTreeProfile(root ASTHandle, names []usymbol.Symbol, requireBool bool, config SMT2PPExprConfig) (ASTHandle, error) {
	return m.smt2PPExprTreeWithInfoProfile(root, names, requireBool, nil, config)
}

func (m *ASTManager) smt2PPExprTreeWithInfo(root ASTHandle, names []usymbol.Symbol, requireBool bool, info *smt2PPInfo) (ASTHandle, error) {
	return m.smt2PPExprTreeWithInfoProfile(root, names, requireBool, info, SMT2PPExprConfig{DecimalPrecision: 10})
}

func (m *ASTManager) smt2PPExprTreeWithInfoProfile(root ASTHandle, names []usymbol.Symbol, requireBool bool, info *smt2PPInfo, config SMT2PPExprConfig) (ASTHandle, error) {
	return m.smt2PPExprTreeWithInfoAt(root, names, requireBool, info, 0, true, config)
}

func (m *ASTManager) smt2PPExprTreeWithInfoAt(root ASTHandle, names []usymbol.Symbol, requireBool bool, info *smt2PPInfo, enclosingAliases int, allowAliases bool, config SMT2PPExprConfig) (ASTHandle, error) {
	profile := SMT2PPExprProfile{Decimal: config.Decimal, DecimalPrecision: config.DecimalPrecision, BVLiterals: true, MaxDepth: 5, MinAliasSize: 10, FlatAssoc: true}
	if config.Profile != nil {
		profile = *config.Profile
	}
	boolSort, err := m.MkBoolSort()
	if err != nil {
		return ASTHandle{}, err
	}
	rootKind, err := m.GetKind(root)
	if err != nil {
		return ASTHandle{}, err
	}
	if rootKind == ASTQuantifier {
		format, err := m.smt2PPQuantifierRootWithInfoAt(root, names, info, enclosingAliases, allowAliases, config)
		if err != nil || !allowAliases {
			return format, err
		}
		// The source printer runs pp_let once more on the completed root,
		// after the quantifier has already wrapped its own body.
		return m.smt2PPLetAt(format, nil, enclosingAliases)
	}
	// shared_occs visits each compound node's children once, but records a
	// second incoming edge as shared. Its default profile ignores atomic nodes
	// and quantifier patterns, while visiting each quantifier body.
	occurrences := make(map[ASTHandle]uint32)
	var countOccurrences func(ASTHandle) error
	countOccurrences = func(h ASTHandle) error {
		kind, e := m.GetKind(h)
		if e != nil {
			return e
		}
		if kind == ASTQuantifier {
			occurrences[h]++
			if occurrences[h] > 1 {
				return nil
			}
			body, e := m.GetQuantifierBody(h)
			if e != nil {
				return e
			}
			return countOccurrences(body)
		}
		if kind != ASTApp {
			return nil
		}
		n, e := m.GetNumArgs(h)
		if e != nil || n == 0 {
			return e
		}
		occurrences[h]++
		if occurrences[h] > 1 {
			return nil
		}
		for i := uint32(0); i < n; i++ {
			child, e := m.GetArg(h, i)
			if e != nil {
				return e
			}
			if e = countOccurrences(child); e != nil {
				return e
			}
		}
		return nil
	}
	if err := countOccurrences(root); err != nil {
		return ASTHandle{}, err
	}

	// These are the source info_stack fields. A flattened application leaves
	// its child entries on the stack, so its own weight and depth disappear.
	type result struct {
		docs          []ASTHandle
		level         uint32
		weight, depth uint32
	}
	bindings := make([]smt2LetBinding, 0)
	aliases := make(map[ASTHandle]int)
	usedNames := make(map[string]bool)
	for _, name := range names {
		s, e := m.Symbols.String(name)
		if e != nil {
			return ASTHandle{}, e
		}
		usedNames[s] = true
	}
	nextAlias := uint32(1)
	registerAlias := func(h ASTHandle, format ASTHandle, level, depth, weight uint32, shared bool) (result, error) {
		for {
			name := "a!" + strconv.FormatUint(uint64(nextAlias), 10)
			nextAlias++
			if usedNames[name] {
				continue
			}
			if e := m.traceSMT2Alias(h, name, depth, weight, level, shared); e != nil {
				return result{}, e
			}
			aliases[h] = len(bindings)
			bindings = append(bindings, smt2LetBinding{name: name, format: format, level: level})
			aliasFormat, e := m.FormatString(name)
			return result{docs: []ASTHandle{aliasFormat}, level: level + 1, weight: 1, depth: 1}, e
		}
	}
	var visit func(ASTHandle, bool) (result, error)
	visit = func(h ASTHandle, flatten bool) (result, error) {
		if idx, found := aliases[h]; found {
			binding := bindings[idx]
			format, e := m.FormatString(binding.name)
			return result{docs: []ASTHandle{format}, level: binding.level + 1, weight: 1, depth: 1}, e
		}
		kind, e := m.GetKind(h)
		if e != nil {
			return result{}, e
		}
		if kind == ASTQuantifier {
			var childInfo smt2PPInfo
			format, e := m.smt2PPQuantifierRootWithInfoAt(h, names, &childInfo, enclosingAliases+len(bindings), allowAliases, config)
			if e != nil {
				return result{}, e
			}
			if allowAliases && h != root && (childInfo.depth >= profile.MaxDepth || occurrences[h] > 1) {
				return registerAlias(h, format, 0, childInfo.depth, childInfo.weight, occurrences[h] > 1)
			}
			return result{docs: []ASTHandle{format}, weight: childInfo.weight, depth: childInfo.depth}, nil
		}
		if kind != ASTVar && kind != ASTApp {
			return result{}, ErrASTDomain
		}
		sort, e := m.GetSort(h)
		if e != nil {
			return result{}, e
		}
		if requireBool && h == root && sort != boolSort {
			return result{}, ErrASTDomain
		}
		if kind == ASTVar {
			index, e := m.GetVarIndex(h)
			if e != nil {
				return result{}, e
			}
			format, e := m.SMT2PPVar(index, names)
			return result{docs: []ASTHandle{format}, weight: 1, depth: 1}, e
		}
		count, e := m.GetNumArgs(h)
		if e != nil {
			return result{}, e
		}
		if count == 0 {
			if sort != boolSort {
				format, matched, e := m.smt2PPArithLiteral(h, profile.Decimal, profile.DecimalPrecision)
				if e != nil {
					return result{}, e
				}
				if matched {
					return result{docs: []ASTHandle{format}, weight: 1, depth: 1}, nil
				}
				format, matched, e = m.SMT2PPStringLiteral(h)
				if e != nil {
					return result{}, e
				}
				if matched {
					return result{docs: []ASTHandle{format}, weight: 1, depth: 1}, nil
				}
				matched, e = m.IsBitVectorNumeral(h)
				if e != nil {
					return result{}, e
				}
				if matched {
					format, e = m.ppBitVectorLiteral(h, profile.BVNegative, profile.BVLiterals, config.Globals)
					return result{docs: []ASTHandle{format}, weight: 1, depth: 1}, e
				}
				matched, e = m.smt2IsFloatingPointNumeral(h)
				if e != nil {
					return result{}, e
				}
				if matched {
					format, e = m.ppFloatLiteral(h, profile.BVLiterals, profile.FPRealLiterals, config.Globals)
					return result{docs: []ASTHandle{format}, weight: 1, depth: 1}, e
				}
				format, matched, e = m.SMT2PPDatalogLiteral(h)
				if e != nil {
					return result{}, e
				}
				if matched {
					return result{docs: []ASTHandle{format}, weight: 1, depth: 1}, nil
				}
				// The source pp_const falls through to pp_fdecl for other
				// nullary built-ins, including indexed as-array declarations.
			}
			var labels []usymbol.Symbol
			labeled, e := m.IsLabelLit(h, &labels)
			if e != nil {
				return result{}, e
			}
			if labeled {
				body, e := m.FormatString("true")
				if e != nil {
					return result{}, e
				}
				format, e := m.SMT2PPLabels(true, labels, body)
				return result{docs: []ASTHandle{format}, weight: 1, depth: 1}, e
			}
			decl, e := m.GetDecl(h)
			if e != nil {
				return result{}, e
			}
			format, _, e := m.SMT2Fdecl(decl)
			return result{docs: []ASTHandle{format}, weight: 1, depth: 1}, e
		}
		args := make([]ASTHandle, 0, count)
		value := result{weight: 1, depth: 1}
		patternChildrenApps := true
		for i := uint32(0); i < count; i++ {
			child, e := m.GetArg(h, i)
			if e != nil {
				return result{}, e
			}
			childKind, e := m.GetKind(child)
			if e != nil {
				return result{}, e
			}
			if childKind != ASTApp {
				patternChildrenApps = false
			}
			flat := false
			if profile.FlatAssoc && childKind == ASTApp && occurrences[child] == 1 {
				flat, e = m.smt2FlatAssocCandidate(h, child)
				if e != nil {
					return result{}, e
				}
			}
			childResult, e := visit(child, flat)
			if e != nil {
				return result{}, e
			}
			if childResult.level > value.level {
				value.level = childResult.level
			}
			value.weight += childResult.weight
			if childResult.depth > value.depth {
				value.depth = childResult.depth
			}
			args = append(args, childResult.docs...)
		}
		if flatten {
			value.docs = args
			value.weight--
			return value, nil
		}
		value.depth++
		var positive bool
		var labels []usymbol.Symbol
		labeled, e := m.IsLabel(h, &positive, &labels)
		if e != nil {
			return result{}, e
		}
		var format ASTHandle
		if labeled {
			if len(args) != 1 {
				return result{}, ErrASTDomain
			}
			format, e = m.SMT2PPLabels(positive, labels, args[0])
		} else {
			family, familyErr := m.GetFamilyID(h)
			if familyErr != nil {
				return result{}, familyErr
			}
			declKind, kindErr := m.GetDeclKind(h)
			if kindErr != nil {
				return result{}, kindErr
			}
			if family == 2 && declKind == 0 && patternChildrenApps {
				format, e = m.FormatSeq5(args)
			} else {
				decl, declErr := m.GetDecl(h)
				if declErr != nil {
					return result{}, declErr
				}
				format, e = m.SMT2PPAppFormat(decl, args)
			}
		}
		if e != nil {
			return result{}, e
		}
		if allowAliases && h != root && (value.depth >= profile.MaxDepth || value.weight >= profile.MinAliasSize && occurrences[h] > 1) {
			return registerAlias(h, format, value.level, value.depth, value.weight, occurrences[h] > 1)
		}
		value.docs = []ASTHandle{format}
		return value, nil
	}
	printed, err := visit(root, false)
	if err != nil {
		return ASTHandle{}, err
	}
	if info != nil {
		*info = smt2PPInfo{weight: printed.weight, depth: printed.depth, lets: uint32(len(bindings))}
	}
	if !allowAliases {
		return printed.docs[0], nil
	}
	return m.smt2PPLetAt(printed.docs[0], bindings, enclosingAliases)
}
