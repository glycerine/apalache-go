package fp

// Source: dl_relation_manager.cpp mixed join fallback, dl_product_relation.cpp,
// and instr_io/instr_union's inference of an empty target from its source kind.
// This public consumer accepts extensional unary DOC plus a finite Bool/BV
// input, one acyclic rule per mixed head, and query auxiliaries. Ground rules
// may populate unary DOC, all-Bool finite inputs, and unseeded Bool/BV products
// assembled under favourite DOC; broader programs keep the existing finite path.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

type udocProductFilter struct {
	column uint32
	value  ast.ASTHandle
}
type udocProductMembership struct {
	declaration                ast.ASTHandle
	productColumns, docColumns []uint32
	product                    bool
}
type udocProductRulePlan struct {
	head, doc, finite, source ast.ASTHandle
	pair                      *udocProductPairPlan
	multiStart                ast.ASTHandle
	multiStartKind            uint8
	multi                     []udocProductMultiInput
	docFirst                  bool
	docColumns, finiteColumns []uint32
	filters                   []udocProductFilter
	docMemberships            []udocProductMembership
	identical                 [][]uint32
	removed                   []uint32
	cycles                    [][]uint32
	output                    []uint32
	inputKind                 uint8 // 0 product/mixed, 1 original DOC, 2 original finite
	ruleIndex                 int
}

type udocProductMultiInput struct {
	declaration               ast.ASTHandle
	kind                      uint8 // 0 product, 1 DOC, 2 finite table
	sorts                     []ast.ASTHandle
	leftColumns, rightColumns []uint32
}

func similarityMembershipTail(m *ast.ASTManager, rule *base.Rule, positive int, extraDocs, extraProducts map[ast.ASTHandle][]ast.ASTHandle) (int, ast.ASTHandle, error) {
	if positive == 0 {
		return -1, ast.ASTHandle{}, nil
	}
	tail, _, err := rule.Tail(positive - 1)
	if err != nil {
		return -1, ast.ASTHandle{}, err
	}
	declaration, err := m.GetDecl(tail)
	if err != nil {
		return -1, ast.ASTHandle{}, err
	}
	_, rawDoc := extraDocs[declaration]
	_, product := extraProducts[declaration]
	if !rawDoc && !product {
		return -1, ast.ASTHandle{}, nil
	}
	return positive - 1, tail, nil
}

func productDocMembership(m *ast.ASTManager, arguments []ast.ASTHandle, tail ast.ASTHandle, declaration ast.ASTHandle, product bool) (udocProductMembership, []ast.ASTHandle, error) {
	arity, err := m.GetNumArgs(tail)
	if err != nil {
		return udocProductMembership{}, nil, err
	}
	if arity != 1 && !product {
		return udocProductMembership{}, nil, ErrUDocDatalogFragment
	}
	membership := udocProductMembership{declaration: declaration, product: product, productColumns: make([]uint32, 0, arity), docColumns: make([]uint32, 0, arity)}
	auxiliaryArguments := make([]ast.ASTHandle, arity)
	for column := uint32(0); column < arity; column++ {
		argument, err := m.GetArg(tail, column)
		if err != nil {
			return udocProductMembership{}, nil, err
		}
		kind, err := m.GetKind(argument)
		if err != nil || kind != ast.ASTVar {
			return udocProductMembership{}, nil, errors.Join(err, ErrUDocDatalogFragment)
		}
		auxiliaryArguments[column] = argument
		for productColumn, candidate := range arguments {
			if candidate == argument {
				membership.productColumns = append(membership.productColumns, uint32(productColumn))
				membership.docColumns = append(membership.docColumns, column)
				break
			}
		}
	}
	return membership, auxiliaryArguments, nil
}

type udocProductProgram struct {
	rules           map[ast.ASTHandle]*udocProductRulePlan
	sorts           map[ast.ASTHandle][]ast.ASTHandle
	finiteSorts     map[ast.ASTHandle][]ast.ASTHandle
	docs, finite    map[ast.ASTHandle]bool
	order           []ast.ASTHandle
	groundFacts     []*udocProductGroundRule
	groundProducts  map[ast.ASTHandle]int  // Mixed ground input -> its BV column.
	productSeeds    map[ast.ASTHandle]bool // Similarity auxiliaries inheriting product kind.
	docProductTails map[ast.ASTHandle]int
}

func (p *udocProductProgram) productHead(d ast.ASTHandle) bool {
	if p.productSeeds[d] {
		return true
	}
	if _, found := p.groundProducts[d]; found {
		return true
	}
	plan := p.rules[d]
	return plan != nil && plan.inputKind == 0
}

func udocProductFiniteKind(table rel.FiniteTable) string {
	switch table.(type) {
	case *rel.HashTable:
		return "hashtable"
	case *rel.SparseTable:
		return "sparse"
	case *rel.LazySparseTable:
		return "lazy_sparse"
	}
	return ""
}

func productArguments(m *ast.ASTManager, app ast.ASTHandle) ([]ast.ASTHandle, error) {
	count, err := m.GetNumArgs(app)
	if err != nil {
		return nil, err
	}
	args := make([]ast.ASTHandle, count)
	for i := range args {
		args[i], err = m.GetArg(app, uint32(i))
		if err != nil {
			return nil, err
		}
	}
	return args, nil
}

func planUDocProductAssembly(m *ast.ASTManager, plan *udocProductRulePlan, arguments []ast.ASTHandle, head ast.ASTHandle, allowConstants bool) error {
	variables := make(map[uint32]uint32)
	for column, argument := range arguments {
		kind, err := m.GetKind(argument)
		if err != nil {
			return err
		}
		if kind != ast.ASTVar {
			value, err := m.IsValue(argument)
			if err != nil {
				return err
			}
			if !allowConstants || !value {
				return ErrUDocDatalogFragment
			}
			plan.filters = append(plan.filters, udocProductFilter{uint32(column), argument})
			continue
		}
		index, err := m.GetVarIndex(argument)
		if err != nil {
			return err
		}
		if prior, found := variables[index]; found {
			if allowConstants {
				plan.identical = append(plan.identical, []uint32{prior, uint32(column)})
			}
		} else {
			variables[index] = uint32(column)
		}
	}
	args, err := productArguments(m, head)
	if err != nil {
		return err
	}
	wanted, used := make([]uint32, len(args)), make(map[uint32]bool)
	for i, arg := range args {
		kind, err := m.GetKind(arg)
		if err != nil {
			return err
		}
		if kind != ast.ASTVar {
			return ErrUDocDatalogFragment
		}
		index, err := m.GetVarIndex(arg)
		if err != nil {
			return err
		}
		column, found := variables[index]
		if !found || used[column] {
			return ErrUDocDatalogFragment
		}
		wanted[i], used[column] = column, true
	}
	var current []uint32
	for column := range arguments {
		if used[uint32(column)] {
			current = append(current, uint32(column))
		} else {
			plan.removed = append(plan.removed, uint32(column))
		}
	}
	for destination, want := range wanted {
		position := destination
		for current[position] != want {
			position++
		}
		if position != destination {
			plan.cycles = append(plan.cycles, []uint32{uint32(destination), uint32(position)})
			current[destination], current[position] = current[position], current[destination]
		}
	}
	plan.output = wanted
	return nil
}

func (e *PropositionalEngine) planUDocProducts(factory *rel.UDocRelationFactory, docFavourite bool, known map[ast.ASTHandle][]ast.ASTHandle) (program *udocProductProgram, resultErr error) {
	return e.planUDocProductsForSet(factory, docFavourite, known, e.set, nil, nil)
}

func (e *PropositionalEngine) planUDocProductsForSet(factory *rel.UDocRelationFactory, docFavourite bool, known map[ast.ASTHandle][]ast.ASTHandle, source *base.RuleSet, extraDocs, extraProducts map[ast.ASTHandle][]ast.ASTHandle) (program *udocProductProgram, resultErr error) {
	if factory.FormulaChecked() {
		return nil, nil
	}
	if source == nil {
		return nil, ErrUDocDatalogFragment
	}
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	var rules []*base.Rule
	defer func() {
		var cleanup error
		for _, rule := range rules {
			cleanup = errors.Join(cleanup, rule.Close())
		}
		if errors.Is(resultErr, ErrUDocDatalogFragment) && cleanup == nil {
			program, resultErr = nil, nil
		} else {
			resultErr = errors.Join(resultErr, cleanup)
		}
	}()
	definitions := make(map[ast.ASTHandle]int)
	groundRules := make(map[int]*udocProductGroundRule)
	groundHeads := make(map[ast.ASTHandle]bool)
	forwardHeads := make(map[ast.ASTHandle]bool)
	pairHeads := make(map[ast.ASTHandle]bool)
	multiHeads := make(map[ast.ASTHandle]bool)
	for i := 0; i < count; i++ {
		rule, err := source.Rule(i)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
		fact, err := udocProductGroundFact(e.manager, rule)
		if err != nil {
			return nil, err
		}
		if fact != nil {
			groundRules[i], groundHeads[fact.head] = fact, true
			continue
		}
		head, err := rule.HeadDeclaration()
		if err != nil {
			return nil, err
		}
		definitions[head]++
	}
	p := &udocProductProgram{rules: make(map[ast.ASTHandle]*udocProductRulePlan), sorts: make(map[ast.ASTHandle][]ast.ASTHandle), finiteSorts: make(map[ast.ASTHandle][]ast.ASTHandle), docs: make(map[ast.ASTHandle]bool), finite: make(map[ast.ASTHandle]bool), groundProducts: make(map[ast.ASTHandle]int), productSeeds: make(map[ast.ASTHandle]bool), docProductTails: make(map[ast.ASTHandle]int)}
	for declaration := range extraProducts {
		p.productSeeds[declaration] = true
	}
	boolean, err := e.manager.MkBoolSort()
	if err != nil {
		return nil, err
	}
	for ruleIndex := range rules {
		fact := groundRules[ruleIndex]
		if fact == nil {
			continue
		}
		if extraProducts[fact.head] != nil {
			p.productSeeds[fact.head] = true
			continue
		}
		if _, found := p.groundProducts[fact.head]; found {
			continue
		}
		mixed, err := e.planUDocMixedGround(p, fact.head, boolean, docFavourite, known)
		if err != nil {
			return nil, err
		}
		if mixed && definitions[fact.head] != 0 {
			return nil, ErrUDocDatalogFragment
		}
	}
	isDoc := func(d ast.ASTHandle) (bool, error) {
		if sorts, found := extraDocs[d]; found {
			return factory.CanHandleSignature(sorts)
		}
		state := e.predicates[d]
		if state == nil || state.external != nil || state.explicitFiniteRepresentation || len(state.sorts) != 1 || !e.relations.CanHandleTableSorts(state.sorts) {
			return false, nil
		}
		selected := len(state.requestedRepresentations) == 1 && state.requestedRepresentations[0] == "doc" || docFavourite && len(state.requestedRepresentations) == 0
		if !selected {
			return false, nil
		}
		return factory.CanHandleSignature(state.sorts)
	}
	for ruleIndex, rule := range rules {
		if groundRules[ruleIndex] != nil {
			continue
		}
		head, err := rule.HeadDeclaration()
		if err != nil {
			return nil, err
		}
		if e.auxiliary[head] {
			continue
		}
		state := e.predicates[head]
		if definitions[head] != 1 || groundHeads[head] || state == nil || state.external != nil || len(state.requestedRepresentations) != 0 || !e.relations.CanHandleTableSorts(state.sorts) {
			return nil, ErrUDocDatalogFragment
		}
		hasBool, hasOther := false, false
		for _, sort := range state.sorts {
			hasBool = hasBool || sort == boolean
			hasOther = hasOther || sort != boolean
		}
		if !hasBool || !hasOther {
			return nil, ErrUDocDatalogFragment
		}
		if known[head] == nil && state.table != nil {
			rows, err := state.table.Len()
			if err != nil {
				return nil, err
			}
			if rows != 0 {
				return nil, ErrUDocDatalogFragment
			}
		}
		positive, err := rule.PositiveTailSize()
		if err != nil {
			return nil, err
		}
		total, err := rule.TailSize()
		if err != nil {
			return nil, err
		}
		membershipIndex, membershipTail, err := similarityMembershipTail(e.manager, rule, positive, extraDocs, extraProducts)
		if err != nil {
			return nil, err
		}
		basePositive, baseTotal := positive, total
		if membershipIndex >= 0 {
			basePositive--
			baseTotal--
		}
		if basePositive == 0 && baseTotal == 0 && membershipIndex >= 0 {
			auxiliary, err := e.manager.GetDecl(membershipTail)
			if err != nil {
				return nil, err
			}
			_, productAuxiliary := extraProducts[auxiliary]
			if productAuxiliary {
				forwardHeads[head] = true
				continue
			}
		}
		if basePositive == 1 && baseTotal == 1 {
			// Source acyclic strata forward an existing product through
			// filtering and head assembly. Keep ordinary targets mixed so
			// their later unions still have two nontransparent components.
			for _, sort := range state.sorts {
				if sort == boolean {
					continue
				}
				bitVector, err := e.manager.IsBitVectorSort(sort)
				if err != nil {
					return nil, err
				}
				if !bitVector {
					return nil, ErrUDocDatalogFragment
				}
			}
			forwardHeads[head] = true
			continue
		}
		if basePositive > 2 {
			if baseTotal != basePositive {
				return nil, ErrUDocDatalogFragment
			}
			for sourceTail := 0; sourceTail < positive; sourceTail++ {
				if sourceTail == membershipIndex {
					continue
				}
				tail, _, cause := rule.Tail(sourceTail)
				if cause != nil {
					return nil, cause
				}
				declaration, cause := e.manager.GetDecl(tail)
				if cause != nil {
					return nil, cause
				}
				if p.productHead(declaration) || definitions[declaration] != 0 {
					continue
				}
				doc, cause := isDoc(declaration)
				if cause != nil {
					return nil, cause
				}
				if doc {
					p.docs[declaration] = true
					continue
				}
				input := e.predicates[declaration]
				if input == nil || input.external != nil || !e.relations.CanHandleTableSorts(input.sorts) {
					return nil, ErrUDocDatalogFragment
				}
				if input.table == nil && known[declaration] == nil || input.table != nil && udocProductFiniteKind(input.table) != e.datalogDefaultTable {
					return nil, ErrUDocDatalogFragment
				}
				if len(input.requestedRepresentations) != 0 && !input.explicitFiniteRepresentation {
					return nil, ErrUDocDatalogFragment
				}
				p.finite[declaration] = true
				p.finiteSorts[declaration] = append([]ast.ASTHandle(nil), input.sorts...)
			}
			for _, sort := range state.sorts {
				if sort == boolean {
					continue
				}
				bitVector, err := e.manager.IsBitVectorSort(sort)
				if err != nil {
					return nil, err
				}
				if !bitVector {
					return nil, ErrUDocDatalogFragment
				}
			}
			multiHeads[head] = true
			continue
		}
		if basePositive != 2 || baseTotal != 2 {
			return nil, ErrUDocDatalogFragment
		}
		tails, decls := make([]ast.ASTHandle, 2), make([]ast.ASTHandle, 2)
		for sourceTail := 0; sourceTail < positive; sourceTail++ {
			if sourceTail == membershipIndex {
				continue
			}
			target := sourceTail
			if membershipIndex >= 0 && sourceTail > membershipIndex {
				target--
			}
			if target < 0 || target >= len(tails) {
				continue
			}
			tails[target], _, err = rule.Tail(sourceTail)
			if err != nil {
				return nil, err
			}
			decls[target], err = e.manager.GetDecl(tails[target])
			if err != nil {
				return nil, err
			}
		}
		leftDoc, err := isDoc(decls[0])
		if err != nil {
			return nil, err
		}
		rightDoc, err := isDoc(decls[1])
		if err != nil {
			return nil, err
		}
		if leftDoc == rightDoc {
			if leftDoc {
				return nil, ErrUDocDatalogFragment
			}
			// Both producers may acquire product kinds later in dependency
			// order. This slice keeps their ordinary target mixed BV/Bool,
			// with the same concrete union boundary as product forwarding.
			for _, sort := range state.sorts {
				if sort == boolean {
					continue
				}
				bitVector, err := e.manager.IsBitVectorSort(sort)
				if err != nil {
					return nil, err
				}
				if !bitVector {
					return nil, ErrUDocDatalogFragment
				}
			}
			pairHeads[head] = true
			continue
		}
		di, ti := 0, 1
		if rightDoc {
			di, ti = 1, 0
		}
		doc, finite := decls[di], decls[ti]
		productInput := e.predicates[finite]
		productCandidate := definitions[finite] != 0 || p.productHead(finite)
		if productCandidate && productInput != nil && productInput.external == nil && !productInput.explicitFiniteRepresentation && len(productInput.requestedRepresentations) == 0 && e.relations.CanHandleTableSorts(productInput.sorts) {
			hasBoolean, hasOtherSort := false, false
			for _, sort := range productInput.sorts {
				hasBoolean = hasBoolean || sort == boolean
				hasOtherSort = hasOtherSort || sort != boolean
			}
			if hasBoolean && hasOtherSort {
				for _, sort := range state.sorts {
					if sort == boolean {
						continue
					}
					bitVector, err := e.manager.IsBitVectorSort(sort)
					if err != nil {
						return nil, err
					}
					if !bitVector {
						return nil, ErrUDocDatalogFragment
					}
				}
				p.docs[doc] = true
				p.docProductTails[head] = di
				pairHeads[head] = true
				continue
			}
		}
		input := e.predicates[finite]
		if definitions[doc] != 0 || definitions[finite] != 0 || input == nil || input.external != nil || !e.relations.CanHandleTableSorts(input.sorts) {
			return nil, ErrUDocDatalogFragment
		}
		if input.table == nil && known[finite] == nil || input.table != nil && udocProductFiniteKind(input.table) != e.datalogDefaultTable {
			return nil, ErrUDocDatalogFragment
		}
		if len(input.requestedRepresentations) != 0 && !input.explicitFiniteRepresentation {
			return nil, ErrUDocDatalogFragment
		}
		hasBool, hasOther = false, false
		for _, sort := range input.sorts {
			hasBool = hasBool || sort == boolean
			hasOther = hasOther || sort != boolean
		}
		if !hasBool {
			return nil, ErrUDocDatalogFragment
		}
		mixedGround := groundHeads[finite] && hasOther
		if mixedGround {
			if _, admitted := p.groundProducts[finite]; !admitted {
				return nil, ErrUDocDatalogFragment
			}
		}
		da, err := productArguments(e.manager, tails[di])
		if err != nil {
			return nil, err
		}
		fa, err := productArguments(e.manager, tails[ti])
		if err != nil {
			return nil, err
		}
		plan := &udocProductRulePlan{head: head, doc: doc, finite: finite, docFirst: di == 0}
		for _, args := range [][]ast.ASTHandle{da, fa} {
			seen := make(map[ast.ASTHandle]bool)
			for _, arg := range args {
				kind, err := e.manager.GetKind(arg)
				if err != nil {
					return nil, err
				}
				if kind != ast.ASTVar || seen[arg] {
					return nil, ErrUDocDatalogFragment
				}
				seen[arg] = true
			}
		}
		for i, darg := range da {
			for j, farg := range fa {
				if darg == farg {
					plan.docColumns = append(plan.docColumns, uint32(i))
					plan.finiteColumns = append(plan.finiteColumns, uint32(j))
				}
			}
		}
		arguments := append(append([]ast.ASTHandle(nil), da...), fa...)
		if di == 1 {
			arguments = append(append([]ast.ASTHandle(nil), fa...), da...)
		}
		h, err := rule.Head()
		if err != nil {
			return nil, err
		}
		if membershipIndex >= 0 {
			declaration, err := e.manager.GetDecl(membershipTail)
			if err != nil {
				return nil, err
			}
			_, productMembership := extraProducts[declaration]
			membership, auxiliaryArguments, err := productDocMembership(e.manager, arguments, membershipTail, declaration, productMembership)
			if err != nil {
				return nil, err
			}
			plan.docMemberships = append(plan.docMemberships, membership)
			arguments = append(arguments, auxiliaryArguments...)
		}
		if err = planUDocProductAssembly(e.manager, plan, arguments, h, false); err != nil {
			return nil, err
		}
		p.rules[head], p.sorts[head], p.docs[doc] = plan, append([]ast.ASTHandle(nil), state.sorts...), true
		if !mixedGround {
			p.finite[finite] = true
		}
		p.finiteSorts[finite] = append([]ast.ASTHandle(nil), input.sorts...)
		p.order = append(p.order, head)
	}
	if len(p.order) == 0 && len(p.groundProducts) == 0 {
		return nil, nil
	}
	// Only all-Bool finite tuples reduce to ordinary table unions; admitted
	// mixed tuples must execute their source singleton product assembly.
	for ruleIndex := range rules {
		fact := groundRules[ruleIndex]
		if fact == nil {
			continue
		}
		if extraProducts[fact.head] != nil {
			continue
		}
		if sorts := extraDocs[fact.head]; sorts != nil {
			if supported, cause := factory.CanHandleSignature(sorts); cause != nil || !supported {
				return nil, errors.Join(cause, ErrUDocDatalogFragment)
			}
			p.docs[fact.head] = true
		}
		_, mixed := p.groundProducts[fact.head]
		if !p.docs[fact.head] && !mixed {
			if !p.finite[fact.head] {
				return nil, ErrUDocDatalogFragment
			}
			for _, sort := range p.finiteSorts[fact.head] {
				if sort != boolean {
					return nil, ErrUDocDatalogFragment
				}
			}
		}
		p.groundFacts = append(p.groundFacts, fact)
	}
	for pending := true; pending; {
		pending = false
		advanced := false
		for ruleIndex, rule := range rules {
			if groundRules[ruleIndex] != nil {
				continue
			}
			head, err := rule.HeadDeclaration()
			if err != nil {
				return nil, err
			}
			if p.rules[head] != nil {
				continue
			}
			pending = true
			positive, err := rule.PositiveTailSize()
			if err != nil {
				return nil, err
			}
			total, err := rule.TailSize()
			if err != nil {
				return nil, err
			}
			if pairHeads[head] {
				membershipIndex, membershipTail, err := similarityMembershipTail(e.manager, rule, positive, extraDocs, extraProducts)
				if err != nil {
					return nil, err
				}
				if definitions[head] != 1 || membershipIndex < 0 && (positive != 2 || total != 2) || membershipIndex >= 0 && (positive != 3 || total != 3 || membershipIndex != 2) {
					return nil, ErrUDocDatalogFragment
				}
				docTail := -1
				if value, found := p.docProductTails[head]; found {
					docTail = value
				}
				plan, err := e.planUDocProductPair(p, rule, docTail)
				if err != nil {
					return nil, err
				}
				if plan == nil {
					continue
				}
				if membershipIndex >= 0 {
					var arguments []ast.ASTHandle
					for tailIndex := 0; tailIndex < 2; tailIndex++ {
						tail, _, cause := rule.Tail(tailIndex)
						if cause != nil {
							return nil, cause
						}
						tailArguments, cause := productArguments(e.manager, tail)
						if cause != nil {
							return nil, cause
						}
						arguments = append(arguments, tailArguments...)
					}
					declaration, cause := e.manager.GetDecl(membershipTail)
					if cause != nil {
						return nil, cause
					}
					_, productMembership := extraProducts[declaration]
					membership, auxiliaryArguments, cause := productDocMembership(e.manager, arguments, membershipTail, declaration, productMembership)
					if cause != nil {
						return nil, cause
					}
					plan.docMemberships = append(plan.docMemberships, membership)
					arguments = append(arguments, auxiliaryArguments...)
					headApplication, cause := rule.Head()
					if cause != nil {
						return nil, cause
					}
					plan.filters, plan.identical, plan.removed, plan.cycles, plan.output = nil, nil, nil, nil, nil
					if cause = planUDocProductAssembly(e.manager, plan, arguments, headApplication, true); cause != nil {
						return nil, cause
					}
					plan.identical = nil
				}
				p.rules[head], p.sorts[head] = plan, append([]ast.ASTHandle(nil), e.predicates[head].sorts...)
				p.order = append(p.order, head)
				advanced = true
				continue
			}
			if multiHeads[head] {
				membershipIndex, membershipTail, err := similarityMembershipTail(e.manager, rule, positive, extraDocs, extraProducts)
				if err != nil {
					return nil, err
				}
				if definitions[head] != 1 || total != positive || positive < 3 {
					return nil, ErrUDocDatalogFragment
				}
				if membershipIndex >= 0 && membershipIndex != positive-1 {
					return nil, ErrUDocDatalogFragment
				}
				inputCount := positive
				if membershipIndex >= 0 {
					inputCount--
				}
				plan := &udocProductRulePlan{head: head}
				var arguments []ast.ASTHandle
				priorVariables := make(map[ast.ASTHandle]uint32)
				ready := true
				for tailIndex := 0; tailIndex < inputCount; tailIndex++ {
					tail, negated, cause := rule.Tail(tailIndex)
					if cause != nil {
						return nil, cause
					}
					if negated {
						return nil, ErrUDocDatalogFragment
					}
					declaration, cause := e.manager.GetDecl(tail)
					if cause != nil {
						return nil, cause
					}
					if tailIndex == 0 {
						plan.multiStart = declaration
					}
					kind := uint8(0)
					if !p.productHead(declaration) {
						if p.docs[declaration] {
							kind = 1
						} else if p.finite[declaration] {
							kind = 2
						} else {
							ready = false
							break
						}
					}
					if tailIndex == 0 {
						plan.multiStartKind = kind
					}
					tailArguments, cause := productArguments(e.manager, tail)
					if cause != nil {
						return nil, cause
					}
					input := udocProductMultiInput{declaration: declaration, kind: kind, sorts: append([]ast.ASTHandle(nil), e.predicates[declaration].sorts...)}
					currentVariables := make(map[ast.ASTHandle]uint32)
					for column, argument := range tailArguments {
						kind, cause := e.manager.GetKind(argument)
						if cause != nil {
							return nil, cause
						}
						if kind != ast.ASTVar {
							continue
						}
						if priorColumn, found := priorVariables[argument]; found {
							input.leftColumns = append(input.leftColumns, priorColumn)
							input.rightColumns = append(input.rightColumns, uint32(column))
						}
						if _, found := currentVariables[argument]; !found {
							currentVariables[argument] = uint32(len(arguments) + column)
						}
					}
					if tailIndex > 0 {
						plan.multi = append(plan.multi, input)
					}
					for variable, column := range currentVariables {
						if _, found := priorVariables[variable]; !found {
							priorVariables[variable] = column
						}
					}
					arguments = append(arguments, tailArguments...)
				}
				if !ready || len(plan.multi) == 0 {
					continue
				}
				canStart := plan.multiStartKind == 0 || plan.multi[0].kind == 0 || plan.multiStartKind == 1 && (plan.multi[0].kind == 1 || plan.multi[0].kind == 2) || plan.multiStartKind == 2 && (plan.multi[0].kind == 1 || plan.multi[0].kind == 2)
				if !canStart {
					continue
				}
				if membershipIndex >= 0 {
					declaration, cause := e.manager.GetDecl(membershipTail)
					if cause != nil {
						return nil, cause
					}
					_, productMembership := extraProducts[declaration]
					membership, auxiliaryArguments, cause := productDocMembership(e.manager, arguments, membershipTail, declaration, productMembership)
					if cause != nil {
						return nil, cause
					}
					plan.docMemberships = append(plan.docMemberships, membership)
					arguments = append(arguments, auxiliaryArguments...)
				}
				headApplication, cause := rule.Head()
				if cause != nil {
					return nil, cause
				}
				if cause = planUDocProductAssembly(e.manager, plan, arguments, headApplication, true); cause != nil {
					return nil, cause
				}
				p.rules[head], p.sorts[head] = plan, append([]ast.ASTHandle(nil), e.predicates[head].sorts...)
				p.order = append(p.order, head)
				advanced = true
				continue
			}
			if !e.auxiliary[head] && !forwardHeads[head] || definitions[head] != 1 {
				return nil, ErrUDocDatalogFragment
			}
			membershipIndex, membershipTail, err := similarityMembershipTail(e.manager, rule, positive, extraDocs, extraProducts)
			if err != nil {
				return nil, err
			}
			sourceTailIndex := 0
			if membershipIndex >= 0 {
				if positive != 2 || total != 2 {
					return nil, ErrUDocDatalogFragment
				}
				sourceTailIndex = 1 - membershipIndex
			} else if positive != 1 || total != 1 {
				return nil, ErrUDocDatalogFragment
			}
			tail, _, err := rule.Tail(sourceTailIndex)
			source, err := e.manager.GetDecl(tail)
			if err != nil {
				return nil, err
			}
			if forwardHeads[head] && !p.productHead(source) {
				// A later rule may establish the source's product kind. The
				// no-progress check below rejects recursive or unresolved
				// dependencies; generated DOC/finite auxiliaries stay separate.
				if p.rules[source] != nil || p.docs[source] || p.finite[source] {
					return nil, ErrUDocDatalogFragment
				}
				continue
			}
			if !p.productHead(source) && p.rules[source] == nil && !p.docs[source] && !p.finite[source] {
				continue
			}
			plan := &udocProductRulePlan{head: head, source: source, ruleIndex: ruleIndex}
			if p.docs[source] {
				plan.inputKind = 1
			}
			if p.finite[source] {
				plan.inputKind = 2
			}
			if prior := p.rules[source]; prior != nil {
				plan.inputKind = prior.inputKind
			}
			args, err := productArguments(e.manager, tail)
			if err != nil {
				return nil, err
			}
			if membershipIndex >= 0 {
				declaration, err := e.manager.GetDecl(membershipTail)
				if err != nil {
					return nil, err
				}
				_, productMembership := extraProducts[declaration]
				membership, auxiliaryArguments, err := productDocMembership(e.manager, args, membershipTail, declaration, productMembership)
				if err != nil {
					return nil, err
				}
				plan.docMemberships = append(plan.docMemberships, membership)
				args = append(args, auxiliaryArguments...)
			}
			h, err := rule.Head()
			if err != nil {
				return nil, err
			}
			if err = planUDocProductAssembly(e.manager, plan, args, h, true); err != nil {
				return nil, err
			}
			p.rules[head], p.sorts[head] = plan, append([]ast.ASTHandle{}, e.predicates[head].sorts...)
			p.order = append(p.order, head)
			advanced = true
		}
		if pending && !advanced {
			return nil, ErrUDocDatalogFragment
		}
	}
	for _, d := range e.predicateOrder {
		if !p.productHead(d) && p.rules[d] == nil && !p.docs[d] && !p.finite[d] {
			return nil, ErrUDocDatalogFragment
		}
	}
	return p, nil
}
