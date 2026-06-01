<script lang="ts">
	import * as Sheet from '$lib/components/ui/sheet/index.js';
	import { ScrollArea } from '$lib/components/ui/scroll-area/index.js';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import * as Tooltip from '$lib/components/ui/tooltip/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { secrets } from '$lib/stores/secrets';
	import type { Connector, ElasticConnectorConfig } from '$lib/types';
	import { formatUserEmail } from '$lib/utils/format';
	import ElasticLogo from '$lib/components/ElasticLogo.svelte';
	import CloudIcon from '@lucide/svelte/icons/cloud';
	import TerminalIcon from '@lucide/svelte/icons/terminal';
	import PencilIcon from '@lucide/svelte/icons/pencil';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';

	let {
		connector,
		open,
		onClose,
		onEdit,
		onDelete
	}: {
		connector: Connector | null;
		open: boolean;
		onClose: () => void;
		onEdit: (c: Connector) => void;
		onDelete: (c: Connector) => void;
	} = $props();

	const cloudTypes = ['aws', 'gcp', 'azure', 'kubernetes', 'ssh'];

	const connectorFeatures: Record<string, string[]> = {
		elastic: ['Alert Matching', 'Rule Management', 'Log Collection'],
		aws: ['Cloud Target'],
		gcp: ['Cloud Target'],
		azure: ['Cloud Target'],
		kubernetes: ['K8s Target'],
		ssh: ['Remote Command Detonation']
	};

	const connectorFeatureDescriptions: Record<string, Record<string, string>> = {
		elastic: {
			'Alert Matching': 'Match Elastic Security detection alerts against scenario expectations',
			'Rule Management': 'List and view detection rules in Elastic Security',
			'Log Collection': 'Collect logs from Elasticsearch after detonation'
		},
		aws: { 'Cloud Target': 'Used as a detonation target for AWS attack simulations' },
		gcp: { 'Cloud Target': 'Used as a detonation target for GCP attack simulations' },
		azure: { 'Cloud Target': 'Used as a detonation target for Azure attack simulations' },
		kubernetes: { 'K8s Target': 'Used as a detonation target for Kubernetes attack simulations' },
		ssh: {
			'Remote Command Detonation':
				'Execute shell commands on a remote host over SSH for detonation'
		}
	};

	function getElasticConfig(c: Connector): ElasticConnectorConfig {
		return c.config as unknown as ElasticConnectorConfig;
	}

	function cfg(c: Connector): Record<string, unknown> {
		return c.config as Record<string, unknown>;
	}

	function getSecretGroupName(id: string | undefined): string {
		if (!id) return 'None';
		return $secrets.find((s) => s.id === id)?.name || 'Unknown';
	}
</script>

<Sheet.Root {open} onOpenChange={(v) => { if (!v) onClose(); }}>
	<Sheet.Content side="right" class="w-full p-0 sm:max-w-lg">
		{#if connector}
			{@const c = connector}
			<div class="flex h-full flex-col">
				<!-- Header -->
				<div class="border-b border-border px-6 pb-4 pt-6">
					<div class="flex items-start gap-3 pr-8">
						<div class="flex size-10 shrink-0 items-center justify-center rounded-md bg-muted">
							{#if c.type === 'elastic'}
								<ElasticLogo size={28} />
							{:else if c.type === 'ssh'}
								<TerminalIcon size={20} class="text-muted-foreground" />
							{:else}
								<CloudIcon size={20} class="text-muted-foreground" />
							{/if}
						</div>
						<div class="min-w-0">
							<Sheet.Title class="text-lg font-semibold leading-tight">{c.name}</Sheet.Title>
							<p class="mt-0.5 text-xs uppercase tracking-wide text-muted-foreground">{c.type}</p>
						</div>
					</div>

					<div class="mt-4 flex flex-wrap gap-2">
						<Badge variant={c.enabled ? 'default' : 'secondary'}>
							{c.enabled ? 'Enabled' : 'Disabled'}
						</Badge>
						{#if c.isDefault}
							<Badge variant="outline">Default</Badge>
						{/if}
					</div>
				</div>

				<ScrollArea class="flex-1">
					<div class="space-y-6 px-6 py-5">
						{#if c.description}
							<div>
								<h4 class="mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
									Description
								</h4>
								<p class="text-sm leading-relaxed text-foreground/90">{c.description}</p>
							</div>
							<Separator />
						{/if}

						<!-- Configuration -->
						<div>
							<h4 class="mb-3 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
								Configuration
							</h4>
							<div class="space-y-3 text-sm">
								<div class="grid grid-cols-[130px_1fr] gap-2">
									<span class="font-medium text-muted-foreground">Type</span>
									<span class="uppercase">{c.type}</span>
								</div>

								{#if c.type === 'elastic'}
									<div class="grid grid-cols-[130px_1fr] gap-2">
										<span class="font-medium text-muted-foreground">Kibana URL</span>
										<span class="break-all font-mono text-xs">{getElasticConfig(c).kibana_url}</span>
									</div>
									{#if getElasticConfig(c).cloud_id}
										<div class="grid grid-cols-[130px_1fr] gap-2">
											<span class="font-medium text-muted-foreground">Cloud ID</span>
											<span class="break-all font-mono text-xs">{getElasticConfig(c).cloud_id}</span>
										</div>
									{/if}
									{#if getElasticConfig(c).elasticsearch_url}
										<div class="grid grid-cols-[130px_1fr] gap-2">
											<span class="font-medium text-muted-foreground">Elasticsearch URL</span>
											<span class="break-all font-mono text-xs"
												>{getElasticConfig(c).elasticsearch_url}</span
											>
										</div>
									{/if}
									<div class="grid grid-cols-[130px_1fr] gap-2">
										<span class="font-medium text-muted-foreground">Result Export</span>
										<span>
											{#if getElasticConfig(c).export_enabled}
												<Badge variant="default">Enabled</Badge>
												<span class="ml-2 font-mono text-xs"
													>logs-{getElasticConfig(c).export_datastream || 'asp.results'}-default</span
												>
											{:else}
												<Badge variant="secondary">Disabled</Badge>
											{/if}
										</span>
									</div>
								{:else if c.type === 'aws'}
									<div class="grid grid-cols-[130px_1fr] gap-2">
										<span class="font-medium text-muted-foreground">Role ARN</span>
										<span class="break-all font-mono text-xs">{cfg(c).role_arn || ''}</span>
									</div>
								{:else if c.type === 'gcp'}
									{#if cfg(c).project_id}
										<div class="grid grid-cols-[130px_1fr] gap-2">
											<span class="font-medium text-muted-foreground">Project ID</span>
											<span class="font-mono text-xs">{cfg(c).project_id}</span>
										</div>
									{/if}
									{#if cfg(c).auth_type === 'workload_identity_federation'}
										<div class="grid grid-cols-[130px_1fr] gap-2">
											<span class="font-medium text-muted-foreground">Auth Type</span>
											<Badge variant="secondary">Workload Identity Federation</Badge>
										</div>
										<div class="grid grid-cols-[130px_1fr] gap-2">
											<span class="font-medium text-muted-foreground">Project Number</span>
											<span class="font-mono text-xs">{cfg(c).project_number || ''}</span>
										</div>
										<div class="grid grid-cols-[130px_1fr] gap-2">
											<span class="font-medium text-muted-foreground">Pool ID</span>
											<span class="font-mono text-xs">{cfg(c).pool_id || ''}</span>
										</div>
										<div class="grid grid-cols-[130px_1fr] gap-2">
											<span class="font-medium text-muted-foreground">Provider ID</span>
											<span class="font-mono text-xs">{cfg(c).provider_id || ''}</span>
										</div>
										<div class="grid grid-cols-[130px_1fr] gap-2">
											<span class="font-medium text-muted-foreground">Service Account</span>
											<span class="break-all font-mono text-xs"
												>{cfg(c).service_account_email || ''}</span
											>
										</div>
									{:else}
										<div class="grid grid-cols-[130px_1fr] gap-2">
											<span class="font-medium text-muted-foreground">Auth Type</span>
											<Badge variant="secondary">Service Account</Badge>
										</div>
										<div class="grid grid-cols-[130px_1fr] gap-2">
											<span class="font-medium text-muted-foreground">Credentials</span>
											<span>Configured via secret group</span>
										</div>
									{/if}
								{:else if c.type === 'azure'}
									<div class="grid grid-cols-[130px_1fr] gap-2">
										<span class="font-medium text-muted-foreground">Tenant ID</span>
										<span class="break-all font-mono text-xs">{cfg(c).tenant_id || ''}</span>
									</div>
									<div class="grid grid-cols-[130px_1fr] gap-2">
										<span class="font-medium text-muted-foreground">Subscription ID</span>
										<span class="break-all font-mono text-xs">{cfg(c).subscription_id || ''}</span>
									</div>
									<div class="grid grid-cols-[130px_1fr] gap-2">
										<span class="font-medium text-muted-foreground">Client ID</span>
										<span class="break-all font-mono text-xs">{cfg(c).client_id || ''}</span>
									</div>
									{#if cfg(c).auth_type === 'workload_identity_federation'}
										<div class="grid grid-cols-[130px_1fr] gap-2">
											<span class="font-medium text-muted-foreground">Auth Type</span>
											<Badge variant="secondary">Workload Identity Federation</Badge>
										</div>
										{#if cfg(c).token_file}
											<div class="grid grid-cols-[130px_1fr] gap-2">
												<span class="font-medium text-muted-foreground">Token File</span>
												<span class="break-all font-mono text-xs">{cfg(c).token_file}</span>
											</div>
										{/if}
									{:else}
										<div class="grid grid-cols-[130px_1fr] gap-2">
											<span class="font-medium text-muted-foreground">Auth Type</span>
											<Badge variant="secondary">Service Principal</Badge>
										</div>
									{/if}
								{:else if c.type === 'kubernetes'}
									<div class="grid grid-cols-[130px_1fr] gap-2">
										<span class="font-medium text-muted-foreground">Cluster Name</span>
										<span class="break-all font-mono text-xs">{cfg(c).cluster_name || ''}</span>
									</div>
									<div class="grid grid-cols-[130px_1fr] gap-2">
										<span class="font-medium text-muted-foreground">Region</span>
										<span class="font-mono text-xs">{cfg(c).region || ''}</span>
									</div>
									<div class="grid grid-cols-[130px_1fr] gap-2">
										<span class="font-medium text-muted-foreground">Cloud Connector</span>
										<span class="font-mono text-xs">{cfg(c).cloud_connector || ''}</span>
									</div>
									{#if cfg(c).resource_group}
										<div class="grid grid-cols-[130px_1fr] gap-2">
											<span class="font-medium text-muted-foreground">Resource Group</span>
											<span class="font-mono text-xs">{cfg(c).resource_group}</span>
										</div>
									{/if}
									{#if cfg(c).project}
										<div class="grid grid-cols-[130px_1fr] gap-2">
											<span class="font-medium text-muted-foreground">Project</span>
											<span class="font-mono text-xs">{cfg(c).project}</span>
										</div>
									{/if}
								{:else if c.type === 'ssh'}
									<div class="grid grid-cols-[130px_1fr] gap-2">
										<span class="font-medium text-muted-foreground">Host</span>
										<span class="break-all font-mono text-xs">{cfg(c).host || ''}</span>
									</div>
									<div class="grid grid-cols-[130px_1fr] gap-2">
										<span class="font-medium text-muted-foreground">Username</span>
										<span class="font-mono text-xs">{cfg(c).username || ''}</span>
									</div>
									<div class="grid grid-cols-[130px_1fr] gap-2">
										<span class="font-medium text-muted-foreground">Port</span>
										{#if cfg(c).port}
											<span class="font-mono text-xs">{cfg(c).port}</span>
										{:else}
											<span class="text-xs text-muted-foreground">22 (default)</span>
										{/if}
									</div>
								{/if}

								<div class="grid grid-cols-[130px_1fr] gap-2">
									<span class="font-medium text-muted-foreground">Secret Group</span>
									<span>{getSecretGroupName(c.secretGroupId)}</span>
								</div>
								{#if cloudTypes.includes(c.type)}
									<div class="grid grid-cols-[130px_1fr] gap-2">
										<span class="font-medium text-muted-foreground">Default</span>
										<span>{c.isDefault ? 'Yes' : 'No'}</span>
									</div>
								{/if}
								<div class="grid grid-cols-[130px_1fr] gap-2">
									<span class="font-medium text-muted-foreground">Created</span>
									<span>
										{new Date(c.createdAt).toLocaleString()}
										{#if c.createdBy && c.createdBy !== 'anonymous'}
											<Tooltip.Root>
												<Tooltip.Trigger class="cursor-default text-muted-foreground">
													by {formatUserEmail(c.createdBy)}
												</Tooltip.Trigger>
												<Tooltip.Content>{c.createdBy}</Tooltip.Content>
											</Tooltip.Root>
										{/if}
									</span>
								</div>
								<div class="grid grid-cols-[130px_1fr] gap-2">
									<span class="font-medium text-muted-foreground">Updated</span>
									<span>
										{new Date(c.updatedAt).toLocaleString()}
										{#if c.updatedBy && c.updatedBy !== 'anonymous'}
											<Tooltip.Root>
												<Tooltip.Trigger class="cursor-default text-muted-foreground">
													by {formatUserEmail(c.updatedBy)}
												</Tooltip.Trigger>
												<Tooltip.Content>{c.updatedBy}</Tooltip.Content>
											</Tooltip.Root>
										{/if}
									</span>
								</div>
							</div>
						</div>

						<Separator />

						<!-- Features -->
						<div>
							<h4 class="mb-3 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
								Features
							</h4>
							<div class="space-y-2">
								{#each connectorFeatures[c.type] || [] as feature}
									<div class="rounded-md border border-border p-3">
										<p class="text-sm font-medium">{feature}</p>
										<p class="mt-1 text-xs text-muted-foreground">
											{connectorFeatureDescriptions[c.type]?.[feature] || ''}
										</p>
									</div>
								{/each}
							</div>
						</div>
					</div>
				</ScrollArea>

				<!-- Footer actions -->
				<div class="flex gap-2 border-t border-border px-6 py-4">
					<Button variant="outline" size="sm" class="flex-1" onclick={() => onEdit(c)}>
						<PencilIcon class="size-4" />
						Edit
					</Button>
					<Button variant="destructive" size="sm" class="flex-1" onclick={() => onDelete(c)}>
						<Trash2Icon class="size-4" />
						Delete
					</Button>
				</div>
			</div>
		{/if}
	</Sheet.Content>
</Sheet.Root>
