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
	let assessmentEnabled = $state(false);
	let assessmentDays = $state(30);
	let saving = $state(false);
	let error = $state('');

	$effect(() => {
		if (open) {
			logEnabled = boolOf(config['assessment_log_retention_enabled'], true);
			logDays = numOf(config['assessment_log_retention_days'], 7);
			assessmentEnabled = boolOf(config['assessment_retention_enabled'], false);
			assessmentDays = numOf(config['assessment_retention_days'], 30);
			error = '';
		}
	});

	async function handleSave() {
		saving = true;
		error = '';

		// Only PUT keys whose value changed, one call each (matches the page's
		// existing per-key config writes). Day fields are coerced to integers so
		// the backend's >= 1 validation sees a number.
		const next: Record<string, unknown> = {
			assessment_log_retention_enabled: logEnabled,
			assessment_log_retention_days: Math.trunc(Number(logDays)),
			assessment_retention_enabled: assessmentEnabled,
			assessment_retention_days: Math.trunc(Number(assessmentDays))
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
	<Dialog.Content class="max-w-lg">
		<Dialog.Header>
			<Dialog.Title>Assessment retention</Dialog.Title>
			<Dialog.Description>
				Control how long run logs and whole assessments are kept before they are deleted
				automatically.
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
						Verbose per-run logs older than this are removed; the assessment record is kept.
					</p>
				</div>
			</div>

			<div class="space-y-3">
				<div class="flex items-center justify-between">
					<Label for="assessment-retention-enabled">Delete old assessments</Label>
					<Switch id="assessment-retention-enabled" bind:checked={assessmentEnabled} />
				</div>
				<div class="space-y-1">
					<Label for="assessment-retention-days">Keep assessments for (days)</Label>
					<Input
						id="assessment-retention-days"
						type="number"
						min={1}
						class="w-24"
						bind:value={assessmentDays}
						disabled={!assessmentEnabled}
					/>
					<p class="text-xs text-muted-foreground">
						Whole assessments older than this — results and collected logs included — are
						permanently deleted.
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
