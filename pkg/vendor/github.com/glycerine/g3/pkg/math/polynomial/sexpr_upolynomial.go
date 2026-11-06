package polynomial

// Source math/polynomial/sexpr2upolynomial.cpp, recursive source parser.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"

	ucmdexception "github.com/glycerine/g3/pkg/util/cmdexception"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usexpr "github.com/glycerine/g3/pkg/util/sexpr"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type SexprPolynomialError struct {
	Message   string
	Line, Pos int32
}

func (e *SexprPolynomialError) Error() string { return e.Message }

// The source converter exception derives from cmd_exception. Keep its public
// Go location fields while exposing the same base class to errors.As callers.
func (e *SexprPolynomialError) Unwrap() error {
	return ucmdexception.NewCmdExceptionAt(e.Message, e.Line, e.Pos)
}

var ErrSexprPolynomialPending = errors.New("g3: polynomial conversion cleanup pending")

type sexprPolynomialFrame struct {
	argument UPolynomial
	live     bool
}

// Native frames retain actual source scoped vectors, including stopped cleanup.
// The converter header and actual numerical owners are charged separately.
type SexprPolynomialConverter struct {
	lexicalReceipt unumeral.RationalNumericLexicalReceipt
	arithmetic     sexprPolynomialArithmeticScope
	inputReceipt   upolyRationalInputInvocationReceipt
	borrowed       *usexpr.SexprBorrowedNodes
	self           *SexprPolynomialConverter
	sexpr          *usexpr.SexprManager
	polynomial     *UPolynomialManager
	frames         []sexprPolynomialFrame
	depth          uint32
	header         uint64
	cause          error
	converting     bool
	closed         bool
}

func NewSexprPolynomialConverter(s *usexpr.SexprManager, m *UPolynomialManager) (out *SexprPolynomialConverter, cause error) {
	if m != nil && (m.sexprArithmeticExecuting() || m.pendingSexprArithmetic != nil) {
		return nil, ErrUPolynomialCleanupPending
	}
	cause = s.WithBorrowedNodes(func(*usexpr.SexprBorrowedNodes) error {
		if e := numericRationalService_WithUPolynomialRationals(s.Configuration().Rationals, m, func(*UPolynomialRationalContext) error { return nil }); e != nil {
			return e
		}
		n := uint64(reflect.TypeOf(SexprPolynomialConverter{}).Size())
		if e := s.Configuration().Budget.Reserve(n); e != nil {
			return e
		}
		out = &SexprPolynomialConverter{sexpr: s, polynomial: m, header: n}
		out.self = out
		return nil
	})
	return out, cause
}
func (c *SexprPolynomialConverter) open() error {
	if c == nil || c.self != c {
		return usexpr.ErrSexprOwner
	}
	if c.closed {
		return arena.ErrClosed
	}
	return nil
}
func (c *SexprPolynomialConverter) parseError(h usexpr.SexprHandle, msg string) error {
	r, e := c.borrowed.Record(h)
	if e != nil {
		return e
	}
	return &SexprPolynomialError{msg, int32(r.Line), int32(r.Pos)}
}
func (c *SexprPolynomialConverter) frame() (uint32, error) {
	i := c.depth
	if i == ^uint32(0) {
		return 0, arena.ErrBounds
	}
	if uint64(i) == uint64(len(c.frames)) {
		old := c.frames
		c.frames = append(c.frames, sexprPolynomialFrame{})
		if len(old) != 0 && &old[0] != &c.frames[0] {
			clear(old)
		}
	}
	c.depth++
	return i, nil
}
func (c *SexprPolynomialConverter) Convert(h usexpr.SexprHandle, p UPolynomial) error {
	return c.convertAtDepth(h, p, 0)
}
func (c *SexprPolynomialConverter) convertAtDepth(h usexpr.SexprHandle, p UPolynomial, depth uint32) error {
	return c.convertAtDepthForRoot(h, p, depth, nil)
}
func (c *SexprPolynomialConverter) convertAtDepthForRoot(h usexpr.SexprHandle, p UPolynomial, depth uint32, caller *algebraicRootConstructionScope) error {
	if e := c.open(); e != nil {
		return e
	}
	if c.cause != nil || c.converting || c.arithmetic.active || c.polynomial.sexprArithmeticExecuting() || c.polynomial.pendingSexprArithmetic != nil {
		return ErrSexprPolynomialPending
	}
	c.converting = true
	defer func() { c.converting = false }()
	return c.sexpr.WithBorrowedNodes(func(nodes *usexpr.SexprBorrowedNodes) error {
		c.borrowed = nodes
		defer func() { c.borrowed = nil }()
		return numericRationalService_WithUPolynomialRationalsReceipt(c.sexpr.Configuration().Rationals, c.polynomial, &c.lexicalReceipt, func(ctx *UPolynomialRationalContext) error {
			ctx.inputReceipt = &c.inputReceipt
			if caller != nil {
				caller.parserAccepted = true
			}
			return c.convert(ctx, h, p, depth)
		})
	})
}
func (c *SexprPolynomialConverter) child(r usexpr.SexprBorrowedNode, i uint32) (usexpr.SexprHandle, error) {
	return c.borrowed.Child(r, i)
}
func (c *SexprPolynomialConverter) convert(ctx *UPolynomialRationalContext, h usexpr.SexprHandle, p UPolynomial, depth uint32) (result error) {
	if depth > 1<<16 {
		return c.parseError(h, "invalid univariate polynomial, too complex")
	}
	r, e := c.borrowed.Record(h)
	if e != nil {
		return e
	}
	switch r.Kind {
	case usexpr.SexprComposite:
		num := r.Length
		if num == 0 {
			return c.parseError(h, "invalid univariate polynomial, symbol expected")
		}
		head, e := c.child(r, 0)
		if e != nil {
			return e
		}
		hr, e := c.borrowed.Record(head)
		if e != nil {
			return e
		}
		if hr.Kind != usexpr.SexprSymbol {
			return c.parseError(h, "invalid univariate polynomial, symbol expected")
		}
		op, e := c.operator(hr.Symbol)
		if e != nil {
			return e
		}
		if op != "+" && op != "-" && op != "*" && op != "^" {
			return c.parseError(h, "invalid univariate polynomial, '+', '-', '^' or '*' expected")
		}
		if op == "^" && num != 3 {
			return c.parseError(h, "invalid univariate polynomial, '^' operator expects two arguments")
		}
		if op != "^" && num <= 1 {
			return c.parseError(h, "invalid univariate polynomial, '"+op+"' operator expects at least one argument")
		}
		first, e := c.child(r, 1)
		if e != nil {
			return e
		}
		if e = c.convert(ctx, first, p, depth+1); e != nil {
			return e
		}
		if op == "^" {
			exponent, e := c.child(r, 2)
			if e != nil {
				return e
			}
			er, e := c.borrowed.Record(exponent)
			if e != nil {
				return e
			}
			if er.Kind != usexpr.SexprNumeral {
				return c.parseError(exponent, "invalid univariate polynomial, exponent must be an unsigned integer")
			}
			n, ok, e := ctx.Exponent(er.Numeral)
			if e != nil {
				return e
			}
			if !ok {
				return c.parseError(exponent, "invalid univariate polynomial, exponent must be an unsigned integer")
			}
			return c.arithmeticCall(ctx, "^", p, UPolynomial{}, n)
		}
		if op == "-" && num == 2 {
			return c.arithmeticCall(ctx, "neg", p, UPolynomial{}, 0)
		}
		index, e := c.frame()
		if e != nil {
			return e
		}
		defer func() {
			c.depth--
			// Child numeric cleanup must finish before this older completed vector.
			if c.cause != nil || ctx.Pending() || c.polynomial.rationalInputPending(&c.inputReceipt) || c.arithmetic.active {
				if c.cause == nil {
					c.cause = ErrSexprPolynomialPending
				}
				return
			}
			f := &c.frames[index]
			if f.live {
				if e := c.polynomial.DelPolynomial(&f.argument); e != nil {
					c.cause = e
					ctx.StopVectors(e)
					result = errors.Join(result, e)
				} else {
					f.live = false
				}
			}
		}()
		arg, e := c.polynomial.NewPolynomial()
		if e != nil {
			return e
		}
		c.frames[index] = sexprPolynomialFrame{arg, true}
		for i := uint32(2); i < num; i++ {
			if e = c.polynomial.Reset(arg); e != nil {
				return e
			}
			ch, e := c.child(r, i)
			if e != nil {
				return e
			}
			if e = c.convert(ctx, ch, arg, depth+1); e != nil {
				return e
			}
			e = c.arithmeticCall(ctx, op, p, arg, 0)
			if e != nil {
				return e
			}
		}
		return nil
	case usexpr.SexprNumeral:
		return ctx.SetConstant(p, r.Numeral, func() error { return c.parseError(h, "invalid univariate polynomial, integer coefficient expected") })

	case usexpr.SexprSymbol:
		name, e := c.sexpr.Configuration().Symbols.EqualString(r.Symbol, stringPointer("x"))
		if e != nil {
			return e
		}
		if !name {
			return c.parseError(h, "invalid univariate polynomial, variable 'x' expected")
		}
		return ctx.SetIdentity(p)
	default:
		return c.parseError(h, "invalid univariate polynomial, unexpected ")
	}
}
func (c *SexprPolynomialConverter) Cleanup() error {
	if c != nil && c.self == c && c.converting {
		return ErrSexprPolynomialPending
	}
	if e := c.open(); e != nil {
		return e
	}
	err := numericRationalService_CleanupUPolynomialRationalsReceipt(c.sexpr.Configuration().Rationals, c.polynomial, &c.lexicalReceipt, func() error {
		if e := c.cleanupArithmetic(); e != nil {
			return e
		}
		if e := c.polynomial.cleanupRationalInputInvocation(&c.inputReceipt); e != nil {
			return e
		}
		for i := len(c.frames) - 1; i >= 0; i-- {
			f := &c.frames[i]
			if f.live {
				if e := c.polynomial.DelPolynomial(&f.argument); e != nil {
					return e
				}
				f.live = false
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	c.cause = nil
	return nil
}
func (c *SexprPolynomialConverter) Close() error {
	if c != nil && c.self == c && c.closed {
		return nil
	}
	if e := c.open(); e != nil {
		return e
	}
	if e := c.Cleanup(); e != nil {
		return e
	}
	return c.closeHeader()
}
func (c *SexprPolynomialConverter) closeHeader() error {
	if e := c.open(); e != nil {
		return e
	}
	if c.converting || c.depth != 0 || c.cause != nil {
		return ErrSexprPolynomialPending
	}
	for i := range c.frames {
		if c.frames[i].live {
			return ErrSexprPolynomialPending
		}
	}
	c.sexpr.Configuration().Budget.Release(c.header)
	c.frames = nil
	c.header = 0
	c.closed = true
	return nil
}

func stringPointer(s string) *string { return &s }
func (c *SexprPolynomialConverter) operator(sym usymbol.Symbol) (string, error) {
	if sym.IsNumerical() {
		return "", nil
	}
	for _, op := range []string{"+", "-", "*", "^"} {
		ok, e := c.sexpr.Configuration().Symbols.EqualString(sym, &op)
		if e != nil {
			return "", e
		}
		if ok {
			return op, nil
		}
	}
	return "", nil
}
