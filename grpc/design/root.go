package design

import (
	"sort"

	"goa.design/goa/design"
	"goa.design/goa/eval"
)

var (
	// Root holds the root expression built on process initialization.
	Root = &RootExpr{Design: design.Root}
)

type (
	// RootExpr is the data structure built by the top level GRPC DSL.
	RootExpr struct {
		// Design is the transport agnostic root expression.
		Design *design.RootExpr
		// GRPCServices contains the services created by the DSL.
		GRPCServices []*ServiceExpr
	}
)

// Service returns the service with the given name if any.
func (r *RootExpr) Service(name string) *ServiceExpr {
	for _, res := range r.GRPCServices {
		if res.Name() == name {
			return res
		}
	}
	return nil
}

// ServiceFor creates a new or returns the existing service definition for the
// given service.
func (r *RootExpr) ServiceFor(s *design.ServiceExpr) *ServiceExpr {
	if res := r.Service(s.Name); res != nil {
		return res
	}
	res := &ServiceExpr{
		ServiceExpr: s,
	}
	r.GRPCServices = append(r.GRPCServices, res)
	return res
}

// EvalName is the expression name used by the evaluation engine to display
// error messages.
func (r *RootExpr) EvalName() string {
	return "API GRPC"
}

// WalkSets iterates through the service to finalize and validate them.
func (r *RootExpr) WalkSets(walk eval.SetWalker) {
	var (
		services  eval.ExpressionSet
		endpoints eval.ExpressionSet
	)
	{
		services = make(eval.ExpressionSet, len(r.GRPCServices))
		sort.SliceStable(r.GRPCServices, func(i, j int) bool {
			if r.GRPCServices[j].ParentName == r.GRPCServices[i].Name() {
				return true
			}
			return false
		})
		for i, svc := range r.GRPCServices {
			services[i] = svc
			for _, e := range svc.GRPCEndpoints {
				endpoints = append(endpoints, e)
			}
		}
	}
	walk(services)
	walk(endpoints)
}

// DependsOn is a no-op as the DSL runs when loaded.
func (r *RootExpr) DependsOn() []eval.Root { return nil }

// Packages returns the Go import path to this and the dsl packages.
func (r *RootExpr) Packages() []string {
	return []string{
		"goa.design/goa/grpc/design",
		"goa.design/goa/grpc/dsl",
	}
}
