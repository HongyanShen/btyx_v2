const { request, post, newKey } = require("../../../services/request");
const { formatFen, yuanToFen } = require("../../../services/money");
const { withdrawalLabels } = require("../../../services/labels");
const { WithdrawalStatus } = require("../../../services/generated/enums");

Page({
  data: {
    loading: true,
    error: "",
    wallet: null,
    withdrawals: [],
    amount: "",
    submitting: false,
  },

  onShow() {
    this.load();
    clearInterval(this.refreshTimer);
    this.refreshTimer = setInterval(() => this.load(false), 15000);
  },

  onHide() {
    clearInterval(this.refreshTimer);
  },

  onUnload() {
    clearInterval(this.refreshTimer);
  },

  async load(showLoading = true) {
    if (this.refreshing) {
      return;
    }

    this.refreshing = true;
    if (showLoading) {
      this.setData({ loading: true, error: "" });
    }

    try {
      const [wallet, withdrawals] = await Promise.all([
        request("/wallet"),
        request("/withdrawals"),
      ]);
      this.setData({
        error: "",
        wallet: {
          available: formatFen(wallet.available_fen),
          canWithdraw: !/^0+$/.test(wallet.available_fen),
        },
        withdrawals: withdrawals.map((record) => ({
          ...record,
          amount: formatFen(record.amount_fen),
          label: withdrawalLabels[record.status],
          time: new Date(record.created_at).toLocaleString("zh-CN", {
            timeZone: "Asia/Shanghai",
          }),
          canConfirm:
            record.status === WithdrawalStatus.WithdrawalWaitingConfirmation &&
            !!record.package_info,
        })),
      });
    } catch (error) {
      this.setData({ error: error.message });
    } finally {
      this.refreshing = false;
      this.setData({ loading: false });
    }
  },

  changeAmount(event) {
    this.setData({ amount: event.detail.value });
    this.withdrawalKey = undefined;
  },

  async withdraw() {
    if (this.data.submitting || !this.data.wallet?.canWithdraw) {
      return;
    }

    this.setData({ submitting: true });

    try {
      const amount = yuanToFen(this.data.amount);
      if (amount === "0") {
        throw new Error("请输入大于零的提现金额");
      }
      // Reuse the key after a timeout; a retry must not freeze money twice.
      this.withdrawalKey = this.withdrawalKey || newKey();
      const record = await post(
        "/withdrawals",
        { amount_fen: amount },
        this.withdrawalKey,
      );
      this.withdrawalKey = undefined;
      this.setData({ amount: "" });
      if (
        record.status === WithdrawalStatus.WithdrawalWaitingConfirmation &&
        record.package_info
      ) {
        this.requestReceipt(record);
      } else {
        wx.showToast({ title: "申请已提交", icon: "success" });
      }
      await this.load(false);
    } catch (error) {
      wx.showToast({ title: error.message, icon: "none" });
    } finally {
      this.setData({ submitting: false });
    }
  },

  confirmReceipt(event) {
    const record = this.data.withdrawals.find(
      (item) => item.id === event.currentTarget.dataset.id,
    );
    if (record?.canConfirm) {
      this.requestReceipt(record);
    }
  },

  requestReceipt(record) {
    if (this.confirming) {
      return;
    }

    if (!wx.canIUse("requestMerchantTransfer")) {
      wx.showToast({ title: "请升级微信后重试", icon: "none" });

      return;
    }

    this.confirming = true;
    wx.requestMerchantTransfer({
      mchId: record.merchant_id,
      appId: wx.getAccountInfoSync().miniProgram.appId,
      package: record.package_info,
      // Both outcomes only refresh; the server confirms payment by signed notification/query.
      complete: () => {
        this.confirming = false;
        this.load(false);
      },
    });
  },
});
