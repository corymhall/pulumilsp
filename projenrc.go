package main

import (
	"github.com/corymhall/pulumilsp/projenrc"
	"github.com/projen/projen-go/projen"
	"github.com/projen/projen-go/projen/build"
	"github.com/projen/projen-go/projen/github"
	"github.com/projen/projen-go/projen/github/workflows"
)

func main() {
	project := projen.NewProject(&projen.ProjectOptions{
		Name: projenrc.StrPtr("pulumilsp"),
		GitIgnoreOptions: &projen.IgnoreFileOptions{
			IgnorePatterns: &[]*string{
				projenrc.StrPtr("bin"),
				projenrc.StrPtr("dist"),
				projenrc.StrPtr(".DS_Store"),
			},
		},
	})
	project.DefaultTask().Exec(projenrc.StrPtr("go run projenrc.go"), &projen.TaskStepOptions{})

	vscode := projenrc.NewVscodeProject(project)

	packageVsceTask := project.AddTask(projenrc.StrPtr("package-vsce"), &projen.TaskOptions{
		RequiredEnv: &[]*string{projenrc.StrPtr("PLATFORM"), projenrc.StrPtr("ARCH")},
		Cwd:         projenrc.StrPtr("./editors/vscode"),
		Steps: &[]*projen.TaskStep{
			{Exec: projenrc.StrPtr("mkdir -p ./dist")},
			{Exec: projenrc.StrPtr("npx vsce package --target \"$PLATFORM-$ARCH\" --out ../../dist/ $VERSION")},
		},
	})

	packageGoTask := project.AddTask(projenrc.StrPtr("package-go"), &projen.TaskOptions{
		RequiredEnv: &[]*string{projenrc.StrPtr("GOOS"), projenrc.StrPtr("GOARCH"), projenrc.StrPtr("VERSION")},
		Env: &map[string]*string{
			"CGO_ENABLED": projenrc.StrPtr("1"),
		},
		Steps: &[]*projen.TaskStep{
			{Exec: projenrc.StrPtr("go build -o ./bin/$GOOS-$GOARCH/pulumilsp -ldflags \"-s -w -extldflags '-static'\" ./cmd/pulumilsp")},
			{Exec: projenrc.StrPtr("mkdir -p ./dist")},
			{Exec: projenrc.StrPtr("tar --gzip -cf ./dist/pulumilsp-$VERSION-$GOOS-$GOARCH.tar.gz README.md LICENSE -C ./bin/$GOOS-$GOARCH .")},
		},
	})

	gh := github.NewGitHub(project, &github.GitHubOptions{})
	projenrc.NewGitHubReleaseWorkflow(project, gh, packageVsceTask, packageGoTask)
	buildWorkflow := build.NewBuildWorkflow(project, &build.BuildWorkflowOptions{
		BuildTask: project.BuildTask(),
		PreBuildSteps: &[]*workflows.JobStep{
			projenrc.Workflows_SetupGo(),
			projenrc.Workflows_SetupNode(),
		},
		MutableBuild: projenrc.BoolPtr(false),
	})
	buildPermissions := &workflows.JobPermissions{
		Contents: workflows.JobPermission_READ,
	}
	buildWorkflow.AddPostBuildJob(projenrc.StrPtr("package-vsce"), projenrc.VscePackageWorkflow(
		gh, project.DefaultTask(), packageVsceTask, false, nil, nil, buildPermissions, nil,
	))
	buildWorkflow.AddPostBuildJob(projenrc.StrPtr("package-go"), projenrc.GoPackageWorkflow(
		gh, project.DefaultTask(), packageGoTask, false, nil, nil, buildPermissions, nil,
	))

	vscodePackageTask := project.AddTask(projenrc.StrPtr("package:vscode"), &projen.TaskOptions{
		Steps: &[]*projen.TaskStep{
			{
				Exec: projenrc.StrPtr("npx projen package"),
				Cwd:  projenrc.StrPtr("./editors/vscode"),
			},
		},
	})
	project.PackageTask().Exec(projenrc.StrPtr("go build -o bin/pulumilsp -ldflags \"-s -w\" ./cmd/pulumilsp"), &projen.TaskStepOptions{})
	project.PackageTask().Spawn(vscodePackageTask, &projen.TaskStepOptions{})

	project.Synth()
	vscode.Synth()
}
