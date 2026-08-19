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

// Package compiler exposes a standard way to set up a compiler which can be used for CEL
// expressions and policies.
package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"go.yaml.in/yaml/v3"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common"
	"github.com/google/cel-go/common/env"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/ext"
	"github.com/google/cel-go/policy"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	celpb "cel.dev/expr"
	configpb "cel.dev/expr/conformance"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
	descpb "google.golang.org/protobuf/types/descriptorpb"
)

// FileFormat represents the format of the file being loaded.
type FileFormat int

const (
	// Unspecified is used when the file format is not determined.
	Unspecified FileFormat = iota + 1
	// BinaryProto is used for a binary proto file.
	BinaryProto
	// TextProto is used for a text proto file.
	TextProto
	// TextYAML is used for a YAML file.
	TextYAML
	// CELString is used for a CEL string expression defined in a file with .cel extension.
	CELString
	// CELPolicy is used for a CEL policy file with .celpolicy extension.
	CELPolicy
)

// ExpressionType is an enum for the type of input expression.
type ExpressionType int

const (
	// ExpressionTypeUnspecified is used when the expression type is not specified.
	ExpressionTypeUnspecified ExpressionType = iota
	// CompiledExpressionFile is file containing a checked expression.
	CompiledExpressionFile
	// PolicyFile is a file containing a CEL policy.
	PolicyFile
	// ExpressionFile is a file containing a CEL expression.
	ExpressionFile
	// RawExpressionString is a raw CEL expression string.
	RawExpressionString
)

// PolicyMetadataEnvOption represents a function which accepts a policy metadata map and returns an
// environment option used to extend the CEL environment.
//
// The policy metadata map is generally produced as a byproduct of parsing the policy and it can
// be optionally customised by providing a custom policy parser.
type PolicyMetadataEnvOption func(map[string]any) cel.EnvOption

// Compiler interface is used to set up a compiler with the following capabilities:
// - create a CEL environment
// - create a policy parser
// - fetch policy compiler options
// - fetch policy environment options
//
// Note: This compiler is not the same as the CEL expression compiler, rather it provides an
// abstraction layer which can create the different components needed for parsing and compiling CEL
// expressions and policies.
type Compiler interface {
	// CreateEnv creates a singleton CEL environment with the configured environment options.
	CreateEnv() (*cel.Env, error)
	// CreatePolicyParser creates a policy parser using the optionally configured parser options.
	CreatePolicyParser() (*policy.Parser, error)
	// PolicyCompilerOptions returns the policy compiler options.
	PolicyCompilerOptions() []policy.CompilerOption
}

// CustomMetadataCompiler interface is used to set up a compiler with the following capabilities:
// - fetch policy metadata environment options
type CustomMetadataCompiler interface {
	// PolicyMetadataEnvOptions returns the policy metadata environment options.
	PolicyMetadataEnvOptions() []PolicyMetadataEnvOption
}

type compiler struct {
	envOptions               []cel.EnvOption
	policyParserOptions      []policy.ParserOption
	policyCompilerOptions    []policy.CompilerOption
	policyMetadataEnvOptions []PolicyMetadataEnvOption
	env                      *cel.Env
	envErr                   error
	doOnce                   sync.Once
}

// NewCompiler creates a new compiler with a set of functional options.
func NewCompiler(opts ...any) (Compiler, error) {
	c := &compiler{
		envOptions:               []cel.EnvOption{},
		policyParserOptions:      []policy.ParserOption{},
		policyCompilerOptions:    []policy.CompilerOption{},
		policyMetadataEnvOptions: []PolicyMetadataEnvOption{},
	}
	for _, opt := range opts {
		switch opt := opt.(type) {
		case cel.EnvOption:
			c.envOptions = append(c.envOptions, opt)
		case policy.ParserOption:
			c.policyParserOptions = append(c.policyParserOptions, opt)
		case policy.CompilerOption:
			c.policyCompilerOptions = append(c.policyCompilerOptions, opt)
		case PolicyMetadataEnvOption:
			c.policyMetadataEnvOptions = append(c.policyMetadataEnvOptions, opt)
		default:
			return nil, fmt.Errorf("unsupported compiler option: %v", opt)
		}
	}
	c.envOptions = append(c.envOptions, extensionOpt())
	return c, nil
}

func extensionOpt() cel.EnvOption {
	return func(e *cel.Env) (*cel.Env, error) {
		envConfig := &env.Config{
			Extensions: []*env.Extension{
				{Name: "optional", Version: "latest"},
				{Name: "bindings", Version: "latest"},
			},
		}
		return e.Extend(cel.FromConfig(envConfig, ext.ExtensionOptionFactory))
	}
}

// CreateEnv creates a singleton CEL environment with the configured environment options.
func (c *compiler) CreateEnv() (*cel.Env, error) {
	c.doOnce.Do(func() {
		c.env, c.envErr = cel.NewCustomEnv(c.envOptions...)
	})
	return c.env, c.envErr
}

// CreatePolicyParser creates a policy parser using the optionally configured parser options.
func (c *compiler) CreatePolicyParser() (*policy.Parser, error) {
	return policy.NewParser(c.policyParserOptions...)
}

// PolicyCompilerOptions returns the policy compiler options configured in the compiler.
func (c *compiler) PolicyCompilerOptions() []policy.CompilerOption {
	return c.policyCompilerOptions
}

// PolicyMetadataEnvOptions returns the policy metadata environment options configured in the compiler.
func (c *compiler) PolicyMetadataEnvOptions() []PolicyMetadataEnvOption {
	return c.policyMetadataEnvOptions
}

func loadFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %q: %v", path, err)
	}
	return data, err
}

func loadProtoFile(path string, format FileFormat, out protoreflect.ProtoMessage) error {
	unmarshaller := proto.Unmarshal
	if format == TextProto {
		unmarshaller = prototext.Unmarshal
	}
	data, err := loadFile(path)
	if err != nil {
		return err
	}
	return unmarshaller(data, out)
}

// InferFileFormat infers the file format from the file path.
func InferFileFormat(path string) FileFormat {
	extension := filepath.Ext(path)
	switch extension {
	case ".textproto":
		return TextProto
	case ".yaml":
		return TextYAML
	case ".binarypb", ".fds", ".pb":
		return BinaryProto
	case ".cel":
		return CELString
	case ".celpolicy":
		return CELPolicy
	default:
		return Unspecified
	}
}

// EnvironmentFile returns an EnvOption which loads a serialized CEL environment from a file.
// The file must be in one of the following formats:
// - Textproto
// - YAML
// - Binarypb
func EnvironmentFile(path string) cel.EnvOption {
	return func(e *cel.Env) (*cel.Env, error) {
		format := InferFileFormat(path)
		if format != TextProto && format != TextYAML && format != BinaryProto {
			return nil, fmt.Errorf("file extension must be one of .textproto, .yaml, .binarypb: found %v", format)
		}
		var envConfig *env.Config
		var fileDescriptorSet *descpb.FileDescriptorSet
		switch format {
		case TextProto, BinaryProto:
			pbEnv := &configpb.Environment{}
			var err error
			if err = loadProtoFile(path, format, pbEnv); err != nil {
				return nil, err
			}
			envConfig, fileDescriptorSet, err = envProtoToConfig(pbEnv)
			if err != nil {
				return nil, err
			}
		case TextYAML:
			envConfig = &env.Config{}
			data, err := loadFile(path)
			if err != nil {
				return nil, err
			}
			err = yaml.Unmarshal(data, envConfig)
			if err != nil {
				return nil, fmt.Errorf("yaml.Unmarshal failed to map CEL environment: %w", err)
			}
		default:
			return nil, fmt.Errorf("unsupported format: %v, file: %s", format, path)
		}
		var envOpts []cel.EnvOption
		if fileDescriptorSet != nil {
			envOpts = append(envOpts, cel.TypeDescs(fileDescriptorSet))
		}
		envOpts = append(envOpts, cel.FromConfig(envConfig, ext.ExtensionOptionFactory))
		var err error
		e, err = e.Extend(envOpts...)
		if err != nil {
			return nil, fmt.Errorf("e.Extend() with env options %v failed: %w", envOpts, err)
		}
		return e, nil
	}
}

func envProtoToConfig(pbEnv *configpb.Environment) (*env.Config, *descpb.FileDescriptorSet, error) {
	if pbEnv == nil {
		return nil, nil, fmt.Errorf("proto environment is not set")
	}
	envConfig := env.NewConfig(pbEnv.GetName())
	envConfig.Description = pbEnv.GetDescription()
	envConfig.SetContainer(pbEnv.GetContainer())
	for _, imp := range pbEnv.GetImports() {
		envConfig.AddImports(env.NewImport(imp.GetName()))
	}
	stdLib, err := envToStdLib(pbEnv)
	if err != nil {
		return nil, nil, err
	}
	envConfig.SetStdLib(stdLib)
	extensions := make([]*env.Extension, 0, len(pbEnv.GetExtensions()))
	for _, extension := range pbEnv.GetExtensions() {
		extensions = append(extensions, &env.Extension{Name: extension.GetName(), Version: extension.GetVersion()})
	}
	envConfig.AddExtensions(extensions...)
	if contextVariable := pbEnv.GetContextVariable(); contextVariable != nil {
		envConfig.SetContextVariable(env.NewContextVariable(contextVariable.GetTypeName()))
	}
	functions, variables, err := protoDeclToFunctionsAndVariables(pbEnv.GetDeclarations())
	if err != nil {
		return nil, nil, err
	}
	envConfig.AddFunctions(functions...)
	envConfig.AddVariables(variables...)
	validators, err := envToValidators(pbEnv)
	if err != nil {
		return nil, nil, err
	}
	envConfig.AddValidators(validators...)
	features, err := envToFeatures(pbEnv)
	if err != nil {
		return nil, nil, err
	}
	envConfig.AddFeatures(features...)
	fileDescriptorSet := pbEnv.GetMessageTypeExtension()
	return envConfig, fileDescriptorSet, nil
}

func envToFeatures(pbEnv *configpb.Environment) ([]*env.Feature, error) {
	features := make([]*env.Feature, 0, len(pbEnv.GetFeatures())+1)
	for _, feature := range pbEnv.GetFeatures() {
		features = append(features, env.NewFeature(feature.GetName(), feature.GetEnabled()))
	}
	if pbEnv.GetEnableMacroCallTracking() {
		features = append(features, env.NewFeature("cel.feature.macro_call_tracking", true))
	}
	return features, nil
}

func envToValidators(pbEnv *configpb.Environment) ([]*env.Validator, error) {
	validators := make([]*env.Validator, 0, len(pbEnv.GetValidators()))
	for _, pbValidator := range pbEnv.GetValidators() {
		validator := env.NewValidator(pbValidator.GetName())
		config := map[string]any{}
		for k, v := range pbValidator.GetConfig() {
			val := types.DefaultTypeAdapter.NativeToValue(v)
			config[k] = val.Value()
		}
		validator.SetConfig(config)
		validators = append(validators, validator)
	}
	return validators, nil
}

func protoDeclToFunctionsAndVariables(declarations []*celpb.Decl) ([]*env.Function, []*env.Variable, error) {
	functions := make([]*env.Function, 0, len(declarations))
	variables := make([]*env.Variable, 0, len(declarations))
	for _, decl := range declarations {
		switch decl.GetDeclKind().(type) {
		case *celpb.Decl_Function:
			fn, err := protoDeclToFunction(decl)
			if err != nil {
				return nil, nil, fmt.Errorf("protoDeclToFunction(%s) failed to create function: %w", decl.GetName(), err)
			}
			functions = append(functions, fn)
		case *celpb.Decl_Ident:
			t, err := types.ProtoAsType(decl.GetIdent().GetType())
			if err != nil {
				return nil, nil, fmt.Errorf("types.ProtoAsType(%s) failed to create type: %w", decl.GetIdent().GetType(), err)
			}
			variables = append(variables, env.NewVariable(decl.GetName(), env.SerializeTypeDesc(t)))
		}
	}
	return functions, variables, nil
}

func envToStdLib(pbEnv *configpb.Environment) (*env.LibrarySubset, error) {
	stdLib := env.NewLibrarySubset()
	pbEnvStdLib := pbEnv.GetStdlib()
	if pbEnvStdLib == nil {
		if pbEnv.GetDisableStandardCelDeclarations() {
			stdLib.SetDisabled(true)
			return stdLib, nil
		}
		return nil, nil
	}
	if !stdLib.Disabled {
		stdLib.SetDisabled(pbEnvStdLib.GetDisabled())
	}
	stdLib.SetDisableMacros(pbEnvStdLib.GetDisableMacros())
	stdLib.AddIncludedMacros(pbEnvStdLib.GetIncludeMacros()...)
	stdLib.AddExcludedMacros(pbEnvStdLib.GetExcludeMacros()...)
	if pbEnvStdLib.GetIncludeFunctions() != nil {
		for _, includeFn := range pbEnvStdLib.GetIncludeFunctions() {
			if includeFn.GetFunction() != nil {
				fn, err := protoDeclToFunction(includeFn)
				if err != nil {
					return nil, err
				}
				stdLib.AddIncludedFunctions(fn)
			} else {
				return nil, fmt.Errorf("IncludeFunctions must specify a function decl")
			}
		}
	}
	if pbEnvStdLib.GetExcludeFunctions() != nil {
		for _, excludeFn := range pbEnvStdLib.GetExcludeFunctions() {
			if excludeFn.GetFunction() != nil {
				fn, err := protoDeclToFunction(excludeFn)
				if err != nil {
					return nil, err
				}
				stdLib.AddExcludedFunctions(fn)
			} else {
				return nil, fmt.Errorf("ExcludeFunctions must specify a function decl")
			}
		}
	}
	return stdLib, nil
}

func protoDeclToFunction(decl *celpb.Decl) (*env.Function, error) {
	declFn := decl.GetFunction()
	if declFn == nil {
		return nil, nil
	}
	overloads := make([]*env.Overload, 0, len(declFn.GetOverloads()))
	for _, o := range declFn.GetOverloads() {
		args := make([]*env.TypeDesc, 0, len(o.GetParams()))
		for _, p := range o.GetParams() {
			t, err := types.ProtoAsType(p)
			if err != nil {
				return nil, err
			}
			args = append(args, env.SerializeTypeDesc(t))
		}
		res, err := types.ProtoAsType(o.GetResultType())
		if err != nil {
			return nil, err
		}
		ret := env.SerializeTypeDesc(res)
		if o.IsInstanceFunction {
			overloads = append(overloads, env.NewMemberOverload(o.GetOverloadId(), args[0], args[1:], ret))
		} else {
			overloads = append(overloads, env.NewOverload(o.GetOverloadId(), args, ret))
		}
	}
	return env.NewFunction(decl.GetName(), overloads...), nil
}

// TypeDescriptorSetFile returns an EnvOption which loads type descriptors from a file.
// The file must be in binary format.
func TypeDescriptorSetFile(path string) cel.EnvOption {
	return func(e *cel.Env) (*cel.Env, error) {
		format := InferFileFormat(path)
		if format != BinaryProto {
			return nil, fmt.Errorf("type descriptor must be in binary format")
		}
		fds := &descpb.FileDescriptorSet{}
		if err := loadProtoFile(path, BinaryProto, fds); err != nil {
			return nil, err
		}
		var err error
		e, err = e.Extend(cel.TypeDescs(fds))
		if err != nil {
			return nil, fmt.Errorf("e.Extend() with type descriptor set %v failed: %w", fds, err)
		}
		return e, nil
	}
}

// InputExpression is an interface for an expression which can be compiled into a CEL AST and return
// an optional policy metadata map.
type InputExpression interface {
	// CreateAST creates a CEL AST from the input expression using the provided compiler.
	CreateAST(Compiler) (*cel.Ast, map[string]any, error)
}

// CompiledExpression is an InputExpression which loads a CheckedExpr from a file.
type CompiledExpression struct {
	Path string
}

// CreateAST creates a CEL AST from a checked expression file.
// The file must be in one of the following formats:
// - Binarypb
// - Textproto
func (c *CompiledExpression) CreateAST(_ Compiler) (*cel.Ast, map[string]any, error) {
	var expr exprpb.CheckedExpr
	format := InferFileFormat(c.Path)
	if format != BinaryProto && format != TextProto {
		return nil, nil, fmt.Errorf("invalid file extension wanted: .binarypb or .textproto found: %v", format)
	}
	if err := loadProtoFile(c.Path, format, &expr); err != nil {
		return nil, nil, err
	}
	return cel.CheckedExprToAst(&expr), nil, nil
}

// FileExpression is an InputExpression which loads a CEL expression or policy from a file.
type FileExpression struct {
	Path string
}

// CreateAST creates a CEL AST from a file using the provided compiler:
// - All policy metadata options are executed using the policy metadata map to extend the
// environment.
// - All policy compiler options are passed on to compile the parsed policy.
//
// The file must be in one of the following formats:
// - .cel: CEL string expression
// - .celpolicy: CEL policy
func (f *FileExpression) CreateAST(compiler Compiler) (*cel.Ast, map[string]any, error) {
	e, err := compiler.CreateEnv()
	if err != nil {
		return nil, nil, err
	}
	format := InferFileFormat(f.Path)
	switch format {
	case CELString:
		data, err := loadFile(f.Path)
		if err != nil {
			return nil, nil, err
		}
		src := common.NewStringSource(string(data), f.Path)
		ast, iss := e.CompileSource(src)
		if iss.Err() != nil {
			return nil, nil, fmt.Errorf("e.CompileSource(%q) failed: %w", src.Content(), iss.Err())
		}
		return ast, nil, nil
	case CELPolicy, TextYAML:
		data, err := loadFile(f.Path)
		if err != nil {
			return nil, nil, err
		}
		src := policy.ByteSource(data, f.Path)
		parser, err := compiler.CreatePolicyParser()
		if err != nil {
			return nil, nil, err
		}
		p, iss := parser.Parse(src)
		if iss.Err() != nil {
			return nil, nil, fmt.Errorf("parser.Parse(%q) failed: %w", src.Content(), iss.Err())
		}
		policyMetadata := clonePolicyMetadata(p)
		if meta, ok := compiler.(CustomMetadataCompiler); ok {
			for _, opt := range meta.PolicyMetadataEnvOptions() {
				if e, err = e.Extend(opt(policyMetadata)); err != nil {
					return nil, nil, fmt.Errorf("e.Extend() with metadata option failed: %w", err)
				}
			}
		}
		ast, iss := policy.Compile(e, p, compiler.PolicyCompilerOptions()...)
		if iss.Err() != nil {
			return nil, nil, fmt.Errorf("policy.Compile(%q) failed: %w", src.Content(), iss.Err())
		}
		return ast, policyMetadata, nil
	default:
		return nil, nil, fmt.Errorf("invalid file extension wanted: .cel or .celpolicy or .yaml found: %v", format)
	}
}

func clonePolicyMetadata(p *policy.Policy) map[string]any {
	metadataKeys := p.MetadataKeys()
	metadata := make(map[string]any, len(metadataKeys))
	for _, key := range metadataKeys {
		value, _ := p.Metadata(key)
		metadata[key] = value
	}
	return metadata
}

// RawExpression is an InputExpression which loads a CEL expression from a string.
type RawExpression struct {
	Value string
}

// CreateAST creates a CEL AST from a raw CEL expression using the provided compiler.
func (r *RawExpression) CreateAST(compiler Compiler) (*cel.Ast, map[string]any, error) {
	e, err := compiler.CreateEnv()
	if err != nil {
		return nil, nil, err
	}
	format := InferFileFormat(r.Value)
	if format != Unspecified {
		return nil, nil, fmt.Errorf("invalid raw expression found file with extension: %v", format)
	}
	ast, iss := e.Compile(r.Value)
	if iss.Err() != nil {
		return nil, nil, fmt.Errorf("e.Compile(%q) failed: %w", r.Value, iss.Err())
	}
	return ast, nil, nil
}
