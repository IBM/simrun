# Code Comments Guidelines

When writing or modifying code, follow these guidelines to avoid excessive comments while maintaining clarity.

## Comments to AVOID

### 1. Comments that restate the code
```go
// BAD: Comment restates what the code already says
// Create directory if it doesn't exist
if err := os.MkdirAll(dir, 0755); err != nil {

// BAD: Field name already describes itself
// ElasticsearchURL is the URL of the Elasticsearch instance
ElasticsearchURL string

// BAD: Function name is descriptive enough
// Build the search query
query := c.buildSearchQuery(indicators)
```

### 2. Comments before obvious operations
```go
// BAD: The function call is self-explanatory
// Execute the search
documents, err := c.executeSearch(query)

// BAD: Variable assignment is clear
// Extract the key from the template
key := strings.TrimPrefix(value, "{{")
```

### 3. Redundant struct field comments
```go
// BAD: Every field has an obvious comment
type Config struct {
    // Index is the Elasticsearch index
    Index string
    // Timeout is the timeout duration
    Timeout time.Duration
}
```

## Comments to KEEP

### 1. Non-obvious behavior or business logic
```go
// GOOD: Explains why this specific field is used for matching
// UserAgentField is the field used to search for user-agent containing detonation UUID
UserAgentField string
```

### 2. Default values and constraints
```go
// GOOD: Documents default as inline annotation
OutputDir string `json:"outputDir"` // default: "logs"
```

### 3. Format specifications and examples
```go
// GOOD: Shows expected format with example
// Output structure: {outputDir}/{scenario}/{timestamp}_{execution_id}.ndjson

// GOOD: Template syntax example
// AdditionalFields values can be templates like "{{ indicators.terraformOutput.key }}"
```

### 4. Non-obvious implementation details
```go
// GOOD: Explains caching behavior
client *elasticsearch.Client // cached

// GOOD: Clarifies relationship between options
CloudID string // alternative to ElasticsearchURL
```

### 5. Doc comments on exported types and functions (Go convention)
```go
// GOOD: Standard Go doc comment
// ElasticCollector collects logs from Elasticsearch based on scenario indicators
type ElasticCollector struct {
```

### 6. Complex algorithm explanations
```go
// GOOD: Explains non-trivial logic
// Strip known prefixes to normalize indicator keys
if strings.HasPrefix(key, "indicators.terraformOutput.") {
```

## General Principles

1. **Self-documenting code first**: Use descriptive variable and function names instead of comments
2. **Comment the "why", not the "what"**: Explain intent and reasoning, not mechanics
3. **Inline annotations for metadata**: Use short inline comments for defaults, caching, alternatives
4. **Keep doc comments**: Maintain standard doc comments on exported types/functions
5. **Examples are valuable**: Template formats, output structures, and usage patterns help
6. **Delete over document**: If code needs a comment to be understood, consider refactoring first
