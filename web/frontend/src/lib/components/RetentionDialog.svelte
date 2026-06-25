<script lang="ts">
	import * as Alert from '$lib/components/ui/alert/index.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { updateConfig } from '$lib/api/client';
	import type { AppConfig } from '$lib/types';

	let {
		open = $bindable(),
		config,
		onsaved
	}: {
		open: boolean;
		config: AppConfig;
		onsaved: (changes: Record<string, unknown>) => void;
	} = $props();

	function boolOf(value: unknown, fallback: boolean): boolean {
		return typeof value === 'boolean' ? value : fallback;
	}
	function numOf(value: unknown, fallback: number): number {
		return typeof value === 'number' ? value : fallback;
	}

	// Form state, seeded from current config each time the dialog opens.
	let logEnabled = $state(true);
	let logDays = $state(7);
	let runEnabled = $state(false);
	let runDays = $state(30);
	let saving = $state(false);
	let error = $state('');

	$effect(() => {
		if (open) {
			logEnabled = boolOf(config['run_log_retention_enabled'], true);
			logDays = numOf(config['run_log_retention_days'], 7);
			runEnabled = boolOf(config['run_retention_enabled'], false);
			runDays = numOf(config['run_retention_days'], 30);
			error = '';
		}
	});

	async function handleSave() {
		error = '';

		const logDaysInt = Math.trunc(Number(logDays));
		const runDaysInt = Math.trunc(Number(runDays));

		// Validate up front to avoid a partial save (keys are PUT one at a time).
		// Only check a section's days when its toggle is on.
		if ((logEnabled && logDaysInt < 1) || (runEnabled && runDaysInt < 1)) {
			error = 'Retention periods must be at least 1 day.';
			return;
		}

		saving = true;

		// Only PUT keys whose value changed, one call each (matches the page's
		// existing per-key config writes). For a disabled section, keep the
		// currently stored days value so an empty/invalid greyed-out field is
		// never PUT (the backend would reject it with HTTP 400).
		const next: Record<string, unknown> = {
			run_log_retention_enabled: logEnabled,
			run_log_retention_days: logEnabled
				? logDaysInt
				: numOf(config['run_log_retention_days'], 7),
			run_retention_enabled: runEnabled,
			run_retention_days: runEnabled ? runDaysInt : numOf(config['run_retention_days'], 30)
		};

		const changed = Object.entries(next).filter(([key, value]) => value !== config[key]);

		try {
			for (const [key, value] of changed) {
				await updateConfig(key, value);
			}
			onsaved(Object.fromEntries(changed));
			open = false;
		} catch (e) {
			// Keep the dialog open with the entered values so a rejected day
			// count (HTTP 400) can be corrected without re-typing.
			error = e instanceof Error ? e.message : 'Failed to save retention settings';
		} finally {
			saving = false;
		}
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-lg">
		<Dialog.Header>
			<Dialog.Title>Run retention</Dialog.Title>
			<Dialog.Description>
				Control how long run logs and whole runs are kept before they are deleted automatically.
			</Dialog.Description>
		</Dialog.Header>

		<div class="space-y-6">
			<div class="space-y-3">
				<div class="flex items-center justify-between">
					<Label for="log-retention-enabled">Delete old run logs</Label>
					<Switch id="log-retention-enabled" bind:checked={logEnabled} />
				</div>
				<div class="space-y-1">
					<Label for="log-retention-days">Keep run logs for (days)</Label>
					<Input
						id="log-retention-days"
						type="number"
						min={1}
						class="w-24"
						bind:value={logDays}
						disabled={!logEnabled}
					/>
					<p class="text-xs text-muted-foreground">
						Verbose per-run logs older than this are removed; the run record is kept.
					</p>
				</div>
			</div>

			<div class="space-y-3">
				<div class="flex items-center justify-between">
					<Label for="run-retention-enabled">Delete old runs</Label>
					<Switch id="run-retention-enabled" bind:checked={runEnabled} />
				</div>
				<div class="space-y-1">
					<Label for="run-retention-days">Keep runs for (days)</Label>
					<Input
						id="run-retention-days"
						type="number"
						min={1}
						class="w-24"
						bind:value={runDays}
						disabled={!runEnabled}
					/>
					<p class="text-xs text-muted-foreground">
						Whole runs older than this — results and collected logs included — are permanently
						deleted.
					</p>
				</div>
			</div>

			{#if error}
				<Alert.Root variant="destructive">
					<Alert.Description>{error}</Alert.Description>
				</Alert.Root>
			{/if}
		</div>

		<div class="flex justify-end gap-2 pt-4">
			<Button variant="outline" onclick={() => (open = false)} disabled={saving}>Cancel</Button>
			<Button onclick={handleSave} disabled={saving}>
				{saving ? 'Saving...' : 'Save'}
			</Button>
		</div>
	</Dialog.Content>
</Dialog.Root>
