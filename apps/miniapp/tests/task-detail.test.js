const assert = require("node:assert/strict");
const { readFileSync } = require("node:fs");
const { join } = require("node:path");
const test = require("node:test");
const {
  publicationLink,
} = require("../src/subpackages/task/detail/presentation");

test("小红书分享链接校验兼容新旧短链并检查完整域名", () => {
  assert.equal(
    publicationLink("xhs", "笔记 https://xhslink.cn/o/test?source=share。"),
    "https://xhslink.cn/o/test?source=share",
  );
  assert.equal(
    publicationLink("xhs", "我发布了一篇笔记：http://xhslink.com/a/efgh"),
    "https://xhslink.com/a/efgh",
  );
  assert.equal(
    publicationLink("xhs", "https://xhslink.cn.example.com/o/test"),
    "",
  );
  assert.equal(publicationLink("dy", "https://xhslink.cn/o/test"), "");
});

test("交链接表单的补交入口只显示审核已处理的平台", () => {
  const markup = readFileSync(
    join(__dirname, "../src/subpackages/task/detail/index.wxml"),
    "utf8",
  );
  assert.doesNotMatch(markup, /bindtap="toggleScreenshots"/);
  assert.match(
    markup,
    /wx:if="{{screenshotsOpen && task.hasPlaybackPlatforms}}"\s+id="screenshots"/,
  );
  assert.match(
    markup,
    /wx:if="{{item.showPlayback}}" class="execution-screenshot-row"/,
  );
});

test("统一提交保留各平台输入，部分成功后只重试失败平台", async () => {
  const assignment = {
    id: "assignment",
    price_fen: "750",
    platforms: ["dy", "xhs"],
    steps: [],
    submissions: [],
    status: "in_progress",
  };
  const submissions = [];
  const toasts = [];
  const scrollTargets = [];
  let failPlatform = "";
  const prior = { wx: global.wx, Page: global.Page };
  let page;
  global.Page = (definition) => {
    page = {
      ...definition,
      setData(patch, callback) {
        for (const [key, value] of Object.entries(patch)) {
          const [field, platform] = key.split(".");
          if (platform) {
            this.data[field][platform] = value;
          } else {
            this.data[field] = value;
          }
        }
        callback?.();
      },
    };
  };
  global.wx = {
    getStorageSync: () => "session",
    showToast: ({ title }) => toasts.push(title),
    pageScrollTo: ({ selector }) => scrollTargets.push(selector),
    request(options) {
      if (options.method === "POST") {
        submissions.push(options.data);
        if (options.data.platform === failPlatform) {
          options.success({
            statusCode: 200,
            data: { code: 1, msg: "请重试" },
          });
          return;
        }
        assignment.submissions = assignment.submissions.filter(
          ({ platform }) => platform !== options.data.platform,
        );
        assignment.submissions.push({
          ...options.data,
          status: "pending",
        });
      }
      const data = options.method === "POST" ? null : assignment;
      options.success({ statusCode: 200, data: { code: 0, data } });
    },
  };

  try {
    require("../src/subpackages/task/detail/index");
    page.onLoad({ id: "assignment" });
    await page.load();
    const dy = { currentTarget: { dataset: { platform: "dy" } } };
    const xhs = { currentTarget: { dataset: { platform: "xhs" } } };
    const dyShare = "复制打开抖音，看看作品\nhttps://v.douyin.com/abcd/。";
    const xhsShare = String.raw`上班前vs上班后 笑死了 https\://xhslink.cn/o/5IKcSj0r69I 先复制一下，然后去【小红书】搜索查看笔记。`;
    const changeLink = (event, value) => {
      page.changeLink({ ...event, detail: { value } });
    };

    assert.equal(page.data.task.price, "7.50");
    assert.equal(page.data.task.dualPlatformPrice, "15.00");
    assert.equal(page.data.screenshotsOpen, false);
    assert.match(page.data.platforms[0].placeholder, /直接粘贴抖音的分享文字/);
    changeLink(dy, dyShare);
    changeLink(xhs, dyShare);
    await page.submitLinks();
    assert.equal(submissions.length, 0);
    assert.match(toasts.at(-1), /小红书/);

    changeLink(xhs, "");
    await page.submitLinks();
    assert.equal(submissions.length, 1);
    assert.equal(submissions[0].platform, "dy");
    assert.equal(submissions[0].url, "https://v.douyin.com/abcd/");

    const updatedDyShare =
      "复制打开抖音，看看新作品 https://www.douyin.com/video/123";
    changeLink(dy, updatedDyShare);
    changeLink(xhs, xhsShare);
    failPlatform = "xhs";
    await page.submitLinks();

    assert.deepEqual(
      submissions.slice(1).map(({ platform }) => platform),
      ["dy", "xhs"],
    );
    assert.equal(page.data.links.dy, updatedDyShare);
    assert.equal(
      page.data.platforms[0].submittedURL,
      "https://www.douyin.com/video/123",
    );
    assert.equal(page.data.links.xhs, xhsShare);
    assert.match(toasts.at(-1), /小红书.*请重试/);

    failPlatform = "";
    assignment.submissions[0].status = "rejected";
    await page.load();
    await page.submitLinks();
    assert.equal(submissions.length, 4);
    assert.equal(submissions[3].url, "https://xhslink.cn/o/5IKcSj0r69I");

    await page.submitLinks();
    assert.equal(submissions[4].platform, "dy");
    assert.equal(submissions[4].url, "https://www.douyin.com/video/123");

    const dyRecord = assignment.submissions.find(
      ({ platform }) => platform === "dy",
    );
    page.onLoad({ id: "assignment", section: "screenshots" });
    await page.load();
    assert.equal(page.data.task.hasPlaybackPlatforms, false);
    assert.deepEqual(scrollTargets, []);
    page.setData({ "screenshots.dy": "ineligible" });
    await page.submitScreenshot(dy);
    assert.equal(submissions.length, 5);

    dyRecord.status = "approved";
    page.onLoad({ id: "assignment" });
    await page.load();
    assert.equal(page.data.platforms[0].linkLocked, true);
    assert.equal(page.data.platforms[0].showPlayback, true);
    assert.equal(page.data.platforms[1].showPlayback, false);

    page.onLoad({ id: "assignment", section: "screenshots" });
    await page.load();
    assert.equal(page.data.screenshotsOpen, true);
    assert.equal(page.data.openStage, "links");
    assert.deepEqual(scrollTargets, ["#screenshots"]);
    const unpublishedDraft = "https://xhslink.com/a/next";
    changeLink(xhs, unpublishedDraft);
    page.drafts.dy = { screenshot: "file-id" };
    page.setData({ "screenshots.dy": "file-id" });
    await page.submitScreenshot(dy);
    assert.deepEqual(submissions[5], {
      platform: "dy",
      url: dyRecord.url,
      screenshot: "file-id",
    });
    assert.equal(page.data.links.xhs, unpublishedDraft);
  } finally {
    global.wx = prior.wx;
    global.Page = prior.Page;
  }
});
