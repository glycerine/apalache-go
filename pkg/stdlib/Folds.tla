--------------------------- MODULE Folds -----------------------------------
(**
 * Apalache-compatible replacement for the CommunityModules Folds module.
 * The model checker does not execute MapThenFoldSet, but the declaration and
 * annotation are required for parsing and type-checking specifications that
 * use the community API.
 *)

EXTENDS __apalache_internal

(**
 * Starting from base, apply op to f(x), for all x \in S, choosing set
 * elements with choose.
 *
 * @type: ((a, b) => b, b, c => a, Set(c) => c, Set(c)) => b;
 *)
MapThenFoldSet(__op(_, _), __base, __f(_), __choose(_), __S) ==
  __NotSupportedByModelChecker("MapThenFoldSet. Use FoldSet, FoldSeq, FoldFunction.")

=============================================================================
