const { request } = require("../../services/request");
const {
  hasSession,
  openLogin,
  pendingSession,
} = require("../../services/session");
const { EnrollmentStatus } = require("../../services/generated/enums");
const {
  activePlaybackRows,
  workCard,
  workTabs,
  playbackWeeks,
} = require("../../services/work-view");
const { taskDeadline } = require("../../services/task-progress");
const { scenes, subscribeTasks } = require("../../services/subscriptions");
const subscriptionTemplates = scenes.tasks;

Page({
  data: {
    loading: true,
    error: "",
    signedIn: false,
    showPlayback: false,
    showSubscription: false,
    pendingTasks: 0,
    playbackMessage: "",
    tasks: [],
    selected: "",
    tabs: workTabs([]),
  },

  onShow() {
    this.visible = true;
    this.checkSubscription();
    return this.load();
  },

  onHide() {
    this.visible = false;
    clearInterval(this.deadlineTimer);
  },

  onUnload() {
    this.onHide();
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
    clearInterval(this.deadlineTimer);
    this.setData({ loading: true, error: "" });
    try {
      await pendingSession();
      this.setData({ signedIn: hasSession() });
      if (!hasSession()) return;

      const [records, user] = await Promise.all([
        request("/assignments"),
        request("/me"),
      ]);
      this.records = [...records];
      const weeks = playbackWeeks(this.records);
      const next = activePlaybackRows(weeks)
        .filter(({ pending, deadline, late }) => pending && deadline && !late)
        .sort((a, b) => Date.parse(a.deadline) - Date.parse(b.deadline))[0];
      const playbackMessage = next
        ? "最近截止 " + next.deadlineLabel
        : weeks.pendingTasks
          ? "还有 " + weeks.pendingTasks + " 条待交播放数据"
          : "查看各任务的截图截止时间";

      this.setData({
        tabs: workTabs(this.records.map(workCard).filter(Boolean)),
        showPlayback:
          user.status === EnrollmentStatus.EnrollmentApproved &&
          (weeks.last.rows.length > 0 ||
            weeks.current.rows.length > 0 ||
            weeks.older.length > 0),
        pendingTasks: weeks.pendingTasks,
        playbackMessage,
      });
      this.filter();

      const hasActiveDeadline = () =>
        this.records.some((record) => {
          const deadline = taskDeadline(record);
          return deadline && deadline.remaining !== "等待系统确认";
        });
      if (this.visible && hasActiveDeadline()) {
        this.deadlineTimer = setInterval(() => {
          if (hasActiveDeadline()) this.filter();
          else this.load();
        }, 1000);
      }
    } catch (error) {
      if (hasSession()) this.setData({ error: error.message });
      else this.setData({ signedIn: false });
    } finally {
      if (!hasSession()) {
        this.records = [];
        this.setData({
          tasks: [],
          selected: "",
          tabs: workTabs([]),
          showPlayback: false,
        });
      }
      this.setData({ loading: false });
    }
  },

  checkSubscription() {
    if (!wx.getSetting) return;
    wx.getSetting({
      withSubscriptions: true,
      success: ({ subscriptionsSetting }) => {
        const { mainSwitch, itemSettings = {} } = subscriptionsSetting || {};
        this.setData({
          showSubscription:
            mainSwitch === false ||
            subscriptionTemplates.some((id) => itemSettings[id] === "reject"),
        });
      },
    });
  },

  async subscribe() {
    if (this.subscribing) return;
    this.subscribing = true;
    try {
      await subscribeTasks();
      this.checkSubscription();
    } finally {
      this.subscribing = false;
    }
  },

  openLogin,

  select(event) {
    this.setData({ selected: event.currentTarget.dataset.value });
    this.filter();
  },

  filter() {
    const { selected } = this.data;
    const tasks = (this.records || [])
      .map(workCard)
      .filter((task) => task && (!selected || task.status === selected));
    this.setData({ tasks });
  },

  openPlayback() {
    this.navigate("/subpackages/task/playback/index");
  },

  open(event) {
    this.navigate(
      "/subpackages/task/detail/index?id=" +
        encodeURIComponent(event.currentTarget.dataset.id),
    );
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

  goHome() {
    wx.switchTab({ url: "/pages/home/index" });
  },
});
