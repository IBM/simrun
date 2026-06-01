<script lang="ts">
	import { onMount } from 'svelte';
	import { getRuleCoverage } from '$lib/api/client';
	import type { CoverageResponse, RuleCoverageEntry } from '$lib/types';
	import * as Table from '$lib/components/ui/table/index.js';
	import * as Alert from '$lib/components/ui/alert/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import * as Select from '$lib/components/ui/select/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Skeleton } from '$lib/components/ui/skeleton/index.js';
	import AlertCircleIcon from '@lucide/svelte/icons/alert-circle';
	import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import ArrowUpDownIcon from '@lucide/svelte/icons/arrow-up-down';
	import ShieldCheckIcon from '@lucide/svelte/icons/shield-check';
	import ShieldAlertIcon from '@lucide/svelte/icons/shield-alert';
	import CheckCircleIcon from '@lucide/svelte/icons/check-circle';

	let data: CoverageResponse | null = $state(null);
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

	const severityOrder: Record<string, number> = { critical: 0, high: 1, medium: 2, low: 3 };

	function severityVariant(
		severity: string
	): 'destructive' | 'default' | 'secondary' | 'outline' {
		switch (severity) {
			case 'critical':
				return 'destructive';
			case 'high':
				return 'default';
			case 'medium':
				return 'secondary';
			default:
				return 'outline';
		}
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

	let filteredRules = $derived.by(() => {
		if (!data) return [];
		let rules = data.rules;
		if (search) {
			const q = search.toLowerCase();
			rules = rules.filter(
				(r) =>
					r.name.toLowerCase().includes(q) ||
					r.tags.some((t) => t.toLowerCase().includes(q))
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

	let passingCount = $derived(
		data?.rules.filter((r) => r.lastResult?.passed === true).length ?? 0
	);
	let failingCount = $derived(
		data?.rules.filter((r) => r.lastResult?.passed === false).length ?? 0
	);

	let coverageFilterLabel = $derived(
		coverageFilter === 'all'
			? 'All Rules'
			: coverageFilter === 'covered'
				? 'Covered'
				: 'Not Covered'
	);

	let severityFilterLabel = $derived(
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
		<!-- Loading skeleton: 3 summary cards -->
		<div class="grid grid-cols-3 gap-4">
			{#each Array(3) as _}
				<Skeleton class="h-24 w-full rounded-lg" />
			{/each}
		</div>
		<!-- Loading skeleton: table rows -->
		<div class="space-y-2">
			{#each Array(8) as _}
				<Skeleton class="h-12 w-full" />
			{/each}
		</div>
	{:else if data}
		<!-- Summary cards -->
		<div class="grid grid-cols-3 gap-4">
			<Card.Root>
				<Card.Header class="pb-2">
					<Card.Description>Total Enabled Rules</Card.Description>
				</Card.Header>
				<Card.Content>
					<div class="flex items-center gap-2">
						<ShieldAlertIcon class="h-5 w-5 text-muted-foreground" />
						<span class="text-3xl font-bold">{data.summary.totalRules}</span>
					</div>
				</Card.Content>
			</Card.Root>

			<Card.Root>
				<Card.Header class="pb-2">
					<Card.Description>Covered by Simulations</Card.Description>
				</Card.Header>
				<Card.Content>
					<div class="flex items-center gap-2">
						<ShieldCheckIcon class="h-5 w-5 text-muted-foreground" />
						<span class="text-3xl font-bold">{data.summary.coveredRules}</span>
						<span class="text-sm text-muted-foreground">
							({data.summary.coveragePercent.toFixed(1)}%)
						</span>
					</div>
				</Card.Content>
			</Card.Root>

			<Card.Root>
				<Card.Header class="pb-2">
					<Card.Description>Last Results</Card.Description>
				</Card.Header>
				<Card.Content>
					<div class="flex items-center gap-4">
						<CheckCircleIcon class="h-5 w-5 text-muted-foreground" />
						<span class="text-xl font-semibold text-status-success">
							{passingCount} passing
						</span>
						<span class="text-xl font-semibold text-status-error">
							{failingCount} failing
						</span>
					</div>
				</Card.Content>
			</Card.Root>
		</div>

		<!-- Filter bar -->
		<div class="flex items-center gap-4">
			<div class="max-w-sm flex-1">
				<Input
					placeholder="Search rules or tags..."
					bind:value={search}
				/>
			</div>

			<Select.Root type="single" bind:value={coverageFilter}>
				<Select.Trigger class="w-[160px]">
					{coverageFilterLabel}
				</Select.Trigger>
				<Select.Content>
					<Select.Item value="all" label="All Rules" />
					<Select.Item value="covered" label="Covered" />
					<Select.Item value="uncovered" label="Not Covered" />
				</Select.Content>
			</Select.Root>

			<Select.Root type="single" bind:value={severityFilter}>
				<Select.Trigger class="w-[160px]">
					{severityFilterLabel}
				</Select.Trigger>
				<Select.Content>
					<Select.Item value="all" label="All Severities" />
					<Select.Item value="critical" label="Critical" />
					<Select.Item value="high" label="High" />
					<Select.Item value="medium" label="Medium" />
					<Select.Item value="low" label="Low" />
				</Select.Content>
			</Select.Root>

			<span class="text-sm text-muted-foreground">
				{filteredRules.length} rule{filteredRules.length === 1 ? '' : 's'}
			</span>
		</div>

		<!-- Rules table -->
		<div class="overflow-hidden rounded-lg border">
			<Table.Root>
				<Table.Header class="bg-muted">
					<Table.Row>
						<Table.Head class="w-10"></Table.Head>
						<Table.Head>
							<Button
								variant="ghost"
								size="sm"
								class="-ml-3 h-8"
								onclick={() => toggleSort('name')}
							>
								Rule Name
								<ArrowUpDownIcon class="ml-1 h-4 w-4" />
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
								<ArrowUpDownIcon class="ml-1 h-4 w-4" />
							</Button>
						</Table.Head>
						<Table.Head>Tags</Table.Head>
						<Table.Head>Coverage</Table.Head>
						<Table.Head>Scenarios</Table.Head>
						<Table.Head>Last Result</Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each filteredRules as rule (rule.ruleId)}
						<Table.Row
							class={rule.covered ? 'cursor-pointer hover:bg-accent/50 transition-colors' : ''}
							onclick={() => rule.covered && toggleRow(rule.ruleId)}
						>
							<Table.Cell class="w-10">
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
								<Badge variant={severityVariant(rule.severity)}>
									{rule.severity}
								</Badge>
							</Table.Cell>
							<Table.Cell>
								<div class="flex flex-wrap gap-1">
									{#each rule.tags.slice(0, 3) as tag}
										<Badge variant="outline" class="text-xs">{tag}</Badge>
									{/each}
									{#if rule.tags.length > 3}
										<Badge variant="outline" class="text-xs">
											+{rule.tags.length - 3}
										</Badge>
									{/if}
								</div>
							</Table.Cell>
							<Table.Cell>
								{#if rule.covered}
									<Badge variant="success">Covered</Badge>
								{:else}
									<Badge variant="secondary">Not Covered</Badge>
								{/if}
							</Table.Cell>
							<Table.Cell>
								{rule.scenarios.length}
							</Table.Cell>
							<Table.Cell>
								{#if rule.lastResult}
									<a href="/assessments/{rule.lastResult.runId}" class="inline-block">
										{#if rule.lastResult.passed}
											<Badge variant="success">Passed</Badge>
										{:else}
											<Badge variant="destructive">Failed</Badge>
										{/if}
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
									<Table.Cell></Table.Cell>
									<Table.Cell colspan={6} class="pl-8">
										<div class="flex items-center gap-4 text-sm">
											<a
												href="/scenarios/{scenario.scenarioId}"
												class="font-medium text-primary hover:underline"
											>
												{scenario.scenarioName}
											</a>
											{#if scenario.packName && scenario.simulationId}
												<span class="text-muted-foreground">
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
							<Table.Cell colspan={7} class="text-center py-8 text-muted-foreground">
								No rules match the current filters.
							</Table.Cell>
						</Table.Row>
					{/if}
				</Table.Body>
			</Table.Root>
		</div>
	{/if}
</div>
