package di

import (
	"reflect"
)

// Provider represents a dependency provider with metadata and scope.
type Provider struct {
	fn         interface{}            // Factory function, e.g., func(dep1, dep2) T
	provides   reflect.Type           // Type it provides
	ParamTypes []reflect.Type         // Types of the parameters
	Metadata   map[string]interface{} // Metadata for filtering
	scope      string                 // "singleton" or "prototype"
}

// RegisterProvider registers a provider for type T with metadata and scope.
func (c *Container) RegisterProvider(fn interface{}, metadata map[string]interface{}, scope string) {
	fnType := reflect.TypeOf(fn)
	if fnType.Kind() != reflect.Func {
		panic("provider must be a function")
	}
	if fnType.NumOut() != 1 {
		panic("provider must return exactly one value")
	}

	// Validate scope
	if scope != "singleton" && scope != "prototype" {
		panic("scope must be either 'singleton' or 'prototype'")
	}

	returnType := fnType.Out(0)
	paramTypes := make([]reflect.Type, fnType.NumIn())
	for i := 0; i < fnType.NumIn(); i++ {
		paramTypes[i] = fnType.In(i)
	}
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	provider := &Provider{
		fn:         fn,
		provides:   returnType,
		ParamTypes: paramTypes,
		Metadata:   metadata,
		scope:      scope,
	}
	if _, ok := c.providers[returnType]; !ok {
		c.providers[returnType] = make([]*Provider, 0)
	}
	c.providers[returnType] = append(c.providers[returnType], provider)

	// Log the registration
	c.debug.LogRegistration(fnType, returnType, metadata, scope)
}
