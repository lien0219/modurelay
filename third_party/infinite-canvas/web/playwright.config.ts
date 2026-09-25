import { defineConfig } from "@playwright/test";

export default defineConfig({
    testDir: "./e2e",
    timeout: 45_000,
    expect: { timeout: 8_000 },
    fullyParallel: false,
    retries: 1,
    workers: 1,
    use: {
        baseURL: "http://127.0.0.1:3000",
        trace: "retain-on-failure",
    },
    webServer: {
        command: "bun run start -- --base /infinite-canvas/",
        url: "http://127.0.0.1:3000/infinite-canvas/",
        timeout: 120_000,
        reuseExistingServer: false,
    },
});
