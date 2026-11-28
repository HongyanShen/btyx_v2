import assert from "node:assert/strict";
import fs from "node:fs/promises";
import path from "node:path";
import { chromium } from "playwright-core";

const root = path.resolve(import.meta.dirname, "../../../../..");
const config = await fs.readFile(path.join(root, ".env"), "utf8");
const username =
  process.env.BOOTSTRAP_ADMIN_USERNAME ??
  /^BOOTSTRAP_ADMIN_USERNAME=(.+)$/m.exec(config)?.[1].trim();
const password =
  process.env.BOOTSTRAP_ADMIN_PASSWORD ??
  /^BOOTSTRAP_ADMIN_PASSWORD=(.+)$/m.exec(config)?.[1].trim();
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
  const errors = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.goto(new URL("/tasks", adminURL).href);
  await page.getByLabel("用户名").fill(username);
  await page.getByLabel("密码", { exact: true }).fill(password);
  const settingsResponse = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname === "/tasks" &&
      response.request().method() === "GET",
  );
  await page.getByRole("button", { name: "进入运营后台" }).click();
  assert.equal((await settingsResponse).status(), 200);
  await page
    .getByRole("heading", { name: "营销分发 · 任务管理", exact: true })
    .waitFor();
  await page.getByRole("link", { name: "营销设置", exact: true }).waitFor();
  assert.equal(await page.getByRole("region", { name: "营销设置" }).count(), 0);
  assert.equal(await page.getByText("营销分发设置加载失败").count(), 0);
  await page.locator(".marketing-content-row").first().waitFor();

  const code = await page
    .locator(".marketing-content-row .marketing-id")
    .first()
    .innerText();
  assert.match(code, /^RW-[A-Za-z0-9]{4,}$/);
  const search = page.getByRole("searchbox", { name: "任务编号" });
  for (const value of [code, code.slice(3), "RW-????", ""]) {
    const filtered = page.waitForResponse((response) => {
      const url = new URL(response.url());
      return (
        url.pathname === "/tasks" && url.searchParams.get("task_code") === value
      );
    });
    await search.fill(value);
    await search.press("Tab");
    assert.equal((await filtered).status(), 200);
    await page.waitForFunction(
      (expected) =>
        document.querySelector("#marketing-task-code")?.defaultValue ===
        expected,
      value,
    );
    if (value === "RW-????") {
      await page.getByText("没有符合筛选的内容").waitFor();
    } else if (value) {
      await page.getByRole("cell", { name: code, exact: true }).waitFor();
      assert.equal(await page.locator(".marketing-content-row").count(), 1);
    }
  }

  const layout = await page.evaluate(() => {
    const style = (selector) =>
      getComputedStyle(document.querySelector(selector));
    const bounds = (selector) =>
      document.querySelector(selector).getBoundingClientRect().toJSON();

    return {
      workspace: bounds(".workspace"),
      sidebar: bounds(".sidebar"),
      sidebarColor: style(".sidebar").backgroundColor,
      selectedColor: style(".ant-menu-item-selected").backgroundColor,
      contentColor: style(".workspace-content").backgroundColor,
      contentPadding: style(".workspace-content").padding,
      titleSize: style(".page-heading h2").fontSize,
      cellPadding: style(".marketing-content-row td").padding,
      buttonHeight: bounds(".marketing-toolbar .ant-btn").height,
    };
  });
  assert.equal(layout.workspace.width, 1440);
  assert.equal(layout.workspace.x, 0);
  assert.equal(layout.workspace.y, 0);
  assert.ok(layout.workspace.height >= 1050);
  assert.equal(layout.sidebar.width, 210);
  assert.equal(layout.sidebarColor, "rgb(45, 38, 84)");
  assert.equal(layout.selectedColor, "rgb(139, 130, 190)");
  assert.equal(layout.contentColor, "rgb(255, 255, 255)");
  assert.equal(layout.contentPadding, "22px 28px");
  assert.equal(layout.titleSize, "20px");
  assert.equal(layout.cellPadding, "8px 10px");
  assert.equal(layout.buttonHeight, 35);

  await fs.mkdir(path.join(root, "artifacts/screenshots"), { recursive: true });
  await page.screenshot({
    path: path.join(root, "artifacts/screenshots/admin-tasks-restored.png"),
    fullPage: true,
    animations: "disabled",
  });
  for (const width of [390, 320]) {
    await page.setViewportSize({ width, height: 844 });
    assert.equal(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
      true,
      `Mobile page must not overflow at ${width}px`,
    );
    assert.equal(await page.locator(".sidebar").isVisible(), false);
    const menuButton = page.getByRole("button", {
      name: "打开导航菜单",
      exact: true,
    });
    const navigation = page.getByRole("dialog");
    assert.equal(
      await page.getByRole("button", { name: "素材库", exact: true }).count(),
      0,
    );
    await menuButton.click();
    await navigation
      .getByRole("link", { name: "任务管理", exact: true })
      .click();
    await navigation.waitFor({ state: "hidden" });
    await page
      .getByRole("heading", { name: "营销分发 · 任务管理", exact: true })
      .waitFor();
    await page
      .getByRole("button", { name: "上传营销内容", exact: true })
      .click();
    await page.getByRole("dialog").waitFor();
    const footerFits = await page
      .locator(".ant-modal-footer")
      .evaluate(
        (footer) => footer.getBoundingClientRect().bottom <= innerHeight,
      );
    assert.equal(
      footerFits,
      true,
      `Mobile form actions must stay in view at ${width}px`,
    );
    assert.equal(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
      true,
    );
    await page.getByRole("button", { name: "取消", exact: true }).click();
    await page.getByRole("dialog").waitFor({ state: "hidden" });
    await page.screenshot({
      path: path.join(
        root,
        `artifacts/screenshots/admin-tasks-mobile-${width}.png`,
      ),
      fullPage: true,
      animations: "disabled",
    });
  }
  await page.getByRole("button", { name: "上传营销内容", exact: true }).click();
  await page.getByRole("dialog").waitFor();
  await page.getByLabel("任务标题", { exact: true }).waitFor();
  await page.getByRole("dialog").screenshot({
    path: path.join(root, "artifacts/screenshots/admin-task-form-restored.png"),
  });
  await page.getByRole("button", { name: "取消", exact: true }).click();
  await page.getByRole("dialog").waitFor({ state: "hidden" });

  await page.setViewportSize({ width: 900, height: 900 });
  assert.equal(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
    true,
    "Narrow layout must keep table scrolling inside the content area",
  );
  await page.screenshot({
    path: path.join(root, "artifacts/screenshots/admin-tasks-narrow.png"),
    fullPage: true,
    animations: "disabled",
  });
  await page.setViewportSize({ width: 390, height: 844 });
  for (const [menu, title = menu] of [
    ["分发员报名审核", "营销分发 · 分发员报名审核"],
    ["分发员管理", "营销分发 · 分发员管理"],
    ["发布链接审核", "营销分发 · 发布链接审核"],
    ["分发结算", "营销分发 · 分发结算"],
    ["提现记录"],
    ["余额流水"],
    ["分发协议"],
    ["反馈与咨询"],
    ["操作审计"],
    ["任务管理", "营销分发 · 任务管理"],
  ]) {
    await page
      .getByRole("button", { name: "打开导航菜单", exact: true })
      .click();
    const navigation = page.getByRole("dialog");
    const section = navigation.locator("details").filter({
      has: page.getByRole("link", {
        name: menu,
        exact: true,
        includeHidden: true,
      }),
    });
    if (!(await section.evaluate((element) => element.open)))
      await section.locator("summary").click();
    await navigation.getByRole("link", { name: menu, exact: true }).click();
    await navigation.waitFor({ state: "hidden" });
    await page.getByRole("heading", { name: title, exact: true }).waitFor();
    await page.waitForLoadState("networkidle");
    assert.equal(
      await page.getByRole("alert").count(),
      0,
      `Mobile error state on ${title}`,
    );
    assert.equal(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
      true,
      `Mobile overflow on ${title}`,
    );
  }
  assert.deepEqual(errors, []);
  console.log(
    "任务页样式验收通过：桌面满屏、210px 深紫侧栏、品牌色、标题与表格密度、新建弹窗、900/390/320px 无整页横向溢出、手机抽屉导航与切页。未提交业务数据。",
  );
} finally {
  await browser.close();
}
