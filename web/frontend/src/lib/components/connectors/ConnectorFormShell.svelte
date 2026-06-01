<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';

	let {
		name = $bindable(),
		description = $bindable(),
		isDefault = $bindable(),
		enabled = $bindable(),
		showIsDefault = false,
		showEnabled = false,
		namePlaceholder = '',
		idPrefix = '',
		children
	}: {
		name: string;
		description: string;
		isDefault: boolean;
		enabled: boolean;
		showIsDefault?: boolean;
		showEnabled?: boolean;
		namePlaceholder?: string;
		idPrefix?: string;
		children?: Snippet;
	} = $props();

	const nameId = $derived(idPrefix ? `${idPrefix}Name` : 'name');
	const descId = $derived(idPrefix ? `${idPrefix}Description` : 'description');
	const defaultId = $derived(idPrefix ? `${idPrefix}IsDefault` : 'isDefault');
	const enabledId = $derived(idPrefix ? `${idPrefix}Enabled` : 'enabled');
</script>

<div class="space-y-4">
	<div class="space-y-2">
		<Label for={nameId}>Name</Label>
		<Input id={nameId} placeholder={namePlaceholder} bind:value={name} />
	</div>
	<div class="space-y-2">
		<Label for={descId}>Description</Label>
		<Input id={descId} placeholder="Optional description" bind:value={description} />
	</div>

	{@render children?.()}

	{#if showEnabled}
		<div class="flex items-center gap-2">
			<Switch id={enabledId} bind:checked={enabled} />
			<Label for={enabledId}>Enabled</Label>
		</div>
	{/if}
	{#if showIsDefault}
		<div class="flex items-center gap-2">
			<Switch id={defaultId} bind:checked={isDefault} />
			<Label for={defaultId}>Default</Label>
		</div>
	{/if}
</div>
