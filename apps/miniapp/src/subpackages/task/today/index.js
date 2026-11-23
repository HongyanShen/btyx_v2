const config = require("../../../services/config");
const { EnrollmentStatus } = require("../../../services/generated/enums");
const {
  loadMarketingTasks,
  marketingBusinessDate,
  rememberClaimedAssignment,
} = require("../../../services/marketing-tasks");
const { doubleFen, formatFen } = require("../../../services/money");
const { newKey, post } = require("../../../services/request");
const { isCurrentSession, openLogin, tokenKey } = require("../../../services/session");

function claimAction(user, ready, today, hasContent) {
  if (today) {
    return { action: "open", actionLabel: "查看今日任务" };
  }

  if (!user) {
    return { action: "login", actionLabel: "登录后领取" };
  }

  if (user.blocked) {
    return { action: "", actionLabel: "账号已暂停领取" };
  }

  if (!ready) {
    switch (user.status) {
      case EnrollmentStatus.EnrollmentApproved:
        return { action: "agreement", actionLabel: "去确认协议" };
      case EnrollmentStatus.EnrollmentPending:
        return { action: "enrollment", actionLabel: "查看报名审核" };
      case EnrollmentStatus.EnrollmentRejected:
        return { action: "enrollment", actionLabel: "重新报名" };
      default:
        return { action: "enrollment", actionLabel: "报名营销分发员" };
    }
  }

  return hasContent
    ? { action: "claim", actionLabel: "领取今日任务" }
    : { action: "", actionLabel: "暂无可领取任务" };
}

function displayPrices(priceFen) {
  return {
    price: formatFen(priceFen),
    twoPlatformPrice: formatFen(doubleFen(priceFen)),
  };
}

Page({
  data: {
    ...displayPrices(config.marketingReferencePriceFen),
    stages: ["拿内容和要求", "按要求发布", "交链接", "奖励记账"],
    loading: true,
    claiming: false,
    error: "",
    signedIn: false,
    ready: false,
    blocked: false,
    today: null,
    action: "",
    actionLabel: "",
  },

  onLoad() {
    const pages = getCurrentPages();
    const previous = pages[pages.length - 2];
    if (previous?.route === "pages/home/index") {
      this.initialOverview = previous.recentOverview();
    }
  },

  onShow() {
    if (!this.data.claiming) {
      return this.load();
    }
  },

  onUnload() {
    this.unloaded = true;
  },

  async load() {
    if (this.refreshing) {
      return;
    }

    this.refreshing = true;
    this.setData({ loading: true, error: "" });

    try {
      const overview = this.initialOverview || loadMarketingTasks();
      this.initialOverview = null;
      const { user, today, hasContent, priceFen } = await overview;
      if (this.unloaded) {
        return;
      }

      const ready =
        user?.status === EnrollmentStatus.EnrollmentApproved && !!user.consented;
      const offerPrice = today?.price_fen || priceFen || config.marketingReferencePriceFen;

      this.setData({
        ...displayPrices(offerPrice),
        ...claimAction(user, ready, today, hasContent),
        signedIn: !!user,
        ready,
        today,
        blocked: !!user?.blocked,
      });
    } catch (error) {
      if (!this.unloaded) {
        this.setData({ error: error.message });
      }
    } finally {
      this.refreshing = false;
      if (!this.unloaded) {
        this.setData({ loading: false });
      }
    }
  },

  followAction() {
    if (this.data.loading || this.data.claiming) {
      return;
    }

    if (this.data.error) {
      return this.load();
    }

    switch (this.data.action) {
      case "claim":
        return this.claimToday();
      case "open":
        return this.openAssignment(this.data.today.id);
      case "login":
        return openLogin();
      case "agreement":
        return wx.navigateTo({ url: "/subpackages/member/agreement/index" });
      case "enrollment":
        return wx.navigateTo({ url: "/subpackages/member/enrollment/index" });
    }
  },

  async claimToday() {
    const date = marketingBusinessDate();
    if (this.claimAttempt?.date !== date) {
      this.claimAttempt = { date, key: newKey() };
    }

    this.setData({ claiming: true });

    try {
      const token = wx.getStorageSync(tokenKey);
      const assignment = await post("/tasks/claim", {}, this.claimAttempt.key);
      if (this.unloaded) {
        return;
      }

      this.claimAttempt = null;
      if (!isCurrentSession(token)) {
        await this.load();
        return;
      }
      rememberClaimedAssignment(assignment, token);

      this.setData({
        ...displayPrices(assignment.price_fen),
        today: assignment,
        action: "open",
        actionLabel: "查看今日任务",
      });
      this.openAssignment(assignment.id);
    } catch (error) {
      if (!this.unloaded) {
        wx.showToast({ title: error.message, icon: "none" });
        await this.load();
      }
    } finally {
      if (!this.unloaded) {
        this.setData({ claiming: false });
      }
    }
  },

  openAssignment(id) {
    wx.redirectTo({
      url: `/subpackages/task/detail/index?id=${id}`,
      fail: () => wx.showToast({ title: "任务打开失败，请重试", icon: "none" }),
    });
  },
});
