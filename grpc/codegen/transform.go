package codegen

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"goa.design/goa/codegen"
	"goa.design/goa/design"
)

var (
	transformArrayT *template.Template
	transformMapT   *template.Template
)

type (
	// too many args...

	targs struct {
		sourceVar, targetVar string
		sourcePkg, targetPkg string
		unmarshal            bool
		scope                *codegen.NameScope
	}

	thargs struct {
		sourcePkg, targetPkg string
		unmarshal            bool
		scope                *codegen.NameScope
	}
)

// NOTE: can't initialize inline because https://github.com/golang/go/issues/1817
func init() {
	funcMap := template.FuncMap{"transformAttribute": transformAttributeHelper}
	transformArrayT = template.Must(template.New("transformArray").Funcs(funcMap).Parse(transformArrayTmpl))
	transformMapT = template.Must(template.New("transformMap").Funcs(funcMap).Parse(transformMapTmpl))
}

// ProtoBufTypeTransform produces Go code that initializes the data structure
// defined by target from an instance of the data structure described the
// source. Either the source or target is a type referring to the protocol
// buffer message type. The algorithm matches object fields by name and ignores
// object fields in target that don't have a match in source. The matching and
// generated code leverage mapped attributes so that attribute names may use
// the "name:elem" syntax to define the name of the design attribute and the
// name of the corresponding generated Go struct field. The function returns
// an error if target is not compatible with source (different type, fields of
// different type etc).
//
// sourceVar and targetVar contain the name of the variables that hold the
// source and target data structures respectively.
//
// sourcePkg and targetPkg contain the name of the Go package that defines the
// source or target type respectively in case it's not the same package as where
// the generated code lives.
//
// proto if true indicates whether the code is being generated to initialize
// a Go struct generated from the protocol buffer message type, otherwise to
// initialize a type from a Go struct generated from the protocol buffer message
// type.
//
//   - proto3 syntax is used to refer to a protocol buffer generated Go struct.
//
// scope is used to compute the name of the user types when initializing fields
// that use them.
//
func ProtoBufTypeTransform(source, target design.DataType, sourceVar, targetVar, sourcePkg, targetPkg string, proto bool, scope *codegen.NameScope) (string, []*codegen.TransformFunctionData, error) {
	var (
		satt = &design.AttributeExpr{Type: source}
		tatt = &design.AttributeExpr{Type: target}
	)

	a := targs{sourceVar, targetVar, sourcePkg, targetPkg, proto, scope}
	code, err := transformAttribute(satt, tatt, true, a)
	if err != nil {
		return "", nil, err
	}

	b := thargs{sourcePkg, targetPkg, proto, scope}
	funcs, err := transformAttributeHelpers(source, target, b)
	if err != nil {
		return "", nil, err
	}

	return strings.TrimRight(code, "\n"), funcs, nil
}

// transformAttribute converts source attribute expression to target returning
// the conversion code and error (if any). Either source or target is a
// protocol buffer message type.
func transformAttribute(source, target *design.AttributeExpr, newVar bool, a targs) (string, error) {
	var (
		code string
		err  error
	)
	switch {
	case design.IsArray(source.Type):
		code, err = transformArray(design.AsArray(source.Type), design.AsArray(target.Type), newVar, a)
	case design.IsMap(source.Type):
		code, err = transformMap(design.AsMap(source.Type), design.AsMap(target.Type), newVar, a)
	case design.IsObject(source.Type):
		if code, err = transformObject(source, target, newVar, a); err != nil {
			return "", err
		}
	default:
		assign := "="
		if newVar {
			assign = ":="
		}
		code = fmt.Sprintf("%s %s %s\n", a.targetVar, assign, typeCast(a.sourceVar, source.Type, target.Type, a.unmarshal))
	}
	return code, nil
}

func transformObject(source, target *design.AttributeExpr, newVar bool, a targs) (string, error) {
	var (
		initCode     string
		postInitCode string

		buffer = &bytes.Buffer{}
	)
	{
		walkMatches(source, target, func(src, tgt *design.MappedAttributeExpr, srcAtt, tgtAtt *design.AttributeExpr, n string) {
			if !design.IsPrimitive(srcAtt.Type) {
				return
			}
			var srcPtr, tgtPtr bool
			{
				if a.unmarshal {
					srcPtr = source.IsPrimitivePointer(n, true)
				} else {
					tgtPtr = target.IsPrimitivePointer(n, true)
				}
			}
			deref := ""
			srcField := a.sourceVar + "." + codegen.Goify(src.ElemName(n), true)
			if srcPtr && !tgtPtr {
				if !source.IsRequired(n) {
					postInitCode += fmt.Sprintf("if %s != nil {\n\t%s.%s = %s\n}\n",
						srcField, a.targetVar, codegen.Goify(tgt.ElemName(n), true), "*"+srcField)
					return
				}
				deref = "*"
			} else if !srcPtr && tgtPtr {
				deref = "&"
			}
			initCode += fmt.Sprintf("\n%s: %s%s,", codegen.Goify(tgt.ElemName(n), true), deref, typeCast(srcField, srcAtt.Type, tgtAtt.Type, a.unmarshal))
		})
	}
	if initCode != "" {
		initCode += "\n"
	}
	assign := "="
	if newVar {
		assign = ":="
	}
	deref := "&"
	// if the target is a raw struct no need to return a pointer
	if _, ok := target.Type.(*design.Object); ok {
		deref = ""
	}
	buffer.WriteString(fmt.Sprintf("%s %s %s%s{%s}\n", a.targetVar, assign, deref,
		a.scope.GoFullTypeName(target, a.targetPkg), initCode))
	buffer.WriteString(postInitCode)

	var err error
	{
		walkMatches(source, target, func(src, tgt *design.MappedAttributeExpr, srcAtt, tgtAtt *design.AttributeExpr, n string) {
			b := a
			b.sourceVar = a.sourceVar + "." + codegen.GoifyAtt(srcAtt, src.ElemName(n), true)
			b.targetVar = a.targetVar + "." + codegen.GoifyAtt(tgtAtt, tgt.ElemName(n), true)
			err = isCompatible(srcAtt.Type, tgtAtt.Type, b.sourceVar, b.targetVar)
			if err != nil {
				return
			}

			var (
				code string
			)
			{
				_, ok := srcAtt.Type.(design.UserType)
				switch {
				case design.IsArray(srcAtt.Type):
					code, err = transformArray(design.AsArray(srcAtt.Type), design.AsArray(tgtAtt.Type), false, b)
				case design.IsMap(srcAtt.Type):
					code, err = transformMap(design.AsMap(srcAtt.Type), design.AsMap(tgtAtt.Type), false, b)
				case ok:
					code = fmt.Sprintf("%s = %s(%s)\n", b.targetVar, transformHelperName(srcAtt, tgtAtt, b), b.sourceVar)
				case design.IsObject(srcAtt.Type):
					code, err = transformAttribute(srcAtt, tgtAtt, false, b)
				}
				if err != nil {
					return
				}

				// Nil check handling.
				//
				// We need to check for a nil source if it holds a reference
				// (pointer to primitive or an object, array or map) and is not
				// required. We also want to always check when unmarshaling is
				// the attribute type is not a primitive: either it's a user
				// type and we want to avoid calling transform helper functions
				// with nil value (if unmarshaling then requiredness has been
				// validated) or it's an object, map or array and we need to
				// check for nil to avoid making empty arrays and maps and to
				// avoid derefencing nil.
				var checkNil bool
				{
					checkNil = !design.IsPrimitive(srcAtt.Type) && !src.IsRequired(n) || src.IsPrimitivePointer(n, true) && !a.unmarshal
				}
				if code != "" && checkNil {
					code = fmt.Sprintf("if %s != nil {\n\t%s}\n", b.sourceVar, code)
				}

				// Default value handling.
				//
				// There are 2 cases: one when generating marshaler code
				// (a.unmarshal is false) and the other when generating
				// unmarshaler code (a.unmarshal is true).
				//
				// When generating marshaler code we want to be lax and not
				// assume that required fields are set in case they have a
				// default value, instead the generated code is going to set the
				// fields to their default value (only applies to non-primitive
				// attributes).
				//
				// When generating unmarshaler code we rely on validations
				// running prior to this code so assume required fields are set.
				/*if tgt.HasDefaultValue(n) {
				  if b.unmarshal {
				    code += fmt.Sprintf("if %s == nil {\n\t", b.sourceVar)
				    if tgt.IsPrimitivePointer(n, true) {
				      code += fmt.Sprintf("var tmp %s = %#v\n\t%s = &tmp\n", GoNativeTypeName(tgtAtt.Type), tgtAtt.DefaultValue, b.targetVar)
				    } else {
				      code += fmt.Sprintf("%s = %#v\n", b.targetVar, tgtAtt.DefaultValue)
				    }
				    code += "}\n"
				  } else if src.IsPrimitivePointer(n, true) || !design.IsPrimitive(srcAtt.Type) {
				    code += fmt.Sprintf("if %s == nil {\n\t", b.sourceVar)
				    if tgt.IsPrimitivePointer(n, true) {
				      code += fmt.Sprintf("var tmp %s = %#v\n\t%s = &tmp\n", GoNativeTypeName(tgtAtt.Type), tgtAtt.DefaultValue, b.targetVar)
				    } else {
				      code += fmt.Sprintf("%s = %#v\n", b.targetVar, tgtAtt.DefaultValue)
				    }
				    code += "}\n"
				  }
				}*/
			}
			buffer.WriteString(code)
		})
	}
	if err != nil {
		return "", err
	}
	return buffer.String(), nil
}

func transformArray(source, target *design.Array, newVar bool, a targs) (string, error) {
	if err := isCompatible(source.ElemType.Type, target.ElemType.Type, a.sourceVar+"[0]", a.targetVar+"[0]"); err != nil {
		return "", err
	}
	data := map[string]interface{}{
		"Source":      a.sourceVar,
		"Target":      a.targetVar,
		"NewVar":      newVar,
		"ElemTypeRef": a.scope.GoFullTypeRef(target.ElemType, a.targetPkg),
		"SourceElem":  source.ElemType,
		"TargetElem":  target.ElemType,
		"SourcePkg":   a.sourcePkg,
		"TargetPkg":   a.targetPkg,
		"Unmarshal":   a.unmarshal,
		"Scope":       a.scope,
		"LoopVar":     string(105 + strings.Count(a.targetVar, "[")),
	}
	var buf bytes.Buffer
	if err := transformArrayT.Execute(&buf, data); err != nil {
		panic(err) // bug
	}
	code := buf.String()

	return code, nil
}

func transformMap(source, target *design.Map, newVar bool, a targs) (string, error) {
	if err := isCompatible(source.KeyType.Type, target.KeyType.Type, a.sourceVar+".key", a.targetVar+".key"); err != nil {
		return "", err
	}
	if err := isCompatible(source.ElemType.Type, target.ElemType.Type, a.sourceVar+"[*]", a.targetVar+"[*]"); err != nil {
		return "", err
	}
	data := map[string]interface{}{
		"Source":      a.sourceVar,
		"Target":      a.targetVar,
		"NewVar":      newVar,
		"KeyTypeRef":  a.scope.GoFullTypeRef(target.KeyType, a.targetPkg),
		"ElemTypeRef": a.scope.GoFullTypeRef(target.ElemType, a.targetPkg),
		"SourceKey":   source.KeyType,
		"TargetKey":   target.KeyType,
		"SourceElem":  source.ElemType,
		"TargetElem":  target.ElemType,
		"SourcePkg":   a.sourcePkg,
		"TargetPkg":   a.targetPkg,
		"Unmarshal":   a.unmarshal,
		"Scope":       a.scope,
		"LoopVar":     "",
	}
	if depth := mapDepth(target); depth > 0 {
		data["LoopVar"] = string(97 + depth)
	}
	var buf bytes.Buffer
	if err := transformMapT.Execute(&buf, data); err != nil {
		panic(err) // bug
	}
	return buf.String(), nil
}

// mapDepth returns the level of nested maps. If map not nested, it returns 0.
func mapDepth(mp *design.Map) int {
	return traverseMap(mp.ElemType.Type, 0)
}

func traverseMap(dt design.DataType, depth int, seen ...map[string]struct{}) int {
	if mp := design.AsMap(dt); mp != nil {
		depth++
		depth = traverseMap(mp.ElemType.Type, depth, seen...)
	} else if ar := design.AsArray(dt); ar != nil {
		depth = traverseMap(ar.ElemType.Type, depth, seen...)
	} else if mo := design.AsObject(dt); mo != nil {
		var s map[string]struct{}
		if len(seen) > 0 {
			s = seen[0]
		} else {
			s = make(map[string]struct{})
			seen = append(seen, s)
		}
		key := dt.Name()
		if u, ok := dt.(design.UserType); ok {
			key = u.ID()
		}
		if _, ok := s[key]; ok {
			return depth
		}
		s[key] = struct{}{}
		var level int
		for _, nat := range *mo {
			// if object type has attributes of type map then find out the attribute that has
			// the deepest level of nested maps
			lvl := 0
			lvl = traverseMap(nat.Attribute.Type, lvl, seen...)
			if lvl > level {
				level = lvl
			}
		}
		depth += level
	}
	return depth
}

func transformAttributeHelpers(source, target design.DataType, a thargs, seen ...map[string]*codegen.TransformFunctionData) ([]*codegen.TransformFunctionData, error) {
	var (
		helpers []*codegen.TransformFunctionData
		err     error
	)
	// Do not generate a transform function for the top most user type.
	switch {
	case design.IsArray(source):
		source = design.AsArray(source).ElemType.Type
		target = design.AsArray(target).ElemType.Type
		helpers, err = transformAttributeHelpers(source, target, a, seen...)
	case design.IsMap(source):
		sm := design.AsMap(source)
		tm := design.AsMap(target)
		source = sm.ElemType.Type
		target = tm.ElemType.Type
		helpers, err = transformAttributeHelpers(source, target, a, seen...)
		if err == nil {
			var other []*codegen.TransformFunctionData
			source = sm.KeyType.Type
			target = tm.KeyType.Type
			other, err = transformAttributeHelpers(source, target, a, seen...)
			helpers = append(helpers, other...)
		}
	case design.IsObject(source):
		helpers, err = transformObjectHelpers(source, target, a, seen...)
	}
	if err != nil {
		return nil, err
	}
	return helpers, nil
}

func transformObjectHelpers(source, target design.DataType, a thargs, seen ...map[string]*codegen.TransformFunctionData) ([]*codegen.TransformFunctionData, error) {
	var (
		helpers []*codegen.TransformFunctionData
		err     error

		satt = &design.AttributeExpr{Type: source}
		tatt = &design.AttributeExpr{Type: target}
	)
	walkMatches(satt, tatt, func(src, tgt *design.MappedAttributeExpr, srcAtt, tgtAtt *design.AttributeExpr, n string) {
		if err != nil {
			return
		}
		h, err2 := collectHelpers(srcAtt, tgtAtt, a, src.IsRequired(n), seen...)
		if err2 != nil {
			err = err2
			return
		}
		helpers = append(helpers, h...)
	})
	if err != nil {
		return nil, err
	}
	return helpers, nil
}

// isCompatible returns an error if a and b are not both objects, both arrays,
// both maps or both the same primitive type. actx and bctx are used to build
// the error message if any.
func isCompatible(a, b design.DataType, actx, bctx string) error {
	switch {
	case design.IsObject(a):
		if !design.IsObject(b) {
			return fmt.Errorf("%s is an object but %s type is %s", actx, bctx, b.Name())
		}
	case design.IsArray(a):
		if !design.IsArray(b) {
			return fmt.Errorf("%s is an array but %s type is %s", actx, bctx, b.Name())
		}
	case design.IsMap(a):
		if !design.IsMap(b) {
			return fmt.Errorf("%s is a hash but %s type is %s", actx, bctx, b.Name())
		}
	default:
		if a.Kind() != b.Kind() {
			return fmt.Errorf("%s is a %s but %s type is %s", actx, a.Name(), bctx, b.Name())
		}
	}

	return nil
}

// collectHelpers recursively traverses the given attributes and return the
// transform helper functions required to generate the transform code.
func collectHelpers(source, target *design.AttributeExpr, a thargs, req bool, seen ...map[string]*codegen.TransformFunctionData) ([]*codegen.TransformFunctionData, error) {
	var data []*codegen.TransformFunctionData
	switch {
	case design.IsArray(source.Type):
		helpers, err := transformAttributeHelpers(
			design.AsArray(source.Type).ElemType.Type,
			design.AsArray(target.Type).ElemType.Type,
			a, seen...)
		if err != nil {
			return nil, err
		}
		data = append(data, helpers...)
	case design.IsMap(source.Type):
		helpers, err := transformAttributeHelpers(
			design.AsMap(source.Type).KeyType.Type,
			design.AsMap(target.Type).KeyType.Type,
			a, seen...)
		if err != nil {
			return nil, err
		}
		data = append(data, helpers...)
		helpers, err = transformAttributeHelpers(
			design.AsMap(source.Type).ElemType.Type,
			design.AsMap(target.Type).ElemType.Type,
			a, seen...)
		if err != nil {
			return nil, err
		}
		data = append(data, helpers...)
	case design.IsObject(source.Type):
		if ut, ok := source.Type.(design.UserType); ok {
			name := transformHelperName(source, target, targs{unmarshal: a.unmarshal, scope: a.scope})
			var s map[string]*codegen.TransformFunctionData
			if len(seen) > 0 {
				s = seen[0]
			} else {
				s = make(map[string]*codegen.TransformFunctionData)
				seen = append(seen, s)
			}
			if _, ok := s[name]; ok {
				return nil, nil
			}
			code, err := transformAttribute(ut.Attribute(), target, true,
				targs{"v", "res", a.sourcePkg, a.targetPkg, a.unmarshal, a.scope})
			if err != nil {
				return nil, err
			}
			if !req {
				code = "if v == nil {\n\treturn nil\n}\n" + code
			}
			t := &codegen.TransformFunctionData{
				Name:          name,
				ParamTypeRef:  a.scope.GoFullTypeRef(source, a.sourcePkg),
				ResultTypeRef: a.scope.GoFullTypeRef(target, a.targetPkg),
				Code:          code,
			}
			s[name] = t
			data = append(data, t)
		}
		var err error
		walkMatches(source, target, func(srcm, _ *design.MappedAttributeExpr, src, tgt *design.AttributeExpr, n string) {
			var helpers []*codegen.TransformFunctionData
			helpers, err = collectHelpers(src, tgt, a, srcm.IsRequired(n), seen...)
			if err != nil {
				return
			}
			data = append(data, helpers...)
		})

		if err != nil {
			return nil, err
		}
	}
	return data, nil
}

func walkMatches(source, target *design.AttributeExpr, walker func(src, tgt *design.MappedAttributeExpr, srcc, tgtc *design.AttributeExpr, n string)) {
	src := design.NewMappedAttributeExpr(source)
	tgt := design.NewMappedAttributeExpr(target)
	srcObj := design.AsObject(src.Type)
	tgtObj := design.AsObject(tgt.Type)
	// Map source object attribute names to target object attributes
	attributeMap := make(map[string]*design.AttributeExpr)
	for _, nat := range *srcObj {
		if att := tgtObj.Attribute(nat.Name); att != nil {
			attributeMap[nat.Name] = att
		}
	}
	for _, natt := range *srcObj {
		n := natt.Name
		tgtc, ok := attributeMap[n]
		if !ok {
			continue
		}
		walker(src, tgt, natt.Attribute, tgtc, n)
	}
}

// typeCast type casts the source attribute type based on the target type.
// NOTE: For Int and UInt kinds, protocol buffer Go compiler generates
// int32 and uint32 respectively whereas goa v2 generates int and uint.
//
// proto if true indicates that the target attribute is a protocol buffer type.
func typeCast(sourceVar string, source, target design.DataType, proto bool) string {
	if source.Kind() != design.IntKind && source.Kind() != design.UIntKind {
		return sourceVar
	}
	if proto {
		sourceVar = fmt.Sprintf("%s(%s)", ProtoBufNativeGoTypeName(source), sourceVar)
	} else {
		sourceVar = fmt.Sprintf("%s(%s)", codegen.GoNativeTypeName(source), sourceVar)
	}
	return sourceVar
}

func transformHelperName(satt, tatt *design.AttributeExpr, a targs) string {
	var (
		sname string
		tname string

		suffix = "ProtoBuf"
	)
	{
		sname = a.scope.GoTypeName(satt)
		tname = a.scope.GoTypeName(tatt)
		if a.unmarshal {
			tname += suffix
		} else {
			sname += suffix
		}
	}
	return codegen.Goify(sname+"To"+tname, false)
}

// used by template
func transformAttributeHelper(source, target *design.AttributeExpr, sourceVar, targetVar, sourcePkg, targetPkg string, unmarshal, newVar bool, scope *codegen.NameScope) (string, error) {
	return transformAttribute(source, target, newVar, targs{sourceVar, targetVar, sourcePkg, targetPkg, unmarshal, scope})
}

const transformArrayTmpl = `{{ .Target}} {{ if .NewVar }}:{{ end }}= make([]{{ .ElemTypeRef }}, len({{ .Source }}))
for {{ .LoopVar }}, val := range {{ .Source }} {
  {{ transformAttribute .SourceElem .TargetElem "val" (printf "%s[%s]" .Target .LoopVar) .SourcePkg .TargetPkg .Unmarshal false .Scope -}}
}
`

const transformMapTmpl = `{{ .Target }} {{ if .NewVar }}:{{ end }}= make(map[{{ .KeyTypeRef }}]{{ .ElemTypeRef }}, len({{ .Source }}))
for key, val := range {{ .Source }} {
  {{ transformAttribute .SourceKey .TargetKey "key" "tk" .SourcePkg .TargetPkg .Unmarshal true .Scope -}}
  {{ transformAttribute .SourceElem .TargetElem "val" (printf "tv%s" .LoopVar) .SourcePkg .TargetPkg .Unmarshal true .Scope -}}
  {{ .Target }}[tk] = {{ printf "tv%s" .LoopVar }}
}
`
