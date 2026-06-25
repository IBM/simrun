<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { updateConnector } from '$lib/api/client';
	import { connectors } from '$lib/stores/connectors';
	import type { Connector } from '$lib/types';
	import ConnectorFormShell from './ConnectorFormShell.svelte';
	import AWSConnectorForm, { emptyAWSFields, type AWSFormFields } from './AWSConnectorForm.svelte';
	import SSHConnectorForm, { emptySSHFields, type SSHFormFields } from './SSHConnectorForm.svelte';
	import ElasticConnectorForm, {
		emptyElasticFields,
		type ElasticFormFields
	} from './ElasticConnectorForm.svelte';
	import KubernetesConnectorForm, {
		emptyKubernetesFields,
		type KubernetesFormFields
	} from './KubernetesConnectorForm.svelte';
	import GCPConnectorForm, { emptyGCPFields, type GCPFormFields } from './GCPConnectorForm.svelte';
	import AzureConnectorForm, {
		emptyAzureFields,
		type AzureFormFields
	} from './AzureConnectorForm.svelte';

	let {
		open = $bindable(),
		connector,
		onUpdated
	}: {
		open: boolean;
		connector: Connector | null;
		onUpdated: () => void;
	} = $props();

	const cloudTypes = ['aws', 'gcp', 'azure', 'kubernetes', 'ssh'];

	// Shared shell state.
	let name = $state('');
	let description = $state('');
	let secretGroupId = $state('');
	let isDefault = $state(false);
	let enabled = $state(true);

	// Per-type form state.
	let awsFields = $state<AWSFormFields>(emptyAWSFields());
	let sshFields = $state<SSHFormFields>(emptySSHFields());
	let elasticFields = $state<ElasticFormFields>(emptyElasticFields());
	let k8sFields = $state<KubernetesFormFields>(emptyKubernetesFields());
	let gcpFields = $state<GCPFormFields>(emptyGCPFields());
	let azureFields = $state<AzureFormFields>(emptyAzureFields());

	type FormApi = {
		validate: () => boolean;
		buildConfig: () => Record<string, unknown>;
		populate: (c: Connector) => void;
	};
	let formRef = $state<FormApi | null>(null);

	let updating = $state(false);
	let error = $state('');

	// Re-populate when target connector changes.
	let populatedId = $state('');
	$effect(() => {
		if (!connector) return;
		if (connector.id === populatedId) return;
		populatedId = connector.id;
		name = connector.name;
		description = connector.description;
		secretGroupId = connector.secretGroupId || '';
		enabled = connector.enabled;
		isDefault = connector.isDefault;
		// Reset all per-type field state so previous edits don't bleed across types.
		awsFields = emptyAWSFields();
		sshFields = emptySSHFields();
		elasticFields = emptyElasticFields();
		k8sFields = emptyKubernetesFields();
		gcpFields = emptyGCPFields();
		azureFields = emptyAzureFields();
		error = '';
	});

	// Once the matching form is mounted (formRef populated), populate from the connector.
	let populatedFormFor = $state('');
	$effect(() => {
		if (!connector || !formRef) return;
		const key = `${connector.id}:${connector.type}`;
		if (populatedFormFor === key) return;
		populatedFormFor = key;
		formRef.populate(connector);
	});

	function isFormValid(): boolean {
		if (!connector || !name.trim() || !formRef) return false;
		return formRef.validate();
	}

	async function handleUpdate() {
		if (!connector || !isFormValid() || !formRef) {
			error = 'Please fill in all required fields';
			return;
		}
		updating = true;
		error = '';
		try {
			await updateConnector(
				connector.id,
				name.trim(),
				description.trim(),
				secretGroupId || undefined,
				formRef.buildConfig(),
				enabled,
				isDefault
			);
			onUpdated();
			open = false;
			populatedId = '';
			populatedFormFor = '';
		} catch (e) {
			error = e instanceof Error ? e.message : 'Update failed';
		} finally {
			updating = false;
		}
	}
</script>

<Dialog.Root
	bind:open
	onOpenChange={(o) => {
		if (!o) {
			populatedId = '';
			populatedFormFor = '';
		}
	}}
>
	<Dialog.Content class="sm:max-w-2xl">
		<Dialog.Header>
			<Dialog.Title>Edit Connector</Dialog.Title>
			<Dialog.Description>Update connector settings and credentials.</Dialog.Description>
		</Dialog.Header>
		{#if connector}
			<ConnectorFormShell
				bind:name
				bind:description
				bind:isDefault
				bind:enabled
				showIsDefault={cloudTypes.includes(connector.type)}
				showEnabled={true}
				idPrefix="edit"
			>
				{#if connector.type === 'elastic'}
					<ElasticConnectorForm
						bind:this={formRef}
						bind:fields={elasticFields}
						bind:secretGroupId
						idPrefix="edit"
					/>
				{:else if connector.type === 'aws'}
					<AWSConnectorForm
						bind:this={formRef}
						bind:fields={awsFields}
						bind:secretGroupId
						idPrefix="edit"
					/>
				{:else if connector.type === 'gcp'}
					<GCPConnectorForm
						bind:this={formRef}
						bind:fields={gcpFields}
						bind:secretGroupId
						idPrefix="edit"
					/>
				{:else if connector.type === 'azure'}
					<AzureConnectorForm
						bind:this={formRef}
						bind:fields={azureFields}
						bind:secretGroupId
						idPrefix="edit"
					/>
				{:else if connector.type === 'kubernetes'}
					<KubernetesConnectorForm
						bind:this={formRef}
						bind:fields={k8sFields}
						existingConnectors={$connectors}
						idPrefix="edit"
					/>
				{:else if connector.type === 'ssh'}
					<SSHConnectorForm
						bind:this={formRef}
						bind:fields={sshFields}
						bind:secretGroupId
						idPrefix="edit"
					/>
				{/if}
			</ConnectorFormShell>

			{#if error}
				<p class="text-sm text-destructive">{error}</p>
			{/if}
		{/if}
		<div class="flex justify-end gap-2 pt-4">
			<Button variant="outline" onclick={() => (open = false)}>Cancel</Button>
			<Button onclick={handleUpdate} disabled={updating || !isFormValid()}>
				{updating ? 'Updating...' : 'Update'}
			</Button>
		</div>
	</Dialog.Content>
</Dialog.Root>
