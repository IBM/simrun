<script lang="ts">
	import { onMount } from 'svelte';
	import * as Alert from '$lib/components/ui/alert/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import { Skeleton } from '$lib/components/ui/skeleton/index.js';
	import { getConfig, updateConfig, getVersion } from '$lib/api/client';
	import type { AppConfig, VersionInfo } from '$lib/types';

	let loading = $state(true);
	let error = $state('');
	let config = $state<AppConfig>({});
	let editedValues = $state<Record<string, string>>({});
	let revealedKeys = $state<Set<string>>(new Set());
	let saving = $state<Record<string, boolean>>({});
	let version = $state<VersionInfo | null>(null);

	const sensitivePatterns = ['key', 'secret', 'password', 'token', 'credential'];

	function isSensitive(key: string): boolean {
		const lower = key.toLowerCase();
		return sensitivePatterns.some((p) => lower.includes(p));
	}

	function displayValue(key: string, value: unknown): string {
		const str = typeof value === 'string' ? value : JSON.stringify(value);
		if (isSensitive(key) && !revealedKeys.has(key)) {
			return '********';
		}
		return str;
	}

	function toggleReveal(key: string) {
		const next = new Set(revealedKeys);
		if (next.has(key)) {
			next.delete(key);
		} else {
			next.add(key);
		}
		revealedKeys = next;
	}

	onMount(async () => {
		try {
			const [cfg, ver] = await Promise.all([getConfig(), getVersion()]);
			config = cfg;
			version = ver;
			editedValues = {};
			for (const [key, value] of Object.entries(cfg)) {
				editedValues[key] = typeof value === 'string' ? value : JSON.stringify(value);
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load config';
		} finally {
			loading = false;
		}
	});

	async function handleSave(key: string) {
		saving = { ...saving, [key]: true };
		error = '';
		try {
			let value: unknown = editedValues[key];
			try {
				value = JSON.parse(editedValues[key]);
			} catch {
				// keep as string
			}
			await updateConfig(key, value);
			config = { ...config, [key]: value };
		} catch (e) {
			error = e instanceof Error ? e.message : 'Save failed';
		} finally {
			saving = { ...saving, [key]: false };
		}
	}
</script>

<div class="space-y-6">
	<h1 class="text-2xl font-bold">Configuration</h1>

	{#if error}
		<Alert.Root variant="destructive">
			<Alert.Description>{error}</Alert.Description>
		</Alert.Root>
	{/if}

	{#if loading}
		<div class="space-y-4">
			<Skeleton class="h-64 w-full rounded-xl" />
			<Skeleton class="h-24 w-full rounded-xl" />
		</div>
	{:else}
		<Card.Root class="animate-fade-up stagger-2">
			<Card.Header>
				<Card.Title>Settings</Card.Title>
				<Card.Description>Manage simrun configuration values</Card.Description>
			</Card.Header>
			<Card.Content>
				<div class="space-y-4">
					{#each Object.entries(config) as [key, value]}
						<div class="flex items-center gap-3">
							<Label for="config-{key}" class="min-w-[200px] font-mono">{key}</Label>
							<Input
								id="config-{key}"
								type={isSensitive(key) && !revealedKeys.has(key) ? 'password' : 'text'}
								bind:value={editedValues[key]}
								class="flex-1"
							/>
							{#if isSensitive(key)}
								<Button variant="outline" size="sm" onclick={() => toggleReveal(key)}>
									{revealedKeys.has(key) ? 'Hide' : 'Show'}
								</Button>
							{/if}
							<Button size="sm" onclick={() => handleSave(key)} disabled={saving[key]}>
								{saving[key] ? 'Saving...' : 'Save'}
							</Button>
						</div>
					{/each}
				</div>
			</Card.Content>
		</Card.Root>

		{#if version}
			<Separator />
			<Card.Root class="animate-fade-up stagger-3">
				<Card.Header>
					<Card.Title>Version Info</Card.Title>
				</Card.Header>
				<Card.Content>
					<div class="grid grid-cols-2 gap-3 text-sm md:grid-cols-4">
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
				</Card.Content>
			</Card.Root>
		{/if}
	{/if}
</div>
