import { test, expect } from '@playwright/test'
import { login } from './login'
import { createOrganization } from './create-organization'
// Helper to generate unique values
function uniqueSuffix() {
	const bytes = new Uint8Array(4)
	crypto.getRandomValues(bytes)
	const randomNum = new DataView(bytes.buffer).getUint32(0) % 10000
	return `${Date.now()}-${randomNum}`
}

const NODE_CREATE_PATH = '/nodes/create'

// This test assumes the admin user is set in env vars for login
// and that at least one organization exists to select

test('can create a Fabric peer node using the NodeCreationWizard', async ({ page, baseURL }) => {
	// Step 1: Login as admin
	await login(page, baseURL ?? '')
	await createOrganization(page, baseURL ?? '', { mspId: `test-msp-${uniqueSuffix()}`, description: 'Test organization' })
	// Step 2: Go to node creation wizard
	await page.goto((baseURL ?? '') + NODE_CREATE_PATH)
	await expect(page.getByRole('heading', { name: /create node/i })).toBeVisible()

	// Step 3: Wizard - Select Protocol (Fabric)
	const nodeName = `test-peer-${uniqueSuffix()}`
	await page.getByPlaceholder('Enter node name').fill(nodeName)
	await page.getByRole('button', { name: 'Fabric' }).click()
	await page.getByRole('button', { name: /next/i }).click()

	// Step 4: Wizard - Select Node Type (Peer)
	await page.getByRole('button', { name: 'Peer node' }).click()
	await page.getByRole('button', { name: /next/i }).click()

	// Step 5: Wizard - Configuration
	await page.getByPlaceholder('Enter node name').fill(nodeName)

	// Select the first available organization (assume dropdown is present)
	const orgSelect = page.getByRole('combobox', { name: /organization/i })
	await orgSelect.click()
	// Select the first option (could be improved to select by name if needed)
	await page.getByRole('option').first().click()

	// Select deployment mode "Docker"
	const modeSelect = page.getByRole('combobox', { name: /mode/i })
	await modeSelect.click()
	await page.getByRole('option', { name: /docker/i }).click()
	// Listen Address
	const listenPort = 7000 + (new DataView(crypto.getRandomValues(new Uint8Array(4)).buffer).getUint32(0) % 1000)
	await page.getByPlaceholder('e.g., 0.0.0.0:7051').fill(`0.0.0.0:${listenPort}`)

	// Operations Address
	const opsPort = 9000 + (new DataView(crypto.getRandomValues(new Uint8Array(4)).buffer).getUint32(0) % 1000)
	await page.getByPlaceholder('e.g., 0.0.0.0:9443').fill(`0.0.0.0:${opsPort}`)

	// External Endpoint
	const extPort = 7000 + (new DataView(crypto.getRandomValues(new Uint8Array(4)).buffer).getUint32(0) % 1000)
	await page.getByPlaceholder('e.g., peer0.org1.example.com:7051').fill(`peer0.example.com:${extPort}`)

	// Go to Review step
	await page.getByRole('button', { name: /next/i }).click()

	// Step 6: Review and Submit
	await page.getByRole('button', { name: /create node/i }).click()

	// Wait for navigation to the node detail page or nodes list
	await expect(page.getByText(/General Information/i)).toBeVisible({ timeout: 60000 })
})
