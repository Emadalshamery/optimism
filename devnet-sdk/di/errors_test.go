package di

import (
	"reflect"
	"strings"
	"testing"
)

func TestDependencyResolutionErrorNoProvider(t *testing.T) {
	// Create a container with no providers
	container := NewContainer()

	// Interface to test with
	type TestInterface interface {
		Test()
	}

	// Try to resolve an interface that has no providers
	_, err := Provide[TestInterface](container, nil)

	// Error should not be nil
	if err == nil {
		t.Fatal("Expected error when no provider is registered")
	}

	// Error should be of type DependencyResolutionError
	resErr, ok := err.(*DependencyResolutionError)
	if !ok {
		t.Fatalf("Expected DependencyResolutionError, got %T", err)
	}

	// Check error details
	if resErr.DependencyType != reflect.TypeOf((*TestInterface)(nil)).Elem() {
		t.Errorf("Expected dependency type %v, got %v", reflect.TypeOf((*TestInterface)(nil)).Elem(), resErr.DependencyType)
	}

	if len(resErr.AvailableProviders) != 0 {
		t.Errorf("Expected no available providers, got %d", len(resErr.AvailableProviders))
	}

	// Check error message format
	errMsg := resErr.Error()
	if !strings.Contains(errMsg, "Failed to resolve dependency") {
		t.Errorf("Error message should contain 'Failed to resolve dependency', got: %s", errMsg)
	}

	if !strings.Contains(errMsg, "No providers registered for this type") {
		t.Errorf("Error message should contain 'No providers registered for this type', got: %s", errMsg)
	}
}

func TestDependencyResolutionErrorFiltered(t *testing.T) {
	// Create a container with a provider with specific metadata
	container := NewContainer()

	// Define test types
	type TestService struct{}

	// Register a provider
	container.RegisterProvider(
		func() *TestService { return &TestService{} },
		map[string]interface{}{"version": "1.0", "name": "service1"},
		"singleton",
	)

	// Create name filter for a different name
	nameFilter := NewNameEqualsFilter("name", "service2") // This won't match

	// Try to resolve with filters that won't match
	_, err := Provide[*TestService](container, []Filter{nameFilter})

	// Should get an error
	if err == nil {
		t.Fatal("Expected error when filtering out all providers")
	}

	// Error should be of type DependencyResolutionError
	resErr, ok := err.(*DependencyResolutionError)
	if !ok {
		t.Fatalf("Expected DependencyResolutionError, got %T", err)
	}

	// Check error details
	if resErr.DependencyType != reflect.TypeOf((*TestService)(nil)) {
		t.Errorf("Expected dependency type %v, got %v", reflect.TypeOf((*TestService)(nil)), resErr.DependencyType)
	}

	if len(resErr.AppliedFilters) != 1 {
		t.Errorf("Expected 1 applied filter, got %d", len(resErr.AppliedFilters))
	}

	if len(resErr.AvailableProviders) == 0 {
		t.Error("Expected available providers to be listed")
	}

	if len(resErr.DisqualifiedProviders) == 0 {
		t.Error("Expected disqualified providers to be listed")
	}

	// Check error message for filter details
	errMsg := resErr.Error()
	if !strings.Contains(errMsg, "Type-compatible providers that were disqualified by filters") {
		t.Errorf("Error message should mention disqualified providers, got: %s", errMsg)
	}

	if !strings.Contains(errMsg, "NameEqualsFilter") {
		t.Errorf("Error message should mention the disqualifying filter type, got: %s", errMsg)
	}

	if !strings.Contains(errMsg, "key=name, expected=service2") {
		t.Errorf("Error message should contain filter details, got: %s", errMsg)
	}
}

func TestDependencyResolutionErrorNested(t *testing.T) {
	// Create a container with a provider that depends on an unregistered type
	container := NewContainer()

	// Define test types
	type Dependency interface {
		DoSomething()
	}
	type Service struct {
		Dep Dependency
	}

	// Register a provider for Service that depends on Dependency
	container.RegisterProvider(
		func(dep Dependency) *Service { return &Service{Dep: dep} },
		nil,
		"singleton",
	)

	// Try to resolve Service (should fail because Dependency is not registered)
	_, err := Provide[*Service](container, nil)

	// Should get an error
	if err == nil {
		t.Fatal("Expected error when dependency is missing")
	}

	// Error should mention both the requested type and the missing dependency
	errMsg := err.Error()
	if !strings.Contains(errMsg, "Service") {
		t.Errorf("Error message should mention the Service type, got: %s", errMsg)
	}

	if !strings.Contains(errMsg, "Dependency") {
		t.Errorf("Error message should mention the missing Dependency type, got: %s", errMsg)
	}

	if !strings.Contains(errMsg, "Caused by") {
		t.Errorf("Error message should show the cause chain, got: %s", errMsg)
	}
}

func TestStructAutoResolutionError(t *testing.T) {
	// Create a container with no providers
	container := NewContainer()

	// Define test types with a dependency
	type Dependency interface {
		DoSomething()
	}
	type ServiceWithField struct {
		Dep Dependency `di:""`
	}

	// Try to resolve struct (should fail because Dependency field is missing)
	_, err := Provide[ServiceWithField](container, nil)

	// Should get an error
	if err == nil {
		t.Fatal("Expected error when field dependency is missing")
	}

	// Error message should be detailed and mention the field
	errMsg := err.Error()
	if !strings.Contains(errMsg, "ServiceWithField") {
		t.Errorf("Error message should mention the struct type, got: %s", errMsg)
	}

	if !strings.Contains(errMsg, "Dependency") {
		t.Errorf("Error message should mention the field type, got: %s", errMsg)
	}
}
