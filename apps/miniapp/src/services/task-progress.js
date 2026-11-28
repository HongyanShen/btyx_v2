const {
  AssignmentStatus,
  SubmissionStatus,
  TaskStep,
} = require("./generated/enums");

function isClosed(assignment) {
  return (
    assignment.status === AssignmentStatus.AssignmentExpired ||
    assignment.status === AssignmentStatus.AssignmentRevoked
  );
}

function taskProgress(assignment) {
  const submissions = assignment.submissions || [];
  const completed = new Set(
    (assignment.steps || [])
      .filter((step) => step.done)
      .map((step) => step.key),
  );
  const rejected = submissions.some(
    (submission) => submission.status === SubmissionStatus.SubmissionRejected,
  );
  const linksDone = completed.has(TaskStep.StepLinks);
  const reviewDone =
    linksDone &&
    submissions.every(
      (submission) =>
        submission.status === SubmissionStatus.SubmissionApproved ||
        submission.status === SubmissionStatus.SubmissionWaived ||
        submission.status === SubmissionStatus.SubmissionInvalid,
    );
  const settled = completed.has(TaskStep.StepSettle);
  const notPayable =
    assignment.status === AssignmentStatus.AssignmentNotPayable;
  const closed = isClosed(assignment);
  const definitions = [
    ["拿内容", completed.has(TaskStep.StepContent), "先保存素材，查看发布要求"],
    ["发布", completed.has(TaskStep.StepPublish), "按要求到指定平台发布作品"],
    ["交链接", linksDone, "提交作品的发布链接"],
    ["审核", reviewDone, "你这边做完了，等运营审核"],
    ["结算", settled, "审核通过了，等结算入账"],
  ];
  const current = rejected ? 2 : definitions.findIndex(([, done]) => !done);
  const items = definitions.map(([label, done], index) => {
    let state = done ? "done" : "waiting";
    if (!closed && !notPayable && index === current) {
      state = rejected ? "rejected" : "current";
    }

    return { label, state, mark: state === "done" ? "✓" : index + 1 };
  });

  let hint = definitions[current]?.[2] || "已结算，奖励已进入余额";
  if (closed) {
    hint =
      assignment.status === AssignmentStatus.AssignmentExpired
        ? "任务已失效"
        : "工作人员已撤回任务";
  } else if (rejected) {
    hint = "按驳回原因修改，重新提交对应平台的链接";
  } else if (notPayable) {
    hint = "平台审核结束，本任务不予结算";
  }

  return { items, hint };
}

function taskDeadline(assignment, now = Date.now()) {
  const deadline = assignment.expires_at;
  if (
    isClosed(assignment) ||
    assignment.submissions?.some((submission) => submission.url) ||
    typeof deadline !== "string" ||
    !/^\d{4}-\d{2}-\d{2}T.*(?:Z|[+-]\d{2}:\d{2})$/.test(deadline)
  ) {
    return null;
  }

  const until = Date.parse(deadline);
  if (!Number.isFinite(until)) {
    return null;
  }

  const seconds = Math.max(0, Math.ceil((until - now) / 1000));
  const pad = (value) => String(value).padStart(2, "0");
  const time = new Date(until + 8 * 60 * 60 * 1000).toISOString();
  return {
    time: `${time.slice(5, 10)} ${time.slice(11, 16)}`,
    remaining: seconds
      ? [
          Math.floor(seconds / 3600),
          Math.floor((seconds % 3600) / 60),
          seconds % 60,
        ]
          .map(pad)
          .join(":")
      : "等待系统确认",
  };
}

function beijingDateTime(value) {
  const time = Date.parse(value);
  if (!Number.isFinite(time)) return "";
  const local = new Date(time + 8 * 60 * 60 * 1000).toISOString();
  return `${local.slice(0, 10)} ${local.slice(11, 16)}`;
}

module.exports = { isClosed, taskProgress, taskDeadline, beijingDateTime };
