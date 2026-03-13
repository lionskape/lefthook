package command

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"

	"github.com/evilmartians/lefthook/v2/tests/helpers/cmdtest"
	"github.com/evilmartians/lefthook/v2/tests/helpers/gittest"
)

func TestRun(t *testing.T) {
	root, err := filepath.Abs("src")
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	gitPath := gittest.GitPath(root)
	configPath := filepath.Join(root, "lefthook.yml")

	for i, tt := range [...]struct {
		name, hook, config string
		gitArgs            []string
		envs               map[string]string
		existingDirs       []string
		error              bool
	}{
		{
			name: "Skip case",
			hook: "pre-commit",
			envs: map[string]string{
				"LEFTHOOK": "0",
			},
			error: false,
		},
		{
			name: "Skip case",
			hook: "pre-commit",
			envs: map[string]string{
				"LEFTHOOK": "false",
			},
			error: false,
		},
		{
			name: "Invalid version",
			hook: "pre-commit",
			config: `
min_version: 23.0.1
`,
			error: true,
		},
		{
			name: "Valid version, no hook",
			hook: "pre-commit",
			config: `
min_version: 0.7.9
`,
			error: false,
		},
		{
			name: "Invalid hook",
			hook: "pre-commit",
			config: `
pre-commit:
  parallel: true
  piped: true
`,
			error: true,
		},
		{
			name: "Valid hook",
			hook: "pre-commit",
			config: `
pre-commit:
  parallel: false
  piped: true
`,
			error: false,
		},
		{
			name: "When in git rebase-merge flow",
			hook: "pre-commit",
			config: `
pre-commit:
  parallel: false
  piped: true
  commands:
    echo:
      skip:
        - rebase
        - merge
      run: echo 'SHOULD NEVER RUN'
`,
			existingDirs: []string{
				filepath.Join(gitPath, "rebase-merge"),
			},
			error: false,
		},
		{
			name: "When in git rebase-apply flow",
			hook: "pre-commit",
			config: `
pre-commit:
  parallel: false
  piped: true
  commands:
    echo:
      skip:
        - rebase
        - merge
      run: echo 'SHOULD NEVER RUN'
`,
			existingDirs: []string{
				filepath.Join(gitPath, "rebase-apply"),
			},
			error: false,
		},
		{
			name: "When not in rebase flow",
			hook: "post-commit",
			config: `
post-commit:
  parallel: false
  piped: true
  commands:
    echo:
      skip:
        - rebase
        - merge
      run: echo 'SHOULD RUN'
`,
			error: true,
		},
	} {
		t.Run(fmt.Sprintf("%d: %s", i, tt.name), func(t *testing.T) {
			assert := assert.New(t)
			fs := afero.NewMemMapFs()
			lefthook := &Lefthook{
				fs:   fs,
				repo: gittest.NewRepositoryBuilder().Cmd(cmdtest.NewDumb()).Fs(fs).Root(root).Build(),
			}
			lefthook.repo.Setup()

			// Create files that should exist
			for _, path := range tt.existingDirs {
				assert.NoError(fs.MkdirAll(path, 0o755))
			}

			assert.NoError(afero.WriteFile(fs, configPath, []byte(tt.config), 0o644))
			for env, value := range tt.envs {
				t.Setenv(env, value)
			}

			err = lefthook.Run(t.Context(), RunArgs{Hook: tt.hook, GitArgs: tt.gitArgs})
			if tt.error {
				assert.Error(err)
			} else {
				assert.NoError(err)
			}
		})
	}
}

func TestRunNoGit(t *testing.T) {
	root, err := filepath.Abs("src")
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	configPath := filepath.Join(root, "lefthook.yml")

	for i, tt := range [...]struct {
		name, hook, config string
		error              bool
	}{
		{
			name: "No git: job with files - valid",
			hook: "lint",
			config: `
lint:
  commands:
    check:
      files: echo file.go
      run: echo {files}
`,
			error: false,
		},
		{
			name: "No git: job without files - invalid",
			hook: "lint",
			config: `
lint:
  commands:
    check:
      run: echo hello
`,
			error: true,
		},
		{
			name: "No git: hook-level files inherited by job - valid",
			hook: "lint",
			config: `
lint:
  files: echo file.go
  commands:
    check:
      run: echo {files}
`,
			error: false,
		},
		{
			name: "No git: job in group with files - valid",
			hook: "lint",
			config: `
lint:
  jobs:
    - group:
        jobs:
          - name: check
            run: echo {files}
            files: echo file.go
`,
			error: false,
		},
		{
			name: "No git: job in group without files - invalid",
			hook: "lint",
			config: `
lint:
  jobs:
    - group:
        jobs:
          - name: check
            run: echo hello
`,
			error: true,
		},
		{
			name: "No git: group-level files inherited by sub-job - valid",
			hook: "lint",
			config: `
lint:
  jobs:
    - files: echo file.go
      group:
        jobs:
          - name: check
            run: echo {files}
`,
			error: false,
		},
	} {
		t.Run(fmt.Sprintf("%d: %s", i, tt.name), func(t *testing.T) {
			assert := assert.New(t)
			fs := afero.NewMemMapFs()
			lefthook := &Lefthook{
				fs:    fs,
				repo:  gittest.NewRepositoryBuilder().Cmd(cmdtest.NewDumb()).Fs(fs).Root(root).Build(),
				noGit: true,
			}
			lefthook.repo.Setup()

			assert.NoError(afero.WriteFile(fs, configPath, []byte(tt.config), 0o644))

			err = lefthook.Run(t.Context(), RunArgs{Hook: tt.hook})
			if tt.error {
				assert.Error(err)
			} else {
				assert.NoError(err)
			}
		})
	}
}
