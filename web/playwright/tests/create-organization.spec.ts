import { test } from '@playwright/test'
import { createOrganization } from './create-organization'
import { login } from './login'

test('can login and create a new organization', async ({ page, baseURL }) => {
	await login(page, baseURL ?? '')

	// Use a unique suffix for each test run
	const UNIQUE_SUFFIX = `${Date.now()}-${Math.floor(Math.random() * 10000)}`
	const mspId = `test-msp-${UNIQUE_SUFFIX}`
	const description = `Test organization created by Playwright ${UNIQUE_SUFFIX}`
	await createOrganization(page, baseURL, { mspId, description })
})
