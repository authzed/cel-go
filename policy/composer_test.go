package policy

import (
	"strings"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/debug"
	"github.com/google/cel-go/ext"
)

func TestCompose_SourceInfo(t *testing.T) {
	policyYAML := `name: test_policy
rule:
  match:
    - condition: "2 == 1"
      output: "'hi'"
    - output: "'hello' + ' world'"
`
	src := StringSource(policyYAML, "test_policy.yaml")
	parser, err := NewParser()
	if err != nil {
		t.Fatalf("NewParser() failed: %v", err)
	}
	policy, iss := parser.Parse(src)
	if iss.Err() != nil {
		t.Fatalf("parser.Parse() failed: %v", iss.Err())
	}

	env, err := cel.NewEnv(cel.OptionalTypes(), ext.Bindings())
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	compiledRule, iss := CompileRule(env, policy)
	if iss.Err() != nil {
		t.Fatalf("CompileRule() failed: %v", iss.Err())
	}
	composer, err := NewRuleComposer(env)
	if err != nil {
		t.Fatalf("NewRuleComposer() failed: %v", err)
	}
	compAST, iss := composer.Compose(compiledRule)
	if iss.Err() != nil {
		t.Fatalf("composer.Compose() failed: %v", iss.Err())
	}

	si := compAST.SourceInfo()
	if si.Location != "test_policy.yaml" {
		t.Errorf("SourceInfo.Location got %q, wanted test_policy.yaml", si.Location)
	}
	verifySourceInfoTransfer(t, compiledRule, compAST)
}

func TestCompose_Unnest(t *testing.T) {
	policyYAML := `name: unnest
rule:
  match:
    - condition: "2 == 1"
      output: "'hi'"
    - output: "'hello'"
`
	src := StringSource(policyYAML, "unnest.yaml")
	parser, err := NewParser()
	if err != nil {
		t.Fatalf("NewParser() failed: %v", err)
	}
	policy, iss := parser.Parse(src)
	if iss.Err() != nil {
		t.Fatalf("parser.Parse() failed: %v", iss.Err())
	}

	env, err := cel.NewEnv(cel.OptionalTypes(), ext.Bindings())
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	compiledRule, iss := CompileRule(env, policy)
	if iss.Err() != nil {
		t.Fatalf("CompileRule() failed: %v", iss.Err())
	}

	composer, err := NewRuleComposer(env, ExpressionUnnestHeight(1))
	if err != nil {
		t.Fatalf("NewRuleComposer() failed: %v", err)
	}
	compAST, iss := composer.Compose(compiledRule)
	if iss.Err() != nil {
		t.Fatalf("composer.Compose() failed: %v", iss.Err())
	}

	verifySourceInfoTransfer(t, compiledRule, compAST)
	if t.Failed() {
		t.Logf("composed AST: %s", debug.ToDebugStringWithIDs(compAST.NativeRep().Expr()))
		t.Logf("SourceInfo: %v", compAST.NativeRep().SourceInfo().OffsetRanges())
	}
}

// verifySourceInfoTransfer checks that each offset range in the compiledRule has a corresponding node in composed
func verifySourceInfoTransfer(t *testing.T, compiledRule *CompiledRule, composed *cel.Ast) {
	t.Helper()
	dstRanges := make(map[ast.OffsetRange]ast.Expr)
	check := func(a *cel.Ast) {
		ast.PostOrderVisit(a.NativeRep().Expr(), &transferChecker{
			t:       t,
			srcInfo: a.NativeRep().SourceInfo(),
			dstInfo: composed.NativeRep().SourceInfo(),
			ranges:  &dstRanges})
	}
	ast.PostOrderVisit(composed.NativeRep().Expr(), &collectRanges{sourceInfo: composed.NativeRep().SourceInfo(), ranges: &dstRanges})
	for _, match := range compiledRule.matches {
		check(match.cond)
		check(match.output.expr)
	}
}

type collectRanges struct {
	sourceInfo *ast.SourceInfo
	ranges     *map[ast.OffsetRange]ast.Expr
}

func (r *collectRanges) VisitExpr(e ast.Expr) {
	if or, found := r.sourceInfo.GetOffsetRange(e.ID()); found {
		(*r.ranges)[or] = e
	}
}

func (r *collectRanges) VisitEntryExpr(ast.EntryExpr) {
}

type transferChecker struct {
	t       *testing.T
	srcInfo *ast.SourceInfo
	dstInfo *ast.SourceInfo
	ranges  *map[ast.OffsetRange]ast.Expr
}

func (c *transferChecker) VisitExpr(srcExpr ast.Expr) {
	if srcRange, haveSrc := c.srcInfo.GetOffsetRange(srcExpr.ID()); haveSrc {
		if srcRange.Start == 0 {
			// Ignore synthetic "true" default condition which has an incorrect source location
			return
		}
		dstExpr, haveDst := (*c.ranges)[srcRange]
		if !haveDst {
			c.t.Errorf("composed node not found for rule node: %s", debug.ToDebugString(srcExpr))
			return
		}

		// Check that the two nodes have the same textual representation
		if dstExpr.Kind() == ast.IdentKind && strings.HasPrefix(dstExpr.AsIdent(), "@index") {
			// Skip the check for unnested vars
			return
		}
		dstStr, err := cel.ExprToString(dstExpr, c.dstInfo)
		if err != nil {
			c.t.Errorf("failed to convert dstExpr")
			return
		}
		srcStr, err := cel.ExprToString(srcExpr, c.srcInfo)
		if err != nil {
			c.t.Errorf("failed to convert srcExpr")
			return
		}
		if srcStr != dstStr {
			c.t.Errorf("mismatched nodes, rule: %s composed: %s", srcStr, dstStr)
		}
	}
}

func (c *transferChecker) VisitEntryExpr(ast.EntryExpr) {
}
