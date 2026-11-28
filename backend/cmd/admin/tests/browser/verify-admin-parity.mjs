import assert from "node:assert/strict";
import fs from "node:fs/promises";
import path from "node:path";
import { chromium } from "playwright-core";

const baseline = process.env.BTYX_BROWSER_BASELINE;
const migrated = process.env.BTYX_BROWSER_URL;
const username = process.env.BTYX_BROWSER_USER;
const password = process.env.BTYX_BROWSER_PASSWORD;
assert.ok(baseline && migrated && username && password);

const screens = [
  ["tasks", "/tasks", "营销分发 · 任务管理", "内容预览"],
  ["enrollments", "/users", "营销分发 · 分发员报名审核", "人工复核"],
  ["distributors", "/users/manage", "营销分发 · 分发员管理", "抖音号"],
  ["reviews", "/reviews", "营销分发 · 发布链接审核", "链接"],
  [
    "settlement",
    "/settlements?month=2026-11",
    "营销分发 · 分发结算",
    "按平台算",
  ],
  ["identity", "/admin/realname", "报名审核 · 实名认证记录", "身份证照片"],
  ["ledger", "/ledger", "余额流水", "可用余额变动"],
  ["agreements", "/agreements", "分发协议", "协议标题"],
  ["operators", "/operators", "运营账号", "页面权限"],
  ["contents", "/contents", "素材库", "素材内容"],
  ["withdrawals", "/withdrawals", "提现记录", "申请金额"],
  ["audits", "/audits", "操作审计", "原因 / 备注"],
  ["feedback", "/feedback", "反馈与咨询", "反馈内容"],
];
const directory = path.resolve(
  import.meta.dirname,
  "../../../../../artifacts/screenshots/admin-parity",
  path.basename(process.env.BTYX_ADMIN_BASELINE_DIST || "baseline"),
);
await fs.mkdir(directory, { recursive: true });
const browser = await chromium.launch({ channel: "msedge", headless: true });
const report = [];
const requestedScreens = process.env.BTYX_PARITY_SCREENS?.split(",");
const comparisonPath = path.join(
  directory,
  requestedScreens ? "comparison-focused.json" : "comparison.json",
);

async function loggedIn(base) {
  const context = await browser.newContext({
    locale: "zh-CN",
    timezoneId: "Asia/Shanghai",
    reducedMotion: "reduce",
  });
  const page = await context.newPage();
  await page.goto(`${base}/login`);
  await page.getByLabel("用户名").fill(username);
  await page.getByLabel("密码", { exact: true }).fill(password);
  await page.getByRole("button", { name: "进入运营后台" }).click();
  await page.getByRole("heading", { name: "营销分发 · 任务管理" }).waitFor();
  return page;
}

async function capture(page, base, screen, width, version) {
  const [name, route, heading, column] = screen;
  await page.setViewportSize({ width, height: width < 500 ? 780 : 1000 });
  await page.mouse.move(0, 0);
  const response = await page.goto(base + route);
  assert.equal(response.status(), 200);
  await page.getByRole("heading", { name: heading, exact: true }).waitFor();
  await page
    .getByRole("columnheader", { name: column, exact: true })
    .first()
    .waitFor();
  const container = page.locator("#workspace-content");
  const text = await container.evaluate((node, includeLayout) => {
    const normalize = (value) => value.replace(/\s+/g, " ").trim();
    const layout = {};
    if (includeLayout) {
      const selectors = {
        heading: ".page-heading",
        title: ".page-heading h2",
        description: ".page-heading p, .page-heading .ant-typography-secondary",
        headingButton: ".page-heading a[role=button], .page-heading button",
        card: ".ant-card",
        toolbar: "form.table-toolbar, .table-toolbar, .marketing-filters",
        table: "table",
        tableFrame: ".ant-table-container, .server-table-frame",
        header: "thead th",
        firstRow: "tbody tr:not(.ant-table-measure-row) > td",
        tableIcon: "tbody tr:not(.ant-table-measure-row) svg",
        tableAction: 'tbody :is(a, button, [role="button"])',
        cardBody: ".ant-card-body",
        tag: "tbody .ant-tag, tbody .server-tag",
        empty:
          ".ant-empty, .ant-empty-image, .ant-empty-description, .server-table-placeholder, .server-table-placeholder > *",
        footer: ".server-pager, .ant-pagination",
        footerItem: ".server-pager > *, .ant-pagination > li",
        footerIcon: ".server-pager svg, .ant-pagination svg",
        settlement:
          ".month-picker-trigger, .month-picker-chevron, .settlement-note, .settlement-kpis small a, .settlement-week-actions :is(button, a), .settlement-detail-trigger",
        settlementParts:
          ".settlement-playback, .settlement-week h3 .settlement-note, .settlement-projects, .settlement-check",
        filterIcon:
          ".enrollment-filters svg, .distributor-toolbar svg, .review-filters svg",
        distributorCells:
          ".distributor-table tbody tr:not(.ant-table-measure-row) > :is(td:first-child, td:nth-child(5), td:nth-child(6))",
        distributorSearch:
          ".ant-input-search, .ant-input-affix-wrapper, .ant-input-search .ant-btn, .distributor-search, .distributor-search-field, .distributor-search-button",
        marketingStructure:
          ".marketing-section-heading, .marketing-section-heading h4, .marketing-settings-panel, .marketing-toolbar, .marketing-progress, .marketing-content-row .marketing-row-actions",
        control:
          ".marketing-settings-panel :is(input:not([type=hidden]), button, a, select), .enrollment-tag-add > :not([type=hidden]), .enrollment-filters > :not([type=hidden])",
      };
      for (const [name, selector] of Object.entries(selectors)) {
        layout[name] = [...node.querySelectorAll(selector)]
          .slice(0, 12)
          .map((element) => {
            const box = element.getBoundingClientRect();
            const style = getComputedStyle(element);
            const range = document.createRange();
            range.selectNodeContents(element);
            const contents = range.getBoundingClientRect();
            const bounds = ({ x, y, width, height }) =>
              [x, y, width, height].map(
                (value) => Math.round(value * 100) / 100,
              );
            const textParts = [];
            if (name === "settlementParts") {
              const walker = document.createTreeWalker(
                element,
                NodeFilter.SHOW_TEXT,
              );
              for (
                let part = walker.nextNode();
                part;
                part = walker.nextNode()
              ) {
                if (!part.nodeValue) continue;
                range.selectNodeContents(part);
                textParts.push([
                  part.nodeValue,
                  bounds(range.getBoundingClientRect()),
                ]);
              }
            }
            let intrinsicWidths;
            if (name === "distributorCells") {
              const probe = document.createElement("div");
              probe.style.cssText =
                "position:absolute;visibility:hidden;width:min-content";
              probe.replaceChildren(
                ...Array.from(element.childNodes, (child) =>
                  child.cloneNode(true),
                ),
              );
              element.append(probe);
              const min = probe.getBoundingClientRect().width;
              probe.style.width = "max-content";
              const max = probe.getBoundingClientRect().width;
              probe.remove();
              intrinsicWidths = [min, max];
            }
            return {
              text: normalize(
                element.innerText || element.textContent || "",
              ).slice(0, 80),
              bounds: bounds(box),
              contents: bounds(contents),
              intrinsicWidths,
              textParts: textParts.length ? textParts : undefined,
              color: style.color,
              background: style.backgroundColor,
              font: `${style.fontWeight} ${style.fontSize}/${style.lineHeight}`,
              padding: style.padding,
              border: `${style.borderTopWidth} ${style.borderTopColor} / ${style.borderTopLeftRadius}`,
              verticalAlign: style.verticalAlign,
              fontFamily: style.fontFamily,
              scrollShadow:
                name === "tableFrame"
                  ? getComputedStyle(element, "::after").boxShadow
                  : undefined,
              svgColors:
                element instanceof SVGElement
                  ? [...element.querySelectorAll("[fill], [stroke]")].map(
                      (part) => [
                        part.getAttribute("fill"),
                        part.getAttribute("stroke"),
                      ],
                    )
                  : undefined,
            };
          });
      }
    }
    return {
      headers: [...node.querySelectorAll("th")]
        .map((item) => normalize(item.innerText))
        .filter(Boolean),
      actions: [
        ...node.querySelectorAll(
          'button, [role="button"], .identity-records-pagination a[aria-label], .identity-records-pagination [aria-current="page"]',
        ),
      ]
        .filter((item) => item.getClientRects().length)
        .map((item) => {
          const label = normalize(
            item.getAttribute("aria-label") || item.innerText,
          );
          if (item.matches("span[aria-current=page]")) return `页 ${label}`;
          return label.replace(/^第 (\d+) 页$/, "页 $1");
        })
        .filter(Boolean),
      overflow: document.documentElement.scrollWidth > innerWidth,
      layout,
    };
  }, process.env.BTYX_PARITY_LAYOUT === "1");
  const png = await page.screenshot({
    path: path.join(directory, `${name}-${width}-${version}.png`),
    animations: "disabled",
  });
  const pagination =
    name === "identity"
      ? await page
          .getByRole("navigation", { name: "实名认证记录分页" })
          .screenshot({
            path: path.join(
              directory,
              `${name}-${width}-${version}-pagination.png`,
            ),
            animations: "disabled",
          })
      : undefined;
  return { text, png, pagination };
}

try {
  const [old, current] = await Promise.all([
    loggedIn(baseline),
    loggedIn(migrated),
  ]);
  for (const screen of screens) {
    if (requestedScreens && !requestedScreens.includes(screen[0])) continue;
    for (const width of [1440, 390]) {
      const results = await Promise.all([
        capture(old, baseline, screen, width, "old"),
        capture(current, migrated, screen, width, "new"),
      ]);
      report.push({
        screen: screen[0],
        width,
        sameHeaders:
          JSON.stringify(results[0].text.headers) ===
          JSON.stringify(results[1].text.headers),
        sameActions:
          JSON.stringify(results[0].text.actions) ===
          JSON.stringify(results[1].text.actions),
        exactScreenshot: results[0].png.equals(results[1].png),
        exactPagination: results[0].pagination?.equals(results[1].pagination),
        old: results[0].text,
        new: results[1].text,
      });
    }
  }
} finally {
  await fs.writeFile(comparisonPath, JSON.stringify(report, null, 2));
  await browser.close();
}

const differences = report.filter(
  (item) =>
    !item.sameHeaders ||
    !item.sameActions ||
    !item.exactScreenshot ||
    item.exactPagination === false ||
    item.new.overflow,
);
assert.ok(report.length > 0, "没有找到要比较的后台页面");
assert.equal(
  differences.length,
  0,
  `新旧版存在 ${differences.length} 组差异，见 ${comparisonPath}`,
);
