package apalache

import (
	"path/filepath"
	"strings"

	"github.com/glycerine/tlago"
)

// Retain declaration origin across EXTENDS and INSTANCE flattening. A root
// declaration wins over an unnamed instance, just as it does in tlago's export.
func tlagoDefinitionOrigins(spec *tlago.Spec) map[string]string {
	result := make(map[string]string)
	if spec == nil || spec.Root == nil {
		return result
	}
	var visit func(*tlago.Module, string, map[string]bool)
	visit = func(module *tlago.Module, prefix string, active map[string]bool) {
		if module == nil || active[module.Name] {
			return
		}
		active = cloneMap(active)
		active[module.Name] = true
		for _, parent := range module.Extends {
			visit(spec.Modules[parent], prefix, active)
		}
		for _, instance := range module.Instances {
			nested := prefix
			if instance.Name != "" {
				nested += instance.Name + "!"
			}
			visit(spec.Modules[instance.Module], nested, active)
		}
		for _, definition := range module.Definitions {
			owner := strings.TrimSuffix(filepath.Base(definition.SourcePosition().File), ".tla")
			if owner == "" || owner == "." {
				owner = module.Name
			}
			result[prefix+definition.Name] = owner
		}
	}
	visit(spec.Root, "", map[string]bool{})
	return result
}

func tlagoWithoutNames(scope map[string]string, names []string) map[string]string {
	result := cloneMap(scope)
	for _, name := range names {
		result[name] = "local"
	}
	return result
}

// Unlike a context-free bottom-up rewrite, this walk excludes local operators
// and formal parameters from the imported builtin namespace.
func rewriteTlagoResolvedExpressions(module *Module, sources *SourceStore, origins map[string]string, tracker TransformationTracker, rule func(Expr, map[string]string) (Expr, bool)) *Module {
	var rewrite func(Expr, map[string]string) Expr
	var declaration func(*OperDecl, map[string]string) *OperDecl
	declaration = func(d *OperDecl, scope map[string]string) *OperDecl {
		scope = cloneMap(scope)
		for _, p := range d.FormalParams() {
			scope[p.Name] = "local"
		}
		copy := d.copyWithTag(d.Tag())
		copy.SetBody(rewrite(d.Body(), scope))
		return tracker.TrackOperatorDeclaration(func(*OperDecl) *OperDecl { return copy })(d)
	}
	rewrite = func(e Expr, scope map[string]string) Expr {
		original := e
		switch x := e.(type) {
		case *OperExpr:
			args := x.Arguments()
			for i, a := range args {
				args[i] = rewrite(a, scope)
			}
			e = MustOperExpr(x.Operator(), x.Tag(), args...)
		case *LetInExpr:
			scope = cloneMap(scope)
			ds := x.Declarations()
			for _, d := range ds {
				scope[d.Name()] = "local"
			}
			for i, d := range ds {
				ds[i] = declaration(d, scope)
			}
			e = NewLetInExpr(rewrite(x.Body(), scope), x.Tag(), ds...)
		}
		if replacement, ok := rule(e, scope); ok {
			e = replacement
		}
		return tracker.TrackExpression(func(Expr) Expr { return e })(original)
	}
	ds := module.Declarations()
	for i, d := range ds {
		switch x := d.(type) {
		case *OperDecl:
			ds[i] = declaration(x, origins)
		case *AssumeDecl:
			name, named := x.DefinedName()
			if named {
				ds[i] = NewNamedAssumeDecl(name, rewrite(x.Body(), origins), x.Tag())
			} else {
				ds[i] = NewUnnamedAssumeDecl(rewrite(x.Body(), origins), x.Tag())
			}
		}
	}
	result := NewModule(module.Name(), ds)
	result.copyMetadataFrom(module)
	if sources != nil {
		for i, d := range module.Declarations() {
			sources.OnDeclarationTransformation(d, ds[i])
		}
	}
	return result
}
