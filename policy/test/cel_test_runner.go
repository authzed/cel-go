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

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/tools/celtest"
)

// TestCEL triggers the celtest test runner with a list of custom options which are used to set up
// the compiler tool and test runner.
func TestCEL(t *testing.T) {
	opts := []any{locationCodeEnvOption()}
	testResourcesDir := os.Getenv("RUNFILES_DIR")
	testRunnerOpt := celtest.TestRunnerOptionsFromFlags(testResourcesDir, nil, opts...)
	celtest.TriggerTests(t, testRunnerOpt)
}

func locationCodeEnvOption() cel.EnvOption {
	return cel.Function("locationCode",
		cel.Overload("locationCode_string", []*cel.Type{cel.StringType}, cel.StringType,
			cel.UnaryBinding(locationCode)))
}

func locationCode(ip ref.Val) ref.Val {
	switch ip.(types.String) {
	case "10.0.0.1":
		return types.String("us")
	case "10.0.0.2":
		return types.String("de")
	default:
		return types.String("ir")
	}
}
