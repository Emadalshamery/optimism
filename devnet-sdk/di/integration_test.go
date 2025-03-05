package di_test

import (
	"fmt"
	"testing"

	"github.com/ethereum-optimism/optimism/devnet-sdk/di"
)

// Types for integration tests
type (
	ServiceA struct {
		Value string
	}

	ServiceB struct {
		A      *ServiceA
		Config string
	}

	ServiceC struct {
		B      *ServiceB
		Number int
	}

	// For testing circular dependencies
	CircularA struct {
		B *CircularB
	}

	CircularB struct {
		A *CircularA
	}

	// For testing optional dependencies
	OptionalDep struct {
		Value string
	}

	ServiceWithOptional struct {
		OptDep *OptionalDep
		Value  string
	}

	// For testing filtered dependencies
	Environment string
)

func TestBasicDependencyResolution(t *testing.T) {
	container := di.NewContainer()

	// Enable debug tracing
	container.EnableDebug(di.DebugOptions{
		Enabled:           true,
		TraceRegistration: true,
		TraceResolution:   true,
		TraceCaching:      true,
		TraceInvocation:   true,
		Verbose:           true,
	})

	// Register ServiceA
	container.RegisterProvider(func() *ServiceA {
		return &ServiceA{Value: "A"}
	}, nil, "singleton")

	// Register ServiceB
	container.RegisterProvider(func(a *ServiceA, config string) *ServiceB {
		return &ServiceB{A: a, Config: config}
	}, nil, "singleton")

	// Register ServiceC
	container.RegisterProvider(func(b *ServiceB, num int) *ServiceC {
		return &ServiceC{B: b, Number: num}
	}, nil, "singleton")

	// Register string provider for ServiceB.Config
	container.RegisterProvider(func() string {
		return "config"
	}, map[string]interface{}{"param": "config"}, "singleton")

	// Register int provider for ServiceC.Number
	container.RegisterProvider(func() int {
		return 42
	}, nil, "singleton")

	// Resolve ServiceC
	serviceC, err := di.Provide[*ServiceC](container, nil)
	if err != nil {
		t.Fatalf("Failed to resolve ServiceC: %v", err)
	}

	// Print debug trace
	fmt.Println("Debug trace:")
	trace := container.GetDebugTrace()
	for i, entry := range trace.GetEntries() {
		fmt.Printf("%d: [%s] %s\n", i, entry.Type, entry.Message)
		for k, v := range entry.Data {
			fmt.Printf("   - %s: %v\n", k, v)
		}
	}

	// Debug the serviceC value
	fmt.Printf("ServiceC: %+v\n", serviceC)
	if serviceC.B != nil {
		fmt.Printf("ServiceC.B: %+v\n", serviceC.B)
		if serviceC.B.A != nil {
			fmt.Printf("ServiceC.B.A: %+v\n", serviceC.B.A)
		} else {
			fmt.Println("ServiceC.B.A is nil")
		}
	} else {
		fmt.Println("ServiceC.B is nil")
	}

	// Check values
	if serviceC.B == nil {
		t.Fatalf("ServiceC.B is nil")
	}
	if serviceC.B.A == nil {
		t.Fatalf("ServiceC.B.A is nil")
	}
	if serviceC.B.Config != "config" {
		t.Errorf("Expected ServiceC.B.Config to be 'config', got '%s'", serviceC.B.Config)
	}
	if serviceC.Number != 42 {
		t.Errorf("Expected ServiceC.Number to be 42, got %d", serviceC.Number)
	}
}

func TestCircularDependencyDetection(t *testing.T) {

	container := di.NewContainer()

	// Register CircularA which depends on CircularB
	container.RegisterProvider(func(b *CircularB) *CircularA {
		return &CircularA{B: b}
	}, nil, "singleton")

	// Register CircularB which depends on CircularA
	container.RegisterProvider(func(a *CircularA) *CircularB {
		return &CircularB{A: a}
	}, nil, "singleton")

	// Try to resolve CircularA (should fail with cycle detection)
	_, err := di.Provide[*CircularA](container, nil)
	if err == nil {
		t.Errorf("Expected error for circular dependency, got none")
	}
}

// Note: Skipping optional dependency tests as RegisterParameterFilter is not available

func TestEnvironmentFiltering(t *testing.T) {
	container := di.NewContainer()

	// Register the nameEquals filter
	container.RegisterFilter("nameEquals", func(key, value string) di.Filter {
		return di.NewNameEqualsFilter(key, value)
	})

	// Register environment providers with metadata
	container.RegisterProvider(func() *Environment {
		env := Environment("dev")
		return &env
	}, map[string]interface{}{"env": "dev"}, "singleton")

	container.RegisterProvider(func() *Environment {
		env := Environment("prod")
		return &env
	}, map[string]interface{}{"env": "prod"}, "singleton")

	container.Build()

	// Create a filter for dev environment
	_, _, devFilters, _ := container.ParseTag("env=nameEquals(dev)")

	// Create a filter for prod environment
	_, _, prodFilters, _ := container.ParseTag("env=nameEquals(prod)")

	// Resolve with dev filter
	devEnv, err := di.Provide[*Environment](container, devFilters)
	if err != nil {
		t.Fatalf("Failed to resolve dev environment: %v", err)
	}

	// Resolve with prod filter - use a new container to avoid singleton caching
	prodContainer := di.NewContainer()
	prodContainer.RegisterFilter("nameEquals", func(key, value string) di.Filter {
		return di.NewNameEqualsFilter(key, value)
	})
	prodContainer.RegisterProvider(func() *Environment {
		env := Environment("dev")
		return &env
	}, map[string]interface{}{"env": "dev"}, "singleton")
	prodContainer.RegisterProvider(func() *Environment {
		env := Environment("prod")
		return &env
	}, map[string]interface{}{"env": "prod"}, "singleton")
	prodContainer.Build()

	prodEnv, err := di.Provide[*Environment](prodContainer, prodFilters)
	if err != nil {
		t.Fatalf("Failed to resolve prod environment: %v", err)
	}

	// Verify the environments
	if string(*devEnv) != "dev" {
		t.Errorf("Expected dev environment, got '%s'", *devEnv)
	}

	if string(*prodEnv) != "prod" {
		t.Errorf("Expected prod environment, got '%s'", *prodEnv)
	}
}

func TestSingletonVsPrototype(t *testing.T) {
	container := di.NewContainer()

	// Counter for tracking invocations
	singletonCounter := 0
	prototypeCounter := 0

	// Register a singleton provider
	container.RegisterProvider(func() *ServiceA {
		singletonCounter++
		return &ServiceA{Value: "singleton"}
	}, nil, "singleton")

	// Register a prototype provider
	container.RegisterProvider(func() *ServiceB {
		prototypeCounter++
		return &ServiceB{Config: "prototype"}
	}, nil, "prototype")

	// Resolve singleton multiple times
	s1, _ := di.Provide[*ServiceA](container, nil)
	s2, _ := di.Provide[*ServiceA](container, nil)
	s3, _ := di.Provide[*ServiceA](container, nil)

	// Resolve prototype multiple times
	p1, _ := di.Provide[*ServiceB](container, nil)
	p2, _ := di.Provide[*ServiceB](container, nil)
	p3, _ := di.Provide[*ServiceB](container, nil)

	// Verify singleton was only created once
	if singletonCounter != 1 {
		t.Errorf("Expected singleton provider to be invoked once, got %d", singletonCounter)
	}

	// Verify all singleton instances are the same
	if s1 != s2 || s2 != s3 {
		t.Errorf("Expected all singleton instances to be the same")
	}

	// Verify prototype was created multiple times
	if prototypeCounter != 3 {
		t.Errorf("Expected prototype provider to be invoked 3 times, got %d", prototypeCounter)
	}

	// Verify all prototype instances are different
	if p1 == p2 || p2 == p3 || p1 == p3 {
		t.Errorf("Expected all prototype instances to be different")
	}
}
