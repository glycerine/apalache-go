package ast

// Source: src/ast/static_features.cpp:163-365;
// src/ast/static_features.h:128-145.

func staticFeatureIncrement(values *[]uint32, fid FamilyID) {
	if fid < 0 {
		return
	}
	i := int(fid)
	if i >= len(*values) {
		*values = append(*values, make([]uint32, i+1-len(*values))...)
	}
	(*values)[i]++
}

func staticFeatureUninterpretedSortFamily(fid FamilyID) bool {
	return fid == NullFamilyID || fid == 4
}

func (f *StaticFeatures) markTheory(fid FamilyID) {
	// The manager installs the five source builtin families at IDs 0 through 4.
	if fid <= 4 {
		return
	}
	i := int(fid)
	if i >= len(f.theories) {
		f.theories = append(f.theories, make([]bool, i+1-len(f.theories))...)
	}
	if !f.theories[i] {
		f.theories[i] = true
		f.NumMarkedTheories++
	}
}

func (f *StaticFeatures) classifySort(sort ASTHandle) (FamilyID, error) {
	fid, err := f.manager.GetFamilyID(sort)
	if err != nil {
		return NullFamilyID, err
	}
	f.markTheory(fid)
	kind, err := f.manager.GetDeclKind(sort)
	if err != nil {
		return NullFamilyID, err
	}
	switch fid {
	case f.arithFID:
		if kind == 0 {
			f.HasReal = true
		} else if kind == 1 {
			f.HasInt = true
		}
	case f.bitVectorFID:
		if kind == 0 {
			f.HasBitVector = true
		}
	case f.fpaFID:
		if kind == 0 || kind == 1 {
			f.HasFloatingPoint = true
		}
	case f.arrayFID:
		if kind == 0 {
			f.HasArrays = true
		}
	case f.sequenceFID:
		if kind == 0 {
			if f.manager.sequencePlugin != nil && sort == f.manager.external(f.manager.sequencePlugin.stringSort) {
				f.HasString = true
			} else {
				f.HasNonStringSequence = true
			}
		}
	}
	return fid, nil
}

func (f *StaticFeatures) arraySortParameters(sort ASTHandle) ([]ASTHandle, error) {
	m := f.manager
	if err := m.lock(); err != nil {
		return nil, err
	}
	defer m.mu.Unlock()
	r, err := m.requireKind(sort, ASTSort)
	if err != nil {
		return nil, err
	}
	info, err := m.sortInfo(m.sortAt(r.slot).info)
	if err != nil {
		return nil, err
	}
	result := make([]ASTHandle, 0, info.count)
	for i := uint32(0); i < info.count; i++ {
		p, e := m.infoParameter(info, i)
		if e != nil {
			return nil, e
		}
		value, e := m.parameter(p)
		if e != nil {
			return nil, e
		}
		if value.Kind != ASTParameterAST || m.nodeAt(value.ast.slot).kind != ASTSort {
			return nil, ErrASTDomain
		}
		result = append(result, m.external(value.ast))
	}
	return result, nil
}

func (f *StaticFeatures) accountArraySortParameters(sort ASTHandle) error {
	parameters, err := f.arraySortParameters(sort)
	if err != nil {
		return err
	}
	for _, parameter := range parameters {
		fid, e := f.manager.GetFamilyID(parameter)
		if e != nil {
			return e
		}
		f.markTheory(fid)
		kind, e := f.manager.GetDeclKind(parameter)
		if e != nil {
			return e
		}
		switch fid {
		case f.arithFID:
			if kind == 0 {
				f.HasReal = true
			} else if kind == 1 {
				f.HasInt = true
			}
		case f.bitVectorFID:
			f.HasBitVector = f.HasBitVector || kind == 0
		case f.fpaFID:
			f.HasFloatingPoint = f.HasFloatingPoint || kind == 0 || kind == 1
		case f.arrayFID:
			f.HasArrays = f.HasArrays || kind == 0
		}
	}
	return nil
}

func (f *StaticFeatures) accountEquality(h ASTHandle, isEquality bool) error {
	if !isEquality {
		return nil
	}
	f.NumEqualities++
	lhs, err := f.manager.GetArg(h, 0)
	if err != nil {
		return err
	}
	sort, err := f.manager.GetSort(lhs)
	if err != nil {
		return err
	}
	fid, err := f.manager.GetFamilyID(sort)
	if err != nil {
		return err
	}
	if !staticFeatureUninterpretedSortFamily(fid) && fid != f.basicFID {
		staticFeatureIncrement(&f.NumTheoryEqualities, fid)
	}
	return nil
}

func (f *StaticFeatures) accountApplicationTheory(h, resultSort ASTHandle, fid, resultFID FamilyID, isBool, isEquality, gate bool, args uint32) error {
	f.markTheory(fid)
	if fid != NullFamilyID && fid != f.basicFID {
		f.NumInterpretedExprs++
		if isBool {
			staticFeatureIncrement(&f.NumTheoryAtoms, fid)
		} else {
			staticFeatureIncrement(&f.NumTheoryTerms, fid)
		}
		if args == 0 {
			f.NumInterpretedConstants++
		}
	}
	if fid == NullFamilyID && args == 0 && !staticFeatureUninterpretedSortFamily(resultFID) && resultFID != f.basicFID {
		staticFeatureIncrement(&f.NumTheoryConstants, resultFID)
	}
	if resultFID == f.arrayFID {
		if err := f.accountArraySortParameters(resultSort); err != nil {
			return err
		}
	}
	if isEquality || gate {
		return nil
	}
	for i := uint32(0); i < args; i++ {
		arg, err := f.manager.GetArg(h, i)
		if err != nil {
			return err
		}
		sort, err := f.manager.GetSort(arg)
		if err != nil {
			return err
		}
		argFID, err := f.manager.GetFamilyID(sort)
		if err != nil {
			return err
		}
		if !staticFeatureUninterpretedSortFamily(argFID) && argFID != fid {
			f.NumAliens++
			staticFeatureIncrement(&f.NumAliensPerFamily, argFID)
			if argFID == f.arithFID {
				f.NumArithTerms++
				if yes, e := f.isDiffTerm(arg); e != nil {
					return e
				} else if yes {
					f.NumDiffTerms++
					if e = f.accScratch(); e != nil {
						return e
					}
				}
			}
		}
	}
	return nil
}
