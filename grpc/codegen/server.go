package codegen

import (
	"fmt"
	"path/filepath"

	"goa.design/goa/codegen"
	grpcdesign "goa.design/goa/grpc/design"
)

// ServerFiles returns all the server GRPC transport files.
func ServerFiles(genpkg string, root *grpcdesign.RootExpr) []*codegen.File {
	fw := make([]*codegen.File, len(root.GRPCServices))
	for i, svc := range root.GRPCServices {
		fw[i] = server(genpkg, svc)
	}
	return fw
}

// server returns the files defining the GRPC server.
func server(genpkg string, svc *grpcdesign.ServiceExpr) *codegen.File {
	path := filepath.Join(codegen.Gendir, "grpc", codegen.SnakeCase(svc.Name()), "server", "server.go")
	data := GRPCServices.Get(svc.Name())
	title := fmt.Sprintf("%s GRPC server", svc.Name())
	sections := []*codegen.SectionTemplate{
		codegen.Header(title, "server", []*codegen.ImportSpec{
			{Path: "context"},
			{Path: genpkg + "/" + codegen.SnakeCase(svc.Name()), Name: data.Service.PkgName},
			{Path: genpkg + "/grpc/" + codegen.SnakeCase(svc.Name()), Name: svc.Name() + "pb"},
		}),
	}

	sections = append(sections, &codegen.SectionTemplate{Name: "server-struct", Source: serverStructT, Data: data})
	sections = append(sections, &codegen.SectionTemplate{Name: "server-init", Source: serverInitT, Data: data})

	for _, e := range data.Endpoints {
		sections = append(sections, &codegen.SectionTemplate{Name: "server-grpc-interface", Source: serverGRPCInterfaceT, Data: e})
	}

	return &codegen.File{Path: path, SectionTemplates: sections}
}

// input: ServiceData
const serverStructT = `{{ printf "%s implements the %s.%s interface." .ServerStruct .PkgName .ServerInterface | comment }}
type {{ .ServerStruct }} struct {
	endpoints *{{ .Service.PkgName }}.Endpoints
}
`

// input: ServiceData
const serverInitT = `{{ printf "%s instantiates the server struct with the %s service endpoints." .ServerInit .Service.Name | comment }}
func {{ .ServerInit }}(e *{{ .Service.PkgName }}.Endpoints) *{{ .ServerStruct }} {
	return &{{ .ServerStruct }}{e}
}
`

// input: EndpointData
const serverGRPCInterfaceT = `{{ printf "%s implements the %s method in %s.%s interface." .Name .Name .PkgName .ServerInterface | comment }}
func (s *{{ .ServerStruct }}) {{ .Name }}(ctx context.Context, p {{ .Request.Ref }}) ({{ .Response.Ref }}, error) {
	payload := {{ .Request.PayloadInit.Name }}({{ range .Request.PayloadInit.Args }}{{ .Name }}{{ end }})
	v, err := s.endpoints.{{ .Name }}(ctx, payload)
	if err != nil {
		return nil, err
	}
	res := v.({{ .Method.ResultRef }})
	resp := {{ .Response.Init.Name }}({{ range .Response.Init.Args }}{{ .Name }}{{ end }})
	return resp, nil
}
`
