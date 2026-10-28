package apalache

// TextLayout controls the width and indentation used by textual renderers.
type TextLayout struct {
	TextWidth int
	Indent    int
}

func DefaultTextLayout() TextLayout {
	return TextLayout{TextWidth: 80, Indent: 2}
}
