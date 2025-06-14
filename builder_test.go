package imagebuilder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/assert"

	"github.com/containerd/platforms"
	"github.com/openshift/imagebuilder/dockerfile/parser"
)

func TestVolumeSet(t *testing.T) {
	testCases := []struct {
		inputs    []string
		changed   []bool
		result    []string
		covered   []string
		uncovered []string
	}{
		{
			inputs:  []string{"/var/lib", "/var"},
			changed: []bool{true, true},
			result:  []string{"/var"},

			covered:   []string{"/var/lib", "/var/", "/var"},
			uncovered: []string{"/var1", "/", "/va"},
		},
		{
			inputs:  []string{"/var", "/", "/"},
			changed: []bool{true, true, false},
			result:  []string{""},

			covered: []string{"/var/lib", "/var/", "/var", "/"},
		},
		{
			inputs:  []string{"/var", "/var/lib"},
			changed: []bool{true, false},
			result:  []string{"/var"},
		},
	}
	for i, testCase := range testCases {
		s := VolumeSet{}
		for j, path := range testCase.inputs {
			if s.Add(path) != testCase.changed[j] {
				t.Errorf("%d: adding %d %s should have resulted in change %t", i, j, path, testCase.changed[j])
			}
		}
		if !reflect.DeepEqual(testCase.result, []string(s)) {
			t.Errorf("%d: got %v", i, s)
		}
		for _, path := range testCase.covered {
			if !s.Covers(path) {
				t.Errorf("%d: not covered %s", i, path)
			}
		}
		for _, path := range testCase.uncovered {
			if s.Covers(path) {
				t.Errorf("%d: covered %s", i, path)
			}
		}
	}
}

func TestByTarget(t *testing.T) {
	n, err := ParseFile("dockerclient/testdata/Dockerfile.target")
	if err != nil {
		t.Fatal(err)
	}
	stages, err := NewStages(n, NewBuilder(nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(stages) != 4 {
		t.Fatalf("expected 4 stages, got %d", len(stages))
	}
	t.Logf("stages: %#v", stages)

	stages1, found := stages.ByTarget("mytarget")
	if !found {
		t.Fatal("First target not found")
	}
	if len(stages1) != 1 {
		t.Fatalf("expected 1 stages, got %d", len(stages1))
	}
	t.Logf("stages1: %#v", stages1)

	stages2, found := stages.ByTarget("mytarget2")
	if !found {
		t.Fatal("Second target not found")
	}
	if len(stages2) != 1 {
		t.Fatalf("expected 1 stages, got %d", len(stages2))
	}
	t.Logf("stages2: %#v", stages2)

	stages3, found := stages.ByTarget("1")
	if !found {
		t.Fatal("Third target not found")
	}
	if len(stages3) != 1 {
		t.Fatalf("expected 1 stages, got %d", len(stages3))
	}
	t.Logf("stages3: %#v", stages3)
	assert.Equal(t, stages3, stages1)

	stages4, found := stages.ByTarget("2")
	if !found {
		t.Fatal("Fourth target not found")
	}
	if len(stages4) != 1 {
		t.Fatalf("expected 1 stages, got %d", len(stages4))
	}
	t.Logf("stages4: %#v", stages4)
	assert.Equal(t, stages4, stages2)

	stages5, found := stages.ByTarget("mytarget3")
	if !found {
		t.Fatal("Fifth target not found")
	}
	if len(stages5) != 1 {
		t.Fatalf("expected 1 stages, got %d", len(stages5))
	}
	t.Logf("stages5: %#v", stages5)

	stages6, found := stages.ByTarget("3")
	if !found {
		t.Fatal("Sixth target not found")
	}
	if len(stages6) != 1 {
		t.Fatalf("expected 1 stages, got %d", len(stages4))
	}
	t.Logf("stages6: %#v", stages6)
	assert.Equal(t, stages6, stages5)
}

func TestThroughTarget(t *testing.T) {
	n, err := ParseFile("dockerclient/testdata/Dockerfile.target")
	if err != nil {
		t.Fatal(err)
	}
	stages, err := NewStages(n, NewBuilder(nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(stages) != 4 {
		t.Fatalf("expected 4 stages, got %d", len(stages))
	}
	t.Logf("stages: %#v", stages)

	stages1, found := stages.ThroughTarget("mytarget")
	if !found {
		t.Fatal("First target not found")
	}
	if len(stages1) != 2 {
		t.Fatalf("expected 2 stages, got %d", len(stages1))
	}
	t.Logf("stages1: %#v", stages1)

	stages2, found := stages.ThroughTarget("mytarget2")
	if !found {
		t.Fatal("Second target not found")
	}
	if len(stages2) != 3 {
		t.Fatalf("expected 3 stages, got %d", len(stages2))
	}
	t.Logf("stages2: %#v", stages2)

	stages3, found := stages.ThroughTarget("1")
	if !found {
		t.Fatal("Third target not found")
	}
	if len(stages3) != 2 {
		t.Fatalf("expected 2 stages, got %d", len(stages3))
	}
	t.Logf("stages3: %#v", stages3)
	assert.Equal(t, stages3, stages1)

	stages4, found := stages.ThroughTarget("2")
	if !found {
		t.Fatal("Fourth target not found")
	}
	if len(stages4) != 3 {
		t.Fatalf("expected 3 stages, got %d", len(stages4))
	}
	t.Logf("stages4: %#v", stages4)
	assert.Equal(t, stages4, stages2)

	stages5, found := stages.ThroughTarget("mytarget3")
	if !found {
		t.Fatal("Fifth target not found")
	}
	if len(stages5) != 4 {
		t.Fatalf("expected 4 stages, got %d", len(stages5))
	}
	t.Logf("stages5: %#v", stages5)

	stages6, found := stages.ThroughTarget("3")
	if !found {
		t.Fatal("Sixth target not found")
	}
	if len(stages6) != 4 {
		t.Fatalf("expected 4 stages, got %d", len(stages4))
	}
	t.Logf("stages6: %#v", stages6)
	assert.Equal(t, stages6, stages5)
}

func TestMultiStageParse(t *testing.T) {
	n, err := ParseFile("dockerclient/testdata/multistage/Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	stages, err := NewStages(n, NewBuilder(nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(stages) != 3 {
		t.Fatalf("expected 3 stages, got %d", len(stages))
	}
	t.Logf("stages: %#v", stages)
}

func TestMultiStageParseHeadingArg(t *testing.T) {
	n, err := ParseFile("dockerclient/testdata/multistage/Dockerfile.heading-arg")
	if err != nil {
		t.Fatal(err)
	}
	stages, err := NewStages(n, NewBuilder(map[string]string{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(stages) != 3 {
		t.Fatalf("expected 3 stages, got %d", len(stages))
	}

	fromImages := []string{"mirror.gcr.io/golang:1.24", "mirror.gcr.io/busybox:latest", "mirror.gcr.io/golang:1.24"}
	for i, stage := range stages {
		from, err := stage.Builder.From(stage.Node)
		if err != nil {
			t.Fatal(err)
		}

		if expected := fromImages[i]; from != expected {
			t.Fatalf("expected %s, got %s", expected, from)
		}
	}

	t.Logf("stages: %#v", stages)
}

func TestHeadingArg(t *testing.T) {
	for _, tc := range []struct {
		name         string
		args         map[string]string
		expectedFrom string
	}{
		{name: "default", args: map[string]string{}, expectedFrom: "busybox:latest"},
		{name: "override", args: map[string]string{"FOO": "bar"}, expectedFrom: "busybox:bar"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n, err := ParseDockerfile(strings.NewReader(`ARG FOO=latest
ARG BAR=baz
FROM busybox:$FOO
ARG BAZ=banana
RUN echo $FOO $BAR`))
			if err != nil {
				t.Fatal(err)
			}
			b := NewBuilder(tc.args)
			from, err := b.From(n)
			if err != nil {
				t.Fatal(err)
			}
			if from != tc.expectedFrom {
				t.Fatalf("expected %s, got %s", tc.expectedFrom, from)
			}
		})
	}
}

// Test if `FROM some-${SOME-BUILT-IN-ARG}` args gets resolved correctly.
func TestArgResolutionOfDefaultVariables(t *testing.T) {
	// Get architecture from host
	var localspec = platforms.DefaultSpec()
	for _, tc := range []struct {
		dockerfile   string
		name         string
		args         map[string]string
		expectedFrom string
	}{
		{name: "use-default-built-arg",
			dockerfile:   "FROM platform-${TARGETARCH}",
			args:         map[string]string{"FOO": "bar"},
			expectedFrom: "platform-" + localspec.Architecture},
		// Override should not work since we did not declare
		{name: "override-default-built-arg-without-declaration",
			dockerfile:   "FROM platform-${TARGETARCH}",
			args:         map[string]string{"TARGETARCH": "bar"},
			expectedFrom: "platform-" + localspec.Architecture},
		{name: "override-default-built-arg",
			dockerfile:   "ARG TARGETARCH\nFROM platform-${TARGETARCH}",
			args:         map[string]string{"TARGETARCH": "bar"},
			expectedFrom: "platform-bar"},
		{name: "random-built-arg",
			dockerfile:   "ARG FOO\nFROM ${FOO}",
			args:         map[string]string{"FOO": "bar"},
			expectedFrom: "bar"},
		// Arg should not be resolved since we did not declare
		{name: "random-built-arg-without-declaration",
			dockerfile:   "FROM ${FOO}",
			args:         map[string]string{"FOO": "bar"},
			expectedFrom: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n, err := ParseDockerfile(strings.NewReader(tc.dockerfile))
			if err != nil {
				t.Fatal(err)
			}
			stages, err := NewStages(n, NewBuilder(tc.args))
			if err != nil {
				t.Fatal(err)
			}
			from, err := stages[0].Builder.From(n)
			if err != nil {
				t.Fatal(err)
			}
			if from != tc.expectedFrom {
				t.Fatalf("expected %s, got %s", tc.expectedFrom, from)
			}
		})
	}
}

func resolveNodeArgs(b *Builder, node *parser.Node) error {
	for _, c := range node.Children {
		if c.Value != "arg" {
			continue
		}
		step := b.Step()
		if err := step.Resolve(c); err != nil {
			return err
		}
		if err := b.Run(step, NoopExecutor, false); err != nil {
			return err
		}
	}
	return nil
}

func builderHasArgument(b *Builder, argString string) bool {
	for _, arg := range b.Arguments() {
		if arg == argString {
			return true
		}
	}
	return false
}

func TestMultiStageHeadingArgRedefine(t *testing.T) {
	n, err := ParseFile("dockerclient/testdata/multistage/Dockerfile.heading-redefine")
	if err != nil {
		t.Fatal(err)
	}
	stages, err := NewStages(n, NewBuilder(map[string]string{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(stages) != 2 {
		t.Fatalf("expected 2 stages, got %d", len(stages))
	}

	for _, stage := range stages {
		if err := resolveNodeArgs(stage.Builder, stage.Node); err != nil {
			t.Fatal(err)
		}
	}

	firstStageHasArg := false
	for _, arg := range stages[0].Builder.Arguments() {
		if match, err := regexp.MatchString(`FOO=.*`, arg); err == nil && match {
			firstStageHasArg = true
			break
		} else if err != nil {
			t.Fatal(err)
		}
	}
	if firstStageHasArg {
		t.Fatalf("expected FOO to not be present in first stage")
	}

	if !builderHasArgument(stages[1].Builder, "FOO=latest") {
		t.Fatalf("expected FOO=latest in second stage arguments list, got %v", stages[1].Builder.Arguments())
	}
}

func TestMultiStageHeadingArgRedefineOverride(t *testing.T) {
	n, err := ParseFile("dockerclient/testdata/multistage/Dockerfile.heading-redefine")
	if err != nil {
		t.Fatal(err)
	}
	stages, err := NewStages(n, NewBuilder(map[string]string{"FOO": "7"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(stages) != 2 {
		t.Fatalf("expected 2 stages, got %d", len(stages))
	}

	for _, stage := range stages {
		if err := resolveNodeArgs(stage.Builder, stage.Node); err != nil {
			t.Fatal(err)
		}
	}

	firstStageHasArg := false
	for _, arg := range stages[0].Builder.Arguments() {
		if match, err := regexp.MatchString(`FOO=.*`, arg); err == nil && match {
			firstStageHasArg = true
			break
		} else if err != nil {
			t.Fatal(err)
		}
	}
	if firstStageHasArg {
		t.Fatalf("expected FOO to not be present in first stage")
	}

	if !builderHasArgument(stages[1].Builder, "FOO=7") {
		t.Fatalf("expected FOO=7 in second stage arguments list, got %v", stages[1].Builder.Arguments())
	}
}

func TestArgs(t *testing.T) {
	for _, tc := range []struct {
		name          string
		dockerfile    string
		args          map[string]string
		expectedValue string
	}{
		{
			name:          "argOverride",
			dockerfile:    "FROM centos\nARG FOO=stuff\nARG FOO=things\n",
			args:          map[string]string{},
			expectedValue: "FOO=things",
		},
		{
			name:          "argOverrideWithBuildArgs",
			dockerfile:    "FROM centos\nARG FOO=stuff\nARG FOO=things\n",
			args:          map[string]string{"FOO": "bar"},
			expectedValue: "FOO=bar",
		},
		{
			name:          "multiple args in single step",
			dockerfile:    "FROM centos\nARG FOO=stuff WORLD=hello\n",
			args:          map[string]string{},
			expectedValue: "WORLD=hello",
		},
		{
			name:          "multiple args in single step",
			dockerfile:    "FROM centos\nARG FOO=stuff WORLD=hello\n",
			args:          map[string]string{},
			expectedValue: "FOO=stuff",
		},
		{
			name:          "headingArgRedefine",
			dockerfile:    "ARG FOO=stuff\nFROM centos\nARG FOO\n",
			args:          map[string]string{},
			expectedValue: "FOO=stuff",
		},
		{
			name:          "headingArgRedefineWithBuildArgs",
			dockerfile:    "ARG FOO=stuff\nFROM centos\nARG FOO\n",
			args:          map[string]string{"FOO": "bar"},
			expectedValue: "FOO=bar",
		},
		{
			name:          "headingArgRedefineDefault",
			dockerfile:    "ARG FOO=stuff\nFROM centos\nARG FOO=defaultfoovalue\n",
			args:          map[string]string{},
			expectedValue: "FOO=defaultfoovalue",
		},
		{
			name:          "headingArgRedefineDefaultWithBuildArgs",
			dockerfile:    "ARG FOO=stuff\nFROM centos\nARG FOO=defaultfoovalue\n",
			args:          map[string]string{"FOO": "bar"},
			expectedValue: "FOO=bar",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			node, err := ParseDockerfile(strings.NewReader(tc.dockerfile))
			if err != nil {
				t.Fatal(err)
			}

			b := NewBuilder(tc.args)
			if err := resolveNodeArgs(b, node); err != nil {
				t.Fatal(err)
			}

			if !builderHasArgument(b, tc.expectedValue) {
				t.Fatalf("expected %s to be contained in arguments list: %v", tc.expectedValue, b.Arguments())
			}
		})
	}
}

func TestMultiStageArgScope(t *testing.T) {
	n, err := ParseFile("dockerclient/testdata/multistage/Dockerfile.arg-scope")
	if err != nil {
		t.Fatal(err)
	}
	args := map[string]string{
		"SECRET": "secretthings",
		"BAR":    "notsecretthings",
		"UNUSED": "notrightawayanyway",
	}
	stages, err := NewStages(n, NewBuilder(args))
	if err != nil {
		t.Fatal(err)
	}
	if len(stages) != 3 {
		t.Fatalf("expected 3 stages, got %d", len(stages))
	}

	for _, stage := range stages {
		if err := resolveNodeArgs(stage.Builder, stage.Node); err != nil {
			t.Fatal(err)
		}
	}

	if !builderHasArgument(stages[0].Builder, "SECRET=secretthings") {
		t.Fatalf("expected SECRET=secretthings to be contained in first stage arguments list: %v", stages[0].Builder.Arguments())
	}

	secondStageArguments := stages[1].Builder.Arguments()
	secretInSecondStage := false
	for _, arg := range secondStageArguments {
		if match, err := regexp.MatchString(`SECRET=.*`, arg); err == nil && match {
			secretInSecondStage = true
			break
		} else if err != nil {
			t.Fatal(err)
		}
	}
	if secretInSecondStage {
		t.Fatalf("expected SECRET to not be present in second stage")
	}

	if !builderHasArgument(stages[1].Builder, "FOO=test") {
		t.Fatalf("expected FOO=test to be present in second stage arguments list: %v", secondStageArguments)
	}
	if !builderHasArgument(stages[1].Builder, "BAR=notsecretthings") {
		t.Fatalf("expected BAR=notsecretthings to be present in second stage arguments list: %v", secondStageArguments)
	}

	thirdStageArguments := stages[2].Builder.Arguments()
	inheritedInThirdStage := false
	unusedInThirdStage := false
	for _, arg := range thirdStageArguments {
		if match, err := regexp.MatchString(`INHERITED=.*`, arg); err == nil && match {
			inheritedInThirdStage = true
			continue
		} else if err != nil {
			t.Fatal(err)
		}
		if match, err := regexp.MatchString(`UNUSED=.*`, arg); err == nil && match {
			unusedInThirdStage = true
			continue
		} else if err != nil {
			t.Fatal(err)
		}
	}
	if !inheritedInThirdStage {
		t.Fatalf("expected INHERITED to be present in third stage")
	}
	if !unusedInThirdStage {
		t.Fatalf("expected UNUSED to be present in third stage")
	}
}

func TestRun(t *testing.T) {
	f, err := os.Open("dockerclient/testdata/Dockerfile.add")
	if err != nil {
		t.Fatal(err)
	}
	node, err := ParseDockerfile(f)
	if err != nil {
		t.Fatal(err)
	}
	b := NewBuilder(nil)
	from, err := b.From(node)
	if err != nil {
		t.Fatal(err)
	}
	if from != "mirror.gcr.io/busybox" {
		t.Fatalf("unexpected from: %s", from)
	}
	for _, child := range node.Children {
		step := b.Step()
		if err := step.Resolve(child); err != nil {
			t.Fatal(err)
		}
		if err := b.Run(step, LogExecutor, false); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("config: %#v", b.Config())
	t.Logf(node.Dump())
}

type testExecutor struct {
	Preserved    []string
	Copies       []Copy
	Runs         []Run
	Configs      []docker.Config
	Unrecognized []Step
	Err          error
}

func (e *testExecutor) Preserve(path string) error {
	e.Preserved = append(e.Preserved, path)
	return e.Err
}

func (e *testExecutor) EnsureContainerPath(path string) error {
	return e.Err
}

func (e *testExecutor) EnsureContainerPathAs(path, user string, mode *os.FileMode) error {
	return e.Err
}

func (e *testExecutor) Copy(excludes []string, copies ...Copy) error {
	e.Copies = append(e.Copies, copies...)
	return e.Err
}
func (e *testExecutor) Run(run Run, config docker.Config) error {
	e.Runs = append(e.Runs, run)
	e.Configs = append(e.Configs, config)
	return e.Err
}
func (e *testExecutor) UnrecognizedInstruction(step *Step) error {
	e.Unrecognized = append(e.Unrecognized, *step)
	return e.Err
}

func TestBuilder(t *testing.T) {
	testCases := []struct {
		Args         map[string]string
		Dockerfile   string
		From         string
		Copies       []Copy
		Runs         []Run
		Unrecognized []Step
		Config       docker.Config
		Image        *docker.Image
		FromErrFn    func(err error) bool
		RunErrFn     func(err error) bool
	}{
		{
			Dockerfile: "dockerclient/testdata/dir/Dockerfile",
			From:       "mirror.gcr.io/busybox",
			Copies: []Copy{
				{Src: []string{"."}, Dest: "/", Download: false},
				{Src: []string{"."}, Dest: "/dir"},
				{Src: []string{"subdir/"}, Dest: "/test/", Download: false},
			},
			Config: docker.Config{
				Image: "mirror.gcr.io/busybox",
			},
		},
		{
			Dockerfile: "dockerclient/testdata/ignore/Dockerfile",
			From:       "mirror.gcr.io/busybox",
			Copies: []Copy{
				{Src: []string{"."}, Dest: "/"},
			},
			Config: docker.Config{
				Image: "mirror.gcr.io/busybox",
			},
		},
		{
			Dockerfile: "dockerclient/testdata/Dockerfile.env",
			From:       "mirror.gcr.io/busybox",
			Config: docker.Config{
				Env:   []string{"name=value", "name2=value2a            value2b", "name1=value1", "name3=value3a\\n\"value3b\"", "name4=value4a\\nvalue4b"},
				Image: "mirror.gcr.io/busybox",
			},
		},
		{
			Dockerfile: "dockerclient/testdata/Dockerfile.edgecases",
			From:       "mirror.gcr.io/busybox",
			Copies: []Copy{
				{Src: []string{"."}, Dest: "/", Download: true},
				{Src: []string{"."}, Dest: "/test/copy"},
			},
			Runs: []Run{
				{Shell: false, Args: []string{"ls", "-la"}},
				{Shell: false, Args: []string{"echo", "'1234'"}},
				{Shell: true, Args: []string{"echo \"1234\""}},
				{Shell: true, Args: []string{"echo 1234"}},
				{Shell: true, Args: []string{"echo '1234' &&     echo \"456\" &&     echo 789"}},
				{Shell: true, Args: []string{"sh -c 'echo root:testpass         > /tmp/passwd'"}},
				{Shell: true, Args: []string{"mkdir -p /test /test2 /test3/test"}},
			},
			Config: docker.Config{
				User:         "docker:root",
				ExposedPorts: map[docker.Port]struct{}{"6000/tcp": {}, "3000/tcp": {}, "9000/tcp": {}, "5000/tcp": {}},
				Env:          []string{"SCUBA=1 DUBA 3"},
				Cmd:          []string{"/bin/sh", "-c", "echo 'test' | wc -"},
				Image:        "mirror.gcr.io/busybox",
				Volumes:      map[string]struct{}{"/test2": {}, "/test3": {}, "/test": {}},
				WorkingDir:   "/test",
				OnBuild:      []string{"RUN [\"echo\", \"test\"]", "RUN echo test", "COPY . /"},
			},
		},
		{
			Dockerfile: "dockerclient/testdata/Dockerfile.unknown",
			From:       "mirror.gcr.io/busybox",
			Unrecognized: []Step{
				{Command: "health", Message: "HEALTH ", Original: "HEALTH NONE", Args: []string{""}, Flags: []string{}, Env: []string{}},
				{Command: "unrecognized", Message: "UNRECOGNIZED ", Original: "UNRECOGNIZED", Args: []string{""}, Env: []string{}},
			},
			Config: docker.Config{
				Image: "mirror.gcr.io/busybox",
			},
		},
		{
			Dockerfile: "dockerclient/testdata/Dockerfile.exposedefault",
			From:       "mirror.gcr.io/busybox",
			Config: docker.Config{
				ExposedPorts: map[docker.Port]struct{}{"3469/tcp": {}},
				Image:        "mirror.gcr.io/busybox",
			},
		},
		{
			Dockerfile: "dockerclient/testdata/Dockerfile.add",
			From:       "mirror.gcr.io/busybox",
			Copies: []Copy{
				{Src: []string{"https://github.com/openshift/origin/raw/main/README.md"}, Dest: "/README.md", Download: true},
				{Src: []string{"https://github.com/openshift/origin/raw/main/LICENSE"}, Dest: "/", Download: true},
				{Src: []string{"https://github.com/openshift/origin/raw/main/LICENSE"}, Dest: "/A", Download: true},
				{Src: []string{"https://github.com/openshift/origin/raw/main/LICENSE"}, Dest: "/a", Download: true},
				{Src: []string{"https://github.com/openshift/origin/raw/main/LICENSE"}, Dest: "/b/a", Download: true},
				{Src: []string{"https://github.com/openshift/origin/raw/main/LICENSE"}, Dest: "/b/", Download: true},
				{Src: []string{"https://github.com/openshift/ruby-hello-world/archive/master.zip"}, Dest: "/tmp/", Download: true},
			},
			Runs: []Run{
				{Shell: true, Args: []string{"mkdir ./b"}},
			},
			Config: docker.Config{
				Image: "mirror.gcr.io/busybox",
				User:  "root",
			},
		},
		{
			Dockerfile: "dockerclient/testdata/Dockerfile.badhealthcheck",
			From:       "mirror.gcr.io/debian",
			Config: docker.Config{
				Image: "mirror.gcr.io/busybox",
			},
			RunErrFn: func(err error) bool {
				return err != nil && strings.Contains(err.Error(), "HEALTHCHECK requires at least one argument")
			},
		},
		{
			Dockerfile: "dockerclient/testdata/Dockerfile.healthcheck",
			From:       "mirror.gcr.io/debian",
			Config: docker.Config{
				Image: "mirror.gcr.io/debian",
				Cmd:   []string{"/bin/sh", "-c", "/app/main.sh"},
				Healthcheck: &docker.HealthConfig{
					StartPeriod:   8 * time.Second,
					Interval:      5 * time.Second,
					Timeout:       3 * time.Second,
					StartInterval: 10 * time.Second,
					Retries:       3,
					Test:          []string{"CMD-SHELL", "/app/check.sh --quiet"},
				},
			},
		},
		{
			Dockerfile: "dockerclient/testdata/Dockerfile.healthcheck_defaults",
			From:       "mirror.gcr.io/debian",
			Config: docker.Config{
				Image: "mirror.gcr.io/debian",
				Cmd:   []string{"/bin/sh", "-c", "/app/main.sh"},
				Healthcheck: &docker.HealthConfig{
					StartPeriod:   0 * time.Second,
					Interval:      0 * time.Second,
					Timeout:       0 * time.Second,
					StartInterval: 0 * time.Second,
					Retries:       0,
					Test:          []string{"CMD-SHELL", "/app/check.sh --quiet"},
				},
			},
		},
		{
			Dockerfile: "dockerclient/testdata/Dockerfile.envsubst",
			From:       "mirror.gcr.io/busybox",
			Image: &docker.Image{
				ID: "busybox2",
				Config: &docker.Config{
					Env: []string{"FOO=another", "BAR=original"},
				},
			},
			Config: docker.Config{
				Env:    []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", "FOO=value"},
				Labels: map[string]string{"test": "value"},
			},
		},
		{
			Dockerfile: "dockerclient/testdata/Dockerfile.unset",
			From:       "mirror.gcr.io/busybox",
			Image: &docker.Image{
				ID: "busybox2",
				Config: &docker.Config{
					Env: []string{},
				},
			},
			RunErrFn: func(err error) bool {
				return err != nil && strings.Contains(err.Error(), "is not allowed to be unset")
			},
			Config: docker.Config{
				Env:    []string{},
				Labels: map[string]string{"test": ""},
			},
		},
		{
			Dockerfile: "dockerclient/testdata/Dockerfile.args",
			Args:       map[string]string{"BAR": "first"},
			From:       "mirror.gcr.io/busybox",
			Config: docker.Config{
				Image:  "mirror.gcr.io/busybox",
				Env:    []string{"FOO=value", "TEST=", "BAZ=first"},
				Labels: map[string]string{"test": "value"},
			},
			Runs: []Run{
				{Shell: true, Args: []string{"echo $BAR"}},
			},
		},
		{
			Dockerfile: "dockerclient/testdata/volume/Dockerfile",
			From:       "mirror.gcr.io/busybox",
			Image: &docker.Image{
				ID:     "busybox2",
				Config: &docker.Config{},
			},
			Config: docker.Config{
				Env: []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
				Volumes: map[string]struct{}{
					"/var":     {},
					"/var/www": {},
				},
			},
			Copies: []Copy{
				{Src: []string{"file"}, Dest: "/var/www/", Download: true},
				{Src: []string{"file"}, Dest: "/var/", Download: true},
				{Src: []string{"file2"}, Dest: "/var/", Download: true},
			},
		},
		{
			Dockerfile: "dockerclient/testdata/volumerun/Dockerfile",
			From:       "mirror.gcr.io/busybox",
			Config: docker.Config{
				Image: "mirror.gcr.io/busybox",
				Volumes: map[string]struct{}{
					"/var/www": {},
				},
			},
			Runs: []Run{
				{Shell: true, Args: []string{"touch /var/www/file3"}},
			},
			Copies: []Copy{
				{Src: []string{"file"}, Dest: "/var/www/", Download: true},
				{Src: []string{"file2"}, Dest: "/var/www/", Download: true},
				{Src: []string{"file4"}, Dest: "/var/www/", Download: true},
			},
		},
		{
			Dockerfile: "dockerclient/testdata/multistage/Dockerfile",
			From:       "busybox",
			Config: docker.Config{
				Image:      "mirror.gcr.io/busybox",
				WorkingDir: "/tmp",
			},
			FromErrFn: func(err error) bool {
				return err != nil && strings.Contains(err.Error(), "multiple FROM statements are not supported")
			},
			Runs: []Run{
				{Shell: true, Args: []string{"echo foo > bar"}},
			},
			Copies: []Copy{
				{Src: []string{"file"}, Dest: "/var/www/", Download: true},
				{Src: []string{"file2"}, Dest: "/var/www/", Download: true},
				{Src: []string{"file4"}, Dest: "/var/www/", Download: true},
			},
		},
		{
			Dockerfile: "dockerclient/testdata/Dockerfile.shell",
			From:       "public.ecr.aws/docker/library/centos:7",
			Config: docker.Config{
				Image: "public.ecr.aws/docker/library/centos:7",
				Shell: []string{"/bin/bash", "-xc"},
			},
			Runs: []Run{
				{Shell: true, Args: []string{"env"}},
			},
		},
	}
	for i, test := range testCases {
		t.Run(fmt.Sprintf("%s %d", test.Dockerfile, i), func(t *testing.T) {
			data, err := ioutil.ReadFile(test.Dockerfile)
			if err != nil {
				t.Fatalf("%d: %v", i, err)
			}
			node, err := ParseDockerfile(bytes.NewBuffer(data))
			if err != nil {
				t.Fatalf("%d: %v", i, err)
			}
			b := NewBuilder(test.Args)
			from, err := b.From(node)
			if err != nil {
				if test.FromErrFn == nil || !test.FromErrFn(err) {
					t.Errorf("%d: %v", i, err)
				}
				return
			}
			if test.FromErrFn != nil {
				t.Errorf("%d: expected an error from From(), didn't get one", i)
			}
			if from != test.From {
				t.Errorf("%d: unexpected FROM: %s", i, from)
			}
			if test.Image != nil {
				if err := b.FromImage(test.Image, node); err != nil {
					t.Errorf("%d: unexpected error: %v", i, err)
				}
			}

			e := &testExecutor{}
			var lastErr error
			for j, child := range node.Children {
				step := b.Step()
				if err := step.Resolve(child); err != nil {
					lastErr = fmt.Errorf("%d: %d: %s: resolve: %v", i, j, step.Original, err)
					break
				}
				if err := b.Run(step, e, false); err != nil {
					lastErr = fmt.Errorf("%d: %d: %s: run: %v", i, j, step.Original, err)
					break
				}
			}
			if lastErr != nil {
				if test.RunErrFn == nil || !test.RunErrFn(lastErr) {
					t.Errorf("%d: unexpected error: %v", i, lastErr)
				}
				return
			}
			if test.RunErrFn != nil {
				t.Errorf("%d: expected an error from Resolve()/Run()(), didn't get one", i)
			}
			if !reflect.DeepEqual(test.Copies, e.Copies) {
				t.Errorf("%d: unexpected copies: %#v", i, e.Copies)
			}
			if !reflect.DeepEqual(test.Runs, e.Runs) {
				t.Errorf("%d: unexpected runs: %#v", i, e.Runs)
			}
			if !reflect.DeepEqual(test.Unrecognized, e.Unrecognized) {
				t.Errorf("%d: unexpected unrecognized: %#v", i, e.Unrecognized)
			}
			lastConfig := b.RunConfig
			if !reflect.DeepEqual(test.Config, lastConfig) {
				data, _ := json.Marshal(lastConfig)
				expected, _ := json.Marshal(test.Config)
				t.Errorf("%d: unexpected config: %s should be %s", i, string(data), string(expected))
			}
		})
	}
}

func TestRunWithEnvArgConflict(t *testing.T) {
	f, err := os.Open("dockerclient/testdata/Dockerfile.envargconflict")
	if err != nil {
		t.Fatal(err)
	}
	node, err := ParseDockerfile(f)
	if err != nil {
		t.Fatal(err)
	}
	b := NewBuilder(nil)
	from, err := b.From(node)
	if err != nil {
		t.Fatal(err)
	}
	if from != "ubuntu:18.04" {
		t.Fatalf("unexpected from: %s", from)
	}
	for _, child := range node.Children {
		step := b.Step()
		if err := step.Resolve(child); err != nil {
			t.Fatal(err)
		}
		if err := b.Run(step, LogExecutor, false); err != nil {
			t.Fatal(err)
		}
	}
	configString := fmt.Sprintf("%v", b.Config())
	expectedValue := "USER_NAME=my_user_env"
	if !strings.Contains(configString, expectedValue) {
		t.Fatalf("expected %s to be contained in the Configuration list: %s", expectedValue, configString)
	}
	expectedValue = "USER_NAME=my_user_arg"
	if strings.Contains(configString, expectedValue) {
		t.Fatalf("expected %s to NOT be contained in the Configuration list: %s", expectedValue, configString)
	}
	expectedValue = "/home/my_user_env"
	if !strings.Contains(configString, expectedValue) {
		t.Fatalf("expected %s to be contained in the Configuration list: %s", expectedValue, configString)
	}

	t.Logf("config: %#v", b.Config())
	t.Logf(node.Dump())
}

func TestRunWithMultiArg(t *testing.T) {
	f, err := os.Open("dockerclient/testdata/Dockerfile.multiarg")
	if err != nil {
		t.Fatal(err)
	}
	node, err := ParseDockerfile(f)
	if err != nil {
		t.Fatal(err)
	}
	b := NewBuilder(nil)
	from, err := b.From(node)
	if err != nil {
		t.Fatal(err)
	}
	if from != "mirror.gcr.io/alpine" {
		t.Fatalf("unexpected from: %s", from)
	}
	for _, child := range node.Children {
		step := b.Step()
		if err := step.Resolve(child); err != nil {
			t.Fatal(err)
		}
		if err := b.Run(step, LogExecutor, false); err != nil {
			t.Fatal(err)
		}
	}
	configString := fmt.Sprintf("%v", b.Config())
	expectedValue := "multival=a=1 b=2 c=3 d=4"
	if !strings.Contains(configString, expectedValue) {
		t.Fatalf("expected %s to be contained in the Configuration list: %s", expectedValue, configString)
	}

	t.Logf("config: %#v", b.Config())
	t.Logf(node.Dump())
}

func TestParseDockerignore(t *testing.T) {
	dir, err := ioutil.TempDir("", "dockerignore*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	tests := []struct {
		input, result []string
	}{
		{
			input:  []string{"first", "second", "", "third", "fourth"},
			result: []string{"first", "second", "third", "fourth"},
		},
		{
			input:  []string{"#first", "#second", "", "third", "fourth"},
			result: []string{"third", "fourth"},
		},
		{
			input:  []string{"", "first", "second", "", " #third", "#invalid pattern which shouldn't matter ("},
			result: []string{"first", "second", " #third"},
		},
		{
			input:  []string{"", "first", "second", "", "#third", ""},
			result: []string{"first", "second"},
		},
		{
			input:  []string{"first", "second", "", "th#rd", "fourth", "fifth#"},
			result: []string{"first", "second", "th#rd", "fourth", "fifth#"},
		},
		{
			input:  []string{"/first", "second/", "/third/", "///fourth//", "fif/th#", "/"},
			result: []string{"first", "second", "third", "fourth", "fif/th#"},
		},
	}

	testIgnore := func(ignorefile string) {
		for _, test := range tests {
			f, err := os.Create(ignorefile)
			if err != nil {
				t.Fatalf("error creating %q: %v", ignorefile, err)
			}
			fmt.Fprintf(f, "%s\n", strings.Join(test.input, "\n"))
			f.Close()
			excludes, err := ParseDockerignore(dir)
			if err != nil {
				t.Fatalf("error reading %q: %v", ignorefile, err)
			}
			if err := os.Remove(ignorefile); err != nil {
				t.Fatalf("failed to remove ignore file: %v", err)
			}
			if len(excludes) != len(test.result) {
				t.Errorf("expected to read back %#v, got %#v", test.result, excludes)
			}
			for i := range excludes {
				if excludes[i] != test.result[i] {
					t.Errorf("expected to read back %#v, got %#v", test.result, excludes)
				}
			}
		}
	}
	testIgnore(filepath.Join(dir, ".containerignore"))
	testIgnore(filepath.Join(dir, ".dockerignore"))
	// Create empty .dockerignore to test in same directory as .containerignore
	f, err := os.Create(filepath.Join(dir, ".dockerignore"))
	if err != nil {
		t.Fatalf("error creating: %v", err)
	}
	f.Close()
	testIgnore(filepath.Join(dir, ".containerignore"))
	os.Remove(filepath.Join(dir, ".dockerignore"))

	ignorefile := filepath.Join(dir, "ignore")
	for _, test := range tests {
		f, err := os.Create(ignorefile)
		if err != nil {
			t.Fatalf("error creating %q: %v", ignorefile, err)
		}
		fmt.Fprintf(f, "%s\n", strings.Join(test.input, "\n"))
		f.Close()
		excludes, err := ParseIgnore(ignorefile)
		if err != nil {
			t.Fatalf("error reading %q: %v", ignorefile, err)
		}
		if err := os.Remove(ignorefile); err != nil {
			t.Fatalf("failed to remove ignore file: %v", err)
		}
		if len(excludes) != len(test.result) {
			t.Errorf("expected to read back %#v, got %#v", test.result, excludes)
		}
		for i := range excludes {
			if excludes[i] != test.result[i] {
				t.Errorf("expected to read back %#v, got %#v", test.result, excludes)
			}
		}
	}
}
