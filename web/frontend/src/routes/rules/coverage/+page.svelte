<script lang="ts">
	import { onMount } from 'svelte';
	import { getRuleCoverage } from '$lib/api/client';
	import type { CoverageResponse } from '$lib/types';
	import * as Table from '$lib/components/ui/table/index.js';
	import * as Alert from '$lib/components/ui/alert/index.js';
	import * as Select from '$lib/components/ui/select/index.js';
	import * as Tooltip from '$lib/components/ui/tooltip/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Skeleton } from '$lib/components/ui/skeleton/index.js';
	import * as Pagination from '$lib/components/ui/pagination/index.js';
	import { formatRelativeTime, formatTime } from '$lib/utils/format';
	import AlertCircleIcon from '@lucide/svelte/icons/alert-circle';
	import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import ArrowUpDownIcon from '@lucide/svelte/icons/arrow-up-down';
	import ShieldAlertIcon from '@lucide/svelte/icons/shield-alert';

	let data = $state<CoverageResponse | null>(null);
	let loading = $state(true);
	let error = $state('');

	// Filters
	let search = $state('');
	let coverageFilter = $state('all');
	let severityFilter = $state('all');
	let sortField = $state<'name' | 'severity' | 'riskScore'>('name');
	let sortAsc = $state(true);

	// Expanded rows
	let expandedRows = $state<Set<string>>(new Set());

	const SEVERITIES = ['critical', 'high', 'medium', 'low'] as const;
	type Severity = (typeof SEVERITIES)[number];
	const severityOrder: Record<string, number> = { critical: 0, high: 1, medium: 2, low: 3 };

	// Per-severity token classes — drives the breakdown bars, row accents and chips.
	const severityClasses: Record<
		Severity,
		{ text: string; bar: string; accent: string; chip: string }
	> = {
		critical: {
			text: 'text-status-error',
			bar: 'bg-status-error',
			accent: 'border-l-status-error',
			chip: 'bg-status-error/10 text-status-error border-status-error/30'
		},
		high: {
			text: 'text-status-processing',
			bar: 'bg-status-processing',
			accent: 'border-l-status-processing',
			chip: 'bg-status-processing/10 text-status-processing border-status-processing/30'
		},
		medium: {
			text: 'text-status-warning',
			bar: 'bg-status-warning',
			accent: 'border-l-status-warning',
			chip: 'bg-status-warning/10 text-status-warning border-status-warning/30'
		},
		low: {
			text: 'text-status-info',
			bar: 'bg-status-info',
			accent: 'border-l-status-info',
			chip: 'bg-status-info/10 text-status-info border-status-info/30'
		}
	};

	function sevClass(severity: string) {
		return severityClasses[severity as Severity] ?? severityClasses.low;
	}

	function toggleRow(ruleId: string) {
		const next = new Set(expandedRows);
		if (next.has(ruleId)) next.delete(ruleId);
		else next.add(ruleId);
		expandedRows = next;
	}

	function toggleSort(field: 'name' | 'severity' | 'riskScore') {
		if (sortField === field) sortAsc = !sortAsc;
		else {
			sortField = field;
			sortAsc = true;
		}
	}

	// Coverage ring geometry (mirrors the assessment hero gauge).
	const GAUGE_R = 54;
	const GAUGE_CIRC = 2 * Math.PI * GAUGE_R;
	const coveragePct = $derived(data?.summary.coveragePercent ?? 0);
	const gaugeOffset = $derived(GAUGE_CIRC * (1 - coveragePct / 100));

	const passingCount = $derived(
		data?.rules.filter((r) => r.lastResult?.passed === true).length ?? 0
	);
	const failingCount = $derived(
		data?.rules.filter((r) => r.lastResult?.passed === false).length ?? 0
	);

	// Coverage grouped by severity — the headline insight. Only severities that
	// actually appear are shown, kept in escalation order.
	const severityBreakdown = $derived.by(() => {
		if (!data) return [];
		const buckets = new Map<string, { total: number; covered: number }>();
		for (const r of data.rules) {
			const b = buckets.get(r.severity) ?? { total: 0, covered: 0 };
			b.total += 1;
			if (r.covered) b.covered += 1;
			buckets.set(r.severity, b);
		}
		return SEVERITIES.filter((s) => buckets.has(s)).map((s) => {
			const b = buckets.get(s)!;
			return { severity: s, total: b.total, covered: b.covered, pct: (b.covered / b.total) * 100 };
		});
	});

	// The actionable gap: critical/high rules with no simulation coverage.
	const highSevGaps = $derived(
		data?.rules.filter((r) => !r.covered && (r.severity === 'critical' || r.severity === 'high'))
			.length ?? 0
	);

	const filteredRules = $derived.by(() => {
		if (!data) return [];
		let rules = data.rules;
		if (search) {
			const q = search.toLowerCase();
			rules = rules.filter(
				(r) => r.name.toLowerCase().includes(q) || r.tags.some((t) => t.toLowerCase().includes(q))
			);
		}
		if (coverageFilter === 'covered') rules = rules.filter((r) => r.covered);
		else if (coverageFilter === 'uncovered') rules = rules.filter((r) => !r.covered);
		if (severityFilter !== 'all') rules = rules.filter((r) => r.severity === severityFilter);
		rules = [...rules].sort((a, b) => {
			let cmp = 0;
			if (sortField === 'name') cmp = a.name.localeCompare(b.name);
			else if (sortField === 'severity')
				cmp = (severityOrder[a.severity] ?? 99) - (severityOrder[b.severity] ?? 99);
			else if (sortField === 'riskScore') cmp = a.riskScore - b.riskScore;
			return sortAsc ? cmp : -cmp;
		});
		return rules;
	});

	// Client-side pagination over the filtered rules (all rules arrive in one fetch).
	const PER_PAGE = 25;
	let coveragePage = $state(1);

	// Filter/search changes reset to the first page.
	$effect(() => {
		search;
		coverageFilter;
		severityFilter;
		coveragePage = 1;
	});

	const pagedRules = $derived(
		filteredRules.slice((coveragePage - 1) * PER_PAGE, coveragePage * PER_PAGE)
	);
	const startIndex = $derived(filteredRules.length === 0 ? 0 : (coveragePage - 1) * PER_PAGE + 1);
	const endIndex = $derived(Math.min(coveragePage * PER_PAGE, filteredRules.length));

	const coverageFilterLabel = $derived(
		coverageFilter === 'all'
			? 'All Rules'
			: coverageFilter === 'covered'
				? 'Covered'
				: 'Not Covered'
	);

	const severityFilterLabel = $derived(
		severityFilter === 'all'
			? 'All Severities'
			: severityFilter.charAt(0).toUpperCase() + severityFilter.slice(1)
	);

	onMount(async () => {
		try {
			data = await getRuleCoverage();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load rule coverage';
		} finally {
			loading = false;
		}
	});
</script>

<div class="space-y-6">
	<h1 class="text-2xl font-bold">Rule Coverage</h1>

	{#if error}
		<Alert.Root variant="destructive">
			<AlertCircleIcon class="h-4 w-4" />
			<Alert.Description>{error}</Alert.Description>
		</Alert.Root>
	{/if}

	{#if loading}
		<Skeleton class="h-44 w-full rounded-lg" />
		<div class="space-y-2">
			{#each Array(8) as _}
				<Skeleton class="h-12 w-full" />
			{/each}
		</div>
	{:else if data}
		<!-- Hero: coverage gauge + coverage-by-severity breakdown -->
		<div
			class="animate-fade-up stagger-1 flex flex-col gap-6 rounded-lg border bg-card p-5 sm:flex-row sm:items-center sm:gap-8"
		>
			<!-- Coverage gauge -->
			<div class="relative h-[132px] w-[132px] shrink-0 self-center">
				<svg class="h-full w-full -rotate-90" viewBox="0 0 132 132">
					<circle cx="66" cy="66" r={GAUGE_R} fill="none" class="stroke-muted" stroke-width="10" />
					<circle
						cx="66"
						cy="66"
						r={GAUGE_R}
						fill="none"
						class="stroke-primary transition-[stroke-dashoffset] duration-700 ease-out"
						stroke-width="10"
						stroke-linecap="round"
						stroke-dasharray={GAUGE_CIRC}
						stroke-dashoffset={gaugeOffset}
					/>
				</svg>
				<div class="absolute inset-0 flex flex-col items-center justify-center">
					<span class="font-mono text-4xl font-bold leading-none tabular-nums"
						>{Math.round(coveragePct)}</span
					>
					<span class="mt-1 font-mono text-xs text-muted-foreground">% covered</span>
				</div>
			</div>

			<div class="hidden self-stretch border-l sm:block"></div>

			<!-- Severity breakdown -->
			<div class="min-w-0 flex-1">
				<div class="flex flex-wrap items-baseline justify-between gap-x-6 gap-y-2">
					<span class="font-mono text-xs uppercase tracking-wider text-muted-foreground">
						Coverage by severity
					</span>
					<div class="flex flex-wrap items-baseline gap-x-5 gap-y-1">
						<span class="inline-flex items-baseline gap-1.5">
							<span class="font-mono text-base font-bold leading-none"
								>{data.summary.totalRules}</span
							>
							<span class="text-xs text-muted-foreground">rules</span>
						</span>
						<span class="inline-flex items-baseline gap-1.5">
							<span class="font-mono text-base font-bold leading-none text-status-success"
								>{data.summary.coveredRules}</span
							>
							<span class="text-xs text-muted-foreground">covered</span>
						</span>
						<span class="inline-flex items-baseline gap-1.5">
							<span class="font-mono text-base font-bold leading-none text-status-success"
								>{passingCount}</span
							>
							<span class="text-xs text-muted-foreground">passing</span>
						</span>
						<span class="inline-flex items-baseline gap-1.5">
							<span class="font-mono text-base font-bold leading-none text-status-error"
								>{failingCount}</span
							>
							<span class="text-xs text-muted-foreground">failing</span>
						</span>
					</div>
				</div>

				<div class="mt-3 space-y-2">
					{#each severityBreakdown as row}
						<div class="grid grid-cols-[80px_1fr_auto] items-center gap-3">
							<span
								class="inline-flex items-center gap-1.5 font-mono text-xs font-semibold uppercase tracking-wide {sevClass(
									row.severity
								).text}"
							>
								<span class="h-1.5 w-1.5 rounded-sm {sevClass(row.severity).bar}"></span>
								{row.severity}
							</span>
							<div class="h-2 overflow-hidden rounded-full bg-muted">
								<div
									class="h-full rounded-full transition-all duration-700 ease-out {sevClass(
										row.severity
									).bar}"
									style="width: {row.pct}%"
								></div>
							</div>
							<span class="font-mono text-xs whitespace-nowrap text-muted-foreground">
								<span class="font-medium text-foreground">{row.covered}</span>/{row.total} · {Math.round(
									row.pct
								)}%
							</span>
						</div>
					{/each}
				</div>
			</div>
		</div>

		<!-- Actionable gap callout -->
		{#if highSevGaps > 0}
			<div
				class="animate-fade-up stagger-2 flex items-center gap-3 rounded-lg border border-status-error/30 bg-status-error/[0.07] px-4 py-3 text-sm"
			>
				<ShieldAlertIcon class="h-4 w-4 shrink-0 text-status-error" />
				<span>
					<span class="font-mono font-semibold">{highSevGaps}</span>
					critical &amp; high-severity rule{highSevGaps === 1 ? '' : 's'} have no simulation coverage
					— your most important gaps.
				</span>
			</div>
		{/if}

		<!-- Filter bar -->
		<div class="flex flex-wrap items-end gap-4">
			<div class="flex flex-col gap-1.5">
				<label for="rule-search" class="text-xs font-medium text-muted-foreground">Search</label>
				<Input
					id="rule-search"
					placeholder="Search rules or tags…"
					class="h-9 w-[260px]"
					bind:value={search}
				/>
			</div>

			<div class="flex flex-col gap-1.5">
				<span class="text-xs font-medium text-muted-foreground">Coverage</span>
				<Select.Root type="single" bind:value={coverageFilter}>
					<Select.Trigger class="h-9 w-[150px]">{coverageFilterLabel}</Select.Trigger>
					<Select.Content>
						<Select.Item value="all" label="All Rules" />
						<Select.Item value="covered" label="Covered" />
						<Select.Item value="uncovered" label="Not Covered" />
					</Select.Content>
				</Select.Root>
			</div>

			<div class="flex flex-col gap-1.5">
				<span class="text-xs font-medium text-muted-foreground">Severity</span>
				<Select.Root type="single" bind:value={severityFilter}>
					<Select.Trigger class="h-9 w-[150px]">{severityFilterLabel}</Select.Trigger>
					<Select.Content>
						<Select.Item value="all" label="All Severities" />
						<Select.Item value="critical" label="Critical" />
						<Select.Item value="high" label="High" />
						<Select.Item value="medium" label="Medium" />
						<Select.Item value="low" label="Low" />
					</Select.Content>
				</Select.Root>
			</div>

			<span class="ml-auto self-center text-sm text-muted-foreground">
				{filteredRules.length} rule{filteredRules.length === 1 ? '' : 's'}
			</span>
		</div>

		<!-- Rules table -->
		<div class="animate-fade-up stagger-3 overflow-hidden rounded-lg border">
			<Table.Root>
				<Table.Header class="bg-muted">
					<Table.Row>
						<Table.Head class="w-8"></Table.Head>
						<Table.Head>
							<Button
								variant="ghost"
								size="sm"
								class="-ml-3 h-8"
								onclick={() => toggleSort('name')}
							>
								Rule Name
								<ArrowUpDownIcon data-icon="inline-end" />
							</Button>
						</Table.Head>
						<Table.Head>
							<Button
								variant="ghost"
								size="sm"
								class="-ml-3 h-8"
								onclick={() => toggleSort('severity')}
							>
								Severity
								<ArrowUpDownIcon data-icon="inline-end" />
							</Button>
						</Table.Head>
						<Table.Head>Tags</Table.Head>
						<Table.Head>Coverage</Table.Head>
						<Table.Head>Scenarios</Table.Head>
						<Table.Head>Last Result</Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each pagedRules as rule (rule.ruleId)}
						<Table.Row
							class={rule.covered ? 'cursor-pointer transition-colors hover:bg-accent/50' : ''}
							onclick={() => rule.covered && toggleRow(rule.ruleId)}
						>
							<Table.Cell class="w-8 border-l-2 {sevClass(rule.severity).accent}">
								{#if rule.covered}
									{#if expandedRows.has(rule.ruleId)}
										<ChevronDownIcon class="h-4 w-4 text-muted-foreground" />
									{:else}
										<ChevronRightIcon class="h-4 w-4 text-muted-foreground" />
									{/if}
								{/if}
							</Table.Cell>
							<Table.Cell class="font-medium">{rule.name}</Table.Cell>
							<Table.Cell>
								<span
									class="inline-flex h-5 items-center rounded-md border px-1.5 font-mono text-[0.65rem] font-semibold uppercase tracking-wide {sevClass(
										rule.severity
									).chip}"
								>
									{rule.severity}
								</span>
							</Table.Cell>
							<Table.Cell>
								<div class="flex flex-wrap gap-1">
									{#each rule.tags.slice(0, 3) as tag}
										<Badge variant="outline" class="text-xs">{tag}</Badge>
									{/each}
									{#if rule.tags.length > 3}
										<Badge variant="outline" class="text-xs">+{rule.tags.length - 3}</Badge>
									{/if}
								</div>
							</Table.Cell>
							<Table.Cell>
								{#if rule.covered}
									<span
										class="inline-flex items-center gap-1.5 font-mono text-xs font-medium text-status-success"
									>
										<span class="h-1.5 w-1.5 rounded-full bg-status-success"></span>
										Covered
									</span>
								{:else}
									<span
										class="inline-flex items-center gap-1.5 font-mono text-xs text-muted-foreground"
									>
										<span class="h-1.5 w-1.5 rounded-full bg-muted-foreground/40"></span>
										Not covered
									</span>
								{/if}
							</Table.Cell>
							<Table.Cell
								class="font-mono text-xs {rule.scenarios.length === 0
									? 'text-muted-foreground'
									: ''}"
							>
								{rule.scenarios.length}
							</Table.Cell>
							<Table.Cell>
								{#if rule.lastResult}
									<a
										href="/assessments/{rule.lastResult.runId}"
										class="inline-flex items-center gap-2"
										onclick={(e: Event) => e.stopPropagation()}
									>
										<span
											class="inline-flex h-5 items-center rounded-md border px-1.5 font-mono text-[0.65rem] font-semibold {rule
												.lastResult.passed
												? 'border-status-success/25 bg-status-success/10 text-status-success'
												: 'border-status-error/30 bg-status-error/10 text-status-error'}"
										>
											{rule.lastResult.passed ? 'PASS' : 'FAIL'}
										</span>
										<Tooltip.Root>
											<Tooltip.Trigger class="cursor-default text-xs text-muted-foreground">
												{formatRelativeTime(rule.lastResult.timestamp)}
											</Tooltip.Trigger>
											<Tooltip.Content>{formatTime(rule.lastResult.timestamp)}</Tooltip.Content>
										</Tooltip.Root>
									</a>
								{:else}
									<span class="text-sm text-muted-foreground">--</span>
								{/if}
							</Table.Cell>
						</Table.Row>

						<!-- Expanded scenario sub-rows -->
						{#if expandedRows.has(rule.ruleId)}
							{#each rule.scenarios as scenario}
								<Table.Row class="bg-muted/30">
									<Table.Cell class="border-l-2 {sevClass(rule.severity).accent}"></Table.Cell>
									<Table.Cell colspan={6} class="pl-8">
										<div class="flex items-center gap-3 text-sm">
											<a
												href="/scenarios/{scenario.scenarioId}"
												class="font-medium text-primary hover:underline"
											>
												{scenario.scenarioName}
											</a>
											{#if scenario.packName && scenario.simulationId}
												<span class="font-mono text-xs text-muted-foreground">
													{scenario.packName}.{scenario.simulationId}
												</span>
											{/if}
										</div>
									</Table.Cell>
								</Table.Row>
							{/each}
						{/if}
					{/each}

					{#if filteredRules.length === 0}
						<Table.Row>
							<Table.Cell colspan={7} class="py-8 text-center text-muted-foreground">
								No rules match the current filters.
							</Table.Cell>
						</Table.Row>
					{/if}
				</Table.Body>
			</Table.Root>
		</div>

		{#if filteredRules.length > 0}
			<div class="flex flex-wrap items-center justify-between gap-3 pt-1">
				<div class="text-sm text-muted-foreground">
					Showing <span class="font-medium text-foreground">{startIndex}–{endIndex}</span>
					of <span class="font-medium text-foreground">{filteredRules.length}</span>
				</div>

				{#if filteredRules.length > PER_PAGE}
					<Pagination.Root
						count={filteredRules.length}
						perPage={PER_PAGE}
						bind:page={coveragePage}
						class="mx-0 w-auto"
					>
						{#snippet children({ pages, currentPage })}
							<Pagination.Content>
								<Pagination.Item>
									<Pagination.PrevButton />
								</Pagination.Item>
								{#each pages as p (p.key)}
									{#if p.type === 'ellipsis'}
										<Pagination.Item>
											<Pagination.Ellipsis />
										</Pagination.Item>
									{:else}
										<Pagination.Item>
											<Pagination.Link page={p} isActive={currentPage === p.value}>
												{p.value}
											</Pagination.Link>
										</Pagination.Item>
									{/if}
								{/each}
								<Pagination.Item>
									<Pagination.NextButton />
								</Pagination.Item>
							</Pagination.Content>
						{/snippet}
					</Pagination.Root>
				{/if}
			</div>
		{/if}
	{/if}
</div>
