const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");

test("我的反馈、报名咨询及旧反馈链接使用微信原生客服", () => {
  const source = path.join(__dirname, "../src");
  for (const [page, label] of [
    ["pages/me", "我的反馈"],
    ["subpackages/member/enrollment", "「我的反馈」咨询"],
    ["subpackages/member/feedback", "联系微信客服"],
  ]) {
    const markup = fs.readFileSync(path.join(source, page, "index.wxml"), "utf8");
    const buttons = [...markup.matchAll(/<button\b([^>]*)>([\s\S]*?)<\/button>/g)];
    assert.ok(buttons.some(([, attrs, body]) =>
      /\bopen-type="contact"/.test(attrs) && body.includes(label)
    ), page);
  }
  const feedback = path.join(source, "subpackages/member/feedback");
  assert.equal(require(path.join(feedback, "index.json")).navigationBarTitleText, "我的反馈");
});
