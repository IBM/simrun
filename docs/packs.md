# Packs

Install simulations and tune their parameters.

## What a pack is

A **pack** is an external, versioned bundle of attack simulations. Each pack ships Terraform modules, scenario definitions, and a manifest that describes the simulations it contains and the parameters it accepts.

Packs are installed and managed from the **Packs page** (`/packs`). Once a pack is installed, its simulations are available for use in scenario YAML files via the `simrunDetonator.pack` and `simrunDetonator.simulation` fields.

## Shipped packs

Two first-party packs are available:

| Pack | Contents |
|---|---|
| `simrun-base-pack` | Custom simulations targeting AWS, Azure, and GCP. |
| `simrun-stratus-pack` | Wraps [Stratus Red Team](https://github.com/DataDog/stratus-red-team) attack techniques. |

## Installing a pack

1. Open the **Packs** page (`/packs`).
2. Enter the pack name and the source URL or path.
3. Click **Install**. SimRun downloads the pack, validates its manifest, and stores it in the database.
4. After installation the pack appears in the list with its available simulations and declared parameters.

To update a pack to a newer version, use the **Update** action on the same page.

## Pack parameters

Parameters let you tune pack-wide settings — cloud regions, tag values, project identifiers — without editing individual scenario files.

### Built-in parameters

Every pack automatically includes these built-in parameters (defined in the SimRun SDK):

| Parameter | Description |
|---|---|
| `default_tags` | Map of tags applied to every resource created by the pack (e.g. `{"env": "security-test"}`). |
| `aws_region` | AWS region used as the default for all simulations in the pack. |
| `gcp_region` | GCP region used as the default for all simulations in the pack. |
| `azure_location` | Azure location used as the default for all simulations in the pack. |

`gcp_project` is **not** a built-in parameter. Because the GCP project is organisation-specific, pack authors who need it must declare it explicitly (see Custom parameters below).

### Custom parameters

Pack authors declare additional parameters in the pack's `main()` function using `pack.RegisterPackParams(...pack.PackParam)`. These appear alongside the built-ins in the pack's manifest `params_schema` and are available for configuration on the Packs page once the pack is installed.

## Parameter precedence

When a simulation runs, Terraform variable values are resolved in this order (last write wins):

1. **Terraform variable default** — the `default` value declared inside the module's `variable` block.
2. **Pack-level value** — set via the Packs page or `PUT /api/packs/{name}/parameters`; applies to every simulation in the pack.
3. **Per-simulation scenario value** — the `params` map in the scenario YAML for a specific simulation; overrides the pack-level value for that run only.

Map and array parameter values are JSON-encoded before being passed to Terraform as `TF_VAR_<key>`.

## Validation

`PUT /api/packs/{name}/parameters` validates submitted values against the pack's `params_schema`:

- **Declared keys** are strict-validated for type, enum membership, and required constraints.
- **Unknown keys** (keys not declared in the schema) are kept and returned in the response's `unknown_keys` field as a soft warning. They are stored but will not be passed to Terraform unless the pack declares them in a future version.

If the manifest cannot be fetched or the schema is empty, validation falls back to permissive storage (all keys accepted without type checking).

## See also

- [walkthrough.md](walkthrough.md) — run your first detection test end to end
- [scenarios.md](scenarios.md) — reference pack simulations in scenario YAML
