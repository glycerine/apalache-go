package text

// Native numeral output boundary for retained numeric stream consumers.

func (out *NumericTextStream) AppendNumeralText(text string) error {
	return out.AppendCString(text)
}
