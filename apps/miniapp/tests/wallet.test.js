const assert = require("node:assert/strict");
const test = require("node:test");
const {
  withdrawalAmountError,
  presentWallet,
  presentLedger,
} = require("../src/services/wallet-view");

test("收益统计使用服务端金额，收支流水保留每笔的正负方向", () => {
  const wallet = presentWallet({
    earned_fen: "900719925474099300",
    pending_fen: "750",
    month_earned_fen: "2000",
    available_fen: "3750",
    min_withdrawal_fen: "10",
    max_withdrawal_fen: "20000",
  });
  assert.equal(wallet.earned, "9007199254740993.00");
  assert.equal(wallet.pending, "7.50");
  assert.equal(wallet.monthEarned, "20.00");
  assert.equal(wallet.defaultAmount, "37.50");
  assert.equal(
    presentLedger({
      kind: "withdrawal_freeze",
      amount_fen: "-1050",
      created_at: "2026-11-18T16:02:00Z",
    }).signedAmount,
    "−¥10.50",
  );
  assert.equal(
    presentLedger({
      kind: "settlement_credit",
      amount_fen: "750",
      created_at: "2026-11-18T16:02:00Z",
    }).time,
    "2026-11-19 00:02",
  );
});

test("提现默认金额受单笔限额约束，校验余额和最小金额", () => {
  const limits = {
    min_withdrawal_fen: "10",
    max_withdrawal_fen: "20000",
    earned_fen: "0",
    pending_fen: "0",
    month_earned_fen: "0",
  };
  const wallet = presentWallet({ ...limits, available_fen: "30000" });
  assert.equal(wallet.defaultAmount, "200.00");
  assert.match(withdrawalAmountError("200.01", wallet), /单笔最多/);
  assert.match(withdrawalAmountError("0.09", wallet), /单笔最少/);
  assert.match(withdrawalAmountError("1.555", wallet), /两位小数/);
  assert.equal(withdrawalAmountError("20.00", wallet), "");
  const small = presentWallet({ ...limits, available_fen: "50" });
  assert.match(withdrawalAmountError("1.00", small), /超过可提现余额/);
  assert.equal(
    presentWallet({ ...limits, available_fen: "9" }).canWithdraw,
    false,
  );
});
