// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package policy_conformance_test

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/policy"
	"github.com/google/cel-go/tools/celtest"
	"github.com/google/cel-go/tools/compiler"

	_ "cel.dev/expr/conformance/proto3"
)

type testsFlag []string

func (t *testsFlag) String() string {
	return strings.Join(([]string)(*t), ",")
}

func (t *testsFlag) Set(v string) error {
	*t = strings.Split(v, ",")
	for i, v := range *t {
		(*t)[i] = strings.TrimSpace(v)
	}
	return nil
}

func (t *testsFlag) Get() any {
	return ([]string)(*t)
}

type skipTestsFlag []string

func (t *skipTestsFlag) String() string {
	return strings.Join(([]string)(*t), ",")
}

func (t *skipTestsFlag) Set(v string) error {
	*t = strings.Split(v, ",")
	for i, v := range *t {
		(*t)[i] = strings.TrimSpace(v)
	}
	return nil
}

func (t *skipTestsFlag) Get() any {
	return ([]string)(*t)
}

const (
	testsYAMLFileName       = "tests.yaml"
	testsTextprotoFileName  = "tests.textproto"
	policyYAMLFileName      = "policy.yaml"
	configYAMLFileName      = "config.yaml"
	configTextprotoFileName = "config.textproto"
)

var (
	testdataDir string
	tests       testsFlag
	skipTests   skipTestsFlag
)

func init() {
	flag.StringVar(&testdataDir, "testdata_dir", "cel_policy/conformance/testdata", "Path to testdata directory.")
	flag.Var(&tests, "tests", "Paths to run, separate by a comma.")
	flag.Var(&skipTests, "skip_tests", "Tests to skip, separate by a comma.")
}

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

func shouldSkipTest(s string) bool {
	for _, t := range skipTests {
		if strings.HasPrefix(s, t) {
			n := s[len(t):]
			if n == "" || strings.HasPrefix(n, "/") {
				return true
			}
		}
	}
	return false
}

func discoverTestDirs(absTestdataDir string) []string {
	entries, err := os.ReadDir(absTestdataDir)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		topLevelDir := filepath.Join(absTestdataDir, entry.Name())
		if hasTestSuite(topLevelDir) {
			dirs = append(dirs, entry.Name())
			continue
		}
		subEntries, err := os.ReadDir(topLevelDir)
		if err != nil {
			continue
		}
		for _, subEntry := range subEntries {
			if !subEntry.IsDir() {
				continue
			}
			subDir := filepath.Join(topLevelDir, subEntry.Name())
			if hasTestSuite(subDir) {
				dirs = append(dirs, entry.Name()+"/"+subEntry.Name())
			}
		}
	}
	return dirs
}

func hasTestSuite(dir string) bool {
	hasTests := fileExists(filepath.Join(dir, testsYAMLFileName)) || fileExists(filepath.Join(dir, testsTextprotoFileName))
	hasPolicy := fileExists(filepath.Join(dir, policyYAMLFileName))
	return hasTests && hasPolicy
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func locationCodeEnvOption() cel.EnvOption {
	return cel.Function("locationCode",
		cel.Overload("locationCode_string", []*cel.Type{cel.StringType}, cel.StringType,
			cel.UnaryBinding(func(ip ref.Val) ref.Val {
				switch ip.(types.String) {
				case "10.0.0.1":
					return types.String("us")
				case "10.0.0.2":
					return types.String("de")
				default:
					return types.String("ir")
				}
			})))
}

func k8sParserOpts() policy.ParserOption {
	return func(p *policy.Parser) (*policy.Parser, error) {
		p.TagVisitor = policy.K8sTestTagHandler()
		return p, nil
	}
}

func TestConformance(t *testing.T) {
	absTestdataDir, err := rlocation(testdataDir)
	if err != nil {
		log.Fatalf("rlocation(%q) failed: %v", testdataDir, err)
	}

	testDirs := ([]string)(tests)
	if len(testDirs) == 0 {
		testDirs = discoverTestDirs(absTestdataDir)
	}

	for _, dir := range testDirs {
		fullDirPath := filepath.Join(absTestdataDir, dir)
		yamlFile := filepath.Join(fullDirPath, testsYAMLFileName)
		textprotoFile := filepath.Join(fullDirPath, testsTextprotoFileName)
		policyFile := filepath.Join(fullDirPath, policyYAMLFileName)

		yamlExists := fileExists(yamlFile)
		textprotoExists := fileExists(textprotoFile)
		bothExist := yamlExists && textprotoExists

		if yamlExists {
			suffix := ""
			if bothExist {
				suffix = " (yaml)"
			}
			runTestSuite(t, fullDirPath, dir, yamlFile, policyFile, suffix)
		}
		if textprotoExists {
			suffix := ""
			if bothExist {
				suffix = " (textproto)"
			}
			runTestSuite(t, fullDirPath, dir, textprotoFile, policyFile, suffix)
		}
	}
}

func runTestSuite(t *testing.T, fullDirPath, dir, testSuiteFile, policyFile, suffix string) {
	var compileOpts []any
	fdsFlag := flag.Lookup("file_descriptor_set")
	fileDescriptorSetPath := ""
	if fdsFlag != nil {
		fileDescriptorSetPath = fdsFlag.Value.String()
	}
	if fileDescriptorSetPath != "" {
		fdsPath, err := rlocation(fileDescriptorSetPath)
		if err == nil {
			compileOpts = append(compileOpts, compiler.TypeDescriptorSetFile(fdsPath))
		}
	}
	yamlConfig := filepath.Join(fullDirPath, configYAMLFileName)
	textprotoConfig := filepath.Join(fullDirPath, configTextprotoFileName)
	if fileExists(yamlConfig) {
		compileOpts = append(compileOpts, compiler.EnvironmentFile(yamlConfig))
	} else if fileExists(textprotoConfig) {
		compileOpts = append(compileOpts, compiler.EnvironmentFile(textprotoConfig))
	}

	if strings.HasPrefix(dir, "k8s") {
		compileOpts = append(compileOpts, k8sParserOpts())
	}
	compileOpts = append(compileOpts, locationCodeEnvOption())

	testRunnerOpts := []celtest.TestRunnerOption{
		celtest.TestCompiler(compileOpts...),
		celtest.TestSuite(testSuiteFile),
		celtest.TestExpression(policyFile),
		celtest.PartialEvalProgramOption(),
	}
	if fileDescriptorSetPath != "" {
		fdsPath, err := rlocation(fileDescriptorSetPath)
		if err == nil {
			testRunnerOpts = append(testRunnerOpts, celtest.FileDescriptorSet(fdsPath))
		}
	}

	tr, err := celtest.NewTestRunner(testRunnerOpts...)
	if err != nil {
		t.Fatalf("error creating test runner: %v", err)
	}
	programs, compileErr := tr.Programs(t, cel.EvalOptions(cel.OptPartialEval))
	tests, err := tr.Tests(t)
	if err != nil {
		t.Fatalf("error creating tests: %v", err)
	}
	if len(programs) == 0 && compileErr == nil {
		t.Fatalf("no programs created for policy %s", policyFile)
	}
	for _, test := range tests {
		testDisplayName := fmt.Sprintf("%s/%s%s", dir, test.Name(), suffix)
		if shouldSkipTest(testDisplayName) {
			continue
		}
		t.Run(testDisplayName, func(t *testing.T) {
			// Delegate compilation failure verification to the test case matcher.
			// This is necessary for expected compile-time error tests (e.g. compile_errors test suites),
			// ensuring that intentional policy compilation failures are matched against expected errors.
			if compileErr != nil {
				if testResult := test.ResultMatcher()(nil, compileErr); !testResult.Success {
					t.Fatalf("test: %s \n wanted: %v \n failed: %v", test.Name(), testResult.Wanted, testResult.Error)
				}
				return
			}

			err := tr.ExecuteTest(t, programs, test)
			if err != nil {
				t.Fatalf("error executing test: %v", err)
			}
		})
	}
}

func rlocation(path string) (string, error) {
	return runfiles.Rlocation(path)
}
