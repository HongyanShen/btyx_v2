import assert from "node:assert/strict";
import fs from "node:fs/promises";
import path from "node:path";
import { chromium } from "playwright-core";
import { verifyEnrollment } from "./verify-enrollment.mjs";
import { verifyDateTimePicker } from "./verify-date-time-picker.mjs";
import { verifyIdentityRecords } from "./verify-identity-records.mjs";
import { verifyDirectUpload } from "./verify-upload.mjs";

const base = process.env.BTYX_BROWSER_URL;
const username = process.env.BTYX_BROWSER_USER;
const password = process.env.BTYX_BROWSER_PASSWORD;
const reviewer = process.env.BTYX_BROWSER_REVIEWER;
const file = process.env.BTYX_BROWSER_FILE;
assert.ok(
  base && username && password && reviewer && file,
  "Run through TestAdminBrowser",
);

const screenshotDirectory = path.resolve(
  import.meta.dirname,
  "../../../../../artifacts/screenshots",
);
await fs.mkdir(screenshotDirectory, { recursive: true });
const browser = await chromium.launch({ channel: "msedge", headless: true });
const context = await browser.newContext({
  viewport: { width: 1440, height: 1000 },
  locale: "zh-CN",
  timezoneId: "Asia/Shanghai",
  reducedMotion: "reduce",
});
const page = await context.newPage();
page.setDefaultTimeout(10000);
const browserErrors = [];
page.on("pageerror", (error) => browserErrors.push(error.message));
page.on("console", (message) => {
  if (
    message.type() === "error" &&
    /Content Security Policy|Refused to|hydration|not a valid selector/.test(
      message.text(),
    )
  ) {
    browserErrors.push(message.text());
  }
});

const tasks = "营销分发 · 任务管理";
const reviews = "营销分发 · 发布链接审核";
const destinations = [
  ["任务管理", tasks],
  ["分发员报名审核", "营销分发 · 分发员报名审核"],
  ["分发员管理", "营销分发 · 分发员管理"],
  ["发布链接审核", reviews],
  ["分发结算", "营销分发 · 分发结算"],
  ["营销设置", "营销分发 · 营销设置"],
  ["提现记录", "提现记录"],
  ["实名认证记录", "报名审核 · 实名认证记录"],
  ["余额流水", "余额流水"],
  ["分发协议", "分发协议"],
  ["反馈与咨询", "反馈与咨询"],
  ["操作审计", "操作审计"],
  ["运营账号", "运营账号"],
];

async function visit(menu, title = menu, useMobileMenu = false) {
  if (useMobileMenu) {
    await page.getByRole("button", { name: "打开导航菜单" }).click();
  }

  const nav = useMobileMenu
    ? page.getByRole("dialog", { name: "导航菜单" })
    : page;
  const section = (useMobileMenu ? nav : page.locator(".sidebar"))
    .locator("details")
    .filter({
      has: page.getByRole("link", {
        name: menu,
        exact: true,
        includeHidden: true,
      }),
    });
  if (!(await section.evaluate((el) => el.open)))
    await section.locator("summary").click();
  const link = nav.getByRole("link", { name: menu, exact: true });
  const linkURL = await link.getAttribute("href");
  const html = page.waitForResponse(
    (response) =>
      response.request().headers()["hx-request"] === "true" &&
      response.request().method() === "GET" &&
      new URL(response.url()).pathname === new URL(linkURL, base).pathname,
  );
  await link.click();
  const response = await html;
  assert.equal(response.status(), 200);
  assert.match(response.headers()["content-type"], /text\/html/);
  await page.getByRole("heading", { name: title, exact: true }).waitFor();
  if (useMobileMenu) await nav.waitFor({ state: "hidden" });
}

async function screenshot(name, locator = page) {
  await locator.screenshot({
    path: path.join(screenshotDirectory, `htmx-${name}.png`),
    animations: "disabled",
  });
}

async function expectNoOverflow() {
  assert.equal(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
    true,
  );
}

async function login(account, secret) {
  await page.getByLabel("用户名").fill(account);
  await page.getByLabel("密码", { exact: true }).fill(secret);
  await page.getByRole("button", { name: "进入运营后台" }).click();
  await page.waitForURL((url) => url.pathname !== "/login", {
    waitUntil: "load",
  });
}

async function verifyWatermarkStyle(marks, minimumTiles) {
  await marks.first().waitFor();
  const style = await marks.first().evaluate((mark) => {
    const layer = mark.parentElement;
    const area = layer.getBoundingClientRect();
    const visibleTiles = [...layer.children].filter((tile) => {
      const bounds = tile.getBoundingClientRect();
      return bounds.bottom > area.top && bounds.top < area.bottom;
    }).length;
    const { fontSize, color } = getComputedStyle(mark);
    return { fontSize, color, visibleTiles };
  });
  assert.equal(style.fontSize, "11px");
  assert.equal(style.color, "rgba(255, 255, 255, 0.42)");
  assert.ok(style.visibleTiles >= minimumTiles);
}

async function verifyVideoWatermark() {
  const fixture = await page.evaluate(async () => {
    const canvas = document.createElement("canvas");
    canvas.width = 320;
    canvas.height = 180;
    const drawing = canvas.getContext("2d");
    const stream = canvas.captureStream(15);
    const recorder = new MediaRecorder(stream, { mimeType: "video/webm" });
    const chunks = [];
    const recorded = new Promise((resolve) => {
      recorder.addEventListener("dataavailable", (event) =>
        chunks.push(event.data),
      );
      recorder.addEventListener("stop", () => resolve(new Blob(chunks)));
    });
    recorder.start();
    const draw = () => {
      drawing.fillStyle = "#27446a";
      drawing.fillRect(0, 0, 320, 180);
      drawing.fillStyle = "#dedcee";
      drawing.font = "24px sans-serif";
      drawing.fillText("Playback preview", 54, 96);
    };
    draw();
    const frames = setInterval(draw, 65);
    await new Promise((resolve) => setTimeout(resolve, 1300));
    recorder.stop();
    clearInterval(frames);
    stream.getTracks().forEach((track) => track.stop());
    return Array.from(new Uint8Array(await (await recorded).arrayBuffer()));
  });

  const files = "**/api/v1/files/*/download";
  const videoURL = "**/qa-video-preview.webm";
  await page.route(files, (route) =>
    route.fulfill({
      json: {
        code: 0,
        data: {
          name: "预览验收视频.mp4",
          media_type: "video/mp4",
          storage: "tos",
          url: `${base}/qa-video-preview.webm`,
        },
      },
    }),
  );
  await page.route(videoURL, (route) =>
    route.fulfill({ body: Buffer.from(fixture), contentType: "video/webm" }),
  );
  try {
    await poolVideoPreview();
  } finally {
    await page.unroute(videoURL);
    await page.unroute(files);
  }
}

async function poolVideoPreview() {
  await page
    .locator(".marketing-content-row")
    .filter({ hasText: "指派撤回验收任务" })
    .getByRole("button", { name: "预览附件 1" })
    .click();
  const dialog = page.getByRole("dialog", { name: "附件预览" });
  const marks = dialog.getByText(`管理员 · ${username}`, { exact: true });
  const video = dialog.getByLabel("预览验收视频.mp4");
  await verifyWatermarkStyle(marks, 12);
  assert.equal(
    await video.evaluate((element) => {
      const overlay = element.parentElement.querySelector(
        ".operator-watermark",
      );
      return (
        element.controls && getComputedStyle(overlay).pointerEvents === "none"
      );
    }),
    true,
  );
  await video.evaluate(async (element) => {
    element.muted = true;
    await element.play();
  });
  await screenshot("video-watermark", dialog);
  await dialog.getByRole("button", { name: "全屏播放" }).click();
  await dialog.getByRole("button", { name: "退出全屏" }).waitFor();
  assert.equal(
    await marks
      .first()
      .evaluate((mark) => document.fullscreenElement.contains(mark)),
    true,
  );
  await verifyWatermarkStyle(marks, 12);
  await dialog.getByRole("button", { name: "退出全屏" }).click();
  await dialog.getByRole("button", { name: "全屏播放" }).waitFor();
  await dialog.getByRole("button", { name: /^关\s*闭$/ }).click();
  await dialog.waitFor({ state: "hidden" });
}

try {
  await page.goto(`${base}/tasks`);
  await page.getByRole("heading", { name: "欢迎回来" }).waitFor();
  assert.equal(
    await page.getByRole("button", { name: "上传营销内容" }).count(),
    0,
  );
  await page.getByLabel("用户名").fill(username);
  await page.getByLabel("密码", { exact: true }).fill("invalid-password");
  await page.getByRole("button", { name: "显示密码" }).click();
  assert.equal(await page.locator("#password").getAttribute("type"), "text");
  await page.getByRole("button", { name: "隐藏密码" }).click();
  const invalidLogin = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname === "/login" &&
      response.status() === 401,
  );
  await page.getByRole("button", { name: "进入运营后台" }).click();
  await invalidLogin;
  await page.locator("#login-form [role=alert]").waitFor();
  assert.equal(await page.locator("#password").inputValue(), "");
  await login(username, password);
  await page.getByRole("heading", { name: tasks, exact: true }).waitFor();
  await page.locator(".marketing-content-row").first().waitFor();

  const marketingGroup = page.locator(".sidebar details").filter({
    has: page.locator('ul[aria-label="营销分发"]'),
  });
  const summary = marketingGroup.locator("summary");
  const totalBadge = summary.locator(".nav-pending");
  const reviewsBadge = marketingGroup.locator(
    'a[href="/reviews"] .nav-pending',
  );
  assert.equal(await totalBadge.isVisible(), true);
  assert.equal(await reviewsBadge.isVisible(), true);
  assert.equal(await reviewsBadge.textContent(), "4");
  assert.equal(
    await marketingGroup.locator('a[href="/users"] .nav-pending').textContent(),
    "1",
  );
  assert.equal(await totalBadge.textContent(), "5");
  assert.equal(await marketingGroup.locator("a .nav-pending").count(), 2);
  for (const badge of [totalBadge, reviewsBadge]) {
    assert.equal(
      await badge.evaluate((el) => getComputedStyle(el).backgroundColor),
      "rgb(232, 83, 107)",
    );
  }
  await summary.click();
  assert.equal(await totalBadge.isVisible(), true);
  assert.equal(await reviewsBadge.isVisible(), false);
  await summary.click();
  assert.equal(await totalBadge.isVisible(), true);
  assert.equal(await reviewsBadge.isVisible(), true);

  const enrollment = await context.newPage();
  try {
    const pageRequest = "**/users?**";
    await enrollment.goto(`${base}/users`);
    await enrollment.getByRole("columnheader", { name: "抖音号" }).waitFor();
    await enrollment.route(pageRequest, (route) => route.abort("failed"));
    await enrollment.getByRole("button", { name: "刷新" }).click();
    const loadError = enrollment
      .getByRole("alert")
      .filter({ hasText: "网络连接失败" });
    await loadError.waitFor({ timeout: 10000 });
    await enrollment.unroute(pageRequest);
    const beforeRecovery = await enrollment
      .locator("#admin-workspace")
      .elementHandle();
    await enrollment.getByRole("button", { name: "刷新" }).click();
    await beforeRecovery.waitForElementState("hidden");
    await enrollment
      .locator("#admin-workspace.htmx-settling")
      .waitFor({ state: "hidden" });
    await loadError.waitFor({ state: "hidden" });
    await enrollment.getByRole("columnheader", { name: "抖音号" }).waitFor();
    await enrollment
      .getByLabel("新标签", { exact: true })
      .fill("浏览器验收标签");
    await enrollment.getByRole("button", { name: "＋ 添加" }).click();
    await enrollment.getByText("#浏览器验收标签", { exact: true }).waitFor();
    const tagFilter = enrollment.getByLabel("按标签筛选", { exact: true });
    await tagFilter.fill("浏览器验收标签");
    await tagFilter.press("Tab");
    await enrollment.getByText("没有符合筛选条件的报名").waitFor();
    await tagFilter.fill("");
    await tagFilter.press("Tab");
    await enrollment.getByRole("button", { name: "报名人" }).first().waitFor();
    await verifyEnrollment(enrollment, screenshot);
  } finally {
    await enrollment.close();
  }

  const cookie = (await page.context().cookies()).find(
    ({ name }) => name === "btyx_admin_web",
  );
  assert.ok(cookie?.httpOnly && cookie.sameSite === "Strict");
  assert.equal(
    await page.evaluate(() => sessionStorage.getItem("btyx-admin-session")),
    null,
  );
  const initial = await page.evaluate(() => ({
    sidebar: document.querySelector(".sidebar").getBoundingClientRect().width,
    color: getComputedStyle(document.querySelector(".sidebar")).backgroundColor,
    active: getComputedStyle(document.querySelector(".ant-menu-item-selected"))
      .backgroundColor,
    padding: getComputedStyle(document.querySelector(".workspace-content"))
      .padding,
    button: document
      .querySelector(".marketing-toolbar .ant-btn")
      .getBoundingClientRect().height,
  }));
  assert.deepEqual(initial, {
    sidebar: 210,
    color: "rgb(45, 38, 84)",
    active: "rgb(139, 130, 190)",
    padding: "22px 28px",
    button: 35,
  });
  await screenshot("tasks-desktop");
  const marketing = page.getByRole("list", { name: "营销分发" });
  assert.deepEqual(await marketing.locator(".nav-label").allTextContents(), [
    "任务管理",
    "创作任务",
    "创作作品审核",
    "分发员报名审核",
    "分发员管理",
    "发布链接审核",
    "分发结算",
    "提现记录",
    "营销设置",
  ]);
  assert.deepEqual(
    (await page.locator(".sidebar summary").allTextContents()).map((text) =>
      text.replace(/[\d\s]/g, ""),
    ),
    ["营销分发", "报名审核", "通用管理"],
  );
  assert.equal(await page.locator(".workspace-nav [aria-disabled]").count(), 0);
  for (const [alias, path] of [
    ["mkt-people", "/users/manage"],
    ["realname", "/admin/realname"],
    ["mktapps", "/users"],
  ]) {
    await page.goto(`${base}/#/admin/${alias}`);
    await page.waitForURL(`${base}${path}`);
  }
  await page.goto(base);
  await page.waitForURL(`${base}/tasks`);
  await page.getByRole("heading", { name: tasks, exact: true }).waitFor();

  await visit("营销设置", "营销分发 · 营销设置");
  const marketingPolicyForm = page.locator(
    'form[action="/distribution/settings/marketing"]',
  );
  await marketingPolicyForm
    .getByRole("link", { name: "管理创作任务上架 / 下架" })
    .waitFor();
  const settingsViewport = page.viewportSize();
  for (const width of [1440, 390, 320]) {
    await page.setViewportSize({ width, height: width === 1440 ? 1000 : 844 });
    await expectNoOverflow();
    const toggle = marketingPolicyForm.getByLabel("开启派单");
    const bounds = await toggle.boundingBox();
    assert.ok(
      bounds.width <= 48 && bounds.height <= 28,
      "派单开关应为紧凑控件",
    );
    const link = marketingPolicyForm.getByRole("link", {
      name: "管理创作任务上架 / 下架",
    });
    const linkBounds = await link.boundingBox();
    assert.ok(linkBounds.x >= 0 && linkBounds.x + linkBounds.width <= width);
    if (width === 390) await screenshot("settings-mobile");
  }
  await page.setViewportSize(settingsViewport);
  await screenshot("1126-settings");
  const dispatchSwitch = marketingPolicyForm.getByLabel("开启派单");
  const wasOnline = await dispatchSwitch.isChecked();
  await dispatchSwitch.focus();
  await page.keyboard.press("Space");
  assert.equal(await dispatchSwitch.isChecked(), !wasOnline);
  await page.keyboard.press("Space");
  await marketingPolicyForm.getByLabel("每个平台单价（元）").fill("8.15");
  const dailyLimit = marketingPolicyForm.getByLabel("每人每天可领（条）");
  await dailyLimit.fill("0");
  await marketingPolicyForm
    .getByRole("button", { name: "保存营销设置" })
    .click();
  assert.ok(
    await dailyLimit.evaluate(
      (input) =>
        input.validity.rangeUnderflow && input === document.activeElement,
    ),
  );
  await marketingPolicyForm.getByLabel("每人每天可领（条）").fill("2");
  const write = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname === "/distribution/settings/marketing" &&
      response.request().method() === "POST",
  );
  await marketingPolicyForm
    .getByRole("button", { name: "保存营销设置" })
    .click();
  const wrote = await write;
  assert.ok(wrote.status() >= 200 && wrote.status() < 400);
  assert.equal(wrote.request().headers().authorization, undefined);
  assert.ok(new URLSearchParams(wrote.request().postData()).get("csrf"));
  const openHistory = marketingPolicyForm.getByRole("button", {
    name: "修改记录",
  });
  await openHistory.click();
  const settingsHistory = page.getByRole("dialog", { name: "修改记录" });
  await settingsHistory.getByText("单价修改记录", { exact: true }).click();
  await settingsHistory
    .locator("details")
    .getByText(/¥8\.15/)
    .waitFor();
  await settingsHistory.getByRole("button", { name: "关闭" }).click();
  await openHistory.click();
  await page.keyboard.press("Escape");
  await settingsHistory.waitFor({ state: "hidden" });
  const timing = page.locator("[data-distribution-settings]");
  await timing.getByLabel("首次提交发布链接").selectOption("end_of_day");
  assert.equal(await timing.getByLabel("发布链接时限").isVisible(), false);
  await timing.getByLabel("首次提交发布链接").selectOption("fixed_duration");
  await timing.getByLabel("发布链接时限").fill("13.5");
  await timing.getByRole("button", { name: "保存分发设置" }).click();
  await page.getByText("分发设置已保存").waitFor();
  assert.equal(await timing.getByLabel("发布链接时限").inputValue(), "13.5");
  await visit("创作任务", "营销分发 · 创作任务");
  await page.getByText("＋ 新建创作任务", { exact: true }).click();
  const creativeForm = page.locator("form[data-creative-action=create]");
  await creativeForm.getByLabel("任务标题").fill("浏览器创作验收");
  await creativeForm.getByLabel("创作要求").fill("发布图文");
  await creativeForm.getByLabel("创作费（元）").fill("40");
  await creativeForm.getByLabel("最低点赞数").fill("3000");
  await creativeForm.getByLabel("奖励（元）").fill("266");
  await creativeForm.getByRole("button", { name: "添加档位" }).click();
  assert.equal(await creativeForm.getByLabel("最低点赞数").count(), 2);
  await creativeForm.getByRole("button", { name: "删除档位" }).last().click();
  for (const [name, value] of [
    ["start", "2026-11-01T00:00"],
    ["end", "2026-11-30T23:59"],
    ["reward", "2026-12-04T12:00"],
  ])
    await creativeForm.locator('[name="' + name + '"]').fill(value);
  await creativeForm.getByRole("button", { name: "创建并上架" }).click();
  await page.getByRole("row").filter({ hasText: "浏览器创作验收" }).waitFor();
  await screenshot("1126-creative-tasks");
  await visit("创作作品审核", "营销分发 · 创作作品审核");
  await page.getByText("暂无创作作品", { exact: true }).waitFor();
  for (const [menu, title] of destinations.slice(1)) {
    await visit(menu, title);
    await expectNoOverflow();
  }
  await verifyIdentityRecords(page, base, screenshot, expectNoOverflow);
  await visit("运营账号");
  await page.getByRole("button", { name: "新建运营账号" }).click();
  const newOperator = page.getByRole("dialog", { name: "新建运营账号" });
  await newOperator.waitFor();
  const pageGroup = newOperator.getByRole("group", { name: "页面权限" });
  const endpointGroup = newOperator.getByRole("group", { name: "接口权限" });
  for (const group of [pageGroup, endpointGroup]) {
    const all = group.getByRole("checkbox", { name: "全选", exact: true });
    const choices = group.locator("input:is([name=pages],[name=endpoints])");
    assert.ok((await choices.count()) > 1);
    await all.check();
    assert.equal(
      await choices.count(),
      await group
        .locator("input:is([name=pages],[name=endpoints]):checked")
        .count(),
    );
    assert.equal(
      await newOperator
        .locator("input:is([name=pages],[name=endpoints]):checked")
        .count(),
      await choices.count(),
    );
    await choices.first().uncheck();
    assert.equal(await all.evaluate((input) => input.indeterminate), true);
    await all.check();
    await all.uncheck();
    assert.equal(
      await group
        .locator("input:is([name=pages],[name=endpoints]):checked")
        .count(),
      0,
    );
    assert.equal(
      await newOperator
        .locator("input:is([name=pages],[name=endpoints]):checked")
        .count(),
      0,
    );
  }
  const endpointChoices = endpointGroup.locator('input[name="endpoints"]');
  const desktopViewport = page.viewportSize();
  for (const viewport of [desktopViewport, { width: 390, height: 844 }]) {
    await page.setViewportSize(viewport);
    assert.equal(await endpointGroup.getByRole("button").count(), 0);
    const readable = await endpointChoices.evaluateAll((inputs) =>
      inputs.every((input) => {
        const name = document.getElementById(
          input.getAttribute("aria-labelledby"),
        );
        const key = document.getElementById(
          input.getAttribute("aria-describedby"),
        );
        return (
          name &&
          key &&
          key.textContent === input.value &&
          key.getBoundingClientRect().height > 0 &&
          key.getBoundingClientRect().top >= name.getBoundingClientRect().bottom
        );
      }),
    );
    assert.equal(readable, true);
    assert.equal(
      await endpointGroup.evaluate(
        (group) => group.scrollWidth <= group.clientWidth,
      ),
      true,
    );
  }
  await page.setViewportSize(desktopViewport);
  await newOperator.getByLabel("用户名").fill(reviewer);
  await newOperator.getByLabel("初始密码").fill(password);
  const allPages = pageGroup.getByRole("checkbox", {
    name: "全选",
    exact: true,
  });
  await allPages.check();
  await newOperator.getByRole("button", { name: "保存" }).click();
  await newOperator.getByRole("alert").waitFor();
  assert.equal(await allPages.isChecked(), true);
  await allPages.uncheck();
  await newOperator.getByLabel("用户名").fill("browser-new-operator");
  await newOperator.getByLabel("初始密码").fill(password);
  await newOperator
    .getByRole("checkbox", { name: "实名认证记录", exact: true })
    .check();
  await newOperator
    .locator('input[value="GET /identity-verifications"]')
    .check();
  await newOperator.getByRole("button", { name: "保存" }).click();
  await page.getByText("运营账号已创建").waitFor();
  const newOperatorRow = page
    .getByRole("row")
    .filter({ hasText: "browser-new-operator" });
  await newOperatorRow.getByRole("button", { name: "编辑账号" }).click();
  const editOperator = page.getByRole("dialog", {
    name: "编辑运营账号 · browser-new-operator",
  });
  await editOperator.waitFor();
  for (const all of await editOperator
    .getByRole("checkbox", { name: "全选", exact: true })
    .all()) {
    assert.equal(await all.evaluate((input) => input.indeterminate), true);
  }
  assert.equal(
    await editOperator
      .getByRole("checkbox", { name: "实名认证记录", exact: true })
      .isChecked(),
    true,
  );
  for (const all of await editOperator
    .getByRole("checkbox", { name: "全选", exact: true })
    .all()) {
    await all.check();
  }
  await editOperator.getByRole("button", { name: "保存" }).click();
  await newOperatorRow
    .getByRole("cell", { name: "全部", exact: true })
    .nth(1)
    .waitFor();
  const accounts = (
    await (await page.request.get(base + "/api/v1/operators")).json()
  ).data;
  const fullAccount = accounts.find(
    (account) => account.username === "browser-new-operator",
  );
  assert.equal(fullAccount.all_pages, true);
  assert.equal(fullAccount.all_endpoints, true);
  assert.equal(fullAccount.super_admin, false);
  await newOperatorRow.getByRole("button", { name: "编辑账号" }).click();
  await editOperator.waitFor();
  await editOperator.getByRole("switch", { name: "停用账号" }).check();
  await editOperator.getByRole("button", { name: "保存" }).click();
  await newOperatorRow.getByText("已停用").waitFor();
  await page.goto(`${base}/contents`);
  await page.getByRole("heading", { name: "素材库", exact: true }).waitFor();
  await visit("操作审计");
  await verifyDateTimePicker(page, screenshot);
  const marketingEdits = page.getByText("修改营销分发设置", { exact: true });
  await marketingEdits.nth(1).waitFor();
  const auditTable = page.getByRole("region", { name: "操作审计表格" });
  await auditTable.getByText("更新运营账号", { exact: true }).first().waitFor();
  assert.deepEqual(
    await auditTable.getByRole("columnheader").allTextContents(),
    ["操作", "操作人", "平台", "接口结果", "原因 / 备注", "关联编号", "时间"],
  );
  await page.getByLabel("操作人名字").fill(username);
  await page.getByLabel("平台", { exact: true }).selectOption("admin");
  const endpointFilter = page.getByLabel("接口", { exact: true });
  await endpointFilter.selectOption("POST /operators");
  await page.getByRole("button", { name: "查询", exact: true }).click();
  await page.waitForURL(
    (url) => url.searchParams.get("endpoint") === "POST /operators",
  );
  await auditTable
    .getByRole("cell", { name: /失败 HTTP [45]\d\d/ })
    .first()
    .waitFor();
  await auditTable
    .getByRole("cell", { name: /成功 HTTP [23]\d\d/ })
    .first()
    .waitFor();
  await endpointFilter.selectOption("POST /operators/:id");
  await page.getByRole("button", { name: "查询", exact: true }).click();
  await page.waitForURL(
    (url) => url.searchParams.get("endpoint") === "POST /operators/:id",
  );
  const auditRows = auditTable.locator("tbody tr");
  await auditRows.first().getByText("更新运营账号").waitFor();
  for (const row of await auditRows.all()) {
    assert.equal(await row.locator("td").nth(1).innerText(), username);
    assert.equal(await row.locator("td").nth(2).innerText(), "管理后台");
  }
  await page.getByLabel("平台", { exact: true }).selectOption("miniapp");
  await page.getByRole("button", { name: "查询", exact: true }).click();
  await auditTable.getByText("没有符合条件的审计记录").waitFor();
  await page.getByRole("link", { name: "重置", exact: true }).click();
  await marketingEdits.first().waitFor();
  assert.equal(await endpointFilter.inputValue(), "");
  await screenshot("audit-business-columns", auditTable);
  const auditViewport = page.viewportSize();
  await page.setViewportSize({ width: 390, height: 844 });
  await expectNoOverflow();
  await page.setViewportSize(auditViewport);
  await page.getByLabel("开始时间（北京时间）").fill("2099-01-01T00:00");
  await page.getByRole("button", { name: "查询", exact: true }).click();
  await auditTable.getByText("没有符合条件的审计记录").waitFor();
  await page.getByLabel("结束时间（北京时间）").fill("2098-01-01T00:00");
  await page.getByRole("button", { name: "查询", exact: true }).click();
  await page
    .getByRole("alert")
    .filter({ hasText: "结束时间不能早于开始时间" })
    .waitFor();
  await page.getByRole("link", { name: "重置", exact: true }).click();
  await marketingEdits.first().waitFor();
  await page.getByRole("link", { name: "下一页" }).click();
  await page.waitForURL(
    (url) => url.pathname === "/audits" && url.searchParams.get("page") === "2",
  );
  await expectNoOverflow();
  await visit("反馈与咨询");
  await page.goBack();
  await page.getByRole("heading", { name: "操作审计", exact: true }).waitFor();
  await page.goForward();
  await page
    .getByRole("heading", { name: "反馈与咨询", exact: true })
    .waitFor();
  const history = await page.evaluate(() =>
    sessionStorage.getItem("htmx-history-cache"),
  );
  assert.ok(history === null || history === "[]");

  await page.getByRole("link", { name: "查看 / 回复" }).click();
  const reply = page.getByRole("dialog", { name: "回复反馈" });
  const replyRoute = "**/feedback/*/reply?*";
  await page.route(replyRoute, (route) =>
    route.request().method() === "POST"
      ? route.abort("failed")
      : route.continue(),
  );
  await reply.getByLabel("回复内容").fill("断网时保留这段文字");
  await reply.getByRole("button", { name: "确定" }).click();
  await reply.getByText("网络连接失败，请检查网络后重试").waitFor();
  assert.equal(
    await reply.getByLabel("回复内容").inputValue(),
    "断网时保留这段文字",
  );
  await page.unroute(replyRoute);

  await reply.getByLabel("回复内容").fill("   ");
  await reply.getByRole("button", { name: "确定" }).click();
  await reply.getByText("回复内容不能为空").waitFor();
  await reply.getByLabel("回复内容").fill("已核实，验收回复。");
  await reply.getByRole("button", { name: "确定" }).click();
  await page.getByText("已回复，分发员可在小程序查看").waitFor();
  await page.getByRole("cell", { name: "已回复" }).waitFor();

  await visit("发布链接审核", reviews);
  const assignment = page
    .getByRole("row")
    .filter({ hasText: "双平台待审核任务" });
  await assignment.getByRole("button", { name: "驳回", exact: true }).click();
  const rejection = page.getByRole("dialog", { name: "驳回发布链接" });
  await rejection.getByRole("checkbox", { name: "抖音" }).uncheck();
  await rejection.getByRole("checkbox", { name: "小红书" }).uncheck();
  await rejection.getByRole("button", { name: "确认驳回" }).click();
  await rejection.getByText("请选择发布平台", { exact: true }).waitFor();
  await rejection.getByRole("checkbox", { name: "抖音" }).check();
  await rejection.getByRole("checkbox", { name: "小红书" }).check();
  await rejection.getByRole("button", { name: "确认驳回" }).click();
  await rejection.getByText(/请填写驳回原因/).waitFor();
  await rejection.getByLabel("驳回原因").fill("两端作品缺少指定话题");
  const reviewPost = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      /\/reviews\/[^/]+\/reject$/.test(new URL(response.url()).pathname),
  );
  await rejection.getByRole("button", { name: "确认驳回" }).click();
  const reviewed = await reviewPost;
  assert.equal(reviewed.status(), 200);
  const reviewedForm = new URLSearchParams(reviewed.request().postData());
  assert.deepEqual(reviewedForm.getAll("platforms").sort(), ["dy:1", "xhs:1"]);
  assert.equal(reviewedForm.get("reason"), "两端作品缺少指定话题");
  await rejection.waitFor({ state: "hidden" });

  {
    await visit("分发结算", "营销分发 · 分发结算");
    const settlementPage = page.locator(".settlement-page");
    assert.equal(
      await settlementPage.getByText(/每周一结算上周的任务/).count(),
      0,
    );
    assert.equal(
      await settlementPage.getByText(/周一结算前核对链接可见性/).count(),
      0,
    );
    assert.equal(
      await settlementPage.getByText("营销单价", { exact: true }).count(),
      0,
    );
    const monthPicker = page.getByRole("button", { name: /选择结算月份/ });
    const monthPanel = page.getByRole("dialog", { name: "选择结算月份" });
    await monthPicker.press("Enter");
    await monthPanel.getByText("2026 年", { exact: true }).waitFor();
    assert.equal(
      await monthPanel
        .getByRole("button", { name: "12 月", exact: true })
        .isEnabled(),
      false,
    );
    await monthPanel.getByRole("button", { name: "上一年" }).click();
    await monthPanel.getByText("2025 年", { exact: true }).waitFor();
    await monthPanel.getByRole("button", { name: "下一年" }).click();
    await monthPanel.getByText("2026 年", { exact: true }).waitFor();
    await screenshot("settlement-month-picker");
    await page.keyboard.press("Escape");
    await monthPanel.waitFor({ state: "hidden" });
    assert.ok(
      await monthPicker.evaluate((button) => document.activeElement === button),
    );
    await monthPicker.click();
    await monthPanel.getByRole("button", { name: "9 月", exact: true }).click();
    await monthPanel.waitFor({ state: "hidden" });
    await page.getByText("9 月已入账", { exact: true }).waitFor();
    assert.match(await monthPicker.innerText(), /2026 年 9 月/);
    await page.getByRole("button", { name: "2026 年 10 月" }).click();
    const emptyWeek = page.getByText("这周没有要结算的").first();
    await emptyWeek.waitFor();
    await page.getByRole("button", { name: "2026 年 11 月" }).click();
    await page.getByText("¥20.50", { exact: true }).waitFor();
    const week = page
      .locator("section")
      .filter({
        has: page.getByRole("heading", { name: /11\/16.*11\/22/ }),
      })
      .last();
    await week.getByText("1 条等截图，晚交顺延", { exact: true }).waitFor();
    await page.getByRole("button", { name: "刷新", exact: true }).click();
    await screenshot("settlement-overview");
    await week
      .getByRole("button", { name: "结算验收分发员", exact: true })
      .click();
    const person = page.getByRole("dialog", { name: "分发员详情" });
    await person.getByText("微信昵称", { exact: true }).waitFor();
    await screenshot("settlement-distributor", person);
    await person.getByRole("button", { name: "关闭" }).click();

    await week.getByRole("button", { name: "看明细" }).click();
    const detail = page.getByRole("dialog", { name: /结算明细/ });
    await detail.getByRole("cell", { name: "结算验收任务" }).first().waitFor();
    await detail.getByText("截图没交", { exact: true }).waitFor();
    const workLink = detail.getByRole("link", { name: /v\.douyin\.com/ });
    assert.equal(await workLink.getAttribute("target"), "_blank");
    await page
      .context()
      .route("https://v.douyin.com/test-work/", async (route) =>
        route.fulfill({ body: "Local browser QA for the external work link" }),
      );
    const popupPromise = page.waitForEvent("popup");
    await workLink.click();
    const workPopup = await popupPromise;
    await workPopup.waitForURL("https://v.douyin.com/test-work/");
    await workPopup.close();
    await page
      .context()
      .grantPermissions(["clipboard-read", "clipboard-write"]);
    await detail.getByRole("button", { name: "复制" }).first().click();
    assert.equal(
      await page.evaluate(() => navigator.clipboard.readText()),
      "https://v.douyin.com/test-work/",
    );
    await screenshot("settlement-details", detail);
    await detail.getByRole("button", { name: "看截图" }).click();
    const playback = page.getByRole("dialog", { name: "抖音播放量截图" });
    const playbackImage = playback.getByRole("img", { name: "抖音播放量截图" });
    await playbackImage.waitFor();
    assert.equal(
      await playbackImage.evaluate((image) => image.naturalWidth),
      1,
    );
    await verifyWatermarkStyle(
      playback.getByText(`管理员 · ${username}`, { exact: true }),
      6,
    );
    await screenshot("settlement-playback", playback);
    await playback.getByRole("button", { name: "放大截图" }).click();
    await playback.getByRole("button", { name: "缩小截图" }).click();
    await playback.getByRole("button", { name: "关闭截图" }).click();
    await playback.waitFor({ state: "hidden" });
    await detail.getByText("和上面金额一致", { exact: false }).waitFor();
    await detail.getByRole("button", { name: "关闭", exact: true }).click();
    await week.getByRole("button", { name: /核对无误，确认入账/ }).click();
    const settlement = page.getByRole("dialog", { name: "确认入账" });
    await settlement.getByText(/1 条平台记录/).waitFor();
    await screenshot("settlement-confirm", settlement);
    await settlement.getByRole("button", { name: "取消" }).click();
    await settlement.waitFor({ state: "hidden" });
    await week.getByRole("button", { name: /核对无误，确认入账/ }).click();
    const settlementPost = page.waitForResponse(
      (response) =>
        response.request().method() === "POST" &&
        new URL(response.url()).pathname === "/settlements",
    );
    await settlement
      .getByRole("button", { name: "确认入账", exact: true })
      .click();
    const credited = await settlementPost;
    assert.equal(credited.status(), 200);
    assert.ok(credited.request().headers()["idempotency-key"]);
    await settlement.waitFor({ state: "hidden" });
    await week.getByText("已入账", { exact: true }).first().waitFor();
    const download = page.waitForEvent("download");
    await week.getByRole("link", { name: "导出入账明细（Excel）" }).click();
    const workbook = await download;
    assert.match(workbook.suggestedFilename(), /\.xlsx$/);
    const workbookPath = path.join(screenshotDirectory, "settlement-qc.xlsx");
    await workbook.saveAs(workbookPath);
    assert.equal(
      (await fs.readFile(workbookPath)).subarray(0, 2).toString(),
      "PK",
    );
    await screenshot("settlement-posted");

    await page
      .locator(".ant-message-notice")
      .last()
      .waitFor({ state: "hidden" });
    await page.setViewportSize({ width: 390, height: 844 });
    await monthPicker.click();
    await monthPanel.waitFor();
    await page.waitForFunction(() =>
      Boolean(document.activeElement?.closest(".month-picker-panel")),
    );
    await expectNoOverflow();
    const monthBounds = await monthPanel.boundingBox();
    assert.ok(
      monthBounds &&
        monthBounds.x >= 0 &&
        monthBounds.x + monthBounds.width <= 390 &&
        monthBounds.y >= 0 &&
        monthBounds.y + monthBounds.height <= 844,
    );
    await screenshot("settlement-month-picker-mobile");
    await page.getByRole("heading", { name: "营销分发 · 分发结算" }).click();
    await monthPanel.waitFor({ state: "hidden" });
    const tableScroll = week.locator(".settlement-scroll");
    assert.ok(
      await tableScroll.evaluate(
        (table) => table.scrollWidth > table.clientWidth,
      ),
    );
    assert.equal(
      await page.evaluate(() => document.documentElement.scrollWidth),
      390,
    );
    await screenshot("settlement-mobile");
    await week.getByRole("button", { name: "看明细" }).click();
    await detail.getByRole("button", { name: "关闭", exact: true }).waitFor();
    await screenshot("settlement-mobile-details", detail);
    await detail.getByRole("button", { name: "关闭", exact: true }).click();
    await page.setViewportSize({ width: 1440, height: 1000 });

    await page.getByRole("link", { name: "看提现记录 ›" }).click();
    await page.waitForURL("**/withdrawals?month=2026-11");
    await page
      .getByRole("heading", { name: "提现记录", exact: true })
      .waitFor();
    await page.getByText("当前月份 2026-11", { exact: false }).waitFor();
    await page.getByRole("cell", { name: "¥20.50" }).waitFor();
    await page.getByRole("link", { name: "刷新结果" }).click();
    await page.locator("#withdrawal-results").waitFor();
  }
  await visit("余额流水");
  await page.getByRole("cell", { name: "结算入账" }).waitFor();
  await page.getByRole("button", { name: "关联调整", exact: true }).click();
  const adjustment = page.getByRole("dialog", { name: "创建关联调整" });
  await adjustment.getByLabel(/调整金额/).fill("1.25");
  await adjustment.getByLabel("调整原因").fill("验收调整");
  await adjustment.getByRole("button", { name: "确定" }).click();
  await page.getByText("已创建关联调整流水").waitFor();
  await page.getByRole("cell", { name: "验收调整" }).waitFor();

  const assignTitle = "指派撤回验收任务";
  await verifyDirectUpload(page, {
    uploadID: file,
    transferStatus: 503,
    saveTitle: assignTitle,
  });
  await verifyVideoWatermark();
  const poolRow = page
    .locator(".marketing-content-row")
    .filter({ hasText: assignTitle });
  await poolRow.getByRole("button", { name: "修改", exact: true }).click();
  const editContent = page.getByRole("dialog", { name: "修改营销内容" });
  assert.equal(
    await editContent.getByLabel("任务标题").inputValue(),
    assignTitle,
  );
  await editContent.getByLabel("分发语").fill("浏览器验收的新版本分发语");
  await editContent.getByRole("button", { name: "保存修改" }).click();
  await page.getByText("已保存修改", { exact: true }).waitFor();
  await poolRow.getByRole("button", { name: "详情", exact: true }).click();
  const detailContent = page.getByRole("dialog", { name: /指派撤回验收任务/ });
  await detailContent.getByText("浏览器验收的新版本分发语").waitFor();
  assert.deepEqual(
    await detailContent.locator("footer button").allTextContents(),
    ["关闭"],
  );
  await detailContent
    .getByRole("button", { name: "关闭", exact: true })
    .click();
  await poolRow.getByRole("button", { name: "手动指派" }).click();
  const dispatch = page.getByRole("dialog", {
    name: `手动指派 · ${assignTitle}`,
  });
  await dispatch
    .getByRole("combobox")
    .selectOption({ label: "指派验收分发员" });
  const dispatchForm = dispatch.locator("form");
  assert.equal(
    await dispatchForm.evaluate((form) => form.checkValidity()),
    true,
  );
  const assignmentRequest = page.waitForRequest(
    (request) =>
      request.method() === "POST" &&
      /\/tasks\/[^/]+\/assign$/.test(new URL(request.url()).pathname),
    { timeout: 8000 },
  );
  await Promise.all([
    assignmentRequest,
    dispatch.getByRole("button", { name: "确认指派" }).click(),
  ]);
  await page.getByText("已指派，占用分发员当天的一条名额").waitFor();

  await visit("发布链接审核", reviews);
  const creditedTask = page.getByRole("button", {
    name: "结算验收任务",
    exact: true,
  });
  await creditedTask.click();
  const creditedDrawer = page.getByRole("dialog", { name: "结算验收任务" });
  await creditedDrawer.getByRole("button", { name: "查看播放量截图" }).click();
  const reviewedScreenshot = page.getByRole("dialog", {
    name: "抖音播放量截图",
  });
  await reviewedScreenshot
    .getByText(`管理员 · ${username}`, { exact: true })
    .first()
    .waitFor();
  await reviewedScreenshot.getByRole("button", { name: "关闭截图" }).click();
  await reviewedScreenshot.waitFor({ state: "hidden" });
  await creditedDrawer.locator(".ant-drawer-close").click();
  await creditedDrawer.waitFor({ state: "hidden" });
  await page.getByRole("button", { name: assignTitle, exact: true }).click();
  await page.getByRole("button", { name: "撤回指派", exact: true }).click();
  const revoke = page.getByRole("dialog", { name: "撤回这次指派？" });
  await revoke.getByLabel("撤回原因").fill("验收撤回并释放名额");
  await revoke.getByRole("button", { name: "确定" }).click();
  await page.getByText("已撤回，内容与当天名额已释放").waitFor();
  await revoke.waitFor({ state: "hidden" });
  const revokedAssignment = page.getByRole("dialog", { name: assignTitle });
  await revokedAssignment.locator(".ant-drawer-close").click();
  await revokedAssignment.waitFor({ state: "hidden" });

  const reviewStatus = page.getByLabel("状态", { exact: true });
  await page.getByLabel("发布平台", { exact: true }).selectOption("xhs");
  await page.waitForURL((url) => url.searchParams.get("platform") === "xhs");
  await page.reload();
  assert.equal(
    await page.getByLabel("发布平台", { exact: true }).inputValue(),
    "xhs",
  );
  const beforeStatus = await reviewStatus.elementHandle();
  await reviewStatus.selectOption("pending_review");
  await page.waitForURL(
    (url) => url.searchParams.get("status") === "pending_review",
  );
  await beforeStatus.waitForElementState("hidden");
  await page
    .locator("#admin-workspace.htmx-settling")
    .waitFor({ state: "hidden" });
  assert.equal(await reviewStatus.inputValue(), "pending_review");
  const refreshReviews = page.waitForResponse((response) => {
    const url = new URL(response.url());
    return (
      response.request().method() === "GET" &&
      url.pathname === "/reviews" &&
      url.searchParams.get("platform") === "xhs" &&
      url.searchParams.get("status") === "pending_review"
    );
  });
  await page.getByRole("button", { name: "刷新", exact: true }).click();
  assert.equal((await refreshReviews).status(), 200);
  await page.goto(new URL("/reviews?platform=xhs", base).href);
  const approvalRow = page
    .getByRole("row")
    .filter({ hasText: "双平台通过验收任务" });
  await approvalRow.waitFor();
  const links = await approvalRow
    .locator(".publish-review-link a")
    .evaluateAll((anchors) => anchors.map((anchor) => anchor.href));
  assert.ok(links.some((url) => new URL(url).hostname === "v.douyin.com"));
  assert.ok(links.some((url) => new URL(url).hostname === "xhslink.cn"));
  const approve = approvalRow.getByRole("button", {
    name: "通过",
    exact: true,
  });
  const approval = page.getByRole("dialog", { name: "通过发布链接" });
  await approve.click();
  await approval.getByText(/抖音、小红书|小红书、抖音/).waitFor();
  assert.equal(await approval.getByRole("combobox").count(), 0);
  assert.equal(await approval.getByRole("textbox").count(), 0);
  await screenshot("review-approval", approval);
  await approval.getByRole("button", { name: "取消" }).click();
  await approval.waitFor({ state: "hidden" });
  await approvalRow.getByText("待审核", { exact: true }).waitFor();
  const approvalTitle = "双平台通过验收任务";
  await approvalRow.getByRole("button", { name: approvalTitle }).click();
  const approvalDrawer = page.getByRole("dialog", { name: approvalTitle });
  await approvalDrawer
    .getByRole("button", { name: "通过", exact: true })
    .click();
  await approval.getByRole("button", { name: "确认通过" }).click();
  await page.getByText("审核已通过", { exact: true }).waitFor();
  await approval.waitFor({ state: "hidden" });
  await approvalDrawer.locator(".ant-drawer-close").click();
  await approvalDrawer.waitFor({ state: "hidden" });
  await approvalRow.getByText("待结算", { exact: true }).waitFor();

  await visit("任务管理", tasks);
  await poolRow.getByRole("button", { name: "手动指派" }).waitFor();

  await visit("分发员管理", "营销分发 · 分发员管理");
  const member = page.getByRole("row").filter({ hasText: "指派验收分发员" });
  const searchMember = page.getByRole("searchbox", { name: "搜索分发员" });
  await searchMember.fill("指派验收分发员");
  await page.waitForURL(
    (url) => url.searchParams.get("q") === "指派验收分发员",
  );
  assert.equal(await page.locator(".distributor-table tbody tr").count(), 1);
  await page.getByRole("button", { name: "清除", exact: true }).click();
  await page.waitForURL((url) => !url.searchParams.get("q"));
  await member.waitFor();
  await member
    .getByRole("button", { name: "指派验收分发员", exact: true })
    .click();
  await page
    .getByRole("dialog", { name: "分发员详情" })
    .getByRole("button", { name: "拉黑", exact: true })
    .click();
  const block = page.getByRole("dialog", { name: "拉黑 指派验收分发员" });
  await block.getByRole("button", { name: "确认拉黑" }).click();
  await block.getByText("请填写拉黑原因").waitFor();
  await block.getByLabel("拉黑原因").fill("验收暂停领取资格");
  await block.getByRole("button", { name: "确认拉黑" }).click();
  await block.waitFor({ state: "hidden" });
  await member.getByText("已拉黑", { exact: true }).waitFor();
  await member
    .getByRole("button", { name: "指派验收分发员", exact: true })
    .click();
  await page
    .getByRole("dialog", { name: "分发员详情" })
    .getByRole("button", { name: "取消拉黑", exact: true })
    .click();
  const unblock = page.getByRole("dialog", { name: "取消拉黑 指派验收分发员" });
  await unblock.getByRole("button", { name: "确认取消拉黑" }).click();
  await unblock.waitFor({ state: "hidden" });
  await member.getByText("正常", { exact: true }).waitFor();

  await visit("分发协议");
  await page.getByRole("button", { name: "发布新协议", exact: true }).click();
  const agreement = page.getByRole("dialog", { name: "发布新协议" });
  await agreement.getByLabel("协议标题").fill("新版验收协议");
  const editor = agreement.locator(".ql-editor");
  await editor.focus();
  await editor.evaluate((element) => {
    const clipboardData = new DataTransfer();
    clipboardData.setData(
      "text/html",
      '<h2>新版协议测试正文</h2><ol><li><strong style="color:red">正文</strong><u>重点</u></li></ol><img src="image.png">',
    );
    element.dispatchEvent(
      new ClipboardEvent("paste", {
        clipboardData,
        bubbles: true,
        cancelable: true,
      }),
    );
  });
  await agreement.getByText("预览小程序排版", { exact: true }).click();
  const agreementPreview = agreement.locator(
    ".agreement-preview .agreement-content",
  );
  await agreementPreview
    .getByRole("heading", { name: "新版协议测试正文" })
    .waitFor();
  assert.equal(
    await agreementPreview.locator("ol > li > strong").textContent(),
    "正文",
  );
  assert.equal(
    await agreementPreview.locator("ol > li > u").textContent(),
    "重点",
  );
  assert.equal(await agreementPreview.locator("[style], img").count(), 0);
  await agreement.getByRole("button", { name: "确定" }).click();
  await page.getByText("新协议已发布").waitFor();
  await page.getByRole("cell", { name: "新版验收协议" }).waitFor();

  await visit("提现记录");
  const refresh = page.waitForResponse(
    (response) =>
      response.request().headers()["hx-target"] === "withdrawal-results",
  );
  await page.getByRole("link", { name: "刷新结果" }).click();
  assert.equal((await refresh).status(), 200);
  await page.getByRole("columnheader", { name: "处理状态" }).waitFor();

  for (const width of [390, 320]) {
    await page.setViewportSize({ width, height: 780 });
    assert.equal(await page.locator(".sidebar").isVisible(), false);
    for (const [menu, title] of destinations) {
      await visit(menu, title, true);
      await expectNoOverflow();
    }
    await visit("任务管理", tasks, true);
    await screenshot(`tasks-${width}`);
    await page
      .getByRole("button", { name: "上传营销内容", exact: true })
      .click();
    const upload = page.getByRole("dialog");
    await upload.getByLabel("任务标题", { exact: true }).waitFor();
    const footer = upload.locator(".ant-modal-footer");
    assert.ok(
      await footer.evaluate(
        (element) => element.getBoundingClientRect().bottom <= innerHeight,
      ),
    );
    await screenshot(`upload-${width}`, upload);
    await upload.getByRole("button", { name: "取消", exact: true }).click();
    await upload.waitFor({ state: "hidden" });
    await visit("反馈与咨询", "反馈与咨询", true);
    await page.getByRole("link", { name: "查看 / 回复" }).click();
    await reply.waitFor();
    await expectNoOverflow();
    assert.ok(
      await reply
        .locator("footer")
        .evaluate(
          (element) => element.getBoundingClientRect().bottom <= innerHeight,
        ),
    );
    await screenshot(`feedback-${width}`);
    await page.keyboard.press("Escape");
    await reply.waitFor({ state: "hidden" });
  }

  await visit("营销设置", "营销分发 · 营销设置", true);
  await page.setViewportSize({ width: 1440, height: 1000 });
  const scheduleForm = page.locator("[data-distribution-settings]");
  const frequency = scheduleForm.getByLabel("结算方式");
  const monthDay = scheduleForm.locator('[name="settlement_month_day"]');
  const wait = scheduleForm.locator('[name="settlement_delay_minutes"]');
  const clock = scheduleForm.locator('[name="settlement_clock"]');
  const cutoff = scheduleForm.locator('[name="playback_cutoff_clock"]');
  const deferral = scheduleForm.locator('[name="playback_deferral_periods"]');
  assert.equal(await frequency.inputValue(), "weekly");
  await frequency.selectOption("daily");
  await clock.waitFor();
  assert.equal(await monthDay.isVisible(), false);
  await frequency.selectOption("after_delay");
  await wait.waitFor();
  assert.equal(await cutoff.isVisible(), false);
  await frequency.selectOption("monthly");
  await monthDay.fill("31");
  await clock.fill("16:30");
  await cutoff.fill("15:00");
  await deferral.fill("2");
  await scheduleForm.getByRole("button", { name: "保存分发设置" }).click();
  await page
    .getByRole("status")
    .filter({ hasText: "分发设置已保存" })
    .waitFor();
  await page.reload();
  assert.equal(await frequency.inputValue(), "monthly");
  assert.equal(await monthDay.inputValue(), "31");
  assert.equal(await clock.inputValue(), "16:30");
  assert.equal(await cutoff.inputValue(), "15:00");
  assert.equal(await deferral.inputValue(), "2");
  await screenshot("distribution-settings");
  await page.setViewportSize({ width: 320, height: 780 });
  await expectNoOverflow();
  await screenshot("distribution-settings-mobile");

  await visit("反馈与咨询", "反馈与咨询", true);
  await page.getByRole("button", { name: "打开导航菜单" }).click();
  await page.getByRole("button", { name: "退出", exact: true }).click();
  await page.getByRole("heading", { name: "欢迎回来" }).waitFor();
  assert.equal(
    await page
      .context()
      .cookies()
      .then((cookies) => cookies.some(({ name }) => name === "btyx_admin_web")),
    false,
  );
  await screenshot("login-320");
  await page.goBack().catch(() => {});
  await page.getByRole("heading", { name: "欢迎回来" }).waitFor();

  await login(reviewer, password);
  await page.getByRole("heading", { name: "反馈与咨询" }).waitFor();
  await visit("任务管理", tasks, true);
  await page.getByRole("heading", { name: tasks, exact: true }).waitFor();
  await page.locator(".marketing-content-row").first().waitFor();
  assert.equal(
    await page.getByRole("button", { name: "上传营销内容" }).count(),
    0,
  );
  assert.equal(
    await page.getByRole("button", { name: "手动指派", exact: true }).count(),
    0,
  );
  assert.equal(
    await page.getByRole("button", { name: "保存失效规则" }).count(),
    0,
  );
  await visit("反馈与咨询", "反馈与咨询", true);
  await page.getByRole("heading", { name: "反馈与咨询" }).waitFor();
  assert.equal(
    await page.locator('.server-menu a[href="/operators"]').count(),
    0,
  );
  await page.getByRole("link", { name: "查看", exact: true }).click();
  await page.getByRole("dialog", { name: "反馈详情" }).waitFor();
  assert.equal(await page.getByLabel("回复内容").count(), 0);
  await page.keyboard.press("Escape");
  await page.getByRole("button", { name: "打开导航菜单" }).click();
  assert.equal(await page.locator('.server-menu a[href="/audits"]').count(), 0);
  const unauthorized = await page.goto(`${base}/audits`);
  assert.equal(unauthorized.status(), 403);
  await page.getByRole("heading", { name: "无权读取此页面的数据" }).waitFor();
  for (const endpoint of ["weeks?month=2026-11", "export?through=2026-11-23"]) {
    const result = await page.request.get(
      `${base}/api/v1/settlements/${endpoint}`,
    );
    assert.equal(result.status(), 403);
  }

  assert.equal(
    (await page.goto(`${base}/distribution/settings`)).status(),
    403,
  );
  assert.equal(
    (await page.request.get(`${base}/api/v1/distribution/settings`)).status(),
    403,
  );

  await page.goto(base + "/feedback");
  await page.getByRole("button", { name: "打开导航菜单" }).click();
  await page
    .getByRole("dialog", { name: "导航菜单" })
    .getByRole("link", { name: reviewer, exact: true })
    .click();
  await page.getByRole("heading", { name: "个人信息", exact: true }).waitFor();
  await expectNoOverflow();
  await page.goto(base + "/feedback");
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page
    .locator(".sidebar")
    .getByRole("link", { name: reviewer, exact: true })
    .click();
  await page.waitForURL("**/profile");
  await page.getByRole("heading", { name: "重置密码" }).waitFor();
  const newPassword = password + "-updated";
  async function changePassword(current, confirmation) {
    await page.getByLabel("当前密码").fill(current);
    await page.getByLabel("新密码", { exact: true }).fill(newPassword);
    await page.getByLabel("确认新密码").fill(confirmation);
    await page.getByRole("button", { name: "确认重置密码" }).click();
  }
  await changePassword(password, newPassword + "x");
  await page
    .getByRole("alert")
    .filter({ hasText: "两次输入的新密码不一致" })
    .waitFor();
  await changePassword(password, newPassword);
  await page.getByRole("status").filter({ hasText: "密码已更新" }).waitFor();
  await login(reviewer, newPassword);
  await page
    .locator(".sidebar")
    .getByRole("link", { name: reviewer, exact: true })
    .click();
  await page.getByRole("heading", { name: "个人信息", exact: true }).waitFor();
  await page.getByRole("heading", { name: "重置密码" }).waitFor();
  assert.deepEqual(browserErrors, []);
  console.log(
    "Edge QC passed: HTMX navigation and forms, business mutations, upload protocol, permissions, browser history, desktop and mobile layouts.",
  );
} catch (error) {
  await screenshot("failure").catch(() => {});
  console.error(
    `Browser verification failed at ${new URL(page.url()).pathname}`,
    browserErrors,
  );
  throw error;
} finally {
  await browser.close();
}
