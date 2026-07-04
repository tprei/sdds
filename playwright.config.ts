import { defineConfig, devices } from '@playwright/test';

const apiBaseURL =
  process.env.SDDS_SYNTHETICS_API_BASE_URL ?? 'http://127.0.0.1:18080';
const webPort = process.env.SDDS_SYNTHETICS_WEB_PORT ?? '19006';
const webBaseURL =
  process.env.PLAYWRIGHT_BASE_URL ?? `http://localhost:${webPort}`;

export default defineConfig({
  outputDir: 'test-results',
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  reporter: [
    ['list'],
    ['html', { open: 'never', outputFolder: 'playwright-report' }],
  ],
  testDir: 'tests/synthetics',
  use: {
    baseURL: webBaseURL,
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure',
  },
  webServer: {
    command: 'pnpm --filter @sdds/mobile exec expo start --web --localhost',
    env: {
      EXPO_NO_TELEMETRY: '1',
      EXPO_PUBLIC_SDDS_API_BASE_URL: apiBaseURL,
      RCT_METRO_PORT: webPort,
    },
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
    url: webBaseURL,
  },
  workers: process.env.CI ? 1 : undefined,
});
