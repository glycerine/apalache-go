package ast

// Source: src/ast/static_features.cpp:153-163,380-536 and the Boolean,
// quantifier, uninterpreted portions of update_core at lines 163-365.

func (f *StaticFeatures) exprID(h ASTHandle) (uint32, error) { return f.manager.GetASTID(h) }

func (f *StaticFeatures) depth(h ASTHandle) uint32 {
	id, err := f.exprID(h)
	if err != nil || f.exprDepth[id] == 0 {
		return 1
	}
	return f.exprDepth[id]
}
func (f *StaticFeatures) setDepth(h ASTHandle, d uint32) error {
	id, err := f.exprID(h)
	if err != nil {
		return err
	}
	f.exprDepth[id] = d
	return nil
}
func (f *StaticFeatures) mapDepth(values map[uint32]uint32, h ASTHandle) uint32 {
	id, _ := f.exprID(h)
	return values[id]
}
func (f *StaticFeatures) setMapDepth(values map[uint32]uint32, h ASTHandle, d uint32) error {
	id, err := f.exprID(h)
	if err == nil {
		values[id] = d
	}
	return err
}

func (f *StaticFeatures) basicKind(h ASTHandle) (int32, bool, error) {
	if k, err := f.manager.GetKind(h); err != nil || k != ASTApp {
		return 0, false, err
	}
	fid, err := f.manager.GetFamilyID(h)
	if err != nil || fid != f.basicFID {
		return 0, false, err
	}
	op, err := f.manager.GetDeclKind(h)
	return op, err == nil, err
}

func (f *StaticFeatures) isGate(h ASTHandle) (bool, error) {
	op, yes, err := f.basicKind(h)
	if err != nil || !yes {
		return false, err
	}
	switch op {
	case ASTOpIte, ASTOpAnd, ASTOpOr, ASTOpIff, ASTOpXor, ASTOpImplies:
		return true, nil
	}
	return false, nil
}

func (f *StaticFeatures) isBool(h ASTHandle) (bool, error) {
	s, err := f.manager.GetSort(h)
	if err != nil {
		return false, err
	}
	b, err := f.manager.MkBoolSort()
	return s == b, err
}

func (f *StaticFeatures) updateCore(h ASTHandle) error {
	f.NumExprs++
	kind, err := f.manager.GetKind(h)
	if err != nil {
		return err
	}
	sort, err := f.manager.GetSort(h)
	if err != nil {
		return err
	}
	resultFID, err := f.classifySort(sort)
	if err != nil {
		return err
	}
	gate, err := f.isGate(h)
	if err != nil {
		return err
	}
	isBool, err := f.isBool(h)
	if err != nil {
		return err
	}
	if gate {
		f.CNF = false
		f.NumNestedFormulas++
		op, _, _ := f.basicKind(h)
		switch op {
		case ASTOpIte:
			if isBool {
				f.NumITEFormulas++
			} else {
				f.NumITETerms++
				if err = f.accountTermITE(h); err != nil {
					return err
				}
			}
		case ASTOpAnd:
			f.NumANDs++
		case ASTOpOr:
			f.NumORs++
		case ASTOpIff:
			f.NumIFFs++
		}
	}
	if isBool {
		f.NumBoolExprs++
		if kind == ASTApp {
			n, e := f.manager.GetNumArgs(h)
			if e != nil {
				return e
			}
			if n == 0 {
				f.NumBoolConstants++
			}
		}
	}
	isEquality := false
	if op, yes, e := f.basicKind(h); e != nil {
		return e
	} else if yes {
		isEquality = op == ASTOpEq
	}
	if err = f.accountEquality(h, isEquality); err != nil {
		return err
	}
	if err = f.accountArithmetic(h, isEquality); err != nil {
		return err
	}
	if kind == ASTQuantifier {
		f.NumQuantifiers++
		n, e := f.manager.GetNumPatterns(h)
		if e != nil {
			return e
		}
		if n > 0 {
			f.NumQuantifiersWithPatterns++
		}
		for i := uint32(0); i < n; i++ {
			p, e := f.manager.GetPattern(h, i)
			if e != nil {
				return e
			}
			if k, e := f.manager.GetKind(p); e != nil {
				return e
			} else if k == ASTApp {
				args, e := f.manager.GetNumArgs(p)
				if e != nil {
					return e
				}
				if args > 1 {
					f.NumQuantifiersWithMultiPatterns++
					break
				}
			}
		}
		return nil
	}
	if kind != ASTApp {
		return nil
	}
	fid, err := f.manager.GetFamilyID(h)
	if err != nil {
		return err
	}
	decl, err := f.manager.GetDecl(h)
	if err != nil {
		return err
	}
	declID, err := f.manager.GetASTID(decl)
	if err != nil {
		return err
	}
	f.NumApps[declID]++
	args, err := f.manager.GetNumArgs(h)
	if err != nil {
		return err
	}
	if fid == NullFamilyID {
		f.NumUninterpretedExprs++
		if args == 0 {
			f.NumUninterpretedConstants++
		}
		arity, e := f.manager.GetArity(decl)
		if e != nil {
			return e
		}
		if arity > 0 {
			if _, marked := f.visited[decl]; !marked {
				f.visited[decl] = struct{}{}
				f.NumUninterpretedFunctions++
			}
		}
	}
	return f.accountApplicationTheory(h, sort, fid, resultFID, isBool, isEquality, gate, args)
}

func (f *StaticFeatures) process(h ASTHandle, formContext, orAndContext, iteContext bool, stackDepth uint32) error {
	kind, err := f.manager.GetKind(h)
	if err != nil {
		return err
	}
	if kind == ASTVar {
		return nil
	}
	if _, marked := f.visited[h]; marked {
		f.NumSharing++
		return nil
	}
	if stackDepth > f.MaxStackDepth {
		return nil
	}
	f.visited[h] = struct{}{}
	if err = f.updateCore(h); err != nil {
		return err
	}
	if kind == ASTQuantifier {
		body, e := f.manager.GetQuantifierBody(h)
		if e != nil {
			return e
		}
		if e = f.process(body, false, false, false, stackDepth+1); e != nil {
			return e
		}
		return f.setDepth(h, f.depth(body)+1)
	}
	if kind != ASTApp {
		return ErrASTDomain
	}
	formNew, orAndNew, iteNew := false, false, false
	if op, yes, e := f.basicKind(h); e != nil {
		return e
	} else if yes {
		switch op {
		case ASTOpIte:
			formNew, err = f.isBool(h)
			if err != nil {
				return err
			}
			iteNew = true
		case ASTOpAnd, ASTOpOr:
			formNew = true
			orAndNew = true
		case ASTOpIff:
			formNew = true
		}
	}
	var depth, formDepth, orAndDepth, iteDepth uint32
	n, err := f.manager.GetNumArgs(h)
	if err != nil {
		return err
	}
	for i := uint32(0); i < n; i++ {
		arg, e := f.manager.GetArg(h, i)
		if e != nil {
			return e
		}
		if yes, child, e := f.manager.IsNot(arg); e != nil {
			return e
		} else if yes {
			arg = child
		}
		if e = f.process(arg, formNew, orAndNew, iteNew, stackDepth+1); e != nil {
			return e
		}
		if d := f.depth(arg); d > depth {
			depth = d
		}
		if formNew {
			if d := f.mapDepth(f.formulaDepth, arg); d > formDepth {
				formDepth = d
			}
		}
		if orAndNew {
			if d := f.mapDepth(f.orAndDepth, arg); d > orAndDepth {
				orAndDepth = d
			}
		}
		if iteNew {
			if d := f.mapDepth(f.iteDepth, arg); d > iteDepth {
				iteDepth = d
			}
		}
	}
	depth++
	if err = f.setDepth(h, depth); err != nil {
		return err
	}
	if depth > f.MaxDepth {
		f.MaxDepth = depth
	}
	if formNew {
		formDepth++
		if !formContext {
			f.NumFormulaTrees++
			f.SumFormulaDepth += formDepth
			if formDepth > f.MaxFormulaDepth {
				f.MaxFormulaDepth = formDepth
			}
		}
		if err = f.setMapDepth(f.formulaDepth, h, formDepth); err != nil {
			return err
		}
	}
	if orAndNew {
		orAndDepth++
		if !orAndContext {
			f.NumOrAndTrees++
			f.SumOrAndTreeDepth += orAndDepth
			if orAndDepth > f.MaxOrAndTreeDepth {
				f.MaxOrAndTreeDepth = orAndDepth
			}
		}
		if err = f.setMapDepth(f.orAndDepth, h, orAndDepth); err != nil {
			return err
		}
	}
	if iteNew {
		iteDepth++
		if !iteContext {
			f.NumITETrees++
			f.SumITETreeDepth += iteDepth
			if iteDepth >= f.MaxITETreeDepth {
				f.MaxITETreeDepth = iteDepth
			}
		}
		if err = f.setMapDepth(f.iteDepth, h, iteDepth); err != nil {
			return err
		}
	}
	return nil
}

func (f *StaticFeatures) processRoot(h ASTHandle) error {
	if _, marked := f.visited[h]; marked {
		f.NumSharing++
		return nil
	}
	f.NumRoots++
	if op, yes, err := f.basicKind(h); err != nil {
		return err
	} else if yes && op == ASTOpOr {
		f.visited[h] = struct{}{}
		f.NumClauses++
		f.NumBoolExprs++
		n, e := f.manager.GetNumArgs(h)
		if e != nil {
			return e
		}
		f.SumClauseSize += n
		if n == 2 {
			f.NumBinClauses++
		}
		var depth, formDepth, orDepth uint32
		for i := uint32(0); i < n; i++ {
			arg, e := f.manager.GetArg(h, i)
			if e != nil {
				return e
			}
			if yes, child, e := f.manager.IsNot(arg); e != nil {
				return e
			} else if yes {
				arg = child
			}
			if e = f.process(arg, true, true, false, 0); e != nil {
				return e
			}
			if d := f.depth(arg); d > depth {
				depth = d
			}
			if d := f.mapDepth(f.formulaDepth, arg); d > formDepth {
				formDepth = d
			}
			if d := f.mapDepth(f.orAndDepth, arg); d > orDepth {
				orDepth = d
			}
		}
		depth++
		if err = f.setDepth(h, depth); err != nil {
			return err
		}
		if depth > f.MaxDepth {
			f.MaxDepth = depth
		}
		formDepth++
		f.NumFormulaTrees++
		f.SumFormulaDepth += formDepth
		if formDepth > f.MaxFormulaDepth {
			f.MaxFormulaDepth = formDepth
		}
		if err = f.setMapDepth(f.formulaDepth, h, formDepth); err != nil {
			return err
		}
		orDepth++
		f.NumOrAndTrees++
		f.SumOrAndTreeDepth += orDepth
		if orDepth > f.MaxOrAndTreeDepth {
			f.MaxOrAndTreeDepth = orDepth
		}
		return f.setMapDepth(f.orAndDepth, h, orDepth)
	}
	gate, err := f.isGate(h)
	if err != nil {
		return err
	}
	if !gate {
		f.SumClauseSize++
		f.NumUnits++
		f.NumClauses++
	}
	return f.process(h, false, false, false, 0)
}

func (f *StaticFeatures) Collect(formulas ...ASTHandle) error {
	if err := f.open(); err != nil {
		return err
	}
	for _, h := range formulas {
		if err := f.processRoot(h); err != nil {
			return err
		}
	}
	return nil
}
