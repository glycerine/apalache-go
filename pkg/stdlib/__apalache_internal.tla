------------------- MODULE __apalache_internal -------------------------------
\* @type: Seq(a) => Int;
__ApalacheSeqCapacity(seq) == 0

\* @type: Str => a;
__NotSupportedByModelChecker(message) == CHOOSE x \in {}: TRUE
===============================================================================
