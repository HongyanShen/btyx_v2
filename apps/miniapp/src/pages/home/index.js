const {
  canUseLocalDemo,
  localLogin,
  openLogin,
} = require("../../services/session");
const { loadMarketingTasks } = require("../../services/marketing-tasks");
const { formatFen } = require("../../services/money");
const { homeGuide, marketingTaskStatus } = require("./guide");
const config = require("../../services/config");

Page({
  data: {
    loading: true,
    error: "",
    hasContent: false,
    user: null,
    guide: homeGuide(null),
    today: null,
    referencePrice: formatFen(config.marketingReferencePriceFen),
    localDemo: false,
    entries: [
      { key: "ai", label: "AI选品带货", soon: true },
      { key: "video", label: "达人切片", soon: true },
      { key: "camera", label: "素人拍素材", soon: true },
      { key: "marketing", label: "营销分发", soon: false },
      { key: "ranking", label: "收益榜", soon: true },
    ],
  },

  onShow() {
    this.load();
  },

  async load() {
    if (this.refreshing) {
      return;
    }

    this.refreshing = true;
    this.setData({ loading: true, error: "" });

    try {
      const { user, hasContent, today, priceFen } = await loadMarketingTasks();
      this.setData({
        user,
        guide: homeGuide(user, today),
        today: today
          ? { ...today, cardStatus: marketingTaskStatus(today) }
          : null,
        localDemo: !user && canUseLocalDemo(),
        hasContent,
        referencePrice: formatFen(priceFen || config.marketingReferencePriceFen),
      });
    } catch (error) {
      this.setData({
        error: error.message,
        user: null,
        guide: homeGuide(null),
        today: null,
        hasContent: false,
        localDemo: canUseLocalDemo(),
      });
    } finally {
      this.refreshing = false;
      this.setData({ loading: false });
    }
  },

  async demoLogin() {
    try {
      await localLogin();
      await this.load();
    } catch (error) {
      this.setData({ error: error.message });
    }
  },

  goEnrollment() {
    wx.navigateTo({ url: "/subpackages/member/enrollment/index" });
  },

  goAgreement() {
    wx.navigateTo({ url: "/subpackages/member/agreement/index" });
  },

  onPullDownRefresh() {
    this.load().finally(() => wx.stopPullDownRefresh());
  },

  marketing() {
    wx.pageScrollTo({ selector: "#marketing", duration: 240 });
  },

  openTodayTask() {
    if (this.data.loading || this.data.guide.action !== "tasks") {
      return;
    }

    wx.navigateTo({
      url: "/subpackages/task/today/index",
      fail: () => wx.showToast({ title: "任务详情打开失败，请重试", icon: "none" }),
    });
  },

  openAssignment() {
    const { today } = this.data;
    if (!today) {
      return;
    }

    const section = today.cardStatus.section
      ? `&section=${today.cardStatus.section}`
      : "";
    wx.navigateTo({
      url: `/subpackages/task/detail/index?id=${today.id}${section}`,
    });
  },

  followGuide() {
    if (this.data.loading) {
      return;
    }

    if (this.data.error) {
      this.load();
      return;
    }

    switch (this.data.guide.action) {
      case "enrollment":
        this.goEnrollment();
        break;
      case "agreement":
        this.goAgreement();
        break;
      case "assignment":
        this.openAssignment();
        break;
      case "tasks":
        this.openTodayTask();
        break;
      case "blocked":
        this.marketing();
        break;
      case "login":
        openLogin();
        break;
    }
  },

  soon() {
    wx.showToast({ title: "即将上线，敬请期待", icon: "none" });
  },
});
