const { request } = require("../../../services/request");
const {
  isCurrentSession,
  pendingSession,
  tokenKey,
} = require("../../../services/session");
const { formatFen, yuanToFen } = require("../../../services/money");
const { WithdrawalStatus } = require("../../../services/generated/enums");
const {
  presentWallet,
  withdrawalAmountError,
} = require("../../../services/wallet-view");
const {
  getWithdrawalAttempt,
  presentWithdrawal,
  requestWithdrawalReceipt,
  submitWithdrawal,
} = require("../../../services/withdrawals");

const waiting = WithdrawalStatus.WithdrawalWaitingConfirmation;
const success = WithdrawalStatus.WithdrawalSuccess;

const { subscribe } = require("../../../services/subscriptions");

Page({
  data: {
    loading: true,
    error: "",
    wallet: null,
    withdrawals: [],
    hasWithdrawals: false,
    waitingRecords: [],
    amount: "",
    amountError: "",
    submitting: false,
    retrying: false,
    confirming: false,
    completed: null,
  },

  onShow() {
    this.visible = true;
    clearInterval(this.refreshTimer);
    this.refreshTimer = setInterval(() => this.load(false), 15000);
    return this.load(!this.data.wallet);
  },

  onHide() {
    this.visible = false;
    clearInterval(this.refreshTimer);
  },

  onUnload() {
    this.onHide();
  },

  onPullDownRefresh() {
    return this.load(false).finally(() => wx.stopPullDownRefresh());
  },

  retryLoad() {
    return this.load(!this.data.wallet);
  },

  load(showLoading) {
    if (this.refreshing) return this.refreshing;
    this.refreshing = this.loadRecords(showLoading).finally(() => {
      this.refreshing = null;
    });
    return this.refreshing;
  },

  async loadRecords(showLoading) {
    if (showLoading) this.setData({ loading: true, error: "" });
    try {
      await pendingSession();
      const token = wx.getStorageSync(tokenKey);
      const [balance, records, member] = await Promise.all([
        request("/wallet"),
        request("/withdrawals"),
        this.memberToken === token && this.memberID
          ? { id: this.memberID }
          : request("/me"),
      ]);
      if (!member?.id || !isCurrentSession(token))
        throw new Error("登录状态已变化，请重新加载");
      this.memberToken = wx.getStorageSync(tokenKey);
      if (member.id !== this.memberID) {
        this.selectedID = "";
        this.autoReceiptID = "";
        this.amountEdited = false;
      }
      this.memberID = member.id;
      const attempt = getWithdrawalAttempt(member.id);
      const wallet = presentWallet(balance);
      const withdrawals = records.map(presentWithdrawal);
      const selected = withdrawals.find(({ id }) => id === this.selectedID);
      const completed = selected?.status === success ? selected : null;
      const waitingRecords = withdrawals.filter(
        ({ status }) => status === waiting,
      );
      this.setData({
        wallet,
        withdrawals,
        hasWithdrawals: withdrawals.length > 0,
        waitingRecords,
        completed,
        error: "",
        retrying: Boolean(attempt) && !this.data.submitting,
        amount: attempt
          ? formatFen(attempt.amountFen)
          : this.amountEdited || this.data.submitting
            ? this.data.amount
            : wallet.defaultAmount,
      });
      if (
        this.visible &&
        selected?.id === this.autoReceiptID &&
        selected?.canConfirm
      ) {
        this.autoReceiptID = "";
        this.requestReceipt(selected);
      }
    } catch (error) {
      this.setData({ error: error.message });
    } finally {
      this.setData({ loading: false });
    }
  },

  changeAmount(event) {
    this.setAmount(event.detail.value);
  },

  finishAmountEdit() {
    const { amount, wallet, submitting, retrying } = this.data;
    if (!wallet?.canWithdraw || submitting || retrying || !amount) return;
    const amountError = withdrawalAmountError(amount, wallet);
    this.setData({
      amount: amountError ? amount : formatFen(yuanToFen(amount)),
      amountError,
    });
  },

  useAll() {
    if (this.data.wallet?.canWithdraw)
      this.setAmount(this.data.wallet.defaultAmount);
  },

  setAmount(amount) {
    const { retrying, submitting, wallet } = this.data;
    if (retrying || submitting || !wallet?.canWithdraw) return;
    this.amountEdited = true;
    this.setData({
      amount,
      amountError: amount ? withdrawalAmountError(amount, wallet) : "",
    });
  },

  async withdraw() {
    const { amount, submitting, wallet, retrying, error } = this.data;
    if (submitting || (!retrying && (!wallet?.canWithdraw || error))) return;
    if (!this.memberID || !isCurrentSession(this.memberToken)) {
      this.setData({ amountError: "登录状态已变化，请重新加载" });
      await this.load(false);
      return;
    }
    const amountError = retrying ? "" : withdrawalAmountError(amount, wallet);
    if (amountError) {
      this.setData({ amountError });
      return;
    }

    this.setData({ submitting: true, amountError: "" });
    try {
      if (wx.requestSubscribeMessage) await subscribe("withdrawal");
      const record = await submitWithdrawal(this.memberID, yuanToFen(amount));
      this.selectedID = record.id;
      this.autoReceiptID = record.id;
      this.amountEdited = false;
      this.setData({ retrying: false });
      if (record.status !== waiting) {
        wx.showToast({ title: "申请已提交", icon: "success" });
      }
      if (this.refreshing) await this.refreshing;
      await this.load(false);
      this.setData({ amount: this.data.wallet?.defaultAmount || "" });
    } catch (error) {
      const attempt = getWithdrawalAttempt(this.memberID);
      this.amountEdited = true;
      this.setData({ retrying: Boolean(attempt), amountError: error.message });
    } finally {
      this.setData({ submitting: false });
    }
  },

  confirmReceipt(event) {
    if (this.data.confirming) return;
    const record = this.data.waitingRecords.find(
      ({ id }) => id === event.currentTarget.dataset.id,
    );
    if (record) {
      this.selectedID = record.id;
      this.autoReceiptID = "";
      this.requestReceipt(record);
    }
  },

  async requestReceipt(record) {
    if (this.data.confirming) return;
    this.setData({ confirming: true });
    try {
      await requestWithdrawalReceipt(record);
    } catch {
      wx.showToast({ title: "微信收款页面打开失败，请重试", icon: "none" });
    } finally {
      this.setData({ confirming: false });
      if (this.refreshing) await this.refreshing;
      if (this.visible) await this.load(false);
    }
  },

  openRecords() {
    if (this.navigating) return;
    this.navigating = true;
    wx.navigateTo({
      url: "/subpackages/member/withdrawal-records/index",
      fail: () =>
        wx.showToast({ title: "提现记录打开失败，请重试", icon: "none" }),
      complete: () => {
        this.navigating = false;
      },
    });
  },

  goWork() {
    wx.switchTab({ url: "/pages/work/index" });
  },

  goWallet() {
    wx.switchTab({ url: "/pages/wallet/index" });
  },
});
