<script lang="ts">
	import { goto } from '$app/navigation';
	import * as Alert from '$lib/components/ui/alert/index.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import * as Table from '$lib/components/ui/table/index.js';
	import * as Empty from '$lib/components/ui/empty/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { listScenarios, runScenario } from '$lib/api/client';
	import type { SavedScenario } from '$lib/types';
	import { scenarioTypeVariant } from '$lib/utils/format';
	import FileIcon from '@lucide/svelte/icons/file';

	let {
		open = $bindable()
	}: {
		open: boolean;
	} = $props();

	let loading = $state(true);
	let error = $state('');
	let selectedScenarioId = $state('');
	let parallelism = $state(5);
	let timeout = $state('10m');
	let running = $state(false);
	let dialogScenarios = $state<SavedScenario[]>([]);

	let selectedScenarioType = $derived(
		dialogScenarios.find((s) => s.id === selectedScenarioId)?.type || 'standard'
	);

	let needsTimeout = $derived(
		selectedScenarioType === 'explore' || selectedScenarioType === 'collect'
	);

	$effect(() => {
		if (open) {
			loading = true;
			error = '';
			selectedScenarioId = '';
			running = false;
			timeout = '10m';
			// Fetch first 100 most-recently-updated. A picker for >100 scenarios
			// needs a real search affordance — out of scope here.
			listScenarios(1, 100, {})
				.then((page) => {
					dialogScenarios = page.scenarios;
				})
				.catch((e) => {
					error = e instanceof Error ? e.message : 'Failed to load scenarios';
				})
				.finally(() => {
					loading = false;
				});
		}
	});

	async function handleRun() {
		if (!selectedScenarioId) return;
		if (needsTimeout && timeout && !/^\d+[smh]$/.test(timeout)) {
			error = 'Invalid timeout format. Use a duration like 10m, 30s, or 1h.';
			return;
		}
		running = true;
		error = '';
		try {
			const isExplore = selectedScenarioType === 'explore';
			const result = await runScenario(
				selectedScenarioId,
				parallelism,
				isExplore,
				false,
				needsTimeout ? timeout : undefined
			);
			open = false;
			goto(`/assessments/${result.runId}`);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to start assessment';
			running = false;
		}
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="max-w-xl">
		<Dialog.Header>
			<Dialog.Title>New Assessment</Dialog.Title>
			<Dialog.Description>Select a saved scenario and start a new assessment.</Dialog.Description>
		</Dialog.Header>

		{#if error}
			<Alert.Root variant="destructive">
				<Alert.Description>{error}</Alert.Description>
			</Alert.Root>
		{/if}

		{#if loading}
			<p class="text-sm text-muted-foreground">Loading scenarios...</p>
		{:else if dialogScenarios.length === 0}
			<Empty.Root>
				<Empty.Header>
					<Empty.Media variant="icon">
						<FileIcon />
					</Empty.Media>
					<Empty.Title>No saved scenarios</Empty.Title>
					<Empty.Description
						>Create a scenario first, then come back to start an assessment.</Empty.Description
					>
				</Empty.Header>
				<Empty.Content>
					<Button
						onclick={() => {
							open = false;
							goto('/scenarios');
						}}>Go to Scenarios</Button
					>
				</Empty.Content>
			</Empty.Root>
		{:else}
			<div class="max-h-64 overflow-y-auto border rounded-md">
				<Table.Root>
					<Table.Header>
						<Table.Row>
							<Table.Head class="w-12"></Table.Head>
							<Table.Head>Name</Table.Head>
							<Table.Head>Type</Table.Head>
							<Table.Head>Updated</Table.Head>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each dialogScenarios as scenario}
							<Table.Row
								class="cursor-pointer {selectedScenarioId === scenario.id ? 'bg-muted' : ''}"
								onclick={() => (selectedScenarioId = scenario.id)}
							>
								<Table.Cell>
									<input
										type="radio"
										name="scenario"
										value={scenario.id}
										checked={selectedScenarioId === scenario.id}
										onchange={() => (selectedScenarioId = scenario.id)}
										class="h-4 w-4"
									/>
								</Table.Cell>
								<Table.Cell class="font-medium">{scenario.name}</Table.Cell>
								<Table.Cell>
									<Badge variant={scenarioTypeVariant(scenario.type)}
										>{scenario.type || 'standard'}</Badge
									>
								</Table.Cell>
								<Table.Cell>{new Date(scenario.updatedAt).toLocaleDateString()}</Table.Cell>
							</Table.Row>
						{/each}
					</Table.Body>
				</Table.Root>
			</div>

			<div class="space-y-4 pt-4">
				<div class="flex items-center gap-4">
					<div class="flex items-center gap-2">
						<Label for="parallelism">Parallelism</Label>
						<Input
							id="parallelism"
							type="number"
							min={1}
							max={20}
							class="w-20"
							bind:value={parallelism}
						/>
					</div>

					{#if needsTimeout}
						<div class="flex items-center gap-2">
							<Label for="timeout">Timeout</Label>
							<Input id="timeout" placeholder="10m" class="w-24" bind:value={timeout} />
						</div>
					{/if}
				</div>

				{#if needsTimeout}
					<div class="rounded-md border border-border bg-muted/30 p-3">
						<p class="text-xs text-muted-foreground">
							{#if selectedScenarioType === 'explore'}
								Explore mode: searches all alerts for indicators instead of matching specific rules.
								Waits for the full timeout to discover all triggered alerts.
							{:else}
								Collect mode: collects logs after detonation for analysis. Waits for the timeout
								period before collecting.
							{/if}
						</p>
					</div>
				{/if}

				<div class="flex justify-end gap-2">
					<Button variant="outline" onclick={() => (open = false)}>Cancel</Button>
					<Button onclick={handleRun} disabled={running || !selectedScenarioId}>
						{running ? 'Starting...' : 'Start'}
					</Button>
				</div>
			</div>
		{/if}
	</Dialog.Content>
</Dialog.Root>
