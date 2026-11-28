const { WithdrawalStatus } = require("./generated/enums");
const { withdrawalLabels } = require("./labels");
const { formatFen } = require("./money");
const { walletTime } = require("./wallet-view");
const { newKey, post } = require("./request");

function presentWithdrawal(record) {
  const { status } = record;
  const success = status === WithdrawalStatus.WithdrawalSuccess;
  const refunded =
    status === WithdrawalStatus.WithdrawalFailed ||
    status === WithdrawalStatus.WithdrawalCancelled;
  const waiting = status === WithdrawalStatus.WithdrawalWaitingConfirmation;
  const canConfirm = waiting && Boolean(record.merchant_id && record.package_info);
  const frozen = !success && !refunded;
  const showReason =
    status === WithdrawalStatus.WithdrawalUnknown ||
    status === WithdrawalStatus.WithdrawalBlockedConfig;
  let reason = showReason ? record.reason || "" : "";
  if (waiting && !canConfirm) {
    reason = "本笔暂无收款凭证，请等待微信处理结果";
  }
  return {
    ...record,
    amount: formatFen(record.amount_fen),
    label: withdrawalLabels[status] || "转账结果待确认",
    time: walletTime(record.created_at),
    reason,
    frozen,
    waitingConfirmation: waiting,
    canConfirm,
    tone: success ? "done" : frozen ? "waiting" : "neutral",
  };
}

function requestWithdrawalReceipt(record) {
  if (!record.canConfirm) {
    wx.showToast({ title: "暂缺收款凭证，请等待状态更新", icon: "none" });
    return Promise.resolve();
  }

  if (!wx.canIUse("requestMerchantTransfer")) {
    wx.showToast({ title: "请在支持收款的微信版本中重试", icon: "none" });
    return Promise.resolve();
  }

  return new Promise((resolve) => {
    wx.requestMerchantTransfer({
      mchId: record.merchant_id,
      appId: wx.getAccountInfoSync().miniProgram.appId,
      package: record.package_info,
      fail: (error) => {
        if (!/\bcancel(?:led)?\b/i.test(error?.errMsg || "")) {
          wx.showToast({ title: "微信收款页打开失败，请重试", icon: "none" });
        }
      },
      complete: resolve,
    });
  });
}

function withdrawalAttemptKey(memberID) {
  return "btyx-pending-withdrawal:" + memberID;
}

function getWithdrawalAttempt(memberID) {
  const attempt = wx.getStorageSync(withdrawalAttemptKey(memberID));
  return attempt?.key && typeof attempt.amountFen === "string" ? attempt : null;
}

async function submitWithdrawal(memberID, amountFen) {
  const storageKey = withdrawalAttemptKey(memberID);
  let attempt = getWithdrawalAttempt(memberID);
  if (!attempt) {
    attempt = { amountFen, key: newKey() };
    wx.setStorageSync(storageKey, attempt);
  }
  let record;
  try {
    record = await post(
      "/withdrawals",
      { amount_fen: attempt.amountFen },
      attempt.key,
    );
  } catch (error) {
    if (
      error.status >= 400 &&
      error.status < 500 &&
      ![401, 403, 408, 429].includes(error.status)
    ) {
      wx.removeStorageSync(storageKey);
    }
    throw error;
  }
  wx.removeStorageSync(storageKey);
  return record;
}

module.exports = {
  presentWithdrawal,
  requestWithdrawalReceipt,
  getWithdrawalAttempt,
  submitWithdrawal,
};
