<script lang="ts">
	import { onMount } from 'svelte';
	import * as Alert from '$lib/components/ui/alert/index.js';
	import * as Empty from '$lib/components/ui/empty/index.js';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Skeleton } from '$lib/components/ui/skeleton/index.js';
	import MoreVerticalIcon from '@lucide/svelte/icons/more-vertical';
	import PencilIcon from '@lucide/svelte/icons/pencil';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import ArrowRightIcon from '@lucide/svelte/icons/arrow-right';
	import { connectors, loadConnectors } from '$lib/stores/connectors';
	import { secrets, loadSecrets } from '$lib/stores/secrets';
	import type { Connector, ElasticConnectorConfig } from '$lib/types';
	import ElasticLogo from '$lib/components/ElasticLogo.svelte';
	import ConnectorDetail from '$lib/components/connectors/ConnectorDetail.svelte';
	import ConnectorCreateDialog from '$lib/components/connectors/ConnectorCreateDialog.svelte';
	import ConnectorEditDialog from '$lib/components/connectors/ConnectorEditDialog.svelte';
	import ConnectorDeleteDialog from '$lib/components/connectors/ConnectorDeleteDialog.svelte';
	import PlugIcon from '@lucide/svelte/icons/plug';
	import CloudIcon from '@lucide/svelte/icons/cloud';
	import TerminalIcon from '@lucide/svelte/icons/terminal';

	let loading = $state(true);
	let error = $state('');

	let createDialogOpen = $state(false);
	let editDialogOpen = $state(false);
	let deleteDialogOpen = $state(false);
	let editTarget = $state<Connector | null>(null);
	let deleteTarget = $state<Connector | null>(null);
	let selectedConnector = $state<Connector | null>(null);

	onMount(async () => {
		try {
			await Promise.all([loadConnectors(), loadSecrets()]);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load data';
		} finally {
			loading = false;
		}
	});

	function openDetail(c: Connector) {
		selectedConnector = c;
	}

	function handleCardKeydown(e: KeyboardEvent, c: Connector) {
		if (e.key === 'Enter' || e.key === ' ') {
			e.preventDefault();
			openDetail(c);
		}
	}

	function closeDetail() {
		selectedConnector = null;
	}

	function openEdit(c: Connector) {
		editTarget = c;
		editDialogOpen = true;
	}

	function openDelete(c: Connector) {
		deleteTarget = c;
		deleteDialogOpen = true;
	}

	const connectorFeatures: Record<string, string[]> = {
		elastic: ['Alert Matching', 'Rule Management', 'Log Collection'],
		aws: ['Cloud Target'],
		gcp: ['Cloud Target'],
		azure: ['Cloud Target'],
		kubernetes: ['K8s Target'],
		ssh: ['Remote Command Detonation']
	};

	function getSecretGroupName(id: string | undefined): string {
		if (!id) return 'None';
		return $secrets.find((s) => s.id === id)?.name ?? 'Unknown';
	}

	function getConnectorCardInfo(c: Connector): string {
		const cfg = c.config as Record<string, unknown>;
		if (c.type === 'elastic') {
			return (c.config as unknown as ElasticConnectorConfig).kibana_url || '';
		} else if (c.type === 'aws') {
			return (cfg.role_arn as string) || '';
		} else if (c.type === 'gcp') {
			if (cfg.auth_type === 'workload_identity_federation') {
				return (cfg.service_account_email as string) || 'WIF';
			}
			return 'Service Account';
		} else if (c.type === 'azure') {
			if (cfg.auth_type === 'workload_identity_federation') {
				return (cfg.client_id as string) || 'WIF';
			}
			const tenantId = (cfg.tenant_id as string) || '';
			const subId = (cfg.subscription_id as string) || '';
			if (tenantId && subId) return `${tenantId} / ${subId}`;
			return tenantId || subId || '';
		} else if (c.type === 'kubernetes') {
			const cluster = (cfg.cluster_name as string) || '';
			const region = (cfg.region as string) || '';
			if (cluster && region) return `${cluster} (${region})`;
			return cluster || '';
		} else if (c.type === 'ssh') {
			const host = (cfg.host as string) || '';
			const username = (cfg.username as string) || '';
			const port = cfg.port as number | undefined;
			if (username && host) return port ? `${username}@${host}:${port}` : `${username}@${host}`;
			return host || '';
		}
		return '';
	}
</script>


<div class="space-y-6">
	<div class="flex items-center justify-between">
			<h1 class="text-2xl font-bold">Connectors</h1>
			<Button onclick={() => (createDialogOpen = true)}>New Connector</Button>
		</div>

		{#if error}
			<Alert.Root variant="destructive">
				<Alert.Description>{error}</Alert.Description>
			</Alert.Root>
		{/if}

		{#if loading}
			<div class="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
				{#each Array(3) as _}
					<Skeleton class="h-48 w-full rounded-xl" />
				{/each}
			</div>
		{:else if $connectors.length === 0}
			<Empty.Root>
				<Empty.Header>
					<Empty.Media variant="icon">
						<PlugIcon />
					</Empty.Media>
					<Empty.Title>No connectors configured</Empty.Title>
					<Empty.Description>
						Connect to external systems like Elastic Security or cloud providers for attack
						simulation and alert matching.
					</Empty.Description>
				</Empty.Header>
				<Empty.Content>
					<Button onclick={() => (createDialogOpen = true)}>New Connector</Button>
				</Empty.Content>
			</Empty.Root>
		{:else}
			<div class="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
				{#each $connectors as connector, i (connector.id)}
					<div
						role="button"
						tabindex="0"
						aria-label="View {connector.name} details"
						onclick={() => openDetail(connector)}
						onkeydown={(e) => handleCardKeydown(e, connector)}
						class="group relative flex h-full cursor-pointer flex-col overflow-hidden rounded-xl border bg-card p-5 text-left transition-all duration-200 animate-fade-up
							hover:-translate-y-0.5 hover:shadow-sm
							focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
						style="animation-delay: {i * 60}ms"
					>
						<!-- accent line on the left edge, draws down on hover -->
						<span
							class="absolute inset-y-0 left-0 w-[3px] origin-top scale-y-0 bg-primary transition-transform duration-300 ease-out group-hover:scale-y-100"
						></span>

						<!-- header -->
						<div class="flex items-start gap-3">
							<div class="flex size-10 shrink-0 items-center justify-center rounded-md bg-muted">
								{#if connector.type === 'elastic'}
									<ElasticLogo size={28} />
								{:else if connector.type === 'ssh'}
									<TerminalIcon size={20} class="text-muted-foreground" />
								{:else}
									<CloudIcon size={20} class="text-muted-foreground" />
								{/if}
							</div>
							<div class="min-w-0 flex-1">
								<h3 class="truncate text-base font-semibold tracking-tight">{connector.name}</h3>
								<p class="mt-0.5 text-xs uppercase tracking-wide text-muted-foreground">
									{connector.type}{#if connector.isDefault}<span class="normal-case"> · Default</span>{/if}
								</p>
							</div>
							<Badge variant={connector.enabled ? 'default' : 'secondary'} class="shrink-0">
								{connector.enabled ? 'Enabled' : 'Disabled'}
							</Badge>
							<DropdownMenu.Root>
								<DropdownMenu.Trigger>
									{#snippet child({ props })}
										<button
											{...props}
											class="flex size-7 shrink-0 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
											aria-label="Connector actions"
											onclick={(e) => e.stopPropagation()}
										>
											<MoreVerticalIcon class="size-4" />
										</button>
									{/snippet}
								</DropdownMenu.Trigger>
								<DropdownMenu.Content align="end" class="w-36">
									<DropdownMenu.Item onclick={() => openEdit(connector)}>
										<PencilIcon class="size-4" />
										Edit
									</DropdownMenu.Item>
									<DropdownMenu.Separator />
									<DropdownMenu.Item variant="destructive" onclick={() => openDelete(connector)}>
										<Trash2Icon class="size-4" />
										Delete
									</DropdownMenu.Item>
								</DropdownMenu.Content>
							</DropdownMenu.Root>
						</div>

						<!-- body -->
						{#if connector.description}
							<p class="mt-4 line-clamp-2 text-sm text-muted-foreground">
								{connector.description}
							</p>
						{/if}
						{#if getConnectorCardInfo(connector)}
							<p class="mt-3 truncate font-mono text-xs text-muted-foreground">
								{getConnectorCardInfo(connector)}
							</p>
						{/if}
						{#if connectorFeatures[connector.type]}
							<div class="mt-3 flex flex-wrap gap-1.5">
								{#each connectorFeatures[connector.type].slice(0, 5) as feature}
									<span
										class="rounded-md bg-primary/10 px-1.5 py-0.5 text-[0.68rem] font-medium text-primary"
									>
										{feature}
									</span>
								{/each}
							</div>
						{/if}

						<!-- footer -->
						<div class="mt-auto flex items-center gap-2 border-t pt-4 text-xs text-muted-foreground">
							<span class="truncate">Secrets: {getSecretGroupName(connector.secretGroupId)}</span>
							<span
								class="ml-auto inline-flex shrink-0 items-center gap-1 font-medium text-primary opacity-0 transition-all duration-200 -translate-x-1 group-hover:translate-x-0 group-hover:opacity-100"
							>
								View details
								<ArrowRightIcon class="size-3.5" />
							</span>
						</div>
					</div>
				{/each}
			</div>
		{/if}
</div>

<ConnectorDetail
	connector={selectedConnector}
	open={selectedConnector !== null}
	onClose={closeDetail}
	onEdit={openEdit}
	onDelete={openDelete}
/>

<ConnectorCreateDialog bind:open={createDialogOpen} onCreated={loadConnectors} />

<ConnectorEditDialog
	bind:open={editDialogOpen}
	connector={editTarget}
	onUpdated={async () => {
		const editedId = editTarget?.id;
		await loadConnectors();
		editTarget = null;
		if (editedId && selectedConnector && selectedConnector.id === editedId) {
			const updated = $connectors.find((c) => c.id === editedId);
			if (updated) selectedConnector = updated;
		}
	}}
/>

<ConnectorDeleteDialog
	bind:open={deleteDialogOpen}
	connector={deleteTarget}
	onDeleted={async () => {
		const deletedId = deleteTarget?.id;
		await loadConnectors();
		if (deletedId && selectedConnector?.id === deletedId) {
			selectedConnector = null;
		}
		deleteTarget = null;
	}}
/>
