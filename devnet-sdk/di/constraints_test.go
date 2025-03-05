package di_test

import (
	"reflect"
	"testing"

	"github.com/ethereum-optimism/optimism/devnet-sdk/di"
)

// Helper types for testing constraint solving
type TestInt int
type TestString string

// TestStruct with different field types and constraints
type TestConstraintStruct struct {
	Int       *TestInt    `di:"version=versionGreaterThan(1.0)"`
	String    *TestString `di:"env=production"`
	SharedEnv *TestString `di:"env=$env"`
	OtherEnv  *TestString `di:"env=$env"`
}

func TestSelectProviders(t *testing.T) {
	t.Run("SimpleConstraints", func(t *testing.T) {
		container := di.NewContainer()

		// Register providers
		container.RegisterProvider(func() *TestInt { return intPtr(42) }, map[string]interface{}{
			"version": "2.0",
		}, "singleton")

		container.RegisterProvider(func() *TestString { return strPtr("production-string") }, map[string]interface{}{
			"env": "production",
		}, "singleton")

		// Resolve a struct using constraints
		result, err := di.ProvideStruct[TestConstraintStruct](container)
		if err != nil {
			t.Fatalf("Failed to resolve struct: %v", err)
		}

		// Check int
		if result.Int == nil || *result.Int != 42 {
			t.Errorf("Expected Int to be 42, got %v", result.Int)
		}

		// Check string
		if result.String == nil || *result.String != "production-string" {
			t.Errorf("Expected String to be 'production-string', got %v", result.String)
		}
	})

	t.Run("MultipleProvidersWithFilters", func(t *testing.T) {
		container := di.NewContainer()

		// Register multiple providers for the same type with different metadata
		container.RegisterProvider(func() *TestInt { return intPtr(10) }, map[string]interface{}{
			"version": "0.5",
		}, "singleton")

		container.RegisterProvider(func() *TestInt { return intPtr(20) }, map[string]interface{}{
			"version": "1.5",
		}, "singleton")

		container.RegisterProvider(func() *TestInt { return intPtr(30) }, map[string]interface{}{
			"version": "2.0",
		}, "singleton")

		// Resolve a struct using a filter to select a specific provider
		type FilterStruct struct {
			Value *TestInt `di:"version=versionGreaterThan(1.0)"`
		}

		result, err := di.ProvideStruct[FilterStruct](container)
		if err != nil {
			t.Fatalf("Failed to resolve struct: %v", err)
		}

		// We should get the first provider that matches the filter (version > 1.0)
		if result.Value == nil || *result.Value != 20 {
			t.Errorf("Expected Value to be 20, got %v", result.Value)
		}
	})

	t.Run("PlaceholderConsistency", func(t *testing.T) {
		container := di.NewContainer()

		// Register multiple providers for TestString
		container.RegisterProvider(func() *TestString { return strPtr("dev-string") }, map[string]interface{}{
			"env": "development",
		}, "singleton")

		container.RegisterProvider(func() *TestString { return strPtr("prod-string") }, map[string]interface{}{
			"env": "production",
		}, "singleton")

		// Resolve struct with two fields using the same placeholder
		result, err := di.ProvideStruct[struct {
			String1 *TestString `di:"env=$environment"`
			String2 *TestString `di:"env=$environment"`
		}](container)
		if err != nil {
			t.Fatalf("Failed to resolve struct: %v", err)
		}

		// Both fields should have the same environment value
		if *result.String1 != *result.String2 {
			t.Errorf("Expected String1 and String2 to have the same value, got '%s' and '%s'",
				*result.String1, *result.String2)
		}
	})

	t.Run("NoMatchingProvider", func(t *testing.T) {
		container := di.NewContainer()

		// Register a provider with version 1.0
		container.RegisterProvider(func() *TestInt { return intPtr(10) }, map[string]interface{}{
			"version": "1.0",
		}, "singleton")

		// Try to resolve a struct requiring version > 2.0
		_, err := di.ProvideStruct[struct {
			Value *TestInt `di:"version=versionGreaterThan(2.0)"`
		}](container)

		// Should fail because no provider matches the constraint
		if err == nil {
			t.Errorf("Expected error when no provider matches constraints, got none")
		}
	})

	t.Run("BacktrackingSelectionAlgorithm", func(t *testing.T) {
		container := di.NewContainer()

		// Register providers with different combinations of metadata
		container.RegisterProvider(func() *TestInt { return intPtr(1) }, map[string]interface{}{
			"version": "1.0",
			"env":     "dev",
		}, "singleton")

		container.RegisterProvider(func() *TestInt { return intPtr(2) }, map[string]interface{}{
			"version": "2.0",
			"env":     "prod",
		}, "singleton")

		container.RegisterProvider(func() *TestString { return strPtr("a") }, map[string]interface{}{
			"version": "1.0",
			"env":     "dev",
		}, "singleton")

		container.RegisterProvider(func() *TestString { return strPtr("b") }, map[string]interface{}{
			"version": "2.0",
			"env":     "prod",
		}, "singleton")

		// Resolve a struct with placeholder constraints that force backtracking
		result, err := di.ProvideStruct[struct {
			Int    *TestInt    `di:"version=$ver,env=$env"`
			String *TestString `di:"version=$ver,env=$env"`
		}](container)
		if err != nil {
			t.Fatalf("Failed to resolve struct: %v", err)
		}

		// The constraint solver should find a consistent assignment
		// Both should be either (1, "a") or (2, "b")
		if (*result.Int == 1 && *result.String != "a") || (*result.Int == 2 && *result.String != "b") {
			t.Errorf("Expected consistent assignment, got Int=%d and String=%s",
				*result.Int, *result.String)
		}
	})
}

func TestCopyAssignment(t *testing.T) {
	t.Run("EmptyMap", func(t *testing.T) {
		empty := make(map[string]map[string]string)
		copy := di.CopyAssignment(empty)

		if len(copy) != 0 {
			t.Errorf("Expected empty map, got map with %d entries", len(copy))
		}

		// Modify original - shouldn't affect copy
		empty["key"] = map[string]string{"inner": "value"}
		if len(copy) != 0 {
			t.Errorf("Copy should not be affected by changes to original")
		}
	})

	t.Run("NestedMap", func(t *testing.T) {
		original := map[string]map[string]string{
			"env": {
				"environment": "production",
			},
			"version": {
				"ver": "2.0",
			},
		}

		copy := di.CopyAssignment(original)

		// Verify copy has same content
		if !reflect.DeepEqual(original, copy) {
			t.Errorf("Copy should have same content as original")
		}

		// Modify original - shouldn't affect copy
		original["env"]["environment"] = "development"
		original["new"] = map[string]string{"key": "value"}

		if copy["env"]["environment"] != "production" {
			t.Errorf("Copy should not be affected by changes to original values")
		}

		if _, exists := copy["new"]; exists {
			t.Errorf("Copy should not be affected by new keys in original")
		}
	})

	t.Run("DeepCopyIsolation", func(t *testing.T) {
		original := map[string]map[string]string{
			"key": {"inner": "value"},
		}

		copy := di.CopyAssignment(original)

		// Modify the copy
		copy["key"]["inner"] = "modified"
		copy["key"]["new"] = "added"

		// Original should be unchanged
		if original["key"]["inner"] != "value" {
			t.Errorf("Original should not be affected by changes to copy values")
		}

		if _, exists := original["key"]["new"]; exists {
			t.Errorf("Original should not be affected by new keys in copy")
		}
	})
}

// Helper functions
func intPtr(i int) *TestInt {
	v := TestInt(i)
	return &v
}

func strPtr(s string) *TestString {
	v := TestString(s)
	return &v
}
