package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Login/activation/public subscription delivery have their own rate-limit and
// access contracts. Public panel/page configuration stays in this matrix.
var authorizationInventoryExcluded = map[string]bool{
	"POST /api/auth/login":                true,
	"POST /api/subscription/activate":     true,
	"POST /api/v1/subscriptions/activate": true,
	"GET /api/sub/{subscription_id}/info": true,
}

func authorizationMuxRegistration(node ast.Node) (*ast.CallExpr, bool) {
	call, ok := node.(*ast.CallExpr)
	if !ok || len(call.Args) < 2 {
		return nil, false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil, false
	}
	return call, selector.Sel.Name == "Handle" || selector.Sel.Name == "HandleFunc"
}

func authorizationRegistrationPattern(node ast.Node) (string, bool) {
	call, ok := authorizationMuxRegistration(node)
	if !ok {
		return "", false
	}
	literal, ok := call.Args[0].(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	pattern, err := strconv.Unquote(literal.Value)
	if err != nil || authorizationInventoryExcluded[pattern] {
		return "", false
	}
	return pattern, strings.Contains(pattern, " /api/")
}

func authorizationRegisteredPatterns(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	requireRepositorySuccess(t, err)
	patterns := map[string]bool{}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, 0)
		requireRepositorySuccess(t, err)
		ast.Inspect(file, func(node ast.Node) bool {
			if pattern, ok := authorizationRegistrationPattern(node); ok {
				patterns[pattern] = true
			}
			return true
		})
	}
	result := make([]string, 0, len(patterns))
	for pattern := range patterns {
		result = append(result, pattern)
	}
	sort.Strings(result)
	return result
}

func TestActualRegisteredRouteAuthorizationInventoryIsComplete(t *testing.T) {
	var expected []string
	seen := map[string]bool{}
	for _, contract := range loadAuthorizationRouteContracts(t) {
		if seen[contract.Pattern] {
			t.Fatalf("duplicate authorization contract: %s", contract.Pattern)
		}
		seen[contract.Pattern] = true
		expected = append(expected, contract.Pattern)
	}
	sort.Strings(expected)
	requireRepositoryEqual(t, "every registered API alias has a frozen authorization contract", authorizationRegisteredPatterns(t), expected)
}
