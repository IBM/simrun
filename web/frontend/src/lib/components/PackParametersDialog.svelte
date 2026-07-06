<script lang="ts">
	import * as Alert from '$lib/components/ui/alert/index.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import {
		getConfig,
		getPackManifest,
		getPackParameters,
		updatePackParameters,
		ValidationError
	} from '$lib/api/client';
	import SchemaForm from '$lib/components/SchemaForm.svelte';
	import { hasBlankEntry } from '$lib/components/KeyValueEditor.svelte';
	import type { AppConfig, PackManifest } from '$lib/types';

	let {
		open = $bindable(),
		packName,
		onclose,
		onsuccess
	}: {
		open: boolean;
		packName: string;
		onclose: () => void;
		onsuccess: () => void;
	} = $props();

	let loading = $state(true);
	let saving = $state(false);
	let error = $state('');
	let fieldErrors = $state<Record<string, string>>({});
	let values = $state<Record<string, unknown>>({});
	let schema = $state<Record<string, unknown> | undefined>(undefined);
	let unknownKeysFromServer = $state<string[]>([]);
	let orgDefaultTags = $state<Record<string, string>>({});

	$effect(() => {
		if (open && packName) {
			void load();
		}
	});

	async function load() {
		loading = true;
		error = '';
		fieldErrors = {};
		try {
			const [params, manifest, cfg] = await Promise.all([
				getPackParameters(packName),
				getPackManifest(packName).catch(() => undefined as PackManifest | undefined),
				// Inherited tags are display-only; a config fetch failure
				// should not block editing pack parameters.
				getConfig().catch(() => ({}) as AppConfig)
			]);
			values = params;
			schema = manifest?.params_schema;
			orgDefaultTags = extractStringMap(cfg['default_tags']);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load parameters';
		} finally {
			loading = false;
		}
	}

	// Org default_tags from app config; tolerates missing or malformed
	// values (validation server-side may not have run on legacy rows).
	function extractStringMap(v: unknown): Record<string, string> {
		if (!v || typeof v !== 'object' || Array.isArray(v)) return {};
		const out: Record<string, string> = {};
		for (const [k, val] of Object.entries(v)) {
			if (typeof val === 'string') out[k] = val;
		}
		return out;
	}

	// Reject string-map params (e.g. default_tags) that contain blank keys or
	// values. Terraform/cloud providers can't apply empty tag keys or values,
	// so saving them silently breaks every sim in the pack. Returns per-field
	// error messages keyed by param name.
	function validateMapParams(): Record<string, string> {
		const errs: Record<string, string> = {};
		const props =
			(schema?.properties as
				| Record<string, { type?: string; additionalProperties?: { type?: string } }>
				| undefined) ?? {};
		for (const [name, prop] of Object.entries(props)) {
			if (prop?.type !== 'object' || prop.additionalProperties?.type !== 'string') continue;
			const map = values[name];
			if (!map || typeof map !== 'object') continue;
			if (hasBlankEntry(map as Record<string, unknown>)) {
				errs[name] = 'Keys and values cannot be empty — remove or fill blank entries.';
			}
		}
		return errs;
	}

	async function handleSave() {
		const mapErrors = validateMapParams();
		if (Object.keys(mapErrors).length > 0) {
			fieldErrors = mapErrors;
			error = 'Tag keys and values cannot be empty.';
			return;
		}

		saving = true;
		error = '';
		fieldErrors = {};
		try {
			const resp = await updatePackParameters(packName, values);
			unknownKeysFromServer = resp.unknown_keys ?? [];
			onsuccess();
			onclose();
		} catch (e) {
			if (e instanceof ValidationError) {
				const next: Record<string, string> = {};
				for (const err of e.errors) {
					next[err.key] = err.message;
				}
				fieldErrors = next;
				error = e.message;
			} else {
				error = e instanceof Error ? e.message : 'Failed to save parameters';
			}
		} finally {
			saving = false;
		}
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-lg">
		<Dialog.Header>
			<Dialog.Title>Pack Parameters</Dialog.Title>
			<Dialog.Description>
				Configure parameters for "{packName}". These are passed to the pack on execution.
			</Dialog.Description>
		</Dialog.Header>

		{#if loading}
			<p class="text-sm text-muted-foreground">Loading...</p>
		{:else}
			<div class="space-y-4">
				<SchemaForm
					{schema}
					{values}
					errors={fieldErrors}
					inheritedDefaultTags={orgDefaultTags}
					onchange={(next) => (values = next)}
				/>

				{#if unknownKeysFromServer.length > 0}
					<Alert.Root>
						<Alert.Description class="text-xs">
							Saved keys not in schema: {unknownKeysFromServer.join(', ')}
						</Alert.Description>
					</Alert.Root>
				{/if}

				{#if error}
					<Alert.Root variant="destructive">
						<Alert.Description>{error}</Alert.Description>
					</Alert.Root>
				{/if}
			</div>

			<div class="flex justify-end gap-2 pt-4">
				<Button variant="outline" onclick={onclose}>Cancel</Button>
				<Button onclick={handleSave} disabled={saving}>
					{saving ? 'Saving...' : 'Save'}
				</Button>
			</div>
		{/if}
	</Dialog.Content>
</Dialog.Root>
