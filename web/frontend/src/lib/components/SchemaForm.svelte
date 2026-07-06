<script lang="ts">
	import * as Alert from '$lib/components/ui/alert/index.js';
	import * as Collapsible from '$lib/components/ui/collapsible/index.js';
	import * as Select from '$lib/components/ui/select/index.js';
	import * as Tooltip from '$lib/components/ui/tooltip/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import KeyValueEditor from '$lib/components/KeyValueEditor.svelte';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import InfoIcon from '@lucide/svelte/icons/info';
	import XIcon from '@lucide/svelte/icons/x';
	import AlertTriangleIcon from '@lucide/svelte/icons/alert-triangle';

	type Property = {
		type?: string;
		description?: string;
		default?: unknown;
		enum?: string[];
		additionalProperties?: { type?: string };
	};

	let {
		schema,
		values,
		errors = {},
		// Names that should be grouped under "Cloud defaults". Order here
		// drives render order inside the section — regions first, then
		// the broad default_tags knob at the bottom.
		builtinNames = ['aws_region', 'gcp_region', 'azure_location', 'default_tags'],
		// Org-wide default tags from app settings, shown read-only inside the
		// default_tags section. Never written back through onchange.
		inheritedDefaultTags = {},
		onchange
	}: {
		schema: Record<string, unknown> | undefined;
		values: Record<string, unknown>;
		errors?: Record<string, string>;
		builtinNames?: string[];
		inheritedDefaultTags?: Record<string, string>;
		onchange: (next: Record<string, unknown>) => void;
	} = $props();

	let properties = $derived<Record<string, Property>>(
		(schema?.properties as Record<string, Property> | undefined) ?? {}
	);
	let required = $derived<string[]>((schema?.required as string[] | undefined) ?? []);

	let { builtinEntries, customEntries } = $derived.by(() => {
		const builtinMap = new Map<string, Property>();
		const custom: [string, Property][] = [];
		for (const [name, prop] of Object.entries(properties)) {
			if (builtinNames.includes(name)) {
				builtinMap.set(name, prop);
			} else {
				custom.push([name, prop]);
			}
		}
		// Render built-ins in the order declared by builtinNames so
		// regions appear before the broader default_tags knob.
		const builtin: [string, Property][] = [];
		for (const name of builtinNames) {
			const prop = builtinMap.get(name);
			if (prop) builtin.push([name, prop]);
		}
		return { builtinEntries: builtin, customEntries: custom };
	});

	// Unknown saved keys: present in values but not declared in schema.
	let unknownKeys = $derived<string[]>(Object.keys(values).filter((k) => !(k in properties)));

	// Auto-expand the cloud defaults section if any built-in has a saved
	// value or org tags are inherited, otherwise start collapsed.
	// svelte-ignore state_referenced_locally
	let cloudOpen = $state(
		builtinEntries.some(([name]) => values[name] !== undefined) ||
			Object.keys(inheritedDefaultTags).length > 0
	);

	function update(name: string, value: unknown) {
		const next = { ...values, [name]: value };
		onchange(next);
	}

	function removeKey(name: string) {
		const next = { ...values };
		delete next[name];
		onchange(next);
	}

	function isRequired(name: string): boolean {
		return required.includes(name);
	}
</script>

{#snippet field(name: string, prop: Property)}
	{@const errorMsg = errors[name]}
	{@const value = values[name]}
	<div class="space-y-1.5">
		<div class="flex items-center gap-1.5">
			<Label for="schema-form-{name}" class="font-mono text-xs">
				{name}
				{#if isRequired(name)}
					<span class="text-destructive">*</span>
				{/if}
			</Label>
			{#if prop.description}
				<Tooltip.Root>
					<Tooltip.Trigger>
						<InfoIcon class="size-3 text-muted-foreground" />
					</Tooltip.Trigger>
					<Tooltip.Content>
						<p class="max-w-xs text-xs">{prop.description}</p>
					</Tooltip.Content>
				</Tooltip.Root>
			{/if}
		</div>

		{#if prop.type === 'boolean'}
			<Switch
				id="schema-form-{name}"
				checked={value === true}
				onCheckedChange={(checked) => update(name, checked)}
			/>
		{:else if prop.type === 'string' && prop.enum && prop.enum.length > 0}
			{@const stringValue = typeof value === 'string' ? value : ''}
			<Select.Root type="single" value={stringValue} onValueChange={(v) => update(name, v ?? '')}>
				<Select.Trigger class="w-full">
					{stringValue || (typeof prop.default === 'string' ? prop.default : 'Select...')}
				</Select.Trigger>
				<Select.Content>
					{#each prop.enum as opt}
						<Select.Item value={opt} label={opt} />
					{/each}
				</Select.Content>
			</Select.Root>
		{:else if prop.type === 'object' && prop.additionalProperties?.type === 'string'}
			<KeyValueEditor
				{value}
				inherited={name === 'default_tags' ? inheritedDefaultTags : {}}
				onchange={(next) => update(name, next)}
			/>
		{:else}
			{@const stringValue = typeof value === 'string' ? value : value == null ? '' : String(value)}
			<Input
				id="schema-form-{name}"
				value={stringValue}
				placeholder={prop.default != null ? String(prop.default) : ''}
				oninput={(e) => update(name, (e.target as HTMLInputElement).value)}
			/>
		{/if}

		{#if errorMsg}
			<p class="text-xs text-destructive">{errorMsg}</p>
		{/if}
	</div>
{/snippet}

<div class="space-y-4">
	{#if customEntries.length > 0}
		<div class="space-y-3">
			{#each customEntries as [name, prop] (name)}
				{@render field(name, prop)}
			{/each}
		</div>
	{/if}

	{#if builtinEntries.length > 0}
		<Collapsible.Root bind:open={cloudOpen}>
			<Collapsible.Trigger class="w-full">
				<div
					class="flex items-center gap-2 text-xs font-medium text-muted-foreground hover:text-foreground transition-colors"
				>
					<ChevronRightIcon
						size={14}
						class={cloudOpen ? 'rotate-90 transition-transform' : 'transition-transform'}
					/>
					<span>Cloud defaults</span>
				</div>
			</Collapsible.Trigger>
			<Collapsible.Content>
				<div class="space-y-3 pt-3 pl-5">
					{#each builtinEntries as [name, prop] (name)}
						{@render field(name, prop)}
					{/each}
				</div>
			</Collapsible.Content>
		</Collapsible.Root>
	{/if}

	{#if customEntries.length === 0 && builtinEntries.length === 0}
		<p class="text-xs text-muted-foreground">This pack has no declared parameters.</p>
	{/if}

	{#if unknownKeys.length > 0}
		<Alert.Root>
			<AlertTriangleIcon class="size-4" />
			<Alert.Description>
				<p class="text-xs mb-2">
					The following keys are saved but not declared in this pack's schema:
				</p>
				<ul class="space-y-1">
					{#each unknownKeys as key}
						<li class="flex items-center justify-between gap-2 text-xs">
							<code class="font-mono">{key}</code>
							<Button variant="ghost" size="sm" class="h-6 px-2" onclick={() => removeKey(key)}>
								<XIcon data-icon="inline-start" />
								Remove
							</Button>
						</li>
					{/each}
				</ul>
			</Alert.Description>
		</Alert.Root>
	{/if}
</div>
