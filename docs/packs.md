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

A pack is a compiled binary. There are two ways to install one from the **Packs** page (`/packs`), shown as **Remote** and **Upload** tabs in the install dialog.

### Remote — pull a published pack

Use this for shipped or published packs.

1. Open the **Packs** page (`/packs`) and start a new install.
2. Choose **Remote**, then enter the pack name, its source (the release location), and optionally a version.
3. Click **Install**. SimRun fetches the pack, validates its manifest, and stores it in the database.

To move a remote pack to a newer version later, use the **Update** action on the same page.

### Upload — install a local build

Use this to install a pack binary you built yourself — ideal for **rapid development and testing of simulations**, or when publishing a pack isn't appropriate for your environment.

1. Build the pack binary locally (e.g. `go build`, or `mise run build` in the pack repo).
2. On the **Packs** page, start a new install and choose **Upload**.
3. Enter the pack name and select the compiled binary, then install. SimRun stores the binary, runs it to read its manifest, and lists its simulations — exactly as for a remote pack.

After installation — by either method — the pack appears in the list with its available simulations and declared parameters.

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

### Custom parameters

Pack authors declare additional parameters in the pack's `main()` function using `pack.RegisterPackParams(...pack.PackParam)`. These appear alongside the built-ins in the pack's manifest `params_schema` and are available for configuration on the Packs page once the pack is installed.

## Parameter precedence

When a simulation runs, Terraform variable values are resolved in this order (last write wins):

1. **Terraform variable default** — the `default` value declared inside the module's `variable` block.
2. **Pack-level value** — set via the Packs page; applies to every simulation in the pack.
3. **Per-simulation scenario value** — the `params` map in the scenario YAML for a specific simulation; overrides the pack-level value for that run only.

Map and array parameter values are JSON-encoded before being passed to Terraform as `TF_VAR_<key>`.

## See also

- [walkthrough.md](walkthrough.md) — run your first detection test end to end
- [scenarios.md](scenarios.md) — reference pack simulations in scenario YAML
