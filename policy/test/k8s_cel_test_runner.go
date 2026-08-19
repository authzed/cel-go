// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package test

import (
	"os"
	"testing"

	"github.com/google/cel-go/policy"
	"github.com/google/cel-go/tools/celtest"
)

// TestK8sCEL triggers compilation and test execution of a k8s policy which
// contains custom policy tags. Custom parser options are used to configure the
// parser to handle the custom policy tags. Tests are triggered with a list of
// custom CEL environment options.
func TestK8sCEL(t *testing.T) {
	parserOpt := policy.ParserOption(testK8sPolicyParser)
	testResourcesDir := os.Getenv("RUNFILES_DIR")
	testRunnerOpt := celtest.TestRunnerOptionsFromFlags(testResourcesDir, nil, parserOpt)
	celtest.TriggerTests(t, testRunnerOpt)
}

func testK8sPolicyParser(p *policy.Parser) (*policy.Parser, error) {
	p.TagVisitor = policy.K8sTestTagHandler()
	return p, nil
}
