<script lang="ts">
	import * as Alert from '$lib/components/ui/alert/index.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import {
		getPackManifest,
		getPackParameters,
		updatePackParameters,
		ValidationError
	} from '$lib/api/client';
	import SchemaForm from '$lib/components/SchemaForm.svelte';
	import type { PackManifest } from '$lib/types';

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
			const [params, manifest] = await Promise.all([
				getPackParameters(packName),
				getPackManifest(packName).catch(() => undefined as PackManifest | undefined)
			]);
			values = params;
			schema = manifest?.params_schema;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load parameters';
		} finally {
			loading = false;
		}
	}

	async function handleSave() {
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
	<Dialog.Content class="max-w-lg">
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
