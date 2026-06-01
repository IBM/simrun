<script lang="ts">
	import * as Select from '$lib/components/ui/select/index.js';
	import * as Popover from '$lib/components/ui/popover/index.js';
	import * as Command from '$lib/components/ui/command/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import ExpectationRow from './ExpectationRow.svelte';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import { createEmptyExpectation } from '$lib/utils/yaml-generator';
	import type { FormScenario, FormExpectation, FormParam } from '$lib/utils/yaml-generator';
	import type {
		Pack,
		PackManifest,
		SimulationManifest,
		TemplateManifest,
		ElasticRule
	} from '$lib/types';
	import { extractTerraformOutputs } from '$lib/utils/terraform';
	import { cn } from '$lib/utils.js';
	import { tick } from 'svelte';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import CopyIcon from '@lucide/svelte/icons/copy';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import XIcon from '@lucide/svelte/icons/x';
	import CheckIcon from '@lucide/svelte/icons/check';
	import ChevronsUpDownIcon from '@lucide/svelte/icons/chevrons-up-down';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import ShieldAlertIcon from '@lucide/svelte/icons/shield-alert';
	import SyringeIcon from '@lucide/svelte/icons/syringe';

	type ComboboxItem = {
		id: string;
		name: string;
		scope: string;
		kind: 'simulation' | 'template';
	};

	import type { ScenarioType } from '$lib/types';

	let {
		scenario,
		index,
		packs,
		packManifests,
		elasticRules,
		scenarioFileType = 'standard',
		onupdate,
		onremove,
		onduplicate,
		canRemove,
		onpackchange,
		initiallyCollapsed = false
	}: {
		scenario: FormScenario;
		index: number;
		packs: Pack[];
		packManifests: Map<string, PackManifest>;
		elasticRules: ElasticRule[];
		scenarioFileType?: ScenarioType;
		onupdate: (scenario: FormScenario) => void;
		onremove: () => void;
		onduplicate: () => void;
		canRemove: boolean;
		onpackchange: (packName: string) => void;
		initiallyCollapsed?: boolean;
	} = $props();

	// svelte-ignore state_referenced_locally
	let open = $state(!initiallyCollapsed);

	let simulationComboboxOpen = $state(false);
	let simulationTriggerRef = $state<HTMLButtonElement>(null!);

	let manifest = $derived<PackManifest | undefined>(
		scenario.pack ? packManifests.get(scenario.pack) : undefined
	);

	let simulations = $derived<SimulationManifest[]>(manifest?.simulations ?? []);
	let templates = $derived<TemplateManifest[]>(manifest?.templates ?? []);

	let comboboxItems = $derived<ComboboxItem[]>([
		...simulations.map((s) => ({
			id: s.id,
			name: s.name,
			scope: s.scope,
			kind: 'simulation' as const
		})),
		...templates.map((t) => ({
			id: t.id,
			name: t.name,
			scope: t.scope,
			kind: 'template' as const
		}))
	]);

	let selectedId = $derived(
		scenario.scenarioType === 'inject' ? scenario.template : scenario.simulation
	);

	let selectedSimulation = $derived<SimulationManifest | undefined>(
		simulations.find((s) => s.id === scenario.simulation)
	);

	let selectedTemplate = $derived<TemplateManifest | undefined>(
		templates.find((t) => t.id === scenario.template)
	);

	// Extract available param keys from params_schema
	let availableParams = $derived.by(() => {
		if (!selectedSimulation?.params_schema) return [] as { key: string; description: string }[];
		const schema = selectedSimulation.params_schema as {
			properties?: Record<string, { description?: string }>;
		};
		if (!schema.properties) return [] as { key: string; description: string }[];
		return Object.entries(schema.properties).map(([key, prop]) => ({
			key,
			description: prop.description || ''
		}));
	});

	// Extract terraform output names from the simulation
	let terraformOutputs = $derived<string[]>(
		selectedSimulation?.terraform ? extractTerraformOutputs(selectedSimulation.terraform) : []
	);

	let loadingManifest = $derived(scenario.pack !== '' && !packManifests.has(scenario.pack));

	let packLabel = $derived(scenario.pack || 'Select a pack');

	let comboboxLabel = $derived.by(() => {
		if (!scenario.pack) return 'Select a pack first';
		if (loadingManifest) return 'Loading...';
		if (!selectedId) return 'Select a simulation or template';
		const item = comboboxItems.find((i) => i.id === selectedId);
		return item ? `${item.name} (${item.id})` : selectedId;
	});

	let isInject = $derived(scenario.scenarioType === 'inject');

	function handleNameChange(e: Event) {
		onupdate({ ...scenario, name: (e.target as HTMLInputElement).value });
	}

	function handleComboboxSelect(item: ComboboxItem) {
		if (item.kind === 'simulation') {
			const sim = simulations.find((s) => s.id === item.id);

			// Initialize params from schema if available
			let params: FormParam[] = [];
			if (sim?.params_schema) {
				const schema = sim.params_schema as { properties?: Record<string, unknown> };
				if (schema.properties) {
					params = Object.keys(schema.properties).map((key) => ({ key, value: '' }));
				}
			}

			onupdate({
				...scenario,
				scenarioType: 'detonate',
				simulation: item.id,
				template: '',
				injectIndex: '',
				templateVars: [],
				params,
				indicators: {
					terraformOutput: [],
					static: []
				}
			});
		} else {
			const tmpl = templates.find((t) => t.id === item.id);
			let templateVars: FormParam[] = [];
			if (tmpl?.vars) {
				templateVars = Object.keys(tmpl.vars).map((key) => ({ key, value: '' }));
			}

			onupdate({
				...scenario,
				scenarioType: 'inject',
				template: item.id,
				simulation: '',
				injectIndex: '',
				templateVars,
				params: [],
				indicators: {
					terraformOutput: [],
					static: []
				},
				collection: { enabled: false, type: '', index: '', additionalFields: [] }
			});
		}
	}

	function handleParamChange(paramIndex: number, value: string) {
		const newParams = [...scenario.params];
		newParams[paramIndex] = { ...newParams[paramIndex], value };
		onupdate({ ...scenario, params: newParams });
	}

	function addParam() {
		onupdate({ ...scenario, params: [...scenario.params, { key: '', value: '' }] });
	}

	function removeParam(paramIndex: number) {
		onupdate({ ...scenario, params: scenario.params.filter((_, i) => i !== paramIndex) });
	}

	function handleParamKeyChange(paramIndex: number, key: string) {
		const newParams = [...scenario.params];
		newParams[paramIndex] = { ...newParams[paramIndex], key };
		onupdate({ ...scenario, params: newParams });
	}

	function handleTemplateVarChange(varIndex: number, field: 'key' | 'value', val: string) {
		const newVars = [...scenario.templateVars];
		newVars[varIndex] = { ...newVars[varIndex], [field]: val };
		onupdate({ ...scenario, templateVars: newVars });
	}

	function handleInjectIndexChange(e: Event) {
		onupdate({ ...scenario, injectIndex: (e.target as HTMLInputElement).value });
	}

	function toggleTerraformOutput(output: string) {
		const current = scenario.indicators.terraformOutput;
		const newOutputs = current.includes(output)
			? current.filter((o) => o !== output)
			: [...current, output];
		onupdate({
			...scenario,
			indicators: { ...scenario.indicators, terraformOutput: newOutputs }
		});
	}

	function addStaticIndicator() {
		onupdate({
			...scenario,
			indicators: { ...scenario.indicators, static: [...scenario.indicators.static, ''] }
		});
	}

	function updateStaticIndicator(i: number, value: string) {
		const newStatic = [...scenario.indicators.static];
		newStatic[i] = value;
		onupdate({ ...scenario, indicators: { ...scenario.indicators, static: newStatic } });
	}

	function removeStaticIndicator(i: number) {
		onupdate({
			...scenario,
			indicators: {
				...scenario.indicators,
				static: scenario.indicators.static.filter((_, idx) => idx !== i)
			}
		});
	}

	function handleExpectationUpdate(expIndex: number, expectation: FormExpectation) {
		const newExpectations = [...scenario.expectations];
		newExpectations[expIndex] = expectation;
		onupdate({ ...scenario, expectations: newExpectations });
	}

	function handleExpectationRemove(expIndex: number) {
		const newExpectations = scenario.expectations.filter((_, i) => i !== expIndex);
		onupdate({ ...scenario, expectations: newExpectations });
	}

	function addExpectation() {
		onupdate({
			...scenario,
			expectations: [...scenario.expectations, createEmptyExpectation()]
		});
	}

	const collectorTypeOptions = [{ value: 'elastic', label: 'Elastic Collector' }];

	let collectorTypeLabel = $derived(
		collectorTypeOptions.find((o) => o.value === scenario.collection.type)?.label ??
			'Select collector'
	);


	function handleCollectionIndexChange(e: Event) {
		onupdate({
			...scenario,
			collection: { ...scenario.collection, index: (e.target as HTMLInputElement).value }
		});
	}

	function addCollectionField() {
		onupdate({
			...scenario,
			collection: {
				...scenario.collection,
				additionalFields: [...scenario.collection.additionalFields, { key: '', value: '' }]
			}
		});
	}

	function updateCollectionField(fieldIndex: number, field: 'key' | 'value', newValue: string) {
		const newFields = [...scenario.collection.additionalFields];
		newFields[fieldIndex] = { ...newFields[fieldIndex], [field]: newValue };
		onupdate({
			...scenario,
			collection: { ...scenario.collection, additionalFields: newFields }
		});
	}

	function removeCollectionField(fieldIndex: number) {
		onupdate({
			...scenario,
			collection: {
				...scenario.collection,
				additionalFields: scenario.collection.additionalFields.filter((_, i) => i !== fieldIndex)
			}
		});
	}
</script>

<div
	class={cn(
		'rounded-lg border border-border bg-card text-card-foreground shadow-sm',
		!scenario.enabled && 'opacity-60'
	)}
>
	<button
		type="button"
		class="flex w-full items-center justify-between px-4 py-2.5 hover:bg-accent/50 transition-colors rounded-lg"
		onclick={() => (open = !open)}
		aria-expanded={open}
	>
		<div class="flex items-center gap-2 min-w-0">
			<ChevronRightIcon
				class={cn('transition-transform duration-200 shrink-0', open && 'rotate-90')}
				size={16}
			/>
			<span class="text-sm font-medium truncate">{scenario.name || `Scenario ${index + 1}`}</span>
			{#if !open && scenario.pack}
				<span class="text-xs text-muted-foreground truncate"
					>{scenario.pack}{selectedId ? ` / ${selectedId}` : ''}</span
				>
				{#if isInject}
					<Badge variant="outline" class="text-[10px] px-1.5 py-0 gap-1 bg-attr-asset/10 text-attr-asset border-attr-asset/30">
						<SyringeIcon class="size-2.5" />
						inject
					</Badge>
				{/if}
			{/if}
		</div>
		<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div class="flex items-center gap-1 shrink-0" onclick={(e) => e.stopPropagation()}>
			<Switch
				checked={scenario.enabled}
				onCheckedChange={(checked) => onupdate({ ...scenario, enabled: checked })}
				aria-label="Enable scenario"
			/>
			<Button variant="ghost" size="sm" onclick={onduplicate} aria-label="Duplicate scenario">
				<CopyIcon size={16} class="mr-1" />
				Duplicate
			</Button>
			{#if canRemove}
				<Button variant="ghost" size="sm" onclick={onremove} aria-label="Remove scenario">
					<Trash2Icon size={16} class="mr-1" />
					Remove
				</Button>
			{/if}
		</div>
	</button>
	<div class="scenario-collapse-grid" class:scenario-collapse-open={open}>
		<div class="overflow-hidden min-h-0">
			<div class="space-y-4 px-4 pb-4 pt-2">
				<div>
					<Label for="scenario-name-{index}" class="mb-1 block">Scenario Name</Label>
					<Input
						id="scenario-name-{index}"
						placeholder="e.g., Detect S3 exfiltration"
						value={scenario.name}
						oninput={handleNameChange}
					/>
				</div>

				<div class="grid gap-4 sm:grid-cols-2">
					<div>
						<Label class="mb-1 block">Pack</Label>
						<Select.Root
							type="single"
							value={scenario.pack}
							onValueChange={(v) => {
								const packName = v ?? '';
								onupdate({
									...scenario,
									pack: packName,
									scenarioType: 'detonate',
									simulation: '',
									template: '',
									injectIndex: '',
									templateVars: [],
									params: [],
									indicators: { terraformOutput: [], static: [] },
									collection: { enabled: false, type: '', index: '', additionalFields: [] }
								});
								if (packName) onpackchange(packName);
							}}
						>
							<Select.Trigger class="w-full">
								{packLabel}
							</Select.Trigger>
							<Select.Content>
								{#each packs as pack}
									<Select.Item value={pack.name} label={pack.name} />
								{/each}
							</Select.Content>
						</Select.Root>
					</div>
					<div>
						<Label class="mb-1 block">Simulation / Template</Label>
						<Popover.Root bind:open={simulationComboboxOpen}>
							<Popover.Trigger bind:ref={simulationTriggerRef}>
								{#snippet child({ props })}
									<Button
										{...props}
										variant="outline"
										class="w-full justify-between font-normal"
										role="combobox"
										aria-expanded={simulationComboboxOpen}
										disabled={!scenario.pack || loadingManifest}
									>
										<span class="truncate">{comboboxLabel}</span>
										<ChevronsUpDownIcon class="ml-2 shrink-0 opacity-50" size={16} />
									</Button>
								{/snippet}
							</Popover.Trigger>
							<Popover.Content class="w-[--bits-popover-trigger-width] p-0" align="start">
								<Command.Root>
									<Command.Input placeholder="Search simulations & templates..." />
									<Command.List>
										<Command.Empty>No items found.</Command.Empty>
										{#if simulations.length > 0}
											<Command.Group heading="Simulations">
												{#each simulations as sim (sim.id)}
													<Command.Item
														value="{sim.name} ({sim.id})"
														onSelect={() => {
															handleComboboxSelect({
																id: sim.id,
																name: sim.name,
																scope: sim.scope,
																kind: 'simulation'
															});
															simulationComboboxOpen = false;
															tick().then(() => simulationTriggerRef?.focus());
														}}
													>
														<CheckIcon
															class={cn(
																'mr-2 shrink-0',
																selectedId !== sim.id && 'text-transparent'
															)}
															size={16}
														/>
														<ShieldAlertIcon class="mr-1.5 size-3.5 text-muted-foreground shrink-0" />
														<span class="truncate"
															>{sim.name} <span class="text-muted-foreground">({sim.id})</span></span
														>
													</Command.Item>
												{/each}
											</Command.Group>
										{/if}
										{#if templates.length > 0}
											<Command.Group heading="Injection Templates">
												{#each templates as tmpl (tmpl.id)}
													<Command.Item
														value="{tmpl.name} ({tmpl.id})"
														onSelect={() => {
															handleComboboxSelect({
																id: tmpl.id,
																name: tmpl.name,
																scope: tmpl.scope,
																kind: 'template'
															});
															simulationComboboxOpen = false;
															tick().then(() => simulationTriggerRef?.focus());
														}}
													>
														<CheckIcon
															class={cn(
																'mr-2 shrink-0',
																selectedId !== tmpl.id && 'text-transparent'
															)}
															size={16}
														/>
														<SyringeIcon class="mr-1.5 size-3.5 text-attr-asset shrink-0" />
														<span class="truncate"
															>{tmpl.name} <span class="text-muted-foreground">({tmpl.id})</span></span
														>
													</Command.Item>
												{/each}
											</Command.Group>
										{/if}
									</Command.List>
								</Command.Root>
							</Popover.Content>
						</Popover.Root>
					</div>
				</div>

				{#if selectedId}
					{#if isInject}
						<!-- Inject-specific fields -->
						<Separator />
						<div class="space-y-3">
							<h4 class="text-sm font-medium">Injection Configuration</h4>
							<div>
								<Label class="text-xs mb-1 block">Datastream</Label>
								<div class="flex items-center gap-0">
									<span class="shrink-0 rounded-l-md border border-r-0 border-input bg-muted px-2.5 py-2 text-xs font-mono text-muted-foreground">logs-</span>
									<Input
										placeholder="e.g., okta.system"
										value={scenario.injectIndex}
										oninput={handleInjectIndexChange}
										class="font-mono text-sm rounded-l-none border-l-0 rounded-r-none"
									/>
									<span class="shrink-0 rounded-r-md border border-l-0 border-input bg-muted px-2.5 py-2 text-xs font-mono text-muted-foreground">-default</span>
								</div>
							</div>
						</div>

						<!-- Template Variables -->
					{#if scenario.templateVars.length > 0}
						<Separator />
						<div class="space-y-3">
							<h4 class="text-sm font-medium">Template Variables</h4>
							{#each scenario.templateVars as tvar, vi}
								<div class="flex items-start gap-2">
									<div class="grid flex-1 gap-2 sm:grid-cols-2">
										<Input
											value={tvar.key}
											disabled
											class="font-mono text-xs"
										/>
										<Input
											placeholder={selectedTemplate?.vars?.[tvar.key] || 'Value'}
											value={tvar.value}
											oninput={(e) =>
												handleTemplateVarChange(vi, 'value', (e.target as HTMLInputElement).value)}
										/>
									</div>
								</div>
							{/each}
						</div>
					{/if}

						<!-- Inject: Static indicators -->
						<Separator />
						<div class="space-y-3">
							<div class="flex items-center justify-between">
								<h4 class="text-sm font-medium">Indicators</h4>
								<Button variant="outline" size="sm" onclick={addStaticIndicator}>
									<PlusIcon size={14} class="mr-1" />
									Add Static Indicator
								</Button>
							</div>
							{#each scenario.indicators.static as indicator, si}
								<div class="flex gap-2">
									<Input
										placeholder="e.g., 192.168.1.1"
										value={indicator}
										oninput={(e) =>
											updateStaticIndicator(si, (e.target as HTMLInputElement).value)}
										class="flex-1"
									/>
									<Button
										variant="ghost"
										size="icon"
										onclick={() => removeStaticIndicator(si)}
										aria-label="Remove indicator"
									>
										<XIcon size={16} />
									</Button>
								</div>
							{/each}
							{#if scenario.indicators.static.length === 0}
								<p class="text-xs text-muted-foreground">
									Add static indicators if needed for alert correlation.
								</p>
							{/if}
						</div>
					{:else}
						<!-- Detonate-specific: Parameters Section -->
						{#if availableParams.length > 0 || scenario.params.length > 0}
							<Separator />
							<div class="space-y-3">
								<div class="flex items-center justify-between">
									<h4 class="text-sm font-medium">Parameters</h4>
									<Button variant="outline" size="sm" onclick={addParam}>
										<PlusIcon size={14} class="mr-1" />
										Add Param
									</Button>
								</div>
								{#each scenario.params as param, pi}
									<div class="flex items-start gap-2">
										<div class="grid flex-1 gap-2 sm:grid-cols-2">
											<div>
												{#if availableParams.find((p) => p.key === param.key)}
													<Input value={param.key} disabled class="font-mono text-xs" />
												{:else}
													<Input
														placeholder="Parameter key"
														value={param.key}
														oninput={(e) =>
															handleParamKeyChange(pi, (e.target as HTMLInputElement).value)}
														class="font-mono text-xs"
													/>
												{/if}
											</div>
											<div>
												<Input
													placeholder={availableParams.find((p) => p.key === param.key)
														?.description || 'Value'}
													value={param.value}
													oninput={(e) =>
														handleParamChange(pi, (e.target as HTMLInputElement).value)}
												/>
											</div>
										</div>
										<Button
											variant="ghost"
											size="icon"
											onclick={() => removeParam(pi)}
											aria-label="Remove parameter"
										>
											<XIcon size={16} />
										</Button>
									</div>
								{/each}
								{#if scenario.params.length === 0}
									<p class="text-xs text-muted-foreground">
										No parameters configured. Click "Add Param" to add custom parameters.
									</p>
								{/if}
							</div>
						{/if}

						<!-- Detonate-specific: Indicators Section -->
						{#if terraformOutputs.length > 0 || scenario.indicators.static.length > 0}
							<Separator />
							<div class="space-y-3">
								<h4 class="text-sm font-medium">Indicators</h4>

								{#if terraformOutputs.length > 0}
									<div class="space-y-2">
										<Label class="text-xs">Terraform Outputs</Label>
										<div class="flex flex-wrap gap-1.5">
											{#each terraformOutputs as output}
												<button type="button" onclick={() => toggleTerraformOutput(output)}>
													<Badge
														variant={scenario.indicators.terraformOutput.includes(output)
															? 'default'
															: 'outline'}
														class="cursor-pointer font-mono text-xs"
													>
														{output}
													</Badge>
												</button>
											{/each}
										</div>
										<p class="text-xs text-muted-foreground">
											Click to toggle which terraform outputs to include as indicators.
										</p>
									</div>
								{/if}

								<div class="space-y-2">
									<div class="flex items-center justify-between">
										<Label class="text-xs">Static Indicators</Label>
										<Button variant="outline" size="sm" onclick={addStaticIndicator}>
											<PlusIcon size={14} class="mr-1" />
											Add
										</Button>
									</div>
									{#each scenario.indicators.static as indicator, si}
										<div class="flex gap-2">
											<Input
												placeholder="e.g., 192.168.1.1"
												value={indicator}
												oninput={(e) =>
													updateStaticIndicator(si, (e.target as HTMLInputElement).value)}
												class="flex-1"
											/>
											<Button
												variant="ghost"
												size="icon"
												onclick={() => removeStaticIndicator(si)}
												aria-label="Remove indicator"
											>
												<XIcon size={16} />
											</Button>
										</div>
									{/each}
								</div>
							</div>
						{:else}
							<Separator />
							<div class="space-y-3">
								<div class="flex items-center justify-between">
									<h4 class="text-sm font-medium">Indicators</h4>
									<Button variant="outline" size="sm" onclick={addStaticIndicator}>
										<PlusIcon size={14} class="mr-1" />
										Add Static Indicator
									</Button>
								</div>
								{#each scenario.indicators.static as indicator, si}
									<div class="flex gap-2">
										<Input
											placeholder="e.g., 192.168.1.1"
											value={indicator}
											oninput={(e) =>
												updateStaticIndicator(si, (e.target as HTMLInputElement).value)}
											class="flex-1"
										/>
										<Button
											variant="ghost"
											size="icon"
											onclick={() => removeStaticIndicator(si)}
											aria-label="Remove indicator"
										>
											<XIcon size={16} />
										</Button>
									</div>
								{/each}
								{#if scenario.indicators.static.length === 0}
									<p class="text-xs text-muted-foreground">
										No terraform outputs found for this simulation. Add static indicators if needed.
									</p>
								{/if}
							</div>
						{/if}
					{/if}

					<!-- Log Collection Section (collect type only) -->
					{#if !isInject && scenarioFileType === 'collect'}
						<Separator />
						<div class="space-y-3">
							<h4 class="text-sm font-medium">Log Collection</h4>

							<div class="space-y-3">
								<div>
									<Label class="text-xs mb-1 block">Collector</Label>
									<Select.Root
										type="single"
										value={scenario.collection.type}
										onValueChange={(v) => {
											if (v)
												onupdate({
													...scenario,
													collection: { ...scenario.collection, type: v as 'elastic' }
												});
										}}
									>
										<Select.Trigger class="w-full">
											{collectorTypeLabel}
										</Select.Trigger>
										<Select.Content>
											{#each collectorTypeOptions as opt}
												<Select.Item value={opt.value} label={opt.label} />
											{/each}
										</Select.Content>
									</Select.Root>
								</div>

								{#if scenario.collection.type}
									<div>
										<Label class="text-xs mb-1 block">Elasticsearch Index</Label>
										<Input
											placeholder="e.g., logs-aws.cloudtrail-*"
											value={scenario.collection.index}
											oninput={handleCollectionIndexChange}
											class="font-mono text-sm"
										/>
										<p class="text-xs text-muted-foreground mt-1">
											Index pattern to search for logs after detonation
										</p>
									</div>

									<div class="space-y-2">
										<div class="flex items-center justify-between">
											<Label class="text-xs">Additional Query Fields</Label>
											<Button variant="outline" size="sm" onclick={addCollectionField}>
												<PlusIcon size={14} class="mr-1" />
												Add Field
											</Button>
										</div>
										<p class="text-xs text-muted-foreground">
											Additional fields to match. Use <code
												class="bg-muted px-1 py-0.5 rounded text-xs"
												>{'{{ indicators.terraformOutput.key }}'}</code
											> for dynamic values.
										</p>
										{#each scenario.collection.additionalFields as field, fi}
											<div class="flex items-center gap-2">
												<Input
													placeholder="Field name (e.g., cloud.account.id)"
													value={field.key}
													oninput={(e) =>
														updateCollectionField(
															fi,
															'key',
															(e.target as HTMLInputElement).value
														)}
													class="flex-1 font-mono text-xs"
												/>
												<span class="text-muted-foreground">=</span>
												<Input
													placeholder="Value or template"
													value={field.value}
													oninput={(e) =>
														updateCollectionField(
															fi,
															'value',
															(e.target as HTMLInputElement).value
														)}
													class="flex-1 font-mono text-xs"
												/>
												<Button
													variant="ghost"
													size="icon"
													onclick={() => removeCollectionField(fi)}
													aria-label="Remove field"
												>
													<XIcon size={16} />
												</Button>
											</div>
										{/each}
									</div>
								{/if}
							</div>
						</div>
					{/if}
				{/if}

				{#if scenarioFileType === 'standard'}
				<Separator />

				<div class="space-y-3">
					<div class="flex items-center justify-between">
						<h4 class="text-sm font-medium">Expectations</h4>
						<Button variant="outline" size="sm" onclick={addExpectation}>
							<PlusIcon size={14} class="mr-1" />
							Add Expectation
						</Button>
					</div>

					{#each scenario.expectations as expectation, expIndex}
						<ExpectationRow
							{expectation}
							{elasticRules}
							onupdate={(exp) => handleExpectationUpdate(expIndex, exp)}
							onremove={() => handleExpectationRemove(expIndex)}
							canRemove={scenario.expectations.length > 1}
						/>
					{/each}
				</div>
			{/if}
			</div>
		</div>
	</div>
</div>

<style>
	.scenario-collapse-grid {
		display: grid;
		grid-template-rows: 0fr;
		transition: grid-template-rows 200ms ease-out;
	}
	.scenario-collapse-open {
		grid-template-rows: 1fr;
	}
</style>
