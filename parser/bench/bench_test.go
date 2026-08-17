// Copyright 2026 Google LLC
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

package bench

import (
	"fmt"
	"testing"

	"github.com/google/cel-go/common"
	"github.com/google/cel-go/parser"
)

func newBenchmarkParser(tb testing.TB) *parser.Parser {
	tb.Helper()
	p, err := parser.NewParser(
		parser.Macros(append(parser.AllMacros, OptMapMacro)...),
		parser.EnableOptionalSyntax(true),
		parser.EnableIdentEscapeSyntax(true),
		parser.MaxRecursionDepth(512),
	)
	if err != nil {
		tb.Fatalf("parser.NewParser() failed: %v", err)
	}
	return p
}

func TestExpectedResult(t *testing.T) {
	p := newBenchmarkParser(t)
	for _, cat := range GetCategories() {
		t.Run(cat.Name, func(t *testing.T) {
			for i, tc := range cat.Cases {
				t.Run(fmt.Sprintf("%d_%s", i, tc.Expr), func(t *testing.T) {
					src := common.NewTextSource(tc.Expr)
					_, errs := p.Parse(src)
					hasErr := len(errs.GetErrors()) > 0
					switch tc.Result {
					case ParseResultSuccess:
						if hasErr {
							t.Errorf("p.Parse(%q) failed unexpectedly: %v", tc.Expr, errs.ToDisplayString())
						}
					case ParseResultError:
						if !hasErr {
							t.Errorf("p.Parse(%q) succeeded unexpectedly, wanted error", tc.Expr)
						}
					}
				})
			}
		})
	}
}

// BenchmarkParse benchmarks parsing organized by workload categories.
func BenchmarkParse(b *testing.B) {
	p := newBenchmarkParser(b)
	for _, cat := range GetCategories() {
		b.Run(cat.Name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for _, tc := range cat.Cases {
					src := common.NewTextSource(tc.Expr)
					_, errs := p.Parse(src)
					hasErr := len(errs.GetErrors()) > 0
					expectedErr := tc.Result == ParseResultError
					if hasErr != expectedErr {
						b.Fatalf("p.Parse(%q) got error: %v, expected error: %v", tc.Expr, hasErr, expectedErr)
					}
				}
			}
		})
	}
}

// BenchmarkParseParallel benchmarks parsing concurrently across goroutines by category.
func BenchmarkParseParallel(b *testing.B) {
	p := newBenchmarkParser(b)
	for _, cat := range GetCategories() {
		b.Run(cat.Name, func(b *testing.B) {
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					for _, tc := range cat.Cases {
						src := common.NewTextSource(tc.Expr)
						_, errs := p.Parse(src)
						hasErr := len(errs.GetErrors()) > 0
						expectedErr := tc.Result == ParseResultError
						if hasErr != expectedErr {
							b.Fatalf("p.Parse(%q) got error: %v, expected error: %v", tc.Expr, hasErr, expectedErr)
						}
					}
				}
			})
		})
	}
}
