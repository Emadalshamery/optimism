package di_test

import (
	"fmt"
	"testing"

	"github.com/ethereum-optimism/optimism/devnet-sdk/di"
)

func TestVersionGreaterThanFilter(t *testing.T) {
	t.Run("AppliesTo", func(t *testing.T) {
		// Create a container to get the filter
		container := di.NewContainer()

		// Parse a tag to get a filter
		_, _, filters, err := container.ParseTag("version=versionGreaterThan(1.0)")
		if err != nil {
			t.Fatalf("Failed to parse tag: %v", err)
		}

		if len(filters) != 1 {
			t.Fatalf("Expected 1 filter, got %d", len(filters))
		}

		filter := filters[0]

		// Test AppliesTo
		if !filter.AppliesTo("version") {
			t.Errorf("Filter should apply to 'version'")
		}

		if filter.AppliesTo("name") {
			t.Errorf("Filter should not apply to 'name'")
		}
	})

	t.Run("Matches", func(t *testing.T) {
		// Create a container to get the filter
		container := di.NewContainer()

		// Parse a tag to get a filter
		_, _, filters, err := container.ParseTag("version=versionGreaterThan(1.0)")
		if err != nil {
			t.Fatalf("Failed to parse tag: %v", err)
		}

		filter := filters[0]

		// Test matches with various versions
		testCases := []struct {
			version  string
			expected bool
		}{
			{"0.9", false},  // Less than
			{"1.0", false},  // Equal
			{"1.0.1", true}, // Greater than
			{"1.1", true},   // Greater than
			{"2.0", true},   // Much greater than
			{"10.0", true},  // Much greater than
		}

		for _, tc := range testCases {
			result := filter.Matches(tc.version)
			if result != tc.expected {
				t.Errorf("For version %s, expected matching to be %v, got %v",
					tc.version, tc.expected, result)
			}
		}

		// Test with non-string value
		if filter.Matches(42) {
			t.Errorf("Filter should not match non-string value")
		}
	})
}

func TestNameEqualsFilter(t *testing.T) {
	t.Run("AppliesTo", func(t *testing.T) {
		// Create a container to get the filter
		container := di.NewContainer()

		// Parse a tag to get a filter
		_, _, filters, err := container.ParseTag("env=nameEquals(production)")
		if err != nil {
			t.Fatalf("Failed to parse tag: %v", err)
		}

		if len(filters) != 1 {
			t.Fatalf("Expected 1 filter, got %d", len(filters))
		}

		filter := filters[0]

		// Test AppliesTo
		if !filter.AppliesTo("env") {
			t.Errorf("Filter should apply to 'env'")
		}

		if filter.AppliesTo("version") {
			t.Errorf("Filter should not apply to 'version'")
		}
	})

	t.Run("Matches", func(t *testing.T) {
		// Create a container to get the filter
		container := di.NewContainer()

		// Parse a tag to get a filter
		_, _, filters, err := container.ParseTag("env=nameEquals(production)")
		if err != nil {
			t.Fatalf("Failed to parse tag: %v", err)
		}

		filter := filters[0]

		// Test exact matching
		if !filter.Matches("production") {
			t.Errorf("Filter should match 'production'")
		}

		if filter.Matches("Production") {
			t.Errorf("Filter should not match 'Production' (case sensitive)")
		}

		if filter.Matches("development") {
			t.Errorf("Filter should not match 'development'")
		}

		// Test with empty string
		emptyContainer := di.NewContainer()
		_, _, emptyFilters, _ := emptyContainer.ParseTag("env=nameEquals()")
		emptyFilter := emptyFilters[0]

		if !emptyFilter.Matches("") {
			t.Errorf("Empty nameEquals filter should match empty string")
		}

		if emptyFilter.Matches("non-empty") {
			t.Errorf("Empty nameEquals filter should not match non-empty string")
		}

		// Test with non-string value
		if filter.Matches(42) {
			t.Errorf("Filter should not match non-string value")
		}
	})
}

func TestFilterRegistration(t *testing.T) {
	t.Run("RegisterCustomFilter", func(t *testing.T) {
		container := di.NewContainer()

		// Register a custom filter
		container.RegisterFilter("greaterThan", func(key, param string) di.Filter {
			value := 0
			_, err := fmt.Sscanf(param, "%d", &value)
			if err != nil {
				return nil
			}

			return &testGreaterThanFilter{
				key:   key,
				value: value,
			}
		})

		// Parse a tag using the custom filter
		_, _, filters, err := container.ParseTag("age=greaterThan(18)")
		if err != nil {
			t.Fatalf("Failed to parse tag with custom filter: %v", err)
		}

		if len(filters) != 1 {
			t.Fatalf("Expected 1 filter, got %d", len(filters))
		}

		// Check if the filter works
		if !filters[0].AppliesTo("age") {
			t.Errorf("Custom filter should apply to 'age'")
		}

		if !filters[0].Matches(20) {
			t.Errorf("Custom filter should match value greater than threshold")
		}

		if filters[0].Matches(15) {
			t.Errorf("Custom filter should not match value less than threshold")
		}
	})

	t.Run("OverrideExistingFilter", func(t *testing.T) {
		container := di.NewContainer()

		// Override the built-in versionGreaterThan filter
		container.RegisterFilter("versionGreaterThan", func(key, param string) di.Filter {
			// Reverse the logic for testing
			return &testVersionLessThanFilter{
				key:     key,
				version: param,
			}
		})

		// Parse a tag using the overridden filter
		_, _, filters, err := container.ParseTag("version=versionGreaterThan(2.0)")
		if err != nil {
			t.Fatalf("Failed to parse tag with overridden filter: %v", err)
		}

		// Check if the overridden filter works (with reversed logic)
		if !filters[0].Matches("1.0") {
			t.Errorf("Overridden filter should match with reversed logic")
		}

		if filters[0].Matches("3.0") {
			t.Errorf("Overridden filter should not match with reversed logic")
		}
	})
}

// Custom filter implementations for testing

type testGreaterThanFilter struct {
	key   string
	value int
}

func (f *testGreaterThanFilter) AppliesTo(key string) bool {
	return key == f.key
}

func (f *testGreaterThanFilter) Matches(value interface{}) bool {
	v, ok := value.(int)
	if !ok {
		return false
	}
	return v > f.value
}

type testVersionLessThanFilter struct {
	key     string
	version string
}

func (f *testVersionLessThanFilter) AppliesTo(key string) bool {
	return key == f.key
}

func (f *testVersionLessThanFilter) Matches(value interface{}) bool {
	v, ok := value.(string)
	if !ok {
		return false
	}
	return v < f.version
}
