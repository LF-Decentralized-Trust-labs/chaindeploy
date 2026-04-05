import { test, expect } from '@playwright/test'
import { login } from './login'

// Helper to generate a unique username with cryptographically secure random values
function uniqueUsername() {
  const bytes = new Uint8Array(4);
  crypto.getRandomValues(bytes);
  const randomNum = new DataView(bytes.buffer).getUint32(0) % 10000;
  return `testuser_${Date.now()}_${randomNum}`;
}

const USER_MANAGEMENT_PATH = '/users'
const NODES_PATH = '/nodes'

// This test assumes the admin user is set in env vars for login
// and that the admin can create users

test('can create a user, logout, login as that user, and see nodes list', async ({ page, baseURL }) => {
  // Step 1: Login as admin
  await login(page, baseURL ?? '')

  // Step 2: Go to user management
  await page.goto((baseURL ?? '') + USER_MANAGEMENT_PATH)
  await expect(page.getByRole('heading', { name: /users/i })).toBeVisible()

  // Step 3: Open the create user dialog
  await page.getByRole('button', { name: /add user/i }).click()
  await expect(page.getByRole('dialog')).toBeVisible()

  // Step 4: Fill in the user creation form
  const username = uniqueUsername()
  const password = 'TestPassword123!'
  await page.getByLabel('Username').fill(username)
  await page.getByLabel('Password').fill(password)
  await page.getByLabel('Role').click()
  await page.getByRole('option', { name: /viewer/i }).click()
  await page.getByRole('button', { name: /create user/i }).click()

  // Wait for dialog to close and user to appear in the list
  await expect(page.getByRole('dialog')).not.toBeVisible({ timeout: 10000 })
  await expect(page.getByText(username)).toBeVisible({ timeout: 10000 })

  // Step 5: Logout
  // Open account/profile menu and click logout (assuming a button or menu exists)
  await page.locator('#user-menu-trigger').click()
  await page.getByRole('menuitem', { name: /Log out/i }).click()

  // Confirm the logout confirmation dialog appears
  await expect(page.getByRole('alertdialog')).toBeVisible()
  // Click the confirm button (assuming it is labeled 'Log out' or similar)
  await page.getByRole('button', { name: /Log out/i }).click()

  // Step 6: Login as the new user
  await expect(page.getByPlaceholder('Enter your username')).toBeVisible()
  await expect(page.getByPlaceholder('Enter your password')).toBeVisible()
  await page.getByPlaceholder('Enter your username').fill(username)
  await page.getByPlaceholder('Enter your password').fill(password)
  await page.getByRole('button', { name: /sign in/i }).click()

  // Step 7: Verify nodes page is visible (empty state shows "Create your first node")
  await expect(page).toHaveURL(/.*\/nodes$/, { timeout: 10000 })
  await expect(page.getByRole('heading', { name: /^(Nodes|Create your first node)$/ })).toBeVisible({ timeout: 10000 })
}) 