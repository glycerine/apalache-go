package rewriter

// Source: src/ast/rewriter/seq_rewriter.cpp:483-519.

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
)

// SequenceRewriter borrows its AST manager and implements the retained theory
// reductions independently of traversal.
type SequenceRewriter struct {
	manager *ast.ASTManager
	family  ast.FamilyID
	solver  SequenceExpressionSolver
}

func NewSequenceRewriter(m *ast.ASTManager) (*SequenceRewriter, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if _, err := m.ProofsEnabled(); err != nil {
		return nil, err
	}
	family, err := m.SequenceFamilyID()
	if err != nil {
		return nil, err
	}
	return &SequenceRewriter{manager: m, family: family}, nil
}

func (r *SequenceRewriter) RewriteApp(decl ast.ASTHandle, args ast.AppArguments, out ast.ASTRef) (status RewriteStatus, err error) {
	if err = r.open(); err != nil {
		return RewriteFailed, err
	}
	family, err := r.manager.GetFamilyID(decl)
	if err != nil {
		return RewriteFailed, err
	}
	if family != r.family {
		return RewriteFailed, nil
	}
	kind, err := r.manager.GetDeclKind(decl)
	if err != nil {
		return RewriteFailed, err
	}
	values := make([]ast.ASTHandle, args.Count())
	for i := uint32(0); i < args.Count(); i++ {
		values[i], err = args.At(i)
		if err != nil {
			return RewriteFailed, err
		}
	}
	switch kind {
	case 0:
		if len(values) == 1 {
			return r.RewriteUnit(values[0], out)
		}
	case 2:
		if len(values) == 1 {
			return RewriteDone, r.manager.SetRef(out, values[0])
		}
		if len(values) == 2 {
			return r.RewriteConcat(values[0], values[1], out)
		}
	case 3:
		if len(values) == 2 {
			return r.RewritePrefix(values[0], values[1], out)
		}
	case 4:
		if len(values) == 2 {
			return r.RewriteSuffix(values[0], values[1], out)
		}
	case 5:
		if len(values) == 2 {
			return r.RewriteContains(values[0], values[1], out)
		}
	case 6:
		if len(values) == 3 {
			return r.RewriteExtract(values[0], values[1], values[2], out)
		}
	case 7:
		if len(values) == 3 {
			return r.RewriteReplace(values[0], values[1], values[2], out)
		}
	case 8:
		if len(values) == 2 {
			return r.RewriteAt(values[0], values[1], out)
		}
	case 9:
		if len(values) == 1 {
			return r.RewriteLength(values[0], out)
		}
	case 10:
		if len(values) == 3 {
			return r.RewriteIndex(values[0], values[1], values[2], out)
		}
		if len(values) == 2 {
			zero, e := r.makeInteger(big.NewInt(0))
			if e != nil {
				return RewriteFailed, e
			}
			zeroRef, e := r.manager.NewRef(zero)
			if e != nil {
				return RewriteFailed, e
			}
			defer func() { err = errors.Join(err, r.manager.DelRef(&zeroRef)) }()
			result, e := r.manager.MkSequenceIndex(values[0], values[1], zero)
			if e != nil {
				return RewriteFailed, e
			}
			if err = r.manager.SetRef(out, result); err != nil {
				return RewriteFailed, err
			}
			return Rewrite1, nil
		}
	case 12:
		if len(values) == 2 {
			return r.RewriteStringInRegex(values[0], values[1], out)
		}
	case 13:
		if len(values) == 1 {
			return r.RewriteRegexPlus(values[0], out)
		}
	case 14:
		if len(values) == 1 {
			return r.RewriteRegexStar(values[0], out)
		}
	case 15:
		if len(values) == 1 {
			return r.RewriteRegexOptional(values[0], out)
		}
	case 17:
		if len(values) == 2 {
			return r.RewriteRegexConcat(values[0], values[1], out)
		}
	case 18:
		if len(values) == 2 {
			return r.RewriteRegexUnion(values[0], values[1], out)
		}
	case 19:
		if len(values) == 2 {
			return r.RewriteRegexIntersect(values[0], values[1], out)
		}
	case 20:
		return r.RewriteRegexLoop(values, out)
	case 21:
		if len(values) == 1 {
			return r.RewriteRegexComplement(values[0], out)
		}
	case 27:
		if len(values) == 1 {
			return r.RewriteIntToString(values[0], out)
		}
	case 28:
		if len(values) == 1 {
			return r.RewriteStringToInt(values[0], out)
		}
	}
	return RewriteFailed, nil
}

func (r *SequenceRewriter) open() error {
	if r == nil || r.manager == nil {
		return ast.ErrASTHandle
	}
	_, err := r.manager.ProofsEnabled()
	return err
}

func (r *SequenceRewriter) assignBoolean(value bool, out ast.ASTRef) (RewriteStatus, error) {
	var result ast.ASTHandle
	var err error
	if value {
		result, err = r.manager.MkTrue()
	} else {
		result, err = r.manager.MkFalse()
	}
	if err != nil {
		return RewriteFailed, err
	}
	return RewriteDone, r.manager.SetRef(out, result)
}

func (r *SequenceRewriter) integerValue(h ast.ASTHandle) (*big.Int, bool, error) {
	text, integer, matched, err := r.manager.ArithmeticNumeralText(h)
	if err != nil || !matched || !integer {
		return nil, false, err
	}
	value, ok := new(big.Int).SetString(text, 10)
	if !ok {
		return nil, false, ast.ErrASTDomain
	}
	return value, true, nil
}

func (r *SequenceRewriter) rationalValue(h ast.ASTHandle) (*big.Rat, bool, error) {
	text, _, matched, err := r.manager.ArithmeticNumeralText(h)
	if err != nil || !matched {
		return nil, false, err
	}
	value, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, false, ast.ErrASTDomain
	}
	return value, true, nil
}

func (r *SequenceRewriter) assignInteger(value *big.Int, out ast.ASTRef) (status RewriteStatus, err error) {
	result, err := r.makeInteger(value)
	if err != nil {
		return RewriteFailed, err
	}
	if err = r.manager.SetRef(out, result); err != nil {
		return RewriteFailed, err
	}
	return RewriteDone, nil
}

func (r *SequenceRewriter) makeInteger(value *big.Int) (result ast.ASTHandle, err error) {
	service := r.manager.NumeralService()
	if service == nil || value == nil {
		return result, ast.ErrASTHandle
	}
	numeral, err := service.NewValue()
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, service.Del(&numeral)) }()
	if err = service.SetString(&numeral, value.String()); err != nil {
		return result, err
	}
	return r.manager.ArithmeticNumeral(&numeral, true)
}

func (r *SequenceRewriter) assignEmptyLike(h ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	sort, err := r.manager.GetSort(h)
	if err != nil {
		return RewriteFailed, err
	}
	empty, err := r.manager.MkSequenceEmpty(sort)
	if err != nil {
		return RewriteFailed, err
	}
	return RewriteDone, r.manager.SetRef(out, empty)
}

func (r *SequenceRewriter) RewriteLength(a ast.ASTHandle, out ast.ASTRef) (status RewriteStatus, err error) {
	if err := r.open(); err != nil {
		return RewriteFailed, err
	}
	leaves, err := r.manager.SequenceFlattenConcat(a)
	if err != nil {
		return RewriteFailed, err
	}
	length := new(big.Int)
	unknown := make([]ast.ASTHandle, 0, len(leaves))
	for _, leaf := range leaves {
		text, stringValue, err := r.manager.SequenceStringValue(leaf)
		if err != nil {
			return RewriteFailed, err
		}
		if stringValue {
			length.Add(length, new(big.Int).SetUint64(uint64(len(text))))
			continue
		}
		unit, err := r.manager.IsSequenceUnit(leaf)
		if err != nil {
			return RewriteFailed, err
		}
		if !unit {
			unknown = append(unknown, leaf)
			continue
		}
		length.Add(length, big.NewInt(1))
	}
	if len(unknown) == 0 {
		return r.assignInteger(length, out)
	}
	if len(unknown) == 1 && length.Sign() == 0 && len(leaves) == 1 {
		return RewriteFailed, nil
	}
	terms := make([]ast.ASTHandle, 0, len(unknown)+1)
	refs := make([]ast.ASTRef, 0, len(unknown)+1)
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			err = errors.Join(err, r.manager.DelRef(&refs[i]))
		}
	}()
	for _, leaf := range unknown {
		term, e := r.manager.MkSequenceLength(leaf)
		if e != nil {
			return RewriteFailed, e
		}
		ref, e := r.manager.NewRef(term)
		if e != nil {
			return RewriteFailed, e
		}
		refs = append(refs, ref)
		terms = append(terms, term)
	}
	if length.Sign() != 0 {
		term, e := r.makeInteger(length)
		if e != nil {
			return RewriteFailed, e
		}
		ref, e := r.manager.NewRef(term)
		if e != nil {
			return RewriteFailed, e
		}
		refs = append(refs, ref)
		terms = append(terms, term)
	}
	family, err := r.manager.ArithmeticFamilyID()
	if err != nil {
		return RewriteFailed, err
	}
	result, err := r.manager.MkFamilyApp(family, 6, nil, terms)
	if err != nil {
		return RewriteFailed, err
	}
	if err = r.manager.SetRef(out, result); err != nil {
		return RewriteFailed, err
	}
	return Rewrite2, nil
}

func (r *SequenceRewriter) assignSequenceParts(parts []ast.ASTHandle, like ast.ASTHandle, out ast.ASTRef) (status RewriteStatus, err error) {
	if len(parts) == 0 {
		return r.assignEmptyLike(like, out)
	}
	if len(parts) == 1 {
		return RewriteDone, r.manager.SetRef(out, parts[0])
	}
	current := parts[len(parts)-1]
	refs := make([]ast.ASTRef, 0, len(parts)-1)
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			err = errors.Join(err, r.manager.DelRef(&refs[i]))
		}
	}()
	for i := len(parts) - 2; i >= 0; i-- {
		next, e := r.manager.MkSequenceConcat(parts[i], current)
		if e != nil {
			return RewriteFailed, e
		}
		ref, e := r.manager.NewRef(next)
		if e != nil {
			return RewriteFailed, e
		}
		refs = append(refs, ref)
		current = next
	}
	if err = r.manager.SetRef(out, current); err != nil {
		return RewriteFailed, err
	}
	return RewriteDone, nil
}

func (r *SequenceRewriter) makeSequencePartsRef(parts []ast.ASTHandle, like ast.ASTHandle) (ref ast.ASTRef, result ast.ASTHandle, err error) {
	ref, err = r.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ref, result, err
	}
	if _, err = r.assignSequenceParts(parts, like, ref); err != nil {
		err = errors.Join(err, r.manager.DelRef(&ref))
		return ref, result, err
	}
	result, err = r.manager.RefGet(ref)
	if err != nil {
		err = errors.Join(err, r.manager.DelRef(&ref))
	}
	return ref, result, err
}

func (r *SequenceRewriter) assignSequenceRelation(leftParts, rightParts []ast.ASTHandle, leftLike, rightLike ast.ASTHandle, prefix bool, resultStatus RewriteStatus, out ast.ASTRef) (status RewriteStatus, err error) {
	leftRef, left, err := r.makeSequencePartsRef(leftParts, leftLike)
	if err != nil {
		return RewriteFailed, err
	}
	defer func() { err = errors.Join(err, r.manager.DelRef(&leftRef)) }()
	rightRef, right, err := r.makeSequencePartsRef(rightParts, rightLike)
	if err != nil {
		return RewriteFailed, err
	}
	defer func() { err = errors.Join(err, r.manager.DelRef(&rightRef)) }()
	var result ast.ASTHandle
	if prefix {
		result, err = r.manager.MkSequencePrefix(left, right)
	} else {
		result, err = r.manager.MkSequenceSuffix(left, right)
	}
	if err != nil {
		return RewriteFailed, err
	}
	if err = r.manager.SetRef(out, result); err != nil {
		return RewriteFailed, err
	}
	return resultStatus, nil
}

func (r *SequenceRewriter) RewriteExtract(a, position, length ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if err := r.open(); err != nil {
		return RewriteFailed, err
	}
	pos, posNumeral, err := r.integerValue(position)
	if err != nil {
		return RewriteFailed, err
	}
	size, sizeNumeral, err := r.integerValue(length)
	if err != nil {
		return RewriteFailed, err
	}
	if posNumeral && pos.Sign() < 0 || sizeNumeral && size.Sign() <= 0 {
		return r.assignEmptyLike(a, out)
	}
	text, constant, err := r.manager.SequenceStringValue(a)
	if err != nil {
		return RewriteFailed, err
	}
	if constant && posNumeral && sizeNumeral {
		textLength := new(big.Int).SetUint64(uint64(len(text)))
		if pos.Cmp(textLength) >= 0 {
			return r.assignEmptyLike(a, out)
		}
		end := new(big.Int).Add(new(big.Int).Set(pos), size)
		if end.Cmp(textLength) > 0 {
			end.Set(textLength)
		}
		startIndex := int(pos.Uint64())
		endIndex := int(end.Uint64())
		result, err := r.manager.MkString(text[startIndex:endIndex])
		if err != nil {
			return RewriteFailed, err
		}
		return RewriteDone, r.manager.SetRef(out, result)
	}
	if !posNumeral || !sizeNumeral || !pos.IsUint64() || !size.IsUint64() {
		return RewriteFailed, nil
	}
	leaves, err := r.manager.SequenceFlattenConcat(a)
	if err != nil {
		return RewriteFailed, err
	}
	for _, leaf := range leaves {
		unit, e := r.manager.IsSequenceUnit(leaf)
		if e != nil {
			return RewriteFailed, e
		}
		if !unit {
			return RewriteFailed, nil
		}
	}
	start, count := pos.Uint64(), size.Uint64()
	if start >= uint64(len(leaves)) {
		return r.assignEmptyLike(a, out)
	}
	end := start + count
	if end < start || end > uint64(len(leaves)) {
		end = uint64(len(leaves))
	}
	return r.assignSequenceParts(leaves[start:end], a, out)
}

func (r *SequenceRewriter) RewriteAt(a, index ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if err := r.open(); err != nil {
		return RewriteFailed, err
	}
	position, numeral, err := r.integerValue(index)
	if err != nil || !numeral {
		return RewriteFailed, err
	}
	if position.Sign() < 0 {
		return r.assignEmptyLike(a, out)
	}
	leaves, err := r.manager.SequenceFlattenConcat(a)
	if err != nil {
		return RewriteFailed, err
	}
	knownLength := uint64(0)
	bounded := true
	for _, leaf := range leaves {
		text, stringValue, e := r.manager.SequenceStringValue(leaf)
		if e != nil {
			return RewriteFailed, e
		}
		if stringValue {
			knownLength += uint64(len(text))
			continue
		}
		unit, e := r.manager.IsSequenceUnit(leaf)
		if e != nil {
			return RewriteFailed, e
		}
		if unit {
			knownLength++
		} else {
			bounded = false
		}
	}
	if bounded && (!position.IsUint64() || position.Uint64() >= knownLength) {
		return r.assignEmptyLike(a, out)
	}
	text, constant, err := r.manager.SequenceStringValue(a)
	if err != nil {
		return RewriteFailed, err
	}
	if constant {
		if !position.IsUint64() || position.Uint64() >= uint64(len(text)) {
			return r.assignEmptyLike(a, out)
		}
		result, err := r.manager.MkString(text[position.Uint64() : position.Uint64()+1])
		if err != nil {
			return RewriteFailed, err
		}
		return RewriteDone, r.manager.SetRef(out, result)
	}
	indexValue := position.Uint64()
	for _, leaf := range leaves {
		unit, e := r.manager.IsSequenceUnit(leaf)
		if e != nil {
			return RewriteFailed, e
		}
		if !unit {
			return RewriteFailed, nil
		}
		if indexValue == 0 {
			return RewriteDone, r.manager.SetRef(out, leaf)
		}
		indexValue--
	}
	return RewriteFailed, nil
}

func (r *SequenceRewriter) RewriteIndex(a, needle, offset ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if err := r.open(); err != nil {
		return RewriteFailed, err
	}
	position, numeral, err := r.integerValue(offset)
	if err != nil || !numeral {
		return RewriteFailed, err
	}
	if position.Sign() < 0 {
		return r.assignInteger(big.NewInt(-1), out)
	}
	base, baseString, err := r.manager.SequenceStringValue(a)
	if err != nil {
		return RewriteFailed, err
	}
	pattern, patternString, err := r.manager.SequenceStringValue(needle)
	if err != nil {
		return RewriteFailed, err
	}
	if baseString && patternString {
		baseLength := new(big.Int).SetUint64(uint64(len(base)))
		if position.Cmp(baseLength) > 0 || position.Cmp(baseLength) == 0 && pattern != "" {
			return r.assignInteger(big.NewInt(-1), out)
		}
		start := position.Uint64()
		found := strings.Index(base[start:], pattern)
		if found < 0 {
			return r.assignInteger(big.NewInt(-1), out)
		}
		answer := new(big.Int).Add(position, new(big.Int).SetUint64(uint64(found)))
		return r.assignInteger(answer, out)
	}
	empty, err := r.manager.IsSequenceEmpty(needle)
	if err != nil {
		return RewriteFailed, err
	}
	if empty && position.Sign() == 0 {
		return RewriteDone, r.manager.SetRef(out, offset)
	}
	return RewriteFailed, nil
}

func (r *SequenceRewriter) RewriteIntToString(a ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if err := r.open(); err != nil {
		return RewriteFailed, err
	}
	value, matched, err := r.rationalValue(a)
	if err != nil || !matched {
		return RewriteFailed, err
	}
	text := ""
	if value.IsInt() && value.Sign() >= 0 {
		text = value.Num().String()
	}
	result, err := r.manager.MkString(text)
	if err != nil {
		return RewriteFailed, err
	}
	return RewriteDone, r.manager.SetRef(out, result)
}

func (r *SequenceRewriter) RewriteStringToInt(a ast.ASTHandle, out ast.ASTRef) (status RewriteStatus, err error) {
	if err = r.open(); err != nil {
		return RewriteFailed, err
	}
	text, stringValue, err := r.manager.SequenceStringValue(a)
	if err != nil {
		return RewriteFailed, err
	}
	if stringValue {
		valid := text != ""
		for i := 0; valid && i < len(text); i++ {
			valid = '0' <= text[i] && text[i] <= '9'
		}
		if !valid {
			return r.assignInteger(big.NewInt(-1), out)
		}
		value, ok := new(big.Int).SetString(text, 10)
		if !ok {
			return RewriteFailed, ast.ErrASTDomain
		}
		return r.assignInteger(value, out)
	}
	integer, matched, err := r.manager.SequenceIntToStringArgument(a)
	if err != nil || !matched {
		return RewriteFailed, err
	}
	zero, err := r.makeInteger(big.NewInt(0))
	if err != nil {
		return RewriteFailed, err
	}
	refs := make([]ast.ASTRef, 0, 3)
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			err = errors.Join(err, r.manager.DelRef(&refs[i]))
		}
	}()
	zeroRef, err := r.manager.NewRef(zero)
	if err != nil {
		return RewriteFailed, err
	}
	refs = append(refs, zeroRef)
	minusOne, err := r.makeInteger(big.NewInt(-1))
	if err != nil {
		return RewriteFailed, err
	}
	minusOneRef, err := r.manager.NewRef(minusOne)
	if err != nil {
		return RewriteFailed, err
	}
	refs = append(refs, minusOneRef)
	family, err := r.manager.ArithmeticFamilyID()
	if err != nil {
		return RewriteFailed, err
	}
	ge, err := r.manager.MkFamilyApp(family, 3, nil, []ast.ASTHandle{integer, zero})
	if err != nil {
		return RewriteFailed, err
	}
	geRef, err := r.manager.NewRef(ge)
	if err != nil {
		return RewriteFailed, err
	}
	refs = append(refs, geRef)
	result, err := r.manager.MkIte(ge, integer, minusOne)
	if err != nil {
		return RewriteFailed, err
	}
	if err = r.manager.SetRef(out, result); err != nil {
		return RewriteFailed, err
	}
	return RewriteDone, nil
}

func (r *SequenceRewriter) sequenceOperation(h ast.ASTHandle, kind int32) (bool, error) {
	return r.manager.IsSequenceOperation(h, kind)
}

func (r *SequenceRewriter) makeRegexEpsilon(regex ast.ASTHandle) (result ast.ASTHandle, err error) {
	regexSort, err := r.manager.GetSort(regex)
	if err != nil {
		return result, err
	}
	sequenceSort, matched, err := r.manager.RegexSequenceSort(regexSort)
	if err != nil || !matched {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return result, err
	}
	empty, err := r.manager.MkSequenceEmpty(sequenceSort)
	if err != nil {
		return result, err
	}
	emptyRef, err := r.manager.NewRef(empty)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, r.manager.DelRef(&emptyRef)) }()
	return r.manager.MkSequenceToRegex(empty)
}

func (r *SequenceRewriter) RewriteRegexConcat(a, b ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if err := r.open(); err != nil {
		return RewriteFailed, err
	}
	fullA, err := r.sequenceOperation(a, 23)
	if err != nil {
		return RewriteFailed, err
	}
	fullB, err := r.sequenceOperation(b, 23)
	if err != nil {
		return RewriteFailed, err
	}
	if fullA && fullB {
		return RewriteDone, r.manager.SetRef(out, a)
	}
	emptyA, err := r.sequenceOperation(a, 22)
	if err != nil {
		return RewriteFailed, err
	}
	if emptyA {
		return RewriteDone, r.manager.SetRef(out, a)
	}
	emptyB, err := r.sequenceOperation(b, 22)
	if err != nil {
		return RewriteFailed, err
	}
	if emptyB {
		return RewriteDone, r.manager.SetRef(out, b)
	}
	epsilonA, err := r.manager.IsRegexEpsilon(a)
	if err != nil {
		return RewriteFailed, err
	}
	if epsilonA {
		return RewriteDone, r.manager.SetRef(out, b)
	}
	epsilonB, err := r.manager.IsRegexEpsilon(b)
	if err != nil {
		return RewriteFailed, err
	}
	if epsilonB {
		return RewriteDone, r.manager.SetRef(out, a)
	}
	return RewriteFailed, nil
}

func (r *SequenceRewriter) RewriteRegexUnion(a, b ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if err := r.open(); err != nil {
		return RewriteFailed, err
	}
	if a == b {
		return RewriteDone, r.manager.SetRef(out, a)
	}
	for _, row := range []struct {
		h    ast.ASTHandle
		kind int32
		out  ast.ASTHandle
	}{{a, 22, b}, {b, 22, a}, {a, 23, a}, {b, 23, b}} {
		matched, err := r.sequenceOperation(row.h, row.kind)
		if err != nil {
			return RewriteFailed, err
		}
		if matched {
			return RewriteDone, r.manager.SetRef(out, row.out)
		}
	}
	starA, err := r.sequenceOperation(a, 14)
	if err != nil {
		return RewriteFailed, err
	}
	epsilonB, err := r.manager.IsRegexEpsilon(b)
	if err != nil {
		return RewriteFailed, err
	}
	if starA && epsilonB {
		return RewriteDone, r.manager.SetRef(out, a)
	}
	starB, err := r.sequenceOperation(b, 14)
	if err != nil {
		return RewriteFailed, err
	}
	epsilonA, err := r.manager.IsRegexEpsilon(a)
	if err != nil {
		return RewriteFailed, err
	}
	if starB && epsilonA {
		return RewriteDone, r.manager.SetRef(out, b)
	}
	return RewriteFailed, nil
}

func (r *SequenceRewriter) RewriteRegexIntersect(a, b ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if err := r.open(); err != nil {
		return RewriteFailed, err
	}
	if a == b {
		return RewriteDone, r.manager.SetRef(out, a)
	}
	for _, row := range []struct {
		h    ast.ASTHandle
		kind int32
		out  ast.ASTHandle
	}{{a, 22, a}, {b, 22, b}, {a, 23, b}, {b, 23, a}} {
		matched, err := r.sequenceOperation(row.h, row.kind)
		if err != nil {
			return RewriteFailed, err
		}
		if matched {
			return RewriteDone, r.manager.SetRef(out, row.out)
		}
	}
	return RewriteFailed, nil
}

func (r *SequenceRewriter) RewriteRegexPlus(a ast.ASTHandle, out ast.ASTRef) (status RewriteStatus, err error) {
	if err = r.open(); err != nil {
		return RewriteFailed, err
	}
	for _, kind := range []int32{22, 23, 13, 14} {
		matched, e := r.sequenceOperation(a, kind)
		if e != nil {
			return RewriteFailed, e
		}
		if matched {
			return RewriteDone, r.manager.SetRef(out, a)
		}
	}
	epsilon, err := r.manager.IsRegexEpsilon(a)
	if err != nil {
		return RewriteFailed, err
	}
	if epsilon {
		return RewriteDone, r.manager.SetRef(out, a)
	}
	star, err := r.manager.MkRegexStar(a)
	if err != nil {
		return RewriteFailed, err
	}
	starRef, err := r.manager.NewRef(star)
	if err != nil {
		return RewriteFailed, err
	}
	defer func() { err = errors.Join(err, r.manager.DelRef(&starRef)) }()
	result, err := r.manager.MkRegexConcat(a, star)
	if err != nil {
		return RewriteFailed, err
	}
	if err = r.manager.SetRef(out, result); err != nil {
		return RewriteFailed, err
	}
	return Rewrite2, nil
}

func (r *SequenceRewriter) RewriteRegexOptional(a ast.ASTHandle, out ast.ASTRef) (status RewriteStatus, err error) {
	if err = r.open(); err != nil {
		return RewriteFailed, err
	}
	epsilon, err := r.makeRegexEpsilon(a)
	if err != nil {
		return RewriteFailed, err
	}
	epsilonRef, err := r.manager.NewRef(epsilon)
	if err != nil {
		return RewriteFailed, err
	}
	defer func() { err = errors.Join(err, r.manager.DelRef(&epsilonRef)) }()
	result, err := r.manager.MkRegexUnion(epsilon, a)
	if err != nil {
		return RewriteFailed, err
	}
	if err = r.manager.SetRef(out, result); err != nil {
		return RewriteFailed, err
	}
	return Rewrite1, nil
}

func (r *SequenceRewriter) RewriteRegexComplement(a ast.ASTHandle, out ast.ASTRef) (status RewriteStatus, err error) {
	if err = r.open(); err != nil {
		return RewriteFailed, err
	}
	for _, row := range []struct {
		from, to int32
	}{{19, 18}, {18, 19}} {
		left, right, matched, e := r.manager.SequenceBinaryOperationArguments(a, row.from)
		if e != nil {
			return RewriteFailed, e
		}
		if !matched {
			continue
		}
		leftComplement, e := r.manager.MkRegexComplement(left)
		if e != nil {
			return RewriteFailed, e
		}
		leftRef, e := r.manager.NewRef(leftComplement)
		if e != nil {
			return RewriteFailed, e
		}
		defer func() { err = errors.Join(err, r.manager.DelRef(&leftRef)) }()
		rightComplement, e := r.manager.MkRegexComplement(right)
		if e != nil {
			return RewriteFailed, e
		}
		rightRef, e := r.manager.NewRef(rightComplement)
		if e != nil {
			return RewriteFailed, e
		}
		defer func() { err = errors.Join(err, r.manager.DelRef(&rightRef)) }()
		var result ast.ASTHandle
		if row.to == 18 {
			result, e = r.manager.MkRegexUnion(leftComplement, rightComplement)
		} else {
			result, e = r.manager.MkRegexIntersect(leftComplement, rightComplement)
		}
		if e != nil {
			return RewriteFailed, e
		}
		if err = r.manager.SetRef(out, result); err != nil {
			return RewriteFailed, err
		}
		return Rewrite2, nil
	}
	sort, err := r.manager.GetSort(a)
	if err != nil {
		return RewriteFailed, err
	}
	empty, err := r.sequenceOperation(a, 22)
	if err != nil {
		return RewriteFailed, err
	}
	if empty {
		result, e := r.manager.MkRegexFull(sort)
		if e != nil {
			return RewriteFailed, e
		}
		return RewriteDone, r.manager.SetRef(out, result)
	}
	full, err := r.sequenceOperation(a, 23)
	if err != nil {
		return RewriteFailed, err
	}
	if full {
		result, e := r.manager.MkRegexEmpty(sort)
		if e != nil {
			return RewriteFailed, e
		}
		return RewriteDone, r.manager.SetRef(out, result)
	}
	return RewriteFailed, nil
}

func (r *SequenceRewriter) assignRegexStar(arg ast.ASTHandle, out ast.ASTRef, status RewriteStatus) (RewriteStatus, error) {
	result, err := r.manager.MkRegexStar(arg)
	if err != nil {
		return RewriteFailed, err
	}
	return status, r.manager.SetRef(out, result)
}

func (r *SequenceRewriter) assignRegexStarUnion(left, right ast.ASTHandle, out ast.ASTRef) (status RewriteStatus, err error) {
	union, err := r.manager.MkRegexUnion(left, right)
	if err != nil {
		return RewriteFailed, err
	}
	unionRef, err := r.manager.NewRef(union)
	if err != nil {
		return RewriteFailed, err
	}
	defer func() { err = errors.Join(err, r.manager.DelRef(&unionRef)) }()
	result, err := r.manager.MkRegexStar(union)
	if err != nil {
		return RewriteFailed, err
	}
	if err = r.manager.SetRef(out, result); err != nil {
		return RewriteFailed, err
	}
	return Rewrite2, nil
}

func (r *SequenceRewriter) RewriteRegexStar(a ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if err := r.open(); err != nil {
		return RewriteFailed, err
	}
	for _, kind := range []int32{14, 23} {
		matched, err := r.sequenceOperation(a, kind)
		if err != nil {
			return RewriteFailed, err
		}
		if matched {
			return RewriteDone, r.manager.SetRef(out, a)
		}
	}
	fullChar, err := r.sequenceOperation(a, 24)
	if err != nil {
		return RewriteFailed, err
	}
	if fullChar {
		sort, err := r.manager.GetSort(a)
		if err != nil {
			return RewriteFailed, err
		}
		result, err := r.manager.MkRegexFull(sort)
		if err != nil {
			return RewriteFailed, err
		}
		return RewriteDone, r.manager.SetRef(out, result)
	}
	empty, err := r.sequenceOperation(a, 22)
	if err != nil {
		return RewriteFailed, err
	}
	if empty {
		epsilon, err := r.makeRegexEpsilon(a)
		if err != nil {
			return RewriteFailed, err
		}
		return RewriteDone, r.manager.SetRef(out, epsilon)
	}
	body, plus, err := r.manager.SequenceUnaryOperationArgument(a, 13)
	if err != nil {
		return RewriteFailed, err
	}
	if plus {
		return r.assignRegexStar(body, out, RewriteDone)
	}
	left, right, union, err := r.manager.SequenceBinaryOperationArguments(a, 18)
	if err != nil {
		return RewriteFailed, err
	}
	if union {
		leftBody, leftStar, e := r.manager.SequenceUnaryOperationArgument(left, 14)
		if e != nil {
			return RewriteFailed, e
		}
		if leftStar {
			return r.assignRegexStarUnion(leftBody, right, out)
		}
		rightBody, rightStar, e := r.manager.SequenceUnaryOperationArgument(right, 14)
		if e != nil {
			return RewriteFailed, e
		}
		if rightStar {
			return r.assignRegexStarUnion(left, rightBody, out)
		}
		leftEpsilon, e := r.manager.IsRegexEpsilon(left)
		if e != nil {
			return RewriteFailed, e
		}
		if leftEpsilon {
			return r.assignRegexStar(right, out, Rewrite2)
		}
		rightEpsilon, e := r.manager.IsRegexEpsilon(right)
		if e != nil {
			return RewriteFailed, e
		}
		if rightEpsilon {
			return r.assignRegexStar(left, out, Rewrite2)
		}
	}
	left, right, concat, err := r.manager.SequenceBinaryOperationArguments(a, 17)
	if err != nil {
		return RewriteFailed, err
	}
	if concat {
		leftBody, leftStar, e := r.manager.SequenceUnaryOperationArgument(left, 14)
		if e != nil {
			return RewriteFailed, e
		}
		rightBody, rightStar, e := r.manager.SequenceUnaryOperationArgument(right, 14)
		if e != nil {
			return RewriteFailed, e
		}
		if leftStar && rightStar {
			return r.assignRegexStarUnion(leftBody, rightBody, out)
		}
	}
	return RewriteFailed, nil
}

func (r *SequenceRewriter) RewriteRegexLoop(args []ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if err := r.open(); err != nil {
		return RewriteFailed, err
	}
	if len(args) != 2 && len(args) != 3 {
		return RewriteFailed, nil
	}
	lower, matched, err := r.integerValue(args[1])
	if err != nil || !matched || lower.Sign() < 0 || !lower.IsUint64() || lower.Uint64() > math.MaxInt32 {
		return RewriteFailed, err
	}
	var result ast.ASTHandle
	if len(args) == 2 {
		result, err = r.manager.MkRegexLoop(args[0], int32(lower.Uint64()))
	} else {
		upper, upperMatched, e := r.integerValue(args[2])
		if e != nil || !upperMatched || upper.Sign() < 0 || !upper.IsUint64() || upper.Uint64() > math.MaxInt32 {
			return RewriteFailed, e
		}
		result, err = r.manager.MkRegexLoop(args[0], int32(lower.Uint64()), int32(upper.Uint64()))
	}
	if err != nil {
		return RewriteFailed, err
	}
	return RewriteDone, r.manager.SetRef(out, result)
}

func (r *SequenceRewriter) RewriteStringInRegex(sequence, regex ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	return r.rewriteStringInRegex(sequence, regex, out)
}

func (r *SequenceRewriter) sequenceMinimumLength(parts []ast.ASTHandle) (length uint64, bounded bool, err error) {
	bounded = true
	for _, part := range parts {
		unit, e := r.manager.IsSequenceUnit(part)
		if e != nil {
			return 0, false, e
		}
		if unit {
			length++
			continue
		}
		empty, e := r.manager.IsSequenceEmpty(part)
		if e != nil {
			return 0, false, e
		}
		if empty {
			continue
		}
		text, stringValue, e := r.manager.SequenceStringValue(part)
		if e != nil {
			return 0, false, e
		}
		if stringValue {
			length += uint64(len(text))
		} else {
			bounded = false
		}
	}
	return length, bounded, nil
}

func (r *SequenceRewriter) cannotContainPrefix(a, b ast.ASTHandle) (bool, error) {
	unitA, err := r.manager.IsSequenceUnit(a)
	if err != nil {
		return false, err
	}
	unitB, err := r.manager.IsSequenceUnit(b)
	if err != nil {
		return false, err
	}
	if unitA && unitB {
		distinct, err := r.manager.AreDistinct(a, b)
		if err != nil || distinct {
			return distinct, err
		}
	}
	left, leftString, err := r.manager.SequenceStringValue(a)
	if err != nil {
		return false, err
	}
	right, rightString, err := r.manager.SequenceStringValue(b)
	if err != nil || !leftString || !rightString {
		return false, err
	}
	for i := 0; i < len(left); i++ {
		if strings.HasSuffix(right, left[i:]) {
			return false, nil
		}
	}
	return true, nil
}

func (r *SequenceRewriter) cannotContainSuffix(a, b ast.ASTHandle) (bool, error) {
	unitA, err := r.manager.IsSequenceUnit(a)
	if err != nil {
		return false, err
	}
	unitB, err := r.manager.IsSequenceUnit(b)
	if err != nil {
		return false, err
	}
	if unitA && unitB {
		distinct, err := r.manager.AreDistinct(a, b)
		if err != nil || distinct {
			return distinct, err
		}
	}
	left, leftString, err := r.manager.SequenceStringValue(a)
	if err != nil {
		return false, err
	}
	right, rightString, err := r.manager.SequenceStringValue(b)
	if err != nil || !leftString || !rightString {
		return false, err
	}
	for i := 1; i <= len(left); i++ {
		if strings.HasSuffix(right, left[:i]) {
			return false, nil
		}
	}
	return true, nil
}

func (r *SequenceRewriter) assignSequenceEmptyEquality(value ast.ASTHandle, status RewriteStatus, out ast.ASTRef) (resultStatus RewriteStatus, err error) {
	sort, err := r.manager.GetSort(value)
	if err != nil {
		return RewriteFailed, err
	}
	empty, err := r.manager.MkSequenceEmpty(sort)
	if err != nil {
		return RewriteFailed, err
	}
	emptyRef, err := r.manager.NewRef(empty)
	if err != nil {
		return RewriteFailed, err
	}
	defer func() { err = errors.Join(err, r.manager.DelRef(&emptyRef)) }()
	equality, err := r.manager.MkEq(value, empty)
	if err != nil {
		return RewriteFailed, err
	}
	if err = r.manager.SetRef(out, equality); err != nil {
		return RewriteFailed, err
	}
	return status, nil
}

func (r *SequenceRewriter) RewriteContains(a, b ast.ASTHandle, out ast.ASTRef) (status RewriteStatus, err error) {
	if err = r.open(); err != nil {
		return RewriteFailed, err
	}
	left, leftString, err := r.manager.SequenceStringValue(a)
	if err != nil {
		return RewriteFailed, err
	}
	right, rightString, err := r.manager.SequenceStringValue(b)
	if err != nil {
		return RewriteFailed, err
	}
	if leftString && rightString {
		return r.assignBoolean(strings.Contains(left, right), out)
	}
	as, err := r.manager.SequenceFlattenConcat(a)
	if err != nil {
		return RewriteFailed, err
	}
	bs, err := r.manager.SequenceFlattenConcat(b)
	if err != nil {
		return RewriteFailed, err
	}
	if len(bs) == 0 {
		return r.assignBoolean(true, out)
	}
	allValues := true
	for _, part := range bs {
		value, e := r.manager.IsValue(part)
		if e != nil {
			return RewriteFailed, e
		}
		allValues = allValues && value
	}
	found := false
	for i := 0; !found && i < len(as); i++ {
		value, e := r.manager.IsValue(as[i])
		if e != nil {
			return RewriteFailed, e
		}
		allValues = allValues && value
		if len(bs) <= len(as)-i {
			j := 0
			for ; j < len(bs) && as[i+j] == bs[j]; j++ {
			}
			found = j == len(bs)
		}
	}
	if found {
		return r.assignBoolean(true, out)
	}
	if allValues {
		return r.assignBoolean(false, out)
	}
	lengthA, boundedA, err := r.sequenceMinimumLength(as)
	if err != nil {
		return RewriteFailed, err
	}
	if boundedA {
		lengthB, _, e := r.sequenceMinimumLength(bs)
		if e != nil {
			return RewriteFailed, e
		}
		if lengthB > lengthA {
			return r.assignBoolean(false, out)
		}
	}
	if len(as) == 0 {
		return r.assignSequenceEmptyEquality(b, Rewrite2, out)
	}
	if len(bs) == 1 {
		needle, stringNeedle, e := r.manager.SequenceStringValue(bs[0])
		if e != nil {
			return RewriteFailed, e
		}
		if stringNeedle {
			for _, part := range as {
				text, stringPart, e := r.manager.SequenceStringValue(part)
				if e != nil {
					return RewriteFailed, e
				}
				if stringPart && strings.Contains(text, needle) {
					return r.assignBoolean(true, out)
				}
			}
		}
	}
	offset := 0
	for offset < len(as) {
		cannot, e := r.cannotContainPrefix(as[offset], bs[0])
		if e != nil {
			return RewriteFailed, e
		}
		if !cannot {
			break
		}
		offset++
	}
	size := len(as)
	for size > offset {
		cannot, e := r.cannotContainSuffix(as[size-1], bs[len(bs)-1])
		if e != nil {
			return RewriteFailed, e
		}
		if !cannot {
			break
		}
		size--
	}
	if offset == size {
		return r.assignSequenceEmptyEquality(b, Rewrite2, out)
	}
	if offset > 0 || size < len(as) {
		baseRef, base, e := r.makeSequencePartsRef(as[offset:size], a)
		if e != nil {
			return RewriteFailed, e
		}
		defer func() { err = errors.Join(err, r.manager.DelRef(&baseRef)) }()
		result, e := r.manager.MkSequenceContains(base, b)
		if e != nil {
			return RewriteFailed, e
		}
		if err = r.manager.SetRef(out, result); err != nil {
			return RewriteFailed, err
		}
		return Rewrite2, nil
	}
	extracted, _, _, extract, err := r.manager.SequenceExtractArguments(b)
	if err != nil {
		return RewriteFailed, err
	}
	if extract && extracted == a {
		return r.assignBoolean(true, out)
	}
	allUnits := true
	for _, part := range bs {
		allUnits, err = r.manager.IsSequenceUnit(part)
		if err != nil {
			return RewriteFailed, err
		}
	}
	for _, part := range as {
		allUnits, err = r.manager.IsSequenceUnit(part)
		if err != nil {
			return RewriteFailed, err
		}
	}
	if !allUnits {
		return RewriteFailed, nil
	}
	if len(as) < len(bs) {
		return r.assignBoolean(false, out)
	}
	refs := make([]ast.ASTRef, 0, len(as)*len(bs))
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			err = errors.Join(err, r.manager.DelRef(&refs[i]))
		}
	}()
	disjuncts := make([]ast.ASTHandle, 0, len(as)-len(bs)+1)
	for i := 0; i <= len(as)-len(bs); i++ {
		conjuncts := make([]ast.ASTHandle, 0, len(bs))
		for j := range bs {
			equality, e := r.manager.MkEq(as[i+j], bs[j])
			if e != nil {
				return RewriteFailed, e
			}
			ref, e := r.manager.NewRef(equality)
			if e != nil {
				return RewriteFailed, e
			}
			refs = append(refs, ref)
			conjuncts = append(conjuncts, equality)
		}
		conjunction, e := r.manager.MkAndArray(conjuncts)
		if e != nil {
			return RewriteFailed, e
		}
		ref, e := r.manager.NewRef(conjunction)
		if e != nil {
			return RewriteFailed, e
		}
		refs = append(refs, ref)
		disjuncts = append(disjuncts, conjunction)
	}
	result, err := r.manager.MkOrArray(disjuncts)
	if err != nil {
		return RewriteFailed, err
	}
	if err = r.manager.SetRef(out, result); err != nil {
		return RewriteFailed, err
	}
	return RewriteFull, nil
}

func (r *SequenceRewriter) RewritePrefix(a, b ast.ASTHandle, out ast.ASTRef) (status RewriteStatus, err error) {
	if err = r.open(); err != nil {
		return RewriteFailed, err
	}
	left, leftString, err := r.manager.SequenceStringValue(a)
	if err != nil {
		return RewriteFailed, err
	}
	right, rightString, err := r.manager.SequenceStringValue(b)
	if err != nil {
		return RewriteFailed, err
	}
	if leftString && rightString {
		return r.assignBoolean(strings.HasPrefix(right, left), out)
	}
	empty, err := r.manager.IsSequenceEmpty(a)
	if err != nil {
		return RewriteFailed, err
	}
	if empty {
		return r.assignBoolean(true, out)
	}
	a1, err := r.manager.SequenceLeftmostConcat(a)
	if err != nil {
		return RewriteFailed, err
	}
	b1, err := r.manager.SequenceLeftmostConcat(b)
	if err != nil {
		return RewriteFailed, err
	}
	left, leftString, err = r.manager.SequenceStringValue(a1)
	if err != nil {
		return RewriteFailed, err
	}
	right, rightString, err = r.manager.SequenceStringValue(b1)
	if err != nil {
		return RewriteFailed, err
	}
	if a1 != b1 && leftString && rightString {
		if len(left) <= len(right) {
			if !strings.HasPrefix(right, left) {
				return r.assignBoolean(false, out)
			}
			if a == a1 {
				return r.assignBoolean(true, out)
			}
			as, e := r.manager.SequenceFlattenConcat(a)
			if e != nil {
				return RewriteFailed, e
			}
			bs, e := r.manager.SequenceFlattenConcat(b)
			if e != nil {
				return RewriteFailed, e
			}
			stripped, e := r.manager.MkString(right[len(left):])
			if e != nil {
				return RewriteFailed, e
			}
			strippedRef, e := r.manager.NewRef(stripped)
			if e != nil {
				return RewriteFailed, e
			}
			defer func() { err = errors.Join(err, r.manager.DelRef(&strippedRef)) }()
			bs[0] = stripped
			return r.assignSequenceRelation(as[1:], bs, a, b, true, RewriteFull, out)
		}
		if !strings.HasPrefix(left, right) {
			return r.assignBoolean(false, out)
		}
		if b == b1 {
			return r.assignBoolean(false, out)
		}
		as, e := r.manager.SequenceFlattenConcat(a)
		if e != nil {
			return RewriteFailed, e
		}
		bs, e := r.manager.SequenceFlattenConcat(b)
		if e != nil {
			return RewriteFailed, e
		}
		stripped, e := r.manager.MkString(left[len(right):])
		if e != nil {
			return RewriteFailed, e
		}
		strippedRef, e := r.manager.NewRef(stripped)
		if e != nil {
			return RewriteFailed, e
		}
		defer func() { err = errors.Join(err, r.manager.DelRef(&strippedRef)) }()
		as[0] = stripped
		return r.assignSequenceRelation(as, bs[1:], a, b, true, RewriteFull, out)
	}
	as, err := r.manager.SequenceFlattenConcat(a)
	if err != nil {
		return RewriteFailed, err
	}
	bs, err := r.manager.SequenceFlattenConcat(b)
	if err != nil {
		return RewriteFailed, err
	}
	equalities := make([]ast.ASTHandle, 0, len(as))
	refs := make([]ast.ASTRef, 0, len(as)+1)
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			err = errors.Join(err, r.manager.DelRef(&refs[i]))
		}
	}()
	i := 0
	for ; i < len(as) && i < len(bs); i++ {
		if as[i] == bs[i] {
			continue
		}
		unitA, e := r.manager.IsSequenceUnit(as[i])
		if e != nil {
			return RewriteFailed, e
		}
		unitB, e := r.manager.IsSequenceUnit(bs[i])
		if e != nil {
			return RewriteFailed, e
		}
		if unitA && unitB {
			eq, e := r.manager.MkEq(as[i], bs[i])
			if e != nil {
				return RewriteFailed, e
			}
			ref, e := r.manager.NewRef(eq)
			if e != nil {
				return RewriteFailed, e
			}
			refs = append(refs, ref)
			equalities = append(equalities, eq)
			continue
		}
		_, stringA, e := r.manager.SequenceStringValue(as[i])
		if e != nil {
			return RewriteFailed, e
		}
		_, stringB, e := r.manager.SequenceStringValue(bs[i])
		if e != nil {
			return RewriteFailed, e
		}
		valueA, e := r.manager.IsValue(as[i])
		if e != nil {
			return RewriteFailed, e
		}
		valueB, e := r.manager.IsValue(bs[i])
		if e != nil {
			return RewriteFailed, e
		}
		if valueA && valueB && stringA && stringB {
			return r.assignBoolean(false, out)
		}
		break
	}
	if i == len(as) {
		result, e := r.manager.MkAndArray(equalities)
		if e != nil {
			return RewriteFailed, e
		}
		if err = r.manager.SetRef(out, result); err != nil {
			return RewriteFailed, err
		}
		return Rewrite3, nil
	}
	if i == len(bs) {
		sort, e := r.manager.GetSort(a)
		if e != nil {
			return RewriteFailed, e
		}
		empty, e := r.manager.MkSequenceEmpty(sort)
		if e != nil {
			return RewriteFailed, e
		}
		emptyRef, e := r.manager.NewRef(empty)
		if e != nil {
			return RewriteFailed, e
		}
		refs = append(refs, emptyRef)
		for j := i; j < len(as); j++ {
			eq, e := r.manager.MkEq(empty, as[j])
			if e != nil {
				return RewriteFailed, e
			}
			ref, e := r.manager.NewRef(eq)
			if e != nil {
				return RewriteFailed, e
			}
			refs = append(refs, ref)
			equalities = append(equalities, eq)
		}
		result, e := r.manager.MkAndArray(equalities)
		if e != nil {
			return RewriteFailed, e
		}
		if err = r.manager.SetRef(out, result); err != nil {
			return RewriteFailed, err
		}
		return Rewrite3, nil
	}
	if i > 0 {
		return r.assignSequenceRelation(as[i:], bs[i:], a, b, true, RewriteDone, out)
	}
	return RewriteFailed, nil
}

func (r *SequenceRewriter) RewriteSuffix(a, b ast.ASTHandle, out ast.ASTRef) (status RewriteStatus, err error) {
	if err = r.open(); err != nil {
		return RewriteFailed, err
	}
	if a == b {
		return r.assignBoolean(true, out)
	}
	emptyA, err := r.manager.IsSequenceEmpty(a)
	if err != nil {
		return RewriteFailed, err
	}
	if emptyA {
		return r.assignBoolean(true, out)
	}
	emptyB, err := r.manager.IsSequenceEmpty(b)
	if err != nil {
		return RewriteFailed, err
	}
	if emptyB {
		sort, err := r.manager.GetSort(a)
		if err != nil {
			return RewriteFailed, err
		}
		empty, err := r.manager.MkSequenceEmpty(sort)
		if err != nil {
			return RewriteFailed, err
		}
		emptyRef, err := r.manager.NewRef(empty)
		if err != nil {
			return RewriteFailed, err
		}
		defer func() { err = errors.Join(err, r.manager.DelRef(&emptyRef)) }()
		result, err := r.manager.MkEq(empty, a)
		if err != nil {
			return RewriteFailed, err
		}
		if err = r.manager.SetRef(out, result); err != nil {
			return RewriteFailed, err
		}
		return Rewrite3, nil
	}
	leftPrefix, leftTail, leftConcat, err := r.manager.SequenceConcatParts(a)
	if err != nil {
		return RewriteFailed, err
	}
	left, leftString := "", false
	if leftConcat {
		left, leftString, err = r.manager.SequenceStringValue(leftTail)
		if err != nil {
			return RewriteFailed, err
		}
	}
	if !leftString {
		leftPrefix = ast.ASTHandle{}
		left, leftString, err = r.manager.SequenceStringValue(a)
		if err != nil {
			return RewriteFailed, err
		}
	}
	rightPrefix, rightTail, rightConcat, err := r.manager.SequenceConcatParts(b)
	if err != nil {
		return RewriteFailed, err
	}
	right, rightString := "", false
	if rightConcat {
		right, rightString, err = r.manager.SequenceStringValue(rightTail)
		if err != nil {
			return RewriteFailed, err
		}
	}
	if !rightString {
		rightPrefix = ast.ASTHandle{}
		right, rightString, err = r.manager.SequenceStringValue(b)
		if err != nil {
			return RewriteFailed, err
		}
	}
	if leftString && rightString {
		if len(left) == len(right) {
			return r.assignBoolean(false, out)
		}
		if len(left) < len(right) {
			if !strings.HasSuffix(right, left) {
				return r.assignBoolean(false, out)
			}
			if !leftConcat {
				return r.assignBoolean(true, out)
			}
			stripped, e := r.manager.MkString(right[:len(right)-len(left)])
			if e != nil {
				return RewriteFailed, e
			}
			strippedRef, e := r.manager.NewRef(stripped)
			if e != nil {
				return RewriteFailed, e
			}
			defer func() { err = errors.Join(err, r.manager.DelRef(&strippedRef)) }()
			rightParts := []ast.ASTHandle{stripped}
			if rightConcat {
				rightParts = []ast.ASTHandle{rightPrefix, stripped}
			}
			return r.assignSequenceRelation([]ast.ASTHandle{leftPrefix}, rightParts, a, b, false, RewriteDone, out)
		}
		if !rightConcat {
			return r.assignBoolean(false, out)
		}
		if !strings.HasSuffix(left, right) {
			return r.assignBoolean(false, out)
		}
		stripped, e := r.manager.MkString(left[:len(left)-len(right)])
		if e != nil {
			return RewriteFailed, e
		}
		strippedRef, e := r.manager.NewRef(stripped)
		if e != nil {
			return RewriteFailed, e
		}
		defer func() { err = errors.Join(err, r.manager.DelRef(&strippedRef)) }()
		leftParts := []ast.ASTHandle{stripped}
		if leftConcat {
			leftParts = []ast.ASTHandle{leftPrefix, stripped}
		}
		return r.assignSequenceRelation(leftParts, []ast.ASTHandle{rightPrefix}, a, b, false, RewriteDone, out)
	}
	as, err := r.manager.SequenceFlattenConcat(a)
	if err != nil {
		return RewriteFailed, err
	}
	bs, err := r.manager.SequenceFlattenConcat(b)
	if err != nil {
		return RewriteFailed, err
	}
	changed := false
	for len(as) > 0 && len(bs) > 0 && as[len(as)-1] == bs[len(bs)-1] {
		as = as[:len(as)-1]
		bs = bs[:len(bs)-1]
		changed = true
	}
	if len(as) > 0 && len(bs) > 0 {
		valueA, e := r.manager.IsValue(as[len(as)-1])
		if e != nil {
			return RewriteFailed, e
		}
		valueB, e := r.manager.IsValue(bs[len(bs)-1])
		if e != nil {
			return RewriteFailed, e
		}
		if valueA && valueB {
			return r.assignBoolean(false, out)
		}
	}
	if !changed {
		return RewriteFailed, nil
	}
	if len(as) == 0 {
		return r.assignBoolean(true, out)
	}
	if len(bs) == 0 {
		sort, e := r.manager.GetSort(a)
		if e != nil {
			return RewriteFailed, e
		}
		empty, e := r.manager.MkSequenceEmpty(sort)
		if e != nil {
			return RewriteFailed, e
		}
		refs := make([]ast.ASTRef, 0, len(as)+1)
		defer func() {
			for i := len(refs) - 1; i >= 0; i-- {
				err = errors.Join(err, r.manager.DelRef(&refs[i]))
			}
		}()
		emptyRef, e := r.manager.NewRef(empty)
		if e != nil {
			return RewriteFailed, e
		}
		refs = append(refs, emptyRef)
		equalities := make([]ast.ASTHandle, 0, len(as))
		for _, part := range as {
			eq, e := r.manager.MkEq(empty, part)
			if e != nil {
				return RewriteFailed, e
			}
			ref, e := r.manager.NewRef(eq)
			if e != nil {
				return RewriteFailed, e
			}
			refs = append(refs, ref)
			equalities = append(equalities, eq)
		}
		result, e := r.manager.MkAndArray(equalities)
		if e != nil {
			return RewriteFailed, e
		}
		if err = r.manager.SetRef(out, result); err != nil {
			return RewriteFailed, err
		}
		return Rewrite3, nil
	}
	return r.assignSequenceRelation(as, bs, a, b, false, RewriteDone, out)
}

func (r *SequenceRewriter) RewriteReplace(a, b, replacement ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if err := r.open(); err != nil {
		return RewriteFailed, err
	}
	base, baseString, err := r.manager.SequenceStringValue(a)
	if err != nil {
		return RewriteFailed, err
	}
	pattern, patternString, err := r.manager.SequenceStringValue(b)
	if err != nil {
		return RewriteFailed, err
	}
	insert, replacementString, err := r.manager.SequenceStringValue(replacement)
	if err != nil {
		return RewriteFailed, err
	}
	if baseString && patternString && replacementString {
		result, err := r.manager.MkString(strings.Replace(base, pattern, insert, 1))
		if err != nil {
			return RewriteFailed, err
		}
		return RewriteDone, r.manager.SetRef(out, result)
	}
	if b == replacement {
		return RewriteDone, r.manager.SetRef(out, a)
	}
	if patternString && pattern == "" {
		result, err := r.manager.MkSequenceConcat(replacement, a)
		if err != nil {
			return RewriteFailed, err
		}
		return Rewrite1, r.manager.SetRef(out, result)
	}
	if baseString && base == "" {
		return RewriteDone, r.manager.SetRef(out, a)
	}
	return RewriteFailed, nil
}

// RewriteUnit ports the byte-valued branch of seq_rewriter::mk_seq_unit.
func (r *SequenceRewriter) RewriteUnit(element ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if err := r.open(); err != nil {
		return RewriteFailed, err
	}
	value, matched, err := r.manager.SequenceUnitByteElementValue(element)
	if err != nil || !matched {
		return RewriteFailed, err
	}
	result, err := r.manager.MkString(string([]byte{value}))
	if err != nil {
		return RewriteFailed, err
	}
	return RewriteDone, r.manager.SetRef(out, result)
}

// RewriteConcat ports seq_rewriter::mk_seq_concat. The caller owns traversal
// and applies the returned revisit depth.
func (r *SequenceRewriter) RewriteConcat(a, b ast.ASTHandle, out ast.ASTRef) (status RewriteStatus, err error) {
	if err := r.open(); err != nil {
		return RewriteFailed, err
	}
	s1, stringA, err := r.manager.SequenceStringValue(a)
	if err != nil {
		return RewriteFailed, fmt.Errorf("sequence concat left string: %w", err)
	}
	s2, stringB, err := r.manager.SequenceStringValue(b)
	if err != nil {
		return RewriteFailed, fmt.Errorf("sequence concat right string: %w", err)
	}
	if stringA && stringB {
		result, err := r.manager.MkString(s1 + s2)
		if err != nil {
			return RewriteFailed, err
		}
		return RewriteDone, r.manager.SetRef(out, result)
	}
	c, d, concatA, err := r.manager.SequenceConcatParts(a)
	if err != nil {
		return RewriteFailed, fmt.Errorf("sequence concat left parts: %w", err)
	}
	if concatA {
		right, err := r.manager.MkSequenceConcat(d, b)
		if err != nil {
			return RewriteFailed, fmt.Errorf("sequence concat suffix: %w", err)
		}
		rightRef, err := r.manager.NewRef(right)
		if err != nil {
			return RewriteFailed, fmt.Errorf("sequence concat suffix reference: %w", err)
		}
		defer func() { err = errors.Join(err, r.manager.DelRef(&rightRef)) }()
		result, err := r.manager.MkSequenceConcat(c, right)
		if err != nil {
			return RewriteFailed, fmt.Errorf("sequence concat reassociation: %w", err)
		}
		if err = r.manager.SetRef(out, result); err != nil {
			return RewriteFailed, fmt.Errorf("sequence concat result: %w", err)
		}
		return Rewrite2, nil
	}
	emptyA, err := r.manager.IsSequenceEmpty(a)
	if err != nil {
		return RewriteFailed, err
	}
	if emptyA {
		return RewriteDone, r.manager.SetRef(out, b)
	}
	emptyB, err := r.manager.IsSequenceEmpty(b)
	if err != nil {
		return RewriteFailed, err
	}
	if emptyB {
		return RewriteDone, r.manager.SetRef(out, a)
	}
	if stringA {
		c, d, concatB, err := r.manager.SequenceConcatParts(b)
		if err != nil {
			return RewriteFailed, err
		}
		if concatB {
			s2, stringC, err := r.manager.SequenceStringValue(c)
			if err != nil {
				return RewriteFailed, err
			}
			if stringC {
				prefix, err := r.manager.MkString(s1 + s2)
				if err != nil {
					return RewriteFailed, err
				}
				prefixRef, err := r.manager.NewRef(prefix)
				if err != nil {
					return RewriteFailed, err
				}
				defer func() { err = errors.Join(err, r.manager.DelRef(&prefixRef)) }()
				result, err := r.manager.MkSequenceConcat(prefix, d)
				if err != nil {
					return RewriteFailed, err
				}
				return RewriteDone, r.manager.SetRef(out, result)
			}
		}
	}
	return RewriteFailed, nil
}
