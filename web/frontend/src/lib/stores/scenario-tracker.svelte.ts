import type { ScenarioResult, RunLogEntry, ScenarioPhase } from '$lib/types';

/** Unified entry that can represent both in-progress and completed scenarios. */
export interface ScenarioEntry {
	name: string;
	status: 'pending' | 'running' | 'completed';
	phase?: ScenarioPhase;
	result?: ScenarioResult;
}

export class ScenarioTracker {
	entries = $state<Record<string, ScenarioEntry>>({});
	logs = $state<RunLogEntry[]>([]);

	/** Entries sorted with running/pending scenarios first, then by name. */
	sortedEntries = $derived(
		Object.values(this.entries).sort((a, b) => {
			const order = { running: 0, pending: 1, completed: 2 };
			const aOrder = order[a.status] ?? 2;
			const bOrder = order[b.status] ?? 2;
			if (aOrder !== bOrder) return aOrder - bOrder;
			return a.name.localeCompare(b.name);
		})
	);

	/** Pre-computed map of scenario name to filtered log entries. */
	logsByScenario = $derived.by(() => {
		// Build execution_id → scenario name mapping from completed results
		const execIdToScenario = new Map<string, string>();
		for (const entry of Object.values(this.entries)) {
			if (entry.result?.executionId) {
				execIdToScenario.set(entry.result.executionId, entry.name);
			}
		}

		const map = new Map<string, RunLogEntry[]>();
		for (const log of this.logs) {
			let scenario = log.fields?.scenario as string | undefined;

			// Fall back to execution_id mapping for logs without scenario field
			if (!scenario) {
				const execId = log.fields?.execution_id as string | undefined;
				if (execId) {
					scenario = execIdToScenario.get(execId);
				}
			}

			if (scenario) {
				let arr = map.get(scenario);
				if (!arr) {
					arr = [];
					map.set(scenario, arr);
				}
				arr.push(log);
			}
		}
		return map;
	});

	reset() {
		this.entries = {};
		this.logs = [];
	}

	/** Update entries from API scenario results (replaces WS-driven status tracking). */
	setScenarios(scenarios: ScenarioResult[]) {
		const updated: Record<string, ScenarioEntry> = {};
		for (const s of scenarios) {
			if (s.status === 'completed' && s.isSuccess !== null) {
				updated[s.name] = { name: s.name, status: 'completed', result: s };
			} else {
				// Carry the partial result for running/pending rows too, so the UI
				// can surface mid-run executor identity and incremental assertions.
				updated[s.name] = { name: s.name, status: s.status, phase: s.phase, result: s };
			}
		}
		this.entries = updated;
	}

	addLog(log: RunLogEntry) {
		this.logs = [...this.logs, log];
	}

	setLogs(logs: RunLogEntry[]) {
		this.logs = logs;
	}

	getLogsForScenario(scenarioName: string): RunLogEntry[] {
		return this.logsByScenario.get(scenarioName) ?? [];
	}
}
