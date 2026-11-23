const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");

test("首页第二步打开领取前详情，只在详情确认后提交并以同一幂等键重试", async () => {
  const prior = { wx: global.wx, Page: global.Page };
  const requests = [];
  const assignmentReads = [];
  const destinations = [];
  const redirects = [];
  const toasts = [];
  let user = { status: "approved", consented: true };
  let hasContent = true;
  let assignments = [];
  let page;
  let pendingClaim;
  global.Page = (definition) => {
    page = {
      ...definition,
      setData(patch) {
        Object.assign(this.data, patch);
      },
    };
  };
  global.wx = {
    getStorageSync: () => "session",
    navigateTo: ({ url }) => destinations.push(url),
    redirectTo: ({ url }) => redirects.push(url),
    showToast: ({ title }) => toasts.push(title),
    request(options) {
      const requestedPath = new URL(options.url).pathname;
      if (options.method === "POST") {
        requests.push(options);
        pendingClaim = options;
        return;
      }

      if (requestedPath === "/api/v1/assignments/allocated") {
        assignmentReads.push(requestedPath);
        options.success({ statusCode: 200, data: { code: 0, data: assignments[0] } });
        return;
      }
      const data = {
        "/api/v1/me": user,
        "/api/v1/tasks/today": { has_content: hasContent, price_fen: "825" },
        "/api/v1/assignments": assignments,
      }[requestedPath];
      assert.ok(data, `unexpected API path: ${requestedPath}`);
      options.success({ statusCode: 200, data: { code: 0, data } });
    },
  };

  try {
    const template = fs.readFileSync(
      path.join(__dirname, "../src/pages/home/index.wxml"),
      "utf8",
    );
    const guideButton = template.match(
      /<view\s+([^<]*\bbindtap="followGuide"[^<]*)>\s*\{\{[^\n]*\bguide\.label\}\}/,
    );
    assert.ok(guideButton);
    assert.doesNotMatch(guideButton[1], /wx:if|wx:elif/);
    assert.doesNotMatch(template, /bindtap="claimToday"/);
    assert.match(template, /bindtap="openTodayTask"/);
    assert.doesNotMatch(template, /openAllTasks/);

    require("../src/pages/home/index");
    const home = page;
    await home.load();
    assert.equal(home.data.hasContent, true);
    assert.equal(home.data.guide.label, "第二步：领今日任务");

    home.followGuide();
    home.openTodayTask();
    const preview = "/subpackages/task/today/index";
    assert.deepEqual(destinations, [preview, preview]);
    assert.equal(requests.length, 0);

    const app = require("../src/app.json");
    const taskPackage = app.subpackages.find(({ root }) => root === "subpackages/task");
    assert.ok(taskPackage.pages.includes("today/index"));
    require("../src/subpackages/task/today/index");
    const detail = page;
    const detailTemplate = fs.readFileSync(
      path.join(__dirname, "../src/subpackages/task/today/index.wxml"),
      "utf8",
    );
    assert.equal((detailTemplate.match(/bindtap="followAction"/g) ?? []).length, 1);

    hasContent = false;
    await detail.onShow();
    assert.equal(detail.data.actionLabel, "暂无可领取任务");
    await detail.followAction();
    assert.equal(requests.length, 0);

    hasContent = true;
    user = { ...user, blocked: true };
    await detail.onShow();
    assert.equal(detail.data.actionLabel, "账号已暂停领取");
    await detail.followAction();
    assert.equal(requests.length, 0);

    user = { ...user, blocked: false };
    await detail.onShow();
    assert.equal(detail.data.price, "8.25");
    assert.equal(detail.data.twoPlatformPrice, "16.50");
    assert.equal(detail.data.actionLabel, "领取今日任务");

    const first = detail.followAction();
    await detail.followAction();
    assert.equal(requests.length, 1);
    pendingClaim.fail();
    await first;
    assert.equal(toasts.length, 1);

    const retry = detail.followAction();
    assert.equal(requests.length, 2);
    const [original, repeated] = requests;
    assert.match(repeated.url, /\/tasks\/claim$/);
    assert.deepEqual(repeated.data, {});
    assert.ok(original.header["Idempotency-Key"]);
    assert.equal(original.header["Idempotency-Key"], repeated.header["Idempotency-Key"]);
    const assignment = {
      id: "allocated",
      steps: [],
      submissions: [],
      platforms: ["dy"],
      date: require("../src/services/marketing-tasks").marketingBusinessDate(),
      price_fen: "825",
      status: "in_progress",
    };
    assignments = [assignment];
    pendingClaim.success({
      statusCode: 200,
      data: {
        code: 0,
        data: assignment,
      },
    });
    await retry;

    assert.deepEqual(redirects, ["/subpackages/task/detail/index?id=allocated"]);
    require("../src/subpackages/task/detail/index");
    const execution = page;
    execution.onLoad({ id: "allocated" });
    await execution.load();
    assert.equal(execution.data.task.id, "allocated");
    assert.equal(assignmentReads.length, 0);
    await execution.load();
    assert.equal(assignmentReads.length, 1);
    await home.load();
    assert.equal(home.data.today.id, "allocated");
    assert.equal(home.data.guide.label, "第三步：把任务做完，交链接");

    await detail.onShow();
    assert.equal(detail.data.actionLabel, "查看今日任务");
    await detail.followAction();
    assert.equal(requests.length, 2);
    assert.deepEqual(redirects, [
      "/subpackages/task/detail/index?id=allocated",
      "/subpackages/task/detail/index?id=allocated",
    ]);
  } finally {
    global.wx = prior.wx;
    global.Page = prior.Page;
  }
});
