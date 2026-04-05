import { Page, expect } from '@playwright/test'

const USERNAME = process.env.PLAYWRIGHT_USER
const PASSWORD = process.env.PLAYWRIGHT_PASSWORD

// Reusable login function
// ProtectedLayout renders the login form inline at any URL when unauthenticated,
// so we navigate to '/' (not '/login', which doesn't exist as a route).
export async function login(page: Page, baseURL: string) {
	await page.goto(baseURL + '/')
	await expect(page.getByPlaceholder('Enter your username')).toBeVisible({ timeout: 10000 })
	await expect(page.getByPlaceholder('Enter your password')).toBeVisible()

	await page.getByPlaceholder('Enter your username').fill(USERNAME || '')
	await page.getByPlaceholder('Enter your password').fill(PASSWORD || '')
	const signInButton = page.getByRole('button', { name: /sign in/i })
	await signInButton.waitFor({ state: 'visible' })
	await signInButton.click()

	await expect(page.getByRole('heading', { name: /^(Nodes|Dashboard|Create your first node)$/ })).toBeVisible({ timeout: 10000 })
}
