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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cel.dev/cel-go/cel"
	testpb "github.com/cel-expr/skills/internal/proto"
)

func TestEvaluateCEL(t *testing.T) {
	envJSON := &Config{
		Name: "basic",
		Variables: []*Variable{
			{Name: "user", Type: "string"},
			{Name: "age", Type: "int"},
			{Name: "count", Type: "uint"},
		},
	}
	namespaceEnvJSON := &Config{
		Name: "namespace",
		Variables: []*Variable{
			{Name: "request.name", Type: "string"},
			{Name: "request.path", Type: "string"},
			{Name: "request.method", Type: "string"},
		},
	}

	tests := []struct {
		name         string
		expr         string
		envConfig    *Config
		testCases    []TestCase
		wantContains string
		wantCoverage string
		wantErr      bool
	}{
		{
			name:         "successful evaluation",
			expr:         `user == "Alice" && age > 18`,
			envConfig:    envJSON,
			testCases:    []TestCase{{TestCase: "test-1", Bindings: map[string]any{"user": "Alice", "age": 20.0}, Expected: true}},
			wantContains: `"testCase":"test-1","status":"pass"`,
			wantErr:      false,
			wantCoverage: "Node: 100.00%, Branch: 50.00%",
		},
		{
			name:         "uint variable evaluation",
			expr:         `count > 5u`,
			envConfig:    envJSON,
			testCases:    []TestCase{{TestCase: "test-uint", Bindings: map[string]any{"count": 10.0}, Expected: true}},
			wantContains: `"testCase":"test-uint","status":"pass"`,
			wantErr:      false,
		},
		{
			name:         "failed evaluation (returns false)",
			expr:         `user == "Alice" && age > 18`,
			envConfig:    envJSON,
			testCases:    []TestCase{{TestCase: "test-2", Bindings: map[string]any{"user": "Bob", "age": 20}, Expected: false}},
			wantContains: `"testCase":"test-2","status":"pass"`,
			wantErr:      false,
			wantCoverage: "Node: 57.14%, Branch: 33.33%",
		},
		{
			name:         "invalid bindings json",
			expr:         `user == "Alice"`,
			envConfig:    envJSON,
			testCases:    []TestCase{{TestCase: "test-3", Bindings: map[string]any{"user": "Bob"}, Expected: false}},
			wantErr:      false,
			wantCoverage: "Node: 100.00%, Branch: 50.00%",
		},
		{
			name:         "missing variables in binding",
			expr:         `user == "Alice" && age > 18`,
			envConfig:    envJSON,
			testCases:    []TestCase{{TestCase: "test-4", Bindings: map[string]any{"user": "Alice"}, Expected: false}},
			wantContains: `"status":"no such attribute(s): age"`,
			wantErr:      false, // Evaluate is resilient to per-test failures but catches them in the result status
		},
		{
			name:         "runtime error evaluation",
			expr:         `1 / 0 == 0`,
			envConfig:    envJSON,
			testCases:    []TestCase{{TestCase: "test-divzero", Bindings: map[string]any{}, Expected: false}},
			wantContains: `division by zero`,
			wantErr:      false,
		},
		{
			name:         "unexpected output type conversion",
			expr:         `type(1)`,
			envConfig:    envJSON,
			testCases:    []TestCase{{TestCase: "test-type-val", Bindings: map[string]any{}, Expected: "int"}},
			wantContains: `unexpected output type`,
			wantErr:      false,
		},
		{
			name:      "compile error due to bad syntax",
			expr:      `user ==`,
			envConfig: envJSON,
			testCases: []TestCase{},
			wantErr:   true,
		},
		{
			name:         "evaluation returns null",
			expr:         `null`,
			envConfig:    envJSON,
			testCases:    []TestCase{{TestCase: "test-null", Bindings: map[string]any{}, Expected: nil}},
			wantContains: `"status":"pass"`,
			wantErr:      false,
		},
		{
			name:         "failed constructing env",
			expr:         "true",
			envConfig:    &Config{Variables: []*Variable{{Name: "a", Type: "invalid"}}},
			testCases:    []TestCase{},
			wantErr:      true,
			wantCoverage: "Node: 100.00%, Branch: 100.00%",
		},
		{
			name:         "namespace evaluation",
			expr:         `request.name == "test"`,
			envConfig:    namespaceEnvJSON,
			testCases:    []TestCase{{TestCase: "test-namespace", Bindings: map[string]any{"request.name": "test"}, Expected: true}},
			wantContains: `"status":"pass"`,
			wantErr:      false,
		},
		{
			name:         "standalone expression without env config",
			expr:         `"hello".size() == 5`,
			envConfig:    nil,
			testCases:    []TestCase{{TestCase: "test-standalone", Expected: true}},
			wantContains: `"testCase":"test-standalone","status":"pass"`,
			wantErr:      false,
			wantCoverage: "Node: 100.00%, Branch: 50.00%",
		},
		{
			name:         "standalone expression with omitted bindings",
			expr:         `1 + 1`,
			envConfig:    envJSON,
			testCases:    []TestCase{{TestCase: "test-no-bindings", Expected: 2.0}},
			wantContains: `"testCase":"test-no-bindings","status":"pass"`,
			wantErr:      false,
		},
		{
			name:         "evaluation result does not match expected",
			expr:         `user == "Alice"`,
			envConfig:    envJSON,
			testCases:    []TestCase{{TestCase: "test-mismatch", Bindings: map[string]any{"user": "Bob"}, Expected: true}},
			wantContains: `"status":"failed: got false, expected true"`,
			wantErr:      false,
		},
		{
			name: "stdlib subsetting evaluation",
			expr: `user.startsWith("A")`,
			envConfig: &Config{
				Name: "subset",
				StdLib: &LibrarySubset{
					ExcludeFunctions: []*FunctionSubset{{Name: "matches"}, {Name: "dyn"}},
				},
				Variables: []*Variable{{Name: "user", Type: "string"}},
			},
			testCases:    []TestCase{{TestCase: "test-subset", Bindings: map[string]any{"user": "Alice"}, Expected: true}},
			wantContains: `"testCase":"test-subset","status":"pass"`,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EvaluateCEL(tt.expr, tt.envConfig, tt.testCases)
			if (err != nil) != tt.wantErr {
				t.Errorf("EvaluateCEL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.wantContains != "" {
				gotBytes, _ := json.Marshal(got)
				gotStr := string(gotBytes)
				if !strings.Contains(gotStr, tt.wantContains) {
					t.Errorf("EvaluateCEL() got = %v, want it to contain %v", gotStr, tt.wantContains)
				}
			}
			if !tt.wantErr && tt.wantCoverage != "" {
				if got.Coverage != tt.wantCoverage {
					t.Errorf("EvaluateCEL() coverage = %v, want %v", got.Coverage, tt.wantCoverage)
				}
			}
		})
	}
}

func TestEvaluateTestData(t *testing.T) {
	// 1. Read testdata/request_headers_env.json
	envData, err := os.ReadFile(filepath.Join("testdata", "request_headers_env.json"))
	if err != nil {
		t.Fatalf("failed to read env config: %v", err)
	}
	var envConfig Config
	if err := json.Unmarshal(envData, &envConfig); err != nil {
		t.Fatalf("failed to unmarshal env config: %v", err)
	}

	// 2. Read testdata/user_agent_mozilla.cel
	exprData, err := os.ReadFile(filepath.Join("testdata", "user_agent_mozilla.cel"))
	if err != nil {
		t.Fatalf("failed to read expression: %v", err)
	}
	expr := string(exprData)

	// 3. Read testdata/user_agent_test.json
	testCasesData, err := os.ReadFile(filepath.Join("testdata", "user_agent_test.json"))
	if err != nil {
		t.Fatalf("failed to read test cases: %v", err)
	}
	var testCases []TestCase
	if err := json.Unmarshal(testCasesData, &testCases); err != nil {
		t.Fatalf("failed to unmarshal test cases: %v", err)
	}

	// 4. Call EvaluateCEL
	got, err := EvaluateCEL(expr, &envConfig, testCases)
	if err != nil {
		t.Fatalf("EvaluateCEL() error = %v", err)
	}

	// 5. Verify the output matches the expectations
	// We expect all 3 test cases to pass (status: "pass")
	for _, res := range got.TestResults {
		if res.Status != "pass" {
			t.Errorf("test case '%s' failed: status = %s", res.TestCase, res.Status)
		}
	}

	// Verify count
	if len(got.TestResults) != 3 {
		t.Errorf("expected 3 evaluation results, got %d", len(got.TestResults))
	}
}

func TestEvaluateCEL_WithProtobufInputs(t *testing.T) {
	envConfig := &Config{
		Name: "pb_test",
		Variables: []*Variable{
			{Name: "msg", Type: "cel.skills.internal.proto.TestMessage"},
		},
	}
	expr := `msg.single_int32 == 42 && msg.single_nested_message.bb == 100`

	testCases := []TestCase{
		{
			TestCase: "pb-case-pass",
			Bindings: map[string]any{
				"msg": map[string]any{
					"singleInt32": 42,
					"singleNestedMessage": map[string]any{
						"bb": 100,
					},
				},
			},
			Expected: true,
		},
		{
			TestCase: "pb-direct-proto-instance",
			Bindings: map[string]any{
				"msg": &testpb.TestMessage{
					SingleInt32: 42,
					SingleNestedMessage: &testpb.TestMessage_NestedMessage{
						Bb: 100,
					},
				},
			},
			Expected: true,
		},
		{
			TestCase: "pb-invalid-unmarshal-error",
			Bindings: map[string]any{
				"msg": "invalid_string_not_object",
			},
			Expected: false,
		},
		{
			TestCase: "pb-marshal-error",
			Bindings: map[string]any{
				"msg": make(chan int),
			},
			Expected: false,
		},
	}

	got, err := EvaluateCEL(expr, envConfig, testCases, cel.Types(&testpb.TestMessage{}))
	if err != nil {
		t.Fatalf("EvaluateCEL() error = %v", err)
	}

	if len(got.TestResults) != 4 {
		t.Fatalf("expected 4 results, got %d", len(got.TestResults))
	}
	if got.TestResults[0].Status != "pass" {
		t.Errorf("expected pass for case 0, got %s", got.TestResults[0].Status)
	}
	if got.TestResults[1].Status != "pass" {
		t.Errorf("expected pass for direct proto instance, got %s", got.TestResults[1].Status)
	}
	if !strings.Contains(got.TestResults[2].Status, "failed unmarshaling protobuf input") {
		t.Errorf("expected protobuf unmarshal error for case 2, got %s", got.TestResults[2].Status)
	}
	if !strings.Contains(got.TestResults[3].Status, "failed marshaling JSON value") {
		t.Errorf("expected JSON marshal error for case 3, got %s", got.TestResults[3].Status)
	}
}

func TestEvaluateCEL_WithCustomFunctions(t *testing.T) {
	envData, err := os.ReadFile(filepath.Join("testdata", "cloud_armor.json"))
	if err != nil {
		t.Fatalf("failed reading cloud_armor.json: %v", err)
	}
	var envConfig Config
	if err := json.Unmarshal(envData, &envConfig); err != nil {
		t.Fatalf("failed unmarshaling cloud_armor.json: %v", err)
	}

	expr := `origin.region_code == "US" || inIpRange(origin.ip, "10.0.0.0/8")`
	testCases := []TestCase{
		{
			TestCase: "runnable_short_circuit",
			Bindings: map[string]any{"origin.ip": "10.0.0.1", "origin.region_code": "US"},
			Expected: true,
		},
		{
			TestCase: "unrunnable_custom_function",
			Bindings: map[string]any{"origin.ip": "10.0.0.1", "origin.region_code": "EU"},
			Expected: true,
		},
	}

	got, err := EvaluateCEL(expr, &envConfig, testCases)
	if err != nil {
		t.Fatalf("EvaluateCEL() error = %v", err)
	}

	if len(got.TestResults) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got.TestResults))
	}
	if got.TestResults[0].Status != "pass" {
		t.Errorf("expected runnable_short_circuit to pass, got %q", got.TestResults[0].Status)
	}
	if !strings.HasPrefix(got.TestResults[1].Status, "skipped: unknown function \"inIpRange\"") {
		t.Errorf("expected unrunnable_custom_function to be skipped due to unknown function, got %q", got.TestResults[1].Status)
	}
	if len(got.SkippedTestCases) != 1 || got.SkippedTestCases[0] != "unrunnable_custom_function" {
		t.Errorf("expected SkippedTestCases = [unrunnable_custom_function], got %v", got.SkippedTestCases)
	}
}

