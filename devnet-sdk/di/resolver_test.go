package di_test

import (
	"testing"

	"github.com/ethereum-optimism/optimism/devnet-sdk/di"
)

// Types for resolver tests
type (
	ResolverInt    int
	ResolverString string
	ResolverBool   bool

	SimpleResolverDep struct {
		Value int
	}

	ComplexResolverDep struct {
		DepA *SimpleResolverDep
		DepB *ResolverString
	}

	CircularDep1 struct {
		Dep *CircularDep2
	}

	CircularDep2 struct {
		Dep *CircularDep1
	}
)

func TestBasicResolution(t *testing.T) {
	t.Run("ResolveSimpleType", func(t *testing.T) {
		container := di.NewContainer()

		// Register a simple provider
		container.RegisterProvider(func() *ResolverInt {
			val := ResolverInt(42)
			return &val
		}, nil, "singleton")

		// Resolve it
		val, err := di.Provide[*ResolverInt](container, nil)
		if err != nil {
			t.Fatalf("Failed to resolve: %v", err)
		}

		if *val != 42 {
			t.Errorf("Expected value 42, got %d", *val)
		}
	})

	t.Run("TypeSafety", func(t *testing.T) {
		container := di.NewContainer()

		// Register a provider that returns *ResolverInt
		container.RegisterProvider(func() *ResolverInt {
			val := ResolverInt(42)
			return &val
		}, nil, "singleton")

		// Try to resolve it as *ResolverString (wrong type)
		_, err := di.Provide[*ResolverString](container, nil)
		if err == nil {
			t.Errorf("Expected error when resolving wrong type, got none")
		}
	})

	t.Run("ResolveWithFilters", func(t *testing.T) {
		container := di.NewContainer()

		// Register multiple providers with different metadata
		container.RegisterProvider(func() *ResolverInt {
			val := ResolverInt(1)
			return &val
		}, map[string]interface{}{
			"env": "dev",
		}, "singleton")

		container.RegisterProvider(func() *ResolverInt {
			val := ResolverInt(2)
			return &val
		}, map[string]interface{}{
			"env": "prod",
		}, "singleton")

		// Create a filter for prod environment
		_, _, filters, _ := container.ParseTag("env=nameEquals(prod)")

		// Resolve with filter
		val, err := di.Provide[*ResolverInt](container, filters)
		if err != nil {
			t.Fatalf("Failed to resolve with filter: %v", err)
		}

		if *val != 2 {
			t.Errorf("Expected filtered value 2, got %d", *val)
		}
	})
}

func TestNestedDependencies(t *testing.T) {
	t.Run("ResolveDependencyTree", func(t *testing.T) {
		container := di.NewContainer()

		// Register int provider for SimpleResolverDep.Value
		container.RegisterProvider(func() int {
			return 42
		}, nil, "singleton")

		// Register providers for a dependency tree
		container.RegisterProvider(func(val int) *SimpleResolverDep {
			return &SimpleResolverDep{Value: val}
		}, nil, "singleton")

		container.RegisterProvider(func() *ResolverString {
			val := ResolverString("hello")
			return &val
		}, nil, "singleton")

		container.RegisterProvider(func(a *SimpleResolverDep, b *ResolverString) *ComplexResolverDep {
			return &ComplexResolverDep{
				DepA: a,
				DepB: b,
			}
		}, nil, "singleton")

		// Resolve the complex dependency
		complex, err := di.Provide[*ComplexResolverDep](container, nil)
		if err != nil {
			t.Fatalf("Failed to resolve dependency tree: %v", err)
		}

		// Verify the dependency tree is correctly resolved
		if complex.DepA.Value != 42 {
			t.Errorf("Expected DepA.Value to be 42, got %d", complex.DepA.Value)
		}

		if *complex.DepB != "hello" {
			t.Errorf("Expected DepB to be 'hello', got '%s'", *complex.DepB)
		}
	})
}

func TestCyclicDependencies(t *testing.T) {
	t.Run("DirectCycle", func(t *testing.T) {
		container := di.NewContainer()

		// Register a provider that depends on itself
		container.RegisterProvider(func(val *ResolverInt) *ResolverInt {
			return val
		}, nil, "singleton")

		// Try to resolve it (should fail with cycle detection)
		_, err := di.Provide[*ResolverInt](container, nil)
		if err == nil {
			t.Errorf("Expected error for direct cyclic dependency, got none")
		}
	})

	t.Run("IndirectCycle", func(t *testing.T) {
		container := di.NewContainer()

		// Create a cycle: CircularDep1 -> CircularDep2 -> CircularDep1
		container.RegisterProvider(func(dep *CircularDep2) *CircularDep1 {
			return &CircularDep1{Dep: dep}
		}, nil, "singleton")

		container.RegisterProvider(func(dep *CircularDep1) *CircularDep2 {
			return &CircularDep2{Dep: dep}
		}, nil, "singleton")

		// Try to resolve it (should fail with cycle detection)
		_, err := di.Provide[*CircularDep1](container, nil)
		if err == nil {
			t.Errorf("Expected error for indirect cyclic dependency, got none")
		}
	})
}

func TestSingletonCaching(t *testing.T) {
	t.Run("SingletonInstance", func(t *testing.T) {
		container := di.NewContainer()

		// Counter for tracking invocations
		invocationCount := 0

		// Register a singleton provider that increments the counter
		container.RegisterProvider(func() *ResolverInt {
			invocationCount++
			val := ResolverInt(invocationCount)
			return &val
		}, nil, "singleton")

		// Request the same singleton multiple times
		val1, _ := di.Provide[*ResolverInt](container, nil)
		val2, _ := di.Provide[*ResolverInt](container, nil)
		val3, _ := di.Provide[*ResolverInt](container, nil)

		// Provider should only be invoked once
		if invocationCount != 1 {
			t.Errorf("Expected provider to be invoked once, got %d invocations", invocationCount)
		}

		// All instances should be the same
		if *val1 != 1 || *val2 != 1 || *val3 != 1 {
			t.Errorf("Expected all singleton instances to be the same value (1), got %d, %d, %d",
				*val1, *val2, *val3)
		}
	})

	t.Run("PrototypeInstance", func(t *testing.T) {
		container := di.NewContainer()

		// Counter for tracking invocations
		invocationCount := 0

		// Register a prototype provider that increments the counter
		container.RegisterProvider(func() *ResolverInt {
			invocationCount++
			val := ResolverInt(invocationCount)
			return &val
		}, nil, "prototype")

		// Request the prototype multiple times
		val1, _ := di.Provide[*ResolverInt](container, nil)
		val2, _ := di.Provide[*ResolverInt](container, nil)
		val3, _ := di.Provide[*ResolverInt](container, nil)

		// Provider should be invoked for each request
		if invocationCount != 3 {
			t.Errorf("Expected provider to be invoked 3 times, got %d invocations", invocationCount)
		}

		// Each instance should have a different value
		if *val1 != 1 || *val2 != 2 || *val3 != 3 {
			t.Errorf("Expected increasing values (1,2,3), got %d, %d, %d",
				*val1, *val2, *val3)
		}
	})
}

func TestErrorHandling(t *testing.T) {
	t.Run("NonExistentType", func(t *testing.T) {
		container := di.NewContainer()

		// Try to resolve a type with no provider
		_, err := di.Provide[*ResolverBool](container, nil)
		if err == nil {
			t.Errorf("Expected error for non-existent type, got none")
		}
	})

	t.Run("MissingDependency", func(t *testing.T) {
		container := di.NewContainer()

		// Register a provider that depends on an unregistered type
		container.RegisterProvider(func(dep *SimpleResolverDep) *ResolverInt {
			val := ResolverInt(dep.Value)
			return &val
		}, nil, "singleton")

		// Try to resolve it
		_, err := di.Provide[*ResolverInt](container, nil)
		if err == nil {
			t.Errorf("Expected error for missing dependency, got none")
		}
	})

	t.Run("FilterNoMatch", func(t *testing.T) {
		container := di.NewContainer()

		// Register a provider
		container.RegisterProvider(func() *ResolverInt {
			val := ResolverInt(1)
			return &val
		}, map[string]interface{}{
			"env": "dev",
		}, "singleton")

		// Create a filter that won't match any provider
		_, _, filters, _ := container.ParseTag("env=nameEquals(prod)")

		// Try to resolve with the filter
		_, err := di.Provide[*ResolverInt](container, filters)
		if err == nil {
			t.Errorf("Expected error when no provider matches filter, got none")
		}
	})
}
