const assert = require("node:assert/strict");
const test = require("node:test");
const {
  executionTimeline,
  presentMaterials,
  taskRequirements,
} = require("../src/subpackages/task/detail/presentation");
const { taskProgress } = require("../src/services/task-progress");

test("执行时间线展开当前步骤，审核和结算仍保留独立的顶部进度", () => {
  const assignment = { status: "in_progress", steps: [], submissions: [] };
  const initial = taskProgress(assignment);
  assert.equal(initial.items.length, 5);
  const timeline = executionTimeline(initial, ["抖音", "小红书"]);
  assert.equal(timeline.active, "content");
  assert.deepEqual(
    timeline.items.map(({ title, state }) => [title, state]),
    [
      ["拿内容和要求", "current"],
      ["发抖音和小红书", "waiting"],
      ["交两条链接", "waiting"],
      ["奖励记账", "waiting"],
    ],
  );

  assignment.steps = ["content", "publish", "links"].map((key) => ({
    key,
    done: true,
  }));
  assignment.submissions = [{ status: "pending", url: "https://example.com" }];
  assert.equal(
    executionTimeline(taskProgress(assignment), ["抖音"]).active,
    "settle",
  );
  assignment.submissions[0].status = "rejected";
  assert.equal(
    executionTimeline(taskProgress(assignment), ["抖音"]).active,
    "links",
  );
});

test("素材采用真实 MIME 类型和文件名，保持图片顺序，签发失败仍可单独重试下载", () => {
  const files = ["first", "video", "second", "unavailable"];
  const tickets = [
    {
      name: "01.jpg",
      media_type: "image/jpeg",
      storage: "tos",
      url: "https://example.com/first",
    },
    {
      name: "成片.MP4",
      media_type: "video/mp4",
      storage: "tos",
      url: "https://example.com/video",
    },
    { name: "02.png", media_type: "image/png", storage: "local" },
    null,
  ];
  const result = presentMaterials(files, tickets);
  assert.deepEqual(
    result.images.map((image) => image.id),
    ["first", "second"],
  );
  assert.equal(result.images[0].imageURL, "https://example.com/first");
  assert.equal(result.images[1].imageURL, "");
  assert.equal(result.videos[0].title, "成片");
  assert.equal(result.attachments[0].id, "unavailable");
});

test("按后台格式拆分分发语、标签、账号和话题要求，保留普通文案换行", () => {
  const result = taskRequirements(
    "第一行\n第二行\nTAG：#校园 #活动\n@账号：@官方\n话题至少带满 5 个（含必带 TAG）",
  );
  assert.deepEqual(
    result.fields.map(({ label, value }) => [label, value]),
    [
      ["分发语", "第一行\n第二行"],
      ["TAG", "#校园 #活动"],
      ["@账号", "@官方"],
    ],
  );
  assert.equal(result.topicNote, "话题至少带满 5 个（含必带 TAG）");
});
