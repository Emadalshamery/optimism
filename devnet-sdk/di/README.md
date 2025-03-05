# Devnet SDK Dependency Injection Framework Documentation

Welcome to the user documentation for the Devnet-SDK Dependency Injection (DI) framework written in Go. This guide introduces the framework step by step, starting with basic concepts and progressing to advanced features, so you can effectively apply it in your projects.

## 1. Introduction to Dependency Injection

**Dependency Injection (DI)** is a design pattern that promotes modularity, testability, and maintainability. Instead of components creating their own dependencies (e.g., a database connection), dependencies are "injected" from an external source. This reduces coupling, making it easier to swap implementations or mock dependencies for testing.

The core of this framework is the **`Container`**, which manages dependencies by:

- **Registering providers**: Defining how to create instances of specific types.
- **Resolving dependencies**: Providing instances as needed, handling their creation and dependency injection.

Let's get started by setting up the `Container`.

---

## 2. Getting Started with the Container

To use the DI framework, you first create a `Container`, the central hub for registering and resolving dependencies.

### Creating a Container

Create a new `Container` with the `NewContainer` function:

```go
package main

import "di"

func main() {
    container := di.NewContainer()
}
```

This initializes a `Container` with default settings, including built-in filters (covered later). Think of it as a blank slate for managing your application's dependencies.

---

## 3. Registering Providers

A **Provider** is a function that creates an instance of a specific type—like a recipe the `Container` follows. Providers can depend on other types, and the `Container` resolves those dependencies automatically.

### Registering a Simple Provider

Use the `RegisterProvider` method, specifying:

- **Provider function**: The function that creates the instance (e.g., a constructor).
- **Metadata**: Optional key-value pairs describing the provider (can be `nil`).
- **Scope**: `"singleton"` (one shared instance) or `"prototype"` (new instance each time).

Example:

```go
type Config struct {
    Host string
    Port int
}

func NewConfig() *Config {
    return &Config{Host: "localhost", Port: 5432}
}

func main() {
    container := di.NewContainer()
    container.RegisterProvider(NewConfig, nil, "singleton")
}
```

- `NewConfig` returns a `*Config`.
- `nil` indicates no metadata.
- `"singleton"` means the same instance is reused.

### Providers with Dependencies

Providers can depend on other registered types. Here's a `Database` that depends on `Config`:

```go
type Database struct {
    Config *Config
}

func NewDatabase(config *Config) *Database {
    return &Database{Config: config}
}

func main() {
    container := di.NewContainer()
    container.RegisterProvider(NewConfig, nil, "singleton")
    container.RegisterProvider(NewDatabase, nil, "singleton")
}
```

When creating a `Database`, the `Container` resolves `*Config` and passes it to `NewDatabase`.

---

## 4. Resolving Dependencies

Use the `Provide` function to request instances from the `Container`.

### Basic Resolution

Specify the desired type with Go's generics:

```go
db, err := di.Provide[*Database](container, nil)
if err != nil {
    panic(err) // Handle error (e.g., no provider registered)
}
fmt.Println(db.Config.Host) // "localhost"
```

- `Provide[*Database]` requests a `*Database`.
- `nil` is for filters (covered later).
- The `Container` resolves dependencies automatically.

### Error Handling

`Provide` returns an error if no provider exists or dependencies can't be resolved. Always check errors to handle missing dependencies.

---

## 5. Understanding Scopes: Singleton and Prototype

The **scope** controls instance management:

- **Singleton**: One instance reused for all requests.
- **Prototype**: A new instance each time.

### Example with Scopes

```go
// Singleton: Same instance
container.RegisterProvider(NewDatabase, nil, "singleton")
db1, _ := di.Provide[*Database](container, nil)
db2, _ := di.Provide[*Database](container, nil)
fmt.Println(db1 == db2) // true

// Prototype: New instance
container.RegisterProvider(NewDatabase, nil, "prototype")
db3, _ := di.Provide[*Database](container, nil)
db4, _ := di.Provide[*Database](container, nil)
fmt.Println(db3 == db4) // false
```

### Choosing a Scope

- **Singleton**: For shared resources (e.g., database connections).
- **Prototype**: For fresh instances (e.g., request handlers).

---

## 6. Using Metadata and Filters

When multiple providers exist for a type (e.g., `Database` for "production" and "development"), **metadata** and **filters** help distinguish them.

### Adding Metadata to Providers

Metadata is a map of key-value pairs attached to providers:

```go
prodMetadata := map[string]interface{}{
    "env": "production",
}
devMetadata := map[string]interface{}{
    "env": "development",
}

container.RegisterProvider(NewDatabase, prodMetadata, "singleton")
container.RegisterProvider(NewDatabase, devMetadata, "singleton")
```

Two `*Database` providers are registered, differentiated by `env`.

### Using Filters to Select Providers

**Filters** select providers based on metadata conditions. They're registered with the `Container` and used in DI tags or manual resolution.

#### Built-in Filters

The framework provides:

- **`nameEquals`**: Matches if a metadata key equals a value.
- **`versionGreaterThan`**: Matches if a metadata key exceeds a version string.

Manual resolution example:

```go
filter := &NameEqualsFilter{key: "env", name: "production"}
db, err := di.Provide[*Database](container, []di.Filter{filter})
if err != nil {
    panic(err)
}
```

In DI tags, use `key=filterType(param)` syntax (see Section 7).

#### Custom Filters

Register custom filters for specific needs, like checking numerical values:

1. **Define the filter**:

```go
type GreaterThanFilter struct {
    key   string
    value int
}

func (f *GreaterThanFilter) AppliesTo(key string) bool {
    return key == f.key
}

func (f *GreaterThanFilter) Matches(value interface{}) bool {
    if v, ok := value.(int); ok {
        return v > f.value
    }
    return false
}
```

2. **Register it**:

```go
container.RegisterFilter("greaterThan", func(key, param string) di.Filter {
    value, _ := strconv.Atoi(param) // Convert param to int
    return &GreaterThanFilter{key: key, value: value}
})
```

3. **Use in a tag**:

```go
type App struct {
    Age int `di:"age=greaterThan(18)"`
}
```

This selects a provider with `"age"` metadata greater than 18.

---

## 7. Auto-Resolving Structs

Automatically populate structs using **DI tags**, ideal for managing multiple dependencies.

### Defining a Struct with DI Tags

Add `di` tags to fields:

```go
type App struct {
    ProdDB  *Database `di:"env=production"`
    DevDB   *Database `di:"env=development"`
    Version string    `di:"version=versionGreaterThan(1.0.0)"`
}
```

- `ProdDB`: `*Database` with `env="production"`.
- `DevDB`: `*Database` with `env="development"`.
- `Version`: `string` with `version > "1.0.0"`.

### Resolving the Struct

Use `ProvideStruct`:

```go
app, err := di.ProvideStruct[App](container)
if err != nil {
    panic(err)
}
```

The `Container` parses tags and injects matching providers.

### Tag Syntax

DI tags support three constraints:

- **Literals**: Exact matches, e.g., `di:"env=production"` or `di:"env='production'"`.
- **Placeholders**: Variables, e.g., `di:"env=$env"`.
- **Filters**: Conditions, e.g., `di:"version=versionGreaterThan(1.0.0)"`.

#### Literals

Exact metadata matches. Quotes are optional for simple values, required for special characters.

#### Placeholders

Ensure fields share metadata values:

```go
type App struct {
    DB1 *Database `di:"env=$env"`
    DB2 *Database `di:"env=$env"`
}
```

`DB1` and `DB2` get providers with the same `env`.

#### Filters

Use `key=filterType(param)` for complex conditions:

```go
type App struct {
    DB *Database `di:"env=nameEquals(production)"`
}
```

Selects a `*Database` with `env="production"`.

---

## 8. Advanced Topics

### Meta-Providers

**Meta-providers** register multiple providers at once:

```go
func registerDatabaseProviders(c *di.Container) {
    c.RegisterProvider(NewConfig, nil, "singleton")
    c.RegisterProvider(NewDatabase, nil, "singleton")
}

func main() {
    container := di.NewContainer()
    container.RegisterMetaProvider(registerDatabaseProviders)
    container.Build() // Executes meta-providers
}
```

Call `Build` to populate the `Container`.

### Registering Custom Filters

Extend functionality with custom filters:

```go
container.RegisterFilter("greaterThan", func(key, param string) di.Filter {
    value, _ := strconv.Atoi(param)
    return &GreaterThanFilter{key: key, value: value}
})
```

Use it in tags: `di:"age=greaterThan(18)"`.

---

## Complete Example

```go
package main

import (
    "di"
    "fmt"
    "strconv"
)

type Config struct {
    Host string
}

type Database struct {
    Config *Config
}

type App struct {
    ProdDB *Database `di:"env=production"`
    DevDB  *Database `di:"env=development"`
}

func NewConfig() *Config {
    return &Config{Host: "localhost"}
}

func NewDatabase(config *Config) *Database {
    return &Database{Config: config}
}

type GreaterThanFilter struct {
    key   string
    value int
}

func (f *GreaterThanFilter) AppliesTo(key string) bool {
    return key == f.key
}

func (f *GreaterThanFilter) Matches(value interface{}) bool {
    if v, ok := value.(int); ok {
        return v > f.value
    }
    return false
}

func main() {
    container := di.NewContainer()

    // Register providers
    container.RegisterProvider(NewConfig, nil, "singleton")
    container.RegisterProvider(NewDatabase, map[string]interface{}{
        "env": "production",
    }, "singleton")
    container.RegisterProvider(NewDatabase, map[string]interface{}{
        "env": "development",
    }, "singleton")

    // Register custom filter
    container.RegisterFilter("greaterThan", func(key, param string) di.Filter {
        value, _ := strconv.Atoi(param)
        return &GreaterThanFilter{key: key, value: value}
    })

    // Resolve struct
    app, err := di.ProvideStruct[App](container)
    if err != nil {
        panic(err)
    }

    fmt.Println(app.ProdDB.Config.Host) // "localhost"
    fmt.Println(app.DevDB.Config.Host)  // "localhost"
}
```

## 11. Error Handling and Debugging

The DI framework provides detailed error messages to help you diagnose dependency resolution issues.

### Enhanced Error Messages

When dependency resolution fails, the framework provides detailed error messages that explain exactly why the resolution failed:

#### 1. Missing Provider

When no provider is registered for a requested type:

```
Failed to resolve dependency of type example.Service
No providers registered for this type
```

#### 2. Filtered Out Providers

When providers exist but are filtered out by constraints:

```
Failed to resolve dependency of type example.Service
Applied filters:
  1. *di.NameEqualsFilter
Type-compatible providers that were disqualified by filters:
  1. Provider for example.Service with metadata map[name:service1 version:1.0]:
     Disqualified by filters:
     1.1. *di.NameEqualsFilter (key=name, expected=service2)
```

This tells you:
- What filters were applied
- Which providers were available but disqualified
- For each disqualified provider, which specific filter caused the disqualification

#### 3. Nested Dependency Failures

For nested dependency failures, the error shows the full dependency chain:

```
Failed to resolve dependency of type example.Service
Caused by: failed to resolve parameter 0 of type example.Database for provider of example.Service:
Failed to resolve dependency of type example.Database
No providers registered for this type
```

#### 4. Struct Field Resolution Failures

For struct auto-resolution failures:

```
Failed to resolve dependency of type example.App
Caused by: error resolving struct field DB: Failed to resolve dependency of type example.Database
No providers registered for this type
```

### Using Debug Mode

For even more detailed information, you can enable debug mode:

```go
container.EnableDebug(di.DebugOptions{
    Enabled: true,
    IncludeValues: true,
})

// ... resolve dependencies ...

// Get the debug trace
trace := container.GetDebugTrace()
fmt.Println(trace.String()) // Prints detailed resolution steps
```

Debug mode tracks all resolution attempts, provider selections, and cache operations, giving you a complete picture of what happened during dependency resolution.

---
