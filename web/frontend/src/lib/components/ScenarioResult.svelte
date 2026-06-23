<script lang="ts">
	import type { RunLogEntry } from '$lib/types';
	import type { ScenarioEntry } from '$lib/stores/scenario-tracker.svelte';
	import * as Collapsible from '$lib/components/ui/collapsible/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import RunLog from './RunLog.svelte';
	import LoaderCircleIcon from '@lucide/svelte/icons/loader-circle';
	import DownloadIcon from '@lucide/svelte/icons/download';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import { getCollectedLogsUrl } from '$lib/api/client';

	let { entry, logs = [] }: { entry: ScenarioEntry; logs?: RunLogEntry[] } = $props();
	let open = $state(false);

	let result = $derived(entry.result);

	let matcherNames = $derived(
		result?.assertions && result.assertions.length > 0
			? [...new Set(result.assertions.map((a) => a.matcherType))]
			: []
	);

	let assertionCounts = $derived.by(() => {
		if (!result?.assertions || result.assertions.length === 0) return null;
		const passed = result.assertions.filter((a) => a.passed).length;
		return { passed, total: result.assertions.length };
	});

	const phaseLabels: Record<string, string> = {
		queued: 'Queued',
		warmup: 'Warming up',
		detonating: 'Detonating',
		matching: 'Matching',
		exploring: 'Exploring',
		collecting: 'Collecting',
		cleanup: 'Cleanup'
	};

	// Timeline node accent driven by terminal status.
	const nodeState = $derived(
		entry.status === 'completed' && result ? (result.isSuccess ? 'pass' : 'fail') : entry.status
	);
</script>

<div class="relative">
	<!-- Timeline marker sitting on the rail -->
	<span
		class="absolute -left-8 top-3.5 z-10 grid h-5 w-5 place-items-center rounded-full border-2 bg-background
			{nodeState === 'pass'
			? 'border-status-success/50'
			: nodeState === 'fail'
				? 'border-status-error/60'
				: nodeState === 'running'
					? 'border-status-processing/60 animate-indicator-pulse'
					: 'border-border'}"
	>
		<span
			class="h-2 w-2 rounded-full
				{nodeState === 'pass'
				? 'bg-status-success'
				: nodeState === 'fail'
					? 'bg-status-error'
					: nodeState === 'running'
						? 'bg-status-processing'
						: 'bg-muted-foreground/40'}"
		></span>
	</span>

	<Collapsible.Root bind:open class="min-w-0 overflow-hidden">
		<Collapsible.Trigger class="w-full">
			<div
				class="flex items-center justify-between gap-4 rounded-md border border-l-2 border-border p-3 text-left transition-colors hover:bg-accent/50
					{open ? 'rounded-b-none' : ''}
					{nodeState === 'fail' ? 'border-l-status-error/70' : 'border-l-border'}"
			>
				<div class="flex min-w-0 items-center gap-2">
					<ChevronRightIcon
						class="h-3.5 w-3.5 shrink-0 text-muted-foreground transition-transform duration-200 {open
							? 'rotate-90'
							: ''}"
					/>
					<div class="flex min-w-0 flex-col gap-0.5">
						<span class="truncate text-sm font-medium">{entry.name}</span>
						<span
							class="flex flex-wrap items-center gap-x-2 font-mono text-xs text-muted-foreground"
						>
							{#if result}
								<span>{result.executorName}</span>
								{#if result.simulationId}
									<span class="text-muted-foreground/50">·</span>
									<span class="truncate">{result.simulationId}</span>
								{/if}
								{#if matcherNames.length > 0}
									<span class="text-muted-foreground/50">·</span>
									<span>{matcherNames.join(', ')}</span>
								{/if}
							{/if}
						</span>
					</div>
				</div>

				<div class="flex shrink-0 items-center gap-3">
					{#if entry.status === 'completed' && result}
						{#if result.discoveredAlerts}
							<span class="font-mono text-xs text-muted-foreground">
								{result.discoveredAlerts.length} alert{result.discoveredAlerts.length !== 1
									? 's'
									: ''}
							</span>
							<Badge variant="secondary">explore</Badge>
						{:else if assertionCounts}
							<!-- Mini assertion bar: one tick per assertion, capped before it gets noisy -->
							{#if assertionCounts.total <= 8}
								<div class="hidden items-center gap-1.5 sm:flex">
									<div class="flex gap-[2px]">
										{#each result.assertions ?? [] as a}
											<span
												class="h-4 w-[6px] rounded-[2px] {a.passed
													? 'bg-status-success'
													: 'bg-status-error'}"
											></span>
										{/each}
									</div>
									<span class="font-mono text-xs text-muted-foreground"
										>{assertionCounts.passed}/{assertionCounts.total}</span
									>
								</div>
							{:else}
								<span class="font-mono text-xs text-muted-foreground"
									>{assertionCounts.passed}/{assertionCounts.total} assertions</span
								>
							{/if}
						{/if}
						<span class="font-mono text-xs text-muted-foreground tabular-nums"
							>{result.durationSecs.toFixed(1)}s</span
						>
						<Badge variant={result.isSuccess ? 'success' : 'destructive'}>
							{result.isSuccess ? 'passed' : 'failed'}
						</Badge>
					{:else if entry.status === 'running' && entry.phase}
						<Badge variant="secondary" class="flex items-center gap-1.5">
							<LoaderCircleIcon class="h-3 w-3 animate-spin" />
							{phaseLabels[entry.phase] || entry.phase}
						</Badge>
					{:else if entry.status === 'pending'}
						<Badge variant="outline">Pending</Badge>
					{/if}
				</div>
			</div>
		</Collapsible.Trigger>
		<Collapsible.Content>
			<div
				class="min-w-0 space-y-3 overflow-hidden rounded-b-md border border-t-0 border-border bg-muted/30 p-4"
			>
				{#if entry.status === 'completed' && result}
					{#if result.errorMessage}
						<div>
							<span class="text-xs font-medium text-muted-foreground">Error</span>
							<p class="mt-1 text-sm text-status-error">{result.errorMessage}</p>
						</div>
					{/if}

					<div class="grid grid-cols-2 gap-3 text-sm sm:grid-cols-3">
						<div>
							<span class="text-xs font-medium text-muted-foreground">Executor</span>
							<p>{result.executorName} ({result.executorType})</p>
						</div>
						<div>
							<span class="text-xs font-medium text-muted-foreground">Execution ID</span>
							<p class="font-mono text-xs">{result.executionId}</p>
						</div>
						{#if result.simulationId}
							<div>
								<span class="text-xs font-medium text-muted-foreground">Simulation ID</span>
								<p class="font-mono text-xs">{result.simulationId}</p>
							</div>
						{/if}
						<div>
							<span class="text-xs font-medium text-muted-foreground">Duration</span>
							<p class="tabular-nums">
								{result.durationSecs.toFixed(1)}s (matching: {result.matchingDurSecs.toFixed(1)}s)
							</p>
						</div>
						<div>
							<span class="text-xs font-medium text-muted-foreground">Executed</span>
							<p>{new Date(result.timeExecuted).toLocaleString()}</p>
						</div>
					</div>

					{#if result.collectedDocCount && result.collectedDocCount > 0}
						<div>
							<span class="text-xs font-medium text-muted-foreground">Collected Logs</span>
							<div class="mt-1 flex items-center gap-2">
								<span class="text-sm">{result.collectedDocCount} documents</span>
								<a href={getCollectedLogsUrl(result.id)} download>
									<Button variant="outline" size="sm" class="h-7 px-2">
										<DownloadIcon data-icon="inline-start" />
										Download NDJSON
									</Button>
								</a>
							</div>
						</div>
					{/if}

					{#if result.discoveredAlerts && result.discoveredAlerts.length > 0}
						<div>
							<span class="text-xs font-medium text-muted-foreground">
								Discovered Alerts ({result.discoveredAlerts.length})
							</span>
							<div class="mt-1.5 space-y-1">
								{#each result.discoveredAlerts as alert}
									<div class="flex items-center gap-2 text-sm">
										<span class="font-mono text-xs font-semibold text-status-success">FOUND</span>
										{#if alert.severity}
											<span class="font-mono text-xs text-status-warning">[{alert.severity}]</span>
										{/if}
										<span>{alert.ruleName}</span>
									</div>
								{/each}
							</div>
						</div>
					{:else if result.discoveredAlerts && result.discoveredAlerts.length === 0}
						<div>
							<span class="text-xs font-medium text-muted-foreground">Discovered Alerts</span>
							<p class="mt-1 text-sm text-muted-foreground">
								No matching alerts found during explore window.
							</p>
						</div>
					{/if}

					{#if result.assertions && result.assertions.length > 0}
						<div>
							<span class="text-xs font-medium text-muted-foreground">Assertions</span>
							<div class="mt-1.5 space-y-1">
								{#each result.assertions as assertion}
									<div class="flex items-center gap-2 text-sm">
										<span
											class="font-mono text-xs font-semibold {assertion.passed
												? 'text-status-success'
												: 'text-status-error'}"
										>
											{assertion.passed ? 'PASS' : 'MISSED'}
										</span>
										<span class="font-mono text-xs text-muted-foreground"
											>[{assertion.matcherType}]</span
										>
										<span>{assertion.alertName}</span>
									</div>
								{/each}
							</div>
						</div>
					{/if}

					{#if result.metadata?.description}
						<div>
							<span class="text-xs font-medium text-muted-foreground">Description</span>
							<p class="mt-1 text-sm">{result.metadata.description}</p>
						</div>
					{/if}
				{:else if entry.status === 'running' && entry.phase}
					<div class="flex items-center gap-2 text-sm text-muted-foreground">
						<LoaderCircleIcon class="h-4 w-4 animate-spin" />
						<span>{phaseLabels[entry.phase] || entry.phase}...</span>
					</div>
				{:else if entry.status === 'pending'}
					<div class="text-sm text-muted-foreground">Waiting to start...</div>
				{/if}

				{#if logs.length > 0}
					<div>
						<span class="text-xs font-medium text-muted-foreground">Logs ({logs.length})</span>
						<div class="mt-2">
							<RunLog entries={logs} class="!h-[250px]" />
						</div>
					</div>
				{/if}
			</div>
		</Collapsible.Content>
	</Collapsible.Root>
</div>
