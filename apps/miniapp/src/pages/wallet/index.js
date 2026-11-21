const { request } = require("../../services/request");
const {
  hasSession,
  openLogin,
  pendingSession,
} = require("../../services/session");
const { formatFen } = require("../../services/money");
const { LedgerKind } = require("../../services/generated/enums");

const ledgerLabels = {
  [LedgerKind.LedgerSettlementCredit]: "周结算入账",
  [LedgerKind.LedgerWithdrawalFreeze]: "提现冻结",
  [LedgerKind.LedgerWithdrawalSuccess]: "提现到账",
  [LedgerKind.LedgerWithdrawalRefund]: "提现退回",
  [LedgerKind.LedgerAdjustment]: "余额调整",
};

Page({
  data: { loading: true, signedIn: false, error: "", wallet: null, ledger: [] },

  onShow() {
    this.load();
  },

  async load() {
    this.setData({ loading: true, error: "", wallet: null, ledger: [] });

    await pendingSession();
    const signedIn = hasSession();
    this.setData({ loading: signedIn, signedIn });
    if (!signedIn) return;

    try {
      const [wallet, ledger] = await Promise.all([
        request("/wallet"),
        request("/ledger"),
      ]);
      this.setData({
        wallet: {
          available: formatFen(wallet.available_fen),
          frozen: formatFen(wallet.frozen_fen),
          withdrawn: formatFen(wallet.withdrawn_fen),
        },
        ledger: ledger.map((entry) => ({
          ...entry,
          amount: formatFen(entry.amount_fen),
          label: ledgerLabels[entry.kind],
          time: new Date(entry.created_at).toLocaleString("zh-CN", {
            timeZone: "Asia/Shanghai",
          }),
        })),
      });
    } catch (error) {
      this.setData({ signedIn: hasSession(), error: error.message });
    } finally {
      this.setData({ loading: false });
    }
  },

  openLogin,

  openWithdrawals() {
    wx.navigateTo({ url: "/subpackages/member/withdrawal/index" });
  },
});
