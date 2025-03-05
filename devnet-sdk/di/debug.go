package di

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
)

// DebugOptions configures the behavior of the debug tracing system
type DebugOptions struct {
	Enabled           bool
	TraceRegistration bool
	TraceResolution   bool
	TraceCaching      bool
	TraceInvocation   bool
	Verbose           bool
}

// DebugEntry represents a single debug log entry
type DebugEntry struct {
	Type    string
	Message string
	Data    map[string]interface{}
}

// DebugTrace holds all debug information for a container
type DebugTrace struct {
	mu      sync.Mutex
	entries []DebugEntry
	options DebugOptions
}

// newDebugTrace creates a new debug trace with the specified options
func newDebugTrace(options DebugOptions) *DebugTrace {
	return &DebugTrace{
		entries: []DebugEntry{},
		options: options,
	}
}

// AddEntry adds a debug entry to the trace
func (t *DebugTrace) AddEntry(entryType, message string, data map[string]interface{}) {
	if !t.options.Enabled {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.entries = append(t.entries, DebugEntry{
		Type:    entryType,
		Message: message,
		Data:    data,
	})

	// If verbose is enabled, also print to stdout
	if t.options.Verbose {
		fmt.Printf("[DI DEBUG] %s: %s\n", entryType, message)
		if len(data) > 0 {
			for k, v := range data {
				fmt.Printf("  - %s: %v\n", k, v)
			}
		}
	}
}

// LogRegistration logs provider registration events
func (t *DebugTrace) LogRegistration(fnType reflect.Type, providesType reflect.Type, metadata map[string]interface{}, scope string) {
	if !t.options.Enabled || !t.options.TraceRegistration {
		return
	}

	var paramTypes []string
	for i := 0; i < fnType.NumIn(); i++ {
		paramTypes = append(paramTypes, fnType.In(i).String())
	}

	t.AddEntry("REGISTER", fmt.Sprintf("Registered provider for %s", providesType), map[string]interface{}{
		"scope":      scope,
		"parameters": strings.Join(paramTypes, ", "),
		"metadata":   metadata,
	})
}

// LogResolutionAttempt logs attempts to resolve dependencies
func (t *DebugTrace) LogResolutionAttempt(targetType reflect.Type, filters []Filter) {
	if !t.options.Enabled || !t.options.TraceResolution {
		return
	}

	var filterStrs []string
	for _, f := range filters {
		filterStrs = append(filterStrs, fmt.Sprintf("%T", f))
	}

	t.AddEntry("RESOLVE", fmt.Sprintf("Resolving dependency for %s", targetType), map[string]interface{}{
		"filters": strings.Join(filterStrs, ", "),
	})
}

// LogProviderSelection logs provider selection decisions
func (t *DebugTrace) LogProviderSelection(targetType reflect.Type, selectedProvider *Provider, filteredOut []*Provider) {
	if !t.options.Enabled || !t.options.TraceResolution {
		return
	}

	var msg string
	if selectedProvider != nil {
		msg = fmt.Sprintf("Selected provider for %s", targetType)
	} else {
		msg = fmt.Sprintf("No provider found for %s", targetType)
	}

	data := map[string]interface{}{}
	if selectedProvider != nil {
		data["selected_scope"] = selectedProvider.scope
		data["selected_metadata"] = selectedProvider.Metadata
	}

	var filteredProviders []string
	for _, p := range filteredOut {
		filteredProviders = append(filteredProviders, fmt.Sprintf("%v (scope: %s)", p.provides, p.scope))
	}
	data["filtered_out"] = strings.Join(filteredProviders, ", ")

	t.AddEntry("SELECT", msg, data)
}

// LogCacheAccess logs cache access events
func (t *DebugTrace) LogCacheAccess(targetType reflect.Type, cacheHit bool, instance interface{}) {
	if !t.options.Enabled || !t.options.TraceCaching {
		return
	}

	var msg string
	if cacheHit {
		msg = fmt.Sprintf("Cache hit for %s", targetType)
	} else {
		msg = fmt.Sprintf("Cache miss for %s", targetType)
	}

	t.AddEntry("CACHE", msg, map[string]interface{}{
		"cache_hit": cacheHit,
	})
}

// LogCacheStore logs cache storage events
func (t *DebugTrace) LogCacheStore(targetType reflect.Type, instance interface{}) {
	if !t.options.Enabled || !t.options.TraceCaching {
		return
	}

	t.AddEntry("CACHE", fmt.Sprintf("Storing %s in cache", targetType), nil)
}

// LogProviderInvocation logs provider invocation events
func (t *DebugTrace) LogProviderInvocation(provider *Provider, args []interface{}, result interface{}, err error) {
	if !t.options.Enabled || !t.options.TraceInvocation {
		return
	}

	var argStrs []string
	for _, arg := range args {
		if arg == nil {
			argStrs = append(argStrs, "nil")
		} else {
			argStrs = append(argStrs, fmt.Sprintf("%v", arg))
		}
	}

	var msg string
	if err != nil {
		msg = fmt.Sprintf("Provider invocation failed for %s", provider.provides)
	} else {
		msg = fmt.Sprintf("Provider invoked for %s", provider.provides)
	}

	data := map[string]interface{}{
		"scope":     provider.scope,
		"arguments": strings.Join(argStrs, ", "),
	}

	if err != nil {
		data["error"] = err.Error()
	}

	t.AddEntry("INVOKE", msg, data)
}

// LogStructResolution logs struct resolution events
func (t *DebugTrace) LogStructResolution(structType reflect.Type, fieldName string, fieldType reflect.Type, fieldTag string, resolvedValue interface{}, err error) {
	if !t.options.Enabled || !t.options.TraceResolution {
		return
	}

	var msg string
	if err != nil {
		msg = fmt.Sprintf("Failed to resolve field %s of type %s in struct %s", fieldName, fieldType, structType)
	} else {
		msg = fmt.Sprintf("Resolved field %s of type %s in struct %s", fieldName, fieldType, structType)
	}

	data := map[string]interface{}{
		"field_tag": fieldTag,
	}

	if err != nil {
		data["error"] = err.Error()
	}

	t.AddEntry("STRUCT", msg, data)
}

// GetEntries returns a copy of all debug entries
func (t *DebugTrace) GetEntries() []DebugEntry {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Create a copy to avoid concurrent modification issues
	entriesCopy := make([]DebugEntry, len(t.entries))
	copy(entriesCopy, t.entries)
	return entriesCopy
}

// PrintTrace prints the entire debug trace
func (t *DebugTrace) PrintTrace() {
	if !t.options.Enabled {
		fmt.Println("Debug tracing is not enabled")
		return
	}

	entries := t.GetEntries()
	fmt.Printf("=== DI DEBUG TRACE (%d entries) ===\n", len(entries))
	for i, entry := range entries {
		fmt.Printf("%d. [%s] %s\n", i+1, entry.Type, entry.Message)
		for k, v := range entry.Data {
			fmt.Printf("   - %s: %v\n", k, v)
		}
	}
	fmt.Println("=== END DEBUG TRACE ===")
}
