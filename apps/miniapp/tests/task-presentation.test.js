const assert = require("node:assert/strict");
const test = require("node:test");
const { taskProgress, taskDeadline } = require("../src/services/task-progress");
const { homeGuide, marketingTaskStatus } = require("../src/pages/home/guide");
const { todaysAssignment } = require("../src/services/marketing-tasks");

const steps = [
  { key: "content", done: true },
  { key: "publish", done: true },
  { key: "links", done: true },
  { key: "data", done: false },
  { key: "settle", done: false },
];

test("the third step calls out rejected links without hiding completed manual steps", () => {
  const progress = taskProgress({
    steps,
    platforms: ["dy", "xhs"],
    submissions: [
      { platform: "dy", status: "approved", url: "https://example.com/dy" },
      { platform: "xhs", status: "rejected", url: "https://example.com/xhs" },
    ],
  });

  assert.deepEqual(
    progress.items.map(({ label, state }) => [label, state]),
    [
      ["拿内容", "done"],
      ["发布", "done"],
      ["交链接", "rejected"],
      ["审核", "waiting"],
      ["结算", "waiting"],
    ],
  );
});

test("review and settlement follow platform decisions and confirmed credit separately", () => {
  const assignment = {
    steps,
    platforms: ["dy"],
    submissions: [
      { platform: "dy", status: "pending", url: "https://example.com/dy" },
    ],
  };

  assert.equal(taskProgress(assignment).items[3].state, "current");
  assignment.submissions[0].status = "approved";
  assert.equal(taskProgress(assignment).items[3].state, "done");
  assert.equal(taskProgress(assignment).items[4].state, "current");
  assignment.submissions[0].settled = true;
  assignment.steps = steps.map((step) =>
    step.key === "settle" ? { ...step, done: true } : step,
  );
  assert.equal(taskProgress(assignment).items[4].state, "done");
});

test("partial submission does not consume the review step for the other platform", () => {
  const progress = taskProgress({
    status: "pending_review",
    steps: steps.map((step) =>
      step.key === "links" ? { ...step, done: false } : step,
    ),
    platforms: ["dy", "xhs"],
    submissions: [
      { platform: "dy", status: "pending", url: "https://example.com/dy" },
    ],
  });

  assert.equal(progress.items[2].state, "current");
  assert.equal(progress.items[3].state, "waiting");
});

test("deadline uses a server time point and stops driving edits once a link is submitted", () => {
  const now = Date.parse("2026-11-18T03:00:00Z");
  const assignment = {
    expires_at: "2026-11-18T11:02:03+08:00",
    submissions: [],
  };

  assert.equal(taskDeadline(assignment, now).remaining, "00:02:03");
  assert.equal(
    taskDeadline(assignment, now + 123000).remaining,
    "等待系统确认",
  );
  assert.equal(taskDeadline({ ...assignment, status: "expired" }, now), null);
  assert.equal(
    taskDeadline(
      { ...assignment, submissions: [{ url: "https://example.com" }] },
      now,
    ),
    null,
  );
  assert.equal(taskDeadline({ due_date: "2026-11-18" }, now), null);
  assert.equal(taskDeadline({ expires_at: "2026-11-18T11:02:03" }, now), null);
});

test("day-end deadline is displayed as next-day midnight in Beijing", () => {
  const assignment = { expires_at: "2026-11-19T16:00:00Z" };
  assert.deepEqual(
    taskDeadline(assignment, Date.parse("2026-11-19T15:59:00Z")),
    {
      time: "11-20 00:00",
      remaining: "00:01:00",
    },
  );
});

test("today's assigned, expired and withdrawn work drive the guide accurately", () => {
  const now = Date.parse("2026-11-17T17:00:00Z");
  const user = { status: "approved", consented: true };
  const record = {
    id: "a",
    date: "2026-11-18",
    source: "assigned",
    status: "in_progress",
  };

  assert.equal(
    todaysAssignment([{ ...record, date: "2026-11-17" }, record], now).id,
    "a",
  );
  assert.equal(homeGuide(user, record).action, "assignment");
  assert.equal(
    homeGuide(user, { ...record, status: "expired" }).label,
    "今日任务已失效，查看记录",
  );
  assert.equal(
    todaysAssignment([{ ...record, status: "revoked" }], now),
    undefined,
  );
  assert.equal(homeGuide({ ...user, consented: false }).action, "agreement");
  assert.equal(homeGuide({ ...user, blocked: true }).action, "blocked");
});

test("marketing card shows the current action using explicit task steps", () => {
  const record = {
    source: "claim",
    status: "in_progress",
    platforms: ["dy", "xhs"],
    steps: [],
    submissions: [],
  };

  assert.deepEqual(marketingTaskStatus(record), {
    message: "已领取 · 第 1/4 步 拿内容和要求",
    button: "继续做",
  });

  const published = {
    ...record,
    source: "assigned",
    status: "pending_review",
    steps: steps.map((step) =>
      step.key === "links" ? { ...step, done: false } : step,
    ),
    submissions: [
      { platform: "dy", status: "pending", url: "https://example.com/dy" },
    ],
  };
  assert.deepEqual(marketingTaskStatus(published), {
    message: "被工作人员指派 · 第 3/4 步 交发布链接",
    button: "继续做",
  });
});

test("marketing card distinguishes review, resubmission and payout", () => {
  const record = {
    source: "reissued",
    status: "pending_review",
    platforms: ["dy"],
    steps,
    submissions: [
      { platform: "dy", status: "pending", url: "https://example.com/dy" },
    ],
  };

  assert.equal(marketingTaskStatus(record).button, "看审核进度");
  record.status = "rejected";
  record.submissions[0].status = "rejected";
  assert.equal(marketingTaskStatus(record).button, "去修改");
  record.status = "pending_settlement";
  record.submissions[0].status = "approved";
  assert.deepEqual(marketingTaskStatus(record), {
    message: "被重新派发 · 待交播放数据",
    button: "去交数据",
    section: "screenshots",
  });
  record.submissions[0].screenshot = "file-id";
  assert.equal(
    marketingTaskStatus(record).message,
    "被重新派发 · 审核通过，等结算",
  );
  assert.equal(
    marketingTaskStatus({ ...record, status: "settled" }).message,
    "已结算 · 奖励已入账",
  );
});
