import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: '../tests/e2e',
  timeout: 30_000,
  retries: 1,
  use: {
    baseURL: 'http://localhost:3000',
    // WebRTC flags (ADR-006 — non-blocking E2E)
    launchOptions: {
      args: [
        '--allow-insecure-localhost',
        '--use-fake-ui-for-media-stream',
        '--use-fake-device-for-media-stream',
      ],
    },
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  // Non-blocking: E2E failures don't break CI (ADR-006)
  reporter: [['html', { outputFolder: '../tests/e2e/reports' }]],
})
