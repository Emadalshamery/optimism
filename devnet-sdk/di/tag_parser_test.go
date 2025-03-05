package di_test

import (
	"reflect"
	"testing"

	"github.com/ethereum-optimism/optimism/devnet-sdk/di"
)

func TestParseTagBasic(t *testing.T) {
	container := di.NewContainer()

	t.Run("EmptyTag", func(t *testing.T) {
		literals, placeholders, filters, err := container.ParseTag("")
		if err != nil {
			t.Errorf("Unexpected error parsing empty tag: %v", err)
		}

		if len(literals) != 0 {
			t.Errorf("Expected 0 literals, got %d", len(literals))
		}

		if len(placeholders) != 0 {
			t.Errorf("Expected 0 placeholders, got %d", len(placeholders))
		}

		if len(filters) != 0 {
			t.Errorf("Expected 0 filters, got %d", len(filters))
		}
	})

	t.Run("SingleKeyValue", func(t *testing.T) {
		literals, placeholders, filters, err := container.ParseTag("env=production")
		if err != nil {
			t.Errorf("Unexpected error parsing simple tag: %v", err)
		}

		if len(literals) != 1 {
			t.Errorf("Expected 1 literal, got %d", len(literals))
		}

		if literals["env"] != "production" {
			t.Errorf("Expected literal value 'production', got '%s'", literals["env"])
		}

		if len(placeholders) != 0 {
			t.Errorf("Expected 0 placeholders, got %d", len(placeholders))
		}

		if len(filters) != 0 {
			t.Errorf("Expected 0 filters, got %d", len(filters))
		}
	})

	t.Run("MultipleKeyValues", func(t *testing.T) {
		literals, placeholders, filters, err := container.ParseTag("env=production,version=1.0,name=test")
		if err != nil {
			t.Errorf("Unexpected error parsing multi-key tag: %v", err)
		}

		if len(literals) != 3 {
			t.Errorf("Expected 3 literals, got %d", len(literals))
		}

		expectedLiterals := map[string]string{
			"env":     "production",
			"version": "1.0",
			"name":    "test",
		}

		for key, expectedValue := range expectedLiterals {
			if literals[key] != expectedValue {
				t.Errorf("Expected literal '%s' to be '%s', got '%s'", key, expectedValue, literals[key])
			}
		}

		if len(placeholders) != 0 {
			t.Errorf("Expected 0 placeholders, got %d", len(placeholders))
		}

		if len(filters) != 0 {
			t.Errorf("Expected 0 filters, got %d", len(filters))
		}
	})
}

func TestParseLiteralValues(t *testing.T) {
	container := di.NewContainer()

	t.Run("UnquotedLiterals", func(t *testing.T) {
		tag := "env=production,version=1.0"
		literals, placeholders, filters, err := container.ParseTag(tag)
		if err != nil {
			t.Fatalf("Failed to parse tag: %v", err)
		}

		expected := map[string]string{
			"env":     "production",
			"version": "1.0",
		}

		if !reflect.DeepEqual(literals, expected) {
			t.Errorf("Expected literals %v, got %v", expected, literals)
		}

		if len(placeholders) != 0 {
			t.Errorf("Expected 0 placeholders, got %d", len(placeholders))
		}

		if len(filters) != 0 {
			t.Errorf("Expected 0 filters, got %d", len(filters))
		}
	})

	t.Run("QuotedLiterals", func(t *testing.T) {
		tag := "env='production',name='test name'"
		literals, placeholders, filters, err := container.ParseTag(tag)
		if err != nil {
			t.Fatalf("Failed to parse tag: %v", err)
		}

		expected := map[string]string{
			"env":  "production",
			"name": "test name",
		}

		if !reflect.DeepEqual(literals, expected) {
			t.Errorf("Expected literals %v, got %v", expected, literals)
		}

		if len(placeholders) != 0 {
			t.Errorf("Expected 0 placeholders, got %d", len(placeholders))
		}

		if len(filters) != 0 {
			t.Errorf("Expected 0 filters, got %d", len(filters))
		}
	})

	t.Run("MixedQuotedAndUnquoted", func(t *testing.T) {
		tag := "env=production,name='test name',version=1.0"
		literals, placeholders, filters, err := container.ParseTag(tag)
		if err != nil {
			t.Fatalf("Failed to parse tag: %v", err)
		}

		expected := map[string]string{
			"env":     "production",
			"name":    "test name",
			"version": "1.0",
		}

		if !reflect.DeepEqual(literals, expected) {
			t.Errorf("Expected literals %v, got %v", expected, literals)
		}

		if len(placeholders) != 0 {
			t.Errorf("Expected 0 placeholders, got %d", len(placeholders))
		}

		if len(filters) != 0 {
			t.Errorf("Expected 0 filters, got %d", len(filters))
		}
	})

	t.Run("SpecialCharacters", func(t *testing.T) {
		tag := "path='/some/path',query='key=value&foo=bar'"
		literals, placeholders, filters, err := container.ParseTag(tag)
		if err != nil {
			t.Fatalf("Failed to parse tag: %v", err)
		}

		expected := map[string]string{
			"path":  "/some/path",
			"query": "key=value&foo=bar",
		}

		if !reflect.DeepEqual(literals, expected) {
			t.Errorf("Expected literals %v, got %v", expected, literals)
		}

		if len(placeholders) != 0 {
			t.Errorf("Expected 0 placeholders, got %d", len(placeholders))
		}

		if len(filters) != 0 {
			t.Errorf("Expected 0 filters, got %d", len(filters))
		}
	})
}

func TestParsePlaceholders(t *testing.T) {
	container := di.NewContainer()

	t.Run("SimplePlaceholder", func(t *testing.T) {
		tag := "env=$environment"
		_, placeholders, _, err := container.ParseTag(tag)
		if err != nil {
			t.Fatalf("Failed to parse tag: %v", err)
		}

		if len(placeholders) != 1 {
			t.Fatalf("Expected 1 placeholder, got %d", len(placeholders))
		}

		if placeholders["env"] != "environment" {
			t.Errorf("Expected placeholder 'env' to be 'environment', got '%s'", placeholders["env"])
		}
	})

	t.Run("MultiplePlaceholders", func(t *testing.T) {
		tag := "env=$environment,version=$version"
		_, placeholders, _, err := container.ParseTag(tag)
		if err != nil {
			t.Fatalf("Failed to parse tag: %v", err)
		}

		expected := map[string]string{
			"env":     "environment",
			"version": "version",
		}

		if !reflect.DeepEqual(placeholders, expected) {
			t.Errorf("Expected placeholders %v, got %v", expected, placeholders)
		}
	})

	t.Run("MixedLiteralsAndPlaceholders", func(t *testing.T) {
		tag := "env=$environment,version=1.0,name='test'"
		literals, placeholders, _, err := container.ParseTag(tag)
		if err != nil {
			t.Fatalf("Failed to parse tag: %v", err)
		}

		expectedLiterals := map[string]string{
			"version": "1.0",
			"name":    "test",
		}

		expectedPlaceholders := map[string]string{
			"env": "environment",
		}

		if !reflect.DeepEqual(literals, expectedLiterals) {
			t.Errorf("Expected literals %v, got %v", expectedLiterals, literals)
		}

		if !reflect.DeepEqual(placeholders, expectedPlaceholders) {
			t.Errorf("Expected placeholders %v, got %v", expectedPlaceholders, placeholders)
		}
	})
}

func TestParseFilterExpressions(t *testing.T) {
	container := di.NewContainer()

	t.Run("SimpleFilter", func(t *testing.T) {
		tag := "version=versionGreaterThan(1.0)"
		_, _, filters, err := container.ParseTag(tag)
		if err != nil {
			t.Fatalf("Failed to parse tag: %v", err)
		}

		if len(filters) != 1 {
			t.Fatalf("Expected 1 filter, got %d", len(filters))
		}

		filter := filters[0]
		if !filter.AppliesTo("version") {
			t.Errorf("Filter should apply to 'version'")
		}

		if !filter.Matches("2.0") {
			t.Errorf("Filter should match '2.0'")
		}

		if filter.Matches("0.5") {
			t.Errorf("Filter should not match '0.5'")
		}
	})

	t.Run("MultipleFilters", func(t *testing.T) {
		tag := "version=versionGreaterThan(1.0),env=nameEquals(production)"
		_, _, filters, err := container.ParseTag(tag)
		if err != nil {
			t.Fatalf("Failed to parse tag: %v", err)
		}

		if len(filters) != 2 {
			t.Fatalf("Expected 2 filters, got %d", len(filters))
		}

		// First filter (version)
		if !filters[0].AppliesTo("version") {
			t.Errorf("First filter should apply to 'version'")
		}

		if !filters[0].Matches("2.0") {
			t.Errorf("First filter should match '2.0'")
		}

		// Second filter (env)
		if !filters[1].AppliesTo("env") {
			t.Errorf("Second filter should apply to 'env'")
		}

		if !filters[1].Matches("production") {
			t.Errorf("Second filter should match 'production'")
		}
	})

	t.Run("MultipleFiltersOnSameKey", func(t *testing.T) {
		tag := "version=versionGreaterThan(1.0),version=versionGreaterThan(2.0)"
		_, _, filters, err := container.ParseTag(tag)
		if err != nil {
			t.Fatalf("Failed to parse tag: %v", err)
		}

		if len(filters) != 2 {
			t.Fatalf("Expected 2 filters, got %d", len(filters))
		}

		// All filters should apply to 'version'
		for i, filter := range filters {
			if !filter.AppliesTo("version") {
				t.Errorf("Filter %d should apply to 'version'", i)
			}
		}

		// Check specific matching behaviors
		if !filters[0].Matches("1.5") {
			t.Errorf("First filter should match '1.5'")
		}

		if filters[1].Matches("1.5") {
			t.Errorf("Second filter should not match '1.5'")
		}

		if !filters[1].Matches("3.0") {
			t.Errorf("Second filter should match '3.0'")
		}
	})

	t.Run("MixedLiteralsPlaceholdersAndFilters", func(t *testing.T) {
		tag := "version=versionGreaterThan(1.0),env=$environment,name='test'"
		literals, placeholders, filters, err := container.ParseTag(tag)
		if err != nil {
			t.Fatalf("Failed to parse tag: %v", err)
		}

		// Check literals
		if literals["name"] != "test" {
			t.Errorf("Expected literal 'name' to be 'test', got '%s'", literals["name"])
		}

		// Check placeholders
		if placeholders["env"] != "environment" {
			t.Errorf("Expected placeholder 'env' to be 'environment', got '%s'", placeholders["env"])
		}

		// Check filters
		if len(filters) != 1 {
			t.Fatalf("Expected 1 filter, got %d", len(filters))
		}

		if !filters[0].AppliesTo("version") {
			t.Errorf("Filter should apply to 'version'")
		}
	})
}

func TestParseTagErrors(t *testing.T) {
	container := di.NewContainer()

	t.Run("InvalidFormat", func(t *testing.T) {
		invalidTags := []string{
			"invalid",           // Missing equals
			"key=",              // Empty value
			"=value",            // Empty key
			"key=value,invalid", // Second part invalid
			"key:value",         // Wrong separator
		}

		for _, tag := range invalidTags {
			_, _, _, err := container.ParseTag(tag)
			if err == nil {
				t.Errorf("Expected error for invalid tag '%s', got none", tag)
			}
		}
	})

	t.Run("UnknownFilterType", func(t *testing.T) {
		_, _, _, err := container.ParseTag("key=unknownFilter(param)")
		if err == nil {
			t.Errorf("Expected error for unknown filter type, got none")
		}
	})

	t.Run("MalformedFilterExpression", func(t *testing.T) {
		malformedFilters := []string{
			"key=filter(unclosed", // Missing closing parenthesis
			"key=filter)",         // Missing opening parenthesis
			"key=(param)",         // Missing filter name
		}

		for _, tag := range malformedFilters {
			_, _, _, err := container.ParseTag(tag)
			if err == nil {
				t.Errorf("Expected error for malformed filter '%s', got none", tag)
			}
		}
	})

	t.Run("MalformedQuotedString", func(t *testing.T) {
		malformedQuotes := []string{
			"key='unclosed", // Missing closing quote
			"key=closing'",  // Missing opening quote
			"key=''extra'",  // Extra quotes
		}

		for _, tag := range malformedQuotes {
			_, _, _, err := container.ParseTag(tag)
			if err == nil {
				t.Errorf("Expected error for malformed quoted string '%s', got none", tag)
			}
		}
	})
}
