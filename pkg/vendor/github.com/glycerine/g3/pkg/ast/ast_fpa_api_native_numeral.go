package ast

// Source: src/api/api_fpa.cpp:284-339. These constructors retain the source
// scoped_mpf lifetime and publish the resulting manager-owned numeral only
// after native-value conversion succeeds.

type fpaNativeNumeralKind uint8

const (
	fpaNativeFloat32 fpaNativeNumeralKind = iota
	fpaNativeFloat64
	fpaNativeInt32
)

func (m *ASTManager) mkFPANativeNumeral(sort ASTHandle, kind fpaNativeNumeralKind, floatValue float64, intValue int32) (out ASTHandle, result error) {
	if result = m.lock(); result != nil {
		return out, result
	}
	defer m.mu.Unlock()
	sortRef, err := m.requireKind(sort, ASTSort)
	if err != nil {
		return out, err
	}
	ebits, sbits, matched, err := m.floatingPointSortShape(sortRef)
	if err != nil {
		return out, err
	}
	if !matched {
		return out, ErrASTDomain
	}
	plugin := m.floatingPointPlugin
	if plugin == nil || plugin.sourceEnded {
		return out, ErrASTHandle
	}
	frame, err := plugin.beginNumeralProbe()
	if err != nil {
		return out, err
	}
	var node astNodeRef
	defer func() {
		plugin.finishNumeralConstruction(frame, &result, &node)
		if result == nil {
			out = m.external(node)
		}
	}()
	switch kind {
	case fpaNativeFloat32:
		err = plugin.numeric.SetFloat(&frame.value, ebits, sbits, float32(floatValue))
	case fpaNativeFloat64:
		err = plugin.numeric.SetDouble(&frame.value, ebits, sbits, floatValue)
	case fpaNativeInt32:
		err = plugin.numeric.SetInt32(&frame.value, ebits, sbits, intValue)
	default:
		return out, ErrASTDomain
	}
	if err != nil {
		return out, err
	}
	node, result = plugin.mkNumeral(&frame.value, nil)
	return out, result
}

func (m *ASTManager) MkFPANumeralFloat32(value float32, sort ASTHandle) (ASTHandle, error) {
	return m.mkFPANativeNumeral(sort, fpaNativeFloat32, float64(value), 0)
}

func (m *ASTManager) MkFPANumeralFloat64(value float64, sort ASTHandle) (ASTHandle, error) {
	return m.mkFPANativeNumeral(sort, fpaNativeFloat64, value, 0)
}

func (m *ASTManager) MkFPANumeralInt32(value int32, sort ASTHandle) (ASTHandle, error) {
	return m.mkFPANativeNumeral(sort, fpaNativeInt32, 0, value)
}
