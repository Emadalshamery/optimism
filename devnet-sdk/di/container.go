package di

import (
	"reflect"
)

// Container is a dependency injection container.
type Container struct {
	providers  map[reflect.Type][]*Provider
	filters    map[string]func(string, string) Filter
	singletons map[reflect.Type]interface{}
	resolving  map[reflect.Type]bool
	debug      *DebugTrace
}

// NewContainer creates a new Container.
func NewContainer() *Container {
	c := &Container{
		providers:  make(map[reflect.Type][]*Provider),
		filters:    make(map[string]func(string, string) Filter),
		singletons: make(map[reflect.Type]interface{}),
		resolving:  make(map[reflect.Type]bool),
		debug:      newDebugTrace(DebugOptions{Enabled: false}),
	}
	// Register built-in filters
	c.RegisterFilter("nameEquals", func(key, param string) Filter {
		return &NameEqualsFilter{key: key, name: param}
	})
	c.RegisterFilter("versionGreaterThan", func(key, param string) Filter {
		return &VersionGreaterThanFilter{key: key, version: param}
	})
	return c
}

// EnableDebug enables debug tracing with the specified options
func (c *Container) EnableDebug(options DebugOptions) {
	options.Enabled = true
	c.debug = newDebugTrace(options)
}

// DisableDebug disables debug tracing
func (c *Container) DisableDebug() {
	c.debug = newDebugTrace(DebugOptions{Enabled: false})
}

// GetDebugTrace returns the debug trace
func (c *Container) GetDebugTrace() *DebugTrace {
	return c.debug
}

// Build initializes the container and performs validation.
func (c *Container) Build() {
	// Currently a no-op, but can be used for validation in the future
}

// RegisterFilter registers a filter factory.
func (c *Container) RegisterFilter(name string, factory func(string, string) Filter) {
	c.filters[name] = factory
}

// Clone creates a copy of the container with the same providers and filters.
func (c *Container) Clone() *Container {
	newC := &Container{
		providers:  make(map[reflect.Type][]*Provider),
		filters:    make(map[string]func(string, string) Filter),
		singletons: make(map[reflect.Type]interface{}),
		resolving:  make(map[reflect.Type]bool),
		debug:      c.debug, // Share the same debug trace
	}

	// Copy providers
	for t, ps := range c.providers {
		newProviders := make([]*Provider, len(ps))
		copy(newProviders, ps)
		newC.providers[t] = newProviders
	}

	// Copy filters
	for name, f := range c.filters {
		newC.filters[name] = f
	}

	return newC
}

// GetProviders returns the providers for a given type (for testing)
func (c *Container) GetProviders(t reflect.Type) []*Provider {
	return c.providers[t]
}

// RegisterMetaProvider registers a function that can register multiple providers
func (c *Container) RegisterMetaProvider(metaProvider func(c *Container)) {
	metaProvider(c)
}
