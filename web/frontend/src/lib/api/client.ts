import type {
	Run,
	Assessment,
	Pack,
	PackManifest,
	LintResponse,
	RunResponse,
	RunListResponse,
	AssessmentListResponse,
	AppConfig,
	VersionInfo,
	SecretGroup,
	SecretEntryInput,
	Schedule,
	Connector,
	TestConnectorResponse,
	ElasticRulesResponse,
	ElasticRule,
	RunLogEntry,
	User,
	CoverageResponse
} from '$lib/types';

const BASE = '/api';

let authRedirecting = false;

export class ValidationError extends Error {
	errors: { key: string; rule: string; message: string }[];
	constructor(message: string, errors: { key: string; rule: string; message: string }[]) {
		super(message);
		this.errors = errors;
	}
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
	const res = await fetch(`${BASE}${path}`, {
		headers: { 'Content-Type': 'application/json' },
		...options
	});
	if (!res.ok) {
		if (res.status === 401 && !authRedirecting) {
			authRedirecting = true;
			window.location.href = '/login';
			throw new Error('Authentication required');
		}
		const body = await res.json().catch(() => ({ error: res.statusText }));
		if (res.status === 400 && Array.isArray(body.errors)) {
			throw new ValidationError(body.error || 'Validation failed', body.errors);
		}
		throw new Error(body.error || res.statusText);
	}
	if (res.status === 204) {
		return undefined as T;
	}
	return res.json();
}

// Assessments - lint & run
export async function lintAssessment(yaml: string): Promise<LintResponse> {
	return request('/assessments/lint', { method: 'POST', body: JSON.stringify({ yaml }) });
}

export async function runAssessment(
	assessmentId: string,
	parallelism?: number,
	exploreMode?: boolean,
	cleanupAlerts?: boolean,
	timeout?: string
): Promise<RunResponse> {
	return request('/runs', {
		method: 'POST',
		body: JSON.stringify({ assessmentId, parallelism, exploreMode, cleanupAlerts, timeout })
	});
}

// Assessments CRUD
export interface ScenarioFilters {
	name?: string;
	types?: string[];
	// Go-style duration: "24h", "168h", etc. Empty/undefined = no constraint.
	since?: string;
}

export async function listAssessments(
	page = 1,
	perPage = 50,
	filters: ScenarioFilters = {}
): Promise<AssessmentListResponse> {
	const qs = new URLSearchParams();
	qs.set('page', String(page));
	qs.set('per_page', String(perPage));
	if (filters.name) qs.set('name', filters.name);
	if (filters.since) qs.set('since', filters.since);
	for (const t of filters.types ?? []) qs.append('type', t);
	return request(`/assessments?${qs.toString()}`);
}

export async function getAssessment(id: string): Promise<Assessment> {
	return request(`/assessments/${id}`);
}

export async function getAssessmentByName(name: string): Promise<Assessment> {
	return request(`/assessments/by-name/${encodeURIComponent(name)}`);
}

export async function saveAssessment(
	name: string,
	yaml: string,
	type?: string
): Promise<Assessment> {
	return request('/assessments', { method: 'POST', body: JSON.stringify({ name, type, yaml }) });
}

export async function updateAssessment(
	id: string,
	name: string,
	yaml: string,
	type?: string
): Promise<void> {
	return request(`/assessments/${id}`, {
		method: 'PUT',
		body: JSON.stringify({ name, type, yaml })
	});
}

export async function deleteAssessment(id: string): Promise<void> {
	return request(`/assessments/${id}`, { method: 'DELETE' });
}

// Runs
export interface RunFilters {
	name?: string;
	types?: string[];
	// Go-style duration: "24h", "168h", etc. Empty/undefined = no constraint.
	since?: string;
	assessmentId?: string;
}

export async function listRuns(
	page = 1,
	perPage = 50,
	filters: RunFilters = {}
): Promise<RunListResponse> {
	const qs = new URLSearchParams();
	qs.set('page', String(page));
	qs.set('per_page', String(perPage));
	if (filters.name) qs.set('name', filters.name);
	if (filters.since) qs.set('since', filters.since);
	if (filters.assessmentId) qs.set('assessment_id', filters.assessmentId);
	for (const t of filters.types ?? []) qs.append('type', t);
	return request(`/runs?${qs.toString()}`);
}

export async function listAssessmentRuns(
	assessmentId: string,
	page = 1,
	perPage = 50
): Promise<RunListResponse> {
	const qs = new URLSearchParams();
	qs.set('page', String(page));
	qs.set('per_page', String(perPage));
	return request(`/assessments/${assessmentId}/runs?${qs.toString()}`);
}

export async function getRun(runId: string): Promise<Run> {
	const data = await request<{ run: Run; scenarios: Run['scenarioResults'] }>(`/runs/${runId}`);
	return { ...data.run, scenarioResults: data.scenarios ?? [] };
}

export async function deleteRun(runId: string): Promise<void> {
	return request(`/runs/${runId}`, { method: 'DELETE' });
}

export async function getRunLogs(runId: string): Promise<RunLogEntry[]> {
	return request(`/runs/${runId}/logs`);
}

// Collected logs
export function getCollectedLogsUrl(scenarioResultId: string): string {
	return `${BASE}/scenario-results/${scenarioResultId}/collected-logs`;
}

// Packs
export async function listPacks(): Promise<Pack[]> {
	return request('/packs');
}

export async function installPack(pack: {
	name: string;
	type: string;
	source: string;
	version?: string;
}): Promise<void> {
	return request('/packs/install', { method: 'POST', body: JSON.stringify(pack) });
}

export async function uploadPack(name: string, file: File): Promise<void> {
	const formData = new FormData();
	formData.append('name', name);
	formData.append('file', file);

	const res = await fetch(`${BASE}/packs/upload`, {
		method: 'POST',
		body: formData
	});

	if (!res.ok) {
		if (res.status === 401 && !authRedirecting) {
			authRedirecting = true;
			window.location.href = '/login';
			throw new Error('Authentication required');
		}
		const body = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(body.error || res.statusText);
	}
}

export async function deletePack(name: string): Promise<void> {
	return request(`/packs/${name}`, { method: 'DELETE' });
}

export async function getPackManifest(name: string): Promise<PackManifest> {
	return request(`/packs/${name}/manifest`);
}

export async function getPackParameters(name: string): Promise<Record<string, unknown>> {
	const resp = await request<{ parameters: Record<string, unknown> }>(`/packs/${name}/parameters`);
	return resp.parameters ?? {};
}

export interface UpdatePackParametersResponse {
	parameters: Record<string, unknown>;
	unknown_keys: string[];
}

export interface PackParameterValidationError {
	key: string;
	rule: string;
	message: string;
}

export interface PackParameterValidationErrorBody {
	error: string;
	errors: PackParameterValidationError[];
}

export async function updatePackParameters(
	name: string,
	parameters: Record<string, unknown>
): Promise<UpdatePackParametersResponse> {
	return request<UpdatePackParametersResponse>(`/packs/${name}/parameters`, {
		method: 'PUT',
		body: JSON.stringify({ parameters })
	});
}

// Secrets
export async function listSecrets(): Promise<SecretGroup[]> {
	return request('/secrets');
}

export async function getSecret(id: string): Promise<SecretGroup> {
	return request(`/secrets/${id}`);
}

export async function createSecret(
	name: string,
	description: string,
	entries: SecretEntryInput[]
): Promise<SecretGroup> {
	return request('/secrets', {
		method: 'POST',
		body: JSON.stringify({ name, description, entries })
	});
}

export async function updateSecret(
	id: string,
	name: string,
	description: string,
	entries: SecretEntryInput[]
): Promise<void> {
	return request(`/secrets/${id}`, {
		method: 'PUT',
		body: JSON.stringify({ name, description, entries })
	});
}

export async function deleteSecret(id: string): Promise<void> {
	return request(`/secrets/${id}`, { method: 'DELETE' });
}

// Schedules
export async function listSchedules(): Promise<Schedule[]> {
	return request('/schedules');
}

export async function getScheduleByAssessment(assessmentId: string): Promise<Schedule | null> {
	try {
		return await request(`/assessments/${assessmentId}/schedule`);
	} catch {
		return null;
	}
}

export async function createSchedule(
	assessmentId: string,
	cronExpression: string,
	enabled: boolean,
	parallelism: number
): Promise<Schedule> {
	return request('/schedules', {
		method: 'POST',
		body: JSON.stringify({ assessmentId, cronExpression, enabled, parallelism })
	});
}

export async function updateSchedule(
	id: string,
	cronExpression: string,
	enabled: boolean,
	parallelism: number
): Promise<void> {
	return request(`/schedules/${id}`, {
		method: 'PUT',
		body: JSON.stringify({ cronExpression, enabled, parallelism })
	});
}

export async function deleteSchedule(id: string): Promise<void> {
	return request(`/schedules/${id}`, { method: 'DELETE' });
}

// Connectors
export async function listConnectors(): Promise<Connector[]> {
	return request('/connectors');
}

export async function getConnector(id: string): Promise<Connector> {
	return request(`/connectors/${id}`);
}

export async function createConnector(
	name: string,
	type: string,
	description: string,
	secretGroupId: string | undefined,
	config: Record<string, unknown>,
	isDefault: boolean = false
): Promise<Connector> {
	return request('/connectors', {
		method: 'POST',
		body: JSON.stringify({ name, type, description, secretGroupId, config, isDefault })
	});
}

export async function updateConnector(
	id: string,
	name: string,
	description: string,
	secretGroupId: string | undefined,
	config: Record<string, unknown>,
	enabled: boolean,
	isDefault: boolean = false
): Promise<void> {
	return request(`/connectors/${id}`, {
		method: 'PUT',
		body: JSON.stringify({ name, description, secretGroupId, config, enabled, isDefault })
	});
}

export async function deleteConnector(id: string): Promise<void> {
	return request(`/connectors/${id}`, { method: 'DELETE' });
}

export async function testConnector(
	type: string,
	secretGroupId: string,
	config: Record<string, unknown>
): Promise<TestConnectorResponse> {
	return request('/connectors/test', {
		method: 'POST',
		body: JSON.stringify({ type, secretGroupId, config })
	});
}

export async function listElasticRules(
	connectorId: string,
	page = 1,
	perPage = 100
): Promise<ElasticRulesResponse> {
	return request(`/connectors/${connectorId}/elastic/rules?page=${page}&per_page=${perPage}`);
}

export async function getElasticRule(connectorId: string, ruleId: string): Promise<ElasticRule> {
	return request(`/connectors/${connectorId}/elastic/rules/${ruleId}`);
}

export async function listElasticRulesAuto(): Promise<ElasticRulesResponse> {
	return request('/elastic/rules');
}

// Rule Coverage
export async function getRuleCoverage(): Promise<CoverageResponse> {
	return request('/rules/coverage');
}

// Config
export async function getConfig(): Promise<AppConfig> {
	return request('/config');
}

export async function updateConfig(key: string, value: unknown): Promise<void> {
	return request('/config', { method: 'PUT', body: JSON.stringify({ key, value }) });
}

// Version
export async function getVersion(): Promise<VersionInfo> {
	return request('/version');
}

// Auth
export async function getCurrentUser(): Promise<User> {
	return request('/auth/me');
}
