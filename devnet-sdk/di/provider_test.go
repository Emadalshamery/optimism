package di_test

import (
	"testing"

	"github.com/ethereum-optimism/optimism/devnet-sdk/di"
)

// Simple types for testing provider registration
type TestValue int
type TestDep struct {
	Value int
}

func TestProviderRegistration(t *testing.T) {
	t.Run("RegisterProviderWithNoParameters", func(t *testing.T) {
		container := di.NewContainer()

		// Register a simple provider with no parameters
		container.RegisterProvider(func() *TestValue {
			val := TestValue(42)
			return &val
		}, nil, "singleton")

		// Check if provider is registered
		val, err := di.Provide[*TestValue](container, nil)
		if err != nil {
			t.Fatalf("Failed to resolve registered provider: %v", err)
		}

		if *val != 42 {
			t.Errorf("Expected value 42, got %d", *val)
		}
	})

	t.Run("RegisterProviderWithParameters", func(t *testing.T) {
		container := di.NewContainer()

		// Register int provider for TestDep.Value
		container.RegisterProvider(func() int {
			return 10
		}, nil, "singleton")

		// Register a dependency
		container.RegisterProvider(func(val int) *TestDep {
			return &TestDep{Value: val}
		}, nil, "singleton")

		// Register a provider that depends on TestDep
		container.RegisterProvider(func(dep *TestDep) *TestValue {
			val := TestValue(dep.Value * 2)
			return &val
		}, nil, "singleton")

		// Check if provider resolves with parameters
		val, err := di.Provide[*TestValue](container, nil)
		if err != nil {
			t.Fatalf("Failed to resolve provider with parameters: %v", err)
		}

		// Value should be TestDep.Value * 2
		if *val != 20 {
			t.Errorf("Expected value 20, got %d", *val)
		}
	})

	t.Run("RegisterProviderWithMetadata", func(t *testing.T) {
		container := di.NewContainer()

		// Register providers with different metadata
		container.RegisterProvider(func() *TestValue {
			val := TestValue(10)
			return &val
		}, map[string]interface{}{
			"env": "development",
		}, "singleton")

		container.RegisterProvider(func() *TestValue {
			val := TestValue(20)
			return &val
		}, map[string]interface{}{
			"env": "production",
		}, "singleton")

		// Create filter for production environment
		_, _, filters, _ := container.ParseTag("env=nameEquals(production)")

		// Resolve with filter
		val, err := di.Provide[*TestValue](container, filters)
		if err != nil {
			t.Fatalf("Failed to resolve provider with filter: %v", err)
		}

		// Should get the production value
		if *val != 20 {
			t.Errorf("Expected filtered value 20, got %d", *val)
		}
	})

	t.Run("RegisterMultipleProvidersForSameType", func(t *testing.T) {
		container := di.NewContainer()

		// Register multiple providers for the same type
		container.RegisterProvider(func() *TestValue {
			val := TestValue(10)
			return &val
		}, map[string]interface{}{
			"id": "first",
		}, "singleton")

		container.RegisterProvider(func() *TestValue {
			val := TestValue(20)
			return &val
		}, map[string]interface{}{
			"id": "second",
		}, "singleton")

		container.RegisterProvider(func() *TestValue {
			val := TestValue(30)
			return &val
		}, map[string]interface{}{
			"id": "third",
		}, "singleton")

		// Without filters, should get the first provider
		val, err := di.Provide[*TestValue](container, nil)
		if err != nil {
			t.Fatalf("Failed to resolve provider: %v", err)
		}

		if *val != 10 {
			t.Errorf("Expected first provider value 10, got %d", *val)
		}
	})
}

func TestProviderErrorHandling(t *testing.T) {
	t.Run("NonFunctionProvider", func(t *testing.T) {
		container := di.NewContainer()

		// Try to register a non-function provider
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("Expected panic for non-function provider, got none")
			}
		}()

		container.RegisterProvider("not a function", nil, "singleton")
	})

	t.Run("ProviderWithNoReturn", func(t *testing.T) {
		container := di.NewContainer()

		// Try to register a function with no return values
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("Expected panic for provider with no return values, got none")
			}
		}()

		container.RegisterProvider(func() {}, nil, "singleton")
	})

	t.Run("ProviderWithMultipleReturns", func(t *testing.T) {
		container := di.NewContainer()

		// Try to register a function with multiple return values
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("Expected panic for provider with multiple return values, got none")
			}
		}()

		container.RegisterProvider(func() (*TestValue, error) {
			return nil, nil
		}, nil, "singleton")
	})
}

func TestProviderScopes(t *testing.T) {
	t.Run("SingletonScope", func(t *testing.T) {
		container := di.NewContainer()

		// Register a provider with a singleton scope
		var counter int
		container.RegisterProvider(func() *TestValue {
			counter++
			val := TestValue(counter)
			return &val
		}, nil, "singleton")

		// Resolve multiple times
		val1, _ := di.Provide[*TestValue](container, nil)
		val2, _ := di.Provide[*TestValue](container, nil)
		val3, _ := di.Provide[*TestValue](container, nil)

		// All values should be the same (counter value 1)
		if *val1 != 1 || *val2 != 1 || *val3 != 1 {
			t.Errorf("Expected all values to be 1 for singleton provider, got %d, %d, %d",
				*val1, *val2, *val3)
		}

		// Counter should be 1 (provider function called once)
		if counter != 1 {
			t.Errorf("Expected provider function to be called once, was called %d times", counter)
		}
	})

	t.Run("PrototypeScope", func(t *testing.T) {
		container := di.NewContainer()

		// Register a provider with a prototype scope
		var counter int
		container.RegisterProvider(func() *TestValue {
			counter++
			val := TestValue(counter)
			return &val
		}, nil, "prototype")

		// Resolve multiple times
		val1, _ := di.Provide[*TestValue](container, nil)
		val2, _ := di.Provide[*TestValue](container, nil)
		val3, _ := di.Provide[*TestValue](container, nil)

		// Values should be different (incremental counter values)
		if *val1 != 1 || *val2 != 2 || *val3 != 3 {
			t.Errorf("Expected different values 1, 2, 3 for prototype provider, got %d, %d, %d",
				*val1, *val2, *val3)
		}

		// Counter should be 3 (provider function called for each resolution)
		if counter != 3 {
			t.Errorf("Expected provider function to be called 3 times, was called %d times", counter)
		}
	})

	t.Run("MixedScopeProviders", func(t *testing.T) {
		container := di.NewContainer()

		// Register int provider for TestDep.Value
		container.RegisterProvider(func() int {
			return 42
		}, nil, "singleton")

		// Register a singleton dependency
		container.RegisterProvider(func(val int) *TestDep {
			return &TestDep{Value: val}
		}, nil, "singleton")

		// Register a prototype provider that depends on the singleton
		var counter int
		container.RegisterProvider(func(dep *TestDep) *TestValue {
			counter++
			val := TestValue(dep.Value * counter)
			return &val
		}, nil, "prototype")

		// Resolve multiple times
		val1, _ := di.Provide[*TestValue](container, nil)
		val2, _ := di.Provide[*TestValue](container, nil)

		// First call: 42 * 1 = 42
		// Second call: 42 * 2 = 84
		if *val1 != 42 || *val2 != 84 {
			t.Errorf("Expected values 42, 84 for mixed scope providers, got %d, %d",
				*val1, *val2)
		}
	})
}
