<script lang="ts">
	import type { Pack, PackManifest, SimulationManifest } from '$lib/types';
	import { MITRE_TACTICS, getTacticName, getTechniqueName, getTechnique } from '$lib/data/mitre';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import * as Tooltip from '$lib/components/ui/tooltip/index.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { ScrollArea } from '$lib/components/ui/scroll-area/index.js';
	import { Skeleton } from '$lib/components/ui/skeleton/index.js';
	import * as Empty from '$lib/components/ui/empty/index.js';
	import { getPackManifest } from '$lib/api/client';
	import { packs } from '$lib/stores/packs';
	import { onMount } from 'svelte';
	import ShieldCheckIcon from '@lucide/svelte/icons/shield-check';
	import ShieldXIcon from '@lucide/svelte/icons/shield-x';
	import LayersIcon from '@lucide/svelte/icons/layers';
	import CrosshairIcon from '@lucide/svelte/icons/crosshair';
	import CloudIcon from '@lucide/svelte/icons/cloud';

	// State
	let loading = $state(true);
	let manifests = $state<Map<string, PackManifest>>(new Map());
	let selectedCell = $state<{ tacticId: string; techniqueId: string } | null>(null);
	let detailDialogOpen = $state(false);

	// Load all pack manifests
	onMount(async () => {
		const results = new Map<string, PackManifest>();
		const loadPromises = $packs.map(async (pack) => {
			try {
				const manifest = await getPackManifest(pack.name);
				results.set(pack.name, manifest);
			} catch {
				// Skip packs that fail to load
			}
		});
		await Promise.all(loadPromises);
		manifests = results;
		loading = false;
	});

	// All simulations across all packs
	let allSimulations = $derived.by(() => {
		const sims: Array<{ sim: SimulationManifest; packName: string }> = [];
		for (const [packName, manifest] of manifests) {
			for (const sim of manifest.simulations) {
				sims.push({ sim, packName });
			}
		}
		return sims;
	});

	// Build coverage map: techniqueId -> array of sims that cover it
	let coverageByTechnique = $derived.by(() => {
		const map = new Map<string, Array<{ sim: SimulationManifest; packName: string }>>();
		for (const entry of allSimulations) {
			const techniques = entry.sim.mitre?.techniques ?? [];
			for (const techId of techniques) {
				const baseId = techId.split('.')[0]; // Normalize sub-techniques
				if (!map.has(baseId)) map.set(baseId, []);
				map.get(baseId)!.push(entry);
				// Also store with the full sub-technique ID
				if (techId !== baseId) {
					if (!map.has(techId)) map.set(techId, []);
					map.get(techId)!.push(entry);
				}
			}
		}
		return map;
	});

	// Build coverage map: tacticId -> array of sims that cover it
	let coverageByTactic = $derived.by(() => {
		const map = new Map<string, Array<{ sim: SimulationManifest; packName: string }>>();
		for (const entry of allSimulations) {
			const tactics = entry.sim.mitre?.tactics ?? [];
			for (const tacticId of tactics) {
				if (!map.has(tacticId)) map.set(tacticId, []);
				map.get(tacticId)!.push(entry);
			}
		}
		return map;
	});

	// Collect unique techniques per tactic from actual simulation data
	let techniquesByTactic = $derived.by(() => {
		const map = new Map<string, Set<string>>();
		for (const entry of allSimulations) {
			const tactics = entry.sim.mitre?.tactics ?? [];
			const techniques = entry.sim.mitre?.techniques ?? [];
			for (const tacticId of tactics) {
				if (!map.has(tacticId)) map.set(tacticId, new Set());
				for (const techId of techniques) {
					map.get(tacticId)!.add(techId);
				}
			}
		}
		return map;
	});

	// Tactics with coverage (only show tactics that have at least one simulation)
	let coveredTactics = $derived(MITRE_TACTICS.filter((t) => coverageByTactic.has(t.id)));

	// Stats
	let totalSimulations = $derived(allSimulations.length);
	let coveredTacticCount = $derived(coveredTactics.length);
	let coveredTechniqueCount = $derived(coverageByTechnique.size);

	// Cell click handler
	function handleCellClick(tacticId: string, techniqueId: string) {
		selectedCell = { tacticId, techniqueId };
		detailDialogOpen = true;
	}

	// Get simulations for a technique under a specific tactic
	function getSimsForCell(tacticId: string, techniqueId: string) {
		return allSimulations.filter(
			(e) =>
				(e.sim.mitre?.tactics ?? []).includes(tacticId) &&
				(e.sim.mitre?.techniques ?? []).includes(techniqueId)
		);
	}

	// Coverage intensity for coloring
	function coverageIntensity(count: number): string {
		if (count === 0) return 'bg-muted/30';
		if (count === 1) return 'bg-primary/20 border-primary/30';
		if (count === 2) return 'bg-primary/35 border-primary/40';
		return 'bg-primary/50 border-primary/50';
	}

	const scopeIcons: Record<string, string> = {
		aws: 'text-attr-environment',
		gcp: 'text-attr-identity',
		azure: 'text-attr-destination',
		generic: 'text-muted-foreground'
	};
</script>

<div class="space-y-6">
	{#if loading}
		<div class="grid gap-4 md:grid-cols-4">
			{#each Array(4) as _}
				<Skeleton class="h-24 w-full rounded-xl" />
			{/each}
		</div>
		<Skeleton class="h-96 w-full rounded-xl" />
	{:else if allSimulations.length === 0}
		<Empty.Root>
			<Empty.Header>
				<Empty.Media variant="icon">
					<ShieldXIcon />
				</Empty.Media>
				<Empty.Title>No coverage data</Empty.Title>
				<Empty.Description
					>Install packs with MITRE ATT&CK mappings to see coverage.</Empty.Description
				>
			</Empty.Header>
		</Empty.Root>
	{:else}
		<!-- Stats bar -->
		<div class="grid gap-3 grid-cols-2 md:grid-cols-4">
			<div class="rounded-lg border border-border bg-card p-4 animate-fade-up stagger-1">
				<div class="flex items-center gap-2 text-muted-foreground mb-1">
					<LayersIcon class="size-4" />
					<span class="text-xs font-medium uppercase tracking-wider">Simulations</span>
				</div>
				<p class="text-2xl font-bold tabular-nums">{totalSimulations}</p>
			</div>
			<div class="rounded-lg border border-border bg-card p-4 animate-fade-up stagger-2">
				<div class="flex items-center gap-2 text-muted-foreground mb-1">
					<ShieldCheckIcon class="size-4" />
					<span class="text-xs font-medium uppercase tracking-wider">Tactics</span>
				</div>
				<p class="text-2xl font-bold tabular-nums">
					{coveredTacticCount}<span class="text-sm font-normal text-muted-foreground"
						>/{MITRE_TACTICS.length}</span
					>
				</p>
			</div>
			<div class="rounded-lg border border-border bg-card p-4 animate-fade-up stagger-3">
				<div class="flex items-center gap-2 text-muted-foreground mb-1">
					<CrosshairIcon class="size-4" />
					<span class="text-xs font-medium uppercase tracking-wider">Techniques</span>
				</div>
				<p class="text-2xl font-bold tabular-nums">{coveredTechniqueCount}</p>
			</div>
			<div class="rounded-lg border border-border bg-card p-4 animate-fade-up stagger-4">
				<div class="flex items-center gap-2 text-muted-foreground mb-1">
					<CloudIcon class="size-4" />
					<span class="text-xs font-medium uppercase tracking-wider">Packs</span>
				</div>
				<p class="text-2xl font-bold tabular-nums">{manifests.size}</p>
			</div>
		</div>

		<!-- Coverage matrix -->
		<div class="rounded-lg border border-border bg-card overflow-hidden animate-fade-up stagger-5">
			<div class="px-4 py-3 border-b border-border">
				<h3 class="text-sm font-semibold">ATT&CK Coverage Matrix</h3>
				<p class="text-xs text-muted-foreground mt-0.5">
					Click any cell to see covering simulations
				</p>
			</div>

			<ScrollArea orientation="both" class="w-full">
				<div class="p-4 min-w-max">
					<div class="flex gap-2">
						{#each coveredTactics as tactic, i}
							{@const tacticSims = coverageByTactic.get(tactic.id) ?? []}
							{@const techs = techniquesByTactic.get(tactic.id) ?? new Set()}
							<div
								class="flex flex-col gap-1 min-w-[140px] animate-fade-up"
								style="animation-delay: {300 + i * 50}ms"
							>
								<!-- Tactic header -->
								<Tooltip.Root>
									<Tooltip.Trigger>
										<div
											class="rounded-md border border-primary/20 bg-primary/5 px-2 py-2 text-center"
										>
											<p
												class="text-[10px] font-semibold uppercase tracking-wider text-primary truncate"
											>
												{tactic.shortName}
											</p>
											<p class="text-[10px] text-muted-foreground mt-0.5">
												{tacticSims.length} sim{tacticSims.length !== 1 ? 's' : ''}
											</p>
										</div>
									</Tooltip.Trigger>
									<Tooltip.Content>
										<p class="font-medium">{tactic.name}</p>
										<p class="text-xs text-muted-foreground">{tactic.id}</p>
									</Tooltip.Content>
								</Tooltip.Root>

								<!-- Technique cells -->
								{#each [...techs] as techId}
									{@const sims = getSimsForCell(tactic.id, techId)}
									{@const count = sims.length}
									<Tooltip.Root>
										<Tooltip.Trigger>
											<button
												class="w-full rounded border {coverageIntensity(
													count
												)} px-2 py-1.5 text-left transition-all hover:ring-1 hover:ring-primary/50 hover:scale-[1.02] cursor-pointer"
												onclick={() => handleCellClick(tactic.id, techId)}
											>
												<p class="text-[10px] font-mono font-medium truncate">{techId}</p>
												<p class="text-[10px] text-muted-foreground truncate leading-tight mt-0.5">
													{getTechniqueName(techId)}
												</p>
											</button>
										</Tooltip.Trigger>
										<Tooltip.Content>
											<p class="font-medium">{getTechniqueName(techId)}</p>
											<p class="text-xs text-muted-foreground">
												{count} simulation{count !== 1 ? 's' : ''}
											</p>
										</Tooltip.Content>
									</Tooltip.Root>
								{/each}
							</div>
						{/each}
					</div>
				</div>
			</ScrollArea>

			<!-- Legend -->
			<div
				class="px-4 py-2 border-t border-border flex items-center gap-4 text-[10px] text-muted-foreground"
			>
				<span class="font-medium">Coverage:</span>
				<span class="flex items-center gap-1">
					<span class="inline-block size-3 rounded border bg-primary/20 border-primary/30"></span>
					1 sim
				</span>
				<span class="flex items-center gap-1">
					<span class="inline-block size-3 rounded border bg-primary/35 border-primary/40"></span>
					2 sims
				</span>
				<span class="flex items-center gap-1">
					<span class="inline-block size-3 rounded border bg-primary/50 border-primary/50"></span>
					3+ sims
				</span>
			</div>
		</div>

		<!-- Tactic breakdown cards -->
		<div class="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
			{#each coveredTactics as tactic, i}
				{@const sims = coverageByTactic.get(tactic.id) ?? []}
				{@const techs = techniquesByTactic.get(tactic.id) ?? new Set()}
				{@const uniqueScopes = new Set(sims.map((s) => s.sim.scope))}
				<div
					class="rounded-lg border border-border bg-card p-4 animate-fade-up"
					style="animation-delay: {i * 40}ms"
				>
					<div class="flex items-center justify-between mb-3">
						<div>
							<h4 class="text-sm font-semibold">{tactic.name}</h4>
							<p class="text-[10px] font-mono text-muted-foreground">{tactic.id}</p>
						</div>
						<Badge variant="outline" class="text-xs tabular-nums">
							{sims.length}
						</Badge>
					</div>

					<!-- Scopes -->
					<div class="flex gap-1 mb-3">
						{#each [...uniqueScopes] as scope}
							<span
								class="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] font-medium border border-border bg-muted/50 {scopeIcons[
									scope
								] ?? ''}"
							>
								{scope.toUpperCase()}
							</span>
						{/each}
					</div>

					<!-- Covered techniques -->
					<div class="flex flex-wrap gap-1">
						{#each [...techs].slice(0, 6) as techId}
							<span
								class="rounded bg-primary/10 px-1.5 py-0.5 text-[10px] font-mono text-primary/80"
							>
								{techId}
							</span>
						{/each}
						{#if techs.size > 6}
							<span class="rounded bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">
								+{techs.size - 6} more
							</span>
						{/if}
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>

<!-- Detail dialog for cell click -->
<Dialog.Root bind:open={detailDialogOpen}>
	<Dialog.Content class="sm:max-w-lg">
		{#if selectedCell}
			{@const sims = getSimsForCell(selectedCell.tacticId, selectedCell.techniqueId)}
			<Dialog.Header>
				<Dialog.Title class="flex items-center gap-2">
					<CrosshairIcon class="size-4 text-primary" />
					{getTechniqueName(selectedCell.techniqueId)}
				</Dialog.Title>
				<Dialog.Description>
					<span class="font-mono text-xs">{selectedCell.techniqueId}</span>
					under
					<span class="font-medium">{getTacticName(selectedCell.tacticId)}</span>
					<span class="font-mono text-xs">({selectedCell.tacticId})</span>
				</Dialog.Description>
			</Dialog.Header>
			<div class="space-y-2 max-h-80 overflow-y-auto">
				{#each sims as { sim, packName }}
					<div class="rounded-md border border-border p-3 space-y-1.5">
						<div class="flex items-center justify-between gap-2">
							<p class="text-sm font-medium truncate">{sim.name}</p>
							<Badge variant="outline" class="shrink-0 text-[10px]">
								{sim.scope.toUpperCase()}
							</Badge>
						</div>
						<p class="text-xs text-muted-foreground line-clamp-2">{sim.description}</p>
						<div class="flex items-center gap-2 text-[10px] text-muted-foreground">
							<span class="font-mono">{sim.id}</span>
							<span>&middot;</span>
							<span>{packName}</span>
						</div>
					</div>
				{/each}
				{#if sims.length === 0}
					<p class="text-sm text-muted-foreground py-4 text-center">
						No simulations cover this combination.
					</p>
				{/if}
			</div>
		{/if}
	</Dialog.Content>
</Dialog.Root>
