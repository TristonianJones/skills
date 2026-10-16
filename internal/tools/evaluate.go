// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tools

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/functions"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	protojson "google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// UnimplementedFunctionError indicates that evaluation encountered a custom
// function declared in the environment configuration that has no runtime
// implementation.
type UnimplementedFunctionError struct {
	Function string
	Overload string
}

func (e *UnimplementedFunctionError) Error() string {
	if e.Overload != "" && e.Overload != e.Function {
		return fmt.Sprintf("unknown function %q (overload %q)", e.Function, e.Overload)
	}
	return fmt.Sprintf("unknown function %q", e.Function)
}

// TestResult is the results of an evaluation.
type TestResult struct {
	TestCase string `json:"testCase"`
	Status   string `json:"status"`
}

// EvaluateExprOutput is the output of the EvaluateCEL function.
type EvaluateExprOutput struct {
	InputSchema      any          `json:"inputSchema,omitempty"`
	OutputSchema     any          `json:"outputSchema,omitempty"`
	TestResults      []TestResult `json:"testResults,omitempty"`
	SkippedTestCases []string     `json:"skippedTestCases,omitempty"`
	Coverage         string       `json:"coverage,omitempty"`
}

// TestCase is a test case for evaluation.
type TestCase struct {
	TestCase string         `json:"testCase" jsonschema_description:"The name of the test case."`
	Bindings map[string]any `json:"bindings,omitempty" jsonschema_description:"The variable bindings for the expression. May be omitted or empty when the expression does not reference any variables."`
	Expected any            `json:"expected" jsonschema_description:"The expected JSON output value of the expression."`
}

func unimplementedFunctionBindings(env *cel.Env, envConfig *Config) []*functions.Overload {
	if envConfig == nil || len(envConfig.Functions) == 0 {
		return nil
	}
	envFuncs := env.Functions()
	seen := make(map[string]bool)
	var bindings []*functions.Overload

	addBinding := func(op, fnName, overloadID string) {
		if op == "" || seen[op] {
			return
		}
		seen[op] = true
		errVal := types.WrapErr(&UnimplementedFunctionError{
			Function: fnName,
			Overload: overloadID,
		})
		bindings = append(bindings, &functions.Overload{
			Operator: op,
			Unary: func(arg ref.Val) ref.Val {
				return errVal
			},
			Binary: func(lhs, rhs ref.Val) ref.Val {
				return errVal
			},
			Function: func(args ...ref.Val) ref.Val {
				return errVal
			},
		})
	}

	for _, fn := range envConfig.Functions {
		if fn == nil || fn.Name == "" {
			continue
		}
		fnDecl := envFuncs[fn.Name]
		if fnDecl != nil && (fnDecl.HasSingletonBinding() || fnDecl.HasLateBinding()) {
			continue
		}
		boundOverloads := make(map[string]bool)
		hasBoundOverload := false
		if fnDecl != nil {
			for _, od := range fnDecl.OverloadDecls() {
				if od.HasBinding() || od.HasLateBinding() {
					boundOverloads[od.ID()] = true
					hasBoundOverload = true
				}
			}
		}
		var firstUnboundOverloadID string
		for _, ov := range fn.Overloads {
			if ov == nil || ov.ID == "" || boundOverloads[ov.ID] {
				continue
			}
			if firstUnboundOverloadID == "" {
				firstUnboundOverloadID = ov.ID
			}
			addBinding(ov.ID, fn.Name, ov.ID)
		}
		if !hasBoundOverload {
			overloadID := ""
			if len(fn.Overloads) == 1 && firstUnboundOverloadID != "" {
				overloadID = firstUnboundOverloadID
			}
			addBinding(fn.Name, fn.Name, overloadID)
		}
	}
	return bindings
}

// EvaluateCEL evaluates a compiled CEL expression against provided variable bindings.
func EvaluateCEL(expr string, envConfig *Config, testCases []TestCase, opts ...cel.EnvOption) (*EvaluateExprOutput, error) {
	env, err := EnvFromConfig(envConfig, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed constructing env: %w", err)
	}

	ast, iss := env.Compile(expr)
	if iss.Err() != nil {
		return nil, fmt.Errorf("compile error: %w", iss.Err())
	}

	inputSchema, err := ComputeInputSchema(env, ast)
	if err != nil {
		return nil, fmt.Errorf("failed computing references: %w", err)
	}
	outputSchemaVal := SchemaFromCELType(env, ast.OutputType())

	progOpts := []cel.ProgramOption{cel.EvalOptions(cel.OptTrackState)}
	if unimpl := unimplementedFunctionBindings(env, envConfig); len(unimpl) > 0 {
		progOpts = append(progOpts, cel.Functions(unimpl...))
	}

	prg, err := env.Program(ast, progOpts...)
	if err != nil {
		return nil, fmt.Errorf("program creation error: %w", err)
	}
	coverageTracker := NewCoverageTracker(ast)
	provider := env.CELTypeProvider()
	var results []TestResult
	var skippedTestCases []string
	for _, tc := range testCases {
		bindings := make(map[string]any, len(tc.Bindings))
		var bindingErr error
		for k, v := range tc.Bindings {
			if _, isProto := v.(proto.Message); !isProto && v != nil {
				var typeName string
				if envConfig != nil {
					for _, varCfg := range envConfig.Variables {
						if varCfg.Name == k {
							typeName = varCfg.Type
							break
						}
					}
				}
				if typeName != "" {
					if td, err := parseType(typeName); err == nil && td != nil {
						typeName = td.TypeName
					}
				}
				if typeName != "" {
					switch typeName {
					case "int":
						if f, ok := v.(float64); ok {
							bindings[k] = int64(f)
							continue
						}
					case "uint":
						if f, ok := v.(float64); ok {
							bindings[k] = uint64(f)
							continue
						}
					}
					if _, found := provider.FindStructType(typeName); found {
						emptyVal := provider.NewValue(typeName, map[string]ref.Val{})
						if emptyVal != nil && !types.IsError(emptyVal) {
							if pbMsg, ok := emptyVal.Value().(proto.Message); ok && pbMsg != nil {
								jsonBytes, err := json.Marshal(v)
								if err != nil {
									bindingErr = fmt.Errorf("failed marshaling JSON value for binding %q: %w", k, err)
									break
								}
								msg := pbMsg.ProtoReflect().New().Interface()
								unopts := protojson.UnmarshalOptions{DiscardUnknown: true}
								if err := unopts.Unmarshal(jsonBytes, msg); err != nil {
									bindingErr = fmt.Errorf("failed unmarshaling protobuf input from JSON value for binding %q (type %s): %w", k, typeName, err)
									break
								}
								bindings[k] = msg
								continue
							}
						}
					}
				}
			}
			bindings[k] = v
		}
		if bindingErr != nil {
			results = append(results, TestResult{
				TestCase: tc.TestCase,
				Status:   bindingErr.Error(),
			})
			continue
		}
		out, details, err := prg.Eval(bindings)
		status := "undefined"
		if err != nil {
			var unimplErr *UnimplementedFunctionError
			if errors.As(err, &unimplErr) {
				coverageTracker.Record(details)
				results = append(results, TestResult{
					TestCase: tc.TestCase,
					Status:   fmt.Sprintf("skipped: %v", unimplErr),
				})
				skippedTestCases = append(skippedTestCases, tc.TestCase)
				continue
			}
			results = append(results, TestResult{
				TestCase: tc.TestCase,
				Status:   err.Error(),
			})
			continue
		}
		if out == nil {
			results = append(results, TestResult{
				TestCase: tc.TestCase,
				Status:   status,
			})
			continue
		}
		coverageTracker.Record(details)
		val, err := out.ConvertToNative(types.JSONValueType)
		if err != nil {
			status = fmt.Sprintf("unexpected output type: %v", out.Value())
			results = append(results, TestResult{
				TestCase: tc.TestCase,
				Status:   status,
			})
			continue
		}
		valPB := protojson.Format(val.(proto.Message))
		var valJSON any
		err = json.Unmarshal([]byte(valPB), &valJSON)
		if err != nil {
			status = fmt.Sprintf("unexpected output type: %v", out.Value())
			results = append(results, TestResult{
				TestCase: tc.TestCase,
				Status:   status,
			})
			continue
		}
		eq := reflect.DeepEqual(valJSON, tc.Expected)
		if eq {
			status = "pass"
		} else {
			status = fmt.Sprintf("failed: got %v, expected %v", valJSON, tc.Expected)
		}
		results = append(results, TestResult{
			TestCase: tc.TestCase,
			Status:   status,
		})
	}

	report := coverageTracker.GenerateReport()
	outputSchema := EvaluateExprOutput{
		InputSchema:      inputSchema,
		OutputSchema:     outputSchemaVal,
		TestResults:      results,
		SkippedTestCases: skippedTestCases,
		Coverage:         fmt.Sprintf("Node: %.2f%%, Branch: %.2f%%", report.NodeCoverage(), report.BranchCoverage()),
	}

	return &outputSchema, nil
}
