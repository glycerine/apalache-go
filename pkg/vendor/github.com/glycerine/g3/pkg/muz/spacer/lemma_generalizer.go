package spacer

// Source: src/muz/spacer/spacer_generalizers.cpp
// lemma_bool_inductive_generalizer and src/muz/spacer/spacer_context.cpp
// pred_transformer::check_inductive.

import (
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

var ErrLemmaSanity = errors.New("muz/spacer: generalized lemma is not inductive")

func (p *PredicateTransformer) checkInductiveCube(cube []ast.ASTHandle, level uint32) (core []ast.ASTRef, usesLevel uint32, inductive bool, resultErr error) {
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, releaseNormalizeRefs(p.manager, core))
			core = nil
		}
	}()
	conjunction, err := p.manager.MkAndArray(cube)
	if err != nil {
		return nil, 0, false, err
	}
	states, err := p.manager.PushNot(conjunction)
	if err != nil {
		return nil, 0, false, err
	}
	head, err := p.Head()
	if err != nil {
		return nil, 0, false, err
	}
	assumptionRefs, err := p.makeAssumptions(head, states)
	if err != nil {
		return nil, 0, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, releaseNormalizeRefs(p.manager, assumptionRefs)) }()
	background := make([]ast.ASTHandle, 0, len(assumptionRefs)+1)
	for i := range assumptionRefs {
		handle, getErr := p.manager.RefGet(assumptionRefs[i])
		if getErr != nil {
			return nil, 0, false, getErr
		}
		background = append(background, handle)
	}
	extension, err := p.manager.RefGet(p.extendLiteral)
	if err != nil {
		return nil, 0, false, err
	}
	background = append(background, extension)
	check, err := p.solver.CheckAssumptions(cube, nil, PropCheckOptions{
		Background: background, NeedCore: true, SubsetBasedCore: true, Level: &level,
	})
	if err != nil {
		return nil, 0, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, check.Close(p.manager)) }()
	if check.Status != lbool.False {
		return nil, check.UsesLevel, false, nil
	}
	// Move the owning core out before closing the solver check. Its formulas
	// may be needed across subsequent checks and result-lemma construction.
	core = check.Core
	check.Core = nil
	return core, check.UsesLevel, true, nil
}

func containsGeneralizedLiteral(values []ast.ASTHandle, value ast.ASTHandle) bool {
	for _, prior := range values {
		if prior == value {
			return true
		}
	}
	return false
}

// expandInductiveLiteral follows expand_literals' growing conjunction vector.
// Constructor field equalities appended during expansion are visited too.
func (p *PredicateTransformer) expandInductiveLiteral(literal ast.ASTHandle) (result *literalExpansion, resultErr error) {
	owner := &literalExpansion{manager: p.manager}
	defer func() {
		if resultErr != nil {
			resultErr = finishLiteralExpansionFailure(owner, resultErr)
		}
	}()
	if _, err := owner.keep(literal, nil); err != nil {
		return nil, err
	}
	conjs := []ast.ASTHandle{literal}
	if err := p.traceLiteralExpansion(conjs, true); err != nil {
		return nil, err
	}
	for i := 0; i < len(conjs); i++ {
		expanded, err := p.expandInductiveLiteralOnce(owner, conjs[i])
		if err != nil {
			return nil, err
		}
		if len(expanded) == 1 && expanded[0] == conjs[i] {
			continue
		}
		conjs[i] = expanded[0]
		family, err := p.manager.GetFamilyID(expanded[0])
		if err != nil {
			return nil, err
		}
		arithmetic, err := p.manager.ArithmeticFamilyID()
		if err != nil {
			return nil, err
		}
		if family == arithmetic && len(expanded) == 2 {
			// Source inserts the second arithmetic bound next to the first,
			// moving the displaced next conjunct to the end of the vector.
			if i+1 == len(conjs) {
				conjs = append(conjs, expanded[1])
			} else {
				conjs = append(conjs, conjs[i+1])
				conjs[i+1] = expanded[1]
			}
			i++
		} else {
			conjs = append(conjs, expanded[1:]...)
		}
	}
	if err := owner.finish(conjs); err != nil {
		return nil, err
	}
	if err := p.traceLiteralExpansion(conjs, false); err != nil {
		return nil, err
	}
	return owner, nil
}

func (p *PredicateTransformer) expandInductiveLiteralOnce(owner *literalExpansion, literal ast.ASTHandle) ([]ast.ASTHandle, error) {
	family, err := p.manager.GetFamilyID(literal)
	if err != nil || family != p.manager.BasicFamilyID() {
		return []ast.ASTHandle{literal}, err
	}
	kind, err := p.manager.GetDeclKind(literal)
	if err != nil || kind != ast.ASTOpEq {
		return []ast.ASTHandle{literal}, err
	}
	left, err := p.manager.GetArg(literal, 0)
	if err != nil {
		return nil, err
	}
	right, err := p.manager.GetArg(literal, 1)
	if err != nil {
		return nil, err
	}
	sort, err := p.manager.GetSort(left)
	if err != nil {
		return nil, err
	}
	sortFamily, err := p.manager.GetFamilyID(sort)
	if err != nil {
		return nil, err
	}
	arithmetic, err := p.manager.ArithmeticFamilyID()
	if err != nil {
		return nil, err
	}
	if sortFamily == arithmetic {
		le, err := owner.keep(p.manager.MkArithmeticLE(left, right))
		if err != nil {
			return nil, err
		}
		ge, err := owner.keep(p.manager.MkArithmeticGE(left, right))
		if err != nil {
			return nil, err
		}
		return []ast.ASTHandle{le, ge}, nil
	}
	if expanded, matched, err := p.expandInductiveDatatypeEquality(owner, left, right); err != nil || matched {
		return expanded, err
	}
	if expanded, matched, err := p.expandInductiveDatatypeEquality(owner, right, left); err != nil || matched {
		return expanded, err
	}
	if expanded, matched, err := p.expandInductiveBitVectorEquality(owner, left, right); err != nil || matched {
		return expanded, err
	}
	if expanded, matched, err := p.expandInductiveBitVectorEquality(owner, right, left); err != nil || matched {
		return expanded, err
	}
	return []ast.ASTHandle{literal}, nil
}

// expandInductiveDatatypeEquality treats value as the constructor side and
// term as the side replaced by a recognizer and accessor equalities.
func (p *PredicateTransformer) expandInductiveDatatypeEquality(owner *literalExpansion, term, value ast.ASTHandle) ([]ast.ASTHandle, bool, error) {
	kind, err := p.manager.GetKind(value)
	if err != nil || kind != ast.ASTApp {
		return nil, false, err
	}
	constructor, err := p.manager.GetDecl(value)
	if err != nil {
		return nil, false, err
	}
	family, err := p.manager.GetFamilyID(constructor)
	if err != nil {
		return nil, false, err
	}
	datatype, err := p.manager.RegisterDatatypePlugin()
	if err != nil || family != datatype {
		return nil, false, err
	}
	declKind, err := p.manager.GetDeclKind(constructor)
	if err != nil || declKind != ast.ASTDatatypeConstructor {
		return nil, false, err
	}
	recognizer, err := p.manager.DatatypeConstructorRecognizer(constructor)
	if err != nil {
		return nil, false, err
	}
	isConstructor, err := owner.keep(p.manager.MkApp(recognizer, []ast.ASTHandle{term}))
	if err != nil {
		return nil, false, err
	}
	result := []ast.ASTHandle{isConstructor}
	accessors, err := p.manager.DatatypeConstructorAccessors(constructor)
	if err != nil {
		return nil, false, err
	}
	termConstructor := false
	if termKind, kindErr := p.manager.GetKind(term); kindErr != nil {
		return nil, false, kindErr
	} else if termKind == ast.ASTApp {
		termDecl, declErr := p.manager.GetDecl(term)
		if declErr != nil {
			return nil, false, declErr
		}
		termConstructor = termDecl == constructor
	}
	for index, accessor := range accessors {
		var selected ast.ASTHandle
		if termConstructor {
			selected, err = p.manager.GetArg(term, uint32(index))
		} else {
			selected, err = owner.keep(p.manager.MkApp(accessor, []ast.ASTHandle{term}))
		}
		if err != nil {
			return nil, false, err
		}
		field, err := p.manager.GetArg(value, uint32(index))
		if err != nil {
			return nil, false, err
		}
		equality, err := owner.keep(p.manager.MkEq(selected, field))
		if err != nil {
			return nil, false, err
		}
		result = append(result, equality)
	}
	return result, true, nil
}

// expandInductiveBitVectorEquality treats value as the numeral side.
func (p *PredicateTransformer) expandInductiveBitVectorEquality(owner *literalExpansion, term, value ast.ASTHandle) ([]ast.ASTHandle, bool, error) {
	text, matched, err := p.manager.BitVectorNumeralText(value)
	if err != nil {
		return nil, false, fmt.Errorf("read bit-vector numeral text: %w", err)
	}
	if !matched {
		return nil, false, nil
	}
	width, err := p.manager.BitVectorSize(value)
	if err != nil {
		return nil, false, fmt.Errorf("read bit-vector numeral width: %w", err)
	}
	if width == 0 {
		return nil, false, ast.ErrASTDomain
	}
	integer := new(big.Int)
	if _, ok := integer.SetString(text, 10); !ok {
		return nil, false, ast.ErrASTDomain
	}
	one, err := owner.keep(p.manager.BitVectorNumeralUnsigned(1, 1))
	if err != nil {
		return nil, false, fmt.Errorf("construct bit-vector one: %w", err)
	}
	result := make([]ast.ASTHandle, 0, width)
	for bit := uint32(0); bit < width; bit++ {
		extract, err := owner.keep(p.manager.MkBitVectorExtract(bit, bit, term))
		if err != nil {
			return nil, false, err
		}
		literal, err := owner.keep(p.manager.MkEq(one, extract))
		if err != nil {
			return nil, false, err
		}
		if integer.Bit(int(bit)) == 0 {
			literal, err = owner.keep(p.manager.MkNot(literal))
			if err != nil {
				return nil, false, err
			}
		}
		result = append(result, literal)
	}
	return result, true, nil
}

func (p *PredicateTransformer) generalizeInductiveLemma(obligation *ProofObligation, lemma *Lemma) (result *Lemma, resultErr error) {
	cube, err := lemma.Cube()
	if err != nil || len(cube) == 0 {
		return lemma, err
	}
	// Source cube and extra_lits are owning vectors. Keep all solver cores
	// and speculative expansion candidates until the final lemma owns its cube.
	expansions := &literalExpansionListOwner{manager: p.manager, runtime: p.manager.NumeralService().RuntimeOwner()}
	defer func() { result, resultErr = finishInductiveGeneralization(expansions, lemma, result, resultErr) }()
	checkCube := func(cube []ast.ASTHandle, level uint32) ([]ast.ASTHandle, uint32, bool, error) {
		refs, used, inductive, err := p.checkInductiveCube(cube, level)
		expansions.adoptRefs(refs)
		if err != nil {
			return nil, used, inductive, err
		}
		handles := make([]ast.ASTHandle, len(refs))
		for i, ref := range refs {
			handles[i], err = p.manager.RefGet(ref)
			if err != nil {
				return nil, used, false, err
			}
		}
		return handles, used, inductive, nil
	}
	p.generalizerStats.count++
	started := time.Now()
	defer func() { p.generalizerStats.elapsed += time.Since(started) }()
	level, err := lemma.Level()
	if err != nil {
		return lemma, err
	}
	truth, err := p.manager.MkTrue()
	if err != nil {
		return lemma, err
	}
	processed := make([]ast.ASTHandle, 0, len(cube))
	dirty := false
	usesLevel := level
	for index := 0; index < len(cube); {
		literal := cube[index]
		cube[index] = truth
		var core []ast.ASTHandle
		var used uint32
		var inductive bool
		if len(cube) > 1 {
			core, used, inductive, err = checkCube(cube, level)
			if err != nil {
				return lemma, err
			}
			if inductive {
				cube = core
				usesLevel = used
				dirty = true
				index = 0
				for index < len(cube) && containsGeneralizedLiteral(processed, cube[index]) {
					index++
				}
				continue
			}
		}
		expansion, expandErr := p.expandInductiveLiteral(literal)
		if expandErr != nil {
			return lemma, expandErr
		}
		expansions.adopt(expansion)
		expanded := expansion.literals
		found := false
		if len(expanded) > 1 || expanded[0] != literal {
			for _, replacement := range expanded {
				cube[index] = replacement
				core, used, inductive, err = checkCube(cube, level)
				if err != nil {
					return lemma, err
				}
				if inductive {
					cube = core
					usesLevel = used
					dirty = true
					found = true
					processed = append(processed, replacement)
					index = 0
					for index < len(cube) && containsGeneralizedLiteral(processed, cube[index]) {
						index++
					}
					break
				}
			}
		}
		if found {
			continue
		}
		cube[index] = literal
		processed = append(processed, literal)
		p.generalizerStats.failures++
		index++
	}
	if !dirty {
		return lemma, nil
	}
	return NewLemmaFromProofObligation(obligation, cube, usesLevel)
}

func (p *PredicateTransformer) checkLemmaSanity(lemma *Lemma) (resultErr error) {
	cube, err := lemma.Cube()
	if err != nil {
		return err
	}
	level, err := lemma.Level()
	if err != nil {
		return err
	}
	core, _, inductive, err := p.checkInductiveCube(cube, level)
	defer func() { resultErr = errors.Join(resultErr, releaseNormalizeRefs(p.manager, core)) }()
	if err != nil {
		return err
	}
	if !inductive {
		return ErrLemmaSanity
	}
	return nil
}
