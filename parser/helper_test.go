// Copyright 2022 Google LLC
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

package parser

import (
	"testing"

	"github.com/authzed/cel-go/common"

	"google.golang.org/protobuf/proto"

	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

func TestExprHelperCopy(t *testing.T) {
	src := common.NewStringSource(
		`noop([1, 2, 3].map(i, Msg{first: 1 + 2, second: a.b, third: {true: true}}))`,
		"")
	p, err := NewParser(
		PopulateMacroCalls(true),
		Macros(
			MapMacro,
			NewGlobalMacro("noop", 1,
				func(eh ExprHelper,
					target *exprpb.Expr,
					args []*exprpb.Expr) (*exprpb.Expr, *common.Error) {
					return eh.Copy(args[0]), nil
				},
			),
		),
	)
	if err != nil {
		t.Fatalf("NewParser() failed: %v", err)
	}
	ast, errs := p.Parse(src)
	if errs != nil && len(errs.GetErrors()) != 0 {
		t.Fatalf("p.Parse(%v) failed: %v", src, errs.ToDisplayString())
	}

	// id generation is consistent between runs, and in this case '27' refers to the
	// macro expression that's the sole argument to the noop() macro.
	macroTarget := ast.GetSourceInfo().GetMacroCalls()[int64(27)]
	if proto.Equal(ast.GetExpr(), macroTarget) {
		t.Errorf("Copy() failed to provide a unique ids: %v", macroTarget)
	}
}
