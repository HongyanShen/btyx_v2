const {
  AssignmentSource,
  AssignmentStatus,
  EnrollmentStatus,
  SubmissionStatus,
} = require("../../services/generated/enums");
const { platformLabels } = require("../../services/labels");
const { taskProgress } = require("../../services/task-progress");

function homeGuide(user, assignment) {
  const enrolled = user?.status === EnrollmentStatus.EnrollmentApproved;
  const ready = enrolled && user.consented;
  const steps = [
    { label: "报名分发员", done: enrolled },
    { label: "领今日任务", done: !!assignment },
    {
      label: "发完交链接",
      labelEnd: "拿奖励",
      done: !!assignment?.submissions?.some((submission) => submission.settled),
    },
  ];

  let action = "enrollment";
  let label = "第一步：报名营销分发员";

  if (!user) {
    action = "login";
    label = "登录后报名营销分发员";
  } else if (user.blocked) {
    action = "blocked";
    label = "账号已暂停领取";
  } else if (user.status === EnrollmentStatus.EnrollmentPending) {
    label = "报名审核中，去看看";
  } else if (user.status === EnrollmentStatus.EnrollmentRejected) {
    label = "报名没通过，查看原因";
  } else if (enrolled && !ready) {
    action = "agreement";
    label = "去同意《营销分发任务协议》";
  } else if (assignment) {
    action = "assignment";
    label = "第三步：把任务做完，交链接";
    if (assignment.status === AssignmentStatus.AssignmentExpired) {
      label = "今日任务已失效，查看记录";
    } else if (assignment.submissions?.some((submission) => submission.url)) {
      label = "查看今日任务与审核进度";
    }
  } else if (ready) {
    action = "tasks";
    label = "第二步：领今日任务";
  }

  return { action, label, steps };
}

function marketingTaskStatus(assignment) {
  const source =
    {
      [AssignmentSource.SourceAssigned]: "被工作人员指派",
      [AssignmentSource.SourceReissued]: "被重新派发",
    }[assignment.source] || "已领取";
  const status = assignment.status;

  switch (status) {
    case AssignmentStatus.AssignmentExpired:
      return { message: "已失效 · 超时未提交链接", button: "查看" };
    case AssignmentStatus.AssignmentSettled:
      return { message: "已结算 · 奖励已入账", button: "查看" };
    case AssignmentStatus.AssignmentNotPayable:
      return { message: "审核结束 · 不予结算", button: "查看" };
    case AssignmentStatus.AssignmentRejected:
      return { message: `${source} · 被驳回，要修改`, button: "去修改" };
    case AssignmentStatus.AssignmentPendingSettlement: {
      const needsScreenshot = assignment.submissions?.some(
        (submission) =>
          !submission.settled &&
          !submission.screenshot &&
          (submission.status === SubmissionStatus.SubmissionApproved ||
            submission.status === SubmissionStatus.SubmissionWaived),
      );
      if (needsScreenshot) {
        return {
          message: `${source} · 待交播放数据`,
          button: "去交数据",
          section: "screenshots",
        };
      }

      return { message: `${source} · 审核通过，等结算`, button: "查看" };
    }
  }

  const { items } = taskProgress(assignment);
  if (
    status === AssignmentStatus.AssignmentPendingReview &&
    items[2].state === "done"
  ) {
    return { message: `${source} · 提交了，审核中`, button: "看审核进度" };
  }

  const step = items.findIndex((item) => item.state === "current");
  const publishTo = assignment.platforms
    ?.map((platform) => platformLabels[platform])
    .join("和");
  const actions = [
    "拿内容和要求",
    publishTo ? `发${publishTo}` : "按要求发布",
    "交发布链接",
  ];
  if (step >= 0 && step < actions.length) {
    return {
      message: `${source} · 第 ${step + 1}/4 步 ${actions[step]}`,
      button: "继续做",
    };
  }

  return { message: `${source} · 查看任务进度`, button: "查看" };
}

module.exports = { homeGuide, marketingTaskStatus };
