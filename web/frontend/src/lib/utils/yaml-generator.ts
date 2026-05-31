export interface FormExpectation {
	type: 'elastic' | 'datadog';
	alertName: string;
	severity: string;
	timeout: string;
}

export interface FormParam {
	key: string;
	value: string;
}

export interface FormIndicators {
	terraformOutput: string[];
	static: string[];
}

export interface FormCollectionField {
	key: string;
	value: string;
}

export interface FormCollection {
	enabled: boolean;
	type: '' | 'elastic';
	index: string;
	additionalFields: FormCollectionField[];
}

export interface FormTarget {
	aws: string;
	gcp: string;
	azure: string;
	kubernetes: string;
}

export interface FormScenario {
	name: string;
	enabled: boolean;
	pack: string;
	scenarioType: 'detonate' | 'inject';
	simulation: string;
	template: string;
	injectIndex: string;
	templateVars: FormParam[];
	params: FormParam[];
	indicators: FormIndicators;
	expectations: FormExpectation[];
	collection: FormCollection;
}

function needsQuoting(str: string): boolean {
	if (/^(true|false|yes|no|on|off|null|~)$/i.test(str)) return true;
	if (/^-?\d+(\.\d+)?([eE][+-]?\d+)?$/.test(str)) return true;
	if (/^0[xXoO]/.test(str)) return true;
	if (str === '' || str.startsWith(' ') || str.endsWith(' ')) return true;
	return /[:#{}"'\n\r{}\[\],&*!|>%@`?-]/.test(str);
}

function escapeYAMLString(str: string): string {
	if (needsQuoting(str)) {
		return `"${str.replace(/\\/g, '\\\\').replace(/"/g, '\\"')}"`;
	}
	return str;
}

export function generateScenarioYAML(
	scenarios: FormScenario[],
	target?: FormTarget,
	scenarioFileType: string = 'standard'
): string {
	const lines: string[] = [];

	// Emit top-level targets if any cloud types are selected
	if (target) {
		const entries = Object.entries(target).filter(([, v]) => v.trim());
		if (entries.length > 0) {
			lines.push('targets:');
			for (const [cloudType, connectorName] of entries) {
				lines.push(`  ${cloudType}: ${escapeYAMLString(connectorName)}`);
			}
			lines.push('');
		}
	}

	lines.push('scenarios:');

	for (const scenario of scenarios) {
		lines.push(`  - name: ${escapeYAMLString(scenario.name)}`);
		if (!scenario.enabled) {
			lines.push('    enabled: false');
		}

		if (scenario.scenarioType === 'inject') {
			generateInjectBlock(lines, scenario);
		} else {
			generateDetonateBlock(lines, scenario);
		}

		// Indicators
		const hasTfOutputs =
			scenario.scenarioType === 'detonate' && scenario.indicators.terraformOutput.length > 0;
		const hasStatic = scenario.indicators.static.filter((s) => s.trim()).length > 0;
		if (hasTfOutputs || hasStatic) {
			lines.push('    indicators:');
			if (hasTfOutputs) {
				lines.push('      terraformOutput:');
				for (const output of scenario.indicators.terraformOutput) {
					lines.push(`        - ${escapeYAMLString(output)}`);
				}
			}
			if (hasStatic) {
				lines.push('      static:');
				for (const s of scenario.indicators.static) {
					if (s.trim()) {
						lines.push(`        - ${escapeYAMLString(s.trim())}`);
					}
				}
			}
		}

		// Collection (only for collect type or detonate scenarios with collection enabled)
		if (
			scenario.scenarioType === 'detonate' &&
			((scenarioFileType === 'collect' && scenario.collection.type && scenario.collection.index.trim()) ||
				(scenarioFileType !== 'collect' &&
					scenario.collection.enabled &&
					scenario.collection.type &&
					scenario.collection.index.trim()))
		) {
			const collectorKeyMap: Record<string, string> = { elastic: 'elasticCollector' };
			const collectorKey = collectorKeyMap[scenario.collection.type] ?? 'elasticCollector';
			lines.push('    collect:');
			lines.push(`      ${collectorKey}:`);
			lines.push(`        index: ${escapeYAMLString(scenario.collection.index.trim())}`);
			const validFields = scenario.collection.additionalFields.filter(
				(f) => f.key.trim() && f.value.trim()
			);
			if (validFields.length > 0) {
				lines.push('        additionalFields:');
				for (const field of validFields) {
					lines.push(`          ${field.key.trim()}: ${escapeYAMLString(field.value.trim())}`);
				}
			}
		}

		lines.push('    expectations:');

		if (scenarioFileType === 'explore' || scenarioFileType === 'collect') {
			// Auto-generate a placeholder expectation for explore/collect types
			const dummyName = `${scenario.name || 'scenario'} - ${scenarioFileType} mode`;
			lines.push('      - elasticSecurityAlert:');
			lines.push(`          name: ${escapeYAMLString(dummyName)}`);
			lines.push('        timeout: 5m');
		} else {
			for (const exp of scenario.expectations) {
				const matcherKey =
					exp.type === 'elastic' ? 'elasticSecurityAlert' : 'datadogSecuritySignal';
				lines.push(`      - ${matcherKey}:`);
				lines.push(`          name: ${escapeYAMLString(exp.alertName)}`);
				if (exp.severity) {
					lines.push(`          severity: ${escapeYAMLString(exp.severity)}`);
				}
				lines.push(`        timeout: ${exp.timeout || '5m'}`);
			}
		}
	}

	return lines.join('\n') + '\n';
}

function generateDetonateBlock(lines: string[], scenario: FormScenario) {
	lines.push('    detonate:');
	lines.push('      simrunDetonator:');
	lines.push(`        pack: ${scenario.pack}`);
	lines.push(`        simulation: ${scenario.simulation}`);

	const validParams = scenario.params.filter((p) => p.key.trim() && p.value.trim());
	if (validParams.length > 0) {
		lines.push('        params:');
		for (const param of validParams) {
			lines.push(`          ${param.key.trim()}: ${escapeYAMLString(param.value.trim())}`);
		}
	}
}

function generateInjectBlock(lines: string[], scenario: FormScenario) {
	lines.push('    inject:');
	lines.push('      elasticInjector:');
	lines.push('        documents:');
	lines.push(`          - index: ${escapeYAMLString(`logs-${scenario.injectIndex}-default`)}`);
	lines.push(`            template: ${scenario.template}`);
	lines.push(`            pack: ${scenario.pack}`);

	const validVars = scenario.templateVars.filter((v) => v.key.trim() && v.value.trim());
	if (validVars.length > 0) {
		lines.push('            vars:');
		for (const v of validVars) {
			lines.push(`              ${v.key.trim()}: ${escapeYAMLString(v.value.trim())}`);
		}
	}
}

export function createEmptyExpectation(): FormExpectation {
	return {
		type: 'elastic',
		alertName: '',
		severity: '',
		timeout: '5m'
	};
}

export function createEmptyTarget(): FormTarget {
	return { aws: '', gcp: '', azure: '', kubernetes: '' };
}

export function createEmptyScenario(): FormScenario {
	return {
		name: '',
		enabled: true,
		pack: '',
		scenarioType: 'detonate',
		simulation: '',
		template: '',
		injectIndex: '',
		templateVars: [],
		params: [],
		indicators: { terraformOutput: [], static: [] },
		expectations: [createEmptyExpectation()],
		collection: { enabled: false, type: '', index: '', additionalFields: [] }
	};
}
