package main

import (
	"fmt"

	"github.com/projen/projen-go/projen"
	"github.com/projen/projen-go/projen/javascript"
	"github.com/projen/projen-go/projen/typescript"
)

func strPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

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

func main() {
	project := projen.NewProject(&projen.ProjectOptions{
		Name: strPtr("pulumilsp"),
		GitIgnoreOptions: &projen.IgnoreFileOptions{
			IgnorePatterns: &[]*string{strPtr("bin")},
		},
	})
	project.DefaultTask().Exec(strPtr("go run projenrc.go"), &projen.TaskStepOptions{})

	vscode := typescript.NewTypeScriptProject(&typescript.TypeScriptProjectOptions{
		DefaultReleaseBranch: strPtr("main"),
		Outdir:               strPtr("editors/vscode"),
		Parent:               project,
		Prettier:             boolPtr(true),
		PrettierOptions: &javascript.PrettierOptions{
			Settings: &javascript.PrettierSettings{
				SingleQuote: boolPtr(true),
			},
		},
		Description: strPtr("Pulumi Language Server Protocol (LSP) extension for Visual Studio Code"),
		Repository:  strPtr("https://github.com/corymhall/pulumilsp"),
		EslintOptions: &javascript.EslintOptions{
			Dirs:     &[]*string{},
			Prettier: boolPtr(true),
		},
		Name:       strPtr("pulumilsp-client"),
		AuthorName: strPtr("corymhall"),
		Deps:       &[]*string{strPtr("vscode-languageclient")},
		DevDeps:    &[]*string{strPtr("@types/vscode"), strPtr("@vscode/vsce")},
	})
	vscode.Gitignore().AddPatterns(strPtr("pulumilsp"))
	vscode.Package().AddField(strPtr("main"), "assets/extension/index.js")
	bundle := vscode.Bundler().AddBundle(strPtr("src/extension.ts"), &javascript.AddBundleOptions{
		Platform:  strPtr("node"),
		Target:    strPtr("node16"),
		Externals: &[]*string{strPtr("vscode")},
		Minify:    boolPtr(true),
	})

	projen.NewIgnoreFile(vscode, strPtr(".vscodeignore"), &projen.IgnoreFileOptions{
		IgnorePatterns: &[]*string{
			strPtr("node_modules"),
			strPtr("!assets/extension/index.js"),
			strPtr("!pulumilsp"),
			strPtr("!README.md"),
			strPtr("!LICENSE"),
			strPtr("!package.json"),
			strPtr("**/*"),
		},
	})

	vscode.AddScripts(&map[string]*string{
		"vscode:prepublish": strPtr(fmt.Sprintf("npx projen %s", *bundle.BundleTask.Name())),
	})

	vscode.PackageTask().Reset(strPtr("npx vsce package --out ../../bin/"), &projen.TaskStepOptions{})
	vscode.Package().AddField(strPtr("activationEvents"), []string{
		"workspaceContains:**/Pulumi.yaml",
	})
	vscode.Package().AddField(strPtr("engines"), map[string]any{
		"vscode": "^1.99.1",
	})
	vscode.Package().AddField(strPtr("contributes"), Contributes{
		Configuration: Configuration{
			Title: "Pulumi Diagnostics LSP",
			Properties: map[string]Property{
				"pulumilsp.server.path": {
					Type:                []string{"string", "null"},
					Default:             nil,
					MarkdownDescription: "Specifies the path to the `pulumilsp` binary to use. Leave as `null` to use the binary bundled with the downloaded extension.",
				},
			},
		},
	})

	project.PackageTask().Exec(strPtr("go build -o bin/pulumilsp -ldflags \"-s -w\" ./cmd/pulumilsp"), &projen.TaskStepOptions{})

	vscodePackageTask := project.AddTask(strPtr("package:vscode"), &projen.TaskOptions{
		Steps: &[]*projen.TaskStep{
			{
				Exec: strPtr(fmt.Sprintf("cp ./bin/pulumilsp ./editors/vscode/")),
			},
			{
				Exec: strPtr("npx projen package"),
				Cwd:  strPtr("./editors/vscode"),
			},
		},
	})

	project.PackageTask().Spawn(vscodePackageTask, &projen.TaskStepOptions{})

	project.Synth()
	vscode.Synth()
}
