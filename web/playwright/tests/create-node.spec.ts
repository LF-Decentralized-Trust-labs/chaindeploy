import { test, expect } from '@playwright/test'
import { login } from './login'
import { createOrganization } from './create-organization'

const FABRIC_NODE_CREATE_PATH = '/nodes/fabric/create'

// Helper to generate unique values
function uniqueSuffix() {
	return `${Date.now()}-${Math.floor(Math.random() * 10000)}`
}

test('can login, create an organization, and create a Fabric node', async ({ page, baseURL }) => {
	await login(page, baseURL ?? '')

	// Create a unique organization
	const UNIQUE_SUFFIX = uniqueSuffix()
	const mspId = `test-msp-${UNIQUE_SUFFIX}`
	const description = `Test organization created by Playwright ${UNIQUE_SUFFIX}`
	await createOrganization(page, baseURL, { mspId, description })

	// Go to Fabric node creation page
	await page.goto(baseURL + FABRIC_NODE_CREATE_PATH)
	await expect(page.getByRole('heading', { name: /create fabric node/i })).toBeVisible({ timeout: 10000 })

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

	// Node type (default is peer, skip unless you want to test orderer)

	// Listen Address
	await page.getByPlaceholder('e.g., 0.0.0.0:7051').fill(`0.0.0.0:${7000 + Math.floor(Math.random() * 1000)}`)
	// Operations Address
	await page.getByPlaceholder('e.g., 0.0.0.0:9443').fill(`0.0.0.0:${9000 + Math.floor(Math.random() * 1000)}`)
	// External Endpoint
	await page.getByPlaceholder('e.g., peer0.org1.example.com:7051').fill(`peer0.example.com:${7000 + Math.floor(Math.random() * 1000)}`)

	// Submit
	await page.getByRole('button', { name: /create node/i }).click()

	// Wait for navigation to the node detail page or nodes list
	await expect(page.getByText(/General Information/i)).toBeVisible({ timeout: 60000 })
})
