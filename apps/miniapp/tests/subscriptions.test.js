const assert = require("node:assert/strict");
const test = require("node:test");
const { subscribe, scenes } = require("../src/services/subscriptions");

test("提交营销报名时同时订阅报名结果和后续指派", () => {
  assert.deepEqual(scenes.enrollment, [
    "lRpXa_GcSSdTfsMcNX1vl4fC9jCsvAiQSr_fjp66nls",
    "RYd5-QWO-HO5Ur2UsI2qI1GkjA_7LrLj4DKk1eTLLhI",
  ]);
});

test("记住选择后每次用户操作仍获取一次授权，逐模板记录真实结果", async () => {
  const records = [];
  let requests = 0;
  const ids = scenes.withdrawal;
  global.wx = {
    getStorageSync: () => "session",
    requestSubscribeMessage({ tmplIds, success }) {
      requests++;
      assert.deepEqual(tmplIds, ids);
      success({ [ids[0]]: "reject", [ids[1]]: "accept" });
    },
    getSetting({ success }) {
      success({
        subscriptionsSetting: {
          mainSwitch: true,
          itemSettings: { [ids[0]]: "reject" },
        },
      });
    },
  };
  try {
    const save = async (batch) => records.push(batch);
    await subscribe("withdrawal", save);
    await subscribe("withdrawal", save);
    assert.equal(requests, 2);
    assert.notEqual(records[0].attempt, records[1].attempt);
    assert.deepEqual(
      records[0].choices.map((c) => c.result),
      ["reject", "accept"],
    );
    assert.equal(records[0].choices[0].remembered, "reject");
    assert.equal(records[0].choices[1].remembered, "");
  } finally {
    delete global.wx;
  }
});
