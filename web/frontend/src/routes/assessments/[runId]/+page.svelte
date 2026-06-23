<script lang="ts">
	import { onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import * as Alert from '$lib/components/ui/alert/index.js';
	import * as Breadcrumb from '$lib/components/ui/breadcrumb/index.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import * as Tabs from '$lib/components/ui/tabs/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Skeleton } from '$lib/components/ui/skeleton/index.js';
	import ScenarioResultComponent from '$lib/components/ScenarioResult.svelte';
	import RunLog from '$lib/components/RunLog.svelte';
	import { currentRun, loadRun } from '$lib/stores/runs';
	import { websocket } from '$lib/stores/websocket';
	import { getRunLogs, getScenario, deleteRun } from '$lib/api/client';
	import { ScenarioTracker } from '$lib/stores/scenario-tracker.svelte';
	import { statusVariant, formatDuration, formatUserEmail } from '$lib/utils/format';
	import type { WSMessage, RunLogEntry } from '$lib/types';
	import * as Tooltip from '$lib/components/ui/tooltip/index.js';
	import UserIcon from '@lucide/svelte/icons/user';
	import CalendarClockIcon from '@lucide/svelte/icons/calendar-clock';
	import FileTextIcon from '@lucide/svelte/icons/file-text';
	import TrashIcon from '@lucide/svelte/icons/trash-2';
	import ClockIcon from '@lucide/svelte/icons/clock';
	import LoaderCircleIcon from '@lucide/svelte/icons/loader-circle';

	let loading = $state(true);
	let error = $state('');
	let tracker = new ScenarioTracker();
	let deleteDialogOpen = $state(false);
	let deleting = $state(false);
	let pollTimer: ReturnType<typeof setInterval> | null = null;
	let scenarioName = $state<string | null>(null);

	const runId = $derived($page.params.runId!);

	const trackerSucceeded = $derived(
		Object.values(tracker.entries).filter((e) => e.status === 'completed' && e.result?.isSuccess)
			.length
	);
	const trackerFailed = $derived(
		Object.values(tracker.entries).filter((e) => e.status === 'completed' && !e.result?.isSuccess)
			.length
	);
	const total = $derived($currentRun?.total ?? 0);
	const succeededPct = $derived(total > 0 ? (trackerSucceeded / total) * 100 : 0);
	const failedPct = $derived(total > 0 ? (trackerFailed / total) * 100 : 0);
	const pendingPct = $derived(Math.max(0, 100 - succeededPct - failedPct));

	// Pass-rate gauge ring geometry.
	const GAUGE_R = 52;
	const GAUGE_CIRC = 2 * Math.PI * GAUGE_R;
	const gaugeOffset = $derived(GAUGE_CIRC * (1 - succeededPct / 100));
	const passPct = $derived(Math.round(succeededPct));

	function stopPolling() {
		if (pollTimer) {
			clearInterval(pollTimer);
			pollTimer = null;
		}
	}

	async function pollRun() {
		try {
			await loadRun(runId);
			if ($currentRun?.scenarioResults) {
				tracker.setScenarios($currentRun.scenarioResults);
			}
			if ($currentRun?.status === 'completed') {
				stopPolling();
			}
		} catch {
			// Silently retry on next interval
		}
	}

	$effect(() => {
		const id = runId;
		loading = true;
		error = '';
		scenarioName = null;
		tracker.reset();
		stopPolling();

		websocket.subscribeToRun(id);

		(async () => {
			try {
				await loadRun(id);
				tracker.setLogs(await getRunLogs(id));
				if ($currentRun?.scenarioResults) {
					tracker.setScenarios($currentRun.scenarioResults);
				}
				if ($currentRun?.scenarioId) {
					try {
						const scenario = await getScenario($currentRun.scenarioId);
						scenarioName = scenario.name;
					} catch {
						// Scenario may have been deleted
					}
				}
				if ($currentRun?.status === 'running') {
					pollTimer = setInterval(pollRun, 5000);
				}
			} catch (e) {
				error = e instanceof Error ? e.message : 'Failed to load assessment';
			} finally {
				loading = false;
			}
		})();
	});

	const unsubscribe = websocket.subscribe((msg: WSMessage | null) => {
		if (!msg) return;
		if (msg.type === 'scenario_log') {
			tracker.addLog(msg.data as RunLogEntry);
		}
	});

	onDestroy(() => {
		unsubscribe();
		stopPolling();
	});

	async function handleDelete() {
		deleting = true;
		error = '';
		try {
			await deleteRun(runId);
			goto('/assessments');
		} catch (e) {
			error = e instanceof Error ? e.message : 'Delete failed';
			deleting = false;
		}
	}
</script>

<div class="space-y-6">
	{#if loading}
		<Skeleton class="h-5 w-48" />
		<div class="rounded-lg border bg-card p-5 space-y-3">
			<div class="flex items-center gap-2.5">
				<Skeleton class="h-5 w-24" />
				<Skeleton class="h-5 w-16 rounded-full" />
			</div>
			<Skeleton class="h-4 w-64" />
			<Skeleton class="h-1.5 w-full rounded-full" />
		</div>
		<div class="space-y-3">
			{#each Array(3) as _}
				<Skeleton class="h-14 w-full" />
			{/each}
		</div>
	{:else if error}
		<Alert.Root variant="destructive">
			<Alert.Description>{error}</Alert.Description>
		</Alert.Root>
	{:else if $currentRun}
		<Breadcrumb.Root class="animate-fade-up stagger-1">
			<Breadcrumb.List>
				<Breadcrumb.Item>
					<Breadcrumb.Link href="/assessments">Assessments</Breadcrumb.Link>
				</Breadcrumb.Item>
				<Breadcrumb.Separator />
				<Breadcrumb.Item>
					<Breadcrumb.Page class="font-mono">{$currentRun.id.slice(0, 8)}</Breadcrumb.Page>
				</Breadcrumb.Item>
			</Breadcrumb.List>
		</Breadcrumb.Root>

		<div
			class="animate-fade-up stagger-2 flex flex-col gap-6 rounded-lg border bg-card p-5 sm:flex-row sm:items-center sm:gap-7"
		>
			<!-- Pass-rate gauge -->
			<div class="relative h-[128px] w-[128px] shrink-0 self-center sm:self-auto">
				<svg class="h-full w-full -rotate-90" viewBox="0 0 128 128">
					<circle cx="64" cy="64" r={GAUGE_R} fill="none" class="stroke-muted" stroke-width="10" />
					<circle
						cx="64"
						cy="64"
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
					<span class="font-mono text-4xl font-bold leading-none tabular-nums">{passPct}</span>
					<span class="mt-1 font-mono text-xs text-muted-foreground">% passed</span>
				</div>
			</div>

			<!-- Identity + detection rail + tallies -->
			<div class="min-w-0 flex-1">
				<div class="flex items-start justify-between gap-4">
					<div class="min-w-0">
						<div class="flex flex-wrap items-center gap-2.5">
							<h1 class="font-mono text-lg font-bold leading-none">{$currentRun.id.slice(0, 8)}</h1>
							<Badge variant={statusVariant($currentRun.status)}>{$currentRun.status}</Badge>
							{#if $currentRun.scenarioType}
								<Badge
									variant={$currentRun.scenarioType === 'explore'
										? 'secondary'
										: $currentRun.scenarioType === 'collect'
											? 'outline'
											: 'default'}
								>
									{$currentRun.scenarioType}
								</Badge>
							{/if}
						</div>

						<div
							class="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-muted-foreground"
						>
							{#if $currentRun.scenarioId}
								<span class="inline-flex items-center gap-1">
									<FileTextIcon class="h-3.5 w-3.5 shrink-0" />
									{#if scenarioName}
										<a
											href={`/scenarios/${$currentRun.scenarioId}`}
											class="max-w-[240px] truncate text-foreground underline-offset-2 hover:underline"
										>
											{scenarioName}
										</a>
									{:else}
										<span class="font-mono">{$currentRun.scenarioId.slice(0, 8)}</span>
									{/if}
								</span>
							{/if}
							{#if $currentRun.scheduleName}
								<span class="inline-flex items-center gap-1">
									<CalendarClockIcon class="h-3.5 w-3.5 shrink-0" />
									<span>{$currentRun.scheduleName}</span>
								</span>
							{:else if $currentRun.createdBy && $currentRun.createdBy !== 'anonymous'}
								<span class="inline-flex items-center gap-1">
									<UserIcon class="h-3.5 w-3.5 shrink-0" />
									<Tooltip.Root>
										<Tooltip.Trigger class="cursor-default">
											{formatUserEmail($currentRun.createdBy)}
										</Tooltip.Trigger>
										<Tooltip.Content>{$currentRun.createdBy}</Tooltip.Content>
									</Tooltip.Root>
								</span>
							{/if}
							<span class="inline-flex items-center gap-1">
								<ClockIcon class="h-3.5 w-3.5 shrink-0" />
								<span class="font-mono text-xs">
									{formatDuration($currentRun.startTime, $currentRun.endTime)}
								</span>
							</span>
						</div>
					</div>

					<Button
						variant="ghost"
						size="icon"
						class="h-8 w-8 shrink-0 text-muted-foreground hover:text-destructive"
						onclick={() => (deleteDialogOpen = true)}
					>
						<TrashIcon class="h-4 w-4" />
					</Button>
				</div>

				<!-- Segmented pass / fail / pending rail -->
				<div class="mt-4 flex h-2.5 overflow-hidden rounded-full bg-muted">
					<div
						class="h-full bg-status-success transition-all duration-500 ease-out"
						style="width: {succeededPct}%"
					></div>
					<div
						class="h-full bg-status-error transition-all duration-500 ease-out"
						style="width: {failedPct}%"
					></div>
					{#if pendingPct > 0}
						<div
							class="h-full bg-status-processing/40 transition-all duration-500 ease-out"
							style="width: {pendingPct}%"
						></div>
					{/if}
				</div>

				<div class="mt-3 flex flex-wrap items-center gap-x-6 gap-y-1">
					<span class="inline-flex items-baseline gap-1.5">
						<span class="font-mono text-xl font-bold leading-none text-status-success"
							>{trackerSucceeded}</span
						>
						<span class="text-xs text-muted-foreground">passed</span>
					</span>
					<span class="inline-flex items-baseline gap-1.5">
						<span class="font-mono text-xl font-bold leading-none text-status-error"
							>{trackerFailed}</span
						>
						<span class="text-xs text-muted-foreground">failed</span>
					</span>
					<span class="inline-flex items-baseline gap-1.5">
						<span class="font-mono text-xl font-bold leading-none">{total}</span>
						<span class="text-xs text-muted-foreground">scenarios</span>
					</span>
					{#if $currentRun.status === 'running'}
						<span class="inline-flex items-center gap-1.5 text-xs text-status-processing">
							<LoaderCircleIcon class="h-3.5 w-3.5 animate-spin" />
							running
						</span>
					{/if}
				</div>
			</div>
		</div>

		<Tabs.Root value="results" class="animate-fade-up stagger-4">
			<Tabs.List>
				<Tabs.Trigger value="results">Results ({tracker.sortedEntries.length})</Tabs.Trigger>
				<Tabs.Trigger value="logs">Logs ({tracker.logs.length})</Tabs.Trigger>
			</Tabs.List>
			<Tabs.Content value="results" class="pt-4">
				{#if tracker.sortedEntries.length > 0}
					<!-- Timeline rail: a single line behind the status markers on each row -->
					<div class="relative space-y-3 pl-8">
						<div
							class="pointer-events-none absolute bottom-2 left-[9px] top-2 w-px bg-gradient-to-b from-border via-border to-transparent"
						></div>
						{#each tracker.sortedEntries as entry (entry.name)}
							<ScenarioResultComponent {entry} logs={tracker.getLogsForScenario(entry.name)} />
						{/each}
					</div>
				{:else}
					<p class="text-sm text-muted-foreground">No scenarios yet.</p>
				{/if}
			</Tabs.Content>
			<Tabs.Content value="logs" class="pt-4">
				<RunLog entries={tracker.logs} />
			</Tabs.Content>
		</Tabs.Root>
	{/if}
</div>

<!-- Delete Confirmation Dialog -->
<Dialog.Root bind:open={deleteDialogOpen}>
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>Delete Assessment</Dialog.Title>
			<Dialog.Description>
				Are you sure you want to delete this assessment? This will permanently remove the
				assessment, all scenario results, and log files. This action cannot be undone.
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
