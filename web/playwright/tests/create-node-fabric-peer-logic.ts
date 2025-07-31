import { Page } from '@playwright/test'

const FABRIC_NODE_CREATE_PATH = '/nodes/fabric/create'

// Helper to generate unique values with cryptographically secure random numbers
function uniqueSuffix() {
	const bytes = new Uint8Array(4)
	crypto.getRandomValues(bytes)
	const randomNum = new DataView(bytes.buffer).getUint32(0) % 10000
	return `${Date.now()}-${randomNum}`
}

/**
 * Creates a Fabric node via the UI.
 *
 * @param page Playwright page instance
 * @param baseURL Base URL of the app
 * @param mspId The MSP ID of the organization to select
 * @returns The node name used for creation
 */
export async function createFabricNode(page: Page, baseURL: string, mspId: string): Promise<string> {
	await page.goto(baseURL + FABRIC_NODE_CREATE_PATH)
	await page.getByRole('heading', { name: /create fabric node/i }).waitFor({ state: 'visible', timeout: 10000 })

	// Fill out the form with unique values
	const nodeName = `test-node-${uniqueSuffix()}`

	// Name
	await page.getByPlaceholder('Enter node name').fill(nodeName)

	// Select the organization just created (by MSP ID)
	const orgSelect = page.getByRole('combobox', { name: /organization/i })
	await orgSelect.click()
	await page.getByRole('option', { name: mspId }).click()

	// Select deployment mode "Docker"
	const modeSelect = page.getByRole('combobox', { name: /mode/i })
	await modeSelect.click()
	await page.getByRole('option', { name: /docker/i }).click()

	// Listen Address - use crypto.getRandomValues for secure random port
	const listenPort = 7000 + (new DataView(crypto.getRandomValues(new Uint8Array(4)).buffer).getUint32(0) % 1000)
	await page.getByPlaceholder('e.g., 0.0.0.0:7051').fill(`0.0.0.0:${listenPort}`)

	// Operations Address - use crypto.getRandomValues for secure random port
	const opsPort = 9000 + (new DataView(crypto.getRandomValues(new Uint8Array(4)).buffer).getUint32(0) % 1000)
	await page.getByPlaceholder('e.g., 0.0.0.0:9443').fill(`0.0.0.0:${opsPort}`)

	// External Endpoint - use crypto.getRandomValues for secure random port
	const extPort = 7000 + (new DataView(crypto.getRandomValues(new Uint8Array(4)).buffer).getUint32(0) % 1000)
	await page.getByPlaceholder('e.g., peer0.org1.example.com:7051').fill(`127.0.0.1:${extPort}`)

	// Submit
	await page.getByRole('button', { name: /create node/i }).click()
	await page.waitForLoadState('networkidle')
	// Wait for navigation to the node detail page or nodes list
	await page.getByText(/Hyperledger Fabric Node/i).waitFor({ state: 'visible', timeout: 60000 })

	return nodeName
}
