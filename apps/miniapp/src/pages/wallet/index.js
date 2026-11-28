const { request } = require("../../services/request");
const {
  hasSession,
  openLogin,
  pendingSession,
} = require("../../services/session");
const { presentWallet } = require("../../services/wallet-view");
const {
  activePlaybackRows,
  playbackWeeks,
} = require("../../services/work-view");

Page({
  data: {
    loading: true,
    signedIn: false,
    error: "",
    wallet: null,
    playback: null,
  },

  onShow() {
    return this.load();
  },

  onPullDownRefresh() {
    return this.load().finally(() => wx.stopPullDownRefresh());
  },

  async load() {
    if (this.refreshing) return this.refreshing;
    this.refreshing = this.loadRecords().finally(() => {
      this.refreshing = null;
    });
    return this.refreshing;
  },

  async loadRecords() {
    this.setData({ loading: true, error: "", wallet: null, playback: null });
    try {
      await pendingSession();
      const signedIn = hasSession();
      this.setData({ signedIn });
      if (!signedIn) return;

      const [wallet, assignments] = await Promise.all([
        request("/wallet"),
        request("/assignments"),
      ]);
      const rows = activePlaybackRows(playbackWeeks(assignments));
      const pending = rows.filter(({ pending }) => pending);
      const next = pending
        .filter(({ deadline, late }) => deadline && !late)
        .sort((a, b) => Date.parse(a.deadline) - Date.parse(b.deadline))[0];
      const scheduled = rows
        .flatMap(({ platforms }) => platforms)
        .filter(
          ({ screenshot, settled, settlementLabel }) =>
            screenshot && !settled && settlementLabel,
        )
        .map(({ settlementLabel }) => settlementLabel)
        .sort()[0];

      this.setData({
        wallet: presentWallet(wallet),
        playback: pending.length
          ? {
              pending: true,
              message: `还有 ${pending.length} 条待交播放数据${next ? `，最近截止 ${next.deadlineLabel}` : ""}`,
            }
          : scheduled
            ? {
                pending: false,
                message: `播放数据已提交，最近一笔 ${scheduled} 起可结算`,
              }
            : null,
      });
    } catch (error) {
      this.setData({ signedIn: hasSession(), error: error.message });
    } finally {
      this.setData({ loading: false });
    }
  },

  openLogin,

  openWithdrawals() {
    this.navigate("/subpackages/member/withdrawal/index");
  },

  openWithdrawalRecords() {
    this.navigate("/subpackages/member/withdrawal-records/index");
  },

  openDetails() {
    this.navigate("/subpackages/member/earnings/index");
  },

  openPlayback() {
    this.navigate("/subpackages/task/playback/index");
  },

  openUpcoming(event) {
    this.navigate(
      "/subpackages/member/wallet-upcoming/index?type=" +
        event.currentTarget.dataset.type,
    );
  },

  openTasks() {
    this.navigate("/subpackages/task/today/index");
  },

  navigate(url) {
    if (this.navigating) return;
    this.navigating = true;
    wx.navigateTo({
      url,
      fail: () => wx.showToast({ title: "页面打开失败，请重试", icon: "none" }),
      complete: () => {
        this.navigating = false;
      },
    });
  },
});
