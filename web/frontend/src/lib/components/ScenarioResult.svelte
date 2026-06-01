<script lang="ts">
	import type { RunLogEntry } from '$lib/types';
	import type { ScenarioEntry } from '$lib/stores/scenario-tracker.svelte';
	import * as Collapsible from '$lib/components/ui/collapsible/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import RunLog from './RunLog.svelte';
	import LoaderCircleIcon from '@lucide/svelte/icons/loader-circle';
	import DownloadIcon from '@lucide/svelte/icons/download';
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
</script>

<Collapsible.Root bind:open class="min-w-0 overflow-hidden">
	<Collapsible.Trigger class="w-full">
		<div
			class="flex items-center justify-between rounded-md border border-border p-3 hover:bg-accent/50 transition-colors"
		>
			<div class="flex items-center gap-3 min-w-0">
				<span class="text-sm font-medium truncate">{entry.name}</span>
				{#if result?.simulationId}
					<span class="text-xs text-muted-foreground font-mono truncate">{result.simulationId}</span
					>
				{/if}
			</div>
			<div class="flex items-center gap-3 shrink-0">
				{#if entry.status === 'completed' && result}
					<span class="text-xs text-muted-foreground">{result.executorName}</span>
					{#if result.discoveredAlerts}
						<span class="text-xs text-muted-foreground">
							{result.discoveredAlerts.length} alert{result.discoveredAlerts.length !== 1 ? 's' : ''} discovered
						</span>
						<Badge variant="secondary">explore</Badge>
					{:else if assertionCounts}
						<span class="text-xs text-muted-foreground">
							{assertionCounts.passed}/{assertionCounts.total} assertions
						</span>
					{/if}
					{#if matcherNames.length > 0}
						<span class="text-xs text-muted-foreground">{matcherNames.join(', ')}</span>
					{/if}
					<span class="text-xs text-muted-foreground">{result.durationSecs.toFixed(1)}s</span>
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
			class="rounded-b-md border border-t-0 border-border p-4 space-y-3 bg-muted/30 min-w-0 overflow-hidden"
		>
			{#if entry.status === 'completed' && result}
				{#if result.errorMessage}
					<div>
						<span class="text-xs font-medium text-muted-foreground">Error</span>
						<p class="text-sm text-destructive mt-1">{result.errorMessage}</p>
					</div>
				{/if}

				<div class="grid grid-cols-2 gap-3 text-sm">
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
						<p>
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
						<div class="flex items-center gap-2 mt-1">
							<span class="text-sm">{result.collectedDocCount} documents</span>
							<a href={getCollectedLogsUrl(result.id)} download>
								<Button variant="outline" size="sm" class="h-7 px-2">
									<DownloadIcon class="h-3 w-3 mr-1" />
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
						<div class="mt-1 space-y-1">
							{#each result.discoveredAlerts as alert}
								<div class="flex items-center gap-2 text-sm">
									<span class="text-success">FOUND</span>
									{#if alert.severity}
										<span class="text-muted-foreground">[{alert.severity}]</span>
									{/if}
									<span>{alert.ruleName}</span>
								</div>
							{/each}
						</div>
					</div>
				{:else if result.discoveredAlerts && result.discoveredAlerts.length === 0}
					<div>
						<span class="text-xs font-medium text-muted-foreground">Discovered Alerts</span>
						<p class="text-sm text-muted-foreground mt-1">No matching alerts found during explore window.</p>
					</div>
				{/if}

				{#if result.assertions && result.assertions.length > 0}
					<div>
						<span class="text-xs font-medium text-muted-foreground">Assertions</span>
						<div class="mt-1 space-y-1">
							{#each result.assertions as assertion}
								<div class="flex items-center gap-2 text-sm">
									<span class={assertion.passed ? 'text-success' : 'text-destructive'}>
										{assertion.passed ? 'PASS' : 'FAIL'}
									</span>
									<span class="text-muted-foreground">[{assertion.matcherType}]</span>
									<span>{assertion.alertName}</span>
								</div>
							{/each}
						</div>
					</div>
				{/if}

				{#if result.metadata?.description}
					<div>
						<span class="text-xs font-medium text-muted-foreground">Description</span>
						<p class="text-sm mt-1">{result.metadata.description}</p>
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
