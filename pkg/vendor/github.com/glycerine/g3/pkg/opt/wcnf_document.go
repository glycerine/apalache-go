package opt

// Source: src/opt/opt_parse.cpp class wcnf.

type WCNFLiteral struct {
	Variable uint32
	Negative bool
}

type WCNFClause struct {
	Weight   uint64
	Literals []WCNFLiteral
}

type WCNFDocument struct {
	NumVariables uint32
	NumClauses   uint32
	TopWeight    uint64
	Clauses      []WCNFClause
}

func (d WCNFDocument) Copy() WCNFDocument {
	r := d
	r.Clauses = make([]WCNFClause, len(d.Clauses))
	for i := range d.Clauses {
		r.Clauses[i].Weight = d.Clauses[i].Weight
		r.Clauses[i].Literals = append([]WCNFLiteral(nil), d.Clauses[i].Literals...)
	}
	return r
}
