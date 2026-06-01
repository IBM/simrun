<script lang="ts" module>
	export interface ElasticFormFields {
		kibanaUrl: string;
		cloudId: string;
		elasticsearchUrl: string;
		exportEnabled: boolean;
		exportDatastream: string;
	}
	export function emptyElasticFields(): ElasticFormFields {
		return {
			kibanaUrl: '',
			cloudId: '',
			elasticsearchUrl: '',
			exportEnabled: false,
			exportDatastream: 'asp.results'
		};
	}
</script>

<script lang="ts">
	import * as Select from '$lib/components/ui/select/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import { secrets } from '$lib/stores/secrets';
	import type { Connector, ElasticConnectorConfig } from '$lib/types';

	let {
		fields = $bindable(),
		secretGroupId = $bindable(),
		idPrefix = ''
	}: {
		fields: ElasticFormFields;
		secretGroupId: string;
		idPrefix?: string;
	} = $props();

	export function validate(): boolean {
		if (fields.exportEnabled && !fields.cloudId.trim()) return false;
		return !!fields.kibanaUrl.trim() && !!secretGroupId;
	}

	export function buildConfig(): Record<string, unknown> {
		const cfg: Record<string, unknown> = { kibana_url: fields.kibanaUrl.trim() };
		if (fields.cloudId.trim()) cfg.cloud_id = fields.cloudId.trim();
		if (fields.elasticsearchUrl.trim()) cfg.elasticsearch_url = fields.elasticsearchUrl.trim();
		cfg.export_enabled = fields.exportEnabled;
		if (fields.exportEnabled) {
			cfg.export_datastream = fields.exportDatastream.trim() || 'asp.results';
		}
		return cfg;
	}

	export function populate(connector: Connector): void {
		const cfg = connector.config as unknown as ElasticConnectorConfig;
		fields.kibanaUrl = cfg.kibana_url || '';
		fields.cloudId = cfg.cloud_id || '';
		fields.elasticsearchUrl = cfg.elasticsearch_url || '';
		fields.exportEnabled = cfg.export_enabled || false;
		fields.exportDatastream = cfg.export_datastream || 'asp.results';
	}

	export function canTest(): boolean {
		return !!fields.kibanaUrl.trim() && !!secretGroupId;
	}

	const secretGroupLabel = $derived(
		secretGroupId
			? ($secrets.find((s) => s.id === secretGroupId)?.name ?? 'Unknown')
			: 'Select secret group'
	);

	const kibanaId = $derived(idPrefix ? `${idPrefix}KibanaUrl` : 'kibanaUrl');
	const cloudIdId = $derived(idPrefix ? `${idPrefix}CloudId` : 'cloudId');
	const esUrlId = $derived(idPrefix ? `${idPrefix}ElasticsearchUrl` : 'elasticsearchUrl');
	const exportEnabledId = $derived(idPrefix ? `${idPrefix}ExportEnabled` : 'exportEnabled');
	const datastreamId = $derived(idPrefix ? `${idPrefix}ExportDatastream` : 'exportDatastream');
</script>

<div class="space-y-2">
	<Label>Secret Group</Label>
	<Select.Root type="single" bind:value={secretGroupId}>
		<Select.Trigger class="w-full">{secretGroupLabel}</Select.Trigger>
		<Select.Content>
			{#each $secrets as secret}
				<Select.Item value={secret.id} label={secret.name} />
			{/each}
		</Select.Content>
	</Select.Root>
	<p class="text-xs text-muted-foreground">
		Select a secret group containing SR_ELASTIC_API_KEY
	</p>
</div>
<div class="space-y-2">
	<Label for={kibanaId}>Kibana URL</Label>
	<Input
		id={kibanaId}
		placeholder="https://your-deployment.kb.us-central1.gcp.cloud.es.io"
		bind:value={fields.kibanaUrl}
	/>
</div>
<div class="space-y-2">
	<Label for={cloudIdId}>Cloud ID {fields.exportEnabled ? '' : '(optional)'}</Label>
	<Input id={cloudIdId} placeholder="deployment-name:..." bind:value={fields.cloudId} />
	{#if fields.exportEnabled && !fields.cloudId.trim()}
		<p class="text-xs text-destructive">Cloud ID is required when result export is enabled</p>
	{/if}
</div>
<div class="space-y-2">
	<Label for={esUrlId}>Elasticsearch URL (optional)</Label>
	<Input
		id={esUrlId}
		placeholder="https://your-deployment.es.us-central1.gcp.cloud.es.io"
		bind:value={fields.elasticsearchUrl}
	/>
</div>
<div class="space-y-3 border-t pt-4">
	<div class="flex items-center justify-between">
		<div class="space-y-0.5">
			<Label for={exportEnabledId}>Export Results to Elasticsearch</Label>
			<p class="text-xs text-muted-foreground">
				Automatically index scenario results after each run
			</p>
		</div>
		<Switch id={exportEnabledId} bind:checked={fields.exportEnabled} />
	</div>
	{#if fields.exportEnabled}
		<div class="space-y-2">
			<Label for={datastreamId}>Datastream</Label>
			<Input
				id={datastreamId}
				placeholder="asp.results"
				bind:value={fields.exportDatastream}
			/>
			<p class="text-xs text-muted-foreground">
				Results will be indexed to:
				<span class="font-mono">logs-{fields.exportDatastream || 'asp.results'}-default</span>
			</p>
		</div>
	{/if}
</div>
