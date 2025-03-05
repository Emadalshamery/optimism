package di

import (
	"fmt"
	"reflect"
	"strings"
)

// TagConstraints represents constraints parsed from a DI tag
type TagConstraints struct {
	Literals     map[string]string
	Placeholders map[string]string
	Filters      []Filter
}

// String returns a string representation of the constraints
func (tc TagConstraints) String() string {
	parts := make([]string, 0)

	for k, v := range tc.Literals {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}

	for k, v := range tc.Placeholders {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}

	return strings.Join(parts, ",")
}

// parseTag parses a DI tag string into literals, placeholders, and filters
func parseTag(tag string, registeredFilters map[string]func(string, string) Filter) (TagConstraints, error) {
	result := TagConstraints{
		Literals:     make(map[string]string),
		Placeholders: make(map[string]string),
		Filters:      make([]Filter, 0),
	}

	if tag == "" {
		return result, nil
	}

	parts := strings.Split(tag, ",")
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			return result, fmt.Errorf("invalid tag part: %s", part)
		}

		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])

		// Check if this is a placeholder
		if strings.HasPrefix(value, "$") {
			// This is a placeholder
			placeholder := value[1:]
			result.Placeholders[key] = placeholder
		} else if strings.Contains(value, "(") && strings.HasSuffix(value, ")") {
			// This is a filter expression like nameEquals(value) or versionGreaterThan(1.0)
			filterNameEnd := strings.Index(value, "(")
			filterName := value[:filterNameEnd]

			// Extract parameter from between parentheses
			paramValue := value[filterNameEnd+1 : len(value)-1]

			// Find the filter factory
			filterFactory, exists := registeredFilters[filterName]
			if !exists {
				return result, fmt.Errorf("unknown di.Filter type: %s", filterName)
			}

			// Create the filter with the key and parameter
			filter := filterFactory(key, paramValue)
			result.Filters = append(result.Filters, filter)
		} else {
			// This is a literal value (remove quotes if present)
			if (strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) ||
				(strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) {
				value = value[1 : len(value)-1]
			}
			result.Literals[key] = value
		}
	}

	return result, nil
}

// ProvideStruct creates and populates a struct of type T based on DI tags.
func ProvideStruct[T any](c *Container) (T, error) {
	var zero T
	t := reflect.TypeOf((*T)(nil)).Elem()

	if t.Kind() != reflect.Struct {
		return zero, fmt.Errorf("T must be a struct type")
	}

	c.debug.LogResolutionAttempt(t, nil)

	val, err := c.autoResolveStruct(t, nil)
	if err != nil {
		return zero, err
	}

	return val.(T), nil
}

// autoResolveStruct auto-resolves a struct-typed dependency.
func (c *Container) autoResolveStruct(t reflect.Type, filters []Filter) (interface{}, error) {
	c.debug.LogStructResolution(t, "", t, "", nil, nil)

	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("type %v is not a struct", t)
	}

	// Create an instance of the struct
	result := reflect.New(t).Elem()
	prototype := false

	// Only handle fields with "di" tags
	numFields := t.NumField()
	fmt.Printf("Auto-resolving struct type: %v with %d fields\n", t, numFields)

	// First, collect all fields with "di" tags
	var taggedFields []FieldConstraint
	for i := 0; i < numFields; i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		tag := field.Tag.Get("di")
		if tag == "" {
			continue
		}

		// Parse the tag
		constraints, err := parseTag(tag, c.filters)
		if err != nil {
			return nil, fmt.Errorf("invalid di tag on field %s: %w", field.Name, err)
		}

		// Create a new FieldConstraint with the parsed constraints
		fieldConstraint := FieldConstraint{
			Field:        field,
			Literals:     constraints.Literals,
			Placeholders: constraints.Placeholders,
			Filters:      constraints.Filters,
		}

		// If we have filters from the parent, add them to the field's filters
		fieldConstraint.Filters = append(fieldConstraint.Filters, filters...)

		taggedFields = append(taggedFields, fieldConstraint)
	}

	// Select providers for all fields with tags
	if len(taggedFields) > 0 {
		providers, err := selectProviders(c, taggedFields)
		if err != nil {
			return nil, fmt.Errorf("error resolving struct: %w", err)
		}

		// Resolve each field
		for _, field := range taggedFields {
			fmt.Printf("Resolving field with di tag: %s of type %v\n", field.Field.Name, field.Field.Type)

			// When a field has placeholders, we need to select the right provider
			if len(field.Placeholders) > 0 && providers[field.Field.Name] == nil {
				// Try to find a matching provider directly
				fieldType := field.Field.Type
				allProviders, ok := c.providers[fieldType]
				if !ok || len(allProviders) == 0 {
					return nil, fmt.Errorf("no suitable provider for field %s", field.Field.Name)
				}

				// Just use the first provider if multiple are available
				// In more complex cases, you'd want to check placeholders against other fields
				provider := allProviders[0]
				val, err := c.invokeProvider(*provider)
				if err != nil {
					return nil, fmt.Errorf("error resolving struct: %w", err)
				}

				// Set the field value
				fieldValue := result.Field(field.Field.Index[0])
				fieldValue.Set(reflect.ValueOf(val))
				continue
			}

			val, err := c.resolveField(field, providers)
			if err != nil {
				// Try to resolve directly using provide
				fieldType := field.Field.Type
				c.debug.AddEntry("RESOLVE", fmt.Sprintf("Failed to resolve field %s with provider, attempting direct resolution", field.Field.Name), nil)

				resolved, err := c.provide(fieldType, field.Filters)
				if err != nil {
					return nil, fmt.Errorf("error resolving struct field %s: %w", field.Field.Name, err)
				}

				// Set the field value
				fieldValue := result.Field(field.Field.Index[0])
				fieldValue.Set(reflect.ValueOf(resolved))
				continue
			}

			// Check if any provider returns a prototype
			if val != nil && reflect.TypeOf(val).Kind() == reflect.Ptr {
				provider, ok := providers[field.Field.Name]
				if ok && provider != nil && provider.scope == "prototype" {
					prototype = true
				}
			}

			// Set the field value
			fieldValue := result.Field(field.Field.Index[0])
			fieldValue.Set(reflect.ValueOf(val))
		}
	}

	// Now resolve all other exported fields without "di" tags
	for i := 0; i < numFields; i++ {
		field := t.Field(i)
		if !field.IsExported() || field.Tag.Get("di") != "" {
			continue
		}

		fmt.Printf("Auto-resolving field without di tag: %s of type %v\n", field.Name, field.Type)

		fieldType := field.Type
		var val interface{}
		var err error

		if fieldType.Kind() == reflect.Struct {
			// Auto-resolve nested struct
			val, err = c.autoResolveStruct(fieldType, nil)
		} else if fieldType.Kind() == reflect.Ptr && fieldType.Elem().Kind() == reflect.Struct {
			// Auto-resolve pointer to struct
			val, err = c.provide(fieldType, nil)
		} else {
			// Try to resolve using the container
			val, err = c.provide(fieldType, nil)
		}

		if err != nil {
			return nil, fmt.Errorf("error auto-resolving field %s: %w", field.Name, err)
		}

		if val != nil {
			fmt.Printf("Successfully auto-resolved field %s: %v\n", field.Name, val)
			fieldValue := result.Field(i)
			fieldValue.Set(reflect.ValueOf(val))
		}
	}

	fmt.Printf("Successfully resolved struct: %v\n", result.Interface())

	// If any provider is a prototype, the struct should be treated as a prototype
	if prototype {
		return result.Interface(), nil
	}

	// Otherwise, cache it as a singleton
	c.singletons[t] = result.Interface()
	return result.Interface(), nil
}

// resolveField resolves a single field in a struct using specified providers.
func (c *Container) resolveField(field FieldConstraint, providers map[string]*Provider) (interface{}, error) {
	// Check if providers map is nil
	if providers == nil {
		// Try to find a provider directly using the container's provide method with filters
		fieldType := field.Field.Type
		c.debug.AddEntry("RESOLVE", fmt.Sprintf("No provider map for field %s, attempting direct resolution", field.Field.Name), nil)

		// If it's a struct or pointer to struct, try to auto-resolve
		if fieldType.Kind() == reflect.Struct {
			resolved, err := c.autoResolveStruct(fieldType, field.Filters)
			if err != nil {
				return nil, fmt.Errorf("failed to auto-resolve struct field %s: %w", field.Field.Name, err)
			}
			return resolved, nil
		}

		if fieldType.Kind() == reflect.Ptr && fieldType.Elem().Kind() == reflect.Struct {
			// Use the container to provide the pointer
			resolved, err := c.provide(fieldType, field.Filters)
			if err != nil {
				return nil, fmt.Errorf("failed to provide pointer to struct field %s: %w", field.Field.Name, err)
			}
			return resolved, nil
		}

		// Try direct resolution with filters
		resolved, err := c.provide(fieldType, field.Filters)
		if err != nil {
			return nil, fmt.Errorf("no suitable provider for field %s: %w", field.Field.Name, err)
		}
		return resolved, nil
	}

	provider, ok := providers[field.Field.Name]
	if !ok || provider == nil {
		// If no provider or nil provider, attempt to auto-resolve if it's a struct or pointer to struct
		fieldType := field.Field.Type
		c.debug.AddEntry("RESOLVE", fmt.Sprintf("No provider for field %s, attempting direct resolution", field.Field.Name), nil)

		// Try to find a provider directly using the container's provide method with filters
		resolved, err := c.provide(fieldType, field.Filters)
		if err == nil {
			return resolved, nil
		}

		// If it's a struct or pointer to struct, try to auto-resolve
		if fieldType.Kind() == reflect.Struct {
			resolved, err := c.autoResolveStruct(fieldType, field.Filters)
			if err != nil {
				return nil, fmt.Errorf("failed to auto-resolve struct field %s: %w", field.Field.Name, err)
			}
			return resolved, nil
		}

		if fieldType.Kind() == reflect.Ptr && fieldType.Elem().Kind() == reflect.Struct {
			// Use the container to provide the pointer
			resolved, err := c.provide(fieldType, field.Filters)
			if err != nil {
				return nil, fmt.Errorf("failed to provide pointer to struct field %s: %w", field.Field.Name, err)
			}
			return resolved, nil
		}

		return nil, fmt.Errorf("no suitable provider for field %s", field.Field.Name)
	}

	// Invoke the provider
	result, err := c.invokeProvider(*provider)
	if err != nil {
		return nil, err
	}

	return result, nil
}
