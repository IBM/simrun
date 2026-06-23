<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page as pageStore } from '$app/stores';
	import { toast } from 'svelte-sonner';
	import * as Alert from '$lib/components/ui/alert/index.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import * as Empty from '$lib/components/ui/empty/index.js';
	import * as Table from '$lib/components/ui/table/index.js';
	import * as Select from '$lib/components/ui/select/index.js';
	import * as ToggleGroup from '$lib/components/ui/toggle-group/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Skeleton } from '$lib/components/ui/skeleton/index.js';
	import { runs } from '$lib/stores/runs';
	import { listRuns, deleteRun, getConfig, type RunFilters } from '$lib/api/client';
	import { goto } from '$app/navigation';
	import {
		statusVariant,
		formatDuration,
		formatUserEmail,
		formatRelativeTime,
		formatTime,
		scenarioTypeVariant
	} from '$lib/utils/format';
	import type { Run, ScenarioType, AppConfig } from '$lib/types';
	import * as Tooltip from '$lib/components/ui/tooltip/index.js';
	import NewAssessmentDialog from '$lib/components/NewAssessmentDialog.svelte';
	import RetentionDialog from '$lib/components/RetentionDialog.svelte';
	import PenLineIcon from '@lucide/svelte/icons/pen-line';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import TimerIcon from '@lucide/svelte/icons/timer';
	import TrashIcon from '@lucide/svelte/icons/trash-2';
	import XIcon from '@lucide/svelte/icons/x';
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';

	const PAGE_SIZES = [25, 50, 100] as const;
	const SCENARIO_TYPES: ScenarioType[] = ['standard', 'explore', 'collect'];
	const TIME_PRESETS = [
		{ value: 'all', label: 'Any time', duration: '' },
		{ value: '24h', label: 'Last 24 hours', duration: '24h' },
		{ value: '48h', label: 'Last 48 hours', duration: '48h' },
		{ value: '72h', label: 'Last 72 hours', duration: '72h' },
		{ value: '1w', label: 'Last week', duration: '168h' },
		{ value: '1mo', label: 'Last month', duration: '720h' }
	] as const;
	type TimePresetValue = (typeof TIME_PRESETS)[number]['value'];

	let loading = $state(true);
	let error = $state('');
	let newAssessmentOpen = $state(false);

	let retentionOpen = $state(false);
	let retentionConfig = $state<AppConfig>({});

	// Load config lazily when opening retention settings so the common case
	// (browsing assessments) doesn't pay for an extra request.
	async function openRetention() {
		try {
			retentionConfig = await getConfig();
			retentionOpen = true;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load retention settings';
		}
	}

	function handleRetentionSaved(changes: Record<string, unknown>) {
		retentionConfig = { ...retentionConfig, ...changes };
	}

	let deleteDialogOpen = $state(false);
	let deleteTarget = $state<Run | null>(null);
	let deleting = $state(false);

	let pageRuns = $state<Run[]>([]);
	let total = $state(0);
	let page = $state(1);
	let perPage = $state<number>(50);

	// Running assessments visible on the current page — drives the live header pill.
	const runningCount = $derived(pageRuns.filter((r) => r.status === 'running').length);

	// Filter state — seeded from URL on mount, persisted via goto() on change.
	let nameFilter = $state('');
	let typeFilter = $state<string[]>([]);
	let sinceFilter = $state<TimePresetValue>('all');

	const hasActiveFilters = $derived(
		nameFilter.length > 0 || typeFilter.length > 0 || sinceFilter !== 'all'
	);

	function currentFilters(): RunFilters {
		const f: RunFilters = {};
		if (nameFilter) f.name = nameFilter;
		if (typeFilter.length > 0) f.types = typeFilter;
		const preset = TIME_PRESETS.find((p) => p.value === sinceFilter);
		if (preset?.duration) f.since = preset.duration;
		return f;
	}

	function syncUrl() {
		const qs = new URLSearchParams();
		if (nameFilter) qs.set('name', nameFilter);
		for (const t of typeFilter) qs.append('type', t);
		if (sinceFilter !== 'all') qs.set('since', sinceFilter);
		if (page > 1) qs.set('page', String(page));
		if (perPage !== 50) qs.set('per_page', String(perPage));
		const query = qs.toString();
		const url = query ? `?${query}` : '/assessments';
		goto(url, { keepFocus: true, noScroll: true, replaceState: true });
	}

	function seedFromUrl() {
		const url = $pageStore.url.searchParams;
		nameFilter = url.get('name') ?? '';
		typeFilter = url.getAll('type').filter((t) => SCENARIO_TYPES.includes(t as ScenarioType));
		const since = url.get('since');
		sinceFilter = (TIME_PRESETS.find((p) => p.value === since)?.value ?? 'all') as TimePresetValue;
		const p = Number(url.get('page'));
		page = Number.isInteger(p) && p > 0 ? p : 1;
		const pp = Number(url.get('per_page'));
		perPage = PAGE_SIZES.includes(pp as (typeof PAGE_SIZES)[number]) ? pp : 50;
	}

	const totalPages = $derived(Math.max(1, Math.ceil(total / perPage)));
	const pageNumbers = $derived(buildPageRange(page, totalPages));
	const startIndex = $derived(total === 0 ? 0 : (page - 1) * perPage + 1);
	const endIndex = $derived(Math.min(page * perPage, total));

	// Build a compact paginator: 1 … (cur-1) cur (cur+1) … last.
	function buildPageRange(current: number, last: number): (number | '...')[] {
		if (last <= 7) return Array.from({ length: last }, (_, i) => i + 1);
		const out: (number | '...')[] = [1];
		const start = Math.max(2, current - 1);
		const end = Math.min(last - 1, current + 1);
		if (start > 2) out.push('...');
		for (let i = start; i <= end; i++) out.push(i);
		if (end < last - 1) out.push('...');
		out.push(last);
		return out;
	}

	let pollTimer: ReturnType<typeof setInterval> | null = null;

	function stopPolling() {
		if (pollTimer) {
			clearInterval(pollTimer);
			pollTimer = null;
		}
	}

	// Poll only when on page 1 — running runs are always newest.
	function startPollingIfNeeded() {
		stopPolling();
		if (page !== 1) return;
		const hasActive = pageRuns.some((r) => r.status === 'running');
		if (hasActive) {
			pollTimer = setInterval(async () => {
				try {
					await load();
					if (!pageRuns.some((r) => r.status === 'running')) {
						stopPolling();
					}
				} catch {
					// Silently retry on next interval
				}
			}, 5000);
		}
	}

	// Monotonic request counter: stale responses (from a previous page or a
	// poll that fired during navigation) are discarded.
	let requestSeq = 0;

	async function load() {
		const seq = ++requestSeq;
		const data = await listRuns(page, perPage, currentFilters());
		if (seq !== requestSeq) return;
		pageRuns = data.runs;
		total = data.total;
		// Keep sidebar/dashboard fresh while on page 1 with no filters applied
		// (otherwise we'd poison the global store with filtered data).
		if (page === 1 && !hasActiveFilters) runs.set(data.runs);
	}

	async function changePage(next: number) {
		if (next < 1 || next > totalPages || next === page) return;
		stopPolling();
		page = next;
		syncUrl();
		loading = true;
		error = '';
		try {
			await load();
			startPollingIfNeeded();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load assessments';
		} finally {
			loading = false;
		}
	}

	async function changePerPage(value: string) {
		const next = Number(value);
		if (!PAGE_SIZES.includes(next as (typeof PAGE_SIZES)[number])) return;
		stopPolling();
		perPage = next;
		page = 1;
		syncUrl();
		loading = true;
		error = '';
		try {
			await load();
			startPollingIfNeeded();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load assessments';
		} finally {
			loading = false;
		}
	}

	let nameDebounce: ReturnType<typeof setTimeout> | null = null;

	// Filter changes always reset to page 1 and re-run load. Name input is
	// debounced 300ms; type/since fire immediately.
	async function applyFilters({ debounce = false }: { debounce?: boolean } = {}) {
		if (nameDebounce) {
			clearTimeout(nameDebounce);
			nameDebounce = null;
		}
		const run = async () => {
			stopPolling();
			page = 1;
			syncUrl();
			loading = true;
			error = '';
			try {
				await load();
				startPollingIfNeeded();
			} catch (e) {
				error = e instanceof Error ? e.message : 'Failed to load assessments';
			} finally {
				loading = false;
			}
		};
		if (debounce) {
			nameDebounce = setTimeout(run, 300);
		} else {
			await run();
		}
	}

	function clearFilters() {
		nameFilter = '';
		typeFilter = [];
		sinceFilter = 'all';
		applyFilters();
	}

	onMount(async () => {
		seedFromUrl();
		try {
			await load();
			startPollingIfNeeded();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load assessments';
		} finally {
			loading = false;
		}
	});

	onDestroy(() => {
		stopPolling();
		if (nameDebounce) clearTimeout(nameDebounce);
	});

	function openDelete(e: Event, run: Run) {
		e.stopPropagation();
		deleteTarget = run;
		deleteDialogOpen = true;
	}

	async function handleDelete() {
		if (!deleteTarget) return;
		deleting = true;
		error = '';
		try {
			await deleteRun(deleteTarget.id);
			// If deleting the last row on a page > 1 leaves it empty, step back.
			if (pageRuns.length === 1 && page > 1) {
				page--;
			}
			await load();
			toast.success('Assessment deleted');
			deleteDialogOpen = false;
			deleteTarget = null;
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Delete failed');
		} finally {
			deleting = false;
		}
	}
</script>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div class="flex items-center gap-3">
			<h1 class="text-2xl font-bold">Assessment History</h1>
			{#if runningCount > 0}
				<span
					class="inline-flex items-center gap-1.5 rounded-full border border-status-processing/35 bg-status-processing/10 px-2.5 py-0.5 font-mono text-xs text-status-processing"
				>
					<span class="h-1.5 w-1.5 animate-pulse rounded-full bg-status-processing"></span>
					{runningCount} running
				</span>
			{/if}
		</div>
		<div class="flex items-center gap-2">
			<Button variant="outline" onclick={openRetention}>
				<TimerIcon data-icon="inline-start" />
				Retention
			</Button>
			<Button onclick={() => (newAssessmentOpen = true)}>
				<PlusIcon data-icon="inline-start" />
				New Assessment
			</Button>
		</div>
	</div>

	<div class="flex flex-wrap items-end gap-3">
		<div class="flex flex-col gap-1.5">
			<label for="name-filter" class="text-muted-foreground text-xs font-medium">Scenario</label>
			<Input
				id="name-filter"
				type="text"
				placeholder="Search by name…"
				class="h-9 w-[220px]"
				bind:value={nameFilter}
				oninput={() => applyFilters({ debounce: true })}
			/>
		</div>

		<div class="flex flex-col gap-1.5">
			<span class="text-muted-foreground text-xs font-medium">Type</span>
			<ToggleGroup.Root
				type="multiple"
				bind:value={typeFilter}
				onValueChange={() => applyFilters()}
				variant="outline"
				size="sm"
			>
				{#each SCENARIO_TYPES as t}
					<ToggleGroup.Item value={t} class="h-9 px-3 capitalize">{t}</ToggleGroup.Item>
				{/each}
			</ToggleGroup.Root>
		</div>

		<div class="flex flex-col gap-1.5">
			<label for="since-filter" class="text-muted-foreground text-xs font-medium">Executed</label>
			<Select.Root
				type="single"
				value={sinceFilter}
				onValueChange={(v) => {
					sinceFilter = v as TimePresetValue;
					applyFilters();
				}}
			>
				<Select.Trigger id="since-filter" class="h-9 w-[170px]">
					{TIME_PRESETS.find((p) => p.value === sinceFilter)?.label ?? 'Any time'}
				</Select.Trigger>
				<Select.Content>
					{#each TIME_PRESETS as preset}
						<Select.Item value={preset.value}>{preset.label}</Select.Item>
					{/each}
				</Select.Content>
			</Select.Root>
		</div>

		{#if hasActiveFilters}
			<Button variant="ghost" size="sm" class="h-9" onclick={clearFilters}>
				<XIcon data-icon="inline-start" />
				Clear filters
			</Button>
		{/if}
	</div>

	{#if error}
		<Alert.Root variant="destructive">
			<Alert.Description>{error}</Alert.Description>
		</Alert.Root>
	{/if}

	{#if loading}
		<div class="space-y-3">
			{#each Array(5) as _, i}
				<Skeleton class="h-12 w-full" />
			{/each}
		</div>
	{:else if pageRuns.length === 0}
		<Empty.Root>
			<Empty.Header>
				<Empty.Media variant="icon">
					<PenLineIcon />
				</Empty.Media>
				<Empty.Title>
					{hasActiveFilters ? 'No matching assessments' : 'No assessments yet'}
				</Empty.Title>
				<Empty.Description>
					{hasActiveFilters
						? 'Try clearing or adjusting your filters.'
						: 'Execute scenario files to see assessment history here.'}
				</Empty.Description>
			</Empty.Header>
			<Empty.Content>
				{#if hasActiveFilters}
					<Button variant="outline" onclick={clearFilters}>
						<XIcon data-icon="inline-start" />
						Clear filters
					</Button>
				{:else}
					<Button onclick={() => (newAssessmentOpen = true)}>
						<PlusIcon data-icon="inline-start" />
						New Assessment
					</Button>
				{/if}
			</Empty.Content>
		</Empty.Root>
	{:else}
		<div class="animate-fade-up stagger-2 overflow-hidden rounded-lg border">
			<Table.Root>
				<Table.Header class="bg-muted">
					<Table.Row>
						<Table.Head>ID</Table.Head>
						<Table.Head>Status</Table.Head>
						<Table.Head>Scenario</Table.Head>
						<Table.Head>Type</Table.Head>
						<Table.Head class="w-[200px]">Results</Table.Head>
						<Table.Head>Started By</Table.Head>
						<Table.Head>Started</Table.Head>
						<Table.Head>Duration</Table.Head>
						<Table.Head class="w-10"></Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each pageRuns as run (run.id)}
						{@const pending = Math.max(0, run.total - run.succeeded - run.failed)}
						<Table.Row
							class="cursor-pointer hover:bg-accent/50 transition-colors"
							onclick={() => goto(`/assessments/${run.id}`)}
						>
							<Table.Cell class="font-mono text-xs">
								{run.id.slice(0, 8)}
							</Table.Cell>
							<Table.Cell>
								<Badge variant={statusVariant(run.status)} class="gap-1.5">
									{#if run.status === 'running'}
										<span class="h-1.5 w-1.5 animate-pulse rounded-full bg-status-processing"
										></span>
									{/if}
									{run.status}
								</Badge>
							</Table.Cell>
							<Table.Cell class="max-w-[200px] truncate">
								{run.scenarioName || '--'}
							</Table.Cell>
							<Table.Cell>
								{#if run.scenarioType}
									<Badge variant={scenarioTypeVariant(run.scenarioType)}>{run.scenarioType}</Badge>
								{:else}
									<span class="text-muted-foreground text-xs">--</span>
								{/if}
							</Table.Cell>
							<Table.Cell>
								{#if run.total > 0}
									<div class="flex items-center gap-2.5">
										<div class="flex h-[7px] w-24 overflow-hidden rounded-full bg-muted">
											<div
												class="h-full bg-status-success"
												style="width: {(run.succeeded / run.total) * 100}%"
											></div>
											<div
												class="h-full bg-status-error"
												style="width: {(run.failed / run.total) * 100}%"
											></div>
											{#if pending > 0}
												<div
													class="h-full bg-status-processing/40"
													style="width: {(pending / run.total) * 100}%"
												></div>
											{/if}
										</div>
										<span class="font-mono text-xs whitespace-nowrap">
											<span class="font-medium text-status-success">{run.succeeded}</span><span
												class="text-muted-foreground">/{run.total}</span
											>
										</span>
									</div>
								{:else}
									<span class="text-muted-foreground text-xs">--</span>
								{/if}
							</Table.Cell>
							<Table.Cell>
								<Tooltip.Root>
									<Tooltip.Trigger class="text-muted-foreground text-xs cursor-default">
										{formatUserEmail(run.createdBy)}
									</Tooltip.Trigger>
									<Tooltip.Content>{run.createdBy}</Tooltip.Content>
								</Tooltip.Root>
							</Table.Cell>
							<Table.Cell>
								<Tooltip.Root>
									<Tooltip.Trigger class="cursor-default whitespace-nowrap">
										{formatRelativeTime(run.startTime)}
									</Tooltip.Trigger>
									<Tooltip.Content>{formatTime(run.startTime)}</Tooltip.Content>
								</Tooltip.Root>
							</Table.Cell>
							<Table.Cell class="font-mono text-xs tabular-nums"
								>{formatDuration(run.startTime, run.endTime)}</Table.Cell
							>
							<Table.Cell>
								<Button
									variant="ghost"
									size="icon"
									class="h-8 w-8 text-muted-foreground hover:text-destructive"
									onclick={(e: Event) => openDelete(e, run)}
								>
									<TrashIcon class="h-4 w-4" />
								</Button>
							</Table.Cell>
						</Table.Row>
					{/each}
				</Table.Body>
			</Table.Root>
		</div>

		<div class="flex flex-wrap items-center justify-between gap-3 pt-1">
			<div class="text-muted-foreground text-sm">
				{#if total > 0}
					Showing <span class="font-medium text-foreground">{startIndex}–{endIndex}</span>
					of <span class="font-medium text-foreground">{total}</span>
				{/if}
			</div>

			<div class="flex items-center gap-4">
				<div class="flex items-center gap-2 text-sm">
					<span class="text-muted-foreground">Rows per page</span>
					<Select.Root type="single" value={String(perPage)} onValueChange={changePerPage}>
						<Select.Trigger class="h-8 w-[72px]">{perPage}</Select.Trigger>
						<Select.Content>
							{#each PAGE_SIZES as size}
								<Select.Item value={String(size)}>{size}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				</div>

				<div class="flex items-center gap-1">
					<Button
						variant="outline"
						size="icon"
						class="h-8 w-8"
						disabled={page <= 1}
						onclick={() => changePage(page - 1)}
						aria-label="Previous page"
					>
						<ChevronLeftIcon class="h-4 w-4" />
					</Button>
					{#each pageNumbers as p}
						{#if p === '...'}
							<span class="px-2 text-muted-foreground text-sm">…</span>
						{:else}
							<Button
								variant={p === page ? 'default' : 'outline'}
								size="icon"
								class="h-8 w-8"
								onclick={() => changePage(p)}
								aria-current={p === page ? 'page' : undefined}
							>
								{p}
							</Button>
						{/if}
					{/each}
					<Button
						variant="outline"
						size="icon"
						class="h-8 w-8"
						disabled={page >= totalPages}
						onclick={() => changePage(page + 1)}
						aria-label="Next page"
					>
						<ChevronRightIcon class="h-4 w-4" />
					</Button>
				</div>
			</div>
		</div>
	{/if}
</div>

<NewAssessmentDialog bind:open={newAssessmentOpen} />

<RetentionDialog
	bind:open={retentionOpen}
	config={retentionConfig}
	onsaved={handleRetentionSaved}
/>

<!-- Delete Confirmation Dialog -->
<Dialog.Root bind:open={deleteDialogOpen}>
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>Delete Assessment</Dialog.Title>
			<Dialog.Description>
				Are you sure you want to delete assessment "{deleteTarget?.id.slice(0, 8)}"? This will
				permanently remove the assessment, all scenario results, and log files. This action cannot
				be undone.
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
