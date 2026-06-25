<script lang="ts">
	import * as Alert from '$lib/components/ui/alert/index.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import * as Select from '$lib/components/ui/select/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import type { Assessment, Schedule } from '$lib/types';
	import {
		createSchedule,
		updateSchedule,
		deleteSchedule,
		getScheduleByAssessment
	} from '$lib/api/client';
	import { describeCronExpression, validateCronExpression, cronPresets } from '$lib/utils/cron';

	let {
		open = $bindable(),
		assessment,
		onclose,
		onsuccess
	}: {
		open: boolean;
		assessment: Assessment;
		onclose: () => void;
		onsuccess: () => void;
	} = $props();

	let loading = $state(true);
	let saving = $state(false);
	let deleting = $state(false);
	let error = $state('');
	let existingSchedule = $state<Schedule | null>(null);
	let cronExpression = $state('0 0 * * *');
	let enabled = $state(true);
	let parallelism = $state(10);

	$effect(() => {
		if (open && assessment) {
			loadSchedule();
		}
	});

	async function loadSchedule() {
		loading = true;
		error = '';
		try {
			const schedule = await getScheduleByAssessment(assessment.id);
			if (schedule) {
				existingSchedule = schedule;
				cronExpression = schedule.cronExpression;
				enabled = schedule.enabled;
				parallelism = schedule.parallelism;
			} else {
				existingSchedule = null;
				cronExpression = '0 0 * * *';
				enabled = true;
				parallelism = 10;
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load schedule';
		} finally {
			loading = false;
		}
	}

	async function handleSave() {
		const validationError = validateCronExpression(cronExpression);
		if (validationError) {
			error = validationError;
			return;
		}

		saving = true;
		error = '';
		try {
			if (existingSchedule) {
				await updateSchedule(existingSchedule.id, cronExpression, enabled, parallelism);
			} else {
				await createSchedule(assessment.id, cronExpression, enabled, parallelism);
			}
			onsuccess();
			onclose();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save schedule';
		} finally {
			saving = false;
		}
	}

	async function handleDelete() {
		if (!existingSchedule) return;
		deleting = true;
		error = '';
		try {
			await deleteSchedule(existingSchedule.id);
			onsuccess();
			onclose();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to delete schedule';
		} finally {
			deleting = false;
		}
	}

	let presetValue = $state('');
	let presetLabel = $derived(
		presetValue
			? (cronPresets.find((p) => p.value === presetValue)?.label ?? 'Choose a preset...')
			: 'Choose a preset...'
	);

	let cronDescription = $derived(describeCronExpression(cronExpression));
	let isValid = $derived(validateCronExpression(cronExpression) === null);
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-lg">
		<Dialog.Header>
			<Dialog.Title>
				{existingSchedule ? 'Edit Schedule' : 'Create Schedule'}
			</Dialog.Title>
			<Dialog.Description>
				Schedule "{assessment.name}" to run automatically.
			</Dialog.Description>
		</Dialog.Header>

		{#if loading}
			<p class="text-sm text-muted-foreground">Loading...</p>
		{:else}
			<div class="space-y-4">
				<div class="space-y-2">
					<Label>Quick Presets</Label>
					<Select.Root
						type="single"
						bind:value={presetValue}
						onValueChange={(v) => {
							if (v) cronExpression = v;
						}}
					>
						<Select.Trigger class="w-full">
							{presetLabel}
						</Select.Trigger>
						<Select.Content>
							{#each cronPresets as preset}
								<Select.Item value={preset.value} label={preset.label} />
							{/each}
						</Select.Content>
					</Select.Root>
				</div>

				<div>
					<Label for="cron">Cron Expression</Label>
					<Input
						id="cron"
						bind:value={cronExpression}
						placeholder="0 0 * * *"
						class={!isValid ? 'border-destructive' : ''}
					/>
					<p class="mt-1 text-xs text-muted-foreground">Format: minute hour day month weekday</p>
				</div>

				{#if isValid}
					<div class="rounded-md bg-muted p-3">
						<p class="text-sm">
							<span class="font-medium">Preview:</span>
							<span class="text-muted-foreground">{cronDescription}</span>
						</p>
					</div>
				{/if}

				<div class="flex items-center justify-between">
					<Label for="schedule-enabled">{enabled ? 'Enabled' : 'Disabled'}</Label>
					<Switch id="schedule-enabled" bind:checked={enabled} />
				</div>

				<div class="space-y-2">
					<Label for="schedule-parallelism">Parallelism</Label>
					<Input
						id="schedule-parallelism"
						type="number"
						min={1}
						max={20}
						class="w-20"
						bind:value={parallelism}
					/>
					<p class="text-xs text-muted-foreground">Max concurrent scenarios per run (1-20)</p>
				</div>

				{#if existingSchedule?.lastRunAt}
					<p class="text-xs text-muted-foreground">
						Last run: {new Date(existingSchedule.lastRunAt).toLocaleString()}
					</p>
				{/if}

				{#if error}
					<Alert.Root variant="destructive">
						<Alert.Description>{error}</Alert.Description>
					</Alert.Root>
				{/if}
			</div>

			<div class="flex justify-between pt-4">
				<div>
					{#if existingSchedule}
						<Button variant="destructive" onclick={handleDelete} disabled={saving || deleting}>
							{deleting ? 'Deleting...' : 'Delete Schedule'}
						</Button>
					{/if}
				</div>
				<div class="flex gap-2">
					<Button variant="outline" onclick={onclose}>Cancel</Button>
					<Button onclick={handleSave} disabled={saving || !isValid}>
						{saving ? 'Saving...' : existingSchedule ? 'Update' : 'Create'}
					</Button>
				</div>
			</div>
		{/if}
	</Dialog.Content>
</Dialog.Root>
