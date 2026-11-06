package ast

// Source: src/ast/fpa_decl_plugin.cpp:786-854.

import usymbol "github.com/glycerine/g3/pkg/util/symbol"

func (m *ASTManager) GetFloatingPointOpNames(names *ASTBuiltinNames, _ usymbol.Symbol) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	if names == nil || m.floatingPointPlugin == nil || !m.floatingPointPlugin.published {
		return ErrASTHandle
	}
	entries := [...]struct {
		name string
		kind int32
	}{
		{"+oo", astFPAPlusInf}, {"-oo", astFPAMinusInf}, {"+zero", astFPAPlusZero}, {"-zero", astFPAMinusZero}, {"NaN", astFPANaN},
		{"roundNearestTiesToEven", astFPARMNearestEven}, {"roundNearestTiesToAway", astFPARMNearestAway},
		{"roundTowardPositive", astFPARMTowardPositive}, {"roundTowardNegative", astFPARMTowardNegative}, {"roundTowardZero", astFPARMTowardZero},
		{"RNE", astFPARMNearestEven}, {"RNA", astFPARMNearestAway}, {"RTP", astFPARMTowardPositive}, {"RTN", astFPARMTowardNegative}, {"RTZ", astFPARMTowardZero},
		{"fp.abs", astFPAAbs}, {"fp.neg", astFPANeg}, {"fp.add", astFPAAdd}, {"fp.sub", astFPASub}, {"fp.mul", astFPAMul}, {"fp.div", astFPADiv},
		{"fp.fma", astFPAFMA}, {"fp.sqrt", astFPASqrt}, {"fp.rem", astFPARem}, {"fp.roundToIntegral", astFPARoundToIntegral},
		{"fp.min", astFPAMin}, {"fp.max", astFPAMax}, {"fp.leq", astFPALe}, {"fp.lt", astFPALt}, {"fp.geq", astFPAGe}, {"fp.gt", astFPAGt}, {"fp.eq", astFPAEq},
		{"fp.isNormal", astFPAIsNormal}, {"fp.isSubnormal", astFPAIsSubnormal}, {"fp.isZero", astFPAIsZero}, {"fp.isInfinite", astFPAIsInf},
		{"fp.isNaN", astFPAIsNaN}, {"fp.isNegative", astFPAIsNegative}, {"fp.isPositive", astFPAIsPositive},
		{"fp", astFPAFP}, {"fp.to_ubv", astFPAToUBV}, {"fp.to_sbv", astFPAToSBV}, {"fp.to_real", astFPAToReal},
		{"to_fp", astFPAToFP}, {"to_fp_unsigned", astFPAToFPUnsigned}, {"to_ieee_bv", astFPAToIEEEBV}, {"fp.to_ieee_bv", astFPAToIEEEBV},
	}
	for _, entry := range entries {
		if e := m.appendBuiltinName(names, entry.name, entry.kind); e != nil {
			return e
		}
	}
	return nil
}

func (m *ASTManager) GetFloatingPointSortNames(names *ASTBuiltinNames, _ usymbol.Symbol) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	if names == nil || m.floatingPointPlugin == nil || !m.floatingPointPlugin.published {
		return ErrASTHandle
	}
	for _, entry := range [...]struct {
		name string
		kind int32
	}{{"FloatingPoint", 0}, {"RoundingMode", 1}, {"Float16", 2}, {"Float32", 3}, {"Float64", 4}, {"Float128", 5}} {
		if e := m.appendBuiltinName(names, entry.name, entry.kind); e != nil {
			return e
		}
	}
	return nil
}
