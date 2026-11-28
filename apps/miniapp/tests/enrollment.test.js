const assert = require("node:assert/strict");
const test = require("node:test");

test("a rejected application prefills its accounts and lets the owner reuse both photos", async () => {
  let definition;
  global.Page = (options) => {
    definition = options;
  };
  require("../src/subpackages/member/enrollment/index");
  delete global.Page;

  const page = {
    ...definition,
    data: { ...definition.data },
    setData(updates) {
      for (const [key, value] of Object.entries(updates)) {
        const [parent, field] = key.split(".");
        if (field) this.data[parent][field] = value;
        else this.data[parent] = value;
      }
    },
  };
  const submissions = [];
  const subscriptionReceipts = [];
  let agreementUnavailable = false;
  let agreement = {
    id: "agreement-one",
    title: "营销分发任务协议",
    body: "协议正文",
  };
  global.wx = {
    getStorageSync: (key) => (key === "btyx-session" ? "test-session" : ""),
    requestSubscribeMessage({ tmplIds, success }) {
      success(Object.fromEntries(tmplIds.map((id) => [id, "reject"])));
    },
    showToast() {},
    request({ method, url, data, success }) {
      if (url.endsWith("/notifications/subscriptions")) {
        subscriptionReceipts.push(data);
        success({ statusCode: 200, data: { code: 0, data: null } });
        return;
      }

      if (url.endsWith("/agreement")) {
        if (agreementUnavailable) {
          success({
            statusCode: 409,
            data: { code: 409, msg: "当前营销分发任务协议尚未发布" },
          });
          return;
        }

        success({ statusCode: 200, data: { code: 0, data: agreement } });
        return;
      }

      if (method !== "GET") {
        submissions.push(data);
        if (data.agreement_id !== agreement.id) {
          success({
            statusCode: 409,
            data: { code: 409, msg: "协议已更新，请重新阅读并确认" },
          });
          return;
        }
      }
      success({
        statusCode: 200,
        data: {
          code: 0,
          data:
            method === "GET"
              ? {
                  id: "owner",
                  enrollment_id: "previous",
                  status: "rejected",
                  identity_verified: false,
                  reason: "小红书主页打不开",
                  douyin: "douyin-owner",
                  xhs: "https://xhslink.com/a/old-profile",
                  enrollment_document: {
                    front_file_id: "previous-front",
                    back_file_id: "previous-back",
                  },
                }
              : null,
        },
      });
    },
  };

  try {
    await page.load();
    assert.equal(page.data.rejected, true);
    assert.equal(page.data.douyin, "douyin-owner");
    assert.equal(page.data.xhs, "");
    assert.equal(page.data.needsXiaohongshuNumber, true);
    assert.deepEqual(page.data.document, {
      front_file_id: "previous-front",
      back_file_id: "previous-back",
    });

    page.data.xhs = "https://www.xiaohongshu.com/user/profile/old";
    await page.submit();
    assert.match(page.data.error, /请填写小红书号/);
    assert.equal(submissions.length, 0);

    page.data.xhs = "xhs_2026";
    await page.load();
    assert.equal(page.data.xhs, "xhs_2026");

    page.photoChanged({
      detail: { side: "front", fileID: "replacement-front" },
    });
    await page.submit();
    assert.match(page.data.error, /识别身份证人像面/);
    assert.equal(submissions.length, 0);

    page.data.document = {
      front_file_id: "previous-front",
      back_file_id: "previous-back",
    };
    agreementUnavailable = true;
    const attempt = page.submit();
    assert.equal(page.data.agreementOpen, false);
    assert.equal(page.data.agreementLoading, true);
    await attempt;
    assert.equal(page.data.agreementOpen, false);
    assert.equal(page.data.agreementLoading, false);
    assert.equal(page.data.error, "暂无协议");
    assert.equal(submissions.length, 0);

    agreementUnavailable = false;
    await page.submit();
    assert.equal(page.data.agreementOpen, true);
    assert.equal(page.data.agreement.id, "agreement-one");
    assert.equal(submissions.length, 0);
    page.dismissAgreement();
    assert.equal(page.data.agreementOpen, false);
    assert.equal(submissions.length, 0);
    assert.equal(page.data.xhs, "xhs_2026");

    await page.submit();
    agreement = { ...agreement, id: "agreement-two", body: "新版协议正文" };
    await page.confirmEnrollment();
    assert.equal(submissions.length, 1);
    assert.equal(submissions[0].agreement_id, "agreement-one");
    assert.equal(page.data.user.status, "rejected");
    assert.equal(page.data.agreement.body, "新版协议正文");
    assert.match(page.data.agreementError, /协议已更新/);

    await page.confirmEnrollment();
    assert.deepEqual(submissions[1], {
      douyin: "douyin-owner",
      xhs: "xhs_2026",
      agreement_id: "agreement-two",
      previous_enrollment_id: "previous",
      front_file_id: "previous-front",
      back_file_id: "previous-back",
    });
    assert.equal(submissions.length, 2);
    assert.equal(page.data.agreementOpen, false);
    assert.equal(page.data.user.status, "pending");
    assert.equal(subscriptionReceipts.length, 2);
    for (const receipt of subscriptionReceipts) {
      assert.equal(receipt.scene, "enrollment");
      assert.equal(receipt.choices[0].template_id, "lRpXa_GcSSdTfsMcNX1vl4fC9jCsvAiQSr_fjp66nls");
      assert.equal(receipt.choices[0].result, "reject");
    }
  } finally {
    delete global.wx;
  }
});
