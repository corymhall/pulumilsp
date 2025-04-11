package projenrc

import (
	"github.com/projen/projen-go/projen"
	"github.com/projen/projen-go/projen/github"
	"github.com/projen/projen-go/projen/github/workflows"
	"github.com/projen/projen-go/projen/release"
)

func Workflows_SetupNode() *workflows.JobStep {
	return &workflows.JobStep{
		Uses: StrPtr("actions/setup-node@v4"),
		With: &map[string]any{
			"node-version": "20.x",
		},
	}
}

func Workflows_SetupGo() *workflows.JobStep {
	return &workflows.JobStep{
		Uses: StrPtr("actions/setup-go@v5"),
		With: &map[string]any{
			"cache-dependency-path": "go.sum",
			"go-version-file":       "go.mod",
		},
	}
}

func NewGitHubReleaseWorkflow(
	project projen.Project,
	gh github.GitHub,
	packageVsceTask projen.Task,
	packageGoTask projen.Task,
) release.Release {
	ghRelease := release.NewRelease(gh, &release.ReleaseOptions{
		PostBuildSteps: &[]*workflows.JobStep{
			{
				Name: StrPtr("Get Version"),
				Id:   StrPtr("get_version"),
				Run:  StrPtr("cat dist/releasetag.txt >> $GITHUB_OUTPUT"),
			},
		},
		ReleaseWorkflowSetupSteps: &[]*workflows.JobStep{
			Workflows_SetupGo(),
			Workflows_SetupNode(),
			{Run: StrPtr("yarn install --check-files --frozen-lockfile")},
		},
		ArtifactsDirectory: StrPtr("dist"),
		Branch:             StrPtr("main"),
		Task:               project.PackageTask(),
		VersionFile:        StrPtr("editors/vscode/package.json"),
	})
	project.TryFindObjectFile(StrPtr(".github/workflows/release.yml")).
		AddOverride(StrPtr("jobs.release.outputs.version"), "${{ steps.get_version.outputs.version }}")

	ifCond := "needs.release.outputs.tag_exists != 'true' && needs.release.outputs.latest_commit == github.sha"
	needs := []*string{StrPtr("release"), StrPtr("release_github")}
	permissions := &workflows.JobPermissions{
		Contents: workflows.JobPermission_WRITE,
	}
	env := &map[string]*string{
		"VERSION": StrPtr("needs.release.outputs.version"),
	}
	vscePackageJob := VscePackageWorkflow(
		gh,
		project.DefaultTask(),
		packageVsceTask,
		true,
		&ifCond,
		&needs,
		permissions,
		env,
	)
	packageGoJob := GoPackageWorkflow(
		gh,
		project.DefaultTask(),
		packageGoTask,
		true,
		&ifCond,
		&needs,
		permissions,
		env,
	)

	updateReleaseJob := UpdateReleaseJob(gh, ifCond, permissions)
	ghRelease.AddJobs(&map[string]*workflows.Job{
		"package-vsce":   vscePackageJob,
		"package-go":     packageGoJob,
		"update-release": updateReleaseJob,
	})
	return ghRelease
}

func VscePackageWorkflow(
	gh github.GitHub,
	defaultTask projen.Task,
	packageVsceTask projen.Task,
	withUploadArtifact bool,
	ifCond *string,
	needs *[]*string,
	permissions *workflows.JobPermissions,
	env *map[string]*string,
) *workflows.Job {
	steps := []*workflows.JobStep{
		github.WorkflowSteps_Checkout(&github.CheckoutOptions{}),
		Workflows_SetupNode(),
		{
			Name: StrPtr("Install Deps"),
			Run:  StrPtr("cd editors/vscode && yarn install --check-files --frozen-lockfile"),
		},
		{
			Name: StrPtr("Package vsce"),
			Run:  gh.Project().RunTaskCommand(packageVsceTask),
			Env: &map[string]*string{
				"PLATFORM": StrPtr("${{ matrix.platform }}"),
				"ARCH":     StrPtr("${{ matrix.arch }}"),
				"VERSION":  StrPtr("${{ env.VERSION }}"),
			},
		},
	}
	if withUploadArtifact {
		steps = append(steps, github.WorkflowSteps_UploadArtifact(&github.UploadArtifactOptions{
			With: &github.UploadArtifactWith{
				Name: StrPtr("pulumilsp-client-${{ matrix.platform }}-${{ matrix.arch }}-${{ github.ref_name }}"),
				Path: StrPtr("./dist/pulumilsp-client-${{ matrix.platform }}-${{ matrix.arch }}-${{ github.ref_name }}.vsix"),
			},
		}))
	}

	return &workflows.Job{
		If:          ifCond,
		Needs:       needs,
		Permissions: permissions,
		Env:         env,
		RunsOn:      &[]*string{StrPtr("ubuntu-latest")},
		Strategy: &workflows.JobStrategy{
			Matrix: &workflows.JobMatrix{
				Include: &[]*map[string]any{
					{"platform": "linux", "arch": "x64"},
					{"platform": "linux", "arch": "arm64"},
					{"platform": "darwin", "arch": "x64"},
					{"platform": "darwin", "arch": "arm64"},
					{"platform": "win32", "arch": "x64"},
				},
			},
		},
		Steps: &steps,
	}
}

func GoPackageWorkflow(
	gh github.GitHub,
	defaultTask projen.Task,
	packageGoTask projen.Task,
	withUploadArtifact bool,
	ifCond *string,
	needs *[]*string,
	permissions *workflows.JobPermissions,
	env *map[string]*string,
) *workflows.Job {
	steps := []*workflows.JobStep{
		github.WorkflowSteps_Checkout(&github.CheckoutOptions{}),
		Workflows_SetupGo(),
		Workflows_SetupNode(),
		{
			Name: StrPtr("Package Go"),
			Run:  gh.Project().RunTaskCommand(packageGoTask),
			Env: &map[string]*string{
				"GOOS":    StrPtr("${{ matrix.platform }}"),
				"GOARCH":  StrPtr("${{ matrix.arch }}"),
				"VERSION": StrPtr("${{ env.VERSION }}"),
			},
		},
	}
	if withUploadArtifact {
		steps = append(steps, github.WorkflowSteps_UploadArtifact(&github.UploadArtifactOptions{
			With: &github.UploadArtifactWith{
				Name: StrPtr("pulumilsp-${{ github.ref_name }}-${{ matrix.platform }}-${{ matrix.arch }}.tar.gz"),
				Path: StrPtr("./dist/pulumilsp-${{ github.ref_name }}-${{ matrix.platform }}-${{ matrix.arch }}.tar.gz"),
			},
		}))
	}
	return &workflows.Job{
		If:          ifCond,
		Needs:       needs,
		Permissions: permissions,
		Env:         env,
		RunsOn:      &[]*string{StrPtr("${{ matrix.os }}")},
		Strategy: &workflows.JobStrategy{
			Matrix: &workflows.JobMatrix{
				Include: &[]*map[string]any{
					{"platform": "linux", "arch": "amd64", "os": "ubuntu-latest"},
					{"platform": "darwin", "arch": "amd64", "os": "macos-latest"},
					{"platform": "darwin", "arch": "arm64", "os": "macos-latest"},
					// TODO: get cross compilation working for these
					// {"platform": "linux", "arch": "arm64", "os": "ubuntu-latest"},
					// {"platform": "windows", "arch": "amd64", "os": "windows-latest"},
				},
			},
		},
		Steps: &steps,
	}
}

func UpdateReleaseJob(
	gh github.GitHub,
	ifCond string,
	permissions *workflows.JobPermissions,
) *workflows.Job {
	return &workflows.Job{
		Permissions: permissions,
		If:          &ifCond,
		Needs:       &[]*string{StrPtr("package-vsce"), StrPtr("package-go"), StrPtr("release")},
		RunsOn:      &[]*string{StrPtr("ubuntu-latest")},
		Env: &map[string]*string{
			"VERSION": StrPtr("needs.release.outputs.version"),
		},
		Steps: &[]*workflows.JobStep{
			github.WorkflowSteps_Checkout(&github.CheckoutOptions{}),
			github.WorkflowSteps_DownloadArtifact(&github.DownloadArtifactOptions{
				With: &github.DownloadArtifactWith{
					MergeMultiple: BoolPtr(true),
					Path:          StrPtr("dist"),
					Pattern:       StrPtr("pulumilsp-*"),
				},
			}),
			{
				Name: StrPtr("Upload Release"),
				Run:  StrPtr("gh release upload $VERSION dist/*"),
				Env: &map[string]*string{
					"VERSION": StrPtr("${{ env.VERSION }}"),
				},
			},
		},
	}
}
