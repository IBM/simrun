<script lang="ts" module>
	export interface GCPFormFields {
		authType: 'wif' | 'service_account';
		projectId: string;
		projectNumber: string;
		poolId: string;
		providerId: string;
		serviceAccountEmail: string;
	}
	export function emptyGCPFields(): GCPFormFields {
		return {
			authType: 'wif',
			projectId: '',
			projectNumber: '',
			poolId: '',
			providerId: '',
			serviceAccountEmail: ''
		};
	}
</script>

<script lang="ts">
	import * as Select from '$lib/components/ui/select/index.js';
	import * as Tabs from '$lib/components/ui/tabs/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { secrets } from '$lib/stores/secrets';
	import type { Connector } from '$lib/types';

	let {
		fields = $bindable(),
		secretGroupId = $bindable(),
		idPrefix = ''
	}: {
		fields: GCPFormFields;
		secretGroupId: string;
		idPrefix?: string;
	} = $props();

	export function validate(): boolean {
		if (fields.authType === 'wif') {
			return (
				!!fields.projectNumber.trim() &&
				!!fields.poolId.trim() &&
				!!fields.providerId.trim() &&
				!!fields.serviceAccountEmail.trim()
			);
		}
		return !!secretGroupId;
	}

	export function buildConfig(): Record<string, unknown> {
		const cfg: Record<string, unknown> = {};
		if (fields.projectId.trim()) cfg.project_id = fields.projectId.trim();
		if (fields.authType === 'wif') {
			cfg.auth_type = 'workload_identity_federation';
			cfg.project_number = fields.projectNumber.trim();
			cfg.pool_id = fields.poolId.trim();
			cfg.provider_id = fields.providerId.trim();
			cfg.service_account_email = fields.serviceAccountEmail.trim();
		}
		return cfg;
	}

	export function populate(connector: Connector): void {
		const cfg = connector.config as Record<string, unknown>;
		fields.projectId = (cfg.project_id as string) || '';
		if (cfg.auth_type === 'workload_identity_federation') {
			fields.authType = 'wif';
			fields.projectNumber = (cfg.project_number as string) || '';
			fields.poolId = (cfg.pool_id as string) || '';
			fields.providerId = (cfg.provider_id as string) || '';
			fields.serviceAccountEmail = (cfg.service_account_email as string) || '';
		} else {
			fields.authType = 'service_account';
		}
	}

	export function canTest(): boolean {
		if (fields.authType !== 'wif') return false;
		return (
			!!fields.projectNumber.trim() &&
			!!fields.poolId.trim() &&
			!!fields.providerId.trim() &&
			!!fields.serviceAccountEmail.trim()
		);
	}

	const secretGroupLabel = $derived(
		secretGroupId
			? ($secrets.find((s) => s.id === secretGroupId)?.name ?? 'Unknown')
			: 'Select secret group'
	);

	const projIdId = $derived(idPrefix ? `${idPrefix}GcpProjectId` : 'gcpProjectId');
	const projNumId = $derived(idPrefix ? `${idPrefix}GcpProjectNumber` : 'gcpProjectNumber');
	const poolId = $derived(idPrefix ? `${idPrefix}GcpPoolId` : 'gcpPoolId');
	const provId = $derived(idPrefix ? `${idPrefix}GcpProviderId` : 'gcpProviderId');
	const saId = $derived(idPrefix ? `${idPrefix}GcpServiceAccountEmail` : 'gcpServiceAccountEmail');
</script>

<div class="space-y-2">
	<Label for={projIdId}>Project ID</Label>
	<Input id={projIdId} placeholder="my-gcp-project" bind:value={fields.projectId} />
	<p class="text-xs text-muted-foreground">
		GCP project ID (injected as GOOGLE_CLOUD_PROJECT)
	</p>
</div>
<div class="space-y-2">
	<Label>Authentication Method</Label>
	<Tabs.Root bind:value={fields.authType}>
		<Tabs.List class="w-full">
			<Tabs.Trigger value="wif" class="flex-1">Workload Identity Federation</Tabs.Trigger>
			<Tabs.Trigger value="service_account" class="flex-1">Service Account</Tabs.Trigger>
		</Tabs.List>
	</Tabs.Root>
</div>

{#if fields.authType === 'wif'}
	<div class="space-y-2">
		<Label for={projNumId}>Project Number</Label>
		<Input id={projNumId} placeholder="123456789012" bind:value={fields.projectNumber} />
		<p class="text-xs text-muted-foreground">The numeric GCP project number (not project ID)</p>
	</div>
	<div class="space-y-2">
		<Label for={poolId}>Workload Identity Pool ID</Label>
		<Input id={poolId} placeholder="simrun-pool" bind:value={fields.poolId} />
	</div>
	<div class="space-y-2">
		<Label for={provId}>Provider ID</Label>
		<Input id={provId} placeholder="aws-provider" bind:value={fields.providerId} />
	</div>
	<div class="space-y-2">
		<Label for={saId}>Service Account Email</Label>
		<Input
			id={saId}
			placeholder="simrun@project-id.iam.gserviceaccount.com"
			bind:value={fields.serviceAccountEmail}
		/>
		<p class="text-xs text-muted-foreground">
			The GCP service account to impersonate via WIF
		</p>
	</div>
{:else}
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
			Select a secret group containing SR_GCP_CREDENTIALS (only for Service Account auth)
		</p>
	</div>
{/if}
