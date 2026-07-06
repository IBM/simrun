<script lang="ts">
	import { onMount } from 'svelte';
	import * as Alert from '$lib/components/ui/alert/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import * as Field from '$lib/components/ui/field/index.js';
	import * as InputGroup from '$lib/components/ui/input-group/index.js';
	import * as Tabs from '$lib/components/ui/tabs/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Skeleton } from '$lib/components/ui/skeleton/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import KeyValueEditor, { hasBlankEntry } from '$lib/components/KeyValueEditor.svelte';
	import { getConfig, updateConfig, getVersion } from '$lib/api/client';
	import { toast } from 'svelte-sonner';
	import type { AppConfig, VersionInfo } from '$lib/types';

	let loading = $state(true);
	let error = $state('');
	let config = $state<AppConfig>({});
	let version = $state<VersionInfo | null>(null);

	let parallelism = $state(0);
	let terraformVersion = $state('');
	let packLogsEnabled = $state(false);
	let sshLoggingEnabled = $state(false);
	let generalSaving = $state(false);
	let generalError = $state('');

	let defaultTags = $state<Record<string, string>>({});
	let tagsSaving = $state(false);
	let tagsError = $state('');

	let runLogRetentionEnabled = $state(false);
	let runLogRetentionDays = $state(7);
	let runRetentionEnabled = $state(false);
	let runRetentionDays = $state(30);
	let retentionSaving = $state(false);
	let retentionError = $state('');

	onMount(async () => {
		try {
			const [cfg, ver] = await Promise.all([getConfig(), getVersion()]);
			config = cfg;
			version = ver;
			parallelism = cfg.parallelism ?? 0;
			terraformVersion = cfg.terraform_version ?? '';
			packLogsEnabled = cfg.pack_logs_enabled ?? false;
			sshLoggingEnabled = cfg.ssh_logging_enabled ?? false;
			defaultTags = cfg.default_tags ?? {};
			runLogRetentionEnabled = cfg.run_log_retention_enabled ?? false;
			runLogRetentionDays = cfg.run_log_retention_days ?? 7;
			runRetentionEnabled = cfg.run_retention_enabled ?? false;
			runRetentionDays = cfg.run_retention_days ?? 30;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load config';
		} finally {
			loading = false;
		}
	});

	// Switches are self-contained transactions: persist immediately and keep
	// the toggled value even on failure, matching the tab Save behavior below.
	async function toggleSwitch(key: string, value: boolean, setError: (msg: string) => void) {
		setError('');
		try {
			await updateConfig(key, value);
			config = { ...config, [key]: value };
			toast.success('Setting saved');
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to save');
		}
	}

	// Persists only the keys whose value actually changed since the last load
	// or save, issuing one PUT per dirty key. Failed saves leave the form
	// values untouched so the user doesn't lose their edits.
	async function saveKeys(
		entries: [string, unknown][],
		setSaving: (v: boolean) => void,
		setError: (v: string) => void
	) {
		const dirty = entries.filter(
			([key, value]) => JSON.stringify(value) !== JSON.stringify(config[key])
		);
		if (dirty.length === 0) return;

		setSaving(true);
		setError('');
		try {
			for (const [key, value] of dirty) {
				await updateConfig(key, value);
			}
			config = { ...config, ...Object.fromEntries(dirty) };
			toast.success('Settings saved');
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Save failed');
		} finally {
			setSaving(false);
		}
	}

	function saveGeneral() {
		return saveKeys(
			[
				['parallelism', Math.trunc(Number(parallelism))],
				['terraform_version', terraformVersion]
			],
			(v) => (generalSaving = v),
			(v) => (generalError = v)
		);
	}

	function saveTags() {
		tagsError = '';
		if (hasBlankEntry(defaultTags)) {
			tagsError = 'Keys and values cannot be empty — remove or fill blank entries.';
			return;
		}
		return saveKeys(
			[['default_tags', defaultTags]],
			(v) => (tagsSaving = v),
			(v) => (tagsError = v)
		);
	}

	function saveRetention() {
		return saveKeys(
			[
				['run_log_retention_days', Math.trunc(Number(runLogRetentionDays))],
				['run_retention_days', Math.trunc(Number(runRetentionDays))]
			],
			(v) => (retentionSaving = v),
			(v) => (retentionError = v)
		);
	}
</script>

<div class="space-y-6">
	<h1 class="text-2xl font-bold">Settings</h1>

	{#if error}
		<Alert.Root variant="destructive">
			<Alert.Description>{error}</Alert.Description>
		</Alert.Root>
	{/if}

	{#if loading}
		<div class="space-y-4">
			<Skeleton class="h-9 w-80" />
			<Skeleton class="h-64 w-full rounded-xl" />
		</div>
	{:else}
		<Tabs.Root value="general">
			<Tabs.List>
				<Tabs.Trigger value="general">General</Tabs.Trigger>
				<Tabs.Trigger value="tags">Default tags</Tabs.Trigger>
				<Tabs.Trigger value="retention">Retention</Tabs.Trigger>
				<Tabs.Trigger value="about">About</Tabs.Trigger>
			</Tabs.List>

			<Tabs.Content value="general" class="mt-4">
				<Card.Root class="animate-fade-up stagger-2">
					<Card.Content>
						<Field.FieldGroup>
							<Field.Field>
								<Field.Label for="parallelism">Parallelism</Field.Label>
								<Input
									id="parallelism"
									type="number"
									min="1"
									class="w-32"
									bind:value={parallelism}
								/>
								<Field.Description>
									Maximum number of scenarios executed concurrently per run.
								</Field.Description>
							</Field.Field>

							<Field.Field>
								<Field.Label for="terraform-version">Terraform version</Field.Label>
								<Input id="terraform-version" bind:value={terraformVersion} placeholder="Latest" />
								<Field.Description>
									Terraform CLI version used to apply simulation packs. Leave blank to use the
									latest available version.
								</Field.Description>
							</Field.Field>

							<Field.Separator />

							<Field.Field orientation="horizontal">
								<Field.Content>
									<Field.Label for="pack-logs">Pack logs</Field.Label>
									<Field.Description>
										Capture Terraform apply/destroy output for each pack run.
									</Field.Description>
								</Field.Content>
								<Switch
									id="pack-logs"
									checked={packLogsEnabled}
									onCheckedChange={(checked) => {
										packLogsEnabled = checked;
										toggleSwitch('pack_logs_enabled', checked, (v) => (generalError = v));
									}}
								/>
							</Field.Field>

							<Field.Field orientation="horizontal">
								<Field.Content>
									<Field.Label for="ssh-logging">SSH session logging</Field.Label>
									<Field.Description>
										Record SSH session transcripts for detonations that use SSH connectors.
									</Field.Description>
								</Field.Content>
								<Switch
									id="ssh-logging"
									checked={sshLoggingEnabled}
									onCheckedChange={(checked) => {
										sshLoggingEnabled = checked;
										toggleSwitch('ssh_logging_enabled', checked, (v) => (generalError = v));
									}}
								/>
							</Field.Field>
						</Field.FieldGroup>

						{#if generalError}
							<Alert.Root variant="destructive" class="mt-4">
								<Alert.Description>{generalError}</Alert.Description>
							</Alert.Root>
						{/if}
					</Card.Content>
					<Card.Footer class="justify-end">
						<Button onclick={saveGeneral} disabled={generalSaving}>
							{generalSaving ? 'Saving...' : 'Save'}
						</Button>
					</Card.Footer>
				</Card.Root>
			</Tabs.Content>

			<Tabs.Content value="tags" class="mt-4">
				<Card.Root class="animate-fade-up stagger-2">
					<Card.Header>
						<Card.Description>
							Org-wide tags applied to every pack's cloud resources. A pack can override an
							individual tag with its own value.
						</Card.Description>
					</Card.Header>
					<Card.Content>
						<KeyValueEditor value={defaultTags} onchange={(next) => (defaultTags = next)} />

						{#if tagsError}
							<Alert.Root variant="destructive" class="mt-4">
								<Alert.Description>{tagsError}</Alert.Description>
							</Alert.Root>
						{/if}
					</Card.Content>
					<Card.Footer class="justify-end">
						<Button onclick={saveTags} disabled={tagsSaving}>
							{tagsSaving ? 'Saving...' : 'Save'}
						</Button>
					</Card.Footer>
				</Card.Root>
			</Tabs.Content>

			<Tabs.Content value="retention" class="mt-4">
				<Card.Root class="animate-fade-up stagger-2">
					<Card.Header>
						<Card.Description>
							Control how long run logs and whole runs are kept before they are deleted
							automatically.
						</Card.Description>
					</Card.Header>
					<Card.Content>
						<Field.FieldGroup>
							<Field.Field orientation="horizontal">
								<Field.Content>
									<Field.Label for="log-retention-enabled">Delete old run logs</Field.Label>
									<Field.Description>
										Verbose per-run logs older than the retention period are removed; the run record
										is kept.
									</Field.Description>
								</Field.Content>
								<Switch
									id="log-retention-enabled"
									checked={runLogRetentionEnabled}
									onCheckedChange={(checked) => {
										runLogRetentionEnabled = checked;
										toggleSwitch('run_log_retention_enabled', checked, (v) => (retentionError = v));
									}}
								/>
							</Field.Field>
							<Field.Field>
								<Field.Label for="log-retention-days">Keep run logs for</Field.Label>
								<InputGroup.Root class="w-32">
									<InputGroup.Input
										id="log-retention-days"
										type="number"
										min="1"
										bind:value={runLogRetentionDays}
										disabled={!runLogRetentionEnabled}
									/>
									<InputGroup.Addon align="inline-end">
										<InputGroup.Text>days</InputGroup.Text>
									</InputGroup.Addon>
								</InputGroup.Root>
							</Field.Field>

							<Field.Separator />

							<Field.Field orientation="horizontal">
								<Field.Content>
									<Field.Label for="run-retention-enabled">Delete old runs</Field.Label>
									<Field.Description>
										Whole runs older than the retention period — results and collected logs included
										— are permanently deleted.
									</Field.Description>
								</Field.Content>
								<Switch
									id="run-retention-enabled"
									checked={runRetentionEnabled}
									onCheckedChange={(checked) => {
										runRetentionEnabled = checked;
										toggleSwitch('run_retention_enabled', checked, (v) => (retentionError = v));
									}}
								/>
							</Field.Field>
							<Field.Field>
								<Field.Label for="run-retention-days">Keep runs for</Field.Label>
								<InputGroup.Root class="w-32">
									<InputGroup.Input
										id="run-retention-days"
										type="number"
										min="1"
										bind:value={runRetentionDays}
										disabled={!runRetentionEnabled}
									/>
									<InputGroup.Addon align="inline-end">
										<InputGroup.Text>days</InputGroup.Text>
									</InputGroup.Addon>
								</InputGroup.Root>
							</Field.Field>
						</Field.FieldGroup>

						{#if retentionError}
							<Alert.Root variant="destructive" class="mt-4">
								<Alert.Description>{retentionError}</Alert.Description>
							</Alert.Root>
						{/if}
					</Card.Content>
					<Card.Footer class="justify-end">
						<Button onclick={saveRetention} disabled={retentionSaving}>
							{retentionSaving ? 'Saving...' : 'Save'}
						</Button>
					</Card.Footer>
				</Card.Root>
			</Tabs.Content>

			<Tabs.Content value="about" class="mt-4">
				<Card.Root class="animate-fade-up stagger-2">
					<Card.Content>
						{#if version}
							<div class="grid grid-cols-2 gap-4 text-sm md:grid-cols-4">
								<div>
									<span class="text-xs font-medium text-muted-foreground">Version</span>
									<p>{version.version}</p>
								</div>
								<div>
									<span class="text-xs font-medium text-muted-foreground">Commit</span>
									<p class="font-mono text-xs">{version.commit}</p>
								</div>
								<div>
									<span class="text-xs font-medium text-muted-foreground">Build Date</span>
									<p>{version.buildDate}</p>
								</div>
								<div>
									<span class="text-xs font-medium text-muted-foreground">Go Version</span>
									<p>{version.goVersion}</p>
								</div>
							</div>
						{/if}
					</Card.Content>
				</Card.Root>
			</Tabs.Content>
		</Tabs.Root>
	{/if}
</div>
