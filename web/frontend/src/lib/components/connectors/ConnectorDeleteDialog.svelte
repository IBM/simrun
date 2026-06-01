<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { deleteConnector } from '$lib/api/client';
	import type { Connector } from '$lib/types';

	let {
		open = $bindable(),
		connector,
		onDeleted
	}: {
		open: boolean;
		connector: Connector | null;
		onDeleted: () => void;
	} = $props();

	let deleting = $state(false);
	let error = $state('');

	async function handleDelete() {
		if (!connector) return;
		deleting = true;
		error = '';
		try {
			await deleteConnector(connector.id);
			onDeleted();
			open = false;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Delete failed';
		} finally {
			deleting = false;
		}
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>Delete Connector</Dialog.Title>
			<Dialog.Description>
				Are you sure you want to delete "{connector?.name}"? This action cannot be undone.
			</Dialog.Description>
		</Dialog.Header>
		{#if error}
			<p class="text-sm text-destructive">{error}</p>
		{/if}
		<div class="flex justify-end gap-2 pt-4">
			<Button variant="outline" onclick={() => (open = false)}>Cancel</Button>
			<Button variant="destructive" onclick={handleDelete} disabled={deleting}>
				{deleting ? 'Deleting...' : 'Delete'}
			</Button>
		</div>
	</Dialog.Content>
</Dialog.Root>
