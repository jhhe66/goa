package codegen

import (
	"fmt"

	"goa.design/goa/codegen"
	"goa.design/goa/codegen/service"
	"goa.design/goa/design"
	grpcdesign "goa.design/goa/grpc/design"
)

// GRPCServices holds the data computed from the design needed to generate the
// transport code of the services.
var GRPCServices = make(ServicesData)

type (
	// ServicesData encapsulates the data computed from the design.
	ServicesData map[string]*ServiceData

	// ServiceData contains the data used to render the code related to a
	// single service.
	ServiceData struct {
		// Service contains the related service data.
		Service *service.Data
		// gRPC package name
		PkgName string
		// Name is the service name.
		Name string
		// Description is the service description.
		Description string
		// Endpoints describes the gRPC service endpoints.
		Endpoints []*EndpointData
		// Messages describes the message data for this service.
		Messages []*MessageData
		// ServerStruct is the name of the gRPC server struct.
		ServerStruct string
		// ServerInit is the name of the constructor of the server
		// struct.
		ServerInit string
		// ServerInterface is the name of the gRPC server interface implemented by the service.
		ServerInterface string
		// TransformHelpers is the list of transform functions required by the
		// constructors.
		TransformHelpers []*codegen.TransformFunctionData
	}

	// EndpointData contains the data used to render the code related to
	// gRPC endpoint.
	EndpointData struct {
		// Name is the name of the endpoint.
		Name string
		// gRPC package name
		PkgName string
		// Description is the description for the endpoint.
		Description string
		// Method is the data for the underlying method expression.
		Method *service.MethodData
		// Request is the name of the request message for the endpoint.
		Request *RequestData
		// Response is the name of the response message for the endpoint.
		Response *ProtoBufTypeData
		// ServerStruct is the name of the gRPC server struct.
		ServerStruct string
		// ServerInterface is the name of the gRPC server interface implemented by the service.
		ServerInterface string
	}

	// MessageData contains the data used to render the code related to a
	// message for a gRPC service.
	MessageData struct {
		// Name is the message name.
		Name string
		// Description is the message description.
		Description string
		// VarName is the variable name that holds the definition.
		VarName string
		// Def is the message definition.
		Def string
		// Type is the underlying type.
		Type design.UserType
	}

	// RequestData contains the data to transform gRPC request type to the
	// corresponding payload type.
	RequestData struct {
		// Name is the name of the request type.
		Name string
		// Ref is the fully qualified reference to the gRPC request type.
		Ref string
		// PayloadInit contains the data required to render the payload
		// constructor if any.
		PayloadInit *InitData
	}

	// ProtoBufTypeData contains the data referring to the generated protocol
	// buffer type and the constructor needed to transform a service type to the
	// protocol buffer type.
	ProtoBufTypeData struct {
		// Name is the type name.
		Name string
		// Init contains the data needed to render and call the type
		// constructor if any.
		Init *InitData
		// Ref is the reference to the type.
		Ref string
		// Example is an example value for the type.
		Example interface{}
	}

	// InitData contains the data required to render a constructor.
	InitData struct {
		// Name is the constructor function name.
		Name string
		// Description is the function description.
		Description string
		// Args is the list of constructor arguments.
		Args []*InitArgData
		// ReturnVarName is the name of the variable to be returned.
		ReturnVarName string
		// ReturnTypeRef is the qualified (including the package name)
		// reference to the return type.
		ReturnTypeRef string
		// Code is the transformation code.
		Code string
	}

	// InitArgData represents a single constructor argument.
	InitArgData struct {
		// Name is the argument name.
		Name string
		// Description is the argument description.
		Description string
		// Reference to the argument, e.g. "&body".
		Ref string
		// FieldName is the name of the data structure field that should
		// be initialized with the argument if any.
		FieldName string
		// TypeName is the argument type name.
		TypeName string
		// TypeRef is the argument type reference.
		TypeRef string
		// Pointer is true if a pointer to the arg should be used.
		Pointer bool
		// Required is true if the arg is required to build the payload.
		Required bool
		// DefaultValue is the default value of the arg.
		DefaultValue interface{}
		// Validate contains the validation code for the argument
		// value if any.
		Validate string
		// Example is a example value
		Example interface{}
	}
)

// Get retrieves the transport data for the service with the given name
// computing it if needed. It returns nil if there is no service with the given
// name.
func (d ServicesData) Get(name string) *ServiceData {
	if data, ok := d[name]; ok {
		return data
	}
	service := grpcdesign.Root.Service(name)
	if service == nil {
		return nil
	}
	d[name] = d.analyze(service)
	return d[name]
}

// Endpoint returns the service method transport data for the endpoint with the
// given name, nil if there isn't one.
func (sd *ServiceData) Endpoint(name string) *EndpointData {
	name = codegen.Goify(name, true)
	for _, ed := range sd.Endpoints {
		if ed.Name == name {
			return ed
		}
	}
	return nil
}

// analyze creates the data necessary to render the code of the given service.
func (d ServicesData) analyze(gs *grpcdesign.ServiceExpr) *ServiceData {
	var (
		sd   *ServiceData
		seen map[string]struct{}

		svc = service.Services.Get(gs.Name())
	)
	{
		sd = &ServiceData{
			Service:         svc,
			Name:            svc.Name,
			Description:     svc.Description,
			PkgName:         svc.Name + "pb",
			ServerStruct:    "Server",
			ServerInit:      "New",
			ServerInterface: codegen.Goify(svc.Name, true) + "Server",
		}
		seen = make(map[string]struct{})
	}
	for _, e := range gs.GRPCEndpoints {
		// Make request to a user type
		if _, ok := e.Request.Type.(design.UserType); !ok {
			e.Request.Type = &design.UserTypeExpr{
				AttributeExpr: wrapAttr(e.Request),
				TypeName:      fmt.Sprintf("%sRequest", ProtoBufify(e.Name(), true)),
			}
		}
		// Make response to a user type
		if _, ok := e.Response.Type.(design.UserType); !ok {
			e.Response.Type = &design.UserTypeExpr{
				AttributeExpr: wrapAttr(e.Response),
				TypeName:      fmt.Sprintf("%sResponse", ProtoBufify(e.Name(), true)),
			}
		}
		sd.Messages = append(sd.Messages, collectMessages(e.Request, seen, svc.Scope)...)
		sd.Messages = append(sd.Messages, collectMessages(e.Response, seen, svc.Scope)...)
		sd.Endpoints = append(sd.Endpoints, &EndpointData{
			Name:            codegen.Goify(e.Name(), true),
			PkgName:         sd.PkgName,
			ServerStruct:    sd.ServerStruct,
			ServerInterface: sd.ServerInterface,
			Description:     e.Description(),
			Method:          svc.Method(e.Name()),
			Request:         buildRequestData(e, sd),
			Response:        buildResponseProtoBufTypeData(e, sd),
		})
	}
	return sd
}

// wrapAttr wraps the given attribute into an attribute named "field" if
// the given attribute is a non-object type. For a raw object type it simply
// returns a dupped attribute.
func wrapAttr(att *design.AttributeExpr) *design.AttributeExpr {
	var attr *design.AttributeExpr
	switch actual := att.Type.(type) {
	case *design.Array:
	case *design.Map:
	case design.Primitive:
		attr = &design.AttributeExpr{
			Type: &design.Object{
				&design.NamedAttributeExpr{
					Name: "field",
					Attribute: &design.AttributeExpr{
						Type:     actual,
						Metadata: design.MetadataExpr{"rpc:tag": []string{"1"}},
					},
				},
			},
		}
	case *design.Object:
		attr = design.DupAtt(att)
	}
	return attr
}

// collectMessages recurses through the attribute to gather all the messages.
func collectMessages(at *design.AttributeExpr, seen map[string]struct{}, scope *codegen.NameScope) (data []*MessageData) {
	if at == nil || at.Type == design.Empty {
		return
	}
	collect := func(at *design.AttributeExpr) []*MessageData { return collectMessages(at, seen, scope) }
	switch dt := at.Type.(type) {
	case design.UserType:
		if _, ok := seen[dt.Name()]; ok {
			return nil
		}
		data = append(data, &MessageData{
			Name:        dt.Name(),
			VarName:     ProtoBufMessageName(at, scope),
			Description: dt.Attribute().Description,
			Def:         ProtoBufMessageDef(dt.Attribute(), scope),
			Type:        dt,
		})
		seen[dt.Name()] = struct{}{}
		data = append(data, collect(dt.Attribute())...)
	case *design.Object:
		for _, nat := range *dt {
			data = append(data, collect(nat.Attribute)...)
		}
	case *design.Array:
		data = append(data, collect(dt.ElemType)...)
	case *design.Map:
		data = append(data, collect(dt.KeyType)...)
		data = append(data, collect(dt.ElemType)...)
	}
	return
}

func buildRequestData(e *grpcdesign.EndpointExpr, sd *ServiceData) *RequestData {
	var (
		name string
		ref  string
		init *InitData

		svc = sd.Service
	)
	{
		name = ProtoBufMessageName(e.Request, svc.Scope)
		ref = ProtoBufFullTypeRef(e.Request, sd.PkgName, svc.Scope)
		if e.MethodExpr.Payload.Type != design.Empty {
			var (
				name string
				desc string
				code string
				arg  *InitArgData

				srcVar = "p"
				retVar = "v"
			)
			{
				name = "New" + svc.Scope.GoTypeName(e.MethodExpr.Payload)
				desc = fmt.Sprintf("%s builds the payload from the gRPC request type of the %q endpoint of the %q service.", name, e.Name(), svc.Name)
				code = protoBufTypeTransformHelper(e.Request, e.MethodExpr.Payload, srcVar, retVar, sd.PkgName, svc.PkgName, false, sd)
				arg = &InitArgData{
					Name:    srcVar,
					Ref:     srcVar,
					TypeRef: svc.Scope.GoFullTypeRef(e.Request, sd.PkgName),
					Example: e.Request.Example(design.Root.API.Random()),
				}
			}
			init = &InitData{
				Name:          name,
				Description:   desc,
				ReturnVarName: retVar,
				ReturnTypeRef: ProtoBufFullTypeRef(e.MethodExpr.Payload, svc.PkgName, svc.Scope),
				Code:          code,
				Args:          []*InitArgData{arg},
			}
		}
	}

	return &RequestData{
		Name:        name,
		Ref:         ref,
		PayloadInit: init,
	}
}

// buildResponseProtoBufTypeData builds the ProtoBufTypeData for a gRPC response
// message. The data contains information to generate a constructor function
// that creates a gRPC response type from the service method result type.
func buildResponseProtoBufTypeData(e *grpcdesign.EndpointExpr, sd *ServiceData) *ProtoBufTypeData {
	var (
		name string
		ref  string

		svc = sd.Service
	)
	{
		name = ProtoBufMessageName(e.Response, svc.Scope)
		ref = ProtoBufFullTypeRef(e.Response, sd.PkgName, svc.Scope)
	}

	var init *InitData
	{
		if e.MethodExpr.Result.Type != design.Empty {
			var (
				iname string
				desc  string
				code  string
				arg   *InitArgData

				srcVar = "res"
				retVar = "v"
			)
			{
				iname = "New" + name
				desc = fmt.Sprintf("%s builds the gRPC response type from the result of the %q endpoint of the %q service.", iname, e.Name(), svc.Name)
				code = protoBufTypeTransformHelper(e.MethodExpr.Result, e.Response, srcVar, retVar, svc.PkgName, sd.PkgName, true, sd)
				arg = &InitArgData{
					Name:    srcVar,
					Ref:     srcVar,
					TypeRef: svc.Scope.GoFullTypeRef(e.MethodExpr.Result, svc.PkgName),
					Example: e.MethodExpr.Result.Example(design.Root.API.Random()),
				}
			}
			init = &InitData{
				Name:          iname,
				Description:   desc,
				ReturnVarName: retVar,
				ReturnTypeRef: ProtoBufFullTypeRef(e.Response, sd.PkgName, svc.Scope),
				Code:          code,
				Args:          []*InitArgData{arg},
			}
		}
	}

	return &ProtoBufTypeData{
		Name: name,
		Init: init,
		Ref:  ref,
	}
}

// protoBufTypeTransformHelper is a helper function to transform a protocol
// buffer message type to a Go type and vice versa. If src and tgt are of
// different types (i.e. the Payload/Result is a non-user type and
// Request/Response message is always a user type), the function returns the
// code for initializing the types appropriately by making use of the wrapped
// "field" attribute. Use this function in places where
// codegen.ProtoBufTypeTransform needs to be called.
func protoBufTypeTransformHelper(src, tgt *design.AttributeExpr, srcVar, tgtVar, srcPkg, tgtPkg string, proto bool, sd *ServiceData) string {
	var (
		code string
		err  error
		h    []*codegen.TransformFunctionData

		svc = sd.Service
	)
	if e := isCompatible(src.Type, tgt.Type, srcVar, tgtVar); e == nil {
		code, h, err = ProtoBufTypeTransform(src.Type, tgt.Type, srcVar, tgtVar, srcPkg, tgtPkg, proto, svc.Scope)
		if err != nil {
			fmt.Println(err.Error()) // TBD validate DSL so errors are not possible
			return ""
		}
		sd.TransformHelpers = codegen.AppendHelpers(sd.TransformHelpers, h)
		return code
	}
	if proto {
		// tgt is a protocol buffer message type. src type is wrapped in an
		// attribute called "field" in tgt.
		pbType := ProtoBufFullMessageName(tgt, tgtPkg, svc.Scope)
		code = fmt.Sprintf("%s := &%s{\nField: %s,\n}", tgtVar, pbType, typeCast(srcVar, src.Type, tgt.Type, proto))
	} else {
		// tgt is a Go type. src is a protocol buffer message type.
		code = fmt.Sprintf("%s := %s\n", tgtVar, typeCast(srcVar+".Field", src.Type, tgt.Type, proto))
	}
	return code
}

// needInit returns true if and only if the given type is or makes use of user
// types.
func needInit(dt design.DataType) bool {
	if dt == design.Empty {
		return false
	}
	switch actual := dt.(type) {
	case design.Primitive:
		return false
	case *design.Array:
		return needInit(actual.ElemType.Type)
	case *design.Map:
		return needInit(actual.KeyType.Type) ||
			needInit(actual.ElemType.Type)
	case *design.Object:
		for _, nat := range *actual {
			if needInit(nat.Attribute.Type) {
				return true
			}
		}
		return false
	case design.UserType:
		return true
	default:
		panic(fmt.Sprintf("unknown data type %T", actual)) // bug
	}
}
