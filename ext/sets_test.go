// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ext

import (
	"fmt"
	"testing"

	"github.com/authzed/cel-go/cel"
)

func TestSets(t *testing.T) {
	setsTests := []struct {
		expr string
	}{
		// set containment
		{expr: `sets.contains([], [])`},
		{expr: `sets.contains([1], [])`},
		{expr: `sets.contains([1], [1])`},
		{expr: `sets.contains([1], [1, 1])`},
		{expr: `sets.contains([1, 1], [1])`},
		{expr: `sets.contains([2, 1], [1])`},
		{expr: `sets.contains([1, 2, 3, 4], [2, 3])`},
		{expr: `sets.contains([1], [1.0, 1])`},
		{expr: `sets.contains([1, 2], [2u, 2.0])`},
		{expr: `sets.contains([1, 2u], [2, 2.0])`},
		{expr: `sets.contains([1, 2.0, 3u], [1.0, 2u, 3])`},
		{expr: `sets.contains([[1], [2, 3]], [[2, 3.0]])`},
		{expr: `!sets.contains([1], [2])`},
		{expr: `!sets.contains([1], [1, 2])`},
		{expr: `!sets.contains([1], ["1", 1])`},
		{expr: `!sets.contains([1], [1.1, 1u])`},
		// set equivalence
		{expr: `sets.equivalent([], [])`},
		{expr: `sets.equivalent([1], [1])`},
		{expr: `sets.equivalent([1], [1, 1])`},
		{expr: `sets.equivalent([1, 1], [1])`},
		{expr: `sets.equivalent([1], [1u, 1.0])`},
		{expr: `sets.equivalent([1], [1u, 1.0])`},
		{expr: `sets.equivalent([1, 2, 3], [3u, 2.0, 1])`},
		{expr: `sets.equivalent([[1.0], [2, 3]], [[1], [2, 3.0]])`},
		{expr: `!sets.equivalent([2, 1], [1])`},
		{expr: `!sets.equivalent([1], [1, 2])`},
		{expr: `!sets.equivalent([1, 2], [2u, 2, 2.0])`},
		{expr: `!sets.equivalent([1, 2], [1u, 2, 2.3])`},
		// set intersection
		{expr: `sets.intersects([1], [1])`},
		{expr: `sets.intersects([1], [1, 1])`},
		{expr: `sets.intersects([1, 1], [1])`},
		{expr: `sets.intersects([2, 1], [1])`},
		{expr: `sets.intersects([1], [1, 2])`},
		{expr: `sets.intersects([1], [1.0, 2])`},
		{expr: `sets.intersects([1, 2], [2u, 2, 2.0])`},
		{expr: `sets.intersects([1, 2], [1u, 2, 2.3])`},
		{expr: `sets.intersects([[1], [2, 3]], [[1, 2], [2, 3.0]])`},
		{expr: `!sets.intersects([], [])`},
		{expr: `!sets.intersects([1], [])`},
		{expr: `!sets.intersects([1], [2])`},
		{expr: `!sets.intersects([1], ["1", 2])`},
		{expr: `!sets.intersects([1], [1.1, 2u])`},
	}

	env := testSetsEnv(t)
	for i, tst := range setsTests {
		tc := tst
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			var asts []*cel.Ast
			pAst, iss := env.Parse(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Parse(%v) failed: %v", tc.expr, iss.Err())
			}
			asts = append(asts, pAst)
			cAst, iss := env.Check(pAst)
			if iss.Err() != nil {
				t.Fatalf("env.Check(%v) failed: %v", tc.expr, iss.Err())
			}
			asts = append(asts, cAst)

			for _, ast := range asts {
				prg, err := env.Program(ast)
				if err != nil {
					t.Fatalf("env.Program() failed: %v", err)
				}
				out, _, err := prg.Eval(cel.NoVars())
				if err != nil {
					t.Fatalf("prg.Eval() failed: %v", err)
				}
				if out.Value() != true {
					t.Errorf("prg.Eval() got %v, wanted true for expr: %s", out.Value(), tc.expr)
				}
			}
		})
	}
}

func testSetsEnv(t *testing.T, opts ...cel.EnvOption) *cel.Env {
	t.Helper()
	baseOpts := []cel.EnvOption{Sets()}
	env, err := cel.NewEnv(append(baseOpts, opts...)...)
	if err != nil {
		t.Fatalf("cel.NewEnv(Sets()) failed: %v", err)
	}
	return env
}
