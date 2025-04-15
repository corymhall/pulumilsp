package projenrc

import (
	"fmt"

	"github.com/projen/projen-go/projen"
	"github.com/projen/projen-go/projen/javascript"
	"github.com/projen/projen-go/projen/typescript"
)

type Contributes struct {
	Configuration Configuration `json:"configuration"`
}

// Configuration represents the configuration for the Pulumi LSP.
type Configuration struct {
	Title      string              `json:"title"`
	Properties map[string]Property `json:"properties"`
}

// Property represents a property in the configuration.
type Property struct {
	Type                []string `json:"type"`
	Default             *string  `json:"default"`
	MarkdownDescription string   `json:"markdownDescription"`
}

func NewVscodeProject(project projen.Project) typescript.TypeScriptProject {
	vscode := typescript.NewTypeScriptProject(&typescript.TypeScriptProjectOptions{
		DefaultReleaseBranch: StrPtr("main"),
		Outdir:               StrPtr("editors/vscode"),
		SampleCode:           BoolPtr(false),
		Parent:               project,
		Prettier:             BoolPtr(true),
		PrettierOptions: &javascript.PrettierOptions{
			Settings: &javascript.PrettierSettings{
				SingleQuote: BoolPtr(true),
			},
		},
		Description: StrPtr("Pulumi Language Server Protocol (LSP) extension for Visual Studio Code"),
		Repository:  StrPtr("https://github.com/corymhall/pulumilsp"),
		EslintOptions: &javascript.EslintOptions{
			Dirs:     &[]*string{},
			Prettier: BoolPtr(true),
		},
		Name:       StrPtr("pulumilsp-client"),
		AuthorName: StrPtr("corymhall"),
		Deps:       &[]*string{StrPtr("vscode-languageclient")},
		DevDeps:    &[]*string{StrPtr("@types/vscode"), StrPtr("@vscode/vsce")},
	})

	vscode.Gitignore().AddPatterns(StrPtr("pulumilsp"))
	vscode.Package().AddField(StrPtr("main"), "assets/extension/index.js")
	bundle := vscode.Bundler().AddBundle(StrPtr("src/extension.ts"), &javascript.AddBundleOptions{
		Platform:  StrPtr("node"),
		Target:    StrPtr("node16"),
		Externals: &[]*string{StrPtr("vscode")},
		Minify:    BoolPtr(true),
	})

	projen.NewIgnoreFile(vscode, StrPtr(".vscodeignore"), &projen.IgnoreFileOptions{
		IgnorePatterns: &[]*string{
			StrPtr("node_modules"),
			StrPtr("!assets/extension/index.js"),
			StrPtr("!pulumilsp"),
			StrPtr("!README.md"),
			StrPtr("!LICENSE"),
			StrPtr("!package.json"),
			StrPtr("**/*"),
		},
	})

	vscode.AddScripts(&map[string]*string{
		"vscode:prepublish": StrPtr(fmt.Sprintf("npx projen %s", *bundle.BundleTask.Name())),
	})

	vscode.PackageTask().Reset(StrPtr("npx vsce package --out ../../bin/"), &projen.TaskStepOptions{})
	vscode.Package().AddField(StrPtr("activationEvents"), []string{
		"workspaceContains:**/Pulumi.yaml",
	})
	vscode.Package().AddField(StrPtr("engines"), map[string]any{
		"vscode": "^1.99.1",
	})
	vscode.Package().AddField(StrPtr("contributes"), Contributes{
		Configuration: Configuration{
			Title: "Pulumi Diagnostics LSP",
			Properties: map[string]Property{
				"pulumilsp.logLevel": {
					Type:                []string{"string"},
					Default:             StrPtr("info"),
					MarkdownDescription: "The log level for the Pulumi LSP. Can be one of 'debug', 'info', 'warn', or 'error'.",
				},
			},
		},
	})
	return vscode

}
