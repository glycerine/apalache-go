package ast

// Selected source surface and ownership adapters:
//
// util/obj_mark.h default expr specialization: constructor/default get_id,
// is_marked reference/pointer overloads -> ASTExprMark.IsMarked; mark(flag)
// and mark(true) overloads -> Mark(h,flag); reset -> Reset. Its complete used
// bit_vector subset is constructor/resize(false)/get/set/reset/destructor.
// The other retained source instantiation, obj_mark<decl, bit_vector,
// decl2uint>, is used by ast_mark and maps to ASTMark's declaration plane.
// No universal obj_mark template adapter is retained.
//
// ast.h public mark1/2(bool), reset_mark1/2, is_marked1/2 -> ASTManager's
// MarkNode1/2, ResetNodeMark1/2, IsNodeMarked1/2. Scalar uint64 owner identity
// replaces the debug void* comparison; source assertions execute only under
// g3debug. Shared-occurrence private mark/functor is a separate pending unit.
//
// ast_fast_mark<1/2>: default constructor, destructor(reset), is_marked,
// reset_mark, mark(true/flag), reset, get_level/set_level -> concrete
// ASTFastMark1/2 constructors, Close, IsMarked, ResetMark, Mark, Reset,
// GetLevel/SetLevel. ast_ref_fast_mark<1/2> maps the same declared methods
// excluding levels to ASTRefFastMark1/2. Their inline16/doubling buffers retain
// capacity on Reset; ref-buffer callback destruction is ASC, after ALL marks
// have been cleared. Failed mark-before-push prefixes are NOT silently reset
// or decremented; they remain source-visible after Reset/Close of listed data.
//
// ast.cpp ast_mark three methods -> ASTMark.IsMarked/Mark/Reset; declaration
// IDs use unsigned get_id()-c_first_decl_id, distinct from expression IDs.
// Marks deliberately survive node deletion and ID reuse. No references are
// retained by the ordinary ID map or nonref fast classes.
// scoped_mark is excluded: retained C++ sources have no consumers.
//
// These cold owners are caller-confined; manager mutations hold its mutex.
// Every actual header/inline array/backing is budgeted. Copies of Go owners
// are rejected by self identity, an explicit adapter for implicit owning C++
// copy/assignment; source implicit copies are not implemented by this packet.
// Checked scalar tokens replace raw borrowed pointers. Freed generations,
// foreign handles, and physically invalid unsigned-wrap continuations return
// typed errors after preserving completed source prefixes. A Close after
// forced manager teardown refunds only external storage, without callbacks
// against already-freed ASTs. Nonref stale-borrow Reset/Close reports a typed
// error and leaves the owner open until manager teardown permits physical
// cleanup. Consumed deletion errors use the core's owned emergency continuation.
//
// No registered mark-specific source test was found. The real bit_blaster
// caller remains integration-dependent on that plugin; the finite tests here
// are additional ownership, failure, ordering and independent model coverage.
// expr_sparse_mark's identity table and other general template families remain
// open; whole ast.h/ast.cpp completion is not claimed.
