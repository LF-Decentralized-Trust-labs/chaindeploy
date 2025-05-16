import { test, expect } from '@playwright/test'
import { login } from './login'

test('can login with valid credentials', async ({ page, baseURL }) => {
	await login(page, baseURL ?? '')
})
