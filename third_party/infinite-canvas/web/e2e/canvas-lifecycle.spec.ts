import { expect, test, type Page } from "@playwright/test";

const APP_BASE = "/infinite-canvas";

test("canvas project persists across reload and project switching", async ({ page }) => {
    await page.goto(`${APP_BASE}/canvas`);
    const create = page.getByRole("button", { name: /New canvas|新建画布/ }).last();
    await expect(create).toBeEnabled();
    await create.click();
    await expect(page).toHaveURL(/\/infinite-canvas\/canvas\/[^/?#]+/);
    const firstProjectUrl = page.url();

    await page.reload();
    await expect(page).toHaveURL(firstProjectUrl);
    await expect(page.getByRole("button", { name: /Open canvas menu|打开画布菜单/ })).toBeVisible();

    await page.getByRole("button", { name: /Open canvas menu|打开画布菜单/ }).click();
    await page.getByRole("menuitem", { name: /New canvas|新建画布/ }).click();
    await expect(page).toHaveURL(/\/infinite-canvas\/canvas\/[^/?#]+/);
    expect(page.url()).not.toBe(firstProjectUrl);

    await page.getByRole("button", { name: /Open canvas menu|打开画布菜单/ }).click();
    await page.getByRole("menuitem", { name: /My Canvases|我的画布/ }).click();
    await expect(page).toHaveURL(/\/infinite-canvas\/canvas\/?$/);
});

test("video generation can be canceled and a pending task resumes after reload", async ({ page }) => {
    await seedMockProvider(page);
    let createCount = 0;
    let pollCount = 0;

    await page.route("https://mock.example/**", async (route) => {
        const request = route.request();
        const url = new URL(request.url());
        if (request.method() === "POST" && url.pathname === "/v1/videos") {
            createCount += 1;
            await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ id: `task-${createCount}`, status: "queued" }) });
            return;
        }
        if (request.method() === "GET" && /\/v1\/videos\/task-/.test(url.pathname)) {
            pollCount += 1;
            await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ id: url.pathname.split("/").pop(), status: "processing" }) });
            return;
        }
        await route.fulfill({ status: 404, contentType: "application/json", body: "{}" });
    });

    await page.goto(`${APP_BASE}/video`);
    const prompt = page.getByPlaceholder(/camera movement|镜头运动/);
    await prompt.fill("E2E video lifecycle");
    await page.getByRole("button", { name: /Generate|开始生成/ }).click();
    await expect(page.getByRole("button", { name: /Cancel generation|取消生成/ })).toBeVisible();
    await expect.poll(() => pollCount).toBeGreaterThan(0);

    await page.getByRole("button", { name: /Cancel generation|取消生成/ }).click();
    await expect(page.getByRole("button", { name: /Generate|开始生成/ })).toBeVisible();

    await prompt.fill("E2E resume lifecycle");
    await page.getByRole("button", { name: /Generate|开始生成/ }).click();
    await expect.poll(() => createCount).toBeGreaterThanOrEqual(2);
    await page.reload();
    await expect.poll(() => pollCount).toBeGreaterThan(1);
    await expect(page.getByRole("button", { name: /Cancel generation|取消生成/ })).toBeVisible();
    await page.getByRole("button", { name: /Cancel generation|取消生成/ }).click();
});

test("local storage diagnostics are available", async ({ page }) => {
    await page.goto(`${APP_BASE}/config`);
    await page.getByText(/Local storage|本地存储/, { exact: true }).first().click();
    await expect(page.getByText(/IndexedDB storage usage|IndexedDB 存储使用情况/)).toBeVisible();
    await expect(page.getByRole("button", { name: /Clean old history|清理过期记录/ })).toBeVisible();
});

async function seedMockProvider(page: Page) {
    await page.goto(`${APP_BASE}/config`);
    await page.waitForTimeout(300);
    await page.evaluate(async () => {
        const db = await new Promise<IDBDatabase>((resolve, reject) => {
            const request = indexedDB.open("infinite-canvas");
            request.onerror = () => reject(request.error);
            request.onsuccess = () => resolve(request.result);
        });
        const persisted = {
            state: {
                config: {
                    channelMode: "local",
                    baseUrl: "https://mock.example",
                    apiKey: "e2e-key",
                    apiFormat: "openai",
                    channels: [{ id: "default", name: "E2E", baseUrl: "https://mock.example", apiKey: "e2e-key", apiFormat: "openai", models: [
                        { name: "gpt-image-2", capability: "image" },
                        { name: "grok-imagine-video", capability: "video" },
                        { name: "gpt-5.5", capability: "text" },
                    ] }],
                    model: "default::gpt-image-2",
                    imageModel: "default::gpt-image-2",
                    videoModel: "default::grok-imagine-video",
                    textModel: "default::gpt-5.5",
                    models: ["default::gpt-image-2", "default::grok-imagine-video", "default::gpt-5.5"],
                },
            },
            version: 0,
        };
        await new Promise<void>((resolve, reject) => {
            const tx = db.transaction("app_state", "readwrite");
            tx.onerror = () => reject(tx.error);
            tx.oncomplete = () => resolve();
            tx.objectStore("app_state").put(JSON.stringify(persisted), "infinite-canvas:ai_config_store");
        });
        db.close();
    });
    await page.reload();
}
