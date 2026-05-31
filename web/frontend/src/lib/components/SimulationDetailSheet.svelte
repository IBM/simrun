<script lang="ts">
	import type { SimulationManifest } from '$lib/types';
	import * as Sheet from '$lib/components/ui/sheet/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import { ScrollArea } from '$lib/components/ui/scroll-area/index.js';
	import * as Tooltip from '$lib/components/ui/tooltip/index.js';
	import { getTacticName, getTechniqueName } from '$lib/data/mitre';
	import ShieldAlertIcon from '@lucide/svelte/icons/shield-alert';
	import ClockIcon from '@lucide/svelte/icons/clock';
	import CloudIcon from '@lucide/svelte/icons/cloud';
	import BrushIcon from '@lucide/svelte/icons/brush';
	import ExternalLinkIcon from '@lucide/svelte/icons/external-link';
	import SwordsIcon from '@lucide/svelte/icons/swords';
	import TargetIcon from '@lucide/svelte/icons/target';
	import BracesIcon from '@lucide/svelte/icons/braces';
	import GlobeIcon from '@lucide/svelte/icons/globe';

	let {
		simulation,
		packName,
		open = $bindable(false)
	}: {
		simulation: SimulationManifest | null;
		packName: string;
		open: boolean;
	} = $props();

	const scopeColors: Record<string, string> = {
		aws: 'bg-attr-environment/15 text-attr-environment border-attr-environment/30',
		gcp: 'bg-attr-identity/15 text-attr-identity border-attr-identity/30',
		azure: 'bg-attr-destination/15 text-attr-destination border-attr-destination/30',
		generic: 'bg-muted text-muted-foreground border-border'
	};

	function scopeColor(scope: string): string {
		return scopeColors[scope] ?? scopeColors.generic;
	}

	function mitreAttackUrl(type: 'tactics' | 'techniques', id: string): string {
		if (type === 'tactics') {
			return `https://attack.mitre.org/tactics/${id}/`;
		}
		// Handle sub-techniques like T1078.004
		const parts = id.split('.');
		if (parts.length > 1) {
			return `https://attack.mitre.org/techniques/${parts[0]}/${parts[1]}/`;
		}
		return `https://attack.mitre.org/techniques/${id}/`;
	}
</script>

<Sheet.Root bind:open>
	<Sheet.Content side="right" class="sm:max-w-lg w-full p-0">
		{#if simulation}
			<div class="flex flex-col h-full">
				<!-- Header -->
				<div class="px-6 pt-6 pb-4 border-b border-border">
					<div class="flex items-start gap-3 pr-8">
						<div class="mt-0.5 rounded-md bg-primary/10 p-2 shrink-0">
							<ShieldAlertIcon class="size-5 text-primary" />
						</div>
						<div class="min-w-0">
							<Sheet.Title class="text-lg font-semibold leading-tight">
								{simulation.name}
							</Sheet.Title>
							<p class="mt-1.5 font-mono text-xs text-muted-foreground break-all">
								{simulation.id}
							</p>
						</div>
					</div>

					<!-- Attribute chips -->
					<div class="flex flex-wrap gap-2 mt-4">
						<Badge variant="outline" class="gap-1.5 {scopeColor(simulation.scope)}">
							<CloudIcon class="size-3" />
							{simulation.scope.toUpperCase()}
						</Badge>
						{#if simulation.isSlow}
							<Badge
								variant="outline"
								class="gap-1.5 bg-status-processing/10 text-status-processing border-status-processing/30"
							>
								<ClockIcon class="size-3" />
								Slow
							</Badge>
						{/if}
						{#if simulation.params_schema && Object.keys(simulation.params_schema).length > 0}
							<Badge variant="outline" class="gap-1.5">
								<BracesIcon class="size-3" />
								Parameterized
							</Badge>
						{/if}
						{#if simulation.terraform}
							<Badge
								variant="outline"
								class="gap-1.5 bg-attr-destination/10 text-attr-destination border-attr-destination/30"
							>
								<GlobeIcon class="size-3" />
								Terraform
							</Badge>
						{/if}
					</div>
				</div>

				<!-- Body -->
				<ScrollArea class="flex-1">
					<div class="px-6 py-5 space-y-6">
						<!-- Description -->
						<div>
							<h4 class="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-2">
								Description
							</h4>
							<div class="rounded-md border border-border p-3 max-h-32 overflow-y-auto">
								<p class="text-sm leading-relaxed text-foreground/90 whitespace-pre-line">
									{simulation.description || 'No description available.'}
								</p>
							</div>
						</div>

						<Separator />

						<!-- MITRE ATT&CK -->
						<div>
							<h4 class="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-3">
								MITRE ATT&CK Mapping
							</h4>

							{#if simulation.mitre?.tactics?.length > 0}
								<div class="mb-4">
									<div class="flex items-center gap-2 mb-2">
										<SwordsIcon class="size-3.5 text-muted-foreground" />
										<span class="text-xs font-medium text-muted-foreground">Tactics</span>
									</div>
									<div class="flex flex-wrap gap-1.5">
										{#each simulation.mitre.tactics as tacticId}
											<Tooltip.Root>
												<Tooltip.Trigger>
													<a
														href={mitreAttackUrl('tactics', tacticId)}
														target="_blank"
														rel="noopener noreferrer"
														class="inline-flex items-center gap-1 rounded-md border border-primary/20 bg-primary/5 px-2 py-1 text-xs font-medium text-primary hover:bg-primary/10 transition-colors"
													>
														{tacticId}
														<ExternalLinkIcon class="size-2.5 opacity-50" />
													</a>
												</Tooltip.Trigger>
												<Tooltip.Content>
													<p>{getTacticName(tacticId)}</p>
												</Tooltip.Content>
											</Tooltip.Root>
										{/each}
									</div>
								</div>
							{/if}

							{#if simulation.mitre?.techniques?.length > 0}
								<div>
									<div class="flex items-center gap-2 mb-2">
										<TargetIcon class="size-3.5 text-muted-foreground" />
										<span class="text-xs font-medium text-muted-foreground">Techniques</span>
									</div>
									<div class="flex flex-wrap gap-1.5">
										{#each simulation.mitre.techniques as techId}
											<Tooltip.Root>
												<Tooltip.Trigger>
													<a
														href={mitreAttackUrl('techniques', techId)}
														target="_blank"
														rel="noopener noreferrer"
														class="inline-flex items-center gap-1 rounded-md border border-border bg-muted/50 px-2 py-1 text-xs font-mono hover:bg-muted transition-colors"
													>
														{techId}
														<ExternalLinkIcon class="size-2.5 opacity-50" />
													</a>
												</Tooltip.Trigger>
												<Tooltip.Content>
													<p>{getTechniqueName(techId)}</p>
												</Tooltip.Content>
											</Tooltip.Root>
										{/each}
									</div>
								</div>
							{/if}

							{#if !simulation.mitre?.tactics?.length && !simulation.mitre?.techniques?.length}
								<p class="text-sm text-muted-foreground italic">No MITRE mapping defined.</p>
							{/if}
						</div>

						<Separator />

						<!-- Pack info -->
						<div>
							<h4 class="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-2">
								Pack
							</h4>
							<p class="text-sm">{packName}</p>
						</div>

						<!-- Params schema -->
						{#if simulation.params_schema && Object.keys(simulation.params_schema).length > 0}
							<Separator />
							<div>
								<h4
									class="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-2"
								>
									Parameters Schema
								</h4>
								<pre
									class="rounded-md border border-border bg-muted/30 p-3 text-xs font-mono overflow-x-auto whitespace-pre-wrap">{JSON.stringify(
										simulation.params_schema,
										null,
										2
									)}</pre>
							</div>
						{/if}
					</div>
				</ScrollArea>

				<!-- Footer -->
				<div class="px-6 py-3 border-t border-border bg-muted/30">
					<div class="flex items-center gap-4 text-xs text-muted-foreground">
						<span class="font-mono">{simulation.scope}.{simulation.id.split('.').pop()}</span>
						{#if simulation.isSlow}
							<span class="flex items-center gap-1">
								<ClockIcon class="size-3" />
								Slow execution
							</span>
						{/if}
					</div>
				</div>
			</div>
		{/if}
	</Sheet.Content>
</Sheet.Root>
