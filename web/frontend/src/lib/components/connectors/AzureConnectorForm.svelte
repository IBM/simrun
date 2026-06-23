<script lang="ts" module>
	export interface AzureFormFields {
		authType: 'wif' | 'service_principal';
		tenantId: string;
		subscriptionId: string;
		clientId: string;
		tokenFile: string;
	}
	export function emptyAzureFields(): AzureFormFields {
		return {
			authType: 'wif',
			tenantId: '',
			subscriptionId: '',
			clientId: '',
			tokenFile: ''
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
		fields: AzureFormFields;
		secretGroupId: string;
		idPrefix?: string;
	} = $props();

	export function validate(): boolean {
		if (!fields.tenantId.trim() || !fields.subscriptionId.trim() || !fields.clientId.trim()) {
			return false;
		}
		if (fields.authType === 'service_principal') return !!secretGroupId;
		return true;
	}

	export function buildConfig(): Record<string, unknown> {
		const cfg: Record<string, unknown> = {
			tenant_id: fields.tenantId.trim(),
			subscription_id: fields.subscriptionId.trim(),
			client_id: fields.clientId.trim()
		};
		if (fields.authType === 'wif') {
			cfg.auth_type = 'workload_identity_federation';
			if (fields.tokenFile.trim()) cfg.token_file = fields.tokenFile.trim();
		}
		return cfg;
	}

	export function populate(connector: Connector): void {
		const cfg = connector.config as Record<string, unknown>;
		fields.tenantId = (cfg.tenant_id as string) || '';
		fields.subscriptionId = (cfg.subscription_id as string) || '';
		fields.clientId = (cfg.client_id as string) || '';
		if (cfg.auth_type === 'workload_identity_federation') {
			fields.authType = 'wif';
			fields.tokenFile = (cfg.token_file as string) || '';
		} else {
			fields.authType = 'service_principal';
		}
	}

	export function canTest(): boolean {
		if (fields.authType !== 'wif') return false;
		return !!fields.tenantId.trim() && !!fields.subscriptionId.trim() && !!fields.clientId.trim();
	}

	function setAuthType(v: string) {
		const next = v === 'wif' ? 'wif' : 'service_principal';
		fields.authType = next;
		if (next === 'wif') secretGroupId = '';
	}

	const secretGroupLabel = $derived(
		secretGroupId
			? ($secrets.find((s) => s.id === secretGroupId)?.name ?? 'Unknown')
			: 'Select secret group'
	);

	const tenantInputId = $derived(idPrefix ? `${idPrefix}TenantId` : 'tenantId');
	const subscriptionInputId = $derived(idPrefix ? `${idPrefix}SubscriptionId` : 'subscriptionId');
	const clientInputId = $derived(idPrefix ? `${idPrefix}ClientId` : 'clientId');
	const tokenFileId = $derived(idPrefix ? `${idPrefix}AzureTokenFile` : 'azureTokenFile');
</script>

<div class="space-y-2">
	<Label for={tenantInputId}>Tenant ID</Label>
	<Input
		id={tenantInputId}
		placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
		bind:value={fields.tenantId}
	/>
</div>
<div class="space-y-2">
	<Label for={subscriptionInputId}>Subscription ID</Label>
	<Input
		id={subscriptionInputId}
		placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
		bind:value={fields.subscriptionId}
	/>
</div>
<div class="space-y-2">
	<Label for={clientInputId}>Client ID</Label>
	<Input
		id={clientInputId}
		placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
		bind:value={fields.clientId}
	/>
</div>
<div class="space-y-2">
	<Label>Authentication Method</Label>
	<Tabs.Root value={fields.authType} onValueChange={setAuthType}>
		<Tabs.List class="w-full">
			<Tabs.Trigger value="wif" class="flex-1">Workload Identity Federation</Tabs.Trigger>
			<Tabs.Trigger value="service_principal" class="flex-1">Service Principal</Tabs.Trigger>
		</Tabs.List>
	</Tabs.Root>
</div>

{#if fields.authType === 'wif'}
	<div class="space-y-2">
		<Label for={tokenFileId}>OIDC Token File</Label>
		<Input
			id={tokenFileId}
			placeholder="/var/run/secrets/eks.amazonaws.com/serviceaccount/token"
			bind:value={fields.tokenFile}
		/>
		<p class="text-xs text-muted-foreground">
			Path to the OIDC token file. Leave empty to use the EKS default.
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
			Select a secret group containing ARM_CLIENT_SECRET (only for Service Principal auth)
		</p>
	</div>
{/if}
