/**
 * Extract output variable names from base64-encoded Terraform HCL.
 * Looks for `output "name" {` blocks.
 */
export function extractTerraformOutputs(base64Terraform: string): string[] {
	try {
		const hcl = atob(base64Terraform);
		const outputRegex = /output\s+"([^"]+)"\s*\{/g;
		const outputs: string[] = [];
		let match;
		while ((match = outputRegex.exec(hcl)) !== null) {
			outputs.push(match[1]);
		}
		return outputs;
	} catch {
		return [];
	}
}
