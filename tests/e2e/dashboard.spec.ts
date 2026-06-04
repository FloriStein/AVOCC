import { test, expect } from '@playwright/test'

// E2E-Baseline: Dashboard lädt und zeigt initialen Zustand (ADR-006 non-blocking).
// Voraussetzung: Docker-Stack auf localhost:3000 läuft.

test.describe('AVOC Dashboard', () => {
  test('Dashboard lädt und zeigt AVOC-Header', async ({ page }) => {
    await page.goto('/')
    await expect(page.locator('h1')).toContainText('AVOC')
  })

  test('Initialer SYSTEM STATE ist IDLE', async ({ page }) => {
    await page.goto('/')
    await expect(page.locator('text=IDLE')).toBeVisible({ timeout: 5_000 })
  })

  test('Safety Panel ist sichtbar', async ({ page }) => {
    await page.goto('/')
    await expect(page.locator('text=Safety')).toBeVisible()
  })

  test('Connection Panel ist sichtbar', async ({ page }) => {
    await page.goto('/')
    await expect(page.locator('text=Connection')).toBeVisible()
  })

  test('Emergency Stop Button ist im DOM', async ({ page }) => {
    await page.goto('/')
    await expect(page.locator('button:has-text("Emergency Stop")')).toBeVisible()
  })
})
