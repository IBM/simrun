<script lang="ts">
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import type { Pack, PackManifest } from '$lib/types';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import * as Tooltip from '$lib/components/ui/tooltip/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import PackParametersDialog from '$lib/components/PackParametersDialog.svelte';
	import PackSimulationsSheet from '$lib/components/PackSimulationsSheet.svelte';
	import { getPackManifest, deletePack } from '$lib/api/client';
	import { getTacticShortName, getTacticName } from '$lib/data/mitre';
	import { formatUserEmail } from '$lib/utils/format';
	import PackageIcon from '@lucide/svelte/icons/package';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import MoreVerticalIcon from '@lucide/svelte/icons/more-vertical';
	import SlidersHorizontalIcon from '@lucide/svelte/icons/sliders-horizontal';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import ClockIcon from '@lucide/svelte/icons/clock';
	import ArrowRightIcon from '@lucide/svelte/icons/arrow-right';

	let { pack, ondelete }: { pack: Pack; ondelete?: () => void } = $props();

	let manifest = $state<PackManifest | null>(null);
	let manifestLoading = $state(false);
	let manifestError = $state<string | null>(null);
	let deleteDialogOpen = $state(false);
	let deleting = $state(false);
	let parametersDialogOpen = $state(false);
	let simulationsSheetOpen = $state(false);

	onMount(() => {
		loadManifest();
	});

	async function loadManifest() {
		if (manifest || manifestLoading) return;
		manifestLoading = true;
		manifestError = null;
		try {
			manifest = await getPackManifest(pack.name);
		} catch (e) {
			manifestError = e instanceof Error ? e.message : 'Failed to load manifest';
		} finally {
			manifestLoading = false;
		}
	}

	async function handleDelete() {
		deleting = true;
		try {
			await deletePack(pack.name);
			deleteDialogOpen = false;
			toast.success('Pack deleted');
			ondelete?.();
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Delete failed');
		} finally {
			deleting = false;
		}
	}

	function openSimulations() {
		if (manifest) simulationsSheetOpen = true;
	}

	let displayVersion = $derived(manifest?.pack.version ?? pack.version);
	let simulationCount = $derived(manifest?.simulations.length ?? null);
	let templateCount = $derived(manifest?.templates?.length ?? 0);

	// Aggregate scopes, tactics and slow-flag across the pack's simulations.
	let scopes = $derived.by(() => {
		if (!manifest) return [];
		const set = new Set<string>();
		for (const sim of manifest.simulations) set.add(sim.scope);
		for (const tmpl of manifest.templates ?? []) set.add(tmpl.scope);
		return [...set];
	});

	let tactics = $derived.by(() => {
		if (!manifest) return [];
		const set = new Set<string>();
		for (const sim of manifest.simulations) {
			for (const t of sim.mitre?.tactics ?? []) set.add(t);
		}
		return [...set];
	});

	let hasSlowSims = $derived(manifest?.simulations.some((s) => s.isSlow) ?? false);

	const scopeDot: Record<string, string> = {
		aws: 'bg-attr-environment',
		gcp: 'bg-attr-identity',
		azure: 'bg-attr-destination'
	};

	let tacticsShown = $derived(tactics.slice(0, 3));
	let tacticsOverflow = $derived(tactics.length - tacticsShown.length);

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' || e.key === ' ') {
			e.preventDefault();
			openSimulations();
		}
	}
</script>

<div
	role="button"
	tabindex="0"
	aria-label="View {pack.name} simulations"
	onclick={openSimulations}
	onkeydown={handleKeydown}
	class="group relative flex h-full flex-col overflow-hidden rounded-xl border bg-card p-5 text-left transition-all duration-200
		hover:-translate-y-0.5 hover:shadow-sm
		focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring
		{manifest ? 'cursor-pointer' : 'cursor-default'}"
>
	<!-- accent line on the left edge, draws down on hover -->
	<span
		class="absolute inset-y-0 left-0 w-[3px] origin-top scale-y-0 bg-primary transition-transform duration-300 ease-out group-hover:scale-y-100"
	></span>

	<!-- header -->
	<div class="flex items-start gap-3">
		<div class="flex size-10 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
			<PackageIcon class="size-5" />
		</div>
		<div class="min-w-0 flex-1">
			<h3 class="truncate text-base font-semibold tracking-tight">{pack.name}</h3>
			<p class="mt-0.5 truncate font-mono text-xs text-muted-foreground">
				{pack.type === 'upload' ? 'uploaded binary' : pack.source}
			</p>
		</div>
		<Badge variant="outline" class="shrink-0 font-mono">v{displayVersion}</Badge>
		<DropdownMenu.Root>
			<DropdownMenu.Trigger>
				{#snippet child({ props })}
					<button
						{...props}
						class="flex size-7 shrink-0 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
						aria-label="Pack actions"
						onclick={(e) => e.stopPropagation()}
					>
						<MoreVerticalIcon class="size-4" />
					</button>
				{/snippet}
			</DropdownMenu.Trigger>
			<DropdownMenu.Content align="end" class="w-40">
				<DropdownMenu.Item onclick={() => (parametersDialogOpen = true)}>
					<SlidersHorizontalIcon class="size-4" />
					Parameters
				</DropdownMenu.Item>
				<DropdownMenu.Separator />
				<DropdownMenu.Item variant="destructive" onclick={() => (deleteDialogOpen = true)}>
					<Trash2Icon class="size-4" />
					Delete
				</DropdownMenu.Item>
			</DropdownMenu.Content>
		</DropdownMenu.Root>
	</div>

	{#if manifestError}
		<div class="mt-3 flex items-center gap-2 text-sm text-destructive">
			<span class="truncate">{manifestError}</span>
			<Button
				variant="ghost"
				size="icon"
				class="size-6 shrink-0"
				onclick={(e) => {
					e.stopPropagation();
					loadManifest();
				}}
				disabled={manifestLoading}
			>
				<RefreshCwIcon class="size-3.5" />
			</Button>
		</div>
	{/if}

	<!-- stats -->
	<div class="mt-4 flex items-baseline gap-5">
		<div>
			<div class="text-2xl font-bold leading-none tracking-tight">
				{simulationCount ?? '—'}
			</div>
			<div class="mt-1 text-[0.7rem] uppercase tracking-wide text-muted-foreground">
				simulation{simulationCount === 1 ? '' : 's'}
			</div>
		</div>
		<div>
			<div
				class="text-2xl font-bold leading-none tracking-tight {templateCount
					? ''
					: 'font-semibold text-muted-foreground'}"
			>
				{manifest ? templateCount : '—'}
			</div>
			<div class="mt-1 text-[0.7rem] uppercase tracking-wide text-muted-foreground">
				template{templateCount === 1 ? '' : 's'}
			</div>
		</div>
		{#if hasSlowSims}
			<span class="ml-auto flex items-center gap-1.5 self-center text-xs text-status-processing">
				<ClockIcon class="size-3.5" />
				has slow sims
			</span>
		{/if}
	</div>

	<!-- scopes + version -->
	<div class="mt-3 flex flex-wrap items-center gap-2">
		{#each scopes as scope}
			<span
				class="inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 font-mono text-[0.72rem] text-muted-foreground"
			>
				<span class="size-2 rounded-full {scopeDot[scope] ?? 'bg-muted-foreground'}"></span>
				{scope === 'generic' ? 'generic' : scope.toUpperCase()}
			</span>
		{/each}
	</div>

	<!-- MITRE tactics -->
	{#if tacticsShown.length > 0}
		<div class="mt-3 flex flex-wrap gap-1.5">
			{#each tacticsShown as tacticId}
				<Tooltip.Root>
					<Tooltip.Trigger
						class="rounded-md bg-primary/10 px-1.5 py-0.5 text-[0.68rem] font-medium text-primary"
					>
						{getTacticShortName(tacticId)}
					</Tooltip.Trigger>
					<Tooltip.Content>{getTacticName(tacticId)}</Tooltip.Content>
				</Tooltip.Root>
			{/each}
			{#if tacticsOverflow > 0}
				<span class="rounded-md bg-muted px-1.5 py-0.5 text-[0.68rem] text-muted-foreground">
					+{tacticsOverflow}
				</span>
			{/if}
		</div>
	{/if}

	<!-- spacer: keeps a minimum gap above the divider, grows to bottom-align footer -->
	<div class="min-h-5 flex-1"></div>

	<!-- footer -->
	<div class="flex items-center gap-2 border-t pt-4 text-xs text-muted-foreground">
		{#if pack.installedBy && pack.installedBy !== 'anonymous'}
			<Tooltip.Root>
				<Tooltip.Trigger class="cursor-default">{formatUserEmail(pack.installedBy)}</Tooltip.Trigger>
				<Tooltip.Content>{pack.installedBy}</Tooltip.Content>
			</Tooltip.Root>
			<span aria-hidden="true">·</span>
		{/if}
		<span>{new Date(pack.createdAt).toLocaleDateString()}</span>
		<span
			class="ml-auto inline-flex items-center gap-1 font-medium text-primary opacity-0 transition-all duration-200 -translate-x-1 group-hover:translate-x-0 group-hover:opacity-100"
		>
			View simulations
			<ArrowRightIcon class="size-3.5" />
		</span>
	</div>
</div>

<PackParametersDialog
	bind:open={parametersDialogOpen}
	packName={pack.name}
	onclose={() => (parametersDialogOpen = false)}
	onsuccess={() => {}}
/>

<PackSimulationsSheet {manifest} packName={pack.name} bind:open={simulationsSheetOpen} />

<Dialog.Root bind:open={deleteDialogOpen}>
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>Delete Pack</Dialog.Title>
			<Dialog.Description>
				Are you sure you want to delete the pack "{pack.name}"? This action cannot be undone.
			</Dialog.Description>
		</Dialog.Header>
		<div class="flex justify-end gap-2 pt-4">
			<Button variant="outline" onclick={() => (deleteDialogOpen = false)}>Cancel</Button>
			<Button variant="destructive" onclick={handleDelete} disabled={deleting}>
				{deleting ? 'Deleting...' : 'Delete'}
			</Button>
		</div>
	</Dialog.Content>
</Dialog.Root>
