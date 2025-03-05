package di

import (
	"fmt"
	"reflect"
	"strings"
)

// DependencyResolutionError represents a failure to resolve a dependency
type DependencyResolutionError struct {
	DependencyType        reflect.Type
	AppliedFilters        []Filter
	AvailableProviders    []*Provider
	DisqualifiedProviders map[*Provider][]Filter // Maps providers to filters that disqualified them
	Cause                 error                  // For nested resolution errors
}

func (e *DependencyResolutionError) Error() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Failed to resolve dependency of type %v\n", e.DependencyType))

	if len(e.AppliedFilters) > 0 {
		sb.WriteString("Applied filters:\n")
		for i, filter := range e.AppliedFilters {
			sb.WriteString(fmt.Sprintf("  %d. %T\n", i+1, filter))
		}
	}

	if len(e.AvailableProviders) == 0 {
		sb.WriteString("No providers registered for this type\n")
	} else if len(e.DisqualifiedProviders) > 0 {
		sb.WriteString("Type-compatible providers that were disqualified by filters:\n")
		i := 1
		for p, filters := range e.DisqualifiedProviders {
			sb.WriteString(fmt.Sprintf("  %d. Provider for %v with metadata %v:\n", i, p.provides, p.Metadata))
			sb.WriteString("     Disqualified by filters:\n")
			for j, f := range filters {
				sb.WriteString(fmt.Sprintf("     %d.%d. %T", i, j+1, f))

				// Add more details about the filter if possible
				switch typedFilter := f.(type) {
				case *NameEqualsFilter:
					sb.WriteString(fmt.Sprintf(" (key=%s, expected=%s)", typedFilter.key, typedFilter.name))
				case *VersionGreaterThanFilter:
					sb.WriteString(fmt.Sprintf(" (key=%s, version=%s)", typedFilter.key, typedFilter.version))
				}
				sb.WriteString("\n")
			}
			i++
		}
	}

	if e.Cause != nil {
		sb.WriteString(fmt.Sprintf("Caused by: %s\n", e.Cause))
	}

	return sb.String()
}

// NewDependencyNotFoundError creates an error for when no provider is registered for a type
func NewDependencyNotFoundError(dependencyType reflect.Type, filters []Filter) *DependencyResolutionError {
	return &DependencyResolutionError{
		DependencyType:     dependencyType,
		AppliedFilters:     filters,
		AvailableProviders: []*Provider{},
	}
}

// NewDependencyFilteredError creates an error for when providers exist but are filtered out
func NewDependencyFilteredError(
	dependencyType reflect.Type,
	filters []Filter,
	providers []*Provider,
	disqualified map[*Provider][]Filter,
) *DependencyResolutionError {
	return &DependencyResolutionError{
		DependencyType:        dependencyType,
		AppliedFilters:        filters,
		AvailableProviders:    providers,
		DisqualifiedProviders: disqualified,
	}
}
