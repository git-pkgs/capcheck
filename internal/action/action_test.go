package action_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type actionStep struct {
	Name string
	Run  string
	Env  map[string]string
}

func loadStep(t *testing.T, name string) actionStep {
	t.Helper()
	data, err := os.ReadFile("../../action.yml")
	if err != nil {
		t.Fatal(err)
	}
	var action struct {
		Runs struct{ Steps []actionStep }
	}
	if err := yaml.Unmarshal(data, &action); err != nil {
		t.Fatal(err)
	}
	for _, step := range action.Runs.Steps {
		if step.Name == name {
			return step
		}
	}
	t.Fatalf("missing action step %q", name)
	return actionStep{}
}

func runStep(t *testing.T, step actionStep, inputs map[string]string, code int) ([]string, string, int) {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is required to run the composite action")
	}
	dir := t.TempDir()
	stub := "#!/usr/bin/env bash\nprintf '%s\\0' \"$@\" > \"$ARGS_FILE\"\nexit \"$STUB_EXIT\"\n"
	for _, name := range []string{"go", "capcheck"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(stub), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "glob.go"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	argsFile := filepath.Join(dir, "args")
	outputFile := filepath.Join(dir, "output")
	marker := filepath.Join(dir, "injected")
	cmd := exec.Command(bash, "--noprofile", "--norc", "-eo", "pipefail", "-c", step.Run)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"ARGS_FILE="+argsFile, "STUB_EXIT="+strconv.Itoa(code), "MARKER="+marker,
		"GITHUB_OUTPUT="+outputFile, "GITHUB_STEP_SUMMARY="+outputFile,
	)
	for key, expression := range step.Env {
		value, ok := inputs[expression]
		if !ok {
			t.Fatalf("no input for %q", expression)
		}
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	if strings.Contains(step.Run, "${{") {
		t.Fatal("action expressions must not be interpolated into shell scripts")
	}
	output, err := cmd.CombinedOutput()
	status := 0
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("run action: %v\n%s", err, output)
		}
		status = exit.ExitCode()
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("input executed as shell code: %v", err)
	}
	data, err := os.ReadFile(argsFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	var args []string
	if len(data) > 0 {
		args = strings.Split(strings.TrimSuffix(string(data), "\x00"), "\x00")
	}
	result, err := os.ReadFile(outputFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return args, string(result), status
}

func TestActionInstallTreatsVersionAsOneArgument(t *testing.T) {
	version := `latest; touch "$MARKER"; $(touch "$MARKER")`
	args, _, status := runStep(t, loadStep(t, "Install capcheck"), map[string]string{
		"${{ inputs.capcheck-version }}": version,
	}, 0)
	want := []string{"install", "github.com/git-pkgs/capcheck/cmd/capcheck@" + version}
	if status != 0 || !reflect.DeepEqual(args, want) {
		t.Fatalf("exit=%d args=%q, want %q", status, args, want)
	}
}

func TestActionCheckPreservesInputsAndExitCode(t *testing.T) {
	config := `path with spaces/$(touch "$MARKER").json`
	baseline := "baseline/`touch \"$MARKER\"`.json"
	ignore := `READ_FILE; touch "$MARKER"`
	for _, packages := range []struct {
		name, input string
		want        []string
	}{
		{"default", "./...", []string{"./..."}},
		{"multiple", "./cmd/...\n\t./internal/...", []string{"./cmd/...", "./internal/..."}},
		{"literal", "*.go $(touch${IFS}$MARKER) -unexpected", []string{"*.go", "$(touch${IFS}$MARKER)", "-unexpected"}},
	} {
		t.Run(packages.name, func(t *testing.T) {
			for _, code := range []int{0, 1, 2} {
				args, output, status := runStep(t, loadStep(t, "Run capcheck"), map[string]string{
					"${{ inputs.config }}": config, "${{ inputs.baseline }}": baseline,
					"${{ inputs.ignore }}": ignore, "${{ inputs.strict }}": "true",
					"${{ inputs.packages }}": packages.input,
				}, code)
				want := append([]string{"check", "--format", "github", "--config", config, "--baseline", baseline, "--strict", "--ignore", ignore, "--"}, packages.want...)
				if status != 0 || output != "exit-code="+strconv.Itoa(code)+"\n" || !reflect.DeepEqual(args, want) {
					t.Fatalf("exit=%d output=%q args=%q, want %q", status, output, args, want)
				}
			}
		})
	}
}

func TestActionSummaryTreatsInputsLiterally(t *testing.T) {
	config := `$(touch "$MARKER")`
	args, output, status := runStep(t, loadStep(t, "Job summary"), map[string]string{
		"${{ inputs.config }}": config, "${{ inputs.baseline }}": "",
		"${{ inputs.packages }}": "./...\n*.go",
	}, 2)
	want := []string{"check", "--config", config, "--", "./...", "*.go"}
	if status != 0 || !strings.Contains(output, "### capcheck") || !reflect.DeepEqual(args, want) {
		t.Fatalf("exit=%d output=%q args=%q, want %q", status, output, args, want)
	}
}

func TestActionFailsWithCheckExitCode(t *testing.T) {
	for _, code := range []int{1, 2} {
		_, _, status := runStep(t, loadStep(t, "Fail if changed"), map[string]string{
			"${{ steps.check.outputs.exit-code }}": strconv.Itoa(code),
		}, 0)
		if status != code {
			t.Fatalf("exit=%d, want %d", status, code)
		}
	}
}
