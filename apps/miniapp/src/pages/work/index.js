const { request } = require("../../services/request");
const {
  hasSession,
  openLogin,
  pendingSession,
} = require("../../services/session");
const {
  assignmentLabels,
  platformLabels,
  sourceLabels,
} = require("../../services/labels");
const {
  AssignmentSource,
  AssignmentStatus,
  SubmissionStatus,
} = require("../../services/generated/enums");
const {
  isClosed,
  taskDeadline,
  taskProgress,
} = require("../../services/task-progress");

Page({
  data: {
    loading: true,
    error: "",
    signedIn: false,
    tasks: [],
    needsScreenshots: false,
    screenshotCount: 0,
    selected: "",
    tabs: [
      { value: "", label: "全部" },
      { value: AssignmentStatus.AssignmentInProgress, label: "进行中" },
      { value: AssignmentStatus.AssignmentPendingReview, label: "待审核" },
      { value: AssignmentStatus.AssignmentRejected, label: "需修改" },
      { value: AssignmentStatus.AssignmentPendingSettlement, label: "待结算" },
      { value: AssignmentStatus.AssignmentSettled, label: "已结算" },
      { value: AssignmentStatus.AssignmentExpired, label: "已失效" },
      { value: AssignmentStatus.AssignmentNotPayable, label: "不予结算" },
    ],
  },

  onShow() {
    this.visible = true;
    this.load();
  },

  onHide() {
    this.visible = false;
    clearInterval(this.deadlineTimer);
  },

  onUnload() {
    this.onHide();
  },

  async load() {
    clearInterval(this.deadlineTimer);
    this.setData({ loading: true, error: "" });

    await pendingSession();
    const signedIn = hasSession();
    this.setData({ loading: signedIn, signedIn });
    if (!signedIn) {
      this.clearRecords();

      return;
    }

    try {
      const tasks = await request("/assignments");
      this.records = tasks
        .filter((task) => task.status !== AssignmentStatus.AssignmentRevoked)
        .map((task) => ({
          ...task,
          statusLabel: assignmentLabels[task.status],
          sourceLabel: sourceLabels[task.source],
          showSource: task.source !== AssignmentSource.SourceClaim,
          reissued: task.source === AssignmentSource.SourceReissued,
          platformText: task.platforms
            .map((platform) => platformLabels[platform])
            .join(" + "),
          progressHint: taskProgress(task).hint,
          notice: task.submissions
            .filter(
              (submission) =>
                submission.reason &&
                submission.status === SubmissionStatus.SubmissionRejected,
            )
            .map(
              (submission) =>
                `${platformLabels[submission.platform]} · ${submission.reason}`,
            )
            .join("\n"),
          needsScreenshot:
            !isClosed(task) &&
            task.submissions.some(
              (submission) =>
                submission.url &&
                !submission.screenshot &&
                !submission.settled &&
                submission.status !== SubmissionStatus.SubmissionInvalid &&
                submission.status !== SubmissionStatus.SubmissionRejected,
            ),
        }));
      this.setData({
        screenshotCount: this.records.filter((task) => task.needsScreenshot)
          .length,
      });
      this.filter();
      if (
        this.visible &&
        this.records.some((task) => this.hasActiveDeadline(task))
      ) {
        this.deadlineTimer = setInterval(() => {
          if (this.records.some((task) => this.hasActiveDeadline(task))) {
            this.filter();
          } else {
            this.load();
          }
        }, 1000);
      }
    } catch (error) {
      if (hasSession()) {
        this.setData({ error: error.message });
      } else {
        this.clearRecords();
      }
    } finally {
      this.setData({ loading: false });
    }
  },

  openLogin,

  clearRecords() {
    this.records = [];
    this.setData({ signedIn: false, tasks: [], screenshotCount: 0 });
  },

  hasActiveDeadline(task) {
    const deadline = taskDeadline(task);
    return !!deadline && deadline.remaining !== "等待系统确认";
  },

  select(event) {
    this.setData({
      selected: event.currentTarget.dataset.value,
      needsScreenshots: false,
    });
    this.filter();
  },

  showScreenshots() {
    this.setData({
      selected: "",
      needsScreenshots: !this.data.needsScreenshots,
    });
    this.filter();
  },

  filter() {
    const { selected, needsScreenshots } = this.data;
    this.setData({
      tasks: (this.records || [])
        .filter((task) =>
          needsScreenshots
            ? task.needsScreenshot
            : !selected || task.status === selected,
        )
        .map((task) => ({
          id: task.id,
          title: task.title,
          platformText: task.platformText,
          statusLabel: task.statusLabel,
          showSource: task.showSource,
          reissued: task.reissued,
          sourceLabel: task.sourceLabel,
          notice: task.notice,
          progressHint: task.progressHint,
          deadline: taskDeadline(task),
        })),
    });
  },

  open(event) {
    const screenshot = this.data.needsScreenshots ? "&section=screenshots" : "";
    wx.navigateTo({
      url: `/subpackages/task/detail/index?id=${event.currentTarget.dataset.id}${screenshot}`,
    });
  },
});
