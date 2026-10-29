package api

// Source: src/api/api_context.cpp:408-419; src/api/z3_api.h:5295-5305.

import "github.com/glycerine/g3/pkg/util/diagnostic"

// EnableTrace follows the public source entry: initialize the generated
// process, intern the C-string tag as a symbol, then enable its bare spelling.
func EnableTrace(tag string) error {
	g, err := globalParamService()
	if err != nil {
		return err
	}
	symbols := g.Context.Symbols()
	s, err := symbols.Intern(tag)
	if err != nil {
		return err
	}
	spelling, present, err := symbols.BareString(s)
	if err != nil {
		return err
	}
	if !present {
		return nil
	}
	diagnostic.BorrowProcessTraceTags().Enable(spelling)
	return nil
}

// DisableTrace removes a tag from the same process trace owner.
func DisableTrace(tag string) {
	diagnostic.BorrowProcessTraceTags().Disable(tag)
}
