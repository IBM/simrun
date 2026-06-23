<script lang="ts">
	import type { PackManifest, SimulationManifest, TemplateManifest } from '$lib/types';
	import * as Sheet from '$lib/components/ui/sheet/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import { ScrollArea } from '$lib/components/ui/scroll-area/index.js';
	import * as Tooltip from '$lib/components/ui/tooltip/index.js';
	import { getTacticShortName, getTacticName, getTechniqueName } from '$lib/data/mitre';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import ArrowLeftIcon from '@lucide/svelte/icons/arrow-left';
	import SearchIcon from '@lucide/svelte/icons/search';
	import ClockIcon from '@lucide/svelte/icons/clock';
	import CloudIcon from '@lucide/svelte/icons/cloud';
	import ShieldAlertIcon from '@lucide/svelte/icons/shield-alert';
	import BracesIcon from '@lucide/svelte/icons/braces';
	import GlobeIcon from '@lucide/svelte/icons/globe';
	import ExternalLinkIcon from '@lucide/svelte/icons/external-link';
	import SwordsIcon from '@lucide/svelte/icons/swords';
	import TargetIcon from '@lucide/svelte/icons/target';
	import SyringeIcon from '@lucide/svelte/icons/syringe';

	type DetailView =
		| { kind: 'simulation'; item: SimulationManifest }
		| { kind: 'template'; item: TemplateManifest };

	let {
		manifest,
		packName,
		open = $bindable(false)
	}: {
		manifest: PackManifest | null;
		packName: string;
		open: boolean;
	} = $props();

	let searchQuery = $state('');
	let selectedDetail = $state<DetailView | null>(null);

	let filteredSimulations = $derived.by(() => {
		if (!manifest) return [];
		const q = searchQuery.toLowerCase().trim();
		if (!q) return manifest.simulations;
		return manifest.simulations.filter(
			(sim) => sim.name.toLowerCase().includes(q) || sim.id.toLowerCase().includes(q)
		);
	});

	let filteredTemplates = $derived.by(() => {
		if (!manifest?.templates) return [];
		const q = searchQuery.toLowerCase().trim();
		if (!q) return manifest.templates;
		return manifest.templates.filter(
			(tmpl) => tmpl.name.toLowerCase().includes(q) || tmpl.id.toLowerCase().includes(q)
		);
	});

	let totalCount = $derived(
		(manifest?.simulations.length ?? 0) + (manifest?.templates?.length ?? 0)
	);

	let decodedTemplateContent = $derived.by(() => {
		if (selectedDetail?.kind !== 'template') return '';
		try {
			const decoded = atob(selectedDetail.item.content);
			try {
				return JSON.stringify(JSON.parse(decoded), null, 2);
			} catch {
				return decoded;
			}
		} catch {
			return '(unable to decode content)';
		}
	});

	function selectSimulation(sim: SimulationManifest) {
		selectedDetail = { kind: 'simulation', item: sim };
	}

	function selectTemplate(tmpl: TemplateManifest) {
		selectedDetail = { kind: 'template', item: tmpl };
	}

	function backToList() {
		selectedDetail = null;
	}

	// Reset state when sheet closes
	$effect(() => {
		if (!open) {
			searchQuery = '';
			selectedDetail = null;
		}
	});

	const scopeColors: Record<string, string> = {
		aws: 'text-attr-environment',
		gcp: 'text-attr-identity',
		azure: 'text-attr-destination'
	};

	const scopeBadgeColors: Record<string, string> = {
		aws: 'bg-attr-environment/15 text-attr-environment border-attr-environment/30',
		gcp: 'bg-attr-identity/15 text-attr-identity border-attr-identity/30',
		azure: 'bg-attr-destination/15 text-attr-destination border-attr-destination/30',
		generic: 'bg-muted text-muted-foreground border-border'
	};

	function scopeBadgeColor(scope: string): string {
		return scopeBadgeColors[scope] ?? scopeBadgeColors.generic;
	}

	function mitreAttackUrl(type: 'tactics' | 'techniques', id: string): string {
		if (type === 'tactics') {
			return `https://attack.mitre.org/tactics/${id}/`;
		}
		const parts = id.split('.');
		if (parts.length > 1) {
			return `https://attack.mitre.org/techniques/${parts[0]}/${parts[1]}/`;
		}
		return `https://attack.mitre.org/techniques/${id}/`;
	}
</script>

<Sheet.Root bind:open>
	<Sheet.Content side="right" class="sm:max-w-lg w-full p-0">
		{#if manifest}
			<div class="flex flex-col h-full">
				{#if selectedDetail?.kind === 'simulation'}
					{@const selectedSimulation = selectedDetail.item}
					<!-- Drill-down: Simulation Detail -->
					<div class="px-6 pt-6 pb-4 border-b border-border">
						<div class="flex items-center gap-2 mb-3">
							<Button
								variant="ghost"
								size="sm"
								class="h-7 w-7 p-0"
								onclick={backToList}
								aria-label="Back to list"
							>
								<ArrowLeftIcon class="size-4" />
							</Button>
							<span class="text-xs text-muted-foreground">Back to list</span>
						</div>
						<div class="flex items-start gap-3 pr-8">
							<div class="mt-0.5 rounded-md bg-primary/10 p-2 shrink-0">
								<ShieldAlertIcon class="size-5 text-primary" />
							</div>
							<div class="min-w-0">
								<Sheet.Title class="text-lg font-semibold leading-tight">
									{selectedSimulation.name}
								</Sheet.Title>
								<p class="mt-1.5 font-mono text-xs text-muted-foreground break-all">
									{selectedSimulation.id}
								</p>
							</div>
						</div>

						<div class="flex flex-wrap gap-2 mt-4">
							<Badge variant="outline" class="gap-1.5 {scopeBadgeColor(selectedSimulation.scope)}">
								<CloudIcon class="size-3" />
								{selectedSimulation.scope.toUpperCase()}
							</Badge>
							{#if selectedSimulation.isSlow}
								<Badge
									variant="outline"
									class="gap-1.5 bg-status-processing/10 text-status-processing border-status-processing/30"
								>
									<ClockIcon class="size-3" />
									Slow
								</Badge>
							{/if}
							{#if selectedSimulation.params_schema && Object.keys(selectedSimulation.params_schema).length > 0}
								<Badge variant="outline" class="gap-1.5">
									<BracesIcon class="size-3" />
									Parameterized
								</Badge>
							{/if}
							{#if selectedSimulation.terraform}
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

					<ScrollArea class="flex-1">
						<div class="px-6 py-5 space-y-6">
							<!-- Description -->
							<div>
								<h4
									class="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-2"
								>
									Description
								</h4>
								<div class="rounded-md border border-border p-3 max-h-32 overflow-y-auto">
									<p class="text-sm leading-relaxed text-foreground/90 whitespace-pre-line">
										{selectedSimulation.description || 'No description available.'}
									</p>
								</div>
							</div>

							<Separator />

							<!-- MITRE ATT&CK -->
							<div>
								<h4
									class="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-3"
								>
									MITRE ATT&CK Mapping
								</h4>

								{#if selectedSimulation.mitre?.tactics?.length > 0}
									<div class="mb-4">
										<div class="flex items-center gap-2 mb-2">
											<SwordsIcon class="size-3.5 text-muted-foreground" />
											<span class="text-xs font-medium text-muted-foreground">Tactics</span>
										</div>
										<div class="flex flex-wrap gap-1.5">
											{#each selectedSimulation.mitre.tactics as tacticId}
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

								{#if selectedSimulation.mitre?.techniques?.length > 0}
									<div>
										<div class="flex items-center gap-2 mb-2">
											<TargetIcon class="size-3.5 text-muted-foreground" />
											<span class="text-xs font-medium text-muted-foreground">Techniques</span>
										</div>
										<div class="flex flex-wrap gap-1.5">
											{#each selectedSimulation.mitre.techniques as techId}
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

								{#if !selectedSimulation.mitre?.tactics?.length && !selectedSimulation.mitre?.techniques?.length}
									<p class="text-sm text-muted-foreground italic">No MITRE mapping defined.</p>
								{/if}
							</div>

							<Separator />

							<!-- Pack info -->
							<div>
								<h4
									class="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-2"
								>
									Pack
								</h4>
								<p class="text-sm">{packName}</p>
							</div>

							<!-- Params schema -->
							{#if selectedSimulation.params_schema && Object.keys(selectedSimulation.params_schema).length > 0}
								<Separator />
								<div>
									<h4
										class="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-2"
									>
										Parameters Schema
									</h4>
									<pre
										class="rounded-md border border-border bg-muted/30 p-3 text-xs font-mono overflow-x-auto whitespace-pre-wrap">{JSON.stringify(
											selectedSimulation.params_schema,
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
							<span class="font-mono"
								>{selectedSimulation.scope}.{selectedSimulation.id.split('.').pop()}</span
							>
							{#if selectedSimulation.isSlow}
								<span class="flex items-center gap-1">
									<ClockIcon class="size-3" />
									Slow execution
								</span>
							{/if}
						</div>
					</div>
				{:else if selectedDetail?.kind === 'template'}
					{@const selectedTemplate = selectedDetail.item}
					<!-- Drill-down: Template Detail -->
					<div class="px-6 pt-6 pb-4 border-b border-border">
						<div class="flex items-center gap-2 mb-3">
							<Button
								variant="ghost"
								size="sm"
								class="h-7 w-7 p-0"
								onclick={backToList}
								aria-label="Back to list"
							>
								<ArrowLeftIcon class="size-4" />
							</Button>
							<span class="text-xs text-muted-foreground">Back to list</span>
						</div>
						<div class="flex items-start gap-3 pr-8">
							<div class="mt-0.5 rounded-md bg-attr-asset/10 p-2 shrink-0">
								<SyringeIcon class="size-5 text-attr-asset" />
							</div>
							<div class="min-w-0">
								<Sheet.Title class="text-lg font-semibold leading-tight">
									{selectedTemplate.name}
								</Sheet.Title>
								<p class="mt-1.5 font-mono text-xs text-muted-foreground break-all">
									{selectedTemplate.id}
								</p>
							</div>
						</div>

						<div class="flex flex-wrap gap-2 mt-4">
							<Badge variant="outline" class="gap-1.5 {scopeBadgeColor(selectedTemplate.scope)}">
								<CloudIcon class="size-3" />
								{selectedTemplate.scope.toUpperCase()}
							</Badge>
							<Badge
								variant="outline"
								class="gap-1.5 bg-attr-asset/10 text-attr-asset border-attr-asset/30"
							>
								<SyringeIcon class="size-3" />
								Injection Template
							</Badge>
						</div>
					</div>

					<ScrollArea class="flex-1">
						<div class="px-6 py-5 space-y-6">
							<!-- Description -->
							<div>
								<h4
									class="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-2"
								>
									Description
								</h4>
								<div class="rounded-md border border-border p-3 max-h-32 overflow-y-auto">
									<p class="text-sm leading-relaxed text-foreground/90 whitespace-pre-line">
										{selectedTemplate.description || 'No description available.'}
									</p>
								</div>
							</div>

							<Separator />

							<!-- Pack info -->
							<div>
								<h4
									class="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-2"
								>
									Pack
								</h4>
								<p class="text-sm">{packName}</p>
							</div>

							<Separator />

							<!-- Template content preview -->
							<div>
								<h4
									class="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-2"
								>
									Template Content
								</h4>
								<pre
									class="rounded-md border border-border bg-muted/30 p-3 text-xs font-mono overflow-x-auto whitespace-pre-wrap max-h-64 overflow-y-auto">{decodedTemplateContent}</pre>
							</div>
						</div>
					</ScrollArea>

					<!-- Footer -->
					<div class="px-6 py-3 border-t border-border bg-muted/30">
						<div class="flex items-center gap-4 text-xs text-muted-foreground">
							<span class="font-mono">{selectedTemplate.id}</span>
						</div>
					</div>
				{:else}
					<!-- List view -->
					<div class="px-6 pt-6 pb-4 border-b border-border">
						<div class="pr-8">
							<Sheet.Title class="text-lg font-semibold">
								{manifest.pack.name}
							</Sheet.Title>
							<p class="mt-1 text-sm text-muted-foreground">
								{manifest.pack.version} &middot; {totalCount}
								{totalCount === 1 ? 'item' : 'items'}
								{#if manifest.simulations.length > 0}
									<span class="text-muted-foreground/70">
										({manifest.simulations.length} simulation{manifest.simulations.length !== 1
											? 's'
											: ''}{manifest.templates?.length
											? `, ${manifest.templates.length} template${manifest.templates.length !== 1 ? 's' : ''}`
											: ''})
									</span>
								{/if}
							</p>
							{#if manifest.pack.description}
								<p class="mt-2 text-sm text-foreground/80">
									{manifest.pack.description}
								</p>
							{/if}
						</div>

						<div class="relative mt-4">
							<SearchIcon class="absolute left-2.5 top-2.5 size-4 text-muted-foreground" />
							<Input
								placeholder="Filter simulations & templates..."
								class="pl-9"
								bind:value={searchQuery}
							/>
						</div>
					</div>

					<ScrollArea class="flex-1">
						<div class="px-6 py-3 space-y-0.5">
							{#if filteredSimulations.length === 0 && filteredTemplates.length === 0}
								<p class="text-sm text-muted-foreground py-4 text-center">
									No items match your search.
								</p>
							{:else}
								{#if filteredSimulations.length > 0}
									{#if filteredTemplates.length > 0}
										<div
											class="flex items-center gap-2 py-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground"
										>
											<ShieldAlertIcon class="size-3.5" />
											Simulations
										</div>
									{/if}
									{#each filteredSimulations as sim}
										<button
											class="w-full flex items-center justify-between gap-2 text-xs py-2 px-2 -mx-2 rounded-md border border-transparent hover:border-border hover:bg-muted/50 transition-all cursor-pointer group"
											onclick={() => selectSimulation(sim)}
										>
											<div class="flex-1 min-w-0 text-left">
												<div class="flex items-center gap-2">
													<span class="font-medium truncate">{sim.name}</span>
													{#if sim.isSlow}
														<ClockIcon class="size-3 text-status-processing shrink-0" />
													{/if}
												</div>
												<div class="flex items-center gap-2 mt-0.5">
													<span class="font-mono text-muted-foreground truncate">{sim.id}</span>
													<span class="text-muted-foreground {scopeColors[sim.scope] ?? ''}">
														{sim.scope}
													</span>
												</div>
												{#if sim.mitre?.tactics?.length > 0}
													<div class="flex gap-1 mt-1">
														{#each sim.mitre.tactics.slice(0, 3) as tacticId}
															<span
																class="rounded bg-primary/10 px-1 py-0.5 text-[10px] font-mono text-primary/80"
															>
																{getTacticShortName(tacticId)}
															</span>
														{/each}
														{#if sim.mitre.tactics.length > 3}
															<span
																class="rounded bg-muted px-1 py-0.5 text-[10px] text-muted-foreground"
															>
																+{sim.mitre.tactics.length - 3}
															</span>
														{/if}
													</div>
												{/if}
											</div>
											<ChevronRightIcon
												class="size-4 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity shrink-0"
											/>
										</button>
									{/each}
								{/if}

								{#if filteredTemplates.length > 0}
									{#if filteredSimulations.length > 0}
										<div class="pt-3">
											<Separator />
										</div>
									{/if}
									<div
										class="flex items-center gap-2 py-2 text-xs font-semibold uppercase tracking-wider text-attr-asset"
									>
										<SyringeIcon class="size-3.5" />
										Injection Templates
									</div>
									{#each filteredTemplates as tmpl}
										<button
											class="w-full flex items-center justify-between gap-2 text-xs py-2 px-2 -mx-2 rounded-md border border-transparent hover:border-attr-asset/20 hover:bg-attr-asset/5 transition-all cursor-pointer group"
											onclick={() => selectTemplate(tmpl)}
										>
											<div class="flex-1 min-w-0 text-left">
												<div class="flex items-center gap-2">
													<SyringeIcon class="size-3 text-attr-asset shrink-0" />
													<span class="font-medium truncate">{tmpl.name}</span>
												</div>
												<div class="flex items-center gap-2 mt-0.5">
													<span class="font-mono text-muted-foreground truncate">{tmpl.id}</span>
													<span class="text-muted-foreground {scopeColors[tmpl.scope] ?? ''}">
														{tmpl.scope}
													</span>
												</div>
											</div>
											<ChevronRightIcon
												class="size-4 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity shrink-0"
											/>
										</button>
									{/each}
								{/if}
							{/if}
						</div>
					</ScrollArea>
				{/if}
			</div>
		{/if}
	</Sheet.Content>
</Sheet.Root>
