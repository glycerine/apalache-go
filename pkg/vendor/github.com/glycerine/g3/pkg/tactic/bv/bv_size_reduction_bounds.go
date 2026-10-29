package bv

// Source: src/tactic/bv/bv_size_reduction_tactic.cpp signed bounds and reduction.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
)

type bvSizeSubstitution struct{ value ast.ASTExprSubstitution }
type bvSizeReductionImplementation struct {
	family        ast.FamilyID
	lower, upper  map[ast.ASTHandle]*big.Int
	keys          map[ast.ASTHandle]bool
	held, scratch []ast.ASTHandle
	replacer      *rewriter.DefaultExprReplacer
	out           ast.ASTRef
	substitutions []*bvSizeSubstitution
	active        *bvSizeSubstitution
	extension     *tactic.ExtensionModelConverter
	filter        *tactic.FilterModelConverter
	pending       []tactic.ModelConverter
	ready         bool
}

func bvSizeSigned(v *big.Int, width uint32) *big.Int {
	modulus := new(big.Int).Lsh(big.NewInt(1), uint(width))
	result := new(big.Int).Mod(v, modulus)
	if result.Bit(int(width)-1) != 0 {
		result.Sub(result, modulus)
	}
	return result
}
func (t *BVSizeReductionTactic) retain(r *bvSizeReductionImplementation, h ast.ASTHandle, scratch bool) error {
	if err := t.manager.IncRef(h); err != nil {
		return err
	}
	if scratch {
		r.scratch = append(r.scratch, h)
	} else {
		r.held = append(r.held, h)
	}
	return nil
}
func (t *BVSizeReductionTactic) releaseScratch(r *bvSizeReductionImplementation, limit int) error {
	for len(r.scratch) > limit {
		i := len(r.scratch) - 1
		if err := t.manager.DecRef(r.scratch[i]); err != nil {
			return err
		}
		r.scratch = r.scratch[:i]
	}
	return nil
}
func (t *BVSizeReductionTactic) updateBound(r *bvSizeReductionImplementation, h ast.ASTHandle, v *big.Int, lower bool) error {
	if !r.keys[h] {
		if err := t.retain(r, h, false); err != nil {
			return err
		}
		r.keys[h] = true
	}
	table := r.upper
	if lower {
		table = r.lower
	}
	old, found := table[h]
	if !found || (lower && old.Cmp(v) < 0) || (!lower && v.Cmp(old) < 0) {
		table[h] = new(big.Int).Set(v)
	}
	return nil
}
func (t *BVSizeReductionTactic) uninterpretedConstant(h ast.ASTHandle) (bool, error) {
	kind, err := t.manager.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := t.manager.GetFamilyID(h)
	if err != nil || family != ast.NullFamilyID {
		return false, err
	}
	n, err := t.manager.GetNumArgs(h)
	return n == 0, err
}
func (t *BVSizeReductionTactic) collectBounds(r *bvSizeReductionImplementation, g *tactic.Goal) error {
	n, err := g.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < n; i++ {
		f, err := g.Form(i)
		if err != nil {
			return err
		}
		negated, arg, err := t.manager.IsNot(f)
		if err != nil {
			return err
		}
		if negated {
			f = arg
		}
		isSLE, err := t.manager.IsBitVectorOperation(f, 23)
		if err != nil {
			return err
		}
		if !isSLE {
			continue
		}
		lhs, err := t.manager.GetArg(f, 0)
		if err != nil {
			return err
		}
		rhs, err := t.manager.GetArg(f, 1)
		if err != nil {
			return err
		}
		width, err := t.manager.BitVectorSize(lhs)
		if err != nil {
			return err
		}
		leftConst, err := t.uninterpretedConstant(lhs)
		if err != nil {
			return err
		}
		rightConst, err := t.uninterpretedConstant(rhs)
		if err != nil {
			return err
		}
		var variable ast.ASTHandle
		var text string
		var found bool
		lower := false
		if leftConst {
			text, found, err = t.manager.BitVectorNumeralText(rhs)
			if err != nil {
				return err
			}
			if found {
				variable = lhs
			}
		}
		if !found && rightConst {
			text, found, err = t.manager.BitVectorNumeralText(lhs)
			if err != nil {
				return err
			}
			if found {
				variable = rhs
				lower = true
			}
		}
		if !found {
			continue
		}
		if err = t.traceCollected(f, negated, lower); err != nil {
			return err
		}
		value, ok := new(big.Int).SetString(text, 10)
		if !ok {
			return ast.ErrASTDomain
		}
		value = bvSizeSigned(value, width)
		if negated {
			if lower {
				value.Sub(value, big.NewInt(1))
			} else {
				value.Add(value, big.NewInt(1))
			}
			if bvSizeSigned(value, width).Cmp(value) != 0 {
				continue
			} // Source ignores an infeasible extreme bound.
			lower = !lower
		}
		if err = t.updateBound(r, variable, value, lower); err != nil {
			return err
		}
	}
	return nil
}

func (t *BVSizeReductionTactic) definition(r *bvSizeReductionImplementation, v ast.ASTHandle, l, u *big.Int) (def, fresh ast.ASTHandle, err error) {
	width, err := t.manager.BitVectorSize(v)
	if err != nil {
		return def, fresh, err
	}
	if l.Cmp(u) == 0 {
		def, err = t.manager.BitVectorNumeralRat(new(big.Rat).SetInt(l), width)
		return def, fresh, err
	}
	var bits uint32
	var mode int
	if l.Sign() < 0 {
		lowerBits := uint32(new(big.Int).Neg(l).BitLen())
		if u.Sign() < 0 {
			bits = lowerBits
			mode = -1
		} else {
			bits = uint32(u.BitLen())
			if lowerBits > bits {
				bits = lowerBits
			}
			bits++
			mode = 1
		}
	} else {
		bits = uint32(u.BitLen())
		mode = 0
	}
	if err = t.traceWidth(v, l, u, bits, mode); err != nil {
		return def, fresh, err
	}
	if bits >= width {
		return def, fresh, nil
	}
	sort, err := t.manager.BitVectorSort(bits)
	if err != nil {
		return def, fresh, err
	}
	fresh, err = t.manager.MkFreshConstNullable(nil, sort)
	if err != nil {
		return def, fresh, err
	}
	if err = t.retain(r, fresh, true); err != nil {
		return def, fresh, err
	}
	if mode == 1 {
		def, err = t.manager.MkBitVectorSignExtend(width-bits, fresh)
		return def, fresh, err
	}
	var prefix ast.ASTHandle
	if mode < 0 {
		prefix, err = t.manager.BitVectorNumeralAllOnes(width - bits)
	} else {
		prefix, err = t.manager.BitVectorNumeralUnsigned(0, width-bits)
	}
	if err != nil {
		return def, fresh, err
	}
	if err = t.retain(r, prefix, true); err != nil {
		return def, fresh, err
	}
	def, err = t.manager.MkBitVectorConcatPair(prefix, fresh)
	return def, fresh, err
}

func (t *BVSizeReductionTactic) releaseConverters(r *bvSizeReductionImplementation, extension, filter bool) error {
	var err error
	if filter && r.filter != nil {
		owner := r.filter
		consumed, e := owner.DecRefConsumed()
		err = errors.Join(err, e)
		if consumed {
			r.filter = nil
			if e != nil {
				r.pending = append(r.pending, owner)
			}
		}
	}
	if extension && r.extension != nil {
		owner := r.extension
		consumed, e := owner.DecRefConsumed()
		err = errors.Join(err, e)
		if consumed {
			r.extension = nil
			if e != nil {
				r.pending = append(r.pending, owner)
			}
		}
	}
	return err
}
func (t *BVSizeReductionTactic) releaseSubstitutions(r *bvSizeReductionImplementation) error {
	for i := len(r.substitutions) - 1; i >= 0; i-- {
		owner := r.substitutions[i]
		if owner == r.active {
			continue
		}
		if err := t.manager.DelExprSubstitution(&owner.value); err != nil {
			return err
		}
		r.substitutions = append(r.substitutions[:i], r.substitutions[i+1:]...)
	}
	return nil
}
func (t *BVSizeReductionTactic) reduce(g *tactic.Goal, result *tactic.TacticResult) (count uint32, completed bool, err error) {
	r := t.current
	m := t.manager
	report, err := tactic.NewTacticReport("bv-size-reduction", g)
	if err != nil {
		return 0, false, err
	}
	defer func() { err = errors.Join(err, report.Close()) }()
	if err = t.collectBounds(r, g); err != nil {
		return 0, false, err
	}
	value, err := m.NewExprSubstitution()
	if err != nil {
		return 0, false, err
	}
	sub := &bvSizeSubstitution{value: value}
	r.substitutions = append(r.substitutions, sub)
	defer func() { err = errors.Join(err, t.releaseSubstitutions(r)) }()
	models, err := g.ModelsEnabled()
	if err != nil {
		return 0, false, err
	}
	if len(r.lower) != 0 && len(r.upper) != 0 {
		if err = t.traceBounds(r); err != nil {
			return 0, false, err
		}
		for variable, lo := range r.lower {
			width, e := m.BitVectorSize(variable)
			if e != nil {
				return 0, false, e
			}
			up, found := r.upper[variable]
			if !found {
				continue
			}
			l, u := bvSizeSigned(lo, width), bvSizeSigned(up, width)
			if e = t.traceInterval(variable, l, u); e != nil {
				return 0, false, e
			}
			if l.Cmp(u) > 0 {
				f, e := m.MkFalse()
				if e != nil {
					return 0, false, e
				}
				return 0, false, g.Assert(f)
			}
			start := len(r.scratch)
			def, fresh, e := t.definition(r, variable, l, u)
			if e != nil {
				return 0, false, e
			}
			if !def.IsNull() {
				if e = t.retain(r, def, true); e != nil {
					return 0, false, e
				}
				if e = m.ExprSubstitutionInsert(sub.value, variable, def, ast.ASTHandle{}, ast.ASTDependency{}); e != nil {
					return 0, false, e
				}
				if models {
					if r.extension == nil {
						r.extension, e = tactic.NewExtensionModel(m, t.services)
						if e != nil {
							return 0, false, e
						}
						if e = r.extension.IncRef(); e != nil {
							return 0, false, e
						}
					}
					decl, e := m.GetDecl(variable)
					if e != nil {
						return 0, false, e
					}
					if e = r.extension.Insert(decl, def); e != nil {
						return 0, false, e
					}
					if !fresh.IsNull() {
						if r.filter == nil {
							r.filter, e = tactic.NewFilterModel(m)
							if e != nil {
								return 0, false, e
							}
							if e = r.filter.IncRef(); e != nil {
								return 0, false, e
							}
							if e = r.filter.BindTrace(t.trace, t.globals); e != nil {
								return 0, false, e
							}
						}
						d, e := m.GetDecl(fresh)
						if e != nil {
							return 0, false, e
						}
						if e = r.filter.Insert(d); e != nil {
							return 0, false, e
						}
					}
				}
				count++
			}
			if e = t.releaseScratch(r, start); e != nil {
				return 0, false, e
			}
		}
	}
	empty, err := m.ExprSubstitutionEmpty(sub.value)
	if err != nil {
		return 0, false, err
	}
	if empty {
		return count, false, nil
	}
	if err = r.replacer.SetSubstitution(&sub.value); err != nil {
		return 0, false, err
	}
	r.active = sub
	if err = t.releaseSubstitutions(r); err != nil {
		return 0, false, err
	}
	n, err := g.Size()
	if err != nil {
		return 0, false, err
	}
	for i := uint32(0); i < n; i++ {
		bad, e := g.Inconsistent()
		if e != nil {
			return 0, false, e
		}
		if bad {
			return count, false, nil
		}
		f, e := g.Form(i)
		if e != nil {
			return 0, false, e
		}
		if e = r.replacer.Rewrite(f, r.out); e != nil {
			return 0, false, e
		}
		newF, e := m.RefGet(r.out)
		if e != nil {
			return 0, false, e
		}
		if e = g.Update(i, newF, ast.ASTHandle{}, ast.ASTDependency{}); e != nil {
			return 0, false, e
		}
	}
	var converter tactic.ModelConverter
	if r.extension != nil {
		converter = r.extension
	}
	if r.filter != nil {
		converter, err = tactic.ConcatModels(m.Budget(), r.filter, converter)
		if err != nil {
			return 0, false, err
		}
	}
	if err = result.SetModelConverter(converter); err != nil {
		if converter != nil && converter != r.filter && converter != r.extension {
			err = errors.Join(err, converter.Close())
		}
		return 0, false, err
	}
	if err = t.releaseConverters(r, true, true); err != nil {
		return 0, false, err
	}
	return count, true, nil
}
