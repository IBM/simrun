import { writable } from 'svelte/store';
import type { WSMessage } from '$lib/types';

function createWebSocketStore() {
	const { subscribe, set } = writable<WSMessage | null>(null);
	let ws: WebSocket | null = null;
	let reconnectTimer: ReturnType<typeof setTimeout>;
	let pendingRunId: string | null = null;

	function connect() {
		const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
		ws = new WebSocket(`${protocol}//${window.location.host}/api/ws`);

		ws.onopen = () => {
			if (pendingRunId) {
				ws!.send(JSON.stringify({ type: 'subscribe', data: { runId: pendingRunId } }));
				pendingRunId = null;
			}
		};

		ws.onmessage = (event) => {
			const msg: WSMessage = JSON.parse(event.data);
			set(msg);
		};

		ws.onclose = () => {
			reconnectTimer = setTimeout(connect, 3000);
		};

		ws.onerror = () => {
			ws?.close();
		};
	}

	function subscribeToRun(runId: string) {
		if (ws?.readyState === WebSocket.OPEN) {
			ws.send(JSON.stringify({ type: 'subscribe', data: { runId } }));
		} else {
			pendingRunId = runId;
		}
	}

	function disconnect() {
		clearTimeout(reconnectTimer);
		ws?.close();
	}

	return { subscribe, connect, disconnect, subscribeToRun };
}

export const websocket = createWebSocketStore();
