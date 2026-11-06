package rewriter

// Source: src/ast/rewriter/bv_bounds.cpp; src/ast/rewriter/bv_bounds.h.

import (
	"errors"
	"math/big"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
)

type bvBoundInterval struct {
	lo, hi  *big.Int
	negated bool
	value   ast.ASTHandle
}

type bvConversion uint8

const (
	bvConverted bvConversion = iota
	bvUnsat
	bvUndefined
)

// BVBounds is the source polynomial-time interval checker for bit-vector
// constraints. Native maps and slices replace obj_map and vector storage.
type BVBounds struct {
	self                       *BVBounds
	manager                    *ast.ASTManager
	lowers, uppers, singletons map[ast.ASTHandle]*big.Int
	negative                   map[ast.ASTHandle][]bvBoundInterval
	okay, closed               bool
}

func NewBVBounds(m *ast.ASTManager) (*BVBounds, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if _, err := m.RegisterBitVectorPlugin(); err != nil {
		return nil, err
	}
	b := &BVBounds{manager: m, okay: true}
	b.self = b
	b.Reset()
	return b, nil
}

func (b *BVBounds) open() error {
	if b == nil || b.self != b || b.closed || b.manager == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func bvCopy(v *big.Int) *big.Int    { return new(big.Int).Set(v) }
func bvPower(width uint32) *big.Int { return new(big.Int).Lsh(big.NewInt(1), uint(width)) }
func bvMax(width uint32) *big.Int   { return new(big.Int).Sub(bvPower(width), big.NewInt(1)) }

func bvBoundsDebugInterval(width uint32, lo, hi *big.Int) error {
	if ast.ASTSourceDebug && (lo == nil || hi == nil || lo.Sign() < 0 || lo.Cmp(hi) > 0 || hi.Cmp(bvPower(width)) >= 0) {
		return ast.ErrASTDomain
	}
	return nil
}

func (b *BVBounds) Reset() {
	if b == nil {
		return
	}
	b.lowers = make(map[ast.ASTHandle]*big.Int)
	b.uppers = make(map[ast.ASTHandle]*big.Int)
	b.singletons = make(map[ast.ASTHandle]*big.Int)
	b.negative = make(map[ast.ASTHandle][]bvBoundInterval)
	b.okay = true
}

func (b *BVBounds) Close() error {
	if b == nil || b.self != b {
		return ast.ErrASTHandle
	}
	if b.closed {
		return nil
	}
	b.Reset()
	b.closed = true
	return nil
}

func (b *BVBounds) IsOkay() bool { return b != nil && b.self == b && !b.closed && b.okay }

// Singletons returns independent integer copies of the source singleton map.
func (b *BVBounds) Singletons() (map[ast.ASTHandle]*big.Int, error) {
	if err := b.open(); err != nil {
		return nil, err
	}
	out := make(map[ast.ASTHandle]*big.Int, len(b.singletons))
	for h, v := range b.singletons {
		out[h] = bvCopy(v)
	}
	return out, nil
}

func (b *BVBounds) isOperation(h ast.ASTHandle, family ast.FamilyID, kind int32) (bool, error) {
	k, err := b.manager.GetKind(h)
	if err != nil || k != ast.ASTApp {
		return false, err
	}
	f, err := b.manager.GetFamilyID(h)
	if err != nil || f != family {
		return false, err
	}
	op, err := b.manager.GetDeclKind(h)
	return op == kind, err
}

func (b *BVBounds) numeral(h ast.ASTHandle) (*big.Int, uint32, bool, error) {
	text, matched, err := b.manager.BitVectorNumeralText(h)
	if err != nil || !matched {
		return nil, 0, matched, err
	}
	value, ok := new(big.Int).SetString(text, 10)
	if !ok {
		return nil, 0, false, ast.ErrASTDomain
	}
	width, err := b.manager.BitVectorSize(h)
	return value, width, err == nil, err
}

func (b *BVBounds) toBound(h ast.ASTHandle) (bool, error) {
	k, err := b.manager.GetKind(h)
	if err != nil || k != ast.ASTApp {
		return false, err
	}
	yes, err := b.manager.IsBitVector(h)
	if err != nil || !yes {
		return false, err
	}
	if yes, err = b.manager.IsBitVectorOperation(h, 4); err != nil || yes { // OP_BADD
		return false, err
	}
	yes, err = b.manager.IsBitVectorNumeral(h)
	return !yes, err
}

func (b *BVBounds) constantAdd(width uint32, h ast.ASTHandle) (ast.ASTHandle, *big.Int, bool, error) {
	if ast.ASTSourceDebug {
		actual, err := b.manager.BitVectorSize(h)
		if err != nil {
			return ast.ASTHandle{}, nil, false, err
		}
		if actual != width {
			return ast.ASTHandle{}, nil, false, ast.ErrASTDomain
		}
	}
	var left, right ast.ASTHandle
	yes, err := b.manager.BitVectorAddOperands(h, &left, &right)
	if err != nil {
		return ast.ASTHandle{}, nil, false, err
	}
	if !yes {
		return h, new(big.Int), true, nil
	}
	if bound, e := b.toBound(left); e != nil {
		return ast.ASTHandle{}, nil, false, e
	} else if bound {
		if value, w, found, e := b.numeral(right); e != nil {
			return ast.ASTHandle{}, nil, false, e
		} else if found && w == width {
			return left, value, true, nil
		}
	}
	if bound, e := b.toBound(right); e != nil {
		return ast.ASTHandle{}, nil, false, e
	} else if bound {
		if value, w, found, e := b.numeral(left); e != nil {
			return ast.ASTHandle{}, nil, false, e
		} else if found && w == width {
			return right, value, true, nil
		}
	}
	return ast.ASTHandle{}, nil, false, nil
}

func (b *BVBounds) record(value ast.ASTHandle, lo, hi *big.Int, negated bool, intervals *[]bvBoundInterval) (bvConversion, error) {
	marker := "["
	if negated {
		marker = "~["
	}
	if err := b.traceValue("record", 25, "record0 ", value, ":"+marker+lo.String()+";"+hi.String()+"]"); err != nil {
		return bvUndefined, err
	}
	width, err := b.manager.BitVectorSize(value)
	if err != nil {
		return bvUndefined, err
	}
	if err = bvBoundsDebugInterval(width, lo, hi); err != nil {
		return bvUndefined, err
	}
	minimum := new(big.Int)
	maximum := bvMax(width)
	if current, found := b.lowers[value]; found {
		minimum = bvCopy(current)
	}
	if current, found := b.uppers[value]; found {
		maximum = bvCopy(current)
	}
	lo, hi = bvCopy(lo), bvCopy(hi)
	loMin := lo.Cmp(minimum) <= 0
	hiMax := hi.Cmp(maximum) >= 0
	if negated {
		if loMin && hiMax {
			return bvUnsat, nil
		}
		if lo.Cmp(maximum) > 0 || hi.Cmp(minimum) < 0 {
			return bvConverted, nil
		}
		if loMin {
			negated = false
			lo.Add(hi, big.NewInt(1))
			hi.Set(maximum)
			loMin, hiMax = lo.Cmp(minimum) <= 0, true
		} else if hiMax {
			negated = false
			hi.Sub(lo, big.NewInt(1))
			lo.Set(minimum)
			hiMax, loMin = hi.Cmp(maximum) >= 0, true
		}
	}
	if loMin {
		lo.Set(minimum)
	}
	if hiMax {
		hi.Set(maximum)
	}
	marker = "["
	if negated {
		marker = "~["
	}
	if err = b.traceValue("record", 57, "record1 ", value, ":"+marker+lo.String()+";"+hi.String()+"]"); err != nil {
		return bvUndefined, err
	}
	if lo.Cmp(hi) > 0 {
		if negated {
			return bvConverted, nil
		}
		return bvUnsat, nil
	}
	if loMin && hiMax {
		if negated {
			return bvUnsat, nil
		}
		return bvConverted, nil
	}
	*intervals = append(*intervals, bvBoundInterval{lo: lo, hi: hi, negated: negated, value: value})
	return bvConverted, nil
}

func (b *BVBounds) BoundLower(value ast.ASTHandle, lower *big.Int) (bool, error) {
	if err := b.open(); err != nil {
		return false, err
	}
	width, err := b.manager.BitVectorSize(value)
	if err != nil {
		return false, err
	}
	if ast.ASTSourceDebug && (lower == nil || lower.Sign() < 0 || lower.Cmp(bvPower(width)) >= 0) {
		return false, ast.ErrASTDomain
	}
	if err = b.traceValue("bound_lo", 524, "lower ", value, ":"+lower.String()); err != nil {
		return false, err
	}
	if current, found := b.lowers[value]; !found || current.Cmp(lower) < 0 {
		b.lowers[value] = bvCopy(lower)
	}
	return b.okay, nil
}

func (b *BVBounds) BoundUpper(value ast.ASTHandle, upper *big.Int) (bool, error) {
	if err := b.open(); err != nil {
		return false, err
	}
	width, err := b.manager.BitVectorSize(value)
	if err != nil {
		return false, err
	}
	if ast.ASTSourceDebug && (upper == nil || upper.Sign() < 0 || upper.Cmp(bvPower(width)) >= 0) {
		return false, ast.ErrASTDomain
	}
	if err = b.traceValue("bound_up", 535, "upper ", value, ":"+upper.String()); err != nil {
		return false, err
	}
	if current, found := b.uppers[value]; !found || upper.Cmp(current) < 0 {
		b.uppers[value] = bvCopy(upper)
	}
	return b.okay, nil
}

func (b *BVBounds) addNegative(value ast.ASTHandle, lo, hi *big.Int) error {
	width, err := b.manager.BitVectorSize(value)
	if err != nil {
		return err
	}
	if err = bvBoundsDebugInterval(width, lo, hi); err != nil {
		return err
	}
	if err = b.traceValue("add_neg_bound", 545, "negative bound ", value, ":"+lo.String()+";"+hi.String()); err != nil {
		return err
	}
	b.negative[value] = append(b.negative[value], bvBoundInterval{lo: bvCopy(lo), hi: bvCopy(hi), value: value})
	return nil
}

func (b *BVBounds) AddBoundUnsigned(value ast.ASTHandle, lo, hi *big.Int, negated bool) (bool, error) {
	if err := b.open(); err != nil {
		return false, err
	}
	width, err := b.manager.BitVectorSize(value)
	if err != nil {
		return false, err
	}
	if err = bvBoundsDebugInterval(width, lo, hi); err != nil {
		return false, err
	}
	marker := "["
	if negated {
		marker = "~["
	}
	if err = b.traceValue("add_bound_unsigned", 453, "bound_unsigned ", value, ": "+marker+lo.String()+";"+hi.String()+"]"); err != nil {
		return false, err
	}
	one, maximum := big.NewInt(1), bvMax(width)
	loMin, hiMax := lo.Sign() == 0, hi.Cmp(maximum) == 0
	if negated {
		switch {
		case loMin && hiMax:
			b.okay = false
		case loMin:
			_, err = b.BoundLower(value, new(big.Int).Add(hi, one))
		case hiMax:
			_, err = b.BoundUpper(value, new(big.Int).Sub(lo, one))
		default:
			err = b.addNegative(value, lo, hi)
		}
		return b.okay, err
	}
	if !loMin {
		if _, err = b.BoundLower(value, lo); err != nil {
			return false, err
		}
	}
	if !hiMax {
		if _, err = b.BoundUpper(value, hi); err != nil {
			return false, err
		}
	}
	return b.okay, nil
}

func (b *BVBounds) convertSigned(value ast.ASTHandle, lo, hi *big.Int, negated bool, intervals *[]bvBoundInterval) (bvConversion, error) {
	width, err := b.manager.BitVectorSize(value)
	if err != nil {
		return bvUndefined, err
	}
	if ast.ASTSourceDebug && (lo == nil || hi == nil || lo.Cmp(hi) > 0) {
		return bvUndefined, ast.ErrASTDomain
	}
	marker := "["
	if negated {
		marker = "~["
	}
	if err = b.traceValue("convert_signed", 476, "convert_signed ", value, ":"+marker+lo.String()+";"+hi.String()+"]"); err != nil {
		return bvUndefined, err
	}
	zero, one, modulus := new(big.Int), big.NewInt(1), bvPower(width)
	loNegative, hiNegative := lo.Sign() < 0, hi.Sign() < 0
	if !loNegative && !hiNegative {
		return b.record(value, lo, hi, negated, intervals)
	}
	if loNegative && hiNegative {
		return b.record(value, new(big.Int).Add(modulus, lo), new(big.Int).Add(modulus, hi), negated, intervals)
	}
	if negated {
		a, err := b.record(value, new(big.Int).Add(modulus, lo), new(big.Int).Sub(modulus, one), true, intervals)
		if err != nil || a == bvUnsat {
			return a, err
		}
		c, err := b.record(value, zero, hi, true, intervals)
		if err != nil || c == bvUnsat {
			return c, err
		}
		return bvConverted, nil
	}
	lower := new(big.Int).Add(hi, one)
	upper := new(big.Int).Sub(new(big.Int).Add(modulus, lo), one)
	if lower.Cmp(upper) <= 0 {
		return b.record(value, lower, upper, true, intervals)
	}
	return bvConverted, nil
}

func (b *BVBounds) AddBoundSigned(value ast.ASTHandle, lo, hi *big.Int, negated bool) (bool, error) {
	marker := " "
	if negated {
		marker = "~"
	}
	if err := b.traceValue("add_bound_signed", 500, "bound_signed ", value, ":"+marker+lo.String()+";"+hi.String()); err != nil {
		return false, err
	}
	var intervals []bvBoundInterval
	converted, err := b.convertSigned(value, lo, hi, negated, &intervals)
	if err != nil {
		return false, err
	}
	if converted == bvUnsat {
		b.okay = false
		return false, nil
	}
	for _, interval := range intervals {
		if _, err = b.AddBoundUnsigned(interval.value, interval.lo, interval.hi, interval.negated); err != nil {
			return false, err
		}
	}
	return b.okay, nil
}

func (b *BVBounds) satisfiableValue(value ast.ASTHandle) (result bool, err error) {
	if err := b.traceValue("is_sat", 606, "is_sat ", value, ""); err != nil {
		return false, err
	}
	defer func() {
		resultText := "0"
		if result {
			resultText = "1"
		}
		traceErr := b.traceValue("is_sat", 608, "is_sat ", value, "\nres: "+resultText)
		err = errors.Join(err, traceErr)
	}()
	width, err := b.manager.BitVectorSize(value)
	if err != nil {
		return false, err
	}
	lower, upper := new(big.Int), bvMax(width)
	if v, found := b.lowers[value]; found {
		lower = bvCopy(v)
	}
	if v, found := b.uppers[value]; found {
		upper = bvCopy(v)
	}
	if lower.Cmp(upper) > 0 {
		return false, nil
	}
	if err = b.traceText("is_sat_core", 623, "is_sat bound:"+lower.String()+"-"+upper.String()); err != nil {
		return false, err
	}
	intervals := b.negative[value]
	sort.Slice(intervals, func(i, j int) bool { return intervals[i].lo.Cmp(intervals[j].lo) < 0 })
	one := big.NewInt(1)
	ptr := bvCopy(lower)
	newLo := bvCopy(lower)
	newHi := new(big.Int).Sub(lower, one)
	sat := false
	for _, interval := range intervals {
		if ptr.Cmp(interval.hi) > 0 {
			continue
		}
		if ptr.Cmp(interval.lo) < 0 {
			if !sat {
				newLo.Set(ptr)
			}
			newHi.Sub(interval.lo, one)
			if newHi.Cmp(upper) > 0 {
				newHi.Set(upper)
			}
			sat = true
		}
		if err = b.traceText("is_sat_core", 644, "is_sat new_lo, new_hi:"+newLo.String()+"-"+newHi.String()); err != nil {
			return false, err
		}
		ptr.Add(interval.hi, one)
		if err = b.traceText("is_sat_core", 646, "is_sat ptr, new_hi:"+ptr.String()+"-"+newHi.String()); err != nil {
			return false, err
		}
		if ptr.Cmp(upper) > 0 {
			break
		}
	}
	if ptr.Cmp(upper) <= 0 {
		if !sat {
			newLo.Set(ptr)
		}
		newHi.Set(upper)
		sat = true
	}
	if newHi.Cmp(upper) < 0 {
		_, err = b.BoundUpper(value, newHi)
	}
	if err == nil && newLo.Cmp(lower) > 0 {
		_, err = b.BoundLower(value, newLo)
	}
	if err != nil {
		return false, err
	}
	if err = b.traceText("is_sat_core", 658, "is_sat new_lo, new_hi:"+newLo.String()+"-"+newHi.String()); err != nil {
		return false, err
	}
	if sat && newLo.Cmp(newHi) == 0 {
		if _, found := b.singletons[value]; found {
			if ast.ASTSourceDebug {
				return false, ast.ErrASTDomain
			}
		} else {
			if err = b.traceValue("record_singleton", 600, "singleton:", value, ":"+newLo.String()); err != nil {
				return false, err
			}
			b.singletons[value] = bvCopy(newLo)
		}
	}
	return sat, nil
}

func (b *BVBounds) IsSatisfiable() (bool, error) {
	if err := b.open(); err != nil {
		return false, err
	}
	if !b.okay {
		return false, nil
	}
	seen := make(map[ast.ASTHandle]bool)
	for _, table := range []map[ast.ASTHandle]*big.Int{b.lowers, b.uppers} {
		for value := range table {
			if seen[value] {
				continue
			}
			seen[value] = true
			if yes, err := b.satisfiableValue(value); err != nil || !yes {
				return yes, err
			}
		}
	}
	for value := range b.negative {
		if seen[value] {
			continue
		}
		if yes, err := b.satisfiableValue(value); err != nil || !yes {
			return yes, err
		}
	}
	return true, nil
}

func (b *BVBounds) bitVectorNumeral(value *big.Int, width uint32) (result ast.ASTHandle, err error) {
	service := b.manager.NumeralService()
	n, err := service.NewValue()
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, service.Del(&n)) }()
	if err = service.SetString(&n, value.String()); err != nil {
		return result, err
	}
	return b.manager.BitVectorNumeral(&n, width)
}

// Rewrite implements bv_bounds::rewrite for bounded Boolean conjunctions and
// disjunctions. The object retains the accumulated bounds, as in the source.
func (b *BVBounds) Rewrite(limit uint32, declaration ast.ASTHandle, arguments ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	if err := b.open(); err != nil {
		return RewriteFailed, err
	}
	family, err := b.manager.GetFamilyID(declaration)
	if err != nil || family != b.manager.BasicFamilyID() {
		return RewriteFailed, err
	}
	kind, err := b.manager.GetDeclKind(declaration)
	if err != nil || kind != ast.ASTOpAnd && kind != ast.ASTOpOr || arguments.Count() > limit {
		return RewriteFailed, err
	}
	negated := kind == ast.ASTOpOr
	type convertedArgument struct {
		original  ast.ASTHandle
		undefined bool
		intervals []bvBoundInterval
	}
	converted := make([]convertedArgument, 0, arguments.Count())
	for i := uint32(0); i < arguments.Count() && b.okay; i++ {
		argument, e := arguments.At(i)
		if e != nil {
			return RewriteFailed, e
		}
		entry := convertedArgument{original: argument}
		status, e := b.convert(argument, &entry.intervals, negated)
		if e != nil {
			return RewriteFailed, e
		}
		entry.undefined = status == bvUndefined
		converted = append(converted, entry)
		if status == bvUnsat {
			b.okay = false
			break
		}
		for _, interval := range entry.intervals {
			if _, e = b.AddBoundUnsigned(interval.value, interval.lo, interval.hi, interval.negated); e != nil {
				return RewriteFailed, e
			}
		}
	}
	sat, err := b.IsSatisfiable()
	if err != nil {
		return RewriteFailed, err
	}
	if !b.okay || !sat {
		value, e := b.manager.MkBoolVal(negated)
		if e != nil {
			return RewriteFailed, e
		}
		return RewriteDone, b.manager.SetRef(out, value)
	}
	remaining := make([]ast.ASTHandle, 0, len(converted)+len(b.singletons))
	hasSingleton := false
	for _, entry := range converted {
		if err = b.traceValue("rewrite", 284, "check red: ", entry.original, ""); err != nil {
			return RewriteFailed, err
		}
		if entry.undefined {
			if err = b.traceText("rewrite", 286, "unprocessed"); err != nil {
				return RewriteFailed, err
			}
			remaining = append(remaining, entry.original)
			continue
		}
		redundant := len(entry.intervals) == 0
		isSingleton := false
		if !redundant {
			value := entry.intervals[0].value
			width, e := b.manager.BitVectorSize(value)
			if e != nil {
				return RewriteFailed, e
			}
			lo, hi := new(big.Int), bvMax(width)
			if v, found := b.lowers[value]; found {
				lo = v
			}
			if v, found := b.uppers[value]; found {
				hi = v
			}
			if err = b.traceValue("rewrite", 302, "bounds: ", value, "["+lo.String()+"-"+hi.String()+"]"); err != nil {
				return RewriteFailed, err
			}
			isSingleton = lo.Cmp(hi) == 0
		}
		if !redundant && !isSingleton {
			remaining = append(remaining, entry.original)
		}
		hasSingleton = hasSingleton || isSingleton
		if err = b.traceValueIf(redundant, "rewrite", 308, "redundant: ", entry.original, ""); err != nil {
			return RewriteFailed, err
		}
	}
	if len(remaining) == int(arguments.Count()) && !hasSingleton {
		return RewriteFailed, nil
	}
	for value, singleton := range b.singletons {
		width, e := b.manager.BitVectorSize(value)
		if e != nil {
			return RewriteFailed, e
		}
		numeral, e := b.bitVectorNumeral(singleton, width)
		if e != nil {
			return RewriteFailed, e
		}
		equality, e := b.manager.MkEq(value, numeral)
		if e != nil {
			return RewriteFailed, e
		}
		if negated {
			equality, e = b.manager.MkNot(equality)
			if e != nil {
				return RewriteFailed, e
			}
		}
		remaining = append(remaining, equality)
	}
	var result ast.ASTHandle
	switch len(remaining) {
	case 0:
		result, err = b.manager.MkBoolVal(!negated)
	case 1:
		result = remaining[0]
	default:
		if negated {
			result, err = b.manager.MkOrArgs(remaining)
		} else {
			result, err = b.manager.MkAndArgs(remaining)
		}
	}
	if err != nil {
		return RewriteFailed, err
	}
	return RewriteDone, b.manager.SetRef(out, result)
}
