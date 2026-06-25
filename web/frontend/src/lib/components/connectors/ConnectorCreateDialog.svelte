<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import ElasticLogo from '$lib/components/ElasticLogo.svelte';
	import CloudIcon from '@lucide/svelte/icons/cloud';
	import TerminalIcon from '@lucide/svelte/icons/terminal';
	import { createConnector, testConnector } from '$lib/api/client';
	import { connectors } from '$lib/stores/connectors';
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
		onCreated
	}: {
		open: boolean;
		onCreated: () => void;
	} = $props();

	const cloudTypes = ['aws', 'gcp', 'azure', 'kubernetes', 'ssh'];

	let step = $state<'type' | 'config'>('type');
	let selectedType = $state<'elastic' | 'aws' | 'gcp' | 'azure' | 'kubernetes' | 'ssh'>('aws');

	// Shared shell state.
	let name = $state('');
	let description = $state('');
	let secretGroupId = $state('');
	let isDefault = $state(false);
	let enabledPlaceholder = $state(true); // not shown in create

	// Per-type form state.
	let awsFields = $state<AWSFormFields>(emptyAWSFields());
	let sshFields = $state<SSHFormFields>(emptySSHFields());
	let elasticFields = $state<ElasticFormFields>(emptyElasticFields());
	let k8sFields = $state<KubernetesFormFields>(emptyKubernetesFields());
	let gcpFields = $state<GCPFormFields>(emptyGCPFields());
	let azureFields = $state<AzureFormFields>(emptyAzureFields());

	// Imperative form ref (bind:this) for the active per-type form.
	type FormApi = {
		validate: () => boolean;
		buildConfig: () => Record<string, unknown>;
		canTest: () => boolean;
	};
	let formRef = $state<FormApi | null>(null);

	let saving = $state(false);
	let testing = $state(false);
	let testResult = $state<{ success: boolean; error?: string } | null>(null);
	let error = $state('');

	function resetForm() {
		step = 'type';
		selectedType = 'aws';
		name = '';
		description = '';
		secretGroupId = '';
		isDefault = false;
		awsFields = emptyAWSFields();
		sshFields = emptySSHFields();
		elasticFields = emptyElasticFields();
		k8sFields = emptyKubernetesFields();
		gcpFields = emptyGCPFields();
		azureFields = emptyAzureFields();
		testResult = null;
		error = '';
	}

	function selectType(t: typeof selectedType) {
		selectedType = t;
		step = 'config';
		testResult = null;
		error = '';
	}

	function isFormValid(): boolean {
		if (!name.trim()) return false;
		if (!formRef) return false;
		return formRef.validate();
	}

	async function handleTest() {
		if (!formRef) return;
		testing = true;
		testResult = null;
		error = '';
		try {
			const result = await testConnector(selectedType, secretGroupId || '', formRef.buildConfig());
			testResult = result;
			if (!result.success) error = result.error || 'Connection test failed';
		} catch (e) {
			testResult = { success: false, error: e instanceof Error ? e.message : 'Test failed' };
			error = testResult.error!;
		} finally {
			testing = false;
		}
	}

	async function handleCreate() {
		if (!isFormValid() || !formRef) {
			error = 'Please fill in all required fields';
			return;
		}
		saving = true;
		error = '';
		try {
			await createConnector(
				name.trim(),
				selectedType,
				description.trim(),
				secretGroupId || undefined,
				formRef.buildConfig(),
				isDefault
			);
			onCreated();
			open = false;
			resetForm();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to create connector';
		} finally {
			saving = false;
		}
	}

	function typeLabel(t: string): string {
		switch (t) {
			case 'elastic':
				return 'Elastic';
			case 'aws':
				return 'AWS';
			case 'gcp':
				return 'GCP';
			case 'azure':
				return 'Azure';
			case 'kubernetes':
				return 'Kubernetes';
			case 'ssh':
				return 'SSH';
			default:
				return t;
		}
	}

	function typeDescription(t: string): string {
		switch (t) {
			case 'elastic':
				return 'Alert matching, rule management, and log collection';
			case 'aws':
				return 'Cloud detonation target for AWS attack simulations';
			case 'gcp':
				return 'Cloud detonation target for GCP attack simulations';
			case 'azure':
				return 'Cloud detonation target for Azure attack simulations';
			case 'kubernetes':
				return 'Kubernetes cluster target for K8s attack simulations';
			case 'ssh':
				return 'Remote command detonation over SSH';
			default:
				return '';
		}
	}
</script>

<Dialog.Root
	bind:open
	onOpenChange={(o) => {
		if (!o) resetForm();
	}}
>
	<Dialog.Content class="sm:max-w-2xl">
		{#if step === 'type'}
			<Dialog.Header>
				<Dialog.Title>New Connector</Dialog.Title>
				<Dialog.Description>Choose the type of connector to set up.</Dialog.Description>
			</Dialog.Header>
			<div class="grid gap-4 sm:grid-cols-2 pt-2">
				<button
					type="button"
					class="flex flex-col items-center gap-3 rounded-lg border-2 p-6 text-left transition-colors hover:bg-muted/50 {selectedType ===
					'elastic'
						? 'border-primary bg-muted/30'
						: 'border-border'}"
					onclick={() => selectType('elastic')}
				>
					<ElasticLogo size={48} />
					<div class="text-center">
						<p class="text-sm font-medium">Elastic</p>
						<p class="text-xs text-muted-foreground mt-1">
							Alert matching, rule management, and log collection
						</p>
					</div>
				</button>
				<button
					type="button"
					class="flex flex-col items-center gap-3 rounded-lg border-2 p-6 text-left transition-colors hover:bg-muted/50 {selectedType ===
					'aws'
						? 'border-primary bg-muted/30'
						: 'border-border'}"
					onclick={() => selectType('aws')}
				>
					<CloudIcon size={48} class="text-attr-environment" />
					<div class="text-center">
						<p class="text-sm font-medium">AWS</p>
						<p class="text-xs text-muted-foreground mt-1">
							Cloud detonation target for AWS attack simulations
						</p>
					</div>
				</button>
				<button
					type="button"
					class="flex flex-col items-center gap-3 rounded-lg border-2 p-6 text-left transition-colors hover:bg-muted/50 {selectedType ===
					'gcp'
						? 'border-primary bg-muted/30'
						: 'border-border'}"
					onclick={() => selectType('gcp')}
				>
					<CloudIcon size={48} class="text-attr-identity" />
					<div class="text-center">
						<p class="text-sm font-medium">GCP</p>
						<p class="text-xs text-muted-foreground mt-1">
							Cloud detonation target for GCP attack simulations
						</p>
					</div>
				</button>
				<button
					type="button"
					class="flex flex-col items-center gap-3 rounded-lg border-2 p-6 text-left transition-colors hover:bg-muted/50 {selectedType ===
					'azure'
						? 'border-primary bg-muted/30'
						: 'border-border'}"
					onclick={() => selectType('azure')}
				>
					<CloudIcon size={48} class="text-status-info" />
					<div class="text-center">
						<p class="text-sm font-medium">Azure</p>
						<p class="text-xs text-muted-foreground mt-1">
							Cloud detonation target for Azure attack simulations
						</p>
					</div>
				</button>
				<button
					type="button"
					class="flex flex-col items-center gap-3 rounded-lg border-2 p-6 text-left transition-colors hover:bg-muted/50 {selectedType ===
					'kubernetes'
						? 'border-primary bg-muted/30'
						: 'border-border'}"
					onclick={() => selectType('kubernetes')}
				>
					<CloudIcon size={48} class="text-attr-destination" />
					<div class="text-center">
						<p class="text-sm font-medium">Kubernetes</p>
						<p class="text-xs text-muted-foreground mt-1">
							Kubernetes cluster target for K8s attack simulations
						</p>
					</div>
				</button>
				<!-- SSH connector hidden from picker: consumption path (pack helper + port support) not yet wired. Re-enable once SSHClientFromConnector lands. -->
			</div>
		{:else}
			<Dialog.Header>
				<Dialog.Title class="flex items-center gap-2">
					{#if selectedType === 'elastic'}
						<ElasticLogo size={24} />
						New Elastic Connector
					{:else if selectedType === 'ssh'}
						<TerminalIcon size={24} />
						New SSH Connector
					{:else}
						<CloudIcon size={24} />
						New {typeLabel(selectedType)} Connector
					{/if}
				</Dialog.Title>
				<Dialog.Description>{typeDescription(selectedType)}</Dialog.Description>
			</Dialog.Header>
			<ConnectorFormShell
				bind:name
				bind:description
				bind:isDefault
				bind:enabled={enabledPlaceholder}
				showIsDefault={cloudTypes.includes(selectedType)}
				showEnabled={false}
				namePlaceholder={`e.g., Production ${typeLabel(selectedType)}`}
			>
				{#if selectedType === 'elastic'}
					<ElasticConnectorForm
						bind:this={formRef}
						bind:fields={elasticFields}
						bind:secretGroupId
					/>
				{:else if selectedType === 'aws'}
					<AWSConnectorForm bind:this={formRef} bind:fields={awsFields} bind:secretGroupId />
				{:else if selectedType === 'gcp'}
					<GCPConnectorForm bind:this={formRef} bind:fields={gcpFields} bind:secretGroupId />
				{:else if selectedType === 'azure'}
					<AzureConnectorForm bind:this={formRef} bind:fields={azureFields} bind:secretGroupId />
				{:else if selectedType === 'kubernetes'}
					<KubernetesConnectorForm
						bind:this={formRef}
						bind:fields={k8sFields}
						existingConnectors={$connectors}
					/>
				{:else if selectedType === 'ssh'}
					<SSHConnectorForm bind:this={formRef} bind:fields={sshFields} bind:secretGroupId />
				{/if}

				{#if testResult}
					<div
						class="rounded-md border p-3 text-sm {testResult.success
							? 'border-status-success/40 bg-status-success/10 text-status-success'
							: 'border-status-error/40 bg-status-error/10 text-status-error'}"
					>
						{testResult.success ? 'Connection successful' : testResult.error}
					</div>
				{/if}
			</ConnectorFormShell>

			<div class="flex justify-between pt-4">
				<div class="flex gap-2">
					<Button variant="outline" onclick={() => (step = 'type')}>Back</Button>
					{#if formRef?.canTest?.()}
						<Button variant="outline" onclick={handleTest} disabled={testing}>
							{testing ? 'Testing...' : 'Test Connection'}
						</Button>
					{/if}
				</div>
				<div class="flex gap-2">
					<Button variant="outline" onclick={() => (open = false)}>Cancel</Button>
					<Button onclick={handleCreate} disabled={saving || !isFormValid()}>
						{saving ? 'Creating...' : 'Create'}
					</Button>
				</div>
			</div>
		{/if}
	</Dialog.Content>
</Dialog.Root>
