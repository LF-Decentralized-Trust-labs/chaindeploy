import { Page, expect } from '@playwright/test'

export type FabricPeerParams = {
	protocol: 'Fabric'
	nodeType: 'Peer node'
	nodeName: string
	organization?: string
	mode?: string
	listenAddress: string
	operationsAddress: string
	externalEndpoint: string
}

export type FabricOrdererParams = {
	protocol: 'Fabric'
	nodeType: 'Orderer node'
	nodeName: string
	organization?: string
	mode?: string
	listenAddress: string
	operationsAddress: string
	externalEndpoint: string
	// Add more Fabric Orderer-specific fields if needed
}

export type BesuNodeParams = {
	protocol: 'Besu'
	nodeType: 'Besu node'
	nodeName: string
	organization?: string
	mode?: string
	listenAddress: string
	operationsAddress?: string
	externalEndpoint?: string
	// Add more Besu-specific fields if needed
}

export type NodeWizardParams = FabricPeerParams | FabricOrdererParams | BesuNodeParams

export async function createNodeWithWizard(page: Page, baseURL: string, params: NodeWizardParams) {
	await page.goto(baseURL + '/nodes/create')
	await expect(page.getByRole('heading', { name: /create node/i })).toBeVisible()

	// Step 1: Protocol
	await page.getByRole('button', { name: params.protocol }).click()
	await page.getByRole('button', { name: /next/i }).click()

	// Step 2: Node Type
	await page.getByRole('button', { name: params.nodeType }).click()
	await page.getByRole('button', { name: /next/i }).click()

	// Step 3: Configuration
	await page.getByPlaceholder('Enter node name').fill(params.nodeName)

	// Organization
	if (params.organization) {
		const orgSelect = page.getByRole('combobox', { name: /organization/i })
		await orgSelect.click()
		await page.getByRole('option', { name: params.organization }).click()
	} else {
		const orgSelect = page.getByRole('combobox', { name: /organization/i })
		await orgSelect.click()
		await page.getByRole('option').first().click()
	}

	// Mode
	if (params.mode) {
		const modeSelect = page.getByRole('combobox', { name: /mode/i })
		await modeSelect.click()
		await page.getByRole('option', { name: new RegExp(params.mode, 'i') }).click()
	}

	// Node-type specific fields
	if (params.protocol === 'Fabric') {
		// Both Peer and Orderer use these fields
		await page.getByPlaceholder('e.g., 0.0.0.0:7051').fill(params.listenAddress)
		await page.getByPlaceholder('e.g., 0.0.0.0:9443').fill(params.operationsAddress)
		await page.getByPlaceholder('e.g., peer0.org1.example.com:7051').fill(params.externalEndpoint)
	} else if (params.protocol === 'Besu') {
		await page.getByPlaceholder('e.g., 0.0.0.0:7051').fill(params.listenAddress)
		if (params.operationsAddress) {
			await page.getByPlaceholder('e.g., 0.0.0.0:9443').fill(params.operationsAddress)
		}
		if (params.externalEndpoint) {
			await page.getByPlaceholder('e.g., peer0.org1.example.com:7051').fill(params.externalEndpoint)
		}
	}

	// Go to Review step
	await page.getByRole('button', { name: /next/i }).click()

	// Step 4: Review and Submit
	await page.getByRole('button', { name: /create node/i }).click()

	// Wait for navigation to the node detail page or nodes list
	await expect(page.getByText(/Hyperledger Fabric Node/i)).toBeVisible({ timeout: 60000 })
}
