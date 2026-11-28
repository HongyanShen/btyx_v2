const assert = require("node:assert/strict");
const test = require("node:test");

test("待确认收款缺少凭证或微信拉起失败时，会给出安全提示", async (t) => {
  const {
    presentWithdrawal,
    requestWithdrawalReceipt,
  } = require("../src/services/withdrawals");
  const previousWX = global.wx;
  t.after(() => {
    global.wx = previousWX;
  });
  const notices = [];
  const requested = [];
  let failMessage = "requestMerchantTransfer:fail mock-private-detail";
  global.wx = {
    canIUse: () => true,
    getAccountInfoSync: () => ({ miniProgram: { appId: "test-appid" } }),
    showToast: ({ title }) => notices.push(title),
    requestMerchantTransfer(options) {
      requested.push(options.package);
      options.fail({ errMsg: failMessage });
      options.complete();
    },
  };
  const bill = {
    id: "bill",
    status: "waiting_confirmation",
    amount_fen: "10",
    created_at: "2026-11-26T08:00:00Z",
    merchant_id: "test-merchant",
  };
  const missing = presentWithdrawal(bill);
  assert.equal(missing.waitingConfirmation, true);
  assert.match(missing.reason, /收款凭证/);
  await requestWithdrawalReceipt(missing);
  assert.deepEqual(requested, []);
  assert.match(notices[0], /收款凭证/);

  const ready = presentWithdrawal({ ...bill, package_info: "test-package" });
  await requestWithdrawalReceipt(ready);
  assert.deepEqual(requested, ["test-package"]);
  assert.match(notices[1], /收款页打开失败/);
  assert.ok(!notices.join().includes("mock-private-detail"));
  failMessage = "requestMerchantTransfer:fail cancel";
  await requestWithdrawalReceipt(ready);
  assert.equal(notices.length, 2);
});

test("提现记录回显资金状态，只有具备收款参数的待确认单可去收款", () => {
  const { presentWithdrawal } = require("../src/services/withdrawals");
  const record = {
    id: "bill",
    amount_fen: "120",
    created_at: "2026-11-25T18:00:00Z",
    merchant_id: "merchant",
    package_info: "package",
  };
  for (const status of [
    "queued",
    "submitting",
    "unknown",
    "waiting_confirmation",
    "blocked_config",
  ]) {
    const shown = presentWithdrawal({ ...record, status });
    assert.equal(shown.frozen, true);
    assert.equal(shown.canConfirm, status === "waiting_confirmation");
    assert.equal(shown.time, "2026-11-26 02:00");
  }
  for (const status of ["success", "failed", "cancelled"]) {
    const shown = presentWithdrawal({
      ...record,
      status,
      reason: "结果待确认",
    });
    assert.equal(shown.frozen, false);
    assert.equal(shown.canConfirm, false);
    assert.equal(shown.reason, "");
  }
  assert.equal(
    presentWithdrawal({
      ...record,
      status: "waiting_confirmation",
      package_info: "",
    }).canConfirm,
    false,
  );
});

test("提现响应丢失后重新打开仍复用原金额和幂等键", async (t) => {
  const storage = new Map([["btyx-session", "test-session"]]);
  const posts = [];
  const previous = { Page: global.Page, wx: global.wx };
  t.after(() => Object.assign(global, previous));
  let definition;
  let user = "member-a";
  let available = "100";
  global.Page = (value) => {
    definition = value;
  };
  global.wx = {
    getStorageSync: (key) => storage.get(key),
    setStorageSync: (key, value) => storage.set(key, value),
    removeStorageSync: (key) => storage.delete(key),
    showToast() {},
    request(options) {
      const path = options.url.split("/api/v1")[1];
      const bill = {
        id: "bill",
        amount_fen: "100",
        status: "queued",
        created_at: "2026-11-25T18:00:00Z",
      };
      let data;
      if (path === "/me") data = { id: user };
      if (path === "/wallet")
        data = {
          available_fen: available,
          frozen_fen: available === "0" ? "100" : "0",
          earned_fen: "100",
          month_earned_fen: "100",
          pending_fen: "0",
          min_withdrawal_fen: "10",
          max_withdrawal_fen: "20000",
        };
      if (path === "/withdrawals") data = posts.length ? [bill] : [];
      if (path === "/withdrawals" && options.method === "POST") {
        posts.push({
          key: options.header["Idempotency-Key"],
          amount: options.data.amount_fen,
        });
        if (posts.length === 1) {
          available = "0";
          options.fail();
          return;
        }
        data = bill;
      }
      options.success({ statusCode: 200, data: { code: 0, data } });
    },
  };
  require("../src/subpackages/member/withdrawal/index");
  const open = async () => {
    const page = {
      ...definition,
      data: structuredClone(definition.data),
      setData(patch) {
        Object.assign(this.data, patch);
      },
    };
    await page.load(true);
    return page;
  };

  const first = await open();
  await first.withdraw();
  assert.equal(first.data.retrying, true);
  user = "member-b";
  const other = await open();
  assert.equal(other.data.retrying, false);
  user = "member-a";
  const replay = await open();
  assert.equal(replay.data.retrying, true);
  assert.equal(replay.data.amount, "1.00");
  assert.equal(replay.data.wallet.frozen, "1.00");
  await replay.withdraw();
  assert.ok(posts[0].key);
  assert.deepEqual(posts[0], posts[1]);
  assert.equal((await open()).data.retrying, false);
});

test("提现金额可在表单内直接编辑，金额校验和重试限制有效", (t) => {
  const previousPage = global.Page;
  t.after(() => {
    global.Page = previousPage;
  });
  let definition;
  global.Page = (value) => {
    definition = value;
  };
  const modulePath =
    require.resolve("../src/subpackages/member/withdrawal/index");
  delete require.cache[modulePath];
  require(modulePath);
  const page = {
    ...definition,
    data: {
      ...structuredClone(definition.data),
      amount: "15.00",
      wallet: {
        available: "15.00",
        availableFen: "1500",
        minFen: "10",
        maxFen: "20000",
        max: "200.00",
        defaultAmount: "15.00",
        canWithdraw: true,
      },
    },
    setData(patch) {
      Object.assign(this.data, patch);
    },
  };
  const input = (value) => page.changeAmount({ detail: { value } });

  input("7.005");
  assert.match(page.data.amountError, /两位小数/);
  page.finishAmountEdit();
  assert.equal(page.data.amount, "7.005");
  input("16");
  assert.match(page.data.amountError, /超过可提现余额/);
  input("7.5");
  assert.equal(page.data.amount, "7.5");
  assert.equal(page.data.amountError, "");
  page.finishAmountEdit();
  assert.equal(page.data.amount, "7.50");
  page.useAll();
  assert.equal(page.data.amount, "15.00");

  page.setData({ retrying: true });
  input("1");
  assert.equal(page.data.amount, "15.00");
});
