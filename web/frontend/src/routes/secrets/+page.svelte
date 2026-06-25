<script lang="ts">
	import { onMount } from 'svelte';
	import * as Alert from '$lib/components/ui/alert/index.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import * as Empty from '$lib/components/ui/empty/index.js';
	import * as Table from '$lib/components/ui/table/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Skeleton } from '$lib/components/ui/skeleton/index.js';
	import { secrets, loadSecrets } from '$lib/stores/secrets';
	import { createSecret, updateSecret, deleteSecret } from '$lib/api/client';
	import * as Tooltip from '$lib/components/ui/tooltip/index.js';
	import { formatUserEmail } from '$lib/utils/format';
	import type { SecretGroup, SecretEntryInput } from '$lib/types';
	import KeyRoundIcon from '@lucide/svelte/icons/key-round';
	import XIcon from '@lucide/svelte/icons/x';

	let loading = $state(true);
	let error = $state('');

	// Create dialog state
	let createDialogOpen = $state(false);
	let newName = $state('');
	let newDescription = $state('');
	let newEntries = $state<{ key: string; value: string }[]>([{ key: '', value: '' }]);
	let saving = $state(false);

	// Edit dialog state
	let editDialogOpen = $state(false);
	let editTarget = $state<SecretGroup | null>(null);
	let editName = $state('');
	let editDescription = $state('');
	let editEntries = $state<{ key: string; value: string; isExisting: boolean }[]>([]);
	let updating = $state(false);

	// Delete dialog state
	let deleteDialogOpen = $state(false);
	let deleteTarget = $state<SecretGroup | null>(null);
	let deleting = $state(false);

	onMount(async () => {
		try {
			await loadSecrets();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load secrets';
		} finally {
			loading = false;
		}
	});

	function addNewEntry() {
		newEntries = [...newEntries, { key: '', value: '' }];
	}

	function removeNewEntry(index: number) {
		newEntries = newEntries.filter((_, i) => i !== index);
	}

	async function handleCreate() {
		const validEntries = newEntries.filter((e) => e.key.trim() && e.value.trim());
		if (!newName.trim() || validEntries.length === 0) return;
		saving = true;
		error = '';
		try {
			await createSecret(
				newName.trim(),
				newDescription.trim(),
				validEntries.map((e) => ({ key: e.key.trim(), value: e.value.trim() }))
			);
			await loadSecrets();
			createDialogOpen = false;
			newName = '';
			newDescription = '';
			newEntries = [{ key: '', value: '' }];
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to create secret';
		} finally {
			saving = false;
		}
	}

	function openEdit(group: SecretGroup) {
		editTarget = group;
		editName = group.name;
		editDescription = group.description;
		editEntries = group.keys.map((k) => ({
			key: k,
			value: '',
			isExisting: true
		}));
		editDialogOpen = true;
	}

	function addEditEntry() {
		editEntries = [...editEntries, { key: '', value: '', isExisting: false }];
	}

	function removeEditEntry(index: number) {
		editEntries = editEntries.filter((_, i) => i !== index);
	}

	async function handleUpdate() {
		if (!editTarget || !editName.trim()) return;
		const entries: SecretEntryInput[] = editEntries
			.filter((e) => e.key.trim())
			.map((e) => ({
				key: e.key.trim(),
				value: e.isExisting && e.value === '' ? null : e.value
			}));
		if (entries.length === 0) return;
		updating = true;
		error = '';
		try {
			await updateSecret(editTarget.id, editName.trim(), editDescription.trim(), entries);
			await loadSecrets();
			editDialogOpen = false;
			editTarget = null;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Update failed';
		} finally {
			updating = false;
		}
	}

	function openDelete(group: SecretGroup) {
		deleteTarget = group;
		deleteDialogOpen = true;
	}

	async function handleDelete() {
		if (!deleteTarget) return;
		deleting = true;
		error = '';
		try {
			await deleteSecret(deleteTarget.id);
			await loadSecrets();
			deleteDialogOpen = false;
			deleteTarget = null;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Delete failed';
		} finally {
			deleting = false;
		}
	}
</script>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<h1 class="text-2xl font-bold">Secrets</h1>
		<Dialog.Root bind:open={createDialogOpen}>
			<Dialog.Trigger>
				{#snippet child({ props })}
					<Button {...props}>New Secret Group</Button>
				{/snippet}
			</Dialog.Trigger>
			<Dialog.Content class="sm:max-w-2xl">
				<Dialog.Header>
					<Dialog.Title>New Secret Group</Dialog.Title>
					<Dialog.Description
						>Create a named group of key-value secrets used as environment variables during runs.</Dialog.Description
					>
				</Dialog.Header>
				<div class="space-y-4">
					<Input placeholder="Group name (e.g. azure)" bind:value={newName} />
					<Input placeholder="Description (optional)" bind:value={newDescription} />
					<div class="space-y-2">
						<div class="flex items-center justify-between">
							<Label>Entries</Label>
							<Button variant="outline" size="sm" onclick={addNewEntry}>Add Entry</Button>
						</div>
						{#each newEntries as entry, i}
							<div class="flex gap-2">
								<Input
									placeholder="Key (e.g. ARM_CLIENT_SECRET)"
									bind:value={entry.key}
									class="flex-1"
								/>
								<Input
									placeholder="Value"
									type="password"
									bind:value={entry.value}
									class="flex-1"
								/>
								{#if newEntries.length > 1}
									<Button
										variant="ghost"
										size="sm"
										class="shrink-0 px-2"
										onclick={() => removeNewEntry(i)}
									>
										<XIcon size={16} />
									</Button>
								{/if}
							</div>
						{/each}
					</div>
				</div>
				<div class="flex justify-end gap-2 pt-4">
					<Button variant="outline" onclick={() => (createDialogOpen = false)}>Cancel</Button>
					<Button
						onclick={handleCreate}
						disabled={saving ||
							!newName.trim() ||
							!newEntries.some((e) => e.key.trim() && e.value.trim())}
					>
						{saving ? 'Creating...' : 'Create'}
					</Button>
				</div>
			</Dialog.Content>
		</Dialog.Root>
	</div>

	{#if error}
		<Alert.Root variant="destructive">
			<Alert.Description>{error}</Alert.Description>
		</Alert.Root>
	{/if}

	{#if loading}
		<div class="space-y-3">
			{#each Array(3) as _}
				<Skeleton class="h-12 w-full" />
			{/each}
		</div>
	{:else if $secrets.length === 0}
		<Empty.Root>
			<Empty.Header>
				<Empty.Media variant="icon">
					<KeyRoundIcon />
				</Empty.Media>
				<Empty.Title>No secrets configured</Empty.Title>
				<Empty.Description
					>Create secret groups to store API keys and credentials as environment variables for runs.</Empty.Description
				>
			</Empty.Header>
			<Empty.Content>
				<Button onclick={() => (createDialogOpen = true)}>New Secret Group</Button>
			</Empty.Content>
		</Empty.Root>
	{:else}
		<Table.Root class="animate-fade-up stagger-2">
			<Table.Header>
				<Table.Row>
					<Table.Head>Name</Table.Head>
					<Table.Head>Description</Table.Head>
					<Table.Head>Keys</Table.Head>
					<Table.Head>Updated</Table.Head>
					<Table.Head class="text-right">Actions</Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#each $secrets as group}
					<Table.Row>
						<Table.Cell class="font-medium">{group.name}</Table.Cell>
						<Table.Cell class="text-muted-foreground">{group.description || '-'}</Table.Cell>
						<Table.Cell>
							<span class="font-mono text-xs"
								>{group.keys.length} key{group.keys.length !== 1 ? 's' : ''}</span
							>
						</Table.Cell>
						<Table.Cell>
							<div class="flex flex-col">
								<span>{new Date(group.updatedAt).toLocaleDateString()}</span>
								{#if group.updatedBy && group.updatedBy !== 'anonymous'}
									<Tooltip.Root>
										<Tooltip.Trigger class="text-xs text-muted-foreground cursor-default w-fit">
											by {formatUserEmail(group.updatedBy)}
										</Tooltip.Trigger>
										<Tooltip.Content>{group.updatedBy}</Tooltip.Content>
									</Tooltip.Root>
								{/if}
							</div>
						</Table.Cell>
						<Table.Cell class="text-right">
							<div class="flex justify-end gap-2">
								<Button variant="outline" size="sm" onclick={() => openEdit(group)}>Edit</Button>
								<Button variant="destructive" size="sm" onclick={() => openDelete(group)}>
									Delete
								</Button>
							</div>
						</Table.Cell>
					</Table.Row>
				{/each}
			</Table.Body>
		</Table.Root>
	{/if}
</div>

<!-- Edit Dialog -->
<Dialog.Root bind:open={editDialogOpen}>
	<Dialog.Content class="sm:max-w-2xl">
		<Dialog.Header>
			<Dialog.Title>Edit Secret Group</Dialog.Title>
			<Dialog.Description
				>Update the secret group. Leave value fields empty to keep existing values.</Dialog.Description
			>
		</Dialog.Header>
		<div class="space-y-4">
			<Input placeholder="Group name" bind:value={editName} />
			<Input placeholder="Description (optional)" bind:value={editDescription} />
			<div class="space-y-2">
				<div class="flex items-center justify-between">
					<Label>Entries</Label>
					<Button variant="outline" size="sm" onclick={addEditEntry}>Add Entry</Button>
				</div>
				{#each editEntries as entry, i}
					<div class="flex gap-2">
						<Input
							placeholder="Key"
							bind:value={entry.key}
							class="flex-1"
							disabled={entry.isExisting}
						/>
						<Input
							placeholder={entry.isExisting ? '(unchanged)' : 'Value'}
							type="password"
							bind:value={entry.value}
							class="flex-1"
						/>
						<Button
							variant="ghost"
							size="sm"
							class="shrink-0 px-2"
							onclick={() => removeEditEntry(i)}
						>
							<XIcon size={16} />
						</Button>
					</div>
				{/each}
			</div>
		</div>
		<div class="flex justify-end gap-2 pt-4">
			<Button variant="outline" onclick={() => (editDialogOpen = false)}>Cancel</Button>
			<Button onclick={handleUpdate} disabled={updating || !editName.trim()}>
				{updating ? 'Updating...' : 'Update'}
			</Button>
		</div>
	</Dialog.Content>
</Dialog.Root>

<!-- Delete Confirmation Dialog -->
<Dialog.Root bind:open={deleteDialogOpen}>
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>Delete Secret Group</Dialog.Title>
			<Dialog.Description>
				Are you sure you want to delete "{deleteTarget?.name}"? This will remove all stored secrets
				in this group. This action cannot be undone.
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
