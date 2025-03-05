package di_test

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/ethereum-optimism/optimism/devnet-sdk/di"
)

// Helper function to create a new container for each test
func newTestContainer() *di.Container {
	return di.NewContainer()
}

// Testdi.ProviderRegistrationWithParameters tests registering di.Providers with parameters and metadata.
func TestProviderRegistrationWithParameters(t *testing.T) {
	t.Run("RegisterProviderWithParametersAndMetadata", func(t *testing.T) {
		c := newTestContainer()
		metadata := map[string]interface{}{"version": "1.0", "name": "test"}
		c.RegisterProvider(func(a *int, b *string) *float64 { return new(float64) }, metadata, "singleton")
		providers := c.GetProviders(reflect.TypeOf((*float64)(nil)))
		if len(providers) != 1 {
			t.Fatalf("Expected 1 di.Provider, got %d", len(providers))
		}
		if len(providers[0].ParamTypes) != 2 {
			t.Errorf("Expected 2 parameters, got %d", len(providers[0].ParamTypes))
		}
		if providers[0].ParamTypes[0] != reflect.TypeOf((*int)(nil)) {
			t.Errorf("Expected first parameter to be *int, got %v", providers[0].ParamTypes[0])
		}
		if providers[0].ParamTypes[1] != reflect.TypeOf((*string)(nil)) {
			t.Errorf("Expected second parameter to be *string, got %v", providers[0].ParamTypes[1])
		}
		if providers[0].Metadata["version"] != "1.0" {
			t.Errorf("Expected version to be 1.0, got %v", providers[0].Metadata["version"])
		}
		if providers[0].Metadata["name"] != "test" {
			t.Errorf("Expected name to be test, got %v", providers[0].Metadata["name"])
		}
	})

	t.Run("RegisterProviderWithoutParameters", func(t *testing.T) {
		c := newTestContainer()
		c.RegisterProvider(func() *int { return new(int) }, nil, "singleton")
		providers := c.GetProviders(reflect.TypeOf((*int)(nil)))
		if len(providers) != 1 {
			t.Fatalf("Expected 1 di.Provider, got %d", len(providers))
		}
		if len(providers[0].ParamTypes) != 0 {
			t.Errorf("Expected 0 parameters, got %d", len(providers[0].ParamTypes))
		}
	})
}

// TestDependencyResolutionWithParameters tests resolving dependencies for di.Providers with parameters.
func TestDependencyResolutionWithParameters(t *testing.T) {
	t.Run("Resolvedi.ProviderWithParameters", func(t *testing.T) {
		c := newTestContainer()
		c.RegisterProvider(func() *int { v := 42; return &v }, nil, "singleton")
		c.RegisterProvider(func() *string { s := "test"; return &s }, nil, "singleton")
		c.RegisterProvider(func(a *int, b *string) *float64 {
			f := float64(*a) + 0.5
			if *b != "test" {
				t.Errorf("Expected 'test', got %s", *b)
			}
			return &f
		}, nil, "prototype")
		c.Build()
		val, err := di.Provide[*float64](c, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if *val != 42.5 {
			t.Errorf("Expected 42.5, got %f", *val)
		}
	})

	t.Run("ResolveNestedDependencies", func(t *testing.T) {
		c := newTestContainer()
		c.RegisterProvider(func() *int { v := 10; return &v }, nil, "singleton")
		c.RegisterProvider(func(a *int) *string {
			s := fmt.Sprintf("value: %d", *a)
			return &s
		}, nil, "singleton")
		c.RegisterProvider(func(s *string) *float64 {
			if *s != "value: 10" {
				t.Errorf("Expected 'value: 10', got %s", *s)
			}
			f := 20.0
			return &f
		}, nil, "singleton")
		c.Build()
		val, err := di.Provide[*float64](c, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if *val != 20.0 {
			t.Errorf("Expected 20.0, got %f", *val)
		}
	})
}

// TestCycleDetection tests the cycle detection mechanism.
func TestCycleDetection(t *testing.T) {
	t.Run("DetectCycle", func(t *testing.T) {
		c := newTestContainer()
		c.RegisterProvider(func(a *float64) *int { return new(int) }, nil, "singleton")
		c.RegisterProvider(func(b *int) *float64 { return new(float64) }, nil, "singleton")
		c.Build()
		_, err := di.Provide[*int](c, nil)
		if err == nil || !strings.Contains(err.Error(), "cycle detected") {
			t.Errorf("Expected cycle detection error, got %v", err)
		}
	})
}

// TestErrorHandlingWithParameters tests error scenarios related to parameters and filters.
func TestErrorHandlingWithParameters(t *testing.T) {
	t.Run("MissingDependency", func(t *testing.T) {
		c := newTestContainer()
		c.RegisterProvider(func(a *int) *float64 { return new(float64) }, nil, "singleton")
		c.Build()
		_, err := di.Provide[*float64](c, nil)
		if err == nil || !strings.Contains(err.Error(), "no provider registered for type") {
			t.Errorf("Expected error for missing dependency, got %v", err)
		}
	})

	t.Run("UnresolvableParameter", func(t *testing.T) {
		c := newTestContainer()
		c.RegisterProvider(func(a *int) *string { return new(string) }, nil, "singleton")
		c.RegisterProvider(func(b *string, c *float64) *int { return new(int) }, nil, "singleton")
		c.Build()
		_, err := di.Provide[*int](c, nil)
		if err == nil || !strings.Contains(err.Error(), "failed to resolve parameter") {
			t.Errorf("Expected error for unresolvable parameter, got %v", err)
		}
	})

	t.Run("Nodi.ProviderMatchesFilter", func(t *testing.T) {
		c := newTestContainer()
		c.RegisterFilter("versionGreaterThan", func(key, param string) di.Filter {
			return &VersionGreaterThanFilter{key: key, version: param}
		})
		c.RegisterProvider(func() *int { v := 42; return &v }, map[string]interface{}{"version": "0.9"}, "singleton")
		c.Build()
		_, err := di.Provide[*int](c, []di.Filter{&VersionGreaterThanFilter{key: "version", version: "1.0"}})
		if err == nil || !strings.Contains(err.Error(), "no provider found matching constraints") {
			t.Errorf("Expected error for no matching di.Provider, got %v", err)
		}
	})
}

// TestScopesWithParameters tests scopes for di.Providers with parameters.
func TestScopesWithParameters(t *testing.T) {
	t.Run("SingletonScopeWithParameters", func(t *testing.T) {
		c := newTestContainer()
		c.RegisterProvider(func() *int { v := 42; return &v }, nil, "singleton")
		c.RegisterProvider(func(a *int) *string {
			s := fmt.Sprintf("value: %d", *a)
			return &s
		}, nil, "singleton")
		c.Build()
		val1, err := di.Provide[*string](c, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		val2, err := di.Provide[*string](c, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if val1 != val2 {
			t.Errorf("Expected same instance for singleton, got different instances")
		}
	})

	t.Run("PrototypeScopeWithParameters", func(t *testing.T) {
		c := newTestContainer()
		c.RegisterProvider(func() *int { v := 42; return &v }, nil, "singleton")
		c.RegisterProvider(func(a *int) *string {
			s := fmt.Sprintf("value: %d", *a)
			return &s
		}, nil, "prototype")
		c.Build()
		val1, err := di.Provide[*string](c, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		val2, err := di.Provide[*string](c, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if val1 == val2 {
			t.Errorf("Expected different instances for prototype, got same instance")
		}
	})
}

// TestMultipleParameters tests di.Providers with multiple parameters.
func TestMultipleParameters(t *testing.T) {
	t.Run("ResolveMultipleParameters", func(t *testing.T) {
		c := newTestContainer()
		c.RegisterProvider(func() *int { v := 10; return &v }, nil, "singleton")
		c.RegisterProvider(func() *string { s := "hello"; return &s }, nil, "singleton")
		c.RegisterProvider(func(a *int, b *string) *float64 {
			f := float64(*a) + 0.5
			if *b != "hello" {
				t.Errorf("Expected 'hello', got %s", *b)
			}
			return &f
		}, nil, "singleton")
		c.Build()
		val, err := di.Provide[*float64](c, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if *val != 10.5 {
			t.Errorf("Expected 10.5, got %f", *val)
		}
	})
}

// TestTypeSafetyWithParameters tests type safety with parameters.
func TestTypeSafetyWithParameters(t *testing.T) {
	t.Run("TypeMismatch", func(t *testing.T) {
		c := newTestContainer()
		c.RegisterProvider(func() *int { v := 42; return &v }, nil, "singleton")
		c.RegisterProvider(func(a *string) *float64 { return new(float64) }, nil, "singleton")
		c.Build()
		_, err := di.Provide[*float64](c, nil)
		if err == nil || !strings.Contains(err.Error(), "failed to resolve parameter") {
			t.Errorf("Expected error for type mismatch, got %v", err)
		}
	})
}

// TestAutoResolveStruct tests auto-resolving structs with the new di.Filter syntax.
func TestAutoResolveStruct(t *testing.T) {
	t.Run("SimpleStruct", func(t *testing.T) {
		type SimpleStruct struct {
			IntValue    *int
			StringValue *string
		}
		c := newTestContainer()
		c.RegisterProvider(func() *int { v := 42; return &v }, nil, "singleton")
		c.RegisterProvider(func() *string { s := "test"; return &s }, nil, "singleton")
		c.Build()
		result, err := di.Provide[SimpleStruct](c, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if *result.IntValue != 42 {
			t.Errorf("Expected IntValue to be 42, got %d", *result.IntValue)
		}
		if *result.StringValue != "test" {
			t.Errorf("Expected StringValue to be 'test', got %s", *result.StringValue)
		}
	})

	t.Run("WithFilterTag", func(t *testing.T) {
		type FilteredStruct struct {
			IntValue *int `di:"version=versionGreaterThan(1.0)"`
		}
		c := newTestContainer()
		c.RegisterFilter("versionGreaterThan", func(key, param string) di.Filter {
			return &VersionGreaterThanFilter{key: key, version: param}
		})
		c.RegisterProvider(func() *int { v := 42; return &v }, map[string]interface{}{"version": "0.9"}, "singleton")
		c.RegisterProvider(func() *int { v := 100; return &v }, map[string]interface{}{"version": "1.1"}, "singleton")
		c.Build()
		result, err := di.Provide[FilteredStruct](c, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if *result.IntValue != 100 {
			t.Errorf("Expected IntValue to be 100 due to di.Filter, got %d", *result.IntValue)
		}
	})

	t.Run("NestedStructWithFilters", func(t *testing.T) {
		type InnerStruct struct {
			IntValue *int `di:"version=versionGreaterThan(1.0)"`
		}
		type OuterStruct struct {
			Inner       InnerStruct
			StringValue *string
		}
		c := newTestContainer()
		c.RegisterFilter("versionGreaterThan", func(key, param string) di.Filter {
			return &VersionGreaterThanFilter{key: key, version: param}
		})
		c.RegisterProvider(func() *int { v := 42; return &v }, map[string]interface{}{"version": "0.9"}, "singleton")
		c.RegisterProvider(func() *int { v := 100; return &v }, map[string]interface{}{"version": "1.1"}, "singleton")
		c.RegisterProvider(func() *string { s := "test"; return &s }, nil, "singleton")
		c.Build()
		result, err := di.Provide[OuterStruct](c, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if *result.Inner.IntValue != 100 {
			t.Errorf("Expected Inner.IntValue to be 100, got %d", *result.Inner.IntValue)
		}
		if *result.StringValue != "test" {
			t.Errorf("Expected StringValue to be 'test', got %s", *result.StringValue)
		}
	})
}

// TestParseTag tests parsing DI tags with the new di.Filter syntax.
func TestParseTag(t *testing.T) {
	c := newTestContainer()
	c.RegisterFilter("versionGreaterThan", func(key, param string) di.Filter {
		return &VersionGreaterThanFilter{key: key, version: param}
	})
	c.RegisterFilter("nameEquals", func(key, param string) di.Filter {
		return &NameEqualsFilter{key: key, name: param}
	})

	t.Run("LiteralsAndFilters", func(t *testing.T) {
		tag := "env=production,version=versionGreaterThan(1.0),name=nameEquals(test),chain_id=$chain"
		literals, placeholders, filters, err := c.ParseTag(tag)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if len(literals) != 1 || literals["env"] != "production" {
			t.Errorf("Expected literals: {env: 'production'}, got %v", literals)
		}
		if len(placeholders) != 1 || placeholders["chain_id"] != "chain" {
			t.Errorf("Expected placeholders: {chain_id: 'chain'}, got %v", placeholders)
		}
		if len(filters) != 2 {
			t.Errorf("Expected 2 filters, got %d", len(filters))
		}
	})

	t.Run("InvalidFilter", func(t *testing.T) {
		tag := "version=unknownFilter(1.0)"
		_, _, _, err := c.ParseTag(tag)
		if err == nil || !strings.Contains(err.Error(), "unknown di.Filter type") {
			t.Errorf("Expected error for unknown di.Filter, got %v", err)
		}
	})
}

// Testdi.ProviderSelectionWithConstraints tests di.Provider selection with the new di.Filter syntax.
func TestProviderSelectionWithConstraints(t *testing.T) {
	t.Run("FilterConstraint", func(t *testing.T) {
		type TestStruct struct {
			IntValue *int `di:"version=versionGreaterThan(1.0)"`
		}
		c := newTestContainer()
		c.RegisterFilter("versionGreaterThan", func(key, param string) di.Filter {
			return &VersionGreaterThanFilter{key: key, version: param}
		})
		c.RegisterProvider(func() *int { v := 42; return &v }, map[string]interface{}{"version": "0.9"}, "singleton")
		c.RegisterProvider(func() *int { v := 100; return &v }, map[string]interface{}{"version": "1.1"}, "singleton")
		c.Build()
		result, err := di.Provide[TestStruct](c, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if *result.IntValue != 100 {
			t.Errorf("Expected IntValue to be 100, got %d", *result.IntValue)
		}
	})

	t.Run("MultipleFilters", func(t *testing.T) {
		type TestStruct struct {
			IntValue *int `di:"version=versionGreaterThan(1.0),name=nameEquals(test)"`
		}
		c := newTestContainer()
		c.RegisterFilter("versionGreaterThan", func(key, param string) di.Filter {
			return &VersionGreaterThanFilter{key: key, version: param}
		})
		c.RegisterFilter("nameEquals", func(key, param string) di.Filter {
			return &NameEqualsFilter{key: key, name: param}
		})
		c.RegisterProvider(func() *int { v := 42; return &v }, map[string]interface{}{"version": "1.1", "name": "test"}, "singleton")
		c.RegisterProvider(func() *int { v := 100; return &v }, map[string]interface{}{"version": "0.9", "name": "test"}, "singleton")
		c.Build()
		result, err := di.Provide[TestStruct](c, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if *result.IntValue != 42 {
			t.Errorf("Expected IntValue to be 42, got %d", *result.IntValue)
		}
	})
}

// TestIntegrationWithExistingFeatures tests integration of filters with scopes.
func TestIntegrationWithExistingFeatures(t *testing.T) {
	t.Run("SingletonWithFilters", func(t *testing.T) {
		type TestStruct struct {
			IntValue *int `di:"version=versionGreaterThan(1.0)"`
		}
		c := newTestContainer()
		c.RegisterFilter("versionGreaterThan", func(key, param string) di.Filter {
			return &VersionGreaterThanFilter{key: key, version: param}
		})
		c.RegisterProvider(func() *int { v := 42; return &v }, map[string]interface{}{"version": "1.1"}, "singleton")
		c.RegisterProvider(func() *int { v := 100; return &v }, map[string]interface{}{"version": "0.9"}, "singleton")
		c.Build()
		result1, err := di.Provide[TestStruct](c, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		result2, err := di.Provide[TestStruct](c, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if *result1.IntValue != 42 || *result2.IntValue != 42 {
			t.Errorf("Expected IntValue to be 42, got %d and %d", *result1.IntValue, *result2.IntValue)
		}
		if result1.IntValue != result2.IntValue {
			t.Errorf("Expected same instance for singleton, got different instances")
		}
	})

	t.Run("PrototypeWithFilters", func(t *testing.T) {
		type TestStruct struct {
			IntValue *int `di:"version=versionGreaterThan(1.0)"`
		}
		c := newTestContainer()
		c.RegisterFilter("versionGreaterThan", func(key, param string) di.Filter {
			return &VersionGreaterThanFilter{key: key, version: param}
		})
		c.RegisterProvider(func() *int { v := 42; return &v }, map[string]interface{}{"version": "1.1"}, "prototype")
		c.RegisterProvider(func() *int { v := 100; return &v }, map[string]interface{}{"version": "0.9"}, "prototype")
		c.Build()
		result1, err := di.Provide[TestStruct](c, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		result2, err := di.Provide[TestStruct](c, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if *result1.IntValue != 42 || *result2.IntValue != 42 {
			t.Errorf("Expected IntValue to be 42, got %d and %d", *result1.IntValue, *result2.IntValue)
		}
		if result1.IntValue == result2.IntValue {
			t.Errorf("Expected different instances for prototype, got same instance")
		}
	})
}

// TestNewCoverageAreas tests additional areas of the DI framework for improved coverage
func TestNewCoverageAreas(t *testing.T) {
	// Test 1: Custom di.Filter Registration and Usage
	t.Run("CustomFilter", func(t *testing.T) {
		type TestStruct struct {
			Age *int `di:"age=greaterThan(18)"`
		}
		c := newTestContainer()
		c.RegisterFilter("greaterThan", func(key, param string) di.Filter {
			value, _ := strconv.Atoi(param)
			return &GreaterThanFilter{key: key, value: value}
		})
		c.RegisterProvider(func() *int { v := 20; return &v }, map[string]interface{}{"age": 20}, "singleton")
		c.RegisterProvider(func() *int { v := 15; return &v }, map[string]interface{}{"age": 15}, "singleton")
		c.Build()
		result, err := di.Provide[TestStruct](c, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if *result.Age != 20 {
			t.Errorf("Expected Age to be 20, got %d", *result.Age)
		}
	})

	// Test 2: Complex di.Filter Combinations
	t.Run("MultipleFiltersOnSameKey", func(t *testing.T) {
		type TestStruct struct {
			IntValue *int `di:"version=versionGreaterThan(1.0),version=versionLessThan(2.0)"`
		}
		c := newTestContainer()
		c.RegisterFilter("versionGreaterThan", func(key, param string) di.Filter {
			return &VersionGreaterThanFilter{key: key, version: param}
		})
		c.RegisterFilter("versionLessThan", func(key, param string) di.Filter {
			return &VersionLessThanFilter{key: key, version: param}
		})
		c.RegisterProvider(func() *int { v := 42; return &v }, map[string]interface{}{"version": "1.5"}, "singleton")
		c.RegisterProvider(func() *int { v := 100; return &v }, map[string]interface{}{"version": "2.5"}, "singleton")
		c.Build()
		result, err := di.Provide[TestStruct](c, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if *result.IntValue != 42 {
			t.Errorf("Expected IntValue to be 42, got %d", *result.IntValue)
		}
	})

	// Test 3: di.Filter Error Handling
	t.Run("NoProviderMatchesFilter", func(t *testing.T) {
		type TestStruct struct {
			IntValue *int `di:"version=versionGreaterThan(3.0)"`
		}
		c := newTestContainer()
		c.RegisterFilter("versionGreaterThan", func(key, param string) di.Filter {
			return &VersionGreaterThanFilter{key: key, version: param}
		})
		c.RegisterProvider(func() *int { v := 42; return &v }, map[string]interface{}{"version": "1.0"}, "singleton")
		c.RegisterProvider(func() *int { v := 100; return &v }, map[string]interface{}{"version": "2.0"}, "singleton")
		c.Build()
		_, err := di.Provide[TestStruct](c, nil)
		if err == nil {
			t.Fatal("Expected error due to no provider matching di.Filter, but got none")
		}
	})

	// Test 4: Placeholder Constraints Across Multiple Fields
	t.Run("PlaceholdersAcrossFields", func(t *testing.T) {
		type TestStruct struct {
			DB1 *Database `di:"env=$env"`
			DB2 *Database `di:"env=$env"`
		}
		c := newTestContainer()
		c.RegisterProvider(func() *Database { return &Database{} }, map[string]interface{}{"env": "production"}, "singleton")
		c.RegisterProvider(func() *Database { return &Database{} }, map[string]interface{}{"env": "development"}, "singleton")
		c.Build()
		result, err := di.Provide[TestStruct](c, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		// Check that DB1 and DB2 have the same env value, not necessarily the same instance
		if result.DB1 != result.DB2 {
			t.Errorf("Expected DB1 and DB2 to have the same env, but got different instances")
		}
	})

	// Test 5: Struct Resolution with Mixed Constraints
	t.Run("MixedConstraints", func(t *testing.T) {
		type TestStruct struct {
			DB     *Database `di:"env=production,version=versionGreaterThan(1.0),chain_id=$chain"`
			Config *Config   `di:"chain_id=$chain"`
		}
		c := newTestContainer()
		c.RegisterFilter("versionGreaterThan", func(key, param string) di.Filter {
			return &VersionGreaterThanFilter{key: key, version: param}
		})
		c.RegisterProvider(func() *Database { return &Database{} }, map[string]interface{}{"env": "production", "version": "1.5", "chain_id": "123"}, "singleton")
		c.RegisterProvider(func() *Config { return &Config{} }, map[string]interface{}{"chain_id": "123"}, "singleton")
		c.Build()
		result, err := di.Provide[TestStruct](c, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if result.DB == nil || result.Config == nil {
			t.Errorf("Expected DB and Config to be resolved, but got nil")
		}
	})

	// Test 6: Metadata Type Handling
	t.Run("MetadataWithIntegers", func(t *testing.T) {
		type TestStruct struct {
			Age *int `di:"age=greaterThan(18)"`
		}
		c := newTestContainer()
		c.RegisterFilter("greaterThan", func(key, param string) di.Filter {
			value, _ := strconv.Atoi(param)
			return &GreaterThanFilter{key: key, value: value}
		})
		c.RegisterProvider(func() *int { v := 20; return &v }, map[string]interface{}{"age": 20}, "singleton")
		c.RegisterProvider(func() *int { v := 15; return &v }, map[string]interface{}{"age": 15}, "singleton")
		c.Build()
		result, err := di.Provide[TestStruct](c, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if *result.Age != 20 {
			t.Errorf("Expected Age to be 20, got %d", *result.Age)
		}
	})

	// Test 7: Provider Selection Logic
	t.Run("ComplexProviderSelection", func(t *testing.T) {
		type TestStruct struct {
			DB1 *Database `di:"env=production,version=versionGreaterThan(1.0)"`
			DB2 *Database `di:"env=development,version=versionLessThan(2.0)"`
		}
		c := newTestContainer()
		c.RegisterFilter("versionGreaterThan", func(key, param string) di.Filter {
			return &VersionGreaterThanFilter{key: key, version: param}
		})
		c.RegisterFilter("versionLessThan", func(key, param string) di.Filter {
			return &VersionLessThanFilter{key: key, version: param}
		})
		c.RegisterProvider(func() *Database { return &Database{} }, map[string]interface{}{"env": "production", "version": "1.5"}, "singleton")
		c.RegisterProvider(func() *Database { return &Database{} }, map[string]interface{}{"env": "development", "version": "1.9"}, "singleton")
		c.RegisterProvider(func() *Database { return &Database{} }, map[string]interface{}{"env": "production", "version": "0.9"}, "singleton")
		c.Build()
		result, err := di.Provide[TestStruct](c, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if result.DB1 == nil || result.DB2 == nil {
			t.Errorf("Expected DB1 and DB2 to be resolved, but got nil")
		}
	})

	// Test 8: Singleton Caching for Structs
	t.Run("SingletonCachingForStructs", func(t *testing.T) {
		type SimpleStruct struct {
			IntValue *int
		}
		c := newTestContainer()
		c.RegisterProvider(func() *int { v := 42; return &v }, nil, "singleton")
		c.Build()
		result1, err := di.Provide[SimpleStruct](c, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		result2, err := di.Provide[SimpleStruct](c, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if result1.IntValue != result2.IntValue {
			t.Errorf("Expected same instance for auto-resolved struct, got different instances")
		}
	})

	// Test 9: Meta-Providers
	t.Run("MetaProvider", func(t *testing.T) {
		c := newTestContainer()
		mp := func(c *di.Container) {
			c.RegisterProvider(func() *int { v := 42; return &v }, nil, "singleton")
			c.RegisterProvider(func() *string { s := "test"; return &s }, nil, "singleton")
		}
		c.RegisterMetaProvider(mp)
		c.Build()
		valInt, err := di.Provide[*int](c, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		valString, err := di.Provide[*string](c, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if *valInt != 42 || *valString != "test" {
			t.Errorf("Expected 42 and 'test', got %d and %s", *valInt, *valString)
		}
	})
}

// Supporting Types for Tests

type Database struct{}

type Config struct{}

type GreaterThanFilter struct {
	key   string
	value int
}

func (f *GreaterThanFilter) AppliesTo(key string) bool {
	return key == f.key
}

func (f *GreaterThanFilter) Matches(value interface{}) bool {
	if v, ok := value.(int); ok {
		return v > f.value
	}
	return false
}

type VersionLessThanFilter struct {
	key     string
	version string
}

func (f *VersionLessThanFilter) AppliesTo(key string) bool {
	return key == f.key
}

func (f *VersionLessThanFilter) Matches(value interface{}) bool {
	if v, ok := value.(string); ok {
		return v < f.version
	}
	return false
}

type VersionGreaterThanFilter struct {
	key     string
	version string
}

func (f *VersionGreaterThanFilter) AppliesTo(key string) bool {
	return key == f.key
}

func (f *VersionGreaterThanFilter) Matches(value interface{}) bool {
	if v, ok := value.(string); ok {
		return v > f.version
	}
	return false
}

type NameEqualsFilter struct {
	key  string
	name string
}

func (f *NameEqualsFilter) AppliesTo(key string) bool {
	return key == f.key
}

func (f *NameEqualsFilter) Matches(value interface{}) bool {
	if v, ok := value.(string); ok {
		return v == f.name
	}
	return false
}

// Types for main package tests
type (
	MainInt    int
	MainString string
	MainBool   bool

	MainService struct {
		Value int
	}
)

func TestProvide(t *testing.T) {
	t.Run("BasicProvide", func(t *testing.T) {
		container := di.NewContainer()

		// Register a provider
		container.RegisterProvider(func() *MainInt {
			val := MainInt(42)
			return &val
		}, nil, "singleton")

		// Use Provide to resolve it
		val, err := di.Provide[*MainInt](container, nil)
		if err != nil {
			t.Fatalf("Failed to resolve: %v", err)
		}

		if *val != 42 {
			t.Errorf("Expected value 42, got %d", *val)
		}
	})

	t.Run("ProvideWithFilters", func(t *testing.T) {
		container := di.NewContainer()

		// Register providers with different metadata
		container.RegisterProvider(func() *MainInt {
			val := MainInt(1)
			return &val
		}, map[string]interface{}{
			"env": "dev",
		}, "singleton")

		container.RegisterProvider(func() *MainInt {
			val := MainInt(2)
			return &val
		}, map[string]interface{}{
			"env": "prod",
		}, "singleton")

		// Create a filter for prod environment
		_, _, prodFilters, _ := container.ParseTag("env=nameEquals(prod)")

		// Resolve with filter
		val, err := di.Provide[*MainInt](container, prodFilters)
		if err != nil {
			t.Fatalf("Failed to resolve with filter: %v", err)
		}

		if *val != 2 {
			t.Errorf("Expected filtered value 2, got %d", *val)
		}
	})

	t.Run("ProvideError", func(t *testing.T) {
		container := di.NewContainer()

		// Try to resolve a type that hasn't been registered
		_, err := di.Provide[*MainString](container, nil)
		if err == nil {
			t.Errorf("Expected error when resolving unregistered type")
		}
	})
}
