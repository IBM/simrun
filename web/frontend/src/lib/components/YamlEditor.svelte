<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { EditorView } from '@codemirror/view';
	import { EditorState } from '@codemirror/state';
	import { basicSetup } from 'codemirror';
	import { yaml } from '@codemirror/lang-yaml';
	import { oneDark } from '@codemirror/theme-one-dark';

	let {
		value = $bindable(''),
		readonly = false,
		onchange
	}: {
		value?: string;
		readonly?: boolean;
		onchange?: (value: string) => void;
	} = $props();

	let container: HTMLDivElement;
	let view: EditorView;

	onMount(() => {
		const extensions = [
			basicSetup,
			yaml(),
			oneDark,
			EditorView.updateListener.of((update) => {
				if (update.docChanged) {
					const newValue = update.state.doc.toString();
					value = newValue;
					onchange?.(newValue);
				}
			})
		];

		if (readonly) {
			extensions.push(EditorState.readOnly.of(true));
		}

		view = new EditorView({
			state: EditorState.create({
				doc: value,
				extensions
			}),
			parent: container
		});
	});

	onDestroy(() => {
		view?.destroy();
	});

	$effect(() => {
		if (view && value !== view.state.doc.toString()) {
			view.dispatch({
				changes: {
					from: 0,
					to: view.state.doc.length,
					insert: value
				}
			});
		}
	});
</script>

<div bind:this={container} class="overflow-auto rounded-md border border-border"></div>
