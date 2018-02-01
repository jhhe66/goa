package dsl

import (
	"goa.design/goa/design"
	"goa.design/goa/eval"
	grpcdesign "goa.design/goa/grpc/design"
)

// GRPC defines gRPC transport specific properties on an API, a service, or a
// single method.
//
// GRPC must appear in a Method expression.
//
// GRPC accepts a single argument which is the defining DSL function.
//
// Example:
//
//    var _ = Service("calculator", func() {
//        Method("add", func() {
//            Description("Add two operands")
//            Payload(Operands)
//            Error(BadRequest, ErrorResult)
//
//            GRPC(func() {
//                Name("add")
//                Response(func() {
//                    Field(1, "sum", Integer, "The sum")
//                })
//            })
//        })
//    })
func GRPC(fn func()) {
	switch actual := eval.Current().(type) {
	case *design.MethodExpr:
		res := grpcdesign.Root.ServiceFor(actual.Service)
		act := res.EndpointFor(actual.Name, actual)
		act.DSLFunc = fn
	default:
		eval.IncompatibleDSL()
	}
}

// Request describes a gRPC request message.
//
// Request must appear in a gRPC endpoint expression to define the request
// message. If Request is not explicitly defined, the Request expression is
// built from the method Payload expression.
//
// Request accepts one argument which describes the shape of the Request
// message. It can be:
//
//  - a string corresponding to an attribute name in the method Payload
//    expression. In this case the corresponding attribute type describes the
//    shape of the request message.
//  - a function listing the request attributes. The attributes inherit the
//    properties (description, type, etc.) of the method Payload attributes
//    with identical names.
//
// Assuming the type:
//
//     var CreatePayload = Type("CreatePayload", func() {
//			   Attribute("name", String, "Name of account")
//     })
//
// The following:
//
//     Method("create", func() {
//         Payload(CreatePayload)
//     })
//
// is equivalent to:
//
//     Method("create", func() {
//         Payload(CreatePayload)
//         GRPC(func() {
//             Request(func() {
//                 Field(1, "name")
//             })
//         })
//     })
//
func Request(args ...interface{}) {
	if len(args) == 0 {
		eval.ReportError("not enough arguments, use Request(name), Request(type), Request(func()) or Request(type, func())")
		return
	}

	var (
		ref    *design.AttributeExpr
		setter func(*design.AttributeExpr)
	)

	// Figure out reference type and setter function
	switch e := eval.Current().(type) {
	case *grpcdesign.EndpointExpr:
		ref = e.MethodExpr.Payload
		setter = func(att *design.AttributeExpr) {
			e.Request = att
		}
	default:
		eval.IncompatibleDSL()
		return
	}

	// Set request attribute
	attr, fn := initAttr(ref, "request", args...)
	if fn != nil {
		eval.Execute(fn, attr)
	}
	if attr.Metadata == nil {
		attr.Metadata = design.MetadataExpr{}
	}
	attr.Metadata["grpc:request"] = []string{}
	setter(attr)
}

// Response describes a gRPC response message.
//
// Response must appear in a gRPC endpoint expression to define the response
// message. If Response is not explicitly defined, the Response expression is
// built from the method Result expression.
//
// Response accepts one argument which describes the shape of the Response
// message. It can be:
//
//  - a string corresponding to an attribute name in the method Result
//    expression. In this case the corresponding attribute type describes the
//    shape of the response message.
//  - a function listing the response attributes. The attributes inherit the
//    properties (description, type, etc.) of the method Result attributes
//    with identical names.
//
// Assuming the type:
//
//     var CreateResult = Type("CreateResult", func() {
//         Attribute("name", String, "Name of account")
//     })
//
// The following:
//
//     Method("create", func() {
//         Result(CreateResult)
//     })
//
// is equivalent to:
//
//     Method("create", func() {
//         Result(CreateResult)
//         GRPC(func() {
//             Response(func() {
//                 Field(1, "name")
//             })
//         })
//     })
//
func Response(args ...interface{}) {
	if len(args) == 0 {
		eval.ReportError("not enough arguments, use Response(name), Response(type), Response(func()) or Response(type, func())")
		return
	}

	var (
		ref    *design.AttributeExpr
		setter func(*design.AttributeExpr)
	)

	// Figure out reference type and setter function
	switch e := eval.Current().(type) {
	case *grpcdesign.EndpointExpr:
		ref = e.MethodExpr.Payload
		setter = func(att *design.AttributeExpr) {
			e.Response = att
		}
	default:
		eval.IncompatibleDSL()
		return
	}

	// Set response attribute
	attr, fn := initAttr(ref, "response", args...)
	if fn != nil {
		eval.Execute(fn, attr)
	}
	if attr != nil {
		if attr.Metadata == nil {
			attr.Metadata = design.MetadataExpr{}
		}
		attr.Metadata["grpc:response"] = []string{}
		setter(attr)
	}
}

// initAttr returns an attribute expression initialized from a source attribute.
func initAttr(ref *design.AttributeExpr, kind string, args ...interface{}) (*design.AttributeExpr, func()) {
	var (
		attr *design.AttributeExpr
		fn   func()
	)
	switch a := args[0].(type) {
	case string:
		if ref.Find(a) == nil {
			eval.ReportError("%q is not found in type", a)
			return nil, nil
		}
		obj := design.AsObject(ref.Type)
		if obj == nil {
			eval.ReportError("%s type must be an object with an attribute with name %#v, got %T", kind, a, ref.Type)
			return nil, nil
		}
		attr = obj.Attribute(a)
		if attr == nil {
			eval.ReportError("%s request type does not have an attribute named %#v", kind, a)
			return nil, nil
		}
		attr = design.DupAtt(attr)
		if attr.Metadata == nil {
			attr.Metadata = design.MetadataExpr{"origin:attribute": []string{a}}
		} else {
			attr.Metadata["origin:attribute"] = []string{a}
		}
	case design.UserType:
		attr = &design.AttributeExpr{Type: a}
		if len(args) > 1 {
			var ok bool
			fn, ok = args[1].(func())
			if !ok {
				eval.ReportError("second argument must be a function")
			}
		}
	case func():
		fn = a
		attr = ref
	default:
		eval.InvalidArgError("attribute name, user type or DSL", a)
		return nil, nil
	}
	return attr, fn
}
