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
	import CheckCircleIcon from '@lucide/svelte/icons/circle-check';
	import XCircleIcon from '@lucide/svelte/icons/circle-x';
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

		<div class="rounded-lg border bg-card animate-fade-up stagger-2">
			<div class="flex items-start justify-between gap-4 px-5 pt-4 pb-3">
				<div class="min-w-0 space-y-1">
					<div class="flex items-center gap-2.5">
						<h1 class="text-lg font-bold font-mono leading-none">{$currentRun.id.slice(0, 8)}</h1>
						<Badge variant={statusVariant($currentRun.status)}>
							{$currentRun.status}
						</Badge>
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

					<div class="flex items-center gap-3 text-sm text-muted-foreground flex-wrap">
						{#if $currentRun.scenarioId}
							<span class="inline-flex items-center gap-1">
								<FileTextIcon class="h-3.5 w-3.5 shrink-0" />
								{#if scenarioName}
									<a
										href={`/scenarios/${$currentRun.scenarioId}`}
										class="text-foreground hover:underline underline-offset-2 truncate max-w-[240px]"
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
							<span class="font-mono text-xs"
								>{formatDuration($currentRun.startTime, $currentRun.endTime)}</span
							>
						</span>
					</div>
				</div>

				<Button
					variant="ghost"
					size="icon"
					class="h-8 w-8 text-muted-foreground hover:text-destructive shrink-0"
					onclick={() => (deleteDialogOpen = true)}
				>
					<TrashIcon class="h-4 w-4" />
				</Button>
			</div>

			<div class="px-5 pb-4 pt-1">
				<div class="flex items-center gap-3">
					<div class="bg-muted relative h-1.5 overflow-hidden rounded-full flex-1">
						<div
							class="bg-success h-full transition-all duration-500 ease-out absolute left-0 top-0"
							style="width: {succeededPct}%"
						></div>
						<div
							class="bg-destructive h-full transition-all duration-500 ease-out absolute top-0"
							style="left: {succeededPct}%; width: {failedPct}%"
						></div>
					</div>
					<div class="flex items-center gap-2.5 text-xs text-muted-foreground whitespace-nowrap">
						<span class="inline-flex items-center gap-1">
							<CheckCircleIcon class="h-3.5 w-3.5 text-success" />
							<span class="font-mono font-medium text-foreground">{trackerSucceeded}</span>
						</span>
						<span class="inline-flex items-center gap-1">
							<XCircleIcon class="h-3.5 w-3.5 text-destructive" />
							<span class="font-mono font-medium text-foreground">{trackerFailed}</span>
						</span>
						<span class="text-muted-foreground/60">/</span>
						<span class="font-mono">{total}</span>
						{#if $currentRun.status === 'running'}
							<LoaderCircleIcon class="h-3.5 w-3.5 animate-spin text-muted-foreground" />
						{/if}
					</div>
				</div>
			</div>
		</div>

		<Tabs.Root value="results" class="animate-fade-up stagger-4">
			<Tabs.List>
				<Tabs.Trigger value="results">Results ({tracker.sortedEntries.length})</Tabs.Trigger>
				<Tabs.Trigger value="logs">Logs ({tracker.logs.length})</Tabs.Trigger>
			</Tabs.List>
			<Tabs.Content value="results" class="space-y-3 pt-4">
				{#if tracker.sortedEntries.length > 0}
					{#each tracker.sortedEntries as entry (entry.name)}
						<ScenarioResultComponent {entry} logs={tracker.getLogsForScenario(entry.name)} />
					{/each}
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
