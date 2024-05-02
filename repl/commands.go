// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package repl

import (
	"errors"
	"fmt"
	"strings"

	antlr "github.com/antlr4-go/antlr/v4"

	"github.com/authzed/cel-go/repl/parser"

	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

var (
	compileUsage = `Compile emits a textproto representation of the compiled expression.
%compile <expr>`

	parseUsage = `Parse emits a textproto representation of the parsed expression.
%parse <expr>`

	declareUsage = `Declare introduces a variable or function for type checking, but
doesn't define a value for it:
%declare <identifier> : <type>
%declare <identifier> (<param_identifier> : <param_type>, ...) : <result-type>`

	deleteUsage = `Delete removes a variable or function declaration from the evaluation context.
%delete <identifier>`

	letUsage = `Let introduces a variable or function defined by a sub-CEL expression.
%let <identifier> (: <type>)? = <expr>
%let <identifier> (<param_identifier> : <param_type>, ...) : <result-type> -> <expr>`

	optionUsage = `Option enables a CEL environment option which enables configuration and
optional language features.
%option --container 'google.protobuf'
%option --extension 'all'`

	loadDescriptorsUsage = `LoadDescriptors loads a protobuf descriptor file (google.protobuf.FileDescriptorSet)
from disk or from a predefined package. Supported packages are "cel-spec-test-types"
(TestAllTypes) and "google-rpc" (AttributeContext).
%load_descriptors 'path/to/descriptor_set.binarypb'
%load_descriptors --pkg 'cel-spec-test-types'`

	exitUsage = `Exit terminates the REPL.
%exit`

	helpUsage = `Help prints usage information for the commands supported by the REPL.
%help`
)

type letVarCmd struct {
	identifier string
	typeHint   *exprpb.Type
	src        string
}

type letFnCmd struct {
	identifier string
	resultType *exprpb.Type
	params     []letFunctionParam
	src        string
}

type delCmd struct {
	identifier string
}

type simpleCmd struct {
	cmd  string
	args []string
}

type compileCmd struct {
	expr string
}

type parseCmd struct {
	expr string
}

type evalCmd struct {
	expr string
}

// Cmder interface provides normalized command name from a repl command.
// Command specifics are available via checked type casting to the specific
// command type.
type Cmder interface {
	// Cmd returns the normalized name for the command.
	Cmd() string
}

func (c *letVarCmd) Cmd() string {
	if c.src == "" {
		return "declare"
	}
	return "let"
}

func (c *letFnCmd) Cmd() string {
	if c.src == "" {
		return "declare"
	}
	return "let"
}

func (c *delCmd) Cmd() string {
	return "delete"
}

func (c *simpleCmd) Cmd() string {
	return c.cmd
}

func (c *compileCmd) Cmd() string {
	return "compile"
}

func (c *parseCmd) Cmd() string {
	return "parse"
}

func (c *evalCmd) Cmd() string {
	return "eval"
}

type commandParseListener struct {
	antlr.DefaultErrorListener
	parser.BaseCommandsListener

	errs  []error
	cmd   Cmder
	usage string
}

func (c *commandParseListener) reportIssue(e error) {
	c.errs = append(c.errs, e)
}

// extractSourceText extracts original text from a parse rule match.
// Preserves original whitespace if possible.
func extractSourceText(ctx antlr.ParserRuleContext) string {
	if ctx.GetStart() == nil || ctx.GetStop() == nil ||
		ctx.GetStart().GetStart() < 0 || ctx.GetStop().GetStop() < 0 {
		// fallback to the normalized parse
		return ctx.GetText()
	}
	s, e := ctx.GetStart().GetStart(), ctx.GetStop().GetStop()
	return ctx.GetStart().GetInputStream().GetText(s, e)
}

// Parse parses a repl command line into a command object. This provides
// the normalized command name plus any parsed parameters (e.g. variable names
// in let statements).
//
// An error is returned if the statement isn't well formed. See the parser
// pacakage for details on the antlr grammar.
func Parse(line string) (Cmder, error) {
	line = strings.TrimSpace(line)
	listener := &commandParseListener{}
	is := antlr.NewInputStream(line)
	lexer := parser.NewCommandsLexer(is)
	lexer.RemoveErrorListeners()
	lexer.AddErrorListener(listener)
	p := parser.NewCommandsParser(antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel))
	p.RemoveErrorListeners()
	p.AddErrorListener(listener)

	antlr.ParseTreeWalkerDefault.Walk(listener, p.StartCommand())

	if len(listener.errs) > 0 {
		errFmt := make([]string, len(listener.errs))
		for i, err := range listener.errs {
			errFmt[i] = err.Error()
		}

		if listener.usage != "" {
			errFmt = append(errFmt, "", "Usage:", listener.usage)
		}
		return nil, fmt.Errorf("invalid command: %v", strings.Join(errFmt, "\n"))
	}
	if listener.cmd.Cmd() == "help" {
		return nil, errors.New(strings.Join([]string{
			compileUsage,
			parseUsage,
			declareUsage,
			deleteUsage,
			letUsage,
			optionUsage,
			loadDescriptorsUsage,
			helpUsage,
			exitUsage,
		}, "\n\n"))
	}
	return listener.cmd, nil
}

// ANTLR interface implementations

// Implement antlr ErrorListener interface for syntax errors.
func (c *commandParseListener) SyntaxError(recognizer antlr.Recognizer, offendingSymbol any, line, column int, msg string, e antlr.RecognitionException) {
	c.errs = append(c.errs, fmt.Errorf("(%d:%d) %s", line, column, msg))
}

// Implement ANTLR interface for commands listener.
func (c *commandParseListener) EnterSimple(ctx *parser.SimpleContext) {
	cmd := "undefined"
	if ctx.GetCmd() != nil {
		cmd = ctx.GetCmd().GetText()[1:]
	}
	var args []string
	for _, arg := range ctx.GetArgs() {
		a := arg.GetText()
		if strings.HasPrefix(a, "-") {
			a = "--" + strings.ToLower(strings.TrimLeft(a, "-"))
		} else {
			a = strings.Trim(a, "\"'")
		}
		args = append(args, a)

	}
	c.cmd = &simpleCmd{cmd: cmd, args: args}
}

func (c *commandParseListener) EnterHelp(ctx *parser.HelpContext) {
	c.cmd = &simpleCmd{cmd: "help"}
}

func (c *commandParseListener) EnterEmpty(ctx *parser.EmptyContext) {
	c.cmd = &simpleCmd{cmd: "null"}
}

func (c *commandParseListener) EnterLet(ctx *parser.LetContext) {
	c.usage = letUsage
	if ctx.GetFn() != nil {
		c.cmd = &letFnCmd{}
	} else if ctx.GetVar_() != nil {
		c.cmd = &letVarCmd{}
	} else {
		c.errs = append(c.errs, fmt.Errorf("missing declaration in let"))
	}
}

func (c *commandParseListener) EnterDeclare(ctx *parser.DeclareContext) {
	c.usage = declareUsage
	if ctx.GetFn() != nil {
		c.cmd = &letFnCmd{}
	} else if ctx.GetVar_() != nil {
		c.cmd = &letVarCmd{}
	} else {
		c.errs = append(c.errs, fmt.Errorf("missing declaration in declare"))
	}
}

func (c *commandParseListener) ExitDeclare(ctx *parser.DeclareContext) {
	var typeHint *exprpb.Type
	switch cmd := c.cmd.(type) {
	case *letVarCmd:
		typeHint = cmd.typeHint
	case *letFnCmd:
		typeHint = cmd.resultType
	}
	if typeHint == nil {
		c.reportIssue(errors.New("result type required for declare"))
	}
}

func (c *commandParseListener) EnterDelete(ctx *parser.DeleteContext) {
	c.usage = deleteUsage
	if ctx.GetVar_() == nil && ctx.GetFn() == nil {
		c.reportIssue(errors.New("missing identifier in delete"))
		return
	}
	c.cmd = &delCmd{}
}

func (c *commandParseListener) EnterCompile(ctx *parser.CompileContext) {
	c.cmd = &compileCmd{}
}

func (c *commandParseListener) EnterParse(ctx *parser.ParseContext) {
	c.cmd = &parseCmd{}
}

func (c *commandParseListener) EnterExprCmd(ctx *parser.ExprCmdContext) {
	c.cmd = &evalCmd{}
}

func (c *commandParseListener) ExitFnDecl(ctx *parser.FnDeclContext) {
	switch cmd := c.cmd.(type) {
	case *letFnCmd:
		if ctx.GetId() == nil {
			c.reportIssue(errors.New("missing identifier in function declaration"))
			return
		}
		if ctx.GetRType() == nil {
			c.reportIssue(errors.New("missing result type in function declaration"))
			return
		}
		cmd.identifier = ctx.GetId().GetText()
		ty, err := ParseType(ctx.GetRType().GetText())
		if err != nil {
			c.reportIssue(err)
		}
		for _, p := range ctx.GetParams() {
			if p.GetT() == nil {
				c.reportIssue(errors.New("missing type in function param declaration"))
				continue
			}
			if p.GetPid() == nil {
				c.reportIssue(errors.New("missing identifier in function param declaration"))
			}
			ty, err := ParseType(p.GetT().GetText())
			if err != nil {
				c.reportIssue(err)
			}
			cmd.params = append(cmd.params, letFunctionParam{
				identifier: p.GetPid().GetText(),
				typeHint:   ty,
			})

		}
		cmd.resultType = ty
	case *delCmd:
		if ctx.GetId() == nil {
			c.reportIssue(errors.New("missing identifier in delete"))
		}
		cmd.identifier = ctx.GetId().GetText()
	default:
		c.reportIssue(errors.New("unexepected function declaration"))
	}
}

func (c *commandParseListener) ExitQualId(ctx *parser.QualIdContext) {
	if ctx.GetRid() == nil {
		c.reportIssue(errors.New("missing root identifier"))
	}
}

func (c *commandParseListener) ExitVarDecl(ctx *parser.VarDeclContext) {
	switch cmd := c.cmd.(type) {
	case *letVarCmd:
		if ctx.GetId() == nil {
			c.reportIssue(errors.New("no identifier in variable declaration"))
			return
		}
		cmd.identifier = ctx.GetId().GetText()
		if ctx.GetT() != nil {
			ty, err := ParseType(ctx.GetT().GetText())
			if err != nil {
				c.reportIssue(err)
			}
			cmd.typeHint = ty
		}
	case *delCmd:
		if ctx.GetId() == nil {
			c.reportIssue(errors.New("missing identifier in delete"))
		}
		cmd.identifier = ctx.GetId().GetText()
	default:
		c.reportIssue(errors.New("unexpected var declaration"))
	}
}

func (c *commandParseListener) ExitExpr(ctx *parser.ExprContext) {
	expr := extractSourceText(ctx)
	switch cmd := c.cmd.(type) {
	case *compileCmd:
		cmd.expr = expr
	case *parseCmd:
		cmd.expr = expr
	case *evalCmd:
		cmd.expr = expr
	case *letFnCmd:
		cmd.src = expr
	case *letVarCmd:
		cmd.src = expr
	default:
		c.reportIssue(errors.New("unexpected CEL expression"))
	}
}
