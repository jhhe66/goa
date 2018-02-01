package codegen

import (
	"path/filepath"

	"goa.design/goa/codegen"
	grpcdesign "goa.design/goa/grpc/design"
)

// ServerTypeFiles returns the gRPC transport type files.
func ServerTypeFiles(genpkg string, root *grpcdesign.RootExpr) []*codegen.File {
	fw := make([]*codegen.File, len(root.GRPCServices))
	seen := make(map[string]struct{})
	for i, r := range root.GRPCServices {
		fw[i] = serverType(genpkg, r, seen)
	}
	return fw
}

// serverType returns the file containing the constructor functions to
// transform the gRPC request types to the corresponding service payload types
// and service result types to the corresponding gRPC response types.
//
// seen keeps track of the constructor names that have already been generated
// to prevent duplicate code generation.
func serverType(genpkg string, svc *grpcdesign.ServiceExpr, seen map[string]struct{}) *codegen.File {
	var (
		path     string
		initData []*InitData

		sd = GRPCServices.Get(svc.Name())
	)
	{
		path = filepath.Join(codegen.Gendir, "grpc", codegen.SnakeCase(svc.Name()), "server", "types.go")
		for _, a := range svc.GRPCEndpoints {
			ed := sd.Endpoint(a.Name())
			if ed.Request != nil && ed.Request.PayloadInit != nil {
				initData = append(initData, ed.Request.PayloadInit)
			}
			if ed.Response != nil && ed.Response.Init != nil {
				initData = append(initData, ed.Response.Init)
			}
		}
	}

	header := codegen.Header(svc.Name()+" gRPC server types", "server",
		[]*codegen.ImportSpec{
			{Path: "unicode/utf8"},
			{Path: genpkg + "/" + codegen.SnakeCase(svc.Name()), Name: sd.Service.PkgName},
			{Path: "goa.design/goa", Name: "goa"},
			{Path: genpkg + "/grpc/" + codegen.SnakeCase(svc.Name()), Name: svc.Name() + "pb"},
		},
	)
	sections := []*codegen.SectionTemplate{header}
	for _, init := range initData {
		sections = append(sections, &codegen.SectionTemplate{
			Name:   "server-type-init",
			Source: typeInitT,
			Data:   init,
		})
	}

	return &codegen.File{Path: path, SectionTemplates: sections}
}

// input: InitData
const typeInitT = `{{ comment .Description }}
func {{ .Name }}({{ range .Args }}{{ .Name }} {{.TypeRef }}, {{ end }}) {{ .ReturnTypeRef }} {
  {{ .Code }}
  return {{ .ReturnVarName }}
}
`
