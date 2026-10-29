package rewriter

// Source: src/ast/rewriter/pb_rewriter.cpp:21-64,198-293;
// src/ast/rewriter/pb_rewriter.h:24-61;
// src/ast/rewriter/pb_rewriter_def.h:27-292.

import (
	"errors"
	"math/big"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type pbTerm struct {
	literal ast.ASTHandle
	coeff   *big.Rat
}

type pbTruth int8

const (
	pbFalse pbTruth = -1
	pbUndef pbTruth = 0
	pbTrue  pbTruth = 1
)

type PseudoBooleanRewriter struct {
	self    *PseudoBooleanRewriter
	manager *ast.ASTManager
	family  ast.FamilyID
	closed  bool
}

func (r *PseudoBooleanRewriter) traceTags() *diagnostic.TraceTags {
	return r.manager.BorrowMemoryTraceTags()
}

func NewPseudoBooleanRewriter(m *ast.ASTManager) (*PseudoBooleanRewriter, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	family, err := m.RegisterPseudoBooleanPlugin()
	if err != nil {
		return nil, err
	}
	r := &PseudoBooleanRewriter{manager: m, family: family}
	r.self = r
	return r, nil
}

func (r *PseudoBooleanRewriter) open() error {
	if r == nil || r.self != r || r.closed || r.manager == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (r *PseudoBooleanRewriter) rational(value *unumeral.OwnedRational) (*big.Rat, error) {
	text, err := r.manager.NumeralService().String(value)
	if err != nil {
		return nil, err
	}
	q, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, ast.ErrASTDomain
	}
	return q, nil
}

func (r *PseudoBooleanRewriter) bound(decl ast.ASTHandle) (_ *big.Rat, result error) {
	value, err := r.manager.PseudoBooleanBound(decl)
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, r.manager.NumeralService().Del(&value)) }()
	return r.rational(&value)
}

func (r *PseudoBooleanRewriter) coefficient(decl ast.ASTHandle, index uint32) (_ *big.Rat, result error) {
	value, err := r.manager.PseudoBooleanCoefficient(decl, index)
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, r.manager.NumeralService().Del(&value)) }()
	return r.rational(&value)
}

func (r *PseudoBooleanRewriter) negated(h ast.ASTHandle) (ast.ASTHandle, bool, error) {
	kind, err := r.manager.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return ast.ASTHandle{}, false, err
	}
	family, err := r.manager.GetFamilyID(h)
	if err != nil || family != r.manager.BasicFamilyID() {
		return ast.ASTHandle{}, false, err
	}
	op, err := r.manager.GetDeclKind(h)
	if err != nil || op != ast.ASTOpNot {
		return ast.ASTHandle{}, false, err
	}
	arg, err := r.manager.GetArg(h, 0)
	return arg, err == nil, err
}

func (r *PseudoBooleanRewriter) negate(h ast.ASTHandle) (ast.ASTHandle, error) {
	if r.manager.IsTrue(h) {
		return r.manager.MkFalse()
	}
	if r.manager.IsFalse(h) {
		return r.manager.MkTrue()
	}
	if arg, yes, err := r.negated(h); err != nil || yes {
		return arg, err
	}
	return r.manager.MkNot(h)
}

func (r *PseudoBooleanRewriter) unique(args []pbTerm, k *big.Rat) ([]pbTerm, error) {
	for i := range args {
		if literal, negated, err := r.negated(args[i].literal); err != nil {
			return nil, err
		} else if negated {
			args[i].literal = literal
			k.Sub(k, args[i].coeff)
			args[i].coeff.Neg(args[i].coeff)
		}
	}
	for i := 0; i < len(args); i++ {
		if r.manager.IsTrue(args[i].literal) {
			k.Sub(k, args[i].coeff)
			args[i] = args[len(args)-1]
			args = args[:len(args)-1]
			i--
		} else if r.manager.IsFalse(args[i].literal) {
			args[i] = args[len(args)-1]
			args = args[:len(args)-1]
			i--
		}
	}
	ids := make(map[ast.ASTHandle]uint32, len(args))
	for _, arg := range args {
		id, err := r.manager.GetASTID(arg.literal)
		if err != nil {
			return nil, err
		}
		ids[arg.literal] = id
	}
	sort.Slice(args, func(i, j int) bool { return ids[args[i].literal] < ids[args[j].literal] })
	if len(args) != 0 {
		out := args[:1]
		for _, arg := range args[1:] {
			last := &out[len(out)-1]
			if last.literal == arg.literal {
				last.coeff.Add(last.coeff, arg.coeff)
			} else {
				out = append(out, arg)
			}
		}
		args = out
	}
	out := args[:0]
	for _, arg := range args {
		if arg.coeff.Sign() != 0 {
			out = append(out, arg)
		}
	}
	return out, nil
}

func pbRatLCM(a, b *big.Int) *big.Int {
	if a.Sign() == 0 || b.Sign() == 0 {
		return new(big.Int)
	}
	g := new(big.Int).GCD(nil, nil, a, b)
	return new(big.Int).Abs(new(big.Int).Mul(new(big.Int).Quo(new(big.Int).Set(a), g), b))
}

func pbRatFloor(q *big.Rat) *big.Int {
	n, rem := new(big.Int), new(big.Int)
	n.QuoRem(q.Num(), q.Denom(), rem)
	if q.Sign() < 0 && rem.Sign() != 0 {
		n.Sub(n, big.NewInt(1))
	}
	return n
}

func pbRatCeil(q *big.Rat) *big.Int {
	n, rem := new(big.Int), new(big.Int)
	n.QuoRem(q.Num(), q.Denom(), rem)
	if q.Sign() > 0 && rem.Sign() != 0 {
		n.Add(n, big.NewInt(1))
	}
	return n
}

func (r *PseudoBooleanRewriter) normalize(args []pbTerm, k *big.Rat, equality bool) ([]pbTerm, pbTruth, error) {
	sum := new(big.Rat)
	for i := range args {
		c := new(big.Rat).Set(args[i].coeff)
		if c.Sign() < 0 {
			args[i].coeff.Neg(c)
			literal, err := r.negate(args[i].literal)
			if err != nil {
				return nil, pbUndef, err
			}
			args[i].literal = literal
			k.Sub(k, c)
		}
		sum.Add(sum, args[i].coeff)
	}
	if !equality && k.Sign() <= 0 {
		return args[:0], pbTrue, nil
	}
	if equality && k.Sign() == 0 && len(args) == 0 {
		return args, pbTrue, nil
	}
	if sum.Cmp(k) < 0 {
		return args[:0], pbFalse, nil
	}
	if equality && k.Cmp(sum) == 0 {
		for i := range args {
			args[i].coeff.SetInt64(1)
		}
		k.SetInt64(int64(len(args)))
		return args, pbUndef, nil
	}
	lcm := new(big.Int).Set(k.Denom())
	for _, arg := range args {
		lcm = pbRatLCM(lcm, arg.coeff.Denom())
	}
	if lcm.Cmp(big.NewInt(1)) != 0 {
		factor := new(big.Rat).SetInt(lcm)
		k.Mul(k, factor)
		for i := range args {
			args[i].coeff.Mul(args[i].coeff, factor)
		}
	}
	if equality {
		return args, pbUndef, nil
	}
	sum.SetInt64(0)
	for i := range args {
		if args[i].coeff.Cmp(k) > 0 {
			args[i].coeff.Set(k)
		}
		sum.Add(sum, args[i].coeff)
	}
	if len(args) == 0 {
		return args, pbFalse, nil
	}
	if sum.Cmp(k) == 0 {
		for i := range args {
			args[i].coeff.SetInt64(1)
		}
		k.SetInt64(int64(len(args)))
	}
	g := new(big.Int)
	for _, arg := range args {
		if arg.coeff.Cmp(k) == 0 {
			continue
		}
		c := arg.coeff.Num()
		if g.Sign() == 0 {
			g.Set(c)
		} else if g.Cmp(big.NewInt(1)) != 0 {
			g.GCD(nil, nil, g, c)
		}
	}
	if g.Sign() == 0 {
		for i := range args {
			args[i].coeff.SetInt64(1)
		}
		k.SetInt64(1)
	} else if g.Cmp(big.NewInt(1)) > 0 {
		kInt := new(big.Int).Set(k.Num())
		kNew := new(big.Int).Quo(kInt, g)
		if new(big.Int).Rem(kInt, g).Sign() != 0 {
			kNew.Add(kNew, big.NewInt(1))
		}
		for i := range args {
			c := new(big.Int).Set(args[i].coeff.Num())
			if args[i].coeff.Cmp(k) == 0 {
				c.Set(kNew)
			} else {
				c.Quo(c, g)
			}
			args[i].coeff.SetInt(c)
		}
		k.SetInt(kNew)
	}
	if k.Cmp(big.NewRat(1, 1)) != 0 {
		min, max := new(big.Rat).Set(args[0].coeff), new(big.Rat).Set(args[0].coeff)
		for _, arg := range args[1:] {
			if arg.coeff.Cmp(min) < 0 {
				min.Set(arg.coeff)
			}
			if arg.coeff.Cmp(max) > 0 {
				max.Set(arg.coeff)
			}
		}
		n0 := new(big.Rat).Quo(new(big.Rat).Set(k), max)
		n1 := pbRatFloor(n0)
		n2 := pbRatCeil(new(big.Rat).Quo(new(big.Rat).Set(k), min))
		n2.Sub(n2, big.NewInt(1))
		if n1.Cmp(n2) == 0 && !n0.IsInt() {
			for i := range args {
				args[i].coeff.SetInt64(1)
			}
			k.SetInt(new(big.Int).Add(n1, big.NewInt(1)))
		}
	}
	return args, pbUndef, nil
}

func (r *PseudoBooleanRewriter) prune(args []pbTerm, k *big.Rat, equality bool) ([]pbTerm, error) {
	if equality {
		return args, nil
	}
	nlt := new(big.Rat)
	occurrences := 0
	for i := 0; nlt.Cmp(k) < 0 && i < len(args); i++ {
		if args[i].coeff.Cmp(k) < 0 {
			nlt.Add(nlt, args[i].coeff)
			occurrences++
		}
	}
	if occurrences > 0 && nlt.Cmp(k) < 0 {
		for i := 0; i < len(args); i++ {
			if args[i].coeff.Cmp(k) < 0 {
				args[i] = args[len(args)-1]
				args = args[:len(args)-1]
				i--
			}
		}
		var err error
		args, err = r.unique(args, k)
		if err != nil {
			return nil, err
		}
		args, _, err = r.normalize(args, k, false)
		return args, err
	}
	return args, nil
}

func (r *PseudoBooleanRewriter) ownedRational(q *big.Rat) (unumeral.OwnedRational, error) {
	s := r.manager.NumeralService()
	v, err := s.NewValue()
	if err != nil {
		return v, err
	}
	if err = s.SetString(&v, q.RatString()); err != nil {
		return v, errors.Join(err, s.Del(&v))
	}
	return v, nil
}

func (r *PseudoBooleanRewriter) build(args []pbTerm, k *big.Rat, equality bool) (result ast.ASTHandle, err error) {
	handles := make([]ast.ASTHandle, len(args))
	coefficients := make([]unumeral.OwnedRational, len(args))
	for i := range args {
		handles[i] = args[i].literal
		coefficients[i], err = r.ownedRational(args[i].coeff)
		if err != nil {
			return result, err
		}
	}
	defer func() {
		for i := range coefficients {
			if coefficients[i] != (unumeral.OwnedRational{}) {
				err = errors.Join(err, r.manager.NumeralService().Del(&coefficients[i]))
			}
		}
	}()
	bound, err := r.ownedRational(k)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, r.manager.NumeralService().Del(&bound)) }()
	if equality {
		return r.manager.MkPseudoBooleanEQ(coefficients, handles, &bound)
	}
	return r.manager.MkPseudoBooleanGE(coefficients, handles, &bound)
}

func (r *PseudoBooleanRewriter) RewriteApp(decl ast.ASTHandle, arguments ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	if err := r.open(); err != nil {
		return RewriteFailed, err
	}
	aux, err := r.manager.IsPseudoBooleanAux(decl)
	if err != nil {
		return RewriteFailed, err
	}
	if aux {
		return RewriteFailed, nil
	}
	family, err := r.manager.GetFamilyID(decl)
	if err != nil || family != r.family {
		return RewriteFailed, err
	}
	kind, err := r.manager.GetDeclKind(decl)
	if err != nil || kind < ast.ASTPBOpAtMost || kind > ast.ASTPBOpEQ {
		return RewriteFailed, err
	}
	k, err := r.bound(decl)
	if err != nil {
		return RewriteFailed, err
	}
	args := make([]pbTerm, arguments.Count())
	for i := range args {
		args[i].literal, err = arguments.At(uint32(i))
		if err != nil {
			return RewriteFailed, err
		}
		args[i].coeff, err = r.coefficient(decl, uint32(i))
		if err != nil {
			return RewriteFailed, err
		}
	}
	if kind == ast.ASTPBOpAtMost || kind == ast.ASTPBOpLE {
		for i := range args {
			args[i].coeff.Neg(args[i].coeff)
		}
		k.Neg(k)
	}
	equality := kind == ast.ASTPBOpEQ
	args, err = r.unique(args, k)
	if err != nil {
		return RewriteFailed, err
	}
	var truth pbTruth
	args, truth, err = r.normalize(args, k, equality)
	if err != nil {
		return RewriteFailed, err
	}
	args, err = r.prune(args, k, equality)
	if err != nil {
		return RewriteFailed, err
	}
	var value ast.ASTHandle
	switch truth {
	case pbTrue:
		value, err = r.manager.MkTrue()
	case pbFalse:
		value, err = r.manager.MkFalse()
	default:
		allUnit := true
		for _, arg := range args {
			allUnit = allUnit && arg.coeff.Cmp(big.NewRat(1, 1)) == 0
		}
		if equality && len(args) == 0 {
			if k.Sign() == 0 {
				value, err = r.manager.MkTrue()
			} else {
				value, err = r.manager.MkFalse()
			}
		} else if !equality && allUnit && k.Cmp(big.NewRat(1, 1)) == 0 {
			handles := make([]ast.ASTHandle, len(args))
			for i := range args {
				handles[i] = args[i].literal
			}
			value, err = r.manager.MkOrArray(handles)
		} else if !equality && allUnit && k.Cmp(new(big.Rat).SetInt64(int64(len(args)))) == 0 {
			handles := make([]ast.ASTHandle, len(args))
			for i := range args {
				handles[i] = args[i].literal
			}
			value, err = r.manager.MkAndArray(handles)
		} else {
			value, err = r.build(args, k, equality)
		}
	}
	if err != nil {
		return RewriteFailed, err
	}
	if err = r.manager.SetRef(out, value); err != nil {
		return RewriteFailed, err
	}
	if err = r.traceRewrite(decl, arguments, value); err != nil {
		return RewriteFailed, err
	}
	return RewriteDone, nil
}

func (r *PseudoBooleanRewriter) Close() error {
	if r == nil || r.self != r {
		return ast.ErrASTHandle
	}
	if r.closed {
		return nil
	}
	r.closed = true
	return nil
}
