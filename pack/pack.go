// Package pack is the SDK for building simrun simulation packs — standalone
// binaries that simrun invokes over a JSON stdin/stdout protocol to detonate
// attacks and report results.
package pack

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

var (
	// packName is the pack name, set via SetPackInfo.
	packName string
	// packVersion is the pack version, set via SetPackInfo.
	packVersion string
	// minSimrunVersion is the minimum simrun version required.
	minSimrunVersion string
	// registry holds all registered simulations.
	registry = make(map[string]*Simulation)
	// templateRegistry holds all registered injection templates.
	templateRegistry = make(map[string]*Template)
	// currentSimulation is set during dispatch for logging context.
	currentSimulation string
)

// SetPackInfo sets the pack metadata. Call this before Run().
func SetPackInfo(name, version, minSimrun string) {
	packName = name
	packVersion = version
	minSimrunVersion = minSimrun
}

// Register registers a simulation with the SDK.
// Call this from your simulation's init() function.
// The ID should be a lean slug (e.g., "ec2-bitcoin-mining"). The SDK combines
// it with Scope to form the simulation ID (scope.slug, e.g., "aws.ec2-bitcoin-mining")
// used in the manifest and wire protocol.
func Register(s Simulation) {
	registerItem("simulation", s.ID, s.Scope, &s, registry)
	validateRequiredOutputs(&s, s.Scope+"."+s.ID)
}

// validateRequiredOutputs panics if Simulation.RequiredOutputs declares
// Terraform output names that are not present as top-level `output "<name>" {}`
// blocks in Simulation.Terraform. It is a no-op when RequiredOutputs is empty.
func validateRequiredOutputs(s *Simulation, fullID string) {
	if len(s.RequiredOutputs) == 0 {
		return
	}
	if s.Terraform == "" {
		panic(fmt.Sprintf("simulation %q: declares RequiredOutputs %v but has no Terraform body", fullID, s.RequiredOutputs))
	}
	declared, err := extractDeclaredOutputs(s.Terraform, fullID+".tf")
	if err != nil {
		panic(fmt.Sprintf("simulation %q: failed to parse embedded Terraform: %v", fullID, err))
	}
	declaredSet := make(map[string]struct{}, len(declared))
	for _, name := range declared {
		declaredSet[name] = struct{}{}
	}
	var missing []string
	for _, name := range s.RequiredOutputs {
		if _, ok := declaredSet[name]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		panic(fmt.Sprintf("simulation %q: missing terraform outputs %v (declared in HCL: %v)", fullID, missing, declared))
	}
}

// RegisterTemplate registers an injection template with the SDK.
// Call this from your template package's init() function.
// The ID should be a lean slug (e.g., "add-group-member"). The SDK combines
// it with Scope to form the template ID (scope.slug, e.g., "okta.add-group-member")
// used in the manifest.
func RegisterTemplate(t Template) {
	registerItem("template", t.ID, t.Scope, &t, templateRegistry)
}

// registerItem validates and registers an item in the given registry.
func registerItem[T any](itemType, id, scope string, item T, registry map[string]T) {
	if scope == "" {
		panic(fmt.Sprintf("%s %q: scope is required", itemType, id))
	}
	if strings.Contains(id, ".") {
		panic(fmt.Sprintf("%s %q: ID slug must not contain '.' — use hyphens instead", itemType, id))
	}
	fullID := scope + "." + id
	if _, exists := registry[fullID]; exists {
		panic(fmt.Sprintf("%s %q: duplicate registration for %s", itemType, id, fullID))
	}
	registry[fullID] = item
}

// GetSimulation returns a registered simulation by ID (scope.slug).
func GetSimulation(id string) (*Simulation, bool) {
	s, ok := registry[id]
	return s, ok
}

// ListSimulations returns the IDs of all registered simulations.
func ListSimulations() []string {
	ids := make([]string, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	return ids
}

// Run is the main entrypoint for a pack. Call this from main().
// It parses CLI arguments and dispatches to the appropriate handler.
func Run() {
	if packName == "" {
		fmt.Fprintln(os.Stderr, "Error: SetPackInfo() must be called before Run()")
		os.Exit(1)
	}

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: pack <command>")
		fmt.Fprintln(os.Stderr, "Commands: manifest, detonate, cleanup")
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cmd := os.Args[1]
	var err error

	switch cmd {
	case "manifest":
		err = handleManifest()
	case "detonate":
		err = handleDetonate(ctx)
	case "cleanup":
		err = handleCleanup(ctx)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func handleManifest() error {
	var input ManifestInput
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil && err != io.EOF {
		return fmt.Errorf("decode manifest input: %w", err)
	}

	defaultTags := extractDefaultTags(input.Parameters)
	resp := buildManifest(defaultTags)
	return json.NewEncoder(os.Stdout).Encode(resp)
}

// extractDefaultTags extracts the "default_tags" parameter as a string map.
// Returns nil if no default_tags parameter is present.
func extractDefaultTags(parameters map[string]any) map[string]string {
	raw, ok := parameters["default_tags"]
	if !ok {
		return nil
	}

	// The value may be map[string]any (from JSON unmarshaling) or map[string]string.
	switch v := raw.(type) {
	case map[string]any:
		tags := make(map[string]string, len(v))
		for k, val := range v {
			if s, ok := val.(string); ok {
				tags[k] = s
			}
		}
		return tags
	case map[string]string:
		return v
	default:
		return nil
	}
}

// encodeResult writes a Result as JSON to stdout.
func encodeResult(result *Result) error {
	return json.NewEncoder(os.Stdout).Encode(result)
}

func handleDetonate(ctx context.Context) error {
	var input DetonateInput
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		return fmt.Errorf("decode input: %w", err)
	}

	return dispatchCommand(ctx, input.Simulation, input.ExecutionID, func(ctx context.Context, s *Simulation) error {
		if s.Detonate == nil {
			return encodeResult(ErrorResult(ErrCodeInternalError, "simulation has no Detonate function"))
		}
		result, err := s.Detonate(ctx, input)
		if err != nil {
			result = ErrorResult(ErrCodeInternalError, err.Error())
		}
		return encodeResult(result)
	})
}

func handleCleanup(ctx context.Context) error {
	var input CleanupInput
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		return fmt.Errorf("decode input: %w", err)
	}

	return dispatchCommand(ctx, input.Simulation, input.ExecutionID, func(ctx context.Context, s *Simulation) error {
		if s.Cleanup == nil {
			return encodeResult(SuccessResult(nil))
		}
		if err := s.Cleanup(ctx, input); err != nil {
			return encodeResult(ErrorResult(ErrCodeInternalError, err.Error()))
		}
		return encodeResult(SuccessResult(nil))
	})
}

// dispatchCommand looks up a simulation and executes a handler function.
func dispatchCommand(ctx context.Context, simulationID, executionID string, handler func(context.Context, *Simulation) error) error {
	simulation, ok := registry[simulationID]
	if !ok {
		return encodeResult(ErrorResult(ErrCodeInvalidParams, fmt.Sprintf("unknown simulation: %s", simulationID)))
	}

	currentSimulation = simulationID
	ctx = WithExecutionID(ctx, executionID)
	return handler(ctx, simulation)
}
