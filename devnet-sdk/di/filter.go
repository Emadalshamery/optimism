package di

// Filter defines a condition that a provider's metadata must satisfy.
type Filter interface {
	AppliesTo(key string) bool
	Matches(value interface{}) bool
}

// providerMatchesFilters checks if a provider's metadata satisfies all filters.
func (c *Container) providerMatchesFilters(p Provider, filters []Filter) bool {
	for _, f := range filters {
		matched := false
		for key, value := range p.Metadata {
			if f.AppliesTo(key) && f.Matches(value) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

// VersionGreaterThanFilter matches if a string metadata value is greater than a version string.
type VersionGreaterThanFilter struct {
	key     string
	version string
}

func (f *VersionGreaterThanFilter) AppliesTo(key string) bool {
	return key == f.key
}

func (f *VersionGreaterThanFilter) Matches(value interface{}) bool {
	if v, ok := value.(string); ok {
		return v > f.version
	}
	return false
}

// NameEqualsFilter matches if a string metadata value equals a name.
type NameEqualsFilter struct {
	key  string
	name string
}

// NewNameEqualsFilter creates a new NameEqualsFilter.
func NewNameEqualsFilter(key, name string) *NameEqualsFilter {
	return &NameEqualsFilter{
		key:  key,
		name: name,
	}
}

func (f *NameEqualsFilter) AppliesTo(key string) bool {
	return key == f.key
}

func (f *NameEqualsFilter) Matches(value interface{}) bool {
	if v, ok := value.(string); ok {
		return v == f.name
	}
	return false
}
