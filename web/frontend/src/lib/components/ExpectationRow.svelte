<script lang="ts">
	import * as Select from '$lib/components/ui/select/index.js';
	import * as Popover from '$lib/components/ui/popover/index.js';
	import * as Command from '$lib/components/ui/command/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import XIcon from '@lucide/svelte/icons/x';
	import CheckIcon from '@lucide/svelte/icons/check';
	import ChevronsUpDownIcon from '@lucide/svelte/icons/chevrons-up-down';
	import type { FormExpectation } from '$lib/utils/yaml-generator';
	import type { ElasticRule } from '$lib/types';
	import { cn } from '$lib/utils.js';
	import { tick } from 'svelte';

	let {
		expectation,
		elasticRules,
		onupdate,
		onremove,
		canRemove
	}: {
		expectation: FormExpectation;
		elasticRules: ElasticRule[];
		onupdate: (expectation: FormExpectation) => void;
		onremove: () => void;
		canRemove: boolean;
	} = $props();

	const matcherOptions = [
		{ value: 'elastic', label: 'Elastic Security Alert' },
		{ value: 'datadog', label: 'Datadog Security Signal' }
	];

	const severityOptions = [
		{ value: '', label: 'Any' },
		{ value: 'low', label: 'Low' },
		{ value: 'medium', label: 'Medium' },
		{ value: 'high', label: 'High' },
		{ value: 'critical', label: 'Critical' }
	];

	let matcherLabel = $derived(
		matcherOptions.find((o) => o.value === expectation.type)?.label ?? 'Select matcher'
	);

	let severityLabel = $derived(
		severityOptions.find((o) => o.value === expectation.severity)?.label ?? 'Any'
	);

	let alertNameComboboxOpen = $state(false);
	let alertNameTriggerRef = $state<HTMLButtonElement>(null!);

	let alertNameLabel = $derived(() => {
		if (!expectation.alertName) return 'Select a rule';
		return expectation.alertName;
	});

	let useCombobox = $derived(expectation.type === 'elastic' && elasticRules.length > 0);

	function handleAlertNameChange(e: Event) {
		onupdate({ ...expectation, alertName: (e.target as HTMLInputElement).value });
	}

	function handleSeverityInputChange(e: Event) {
		onupdate({ ...expectation, severity: (e.target as HTMLInputElement).value });
	}

	function handleTimeoutChange(e: Event) {
		onupdate({ ...expectation, timeout: (e.target as HTMLInputElement).value });
	}
</script>

<div class="flex items-start gap-2">
	<div class="grid flex-1 gap-2 sm:grid-cols-4">
		<div>
			<span class="mb-1 block text-xs font-medium text-muted-foreground">Matcher</span>
			<Select.Root
				type="single"
				value={expectation.type}
				onValueChange={(v) => {
					if (v)
						onupdate({
							...expectation,
							type: v as 'elastic' | 'datadog',
							severity: '',
							alertName: ''
						});
				}}
			>
				<Select.Trigger class="w-full">
					{matcherLabel}
				</Select.Trigger>
				<Select.Content>
					{#each matcherOptions as opt}
						<Select.Item value={opt.value} label={opt.label} />
					{/each}
				</Select.Content>
			</Select.Root>
		</div>
		<div>
			<span class="mb-1 block text-xs font-medium text-muted-foreground">Alert Name</span>
			{#if useCombobox}
				<Popover.Root bind:open={alertNameComboboxOpen}>
					<Popover.Trigger bind:ref={alertNameTriggerRef}>
						{#snippet child({ props })}
							<Button
								{...props}
								variant="outline"
								class="w-full justify-between font-normal"
								role="combobox"
								aria-expanded={alertNameComboboxOpen}
							>
								<span class="truncate">{alertNameLabel()}</span>
								<ChevronsUpDownIcon data-icon="inline-end" class="opacity-50" />
							</Button>
						{/snippet}
					</Popover.Trigger>
					<Popover.Content class="w-[--bits-popover-trigger-width] p-0" align="start">
						<Command.Root
							filter={(value, search) => {
								return value.toLowerCase().includes(search.toLowerCase()) ? 1 : 0;
							}}
						>
							<Command.Input placeholder="Search rules..." />
							<Command.List>
								<Command.Empty>No rule found.</Command.Empty>
								<Command.Group>
									{#each elasticRules as rule (rule.id)}
										<Command.Item
											value={rule.name}
											onSelect={() => {
												onupdate({ ...expectation, alertName: rule.name });
												alertNameComboboxOpen = false;
												tick().then(() => alertNameTriggerRef?.focus());
											}}
										>
											<CheckIcon
												class={cn(
													'mr-2 shrink-0',
													expectation.alertName !== rule.name && 'text-transparent'
												)}
												size={16}
											/>
											<span class="truncate">{rule.name}</span>
										</Command.Item>
									{/each}
								</Command.Group>
							</Command.List>
						</Command.Root>
					</Popover.Content>
				</Popover.Root>
			{:else}
				<Input
					placeholder="Alert rule name"
					value={expectation.alertName}
					oninput={handleAlertNameChange}
				/>
			{/if}
		</div>
		<div>
			<span class="mb-1 block text-xs font-medium text-muted-foreground">Severity</span>
			{#if expectation.type === 'elastic'}
				<Select.Root
					type="single"
					value={expectation.severity}
					onValueChange={(v) => {
						onupdate({ ...expectation, severity: v ?? '' });
					}}
				>
					<Select.Trigger class="w-full">
						{severityLabel}
					</Select.Trigger>
					<Select.Content>
						{#each severityOptions as opt}
							<Select.Item value={opt.value} label={opt.label} />
						{/each}
					</Select.Content>
				</Select.Root>
			{:else}
				<Input
					placeholder="Optional"
					value={expectation.severity}
					oninput={handleSeverityInputChange}
				/>
			{/if}
		</div>
		<div>
			<span class="mb-1 block text-xs font-medium text-muted-foreground">Timeout</span>
			<Input placeholder="5m" value={expectation.timeout} oninput={handleTimeoutChange} />
		</div>
	</div>
	<div class="pt-5">
		<Button
			variant="ghost"
			size="icon"
			onclick={onremove}
			disabled={!canRemove}
			aria-label="Remove expectation"
		>
			<XIcon size={16} />
		</Button>
	</div>
</div>
