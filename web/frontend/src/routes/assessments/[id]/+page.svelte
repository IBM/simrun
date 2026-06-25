<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import * as Alert from '$lib/components/ui/alert/index.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import * as Collapsible from '$lib/components/ui/collapsible/index.js';
	import * as Tooltip from '$lib/components/ui/tooltip/index.js';
	import * as Breadcrumb from '$lib/components/ui/breadcrumb/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Skeleton } from '$lib/components/ui/skeleton/index.js';
	import YamlEditor from '$lib/components/YamlEditor.svelte';
	import ScheduleDialog from '$lib/components/ScheduleDialog.svelte';
	import PenLineIcon from '@lucide/svelte/icons/pen-line';
	import PlayIcon from '@lucide/svelte/icons/play';
	import CalendarIcon from '@lucide/svelte/icons/calendar';
	import TrashIcon from '@lucide/svelte/icons/trash-2';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import CopyIcon from '@lucide/svelte/icons/copy';
	import CheckIcon from '@lucide/svelte/icons/check';
	import CheckCircle2Icon from '@lucide/svelte/icons/check-circle-2';
	import XCircleIcon from '@lucide/svelte/icons/x-circle';
	import CircleDashedIcon from '@lucide/svelte/icons/circle-dashed';
	import {
		getAssessment,
		runAssessment,
		deleteAssessment,
		getScheduleByAssessment,
		listRuns
	} from '$lib/api/client';
	import { scenarioTypeVariant, formatUserEmail, formatDuration } from '$lib/utils/format';
	import { describeCronExpression } from '$lib/utils/cron';
	import { parseScenarioYAML } from '$lib/utils/yaml-parser';
	import type { Assessment, Schedule, Run } from '$lib/types';

	let id = $derived($page.params.id!);

	let loading = $state(true);
	let loadError = $state('');
	let assessment = $state<Assessment | null>(null);
	let schedule = $state<Schedule | null>(null);
	let runs = $state<Run[]>([]);
	let runsLoading = $state(true);

	let scheduleDialogOpen = $state(false);
	let deleteDialogOpen = $state(false);
	let deleting = $state(false);
	let running = $state(false);
	let actionError = $state('');

	let sourceOpen = $state(false);
	let copied = $state(false);

	let now = $state(Date.now());
	let nowTimer: ReturnType<typeof setInterval> | undefined;

	onMount(async () => {
		nowTimer = setInterval(() => (now = Date.now()), 30_000);
		try {
			assessment = await getAssessment(id);
			schedule = await getScheduleByAssessment(id);
		} catch (e) {
			loadError = e instanceof Error ? e.message : 'Failed to load assessment';
			loading = false;
			return;
		}
		loading = false;

		try {
			const all = await listRuns(1, 100, { assessmentId: id });
			runs = all.runs;
		} catch {
			runs = [];
		} finally {
			runsLoading = false;
		}
	});

	onDestroy(() => {
		if (nowTimer) clearInterval(nowTimer);
	});

	let parsed = $derived(assessment ? parseScenarioYAML(assessment.yaml) : null);
	let parsedScenarios = $derived(parsed?.scenarios ?? []);
	let parsedTarget = $derived(parsed?.target);
	let builderSupported = $derived(parsed?.builderSupported !== false);

	let packsUsed = $derived.by(() => {
		const set = new Set<string>();
		for (const s of parsedScenarios) if (s.pack) set.add(s.pack);
		return [...set];
	});

	let expectationsCount = $derived(
		parsedScenarios.reduce((acc, s) => acc + s.expectations.length, 0)
	);

	let targetsActive = $derived.by(() => {
		if (!parsedTarget) return [] as { kind: string; name: string }[];
		const out: { kind: string; name: string }[] = [];
		(['aws', 'gcp', 'azure', 'kubernetes'] as const).forEach((k) => {
			const v = parsedTarget?.[k];
			if (v) out.push({ kind: k, name: v });
		});
		return out;
	});

	let recentRuns = $derived(runs.slice(0, 8));

	let scenarioYamlBytes = $derived(assessment ? new Blob([assessment.yaml]).size : 0);
	let scenarioYamlLines = $derived(assessment ? assessment.yaml.split('\n').length : 0);

	let nextRunAt = $derived.by(() => {
		if (!schedule || !schedule.enabled) return null;
		// re-read `now` so the next-run rolls forward as time passes
		void now;
		return nextCronRun(schedule.cronExpression);
	});

	function expandCronField(field: string, min: number, max: number): Set<number> {
		const set = new Set<number>();
		for (const part of field.split(',')) {
			let step = 1;
			let range = part;
			if (part.includes('/')) {
				const [r, s] = part.split('/');
				range = r;
				step = parseInt(s, 10);
				if (isNaN(step) || step <= 0) step = 1;
			}
			let from = min;
			let to = max;
			if (range !== '*') {
				if (range.includes('-')) {
					const [a, b] = range.split('-').map((x) => parseInt(x, 10));
					if (isNaN(a) || isNaN(b)) continue;
					from = a;
					to = b;
				} else {
					const n = parseInt(range, 10);
					if (isNaN(n)) continue;
					from = n;
					to = n;
				}
			}
			for (let v = from; v <= to; v += step) set.add(v);
		}
		return set;
	}

	function nextCronRun(cron: string, fromDate = new Date()): Date | null {
		const parts = cron.trim().split(/\s+/);
		if (parts.length !== 5) return null;
		const mins = expandCronField(parts[0], 0, 59);
		const hours = expandCronField(parts[1], 0, 23);
		const days = expandCronField(parts[2], 1, 31);
		const months = expandCronField(parts[3], 1, 12);
		const weekdays = expandCronField(parts[4], 0, 6);
		const start = new Date(fromDate);
		start.setSeconds(0, 0);
		start.setMinutes(start.getMinutes() + 1);
		const horizonMinutes = 60 * 24 * 14;
		for (let i = 0; i < horizonMinutes; i++) {
			const d = new Date(start.getTime() + i * 60_000);
			if (
				mins.has(d.getMinutes()) &&
				hours.has(d.getHours()) &&
				days.has(d.getDate()) &&
				months.has(d.getMonth() + 1) &&
				weekdays.has(d.getDay())
			) {
				return d;
			}
		}
		return null;
	}

	function formatRelative(target: string | Date | null | undefined, ref = now): string {
		if (!target) return '—';
		const t = typeof target === 'string' ? new Date(target).getTime() : target.getTime();
		if (isNaN(t)) return '—';
		const diff = t - ref;
		const abs = Math.abs(diff);
		const past = diff < 0;
		const sec = Math.floor(abs / 1000);
		if (sec < 30) return past ? 'just now' : 'imminent';
		if (sec < 60) return past ? `${sec}s ago` : `in ${sec}s`;
		const min = Math.floor(sec / 60);
		if (min < 60) return past ? `${min}m ago` : `in ${min}m`;
		const hr = Math.floor(min / 60);
		if (hr < 24) {
			const remM = min % 60;
			return past ? `${hr}h ago` : remM ? `in ${hr}h ${remM}m` : `in ${hr}h`;
		}
		const day = Math.floor(hr / 24);
		const remH = hr % 24;
		return past ? `${day}d ago` : remH ? `in ${day}d ${remH}h` : `in ${day}d`;
	}

	async function handleRun() {
		if (!assessment) return;
		actionError = '';
		running = true;
		try {
			const resp = await runAssessment(assessment.id);
			await goto(`/runs/${resp.runId}`);
		} catch (e) {
			actionError = e instanceof Error ? e.message : 'Run failed';
		} finally {
			running = false;
		}
	}

	async function handleDelete() {
		if (!assessment) return;
		actionError = '';
		deleting = true;
		try {
			await deleteAssessment(assessment.id);
			await goto('/assessments');
		} catch (e) {
			actionError = e instanceof Error ? e.message : 'Delete failed';
			deleting = false;
		}
	}

	async function reloadSchedule() {
		schedule = await getScheduleByAssessment(id);
	}

	async function copyYaml() {
		if (!assessment) return;
		try {
			await navigator.clipboard.writeText(assessment.yaml);
			copied = true;
			setTimeout(() => (copied = false), 1400);
		} catch {
			/* noop */
		}
	}

	function runStatusTone(r: Run): { label: string; cls: string } {
		if (r.status === 'running') return { label: 'running', cls: 'text-foreground/70' };
		if (r.status === 'failed' || r.failed > 0) return { label: 'failed', cls: 'text-destructive' };
		return { label: 'passed', cls: 'text-success' };
	}

	function formatBytes(n: number): string {
		if (n < 1024) return `${n} B`;
		if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
		return `${(n / 1024 / 1024).toFixed(1)} MB`;
	}
</script>

{#if loading}
	<div class="space-y-4">
		<Skeleton class="h-5 w-48" />
		<Skeleton class="h-9 w-96" />
		<Skeleton class="h-24 w-full" />
		<Skeleton class="h-32 w-full" />
	</div>
{:else if loadError || !assessment}
	<div class="space-y-4">
		<Alert.Root variant="destructive">
			<Alert.Description>{loadError || 'Assessment not found'}</Alert.Description>
		</Alert.Root>
		<Button variant="outline" onclick={() => goto('/assessments')}>Back to assessments</Button>
	</div>
{:else}
	<div class="space-y-6">
		<Breadcrumb.Root class="animate-fade-up stagger-1">
			<Breadcrumb.List>
				<Breadcrumb.Item>
					<Breadcrumb.Link href="/assessments">Assessments</Breadcrumb.Link>
				</Breadcrumb.Item>
				<Breadcrumb.Separator />
				<Breadcrumb.Item>
					<Breadcrumb.Page class="truncate max-w-md">{assessment.name}</Breadcrumb.Page>
				</Breadcrumb.Item>
			</Breadcrumb.List>
		</Breadcrumb.Root>

		<div class="flex items-start justify-between gap-4 animate-fade-up stagger-1">
			<div class="min-w-0 space-y-2">
				<div class="flex items-center gap-3 flex-wrap min-w-0">
					<h1 class="text-2xl font-bold truncate">{assessment.name}</h1>
					<Badge variant={scenarioTypeVariant(assessment.type)} class="capitalize shrink-0">
						{assessment.type || 'standard'}
					</Badge>
				</div>
				<p class="text-sm text-muted-foreground">
					Created {new Date(assessment.createdAt).toLocaleDateString()}
					{#if assessment.createdBy && assessment.createdBy !== 'anonymous'}
						by
						<Tooltip.Root>
							<Tooltip.Trigger class="cursor-default underline-offset-2 hover:underline">
								{formatUserEmail(assessment.createdBy)}
							</Tooltip.Trigger>
							<Tooltip.Content>{assessment.createdBy}</Tooltip.Content>
						</Tooltip.Root>
					{/if}
					· Updated {new Date(assessment.updatedAt).toLocaleDateString()}
				</p>
			</div>

			<div class="flex items-center gap-2 shrink-0">
				<Button
					variant="ghost"
					size="sm"
					class="text-muted-foreground hover:text-destructive"
					onclick={() => (deleteDialogOpen = true)}
				>
					<TrashIcon data-icon="inline-start" />
					Delete
				</Button>
				<Button variant="outline" size="sm" onclick={() => (scheduleDialogOpen = true)}>
					<CalendarIcon data-icon="inline-start" />
					{schedule ? 'Schedule' : 'Set schedule'}
				</Button>
				<Button variant="outline" size="sm" onclick={() => goto(`/assessments/${assessment!.id}/edit`)}>
					<PenLineIcon data-icon="inline-start" />
					Edit
				</Button>
				<Button size="sm" onclick={handleRun} disabled={running}>
					{#if running}
						<LoaderIcon data-icon="inline-start" class="animate-spin" />
					{:else}
						<PlayIcon data-icon="inline-start" />
					{/if}
					Run
				</Button>
			</div>
		</div>

		{#if actionError}
			<Alert.Root variant="destructive">
				<Alert.Description>{actionError}</Alert.Description>
			</Alert.Root>
		{/if}

		<!-- Configuration -->
		<section class="animate-fade-up stagger-2 space-y-3">
			<h2 class="text-sm font-medium text-muted-foreground">Configuration</h2>

			{#if builderSupported}
				<div class="grid grid-cols-1 md:grid-cols-3 gap-4">
					<div class="rounded-lg border bg-card p-4">
						<div class="flex items-center justify-between mb-3">
							<h3 class="text-sm font-medium">Targets</h3>
							<span class="text-xs text-muted-foreground tabular-nums">
								{targetsActive.length}
							</span>
						</div>
						{#if targetsActive.length === 0}
							<p class="text-sm text-muted-foreground">No cloud targets.</p>
						{:else}
							<ul class="space-y-1.5">
								{#each targetsActive as t}
									<li class="flex items-baseline justify-between gap-2 text-sm">
										<span class="text-xs text-muted-foreground capitalize">{t.kind}</span>
										<span class="font-mono text-foreground/90 truncate">{t.name}</span>
									</li>
								{/each}
							</ul>
						{/if}
					</div>

					<div class="rounded-lg border bg-card p-4">
						<div class="flex items-center justify-between mb-3">
							<h3 class="text-sm font-medium">Packs</h3>
							<span class="text-xs text-muted-foreground tabular-nums">{packsUsed.length}</span>
						</div>
						{#if packsUsed.length === 0}
							<p class="text-sm text-muted-foreground">No packs referenced.</p>
						{:else}
							<ul class="space-y-1.5">
								{#each packsUsed as p}
									<li class="flex items-center gap-2 text-sm">
										<span class="h-1 w-1 rounded-full bg-primary/60 shrink-0" aria-hidden="true"
										></span>
										<span class="font-mono text-foreground/90 truncate">{p}</span>
									</li>
								{/each}
							</ul>
						{/if}
					</div>

					<div class="rounded-lg border bg-card p-4">
						<div class="flex items-center justify-between mb-3">
							<h3 class="text-sm font-medium">Scenarios</h3>
							<span class="text-xs text-muted-foreground tabular-nums">
								{parsedScenarios.length} · {expectationsCount} exp
							</span>
						</div>
						{#if parsedScenarios.length === 0}
							<p class="text-sm text-muted-foreground">None.</p>
						{:else}
							<ul class="space-y-1.5">
								{#each parsedScenarios.slice(0, 4) as s}
									<li class="flex items-center justify-between gap-2 text-sm">
										<span class="truncate text-foreground/90">{s.name || '(unnamed)'}</span>
										<span class="text-xs text-muted-foreground tabular-nums shrink-0">
											{s.scenarioType === 'inject' ? 'inject' : 'detonate'} · {s.expectations
												.length} exp
										</span>
									</li>
								{/each}
								{#if parsedScenarios.length > 4}
									<li class="text-xs text-muted-foreground">
										+ {parsedScenarios.length - 4} more
									</li>
								{/if}
							</ul>
						{/if}
					</div>
				</div>
			{:else}
				<div
					class="rounded-lg border border-dashed bg-card/40 px-5 py-4 text-sm text-muted-foreground"
				>
					This assessment uses a detonator type that isn't introspected. Review the source below.
				</div>
			{/if}
		</section>

		<!-- Schedule -->
		<section class="animate-fade-up stagger-3 space-y-3">
			<h2 class="text-sm font-medium text-muted-foreground">Schedule</h2>

			{#if schedule}
				<div class="rounded-lg border bg-card">
					<div
						class="grid grid-cols-1 md:grid-cols-[1.4fr_1fr_1fr_auto] divide-y md:divide-y-0 md:divide-x"
					>
						<div class="px-5 py-4 flex items-start gap-3 min-w-0">
							<span
								class="mt-1.5 inline-block h-2 w-2 rounded-full shrink-0 {schedule.enabled
									? 'bg-success'
									: 'bg-muted-foreground/40'}"
								aria-hidden="true"
							></span>
							<div class="min-w-0">
								<div class="text-xs text-muted-foreground capitalize">
									{schedule.enabled ? 'Enabled' : 'Disabled'}
								</div>
								<div class="mt-0.5 font-medium text-foreground truncate">
									{describeCronExpression(schedule.cronExpression)}
								</div>
								<div class="mt-1 font-mono text-xs text-muted-foreground tabular-nums truncate">
									{schedule.cronExpression}
								</div>
							</div>
						</div>
						<div class="px-5 py-4">
							<div class="text-xs text-muted-foreground">Next run</div>
							{#if schedule.enabled && nextRunAt}
								<div class="mt-0.5 font-medium tabular-nums">{formatRelative(nextRunAt)}</div>
								<div class="mt-1 font-mono text-xs text-muted-foreground tabular-nums">
									{nextRunAt.toLocaleString()}
								</div>
							{:else}
								<div class="mt-0.5 font-medium text-muted-foreground">—</div>
								<div class="mt-1 text-xs text-muted-foreground">
									{schedule.enabled ? 'Unable to compute' : 'Paused'}
								</div>
							{/if}
						</div>
						<div class="px-5 py-4">
							<div class="text-xs text-muted-foreground">Last triggered</div>
							{#if schedule.lastRunAt}
								<div class="mt-0.5 font-medium tabular-nums">
									{formatRelative(schedule.lastRunAt)}
								</div>
								<div class="mt-1 font-mono text-xs text-muted-foreground tabular-nums">
									{new Date(schedule.lastRunAt).toLocaleString()}
								</div>
							{:else}
								<div class="mt-0.5 font-medium text-muted-foreground">—</div>
								<div class="mt-1 text-xs text-muted-foreground">Never</div>
							{/if}
						</div>
						<div class="px-5 py-4 flex md:items-center md:justify-end">
							<Button variant="ghost" size="sm" onclick={() => (scheduleDialogOpen = true)}>
								Configure
							</Button>
						</div>
					</div>
				</div>
			{:else}
				<button
					type="button"
					onclick={() => (scheduleDialogOpen = true)}
					class="w-full rounded-lg border border-dashed bg-card/40 px-5 py-5 flex items-center gap-4 text-left hover:bg-card hover:border-border transition-colors"
				>
					<CalendarIcon class="h-5 w-5 text-muted-foreground" />
					<div class="flex-1">
						<div class="font-medium">No schedule</div>
						<div class="text-sm text-muted-foreground">
							Run this assessment automatically on a recurring cadence.
						</div>
					</div>
					<span class="text-sm text-muted-foreground">Set schedule →</span>
				</button>
			{/if}
		</section>

		<!-- Recent runs -->
		<section class="animate-fade-up stagger-4 space-y-3">
			<div class="flex items-end justify-between gap-2">
				<h2 class="text-sm font-medium text-muted-foreground">
					Recent runs{#if !runsLoading && runs.length > 0}
						<span class="text-muted-foreground/70"> · {runs.length}</span>
					{/if}
				</h2>
				{#if runs.length > recentRuns.length}
					<a
						href="/runs"
						class="text-xs text-muted-foreground hover:text-foreground transition-colors"
					>
						View all →
					</a>
				{/if}
			</div>

			{#if runsLoading}
				<div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-2">
					{#each Array(4) as _}
						<Skeleton class="h-16 w-full" />
					{/each}
				</div>
			{:else if recentRuns.length === 0}
				<div
					class="rounded-lg border border-dashed bg-card/40 px-5 py-6 text-center text-sm text-muted-foreground"
				>
					No runs yet. Press <kbd class="font-mono text-xs px-1.5 py-0.5 rounded border bg-muted"
						>Run</kbd
					> to start collecting history.
				</div>
			{:else}
				<div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-2">
					{#each recentRuns as r (r.id)}
						{@const tone = runStatusTone(r)}
						<button
							type="button"
							onclick={() => goto(`/runs/${r.id}`)}
							class="group text-left rounded-md border bg-card hover:bg-accent/40 hover:border-foreground/20 transition-colors px-3 py-2.5 flex items-start gap-2.5"
						>
							<span
								class="mt-1 h-2 w-2 rounded-full shrink-0 {r.status === 'running'
									? 'bg-foreground/40 animate-pulse'
									: r.failed > 0 || r.status === 'failed'
										? 'bg-destructive'
										: 'bg-success'}"
								aria-hidden="true"
							></span>
							<div class="min-w-0 flex-1">
								<div class="flex items-center gap-1.5">
									{#if r.status === 'running'}
										<CircleDashedIcon class="h-3 w-3 text-foreground/60 animate-spin" />
									{:else if r.failed > 0 || r.status === 'failed'}
										<XCircleIcon class="h-3 w-3 text-destructive" />
									{:else}
										<CheckCircle2Icon class="h-3 w-3 text-success" />
									{/if}
									<span class="text-xs font-medium capitalize {tone.cls}">{tone.label}</span>
								</div>
								<div class="mt-0.5 text-xs text-muted-foreground tabular-nums truncate">
									{formatRelative(r.startTime)} · {formatDuration(r.startTime, r.endTime)}
								</div>
								<div class="mt-0.5 text-xs text-muted-foreground tabular-nums">
									{r.succeeded}/{r.total} passed
								</div>
							</div>
							<ChevronRightIcon
								class="h-3.5 w-3.5 text-muted-foreground/40 group-hover:text-foreground transition-colors mt-1"
							/>
						</button>
					{/each}
				</div>
			{/if}
		</section>

		<!-- Source -->
		<section class="animate-fade-up stagger-5 space-y-3">
			<Collapsible.Root bind:open={sourceOpen}>
				<div class="rounded-lg border bg-card overflow-hidden">
					<Collapsible.Trigger
						class="w-full flex items-center gap-3 px-4 py-3 hover:bg-accent/40 transition-colors"
					>
						<ChevronRightIcon
							class="h-4 w-4 text-muted-foreground transition-transform {sourceOpen
								? 'rotate-90'
								: ''}"
						/>
						<span class="text-sm font-medium">Source</span>
						<span class="font-mono text-xs text-muted-foreground truncate">
							{assessment.name}.yaml
						</span>
						<span class="ml-auto text-xs text-muted-foreground tabular-nums">
							{scenarioYamlLines} lines · {formatBytes(scenarioYamlBytes)}
						</span>
						<button
							type="button"
							onclick={(e) => {
								e.stopPropagation();
								copyYaml();
							}}
							class="ml-2 inline-flex items-center gap-1 rounded px-2 py-1 text-xs text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
							aria-label="Copy YAML"
						>
							{#if copied}
								<CheckIcon class="h-3 w-3 text-success" />
								<span class="text-success">Copied</span>
							{:else}
								<CopyIcon class="h-3 w-3" />
								<span>Copy</span>
							{/if}
						</button>
					</Collapsible.Trigger>
					<Collapsible.Content>
						<div class="border-t">
							<YamlEditor value={assessment.yaml} readonly />
						</div>
					</Collapsible.Content>
				</div>
			</Collapsible.Root>
		</section>
	</div>

	<ScheduleDialog
		bind:open={scheduleDialogOpen}
		{assessment}
		onclose={() => (scheduleDialogOpen = false)}
		onsuccess={async () => {
			scheduleDialogOpen = false;
			await reloadSchedule();
		}}
	/>

	<Dialog.Root bind:open={deleteDialogOpen}>
		<Dialog.Content>
			<Dialog.Header>
				<Dialog.Title>Delete Assessment</Dialog.Title>
				<Dialog.Description>
					Are you sure you want to delete "{assessment.name}"? This action cannot be undone.
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
{/if}
