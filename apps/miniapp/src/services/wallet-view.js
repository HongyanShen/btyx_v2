const { formatFen, yuanToFen } = require("./money");
const { LedgerKind } = require("./generated/enums");

const ledgerLabels = {
  [LedgerKind.LedgerSettlementCredit]: "分发结算入账",
  [LedgerKind.LedgerWithdrawalFreeze]: "提现冻结",
  [LedgerKind.LedgerWithdrawalSuccess]: "提现到账",
  [LedgerKind.LedgerWithdrawalRefund]: "提现退回",
  [LedgerKind.LedgerAdjustment]: "余额调整",
};

function exceedsFen(left, right) {
  return (
    left.length > right.length || (left.length === right.length && left > right)
  );
}

function presentWallet(wallet) {
  const availableFen = wallet.available_fen;
  const minFen = wallet.min_withdrawal_fen;
  const maxFen = wallet.max_withdrawal_fen;
  const canWithdraw = !exceedsFen(minFen, availableFen);
  const frozenFen = wallet.frozen_fen || "0";
  return {
    earned: formatFen(wallet.earned_fen),
    pending: formatFen(wallet.pending_fen),
    monthEarned: formatFen(wallet.month_earned_fen),
    available: formatFen(availableFen),
    frozen: formatFen(frozenFen),
    hasFrozen: frozenFen !== "0",
    availableFen,
    minFen,
    maxFen,
    max: formatFen(maxFen),
    canWithdraw,
    defaultAmount: canWithdraw
      ? formatFen(exceedsFen(availableFen, maxFen) ? maxFen : availableFen)
      : "",
    hasEarnings:
      wallet.earned_fen !== "0" ||
      wallet.pending_fen !== "0" ||
      availableFen !== "0",
  };
}

function withdrawalAmountError(value, wallet) {
  if (!value.trim()) return "请输入提现金额";
  let amount;
  try {
    amount = yuanToFen(value);
  } catch (error) {
    return error.message;
  }

  if (exceedsFen(wallet.minFen, amount))
    return "单笔最少 ¥" + formatFen(wallet.minFen);
  if (exceedsFen(amount, wallet.availableFen))
    return "超过可提现余额 ¥" + wallet.available;
  if (exceedsFen(amount, wallet.maxFen))
    return "单笔最多 ¥" + wallet.max + "，超过请分几次提";
  return "";
}

function walletTime(value) {
  const beijing = new Date(
    Date.parse(value) + 8 * 60 * 60 * 1000,
  ).toISOString();
  return beijing.slice(0, 10) + " " + beijing.slice(11, 16);
}

function presentLedger(entry) {
  const amount = formatFen(entry.amount_fen);
  const incoming = !amount.startsWith("-") && amount !== "0.00";
  return {
    id: entry.id,
    label: entry.kind === LedgerKind.LedgerSettlementCredit && entry.reason?.startsWith("创作任务 · ") ? entry.reason : ledgerLabels[entry.kind],
    reason: entry.reason,
    time: walletTime(entry.created_at),
    signedAmount: amount.startsWith("-")
      ? "−¥" + amount.slice(1)
      : (incoming ? "+¥" : "¥") + amount,
    incoming,
  };
}

module.exports = {
  presentLedger,
  presentWallet,
  walletTime,
  withdrawalAmountError,
};
