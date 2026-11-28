import assert from "node:assert/strict";
import fs from "node:fs/promises";
import path from "node:path";
import { chromium } from "playwright-core";
import { verifyDirectUpload } from "./verify-upload.mjs";

const root = path.resolve(import.meta.dirname, "../../../../..");
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

const adminURL = process.env.BTYX_ADMIN_URL ?? "http://127.0.0.1:8083";
const browser = await chromium.launch({ channel: "msedge", headless: true });

try {
  const page = await browser.newPage({
    viewport: { width: 1440, height: 1050 },
    locale: "zh-CN",
  });
  const failures = [];
  page.on("pageerror", (error) => failures.push(error.message));
  page.on("response", (response) => {
    if (
      new URL(response.url()).origin === new URL(adminURL).origin &&
      !new URL(response.url()).pathname.startsWith("/assets/") &&
      response.status() >= 400
    ) {
      failures.push(
        `HTTP ${response.status()} ${new URL(response.url()).pathname}`,
      );
    }
  });

  await page.goto(adminURL);
  await page.getByLabel("用户名").fill(username);
  await page.getByLabel("密码", { exact: true }).fill(password);
  await page.getByRole("button", { name: "进入运营后台" }).click();
  await page
    .getByRole("heading", { name: "营销分发 · 任务管理", exact: true })
    .waitFor();
  await page
    .getByRole("button", { name: "上传营销内容", exact: true })
    .waitFor();
  await page
    .getByRole("table", { name: "营销内容", exact: true })
    .first()
    .waitFor();
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
    ["分发结算", "营销分发 · 分发结算"],
    ["营销设置", "营销分发 · 营销设置"],
    ["实名认证记录", "报名审核 · 实名认证记录"],
    ["提现记录"],
    ["余额流水"],
    ["分发协议"],
    ["反馈与咨询"],
    ["操作审计"],
    ["运营账号"],
  ]) {
    const section = page.locator(".sidebar details").filter({
      has: page.getByRole("link", {
        name: menu,
        exact: true,
        includeHidden: true,
      }),
    });
    if (!(await section.evaluate((el) => el.open)))
      await section.locator("summary").click();
    await page.getByRole("link", { name: menu, exact: true }).click();
    await page.getByRole("heading", { name: title, exact: true }).waitFor();
    await page.waitForLoadState("networkidle");
    if (menu === "分发员报名审核") {
      await page
        .getByRole("columnheader", { name: "抖音号", exact: true })
        .waitFor();
      const statusResponse = page.waitForResponse((response) => {
        const url = new URL(response.url());
        return (
          url.pathname === "/users" &&
          response.request().headers()["hx-request"] === "true" &&
          url.searchParams.get("status") === "pending"
        );
      });
      await page
        .getByRole("combobox", { name: "按状态筛选" })
        .selectOption("pending");
      assert.equal((await statusResponse).status(), 200);
      const tagOptions = page.locator("#enrollment-filter-tags option");
      if (await tagOptions.count()) {
        const option = tagOptions.first();
        const tagName = await option.getAttribute("value");
        const tagKey = await option.getAttribute("data-tag-key");
        const filteredResponse = page.waitForResponse((response) => {
          const url = new URL(response.url());
          return (
            url.pathname === "/users" &&
            url.searchParams.get("tag") === tagKey &&
            url.searchParams.get("status") === "pending"
          );
        });
        const tag = page.getByLabel("按标签筛选", { exact: true });
        await tag.fill(tagName);
        await tag.press("Tab");
        assert.equal((await filteredResponse).status(), 200);
      }
    }
    assert.equal(
      await page.getByRole("alert").count(),
      0,
      `Error state on ${title}`,
    );
  }

  await verifyDirectUpload(page);
  assert.deepEqual(failures, []);
  await page.getByRole("button", { name: "退出", exact: true }).click();
  await page.getByRole("button", { name: "进入运营后台" }).waitFor();
  console.log(
    "管理后台浏览器验证通过：登录、页面导航、报名筛选、模拟 TOS 直传、退出。",
  );
} finally {
  await browser.close();
}
