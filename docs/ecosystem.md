# Ecosystem & Writing Packs

SimRun's simulations live outside the core binary, in **packs**. This keeps the platform stable while the catalogue of attack techniques grows independently — maintained by us, by the community, or by you for your own environment.

This page covers the maintained satellite projects and how to author a pack of your own. For installing and configuring packs from the UI, see [packs.md](packs.md).

## The satellite projects

### simrun-pack — the reference pack

[**confluentinc/simrun-pack**](https://github.com/confluentinc/simrun-pack) is the first-party `simrun-base-pack` and doubles as the canonical example of how a pack is written. It ships real simulations across AWS and Kubernetes, plus an Okta log injection, organised by MITRE tactic:

```
simulations/
  aws/
    credential-access/credential-scanner-tools/   main.tf + simulation.go
    discovery/s3-list-objects/                     main.tf + simulation.go
  k8s/
    credential-access/eks-web-identity-token-theft/
    privilege-escalation/create-clusterrolebinding/
  shared/                                          reusable Go helpers
injections/
  okta/api-token-create/                           injection.go + injection.tpl
main.go                                            registers everything
```

Each simulation is a directory with a Terraform module (`main.tf`) that stands up the infrastructure and a Go file (`simulation.go`) that registers the simulation and runs the detonation. Read it as a template for the patterns below.

### simrun-stratus-adapter — plug-and-play Stratus Red Team

[**confluentinc/simrun-stratus-adapter**](https://github.com/confluentinc/simrun-stratus-adapter) exposes the entire [Stratus Red Team](https://github.com/DataDog/stratus-red-team) attack-technique registry as a single SimRun pack. Rather than re-implementing techniques, it adapts each one — preserving its MITRE ATT&CK mapping — into the SimRun pack format. Its `main()` is essentially:

```go
func main() {
    pack.SetPackInfo("stratus", Version, "3.0.0")

    for _, technique := range stratus.GetRegistry().ListAttackTechniques() {
        pack.Register(adapter.AdaptTechnique(technique))
    }

    pack.Run()
}
```

Install it like any other pack to get Stratus's cloud attack coverage in SimRun with no per-technique work.

## Writing your own pack

A pack is a small Go program built against the [`github.com/IBM/simrun/pack`](https://github.com/IBM/simrun/tree/main/pack) SDK. At a high level you:

1. Register one or more simulations (and any log-injection templates).
2. Declare the pack's name, version, and the minimum SimRun version it needs.
3. Declare any pack-level parameters.
4. Call `pack.Run()`.

### The entrypoint

`main.go` wires the pack together. Importing each simulation package for its side effect (`_ "..."`) triggers the `init()` that registers it:

```go
package main

import (
    "github.com/IBM/simrun/pack"

    _ "github.com/your-org/your-pack/simulations/aws/discovery/s3-list-objects"
    // ...more simulations
)

// Version is set via ldflags at build time.
var Version = "dev"

func main() {
    pack.SetPackInfo("your-pack", Version, "0.4.0") // name, version, min SimRun version

    pack.RegisterPackParams(
        pack.PackParam{
            Name:        "resource_prefix",
            Type:        pack.PackParamTypeString,
            Description: "Prefix applied to every resource the pack creates.",
            Default:     "simrun",
        },
    )

    pack.Run()
}
```

Pack-level parameters appear on the Packs page once installed and are passed to every simulation's Terraform. The SDK always provides the built-in parameters (`default_tags`, `aws_region`, `gcp_region`, `azure_location`) — declare only the extras you need. See [packs.md](packs.md#pack-parameters) for precedence rules.

### A simulation

Each simulation embeds its Terraform module and registers itself in `init()`:

```go
package simulations

import (
    "context"
    _ "embed"

    "github.com/IBM/simrun/pack"
    packaws "github.com/IBM/simrun/pack/aws"
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/service/s3"
)

//go:embed main.tf
var terraform string

func init() {
    pack.Register(pack.Simulation{
        ID:          "s3-list-objects",
        Name:        "S3 Bucket Object Listing",
        Description: "Lists objects in an S3 bucket to simulate cloud storage discovery.",
        MITRE:       pack.MITREMapping{Tactics: []string{"TA0007"}, Techniques: []string{"T1619"}},
        Scope:       "aws",
        Terraform:   terraform,
        Detonate:    Detonate,
    })
}

// Detonate runs the attack after Terraform has applied the warm-up infrastructure.
func Detonate(ctx context.Context, input pack.DetonateInput) (*pack.Result, error) {
    log := pack.Logger(input)

    cfg, err := packaws.AWSConfig(ctx)
    if err != nil {
        return nil, err
    }

    bucketName := input.TerraformOutputs["bucket_name"]

    s3Client := s3.NewFromConfig(cfg)
    if _, err := s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
        Bucket: aws.String(bucketName),
    }); err != nil {
        return pack.ErrorResult(pack.ErrCodeInternalError, "failed to list objects: "+err.Error()), nil
    }

    return pack.SuccessResult(map[string]any{"bucket_name": bucketName}), nil
}
```

The flow at run time:

1. SimRun applies the simulation's `main.tf` — the **warm-up** that creates the infrastructure the attack needs.
2. Terraform outputs are handed to `Detonate` via `input.TerraformOutputs`.
3. `Detonate` performs the attack and returns a `Result`. Indicators returned in a `SuccessResult` are surfaced for log correlation; SimRun then tears the infrastructure back down.

### The SDK surface

| Function | Purpose |
|---|---|
| `pack.SetPackInfo(name, version, minSimrun)` | Identify the pack and the minimum SimRun version it requires. |
| `pack.RegisterPackParams(...PackParam)` | Declare pack-level parameters beyond the built-ins. |
| `pack.Register(Simulation)` | Register a simulation (call from the package's `init()`). |
| `pack.RegisterTemplate(Template)` | Register a log-injection template (for `inject` scenarios). |
| `pack.Run()` | Hand control to the SDK — must be the last call in `main()`. |
| `pack.SuccessResult(indicators)` / `pack.ErrorResult(code, msg)` | Build the `Result` a `Detonate` returns. |
| `pack.Logger(input)` | Structured logger whose output is captured in the run logs. |

Cloud credential helpers live in `pack/aws`, `pack/gcp`, and `pack/azure`; SSH execution helpers in `pack/ssh`. The credentials come from the connector named in the scenario's `targets` block.

### Building and distributing

Packs are compiled binaries. Both reference repos use [GoReleaser](https://goreleaser.com/) driven from CI to cut versioned releases, injecting `Version` via `ldflags`. To install a published pack, point the Packs page at its source.

While developing, you don't need to publish at all: `go build` your pack and install the binary directly through the Packs page's **Upload** tab. That tightens the author → install → test loop to seconds and lets you run packs that never leave your environment. See [packs.md](packs.md#installing-a-pack) for both methods.

## See also

- [packs.md](packs.md) — installing and configuring packs from the UI
- [scenarios.md](scenarios.md) — referencing pack simulations and templates in scenario YAML
- [concepts.md](concepts.md) — where packs sit in the overall model
