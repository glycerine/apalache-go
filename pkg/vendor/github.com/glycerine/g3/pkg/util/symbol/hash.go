package symbol

// Source: src/util/hash.cpp; src/util/symbol.cpp:48-52.

import uhash "github.com/glycerine/g3/pkg/util/hash"

func spellingHash(s string, init uint32) uint32 { return uhash.String(s, init) }
