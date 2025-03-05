package di

import (
	"fmt"
	"reflect"
)

// selectProviders selects providers for all fields that satisfy the constraints.
func selectProviders(c *Container, fields []FieldConstraint) (map[string]*Provider, error) {
	assignment := make(map[string]map[string]string)
	selected := make(map[string]*Provider)

	for _, field := range fields {
		c.debug.LogResolutionAttempt(field.Field.Type, field.Filters)
	}

	err := tryAssign(c, fields, assignment, selected, 0)
	if err != nil {
		// If we couldn't find providers for all fields, try to find providers for each field individually
		// This is a fallback mechanism for complex filter scenarios
		c.debug.AddEntry("SELECT", "Attempting fallback provider selection for complex filter scenario", nil)

		missingFields := []string{}
		for _, field := range fields {
			if _, ok := selected[field.Field.Name]; !ok {
				fieldType := field.Field.Type
				c.debug.AddEntry("SELECT", fmt.Sprintf("Attempting to resolve field %s individually", field.Field.Name), nil)

				// Skip structs and pointers to structs as they'll be auto-resolved
				if fieldType.Kind() == reflect.Struct || (fieldType.Kind() == reflect.Ptr && fieldType.Elem().Kind() == reflect.Struct) {
					selected[field.Field.Name] = nil // Mark for auto-resolution
					c.debug.LogProviderSelection(fieldType, nil, nil)
					continue
				}

				// Try to find a provider for this field
				providers, ok := c.providers[fieldType]
				if !ok {
					missingFields = append(missingFields, field.Field.Name)
					continue // Skip if no providers found
				}

				// Find a provider that matches the field's constraints
				providerFound := false
				for i := range providers {
					provider := providers[i]

					// Check if provider matches literals and filters
					matches := true

					// Check literals
					for key, literal := range field.Literals {
						if val, ok := provider.Metadata[key].(string); !ok || val != literal {
							matches = false
							break
						}
					}

					// Check filters
					if matches && len(field.Filters) > 0 {
						matches = c.providerMatchesFilters(*provider, field.Filters)
					}

					if matches {
						selected[field.Field.Name] = provider
						c.debug.LogProviderSelection(fieldType, provider, nil)
						providerFound = true
						break
					}
				}

				if !providerFound {
					missingFields = append(missingFields, field.Field.Name)
				}
			}
		}

		// If we still couldn't find providers for all fields, return an error
		if len(missingFields) > 0 {
			return selected, fmt.Errorf("could not find providers for fields: %v", missingFields)
		}
	}

	return selected, nil
}

// tryAssign recursively assigns providers to fields using backtracking.
func tryAssign(c *Container, fields []FieldConstraint, assignment map[string]map[string]string, selected map[string]*Provider, index int) error {
	if index == len(fields) {
		return nil // All fields assigned
	}
	field := fields[index]
	fieldType := field.Field.Type

	// Handle structs and pointers to structs
	if fieldType.Kind() == reflect.Struct || (fieldType.Kind() == reflect.Ptr && fieldType.Elem().Kind() == reflect.Struct) {
		selected[field.Field.Name] = nil // Mark for auto-resolution
		c.debug.LogProviderSelection(fieldType, nil, nil)
		return tryAssign(c, fields, assignment, selected, index+1)
	}

	providers, ok := c.providers[fieldType]
	if !ok {
		c.debug.LogProviderSelection(fieldType, nil, nil)
		return fmt.Errorf("no providers for type %v", fieldType)
	}

	// Keep track of filtered providers for debugging
	filteredProviders := make([]*Provider, 0)

	for i := range providers {
		provider := providers[i] // Get a pointer to the actual provider

		// Check literals
		satisfiesLiterals := true
		for key, literal := range field.Literals {
			if val, ok := provider.Metadata[key].(string); !ok || val != literal {
				satisfiesLiterals = false
				filteredProviders = append(filteredProviders, provider)
				break
			}
		}
		if !satisfiesLiterals {
			continue
		}

		// Check placeholders
		satisfiesPlaceholders := true
		for key, placeholder := range field.Placeholders {
			if assignedVal, ok := assignment[key][placeholder]; ok {
				if val, ok := provider.Metadata[key].(string); !ok || val != assignedVal {
					satisfiesPlaceholders = false
					filteredProviders = append(filteredProviders, provider)
					break
				}
			}
		}
		if !satisfiesPlaceholders {
			continue
		}

		// Check filters
		if !c.providerMatchesFilters(*provider, field.Filters) {
			filteredProviders = append(filteredProviders, provider)
			continue
		}

		// Assign provider
		newAssignment := CopyAssignment(assignment)
		for key, placeholder := range field.Placeholders {
			if _, ok := newAssignment[key]; !ok {
				newAssignment[key] = make(map[string]string)
			}
			if _, ok := newAssignment[key][placeholder]; !ok {
				if val, ok := provider.Metadata[key].(string); ok {
					newAssignment[key][placeholder] = val
				} else {
					continue
				}
			}
		}

		selected[field.Field.Name] = provider
		// Log successful selection
		c.debug.LogProviderSelection(fieldType, provider, filteredProviders)

		if err := tryAssign(c, fields, newAssignment, selected, index+1); err == nil {
			return nil
		}

		// Backtrack
		filteredProviders = append(filteredProviders, provider)
		delete(selected, field.Field.Name) // Backtrack
	}

	// No suitable provider found
	c.debug.LogProviderSelection(fieldType, nil, filteredProviders)
	return fmt.Errorf("no suitable provider for field %s", field.Field.Name)
}

// CopyAssignment creates a deep copy of the assignment map.
func CopyAssignment(assignment map[string]map[string]string) map[string]map[string]string {
	newAssignment := make(map[string]map[string]string)
	for key, inner := range assignment {
		newInner := make(map[string]string)
		for k, v := range inner {
			newInner[k] = v
		}
		newAssignment[key] = newInner
	}
	return newAssignment
}
