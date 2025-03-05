package di_test

import (
	"testing"

	"github.com/ethereum-optimism/optimism/devnet-sdk/di"
)

// Types for container tests
type (
	ContainerInt    int
	ContainerString string
	ContainerBool   bool
)

func TestContainerInit(t *testing.T) {
	t.Run("ContainerInitialization", func(t *testing.T) {
		container := di.NewContainer()

		if container == nil {
			t.Fatalf("Expected non-nil container")
		}

		// Verify initial state is empty
		_, err := di.Provide[*ContainerInt](container, nil)
		if err == nil {
			t.Errorf("Expected error when resolving from empty container")
		}
	})
}

func TestContainerRegistration(t *testing.T) {
	t.Run("RegisterSimpleProvider", func(t *testing.T) {
		container := di.NewContainer()

		// Register a provider
		container.RegisterProvider(func() *ContainerInt {
			val := ContainerInt(42)
			return &val
		}, nil, "singleton")

		// Verify it can be resolved
		val, err := di.Provide[*ContainerInt](container, nil)
		if err != nil {
			t.Fatalf("Failed to resolve provider: %v", err)
		}

		if *val != 42 {
			t.Errorf("Expected value 42, got %d", *val)
		}
	})

	t.Run("RegisterOverwrite", func(t *testing.T) {
		container := di.NewContainer()

		// Register initial provider
		container.RegisterProvider(func() *ContainerInt {
			val := ContainerInt(1)
			return &val
		}, nil, "singleton")

		// Register replacement provider
		container.RegisterProvider(func() *ContainerInt {
			val := ContainerInt(2)
			return &val
		}, nil, "singleton")

		// By default, we should get the first provider
		val, err := di.Provide[*ContainerInt](container, nil)
		if err != nil {
			t.Fatalf("Failed to resolve provider: %v", err)
		}

		if *val != 1 {
			t.Errorf("Expected value 1 (first provider), got %d", *val)
		}
	})

	t.Run("RegisterWithMetadata", func(t *testing.T) {
		container := di.NewContainer()

		metadata := map[string]interface{}{
			"env":    "prod",
			"region": "us-west",
		}

		// Register provider with metadata
		container.RegisterProvider(func() *ContainerString {
			val := ContainerString("hello")
			return &val
		}, metadata, "singleton")

		// The metadata shouldn't affect normal resolution
		val, err := di.Provide[*ContainerString](container, nil)
		if err != nil {
			t.Fatalf("Failed to resolve provider: %v", err)
		}

		if *val != "hello" {
			t.Errorf("Expected value 'hello', got '%s'", *val)
		}
	})

	t.Run("InvalidScope", func(t *testing.T) {
		// Test registering with an invalid scope
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected panic when registering provider with invalid scope")
			}
		}()

		container := di.NewContainer()

		// This should panic due to invalid scope
		container.RegisterProvider(func() *ContainerInt {
			val := ContainerInt(42)
			return &val
		}, nil, "invalid_scope")
	})
}

func TestContainerTagParsing(t *testing.T) {
	t.Run("BasicTag", func(t *testing.T) {
		container := di.NewContainer()

		literals, _, filters, err := container.ParseTag("optional=true,env=nameEquals(prod)")

		if err != nil {
			t.Fatalf("Failed to parse tag: %v", err)
		}

		// Check for optional literal
		if optional, ok := literals["optional"]; !ok || optional != "true" {
			t.Errorf("Expected 'optional' literal with value 'true'")
		}

		if len(filters) != 1 {
			t.Fatalf("Expected 1 filter, got %d", len(filters))
		}

		// Check that the filter is for env=nameEquals(prod)
		filter := filters[0]
		if !filter.AppliesTo("env") {
			t.Errorf("Filter should apply to 'env'")
		}
	})

	t.Run("TypeTag", func(t *testing.T) {
		container := di.NewContainer()

		literals, _, _, err := container.ParseTag("type='json'")

		if err != nil {
			t.Fatalf("Failed to parse tag: %v", err)
		}

		// Check for type literal
		if typeVal, ok := literals["type"]; !ok || typeVal != "json" {
			t.Errorf("Expected 'type' literal with value 'json'")
		}
	})

	t.Run("MultipleFilters", func(t *testing.T) {
		container := di.NewContainer()

		_, _, filters, err := container.ParseTag("env=nameEquals(prod),version=versionGreaterThan(1.0.0)")

		if err != nil {
			t.Fatalf("Failed to parse tag: %v", err)
		}

		if len(filters) != 2 {
			t.Fatalf("Expected 2 filters, got %d", len(filters))
		}

		// Check that the filters apply to the correct keys
		foundEnv := false
		foundVersion := false
		for _, filter := range filters {
			if filter.AppliesTo("env") {
				foundEnv = true
			}
			if filter.AppliesTo("version") {
				foundVersion = true
			}
		}

		if !foundEnv {
			t.Errorf("Expected filter for 'env'")
		}
		if !foundVersion {
			t.Errorf("Expected filter for 'version'")
		}
	})

	t.Run("InvalidTag", func(t *testing.T) {
		container := di.NewContainer()

		_, _, _, err := container.ParseTag("invalid:tag")

		if err == nil {
			t.Errorf("Expected error for invalid tag, got none")
		}
	})
}

// Note: Tests for internal functionality that would require access to the Container's
// provider type information have been removed as those methods are not exported.
