package codegen

import (
	"strconv"
	"testing"

	"goa.design/goa/codegen"
	"goa.design/goa/design"
)

func TestProtoBufMessageDef(t *testing.T) {
	var (
		simpleArray = array(design.Boolean)
		simpleMap   = mapa(design.Int, design.String)
		ut          = &design.UserTypeExpr{AttributeExpr: &design.AttributeExpr{Type: design.Boolean}, TypeName: "UserType"}
		obj         = objectRPC("IntField", design.Int, "ArrayField", simpleArray.Type, "MapField", simpleMap.Type, "UserTypeField", ut)
		rt          = &design.ResultTypeExpr{UserTypeExpr: &design.UserTypeExpr{AttributeExpr: &design.AttributeExpr{Type: design.Boolean}, TypeName: "ResultType"}, Identifier: "application/vnd.goa.example", Views: nil}
		userType    = &design.AttributeExpr{Type: ut}
		resultType  = &design.AttributeExpr{Type: rt}
	)
	cases := map[string]struct {
		att      *design.AttributeExpr
		expected string
	}{
		"BooleanKind": {&design.AttributeExpr{Type: design.Boolean}, "bool"},
		"IntKind":     {&design.AttributeExpr{Type: design.Int}, "sint32"},
		"Int32Kind":   {&design.AttributeExpr{Type: design.Int32}, "sint32"},
		"Int64Kind":   {&design.AttributeExpr{Type: design.Int64}, "sint64"},
		"UIntKind":    {&design.AttributeExpr{Type: design.UInt}, "uint32"},
		"UInt32Kind":  {&design.AttributeExpr{Type: design.UInt32}, "uint32"},
		"UInt64Kind":  {&design.AttributeExpr{Type: design.UInt64}, "uint64"},
		"Float32Kind": {&design.AttributeExpr{Type: design.Float32}, "float"},
		"Float64Kind": {&design.AttributeExpr{Type: design.Float64}, "double"},
		"StringKind":  {&design.AttributeExpr{Type: design.String}, "string"},
		"BytesKind":   {&design.AttributeExpr{Type: design.Bytes}, "bytes"},

		"Array":          {simpleArray, "repeated bool"},
		"Map":            {simpleMap, "map<sint32, string>"},
		"UserTypeExpr":   {userType, "UserType"},
		"ResultTypeExpr": {resultType, "ResultType"},

		"Object": {obj, " {\n\tsint32 int_field = 1;\n\trepeated bool array_field = 2;\n\tmap<sint32, string> map_field = 3;\n\tUserType user_type_field = 4;\n}"},
	}

	for k, tc := range cases {
		scope := codegen.NewNameScope()
		actual := ProtoBufMessageDef(tc.att, scope)
		if actual != tc.expected {
			t.Errorf("%s: got %#v, expected %#v", k, actual, tc.expected)
		}
	}
}

func TestProtoBufNativeMessageTypeName(t *testing.T) {
	cases := map[string]struct {
		dataType design.DataType
		expected string
	}{
		"BooleanKind": {design.Boolean, "bool"},
		"IntKind":     {design.Int, "sint32"},
		"Int32Kind":   {design.Int32, "sint32"},
		"Int64Kind":   {design.Int64, "sint64"},
		"UIntKind":    {design.UInt, "uint32"},
		"UInt32Kind":  {design.UInt32, "uint32"},
		"UInt64Kind":  {design.UInt64, "uint64"},
		"Float32Kind": {design.Float32, "float"},
		"Float64Kind": {design.Float64, "double"},
		"StringKind":  {design.String, "string"},
		"BytesKind":   {design.Bytes, "bytes"},
	}

	for k, tc := range cases {
		actual := ProtoBufNativeMessageTypeName(tc.dataType)
		if actual != tc.expected {
			t.Errorf("%s: got %#v, expected %#v", k, actual, tc.expected)
		}
	}
}

func objectRPC(params ...interface{}) *design.AttributeExpr {
	obj := design.Object{}
	for i := 0; i < len(params); i += 2 {
		name := params[i].(string)
		typ := params[i+1].(design.DataType)
		obj = append(obj, &design.NamedAttributeExpr{
			Name: name,
			Attribute: &design.AttributeExpr{
				Type:     typ,
				Metadata: design.MetadataExpr{"rpc:tag": []string{strconv.Itoa(int(i/2) + 1)}},
			},
		})
	}
	return &design.AttributeExpr{Type: &obj}
}

func array(dt design.DataType) *design.AttributeExpr {
	elem := &design.AttributeExpr{Type: dt}
	return &design.AttributeExpr{Type: &design.Array{ElemType: elem}}
}

func mapa(keyt, elemt design.DataType) *design.AttributeExpr {
	key := &design.AttributeExpr{Type: keyt}
	elem := &design.AttributeExpr{Type: elemt}
	return &design.AttributeExpr{Type: &design.Map{KeyType: key, ElemType: elem}}
}
