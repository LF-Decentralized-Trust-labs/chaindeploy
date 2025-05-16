import { expect } from '@playwright/test'

const ORGANIZATIONS_PATH = '/fabric/organizations'

// Reusable function to create an organization
export async function createOrganization(page, baseURL, { mspId, description, providerIndex = 0 }) {
	// 1. Go to organizations page
	await page.goto(baseURL + ORGANIZATIONS_PATH)
	await expect(page.getByRole('heading', { name: 'Organizations' })).toBeVisible({ timeout: 10000 })

	// 2. Open the create organization dialog
	await page.getByRole('button', { name: /add organization/i }).click()

	// 3. Fill in the form
	await page.getByPlaceholder('Enter MSP ID').fill(mspId)
	await page.getByPlaceholder('Enter organization description').fill(description)

	// If provider select is present, select the specified option
	const providerSelect = page.getByRole('combobox', { name: /key provider/i })
	if (await providerSelect.isVisible().catch(() => false)) {
		await providerSelect.click()
		const option = page.locator('[role="option"]').nth(providerIndex)
		if (await option.isVisible().catch(() => false)) {
			await option.click()
		}
	}

	// 4. Submit the form
	await page.getByRole('button', { name: /create organization/i }).click()
	await expect(page.getByRole('dialog')).not.toBeVisible({ timeout: 10000 })

	// 5. Assert the new organization appears in the list
	await expect(page.getByText(mspId)).toBeVisible({ timeout: 10000 })
	await expect(page.getByText(description)).toBeVisible({ timeout: 10000 })
} 