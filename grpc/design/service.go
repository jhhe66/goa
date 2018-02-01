package design

import (
	"fmt"

	"goa.design/goa/design"
	//"goa.design/goa/eval"
)

type (
	// ServiceExpr describes a gRPC service.
	ServiceExpr struct {
		// ServiceExpr is the service expression that backs this service.
		ServiceExpr *design.ServiceExpr
		// Name of parent service if any
		ParentName string
		// GRPCEndpoints is the list of service endpoints.
		GRPCEndpoints []*EndpointExpr
	}
)

// Name of service (service)
func (svc *ServiceExpr) Name() string {
	return svc.ServiceExpr.Name
}

// Description of service (service)
func (svc *ServiceExpr) Description() string {
	return svc.ServiceExpr.Description
}

// Endpoint returns the service endpoint with the given name or nil if there
// isn't one.
func (svc *ServiceExpr) Endpoint(name string) *EndpointExpr {
	for _, a := range svc.GRPCEndpoints {
		if a.Name() == name {
			return a
		}
	}
	return nil
}

// EndpointFor builds the endpoint for the given method.
func (svc *ServiceExpr) EndpointFor(name string, m *design.MethodExpr) *EndpointExpr {
	if a := svc.Endpoint(name); a != nil {
		return a
	}
	a := &EndpointExpr{
		MethodExpr: m,
		Service:    svc,
	}
	svc.GRPCEndpoints = append(svc.GRPCEndpoints, a)
	return a
}

// EvalName returns the generic definition name used in error messages.
func (svc *ServiceExpr) EvalName() string {
	if svc.Name() == "" {
		return "unnamed service"
	}
	return fmt.Sprintf("service %#v", svc.Name())
}
