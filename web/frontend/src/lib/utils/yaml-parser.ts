import yaml from 'js-yaml';
import type {
	FormScenario,
	FormTarget,
	FormExpectation,
	FormParam,
	FormCollectionField
} from './yaml-generator';
import { createEmptyExpectation } from './yaml-generator';

export interface ParseResult {
	success: boolean;
	scenarios?: FormScenario[];
	target?: FormTarget;
	error?: string;
	builderSupported?: boolean;
}

export function parseScenarioYAML(yamlString: string): ParseResult {
	let doc: Record<string, unknown>;
	try {
		doc = yaml.load(yamlString) as Record<string, unknown>;
	} catch (e) {
		return {
			success: false,
			error: e instanceof Error ? e.message : 'Failed to parse YAML'
		};
	}

	if (!doc || typeof doc !== 'object') {
		return { success: false, error: 'Invalid YAML: expected an object' };
	}

	const scenarios = doc.scenarios;
	if (!Array.isArray(scenarios) || scenarios.length === 0) {
		return { success: false, error: 'Invalid YAML: missing or empty scenarios array' };
	}

	// Check for unsupported detonator types (awsCli — only simrunDetonator is builder-supported)
	const hasUnsupported = scenarios.some((s: Record<string, unknown>) => {
		const det = s.detonate as Record<string, unknown> | undefined;
		if (det) {
			return det.awsCliDetonator;
		}
		// If neither detonate nor inject, unsupported
		if (!s.inject) return true;
		return false;
	});

	if (hasUnsupported) {
		return {
			success: true,
			builderSupported: false
		};
	}

	// Parse targets
	const targets = doc.targets as Record<string, string> | undefined;
	const target: FormTarget = {
		aws: targets?.aws || '',
		gcp: targets?.gcp || '',
		azure: targets?.azure || '',
		kubernetes: targets?.kubernetes || ''
	};

	// Parse each scenario
	const parsed: FormScenario[] = scenarios.map((s: Record<string, unknown>) => {
		const inject = s.inject as Record<string, unknown> | undefined;
		const det = (s.detonate as Record<string, unknown>)?.simrunDetonator as
			| Record<string, unknown>
			| undefined;

		const isInject = !!inject;

		// Parse inject-specific fields
		let template = '';
		let injectIndex = '';
		let injectPack = '';
		let templateVars: FormParam[] = [];

		if (isInject) {
			const elasticInjector = inject.elasticInjector as Record<string, unknown> | undefined;
			const documents = elasticInjector?.documents as Record<string, unknown>[] | undefined;
			if (documents && documents.length > 0) {
				const doc = documents[0];
				const rawIndex = (doc.index as string) || '';
				injectIndex = rawIndex.replace(/^logs-/, '').replace(/-default$/, '');
				template = (doc.template as string) || '';
				injectPack = (doc.pack as string) || '';
				const rawVars = (doc.vars || {}) as Record<string, unknown>;
				templateVars = Object.entries(rawVars).map(([key, value]) => ({
					key,
					value: String(value)
				}));
			}
		}

		// Parse params (detonate only)
		const rawParams = (det?.params || {}) as Record<string, unknown>;
		const params: FormParam[] = Object.entries(rawParams).map(([key, value]) => ({
			key,
			value: String(value)
		}));

		// Parse indicators
		const ind = s.indicators as Record<string, unknown> | undefined;
		const indicators = {
			terraformOutput: Array.isArray(ind?.terraformOutput) ? (ind.terraformOutput as string[]) : [],
			static: Array.isArray(ind?.static) ? (ind.static as string[]) : []
		};

		// Parse expectations
		const rawExps = s.expectations as Record<string, unknown>[] | undefined;
		const expectations: FormExpectation[] = (rawExps || []).map((exp) => {
			const elastic = exp.elasticSecurityAlert as Record<string, string> | undefined;
			const datadog = exp.datadogSecuritySignal as Record<string, string> | undefined;

			if (elastic) {
				return {
					type: 'elastic' as const,
					alertName: elastic.name || '',
					severity: elastic.severity || '',
					timeout: (exp.timeout as string) || '5m'
				};
			}
			if (datadog) {
				return {
					type: 'datadog' as const,
					alertName: datadog.name || '',
					severity: datadog.severity || '',
					timeout: (exp.timeout as string) || '5m'
				};
			}
			return createEmptyExpectation();
		});

		// Parse collection
		const collect = s.collect as Record<string, unknown> | undefined;
		const elasticCollector = collect?.elasticCollector as Record<string, unknown> | undefined;
		const additionalFields = (elasticCollector?.additionalFields || {}) as Record<string, unknown>;
		const collectionFields: FormCollectionField[] = Object.entries(additionalFields).map(
			([key, value]) => ({
				key,
				value: String(value)
			})
		);

		return {
			name: (s.name as string) || '',
			enabled: s.enabled !== false,
			pack: isInject ? injectPack : (det?.pack as string) || '',
			scenarioType: isInject ? ('inject' as const) : ('detonate' as const),
			simulation: (det?.simulation as string) || '',
			template,
			injectIndex,
			templateVars,
			params,
			indicators,
			expectations: expectations.length > 0 ? expectations : [createEmptyExpectation()],
			collection: {
				enabled: !!elasticCollector,
				type: (elasticCollector ? 'elastic' : '') as '' | 'elastic',
				index: (elasticCollector?.index as string) || '',
				additionalFields: collectionFields
			}
		};
	});

	return {
		success: true,
		builderSupported: true,
		scenarios: parsed,
		target
	};
}
