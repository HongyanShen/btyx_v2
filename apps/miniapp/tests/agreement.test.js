const assert = require("node:assert/strict");
const test = require("node:test");

let definition;
global.Page = (options) => {
  definition = options;
};
require("../src/subpackages/member/agreement/index");
delete global.Page;

test("the agreement page clears an older agreement and shows the empty state when none is published", async (t) => {
  const page = {
    ...definition,
    data: { ...definition.data },
    setData(values) {
      Object.assign(this.data, values);
    },
  };
  const agreement = { id: "previous", title: "分发协议", body: "协议正文" };
  let statusCode = 200;
  const requests = [];
  global.wx = {
    getStorageSync: () => "test-session",
    request({ url, success }) {
      if (url.endsWith("/agreement")) {
        requests.push("agreement");
        success({
          statusCode,
          data:
            statusCode === 200
              ? { code: 0, data: agreement }
              : {
                  code: statusCode,
                  msg:
                    statusCode === 409
                      ? "当前营销分发任务协议尚未发布"
                      : "服务暂时不可用",
                },
        });
        return;
      }

      requests.push("profile");
      success({
        statusCode: 200,
        data: { code: 0, data: { consented: true } },
      });
    },
  };
  t.after(() => delete global.wx);

  await page.onShow();
  assert.equal(page.data.agreement.id, "previous");
  assert.equal(page.data.consented, true);

  statusCode = 409;
  await page.onShow();
  assert.equal(page.data.loading, false);
  assert.equal(page.data.agreement, null);
  assert.equal(page.data.consented, false);
  assert.equal(page.data.error, "");
  assert.deepEqual(requests, ["agreement", "profile", "agreement"]);

  statusCode = 500;
  await page.onShow();
  assert.equal(page.data.error, "服务暂时不可用");
});
