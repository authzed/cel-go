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

// Package celtest provides functions for testing CEL policies and expressions.
package celtest

import (
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/tools/compiler"
)

func TestCoverageStats(t *testing.T) {
	testCases := []struct {
		name                       string
		configPath                 string
		testSuitePath              string
		celExpr                    string
		compilerOpts               []any
		totalASTNodes              int64
		coveredBranchNodesComplete []int64
		coveredBranchNodesPartial  []int64
		uncoveredNodes             []int64
	}{
		{
			name:                       "restricted destinations policy coverage",
			configPath:                 "../../policy/testdata/restricted_destinations/config.yaml",
			testSuitePath:              "../../policy/testdata/restricted_destinations/tests.yaml",
			celExpr:                    "../../policy/testdata/restricted_destinations/policy.yaml",
			compilerOpts:               []any{locationCodeEnvOption()},
			totalASTNodes:              44,
			coveredBranchNodesComplete: []int64{1, 7, 11, 12, 19, 23, 28, 29, 30, 31, 32, 33, 34, 36, 37, 38, 39, 40, 42},
			coveredBranchNodesPartial:  []int64{3, 13, 41},
			uncoveredNodes:             []int64{},
		},
		{
			name:                       "k8s policy coverage",
			configPath:                 "../../policy/testdata/k8s/config.yaml",
			testSuitePath:              "../../policy/testdata/k8s/tests.yaml",
			celExpr:                    "../../policy/testdata/k8s/policy.yaml",
			compilerOpts:               []any{k8sParserOpts()},
			totalASTNodes:              38,
			coveredBranchNodesComplete: []int64{},
			coveredBranchNodesPartial:  []int64{8, 16, 17, 18, 19, 21, 22, 23, 24, 25, 26, 31},
			uncoveredNodes:             []int64{5, 6, 7, 11, 12, 38},
		},
		{
			name:                       "policy with custom policy metadata coverage",
			testSuitePath:              "../../tools/celtest/testdata/custom_policy_tests.yaml",
			celExpr:                    "../../tools/celtest/testdata/custom_policy.celpolicy",
			compilerOpts:               []any{customPolicyParserOption(), compiler.PolicyMetadataEnvOption(ParsePolicyVariables)},
			totalASTNodes:              10,
			coveredBranchNodesComplete: []int64{1, 2, 3, 6},
			coveredBranchNodesPartial:  []int64{},
			uncoveredNodes:             []int64{},
		},
		{
			name:                       "raw expression file with unknowns test coverage",
			configPath:                 "../../tools/celtest/testdata/config.yaml",
			testSuitePath:              "../../tools/celtest/testdata/raw_expr_tests.yaml",
			celExpr:                    "../../tools/celtest/testdata/raw_expr.cel",
			compilerOpts:               []any{fnEnvOption()},
			totalASTNodes:              8,
			coveredBranchNodesComplete: []int64{1, 6, 8},
			coveredBranchNodesPartial:  []int64{},
			uncoveredNodes:             []int64{},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.compilerOpts = append(tc.compilerOpts, cel.EnableMacroCallTracking())
			if tc.configPath != "" {
				envOpt := compiler.EnvironmentFile(tc.configPath)
				tc.compilerOpts = append(tc.compilerOpts, envOpt)
			}
			compilerOpt := TestCompiler(tc.compilerOpts...)
			testSuiteOpt := TestSuite(tc.testSuitePath)
			testExprOpt := TestExpression(tc.celExpr)
			tr, err := NewTestRunner(compilerOpt, testSuiteOpt, testExprOpt, EnableCoverage(), PartialEvalProgramOption())
			if err != nil {
				t.Fatalf("compiler.NewCompiler() failed: %v", err)
			}
			programs, err := tr.Programs(t, tr.testProgramOptions...)
			if err != nil {
				t.Fatalf("error creating programs: %v", err)
			}
			tests, err := tr.Tests(t)
			if err != nil {
				t.Fatalf("error creating tests: %v", err)
			}
			for _, test := range tests {
				err := tr.ExecuteTest(t, programs, test)
				if err != nil {
					t.Fatalf("error executing test: %v", err)
				}
			}
			p := programs[0]
			rootNavigableExpr := ast.NavigateAST(p.Ast.NativeRep())
			cr := &coverageReport{
				nodes:                  0,
				coveredNodes:           0,
				branches:               0,
				coveredBooleanOutcomes: 0,
				unencounteredNodes:     []string{},
				unencounteredBranches:  []string{},
			}
			traverseAndCalculateCoverage(t, rootNavigableExpr, p, true, "", cr)
			coverageStats := p.CoverageStats
			if cr.nodes != tc.totalASTNodes {
				t.Errorf("totalASTNodes = %d, want %d", cr.nodes, tc.totalASTNodes)
			}
			for _, id := range tc.coveredBranchNodesComplete {
				if val, ok := coverageStats[id]; !ok || len(val) < 2 {
					t.Errorf("id %d is not covered completely", id)
				}
			}
			for _, id := range tc.coveredBranchNodesPartial {
				if val, ok := coverageStats[id]; !ok || len(val) != 1 {
					t.Errorf("id %d is not covered partially", id)
				}
			}
			for _, id := range tc.uncoveredNodes {
				if _, ok := coverageStats[id]; ok {
					t.Errorf("id %d is covered, expected uncoveredNodes", id)
				}
			}
		})
	}
}
