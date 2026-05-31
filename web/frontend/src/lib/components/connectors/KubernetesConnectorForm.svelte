<script lang="ts" module>
	export interface KubernetesFormFields {
		clusterName: string;
		region: string;
		cloudConnector: string;
		resourceGroup: string;
		project: string;
	}
	export function emptyKubernetesFields(): KubernetesFormFields {
		return {
			clusterName: '',
			region: '',
			cloudConnector: '',
			resourceGroup: '',
			project: ''
		};
	}
</script>

<script lang="ts">
	import * as Select from '$lib/components/ui/select/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import type { Connector } from '$lib/types';

	let {
		fields = $bindable(),
		existingConnectors,
		idPrefix = ''
	}: {
		fields: KubernetesFormFields;
		existingConnectors: Connector[];
		idPrefix?: string;
	} = $props();

	const cloudConnectorType = $derived(
		existingConnectors.find((c) => c.name === fields.cloudConnector)?.type ?? ''
	);

	export function validate(): boolean {
		if (!fields.clusterName.trim() || !fields.region.trim() || !fields.cloudConnector.trim()) {
			return false;
		}
		const cloudConn = existingConnectors.find((c) => c.name === fields.cloudConnector);
		if (cloudConn?.type === 'azure' && !fields.resourceGroup.trim()) return false;
		return true;
	}

	export function buildConfig(): Record<string, unknown> {
		const cfg: Record<string, unknown> = {
			cluster_name: fields.clusterName.trim(),
			region: fields.region.trim(),
			cloud_connector: fields.cloudConnector.trim()
		};
		if (fields.resourceGroup.trim()) cfg.resource_group = fields.resourceGroup.trim();
		if (fields.project.trim()) cfg.project = fields.project.trim();
		return cfg;
	}

	export function populate(connector: Connector): void {
		const cfg = connector.config as Record<string, unknown>;
		fields.clusterName = (cfg.cluster_name as string) || '';
		fields.region = (cfg.region as string) || '';
		fields.cloudConnector = (cfg.cloud_connector as string) || '';
		fields.resourceGroup = (cfg.resource_group as string) || '';
		fields.project = (cfg.project as string) || '';
	}

	export function canTest(): boolean {
		return !!fields.clusterName.trim() && !!fields.region.trim() && !!fields.cloudConnector.trim();
	}

	const clusterId = $derived(idPrefix ? `${idPrefix}ClusterName` : 'clusterName');
	const regionId = $derived(idPrefix ? `${idPrefix}Region` : 'region');
	const rgId = $derived(idPrefix ? `${idPrefix}ResourceGroup` : 'resourceGroup');
	const projId = $derived(idPrefix ? `${idPrefix}Project` : 'project');
</script>

<div class="space-y-2">
	<Label>Cloud Connector</Label>
	<Select.Root type="single" bind:value={fields.cloudConnector}>
		<Select.Trigger class="w-full">
			{fields.cloudConnector || 'Select cloud connector'}
		</Select.Trigger>
		<Select.Content>
			{#each existingConnectors.filter((c) => ['aws', 'gcp', 'azure'].includes(c.type) && c.enabled) as cloudConn}
				<Select.Item
					value={cloudConn.name}
					label="{cloudConn.name} ({cloudConn.type.toUpperCase()})"
				/>
			{/each}
		</Select.Content>
	</Select.Root>
	<p class="text-xs text-muted-foreground">
		AWS, GCP, or Azure connector that provides cloud credentials
	</p>
</div>
<div class="space-y-2">
	<Label for={clusterId}>Cluster Name</Label>
	<Input id={clusterId} placeholder="my-cluster" bind:value={fields.clusterName} />
</div>
<div class="space-y-2">
	<Label for={regionId}>Region</Label>
	<Input id={regionId} placeholder="us-east-1" bind:value={fields.region} />
</div>
{#if cloudConnectorType === 'azure'}
	<div class="space-y-2">
		<Label for={rgId}>Resource Group</Label>
		<Input id={rgId} placeholder="my-resource-group" bind:value={fields.resourceGroup} />
	</div>
{/if}
{#if cloudConnectorType === 'gcp'}
	<div class="space-y-2">
		<Label for={projId}>Project ID</Label>
		<Input id={projId} placeholder="my-gcp-project" bind:value={fields.project} />
		<p class="text-xs text-muted-foreground">
			Optional: defaults to GCP connector's project_id if not set
		</p>
	</div>
{/if}
