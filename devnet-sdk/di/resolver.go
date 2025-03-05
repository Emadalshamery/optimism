package di

import (
	"errors"
	"fmt"
	"reflect"
)

// Provide resolves a dependency of type T with optional filters.
func Provide[T any](c *Container, filters []Filter) (T, error) {
	var zero T
	t := reflect.TypeOf((*T)(nil)).Elem()

	c.debug.LogResolutionAttempt(t, filters)

	val, err := c.provide(t, filters)
	if err != nil {
		return zero, err
	}

	// Handle the case where val is not of type T
	if reflect.TypeOf(val) != t {
		// If val is a struct and T is a pointer to that struct
		if t.Kind() == reflect.Ptr && reflect.TypeOf(val).Kind() == reflect.Struct {
			// Create a new pointer to val
			ptr := reflect.New(reflect.TypeOf(val))
			ptr.Elem().Set(reflect.ValueOf(val))
			return ptr.Interface().(T), nil
		}
		// If val is a pointer and T is the struct it points to
		if reflect.TypeOf(val).Kind() == reflect.Ptr && t.Kind() == reflect.Struct {
			return reflect.ValueOf(val).Elem().Interface().(T), nil
		}
	}

	return val.(T), nil
}

// provide resolves a dependency of type t with filters.
func (c *Container) provide(t reflect.Type, filters []Filter) (interface{}, error) {
	fmt.Printf("Providing type: %v\n", t)

	// Check for circular dependencies
	if resolving, ok := c.resolving[t]; ok && resolving {
		fmt.Printf("Circular dependency detected for type %v\n", t)
		return nil, fmt.Errorf("cycle detected for type %v", t)
	}

	// Check singleton cache
	if instance, ok := c.singletons[t]; ok {
		fmt.Printf("Found in singleton cache: %v = %v\n", t, instance)
		c.debug.LogCacheAccess(t, true, instance)
		return instance, nil
	}
	fmt.Printf("Not found in singleton cache: %v\n", t)
	c.debug.LogCacheAccess(t, false, nil)

	// Mark as resolving
	c.resolving[t] = true
	defer func() { c.resolving[t] = false }()

	// Get providers for this type
	providers := c.GetProviders(t)
	if len(providers) == 0 {
		fmt.Printf("No provider registered for type %v\n", t)

		// If it's a struct, try to auto-resolve it
		if t.Kind() == reflect.Struct {
			fmt.Printf("Resolving struct type: %v\n", t)
			return c.autoResolveStruct(t, filters)
		}

		// If it's a pointer to a struct, try to auto-resolve it too
		if t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.Struct {
			// Create instance and resolve fields
			instance := reflect.New(t.Elem()).Interface()
			c.singletons[t] = instance // Temporarily store to prevent circular dependencies

			// Auto-resolve the struct fields
			resolved, err := c.autoResolveStruct(t.Elem(), filters)
			if err != nil {
				delete(c.singletons, t) // Clean up on error
				return nil, err
			}

			// Create a new pointer to the resolved struct
			result := reflect.New(t.Elem()).Interface()
			reflect.ValueOf(result).Elem().Set(reflect.ValueOf(resolved))

			// Store in singleton cache if needed
			c.singletons[t] = result
			c.debug.LogCacheStore(t, result)

			return result, nil
		}

		// Return enhanced error message when no provider is found
		return nil, NewDependencyNotFoundError(t, filters)
	}
	fmt.Printf("Found %d providers for type %v\n", len(providers), t)

	// Filter providers
	filteredOut := make([]*Provider, 0)
	var selectedProvider *Provider

	for _, p := range providers {
		// Skip providers that don't match filters
		if len(filters) > 0 && !c.providerMatchesFilters(*p, filters) {
			filteredOut = append(filteredOut, p)
			continue
		}

		selectedProvider = p
		break
	}

	if selectedProvider == nil {
		// Enhanced error message for filtered-out providers
		if len(filters) > 0 {
			// For each filtered provider, determine which filters disqualified it
			disqualifiedProviders := make(map[*Provider][]Filter)

			for _, p := range filteredOut {
				disqualifyingFilters := make([]Filter, 0)

				for _, f := range filters {
					matched := false
					for key, value := range p.Metadata {
						if f.AppliesTo(key) && f.Matches(value) {
							matched = true
							break
						}
					}
					if !matched {
						disqualifyingFilters = append(disqualifyingFilters, f)
					}
				}

				disqualifiedProviders[p] = disqualifyingFilters
			}

			return nil, NewDependencyFilteredError(t, filters, providers, disqualifiedProviders)
		}

		return nil, NewDependencyNotFoundError(t, filters)
	}

	fmt.Printf("Selected provider for type %v\n", t)
	c.debug.LogProviderSelection(t, selectedProvider, filteredOut)

	// Invoke the provider
	fmt.Printf("Invoking provider for type %v\n", t)
	result, err := c.invokeProvider(*selectedProvider)
	if err != nil {
		return nil, err
	}

	fmt.Printf("Provider for type %v returned: %v\n", t, result)

	// Cache result if singleton
	if selectedProvider.scope == "singleton" {
		c.singletons[t] = result
		fmt.Printf("Caching singleton result for type %v: %v\n", t, result)
		c.debug.LogCacheStore(t, result)
	}

	return result, nil
}

// invokeProvider invokes a provider function with resolved parameters.
func (c *Container) invokeProvider(p Provider) (interface{}, error) {
	// Resolve parameters for the provider
	fnValue := reflect.ValueOf(p.fn)
	params := make([]reflect.Value, fnValue.Type().NumIn())

	// For tracking debug info
	paramObjects := make([]interface{}, fnValue.Type().NumIn())

	fmt.Printf("Invoking provider for type %v with %d parameters\n", p.provides, fnValue.Type().NumIn())

	for i := 0; i < fnValue.Type().NumIn(); i++ {
		paramType := fnValue.Type().In(i)
		fmt.Printf("  Resolving parameter %d of type %v\n", i, paramType)
		param, err := c.provide(paramType, nil) // No filters for now
		if err != nil {
			// Wrap the error with additional context
			var resErr *DependencyResolutionError
			if errors.As(err, &resErr) {
				// Update the cause for nested dependency resolution errors
				return nil, &DependencyResolutionError{
					DependencyType: p.provides,
					Cause: fmt.Errorf("failed to resolve parameter %d of type %v for provider of %v: %w",
						i, paramType, p.provides, err),
				}
			}

			return nil, fmt.Errorf("failed to resolve parameter %d of type %v: %w", i, paramType, err)
		}
		fmt.Printf("  Resolved parameter %d: %+v\n", i, param)
		params[i] = reflect.ValueOf(param)
		paramObjects[i] = param
	}

	// Invoke the provider function
	fmt.Printf("Calling provider function with %d parameters\n", len(params))
	results := fnValue.Call(params)
	if len(results) != 1 {
		return nil, fmt.Errorf("provider function must return exactly one value")
	}

	result := results[0].Interface()
	fmt.Printf("Provider returned: %+v\n", result)
	c.debug.LogProviderInvocation(&p, paramObjects, result, nil)

	return result, nil
}
