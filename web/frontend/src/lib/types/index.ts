// Run types
export interface Run {
	id: string;
	status: 'running' | 'completed' | 'failed';
	startTime: string;
	endTime: string | null;
	total: number;
	succeeded: number;
	failed: number;
	scenarioId?: string;
	scenarioName?: string;
	scenarioType?: 'standard' | 'explore' | 'collect';
	scheduleId?: string;
	scheduleName?: string;
	createdBy: string;
	createdAt: string;
	scenarioResults?: ScenarioResult[];
}

export interface ScenarioResult {
	id: string;
	runId: string;
	name: string;
	status: 'pending' | 'running' | 'completed';
	phase?: ScenarioPhase;
	isSuccess: boolean | null;
	errorMessage: string;
	durationSecs: number;
	matchingDurSecs: number;
	timeExecuted: string;
	executorName: string;
	executorType: string;
	executionId: string;
	simulationId: string;
	assertions: AssertionInfo[] | null;
	indicators: Indicators | null;
	metadata: ScenarioMetadata | null;
	collectedLogPath?: string;
	collectedDocCount?: number;
	discoveredAlerts?: DiscoveredAlert[] | null;
	createdAt: string;
}

export interface DiscoveredAlert {
	ruleName: string;
	alertId: string;
	severity?: string;
}

export interface AssertionInfo {
	matcherType: string;
	alertName: string;
	passed?: boolean;
}

export interface Indicators {
	terraformOutput?: string[];
	static?: string[];
}

export interface ScenarioMetadata {
	name: string;
	description: string;
}

// Saved scenario types
export type ScenarioType = 'standard' | 'explore' | 'collect';

export interface SavedScenario {
	id: string;
	name: string;
	type: ScenarioType;
	yaml: string;
	createdBy: string;
	updatedBy: string;
	createdAt: string;
	updatedAt: string;
}

// Pack types
export interface Pack {
	id: string;
	name: string;
	type: 'local' | 'remote' | 'upload';
	source: string;
	version: string;
	status: string;
	parameters?: Record<string, unknown>;
	installedBy: string;
	createdAt: string;
	updatedAt: string;
}

export interface PackManifest {
	pack: {
		name: string;
		version: string;
		description: string;
	};
	simulations: SimulationManifest[];
	templates?: TemplateManifest[];
	params_schema?: Record<string, unknown>;
}

export interface SimulationManifest {
	id: string;
	name: string;
	description: string;
	scope: string;
	isSlow: boolean;
	mitre: {
		tactics: string[];
		techniques: string[];
	};
	params_schema?: Record<string, unknown>;
	terraform?: string; // base64-encoded HCL
}

export interface TemplateManifest {
	id: string;
	name: string;
	description: string;
	scope: string;
	content: string; // base64-encoded template content
	vars?: Record<string, string>; // variable names to default values
}

// Secret types
export interface SecretGroup {
	id: string;
	name: string;
	description: string;
	keys: string[];
	createdBy: string;
	updatedBy: string;
	createdAt: string;
	updatedAt: string;
}

export interface SecretEntryInput {
	key: string;
	value: string | null; // null = keep existing encrypted value
}

// Schedule types
export interface Schedule {
	id: string;
	scenarioId: string;
	cronExpression: string;
	enabled: boolean;
	parallelism: number;
	lastRunAt: string | null;
	createdBy: string;
	updatedBy: string;
	createdAt: string;
	updatedAt: string;
}

// Lint types
export interface LintResponse {
	valid: boolean;
	scenarios?: LintedScenario[];
	error?: string;
}

export interface LintedScenario {
	name: string;
	executorType: string;
	executorName: string;
	assertions: number;
}

// Run response
export interface RunResponse {
	runId: string;
}

// Paginated runs response
export interface RunListResponse {
	runs: Run[];
	total: number;
	page: number;
	perPage: number;
}

// Paginated saved-scenarios response
export interface ScenarioListResponse {
	scenarios: SavedScenario[];
	total: number;
	page: number;
	perPage: number;
}

// Config types
export interface AppConfig {
	[key: string]: unknown;
}

// Version
export interface VersionInfo {
	version: string;
	commit: string;
	buildDate: string;
	goVersion: string;
}

// WebSocket message types
export interface WSMessage {
	type: WSMessageType;
	data: unknown;
}

export type WSMessageType =
	| 'run_started'
	| 'scenario_started'
	| 'scenario_status'
	| 'scenario_log'
	| 'assertion_update'
	| 'scenario_completed'
	| 'run_completed';

export type ScenarioPhase =
	| 'queued'
	| 'warmup'
	| 'detonating'
	| 'matching'
	| 'exploring'
	| 'collecting'
	| 'cleanup';

export interface ScenarioStatusData {
	name: string;
	phase: ScenarioPhase;
}

// ScenarioEntry is now defined in scenario-tracker.svelte.ts

export interface RunStartedData {
	totalScenarios: number;
	parallelism: number;
}

export interface ScenarioStartedData {
	name: string;
	executorType: string;
	executorName: string;
}

export interface ScenarioLogData {
	scenarioName: string;
	level: string;
	message: string;
}

export interface RunLogEntry {
	ts: string;
	level: string;
	msg: string;
	fields?: Record<string, unknown>;
}

export interface AssertionUpdateData {
	scenarioName: string;
	matcher: string;
	alertName: string;
	passed: boolean;
}

export interface ScenarioCompletedData {
	name: string;
	isSuccess: boolean;
	durationSeconds: number;
	executionId: string;
}

export interface RunCompletedData {
	totalScenarios: number;
	successScenarios: number;
	failedScenarios: number;
}

// Connector types
export interface Connector {
	id: string;
	name: string;
	type: string;
	description: string;
	secretGroupId?: string;
	config: Record<string, unknown>;
	enabled: boolean;
	isDefault: boolean;
	createdBy: string;
	updatedBy: string;
	createdAt: string;
	updatedAt: string;
}

export interface ElasticConnectorConfig {
	kibana_url: string;
	cloud_id?: string;
	elasticsearch_url?: string;
	export_enabled?: boolean;
	export_datastream?: string;
}

export interface AWSConnectorConfig {
	role_arn: string;
}

export interface GCPConnectorConfig {
	auth_type?: 'workload_identity_federation' | '';
	project_id?: string;
	project_number?: string;
	pool_id?: string;
	provider_id?: string;
	service_account_email?: string;
	credentials_file?: string;
}

export interface AzureConnectorConfig {
	auth_type?: 'workload_identity_federation' | '';
	tenant_id: string;
	subscription_id: string;
	client_id: string;
	token_file?: string;
}

export interface KubernetesConnectorConfig {
	cluster_name: string;
	region: string;
	cloud_connector: string;
	resource_group?: string; // AKS only
	project?: string; // GKE only (falls back to GCP connector's project_id)
}

export interface TestConnectorResponse {
	success: boolean;
	error?: string;
}

export interface ElasticRule {
	id: string;
	rule_id: string;
	name: string;
	description: string;
	enabled: boolean;
	tags: string[];
	severity: string;
	risk_score: number;
	type: string;
	created_at: string;
	updated_at: string;
}

export interface ElasticRulesResponse {
	page: number;
	perPage: number;
	total: number;
	data: ElasticRule[];
}

// Auth types
export interface User {
	email: string;
	name: string;
	picture: string;
}

// Rule coverage types
export interface CoverageResponse {
	summary: CoverageSummary;
	rules: RuleCoverageEntry[];
}

export interface CoverageSummary {
	totalRules: number;
	coveredRules: number;
	coveragePercent: number;
}

export interface RuleCoverageEntry {
	ruleId: string;
	name: string;
	severity: string;
	riskScore: number;
	tags: string[];
	covered: boolean;
	scenarios: CoverageScenario[];
	lastResult?: CoverageLastResult;
}

export interface CoverageScenario {
	scenarioId: string;
	scenarioName: string;
	simulationId?: string;
	packName?: string;
}

export interface CoverageLastResult {
	passed: boolean;
	runId: string;
	timestamp: string;
}
