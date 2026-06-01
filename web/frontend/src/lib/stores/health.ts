import { writable } from 'svelte/store';

const POLL_INTERVAL = 10_000;

function createHealthStore() {
	const { subscribe, set } = writable(true);
	let timer: ReturnType<typeof setInterval>;

	async function check() {
		try {
			const res = await fetch('/health', { signal: AbortSignal.timeout(5000) });
			set(res.ok);
		} catch {
			set(false);
		}
	}

	function start() {
		check();
		timer = setInterval(check, POLL_INTERVAL);
	}

	function stop() {
		clearInterval(timer);
	}

	return { subscribe, start, stop };
}

export const health = createHealthStore();
