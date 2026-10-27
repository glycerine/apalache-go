package apalache

import "fmt"

// SourcePosition is a lexicographically ordered line/column pair.
type SourcePosition struct {
	Line   int
	Column int
}

func (p SourcePosition) String() string { return fmt.Sprintf("%d:%d", p.Line, p.Column) }
func (p SourcePosition) Compare(other SourcePosition) int {
	switch {
	case p.Line < other.Line || p.Line == other.Line && p.Column < other.Column:
		return -1
	case p.Line > other.Line || p.Line == other.Line && p.Column > other.Column:
		return 1
	default:
		return 0
	}
}

// SourceRegion has inclusive boundaries.
type SourceRegion struct {
	Start SourcePosition
	End   SourcePosition
}

func NewSourceRegion(startLine, startColumn, endLine, endColumn int) SourceRegion {
	return SourceRegion{
		Start: SourcePosition{Line: startLine, Column: startColumn},
		End:   SourcePosition{Line: endLine, Column: endColumn},
	}
}

func (r SourceRegion) String() string { return r.Start.String() + "-" + r.End.String() }
func (r SourceRegion) IsInside(larger SourceRegion) bool {
	return r.Start.Compare(larger.Start) >= 0 && r.End.Compare(larger.End) <= 0
}
func (r SourceRegion) Contains(smaller SourceRegion) bool { return smaller.IsInside(r) }
func (r SourceRegion) Intersects(other SourceRegion) bool {
	return r.Start.Compare(other.End) <= 0 && other.Start.Compare(r.End) <= 0
}

// SourceLocation identifies a region within a TLA+ module. Filename follows
// the Scala IR convention and is stored without the .tla suffix.
type SourceLocation struct {
	Filename string
	Region   SourceRegion
}

func (location SourceLocation) String() string {
	return location.Filename + ".tla:" + location.Region.String()
}
