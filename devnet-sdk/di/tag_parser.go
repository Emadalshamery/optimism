package di

import (
	"fmt"
	"reflect"
	"strings"
)

// FieldConstraint holds the constraints for a struct field.
type FieldConstraint struct {
	Field        reflect.StructField
	Literals     map[string]string
	Placeholders map[string]string
	Filters      []Filter
}

// ParseTag parses the DI tag into literals, placeholders, and filters.
func (c *Container) ParseTag(tag string) (literals map[string]string, placeholders map[string]string, filters []Filter, err error) {
	literals = make(map[string]string)
	placeholders = make(map[string]string)
	filters = []Filter{}

	if tag == "" {
		return
	}

	parts := strings.Split(tag, ",")
	for _, part := range parts {
		keyValue := strings.SplitN(part, "=", 2)
		if len(keyValue) != 2 {
			err = fmt.Errorf("invalid tag format: %s", part)
			return
		}

		key := strings.TrimSpace(keyValue[0])
		value := strings.TrimSpace(keyValue[1])

		// Check for empty key or value
		if key == "" || value == "" {
			err = fmt.Errorf("invalid tag format: key and value cannot be empty in '%s'", part)
			return
		}

		// Check for filter expression
		if strings.Contains(value, "(") || strings.Contains(value, ")") {
			// Validate filter expression format
			openParenIndex := strings.Index(value, "(")
			closeParenIndex := strings.LastIndex(value, ")")

			if openParenIndex == -1 || closeParenIndex == -1 {
				err = fmt.Errorf("malformed filter expression: missing parenthesis in '%s'", value)
				return
			}

			if openParenIndex > closeParenIndex {
				err = fmt.Errorf("malformed filter expression: closing parenthesis before opening in '%s'", value)
				return
			}

			filterName := strings.TrimSpace(value[:openParenIndex])
			if filterName == "" {
				err = fmt.Errorf("malformed filter expression: missing filter name in '%s'", value)
				return
			}

			paramValue := value[openParenIndex+1 : closeParenIndex]

			factory, exists := c.filters[filterName]
			if !exists {
				err = fmt.Errorf("unknown di.Filter type: %s", filterName)
				return
			}

			filter := factory(key, paramValue)
			filters = append(filters, filter)
		} else if strings.HasPrefix(value, "$") {
			// Handle placeholder (e.g., $environment)
			placeholder := value[1:]
			placeholders[key] = placeholder
		} else if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
			// Handle placeholder with braces (e.g., ${environment})
			placeholder := value[2 : len(value)-1]
			placeholders[key] = placeholder
		} else {
			// Handle literal value
			// Check for malformed quoted strings
			if strings.Count(value, "'") == 1 {
				err = fmt.Errorf("malformed quoted string: unbalanced quotes in '%s'", value)
				return
			}

			if strings.HasPrefix(value, "'") && !strings.HasSuffix(value, "'") {
				err = fmt.Errorf("malformed quoted string: missing closing quote in '%s'", value)
				return
			}

			if !strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") {
				err = fmt.Errorf("malformed quoted string: missing opening quote in '%s'", value)
				return
			}

			// Check for extra quotes in the middle
			if strings.Count(value, "'") > 2 {
				err = fmt.Errorf("malformed quoted string: extra quotes in '%s'", value)
				return
			}

			// Remove surrounding quotes if present
			if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") {
				value = value[1 : len(value)-1]
			}

			literals[key] = value
		}
	}

	return
}
