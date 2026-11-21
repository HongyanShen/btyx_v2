import assert from "node:assert/strict";
import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "playwright-core";
import { verifyDirectUpload } from "./verify-upload.mjs";

const root = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../../..",
);
const localConfig = await fs.readFile(path.join(root, ".env"), "utf8");
const username =
  process.env.BOOTSTRAP_ADMIN_USERNAME ??
  /^BOOTSTRAP_ADMIN_USERNAME=(.+)$/m.exec(localConfig)?.[1].trim();
const password =
  process.env.BOOTSTRAP_ADMIN_PASSWORD ??
  /^BOOTSTRAP_ADMIN_PASSWORD=(.+)$/m.exec(localConfig)?.[1].trim();
assert.ok(
  username && password,
  "Test operator username and password are required",
);

const browser = await chromium.launch({ channel: "msedge", headless: true });

try {
  const page = await browser.newPage({
    viewport: { width: 1440, height: 1050 },
    locale: "zh-CN",
  });
  const failures = [];
  page.on("pageerror", (error) => failures.push(error.message));
  page.on("response", (response) => {
    if (response.url().includes("/api/v1/") && response.status() >= 400) {
      failures.push(
        `HTTP ${response.status()} ${new URL(response.url()).pathname}`,
      );
    }
  });

  await page.goto("http://127.0.0.1:5173");
  await page.getByLabel("用户名").fill(username);
  await page.getByLabel("密码", { exact: true }).fill(password);
  await page.getByRole("button", { name: "进入运营后台" }).click();
  await page
    .getByRole("heading", { name: "营销分发 · 任务管理", exact: true })
    .waitFor();
  await page
    .getByRole("button", { name: "上传营销内容", exact: true })
    .waitFor();
  await page.locator(".ant-table").waitFor();
  await page.waitForLoadState("networkidle");
  await fs.mkdir(path.join(root, "artifacts/screenshots"), { recursive: true });
  await page.screenshot({
    path: path.join(root, "artifacts/screenshots/admin-tasks.png"),
    fullPage: true,
  });

  for (const [menu, title = menu] of [
    ["分发员报名审核", "营销分发 · 分发员报名审核"],
    ["分发员管理", "营销分发 · 分发员管理"],
    ["发布链接审核", "营销分发 · 发布链接审核"],
    ["分发结算"],
    ["提现记录"],
    ["余额流水"],
    ["分发协议"],
    ["反馈与咨询"],
    ["操作审计"],
    ["运营账号"],
  ]) {
    await page
      .getByRole("menuitem", { name: menu, exact: true })
      .and(page.locator(":not([aria-disabled=true])"))
      .click();
    await page.getByRole("heading", { name: title, exact: true }).waitFor();
    await page.waitForLoadState("networkidle");
    if (menu === "分发员报名审核") {
      await page
        .getByRole("columnheader", { name: "抖音号", exact: true })
        .waitFor();
      await page.getByRole("combobox", { name: "按状态筛选" }).click();
      const statusResponse = page.waitForResponse((response) => {
        const url = new URL(response.url());
        return (
          url.pathname === "/api/v1/users" &&
          url.searchParams.get("status") === "pending"
        );
      });
      await page
        .locator(".ant-select-item-option-content")
        .filter({ hasText: /^待审核$/ })
        .click();
      assert.equal((await statusResponse).status(), 200);
      const tag = page.getByRole("combobox", { name: "按标签筛选" });
      await tag.click();
      await tag.fill("浏览器验证标签");
      const filteredResponse = page.waitForResponse((response) => {
        const url = new URL(response.url());
        return (
          url.pathname === "/api/v1/users" &&
          url.searchParams.get("tag") === "浏览器验证标签"
        );
      });
      await page
        .locator(".ant-select-item-option-content")
        .filter({ hasText: /^浏览器验证标签$/ })
        .click();
      const filtered = await filteredResponse;
      assert.equal(filtered.status(), 200);
      assert.equal(
        new URL(filtered.url()).searchParams.get("status"),
        "pending",
      );
      assert.match(filtered.headers()["x-total-count"], /^\d+$/);
    }
    assert.equal(
      await page.locator(".ant-alert-error").count(),
      0,
      `Error state on ${title}`,
    );
  }

  await verifyDirectUpload(page);
  assert.deepEqual(failures, []);
  await page.getByRole("button", { name: "退出", exact: true }).click();
  await page.getByRole("button", { name: "进入运营后台" }).waitFor();
  console.log(
    "管理后台浏览器验证通过：登录、页面读取接口、模拟 TOS 直传、退出。未连接真实 TOS。",
  );
} finally {
  await browser.close();
}
