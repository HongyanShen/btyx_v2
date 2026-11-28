import assert from "node:assert/strict";
import path from "node:path";
import { chromium } from "playwright-core";
import { verifyCreativeDateTime } from "./verify-date-time-picker.mjs";
const browser = await chromium.launch({ channel: "msedge", headless: true });
try {
  const page = await browser.newPage({
    viewport: { width: 1280, height: 900 },
    locale: "zh-CN",
    timezoneId: "America/Los_Angeles",
  });
  page.setDefaultTimeout(10000);
  const base = process.env.BTYX_ADMIN_URL;
  await page.goto(base);
  await page.getByLabel("用户名").fill(process.env.BOOTSTRAP_ADMIN_USERNAME);
  await page
    .getByLabel("密码", { exact: true })
    .fill(process.env.BOOTSTRAP_ADMIN_PASSWORD);
  await page.getByRole("button", { name: "进入运营后台" }).click();
  await page.waitForURL("**/tasks");
  await page.goto(base + "/creative/tasks");
  await page.clock.setFixedTime(new Date("2026-11-30T16:05:00Z"));
  await verifyCreativeDateTime(page);
  const task = page
    .getByRole("row")
    .filter({ hasText: "双平台创作 · 点赞截图验收" });
  const code = await task.locator(".marketing-note").innerText();
  assert.match(code, /^RW-[A-Za-z0-9]{4,}$/);
  await page.goto(base + "/creative/works?state=linked");
  const row = page
    .locator("article")
    .filter({ hasText: "双平台创作 · 点赞截图验收" });
  const panels = row.locator(".creative-platform");
  assert.ok(
    (await row.getByRole("heading").innerText()).startsWith(code + " · "),
  );
  for (const [index, count] of ["6800", "1200"].entries()) {
    const panel = panels.nth(index);
    await panel.getByRole("button", { name: "查看点赞截图" }).click();
    const preview = page.getByRole("dialog");
    await preview.getByRole("button", { name: "放大截图" }).waitFor();
    assert.equal(
      await preview.locator("img").evaluate((img) => img.naturalWidth),
      640,
    );
    await preview.getByRole("button", { name: "关闭截图" }).click();
    await panel.getByLabel("点赞数").fill(count);
  }
  const evidence = path.resolve(
    import.meta.dirname,
    "../../../../../artifacts/screenshots",
  );
  await page.screenshot({
    path: path.join(evidence, "1126-manual-review.png"),
  });
  page.once("dialog", (dialog) => dialog.accept());
  await row.getByRole("button", { name: "核定点赞奖励" }).click();
  await row.waitFor({ state: "hidden" });
  await page.getByRole("link", { name: "已核奖励" }).click();
  await row.getByText("已核定 6800 赞").waitFor();
  await row.getByText("已核定 1200 赞").waitFor();
  await row.getByText(/点赞奖励.*266.00.*待结算/).waitFor();
  await page.screenshot({
    path: path.join(evidence, "1126-manual-review-result.png"),
  });
} finally {
  await browser.close();
}
