<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page as pageStore } from '$app/stores';
	import { goto } from '$app/navigation';
	import { tick } from 'svelte';
	import * as Alert from '$lib/components/ui/alert/index.js';
	import * as Empty from '$lib/components/ui/empty/index.js';
	import * as Table from '$lib/components/ui/table/index.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import * as Select from '$lib/components/ui/select/index.js';
	import * as ToggleGroup from '$lib/components/ui/toggle-group/index.js';
	import * as Tooltip from '$lib/components/ui/tooltip/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Skeleton } from '$lib/components/ui/skeleton/index.js';
	import ScheduleDialog from '$lib/components/ScheduleDialog.svelte';
	import NewScenarioDialog from '$lib/components/NewScenarioDialog.svelte';
	import FileIcon from '@lucide/svelte/icons/file';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import CalendarIcon from '@lucide/svelte/icons/calendar';
	import TrashIcon from '@lucide/svelte/icons/trash-2';
	import PencilIcon from '@lucide/svelte/icons/pencil';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import XIcon from '@lucide/svelte/icons/x';
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import { loadScenarioPage } from '$lib/stores/scenarios';
	import {
		deleteScenario,
		listSchedules,
		updateScenario,
		type ScenarioFilters
	} from '$lib/api/client';
	import { formatUserEmail, scenarioTypeVariant } from '$lib/utils/format';
	import type { SavedScenario, Schedule, ScenarioType } from '$lib/types';

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

	let pageScenarios = $state<SavedScenario[]>([]);
	let total = $state(0);
	let page = $state(1);
	let perPage = $state<number>(50);

	let scheduleMap = $state<Map<string, Schedule>>(new Map());

	let deleteDialogOpen = $state(false);
	let deleteTarget = $state<SavedScenario | null>(null);
	let deleting = $state(false);

	let scheduleDialogOpen = $state(false);
	let scheduleTarget = $state<SavedScenario | null>(null);

	let newDialogOpen = $state(false);

	let renameId = $state<string | null>(null);
	let renameValue = $state('');
	let renameSaving = $state(false);
	let renameInputEl = $state<HTMLInputElement | null>(null);

	// Filter state — seeded from URL on mount, persisted via goto() on change.
	let nameFilter = $state('');
	let typeFilter = $state<string[]>([]);
	let sinceFilter = $state<TimePresetValue>('all');

	const hasActiveFilters = $derived(
		nameFilter.length > 0 || typeFilter.length > 0 || sinceFilter !== 'all'
	);

	function currentFilters(): ScenarioFilters {
		const f: ScenarioFilters = {};
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
		const url = query ? `?${query}` : '/scenarios';
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

	// Monotonic request counter: stale responses are discarded.
	let requestSeq = 0;

	async function load() {
		const seq = ++requestSeq;
		const data = await loadScenarioPage(page, perPage, currentFilters());
		if (seq !== requestSeq) return;
		pageScenarios = data.scenarios;
		total = data.total;
	}

	async function loadScheduleMap() {
		const schedules = await listSchedules();
		const map = new Map<string, Schedule>();
		for (const schedule of schedules) {
			map.set(schedule.scenarioId, schedule);
		}
		scheduleMap = map;
	}

	async function changePage(next: number) {
		if (next < 1 || next > totalPages || next === page) return;
		page = next;
		syncUrl();
		loading = true;
		error = '';
		try {
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load scenarios';
		} finally {
			loading = false;
		}
	}

	async function changePerPage(value: string) {
		const next = Number(value);
		if (!PAGE_SIZES.includes(next as (typeof PAGE_SIZES)[number])) return;
		perPage = next;
		page = 1;
		syncUrl();
		loading = true;
		error = '';
		try {
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load scenarios';
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
			page = 1;
			syncUrl();
			loading = true;
			error = '';
			try {
				await load();
			} catch (e) {
				error = e instanceof Error ? e.message : 'Failed to load scenarios';
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
		if ($pageStore.url.searchParams.get('new') === '1') {
			newDialogOpen = true;
			const url = new URL($pageStore.url);
			url.searchParams.delete('new');
			goto(url.pathname + url.search, { replaceState: true, noScroll: true, keepFocus: true });
		}
		try {
			await Promise.all([load(), loadScheduleMap()]);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load scenarios';
		} finally {
			loading = false;
		}
	});

	onDestroy(() => {
		if (nameDebounce) clearTimeout(nameDebounce);
	});

	function openDelete(e: Event, scenario: SavedScenario) {
		e.stopPropagation();
		deleteTarget = scenario;
		deleteDialogOpen = true;
	}

	async function handleDelete() {
		if (!deleteTarget) return;
		deleting = true;
		error = '';
		try {
			await deleteScenario(deleteTarget.id);
			// If deleting the last row on a page > 1 leaves it empty, step back.
			if (pageScenarios.length === 1 && page > 1) {
				page--;
				syncUrl();
			}
			await Promise.all([load(), loadScheduleMap()]);
			deleteDialogOpen = false;
			deleteTarget = null;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Delete failed';
		} finally {
			deleting = false;
		}
	}

	function openSchedule(e: Event, scenario: SavedScenario) {
		e.stopPropagation();
		scheduleTarget = scenario;
		scheduleDialogOpen = true;
	}

	async function startRename(e: Event, scenario: SavedScenario) {
		e.stopPropagation();
		renameId = scenario.id;
		renameValue = scenario.name;
		await tick();
		renameInputEl?.focus();
		renameInputEl?.select();
	}

	function cancelRename() {
		renameId = null;
		renameValue = '';
	}

	async function commitRename(scenario: SavedScenario) {
		if (renameSaving) return;
		const next = renameValue.trim();
		if (!next || next === scenario.name) {
			cancelRename();
			return;
		}
		renameSaving = true;
		error = '';
		try {
			await updateScenario(scenario.id, next, scenario.yaml, scenario.type);
			await load();
			renameId = null;
			renameValue = '';
		} catch (e) {
			error = e instanceof Error ? e.message : 'Rename failed';
		} finally {
			renameSaving = false;
		}
	}

	function handleRenameKeydown(e: KeyboardEvent, scenario: SavedScenario) {
		if (e.key === 'Enter') {
			e.preventDefault();
			commitRename(scenario);
		} else if (e.key === 'Escape') {
			e.preventDefault();
			cancelRename();
		}
	}
</script>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<h1 class="text-2xl font-bold">Scenarios</h1>
		<Button onclick={() => (newDialogOpen = true)}>
			<PlusIcon class="mr-2 h-4 w-4" />
			New Scenario
		</Button>
	</div>

	<div class="flex flex-wrap items-end gap-3">
		<div class="flex flex-col gap-1.5">
			<label for="name-filter" class="text-muted-foreground text-xs font-medium">Name</label>
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
			<label for="since-filter" class="text-muted-foreground text-xs font-medium">Updated</label>
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
				<XIcon class="mr-1 h-4 w-4" />
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
			{#each Array(4) as _}
				<Skeleton class="h-12 w-full" />
			{/each}
		</div>
	{:else if pageScenarios.length === 0}
		<Empty.Root>
			<Empty.Header>
				<Empty.Media variant="icon">
					<FileIcon />
				</Empty.Media>
				<Empty.Title>
					{hasActiveFilters ? 'No matching scenarios' : 'No saved scenarios'}
				</Empty.Title>
				<Empty.Description>
					{hasActiveFilters
						? 'Try clearing or adjusting your filters.'
						: 'Save scenario YAML files here for quick access.'}
				</Empty.Description>
			</Empty.Header>
			<Empty.Content>
				{#if hasActiveFilters}
					<Button variant="outline" onclick={clearFilters}>
						<XIcon class="mr-2 h-4 w-4" />
						Clear filters
					</Button>
				{:else}
					<Button onclick={() => (newDialogOpen = true)}>New Scenario</Button>
				{/if}
			</Empty.Content>
		</Empty.Root>
	{:else}
		<div class="animate-fade-up stagger-2 overflow-hidden rounded-lg border">
			<Table.Root>
				<Table.Header class="bg-muted">
					<Table.Row>
						<Table.Head>Name</Table.Head>
						<Table.Head>Type</Table.Head>
						<Table.Head>Schedule</Table.Head>
						<Table.Head>Created</Table.Head>
						<Table.Head>Updated</Table.Head>
						<Table.Head class="w-20"></Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each pageScenarios as scenario}
						<Table.Row
							class="group cursor-pointer hover:bg-accent/50 transition-colors"
							onclick={() => {
								if (renameId !== scenario.id) goto(`/scenarios/${scenario.id}`);
							}}
						>
							<Table.Cell class="font-medium">
								{#if renameId === scenario.id}
									<div
										class="flex items-center gap-2"
										onclick={(e) => e.stopPropagation()}
										role="presentation"
									>
										<Input
											bind:ref={renameInputEl}
											bind:value={renameValue}
											onkeydown={(e: KeyboardEvent) => handleRenameKeydown(e, scenario)}
											onblur={() => commitRename(scenario)}
											disabled={renameSaving}
											class="h-7 max-w-xs"
										/>
										{#if renameSaving}
											<LoaderIcon class="h-3 w-3 animate-spin text-muted-foreground" />
										{/if}
									</div>
								{:else}
									<div class="flex items-center gap-1.5">
										<span class="truncate">{scenario.name}</span>
										<button
											type="button"
											title="Rename"
											aria-label="Rename scenario"
											class="inline-flex h-6 w-6 items-center justify-center rounded text-muted-foreground/60 hover:text-foreground hover:bg-accent opacity-0 group-hover:opacity-100 focus-visible:opacity-100 transition-opacity"
											onclick={(e) => startRename(e, scenario)}
										>
											<PencilIcon class="h-3.5 w-3.5" />
										</button>
									</div>
								{/if}
							</Table.Cell>
							<Table.Cell>
								<Badge variant={scenarioTypeVariant(scenario.type)}>
									{scenario.type || 'standard'}
								</Badge>
							</Table.Cell>
							<Table.Cell>
								{#if scheduleMap.has(scenario.id)}
									{@const schedule = scheduleMap.get(scenario.id)}
									<Badge variant={schedule?.enabled ? 'default' : 'secondary'}>
										{schedule?.enabled ? 'Scheduled' : 'Disabled'}
									</Badge>
								{:else}
									<span class="text-sm text-muted-foreground">None</span>
								{/if}
							</Table.Cell>
							<Table.Cell>
								<div class="flex flex-col">
									<span>{new Date(scenario.createdAt).toLocaleDateString()}</span>
									{#if scenario.createdBy && scenario.createdBy !== 'anonymous'}
										<Tooltip.Root>
											<Tooltip.Trigger class="text-xs text-muted-foreground cursor-default w-fit">
												by {formatUserEmail(scenario.createdBy)}
											</Tooltip.Trigger>
											<Tooltip.Content>{scenario.createdBy}</Tooltip.Content>
										</Tooltip.Root>
									{/if}
								</div>
							</Table.Cell>
							<Table.Cell>
								<div class="flex flex-col">
									<span>{new Date(scenario.updatedAt).toLocaleDateString()}</span>
									{#if scenario.updatedBy && scenario.updatedBy !== 'anonymous'}
										<Tooltip.Root>
											<Tooltip.Trigger class="text-xs text-muted-foreground cursor-default w-fit">
												by {formatUserEmail(scenario.updatedBy)}
											</Tooltip.Trigger>
											<Tooltip.Content>{scenario.updatedBy}</Tooltip.Content>
										</Tooltip.Root>
									{/if}
								</div>
							</Table.Cell>
							<Table.Cell>
								<div class="flex justify-end gap-1">
									<Button
										variant="ghost"
										size="icon"
										title="Schedule"
										class="h-8 w-8 text-muted-foreground hover:text-foreground"
										onclick={(e: Event) => openSchedule(e, scenario)}
									>
										<CalendarIcon class="h-4 w-4" />
									</Button>
									<Button
										variant="ghost"
										size="icon"
										title="Delete"
										class="h-8 w-8 text-muted-foreground hover:text-destructive"
										onclick={(e: Event) => openDelete(e, scenario)}
									>
										<TrashIcon class="h-4 w-4" />
									</Button>
								</div>
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

<!-- Delete Confirmation Dialog -->
<Dialog.Root bind:open={deleteDialogOpen}>
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>Delete Scenario</Dialog.Title>
			<Dialog.Description>
				Are you sure you want to delete "{deleteTarget?.name}"? This action cannot be undone.
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

<!-- Schedule Dialog -->
{#if scheduleTarget}
	<ScheduleDialog
		bind:open={scheduleDialogOpen}
		scenario={scheduleTarget}
		onclose={() => {
			scheduleDialogOpen = false;
			scheduleTarget = null;
		}}
		onsuccess={loadScheduleMap}
	/>
{/if}

<!-- New Scenario type picker -->
<NewScenarioDialog bind:open={newDialogOpen} />
