package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:26-51;
// src/ast/rewriter/bv_rewriter_params.pyg:1-20;
// scripts/mk_genfile_common.py:895-939.

import (
	uparams "github.com/glycerine/g3/pkg/util/params"
	// BVRewriterConfigurationState holds the derived rewriter's original flags.
	// The enclosing configuration charges these fields and its actual generated
	// helper snapshot once, and retains any stopped lookup/destruction prefix.
)

type BVRewriterConfigurationState struct {
	HiDiv0, EliminateSignExtension, MultiplyToConcat, BitToBool                bool
	Trailing, RemainderSimplification, BlastEqualityValue, SplitConcatEquality bool
	UnsignedDivisionToMultiply, NotToArithmetic, NotSimplification, SortAC     bool
	MkBVToNumeral, ExtractPropagation, IteToIdentity, LEExtra                  bool
}

func (p *BVPolynomialConfiguration) BitVectorState() (BVRewriterConfigurationState, error) {
	if e := p.open(); e != nil {
		return BVRewriterConfigurationState{}, e
	}
	return p.bitVector, nil
}

// UpdateBitVector is the original derived updt_params: base first, then local.
// The base constructor remains independent; its two updates precede the local
// suffix in the eventual complete BV rewriter constructor.
func (p *BVPolynomialConfiguration) UpdateBitVector(local *uparams.ParamsRef) error {
	if e := p.Update(local); e != nil {
		return e
	}
	return p.UpdateBitVectorLocal(local)
}

func (p *BVPolynomialConfiguration) bvBoolean(local *uparams.ParamsRef, key string, def bool, dst *bool) error {
	v, e := p.context.GetBoolFallbackName(local, &key, &p.snapshots[3], def)
	if e == nil {
		*dst = v
	}
	return e
}

func (p *BVPolynomialConfiguration) UpdateBitVectorLocal(local *uparams.ParamsRef) (err error) {
	if e := p.open(); e != nil {
		return e
	}
	if e := p.context.Check(local); e != nil {
		return e
	}
	defer func() {
		if err != nil {
			p.fail(err)
		}
	}()
	p.snapshots[3], err = p.globals.GetModuleName("rewriter")
	if err != nil {
		return err
	}
	p.placed[3] = true
	s := &p.bitVector
	if err = p.bvBoolean(local, "hi_div0", true, &s.HiDiv0); err != nil {
		return err
	}
	if err = p.bvBoolean(local, "elim_sign_ext", true, &s.EliminateSignExtension); err != nil {
		return err
	}
	if err = p.bvBoolean(local, "mul2concat", false, &s.MultiplyToConcat); err != nil {
		return err
	}
	if err = p.bvBoolean(local, "bit2bool", true, &s.BitToBool); err != nil {
		return err
	}
	if err = p.bvBoolean(local, "bv_trailing", false, &s.Trailing); err != nil {
		return err
	}
	if err = p.bvBoolean(local, "bv_urem_simpl", false, &s.RemainderSimplification); err != nil {
		return err
	}
	if err = p.bvBoolean(local, "blast_eq_value", false, &s.BlastEqualityValue); err != nil {
		return err
	}
	if err = p.bvBoolean(local, "split_concat_eq", false, &s.SplitConcatEquality); err != nil {
		return err
	}
	if err = p.bvBoolean(local, "udiv2mul", false, &s.UnsignedDivisionToMultiply); err != nil {
		return err
	}
	if err = p.bvBoolean(local, "bvnot2arith", false, &s.NotToArithmetic); err != nil {
		return err
	}
	if err = p.bvBoolean(local, "bv_not_simpl", false, &s.NotSimplification); err != nil {
		return err
	}
	if err = p.bvBoolean(local, "bv_sort_ac", false, &s.SortAC); err != nil {
		return err
	}
	key := "mkbv2num"
	v, e := p.context.GetBoolName(local, &key, false)
	if e != nil {
		return e
	}
	s.MkBVToNumeral = v
	if err = p.bvBoolean(local, "bv_extract_prop", false, &s.ExtractPropagation); err != nil {
		return err
	}
	if err = p.bvBoolean(local, "bv_ite2id", false, &s.IteToIdentity); err != nil {
		return err
	}
	if err = p.bvBoolean(local, "bv_le_extra", false, &s.LEExtra); err != nil {
		return err
	}
	// Source repeats the lookup after LEExtra; do not reuse the earlier SortAC.
	var sortSums bool
	if err = p.bvBoolean(local, "bv_sort_ac", false, &sortSums); err != nil {
		return err
	}
	if err = p.SetSortSums(sortSums); err != nil {
		return err
	}
	return p.end(3)
}

// Source: src/ast/rewriter/bv_rewriter.h:170.
func (p *BVPolynomialConfiguration) SetMkBVToNumeral(v bool) error {
	if e := p.open(); e != nil {
		return e
	}
	p.bitVector.MkBVToNumeral = v
	return nil
}

// Source: src/ast/rewriter/bv_rewriter.cpp:53-60.
// externalRelease represents the original _EXTERNAL_RELEASE build selection.
func BVRewriterParameterDescriptions(d *uparams.ParamDescrs, externalRelease bool) error {
	if e := BVPolynomialParameterDescriptions(d); e != nil {
		return e
	}
	if e := catalogCollectBvRewriterParams(d); e != nil {
		return e
	}
	if !externalRelease {
		description := "(default: false) convert (mkbv [true/false]*) into a numeral"
		return d.InsertName("mkbv2num", uparams.ParamBool, &description, nil, nil)
	}
	return nil
}
