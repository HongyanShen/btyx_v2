const {
  AssignmentSource,
  AssignmentStatus,
  SettlementFrequency,
  SubmissionStatus,
} = require("./generated/enums");
const { platformLabels, sourceLabels } = require("./labels");
const { marketingBusinessDate } = require("./marketing-tasks");
const { beijingDateTime, isClosed, taskDeadline } = require("./task-progress");

const statusDetails = {
  ready: ["待执行", "brand", "去做任务"],
  [AssignmentStatus.AssignmentInProgress]: ["进行中", "brand", "继续"],
  [AssignmentStatus.AssignmentPendingReview]: ["待审核", "warning", "等待审核"],
  [AssignmentStatus.AssignmentPendingSettlement]: [
    "已通过",
    "success",
    "等结算",
  ],
  [AssignmentStatus.AssignmentRejected]: ["已驳回", "danger", "去修改"],
  [AssignmentStatus.AssignmentNotPayable]: ["不予结算", "neutral", "已结束"],
  [AssignmentStatus.AssignmentSettled]: ["已结算", "neutral", "已完成"],
  [AssignmentStatus.AssignmentExpired]: ["已失效", "neutral", "已结束"],
  playback: ["待交数据", "warning", "去交数据"],
};

function platformText(platforms) {
  const names = platforms.map((platform) => platformLabels[platform]);
  return names.length === 1 ? "只发" + names[0] : names.join(" + ");
}

function isPlaybackReviewed(status) {
  return (
    status === SubmissionStatus.SubmissionApproved ||
    status === SubmissionStatus.SubmissionWaived
  );
}

function playbackPlatforms(task) {
  if (isClosed(task) || task.status === AssignmentStatus.AssignmentNotPayable)
    return [];
  return (task.submissions || []).filter(
    (submission) =>
      submission.url &&
      isPlaybackReviewed(submission.status) &&
      (!submission.settled || submission.screenshot),
  );
}

function workCard(task) {
  if (task.status === AssignmentStatus.AssignmentRevoked) return null;

  const submissions = task.submissions || [];
  const links = submissions.filter(
    ({ url, status }) => url && status !== SubmissionStatus.SubmissionInvalid,
  );
  const needsScreenshot = playbackPlatforms(task).some(
    ({ screenshot }) => !screenshot,
  );
  let status = task.status;
  if (
    status === AssignmentStatus.AssignmentPendingSettlement &&
    needsScreenshot
  ) {
    status = "playback";
  }

  const [statusLabel, tone, action] = statusDetails[status];
  const sourceLabel =
    task.source === AssignmentSource.SourceClaim
      ? ""
      : sourceLabels[task.source];
  let footer = [sourceLabel || "领取于", task.date]
    .filter(Boolean)
    .join(sourceLabel ? " · " : " ");
  if (status === AssignmentStatus.AssignmentPendingReview) {
    footer = "已交 " + links.length + " 条链接";
  } else if (status === AssignmentStatus.AssignmentRejected) {
    const reasons = submissions
      .filter(
        ({ reason, status }) =>
          reason && status === SubmissionStatus.SubmissionRejected,
      )
      .map(({ platform, reason }) => platformLabels[platform] + " · " + reason);
    footer = reasons.length
      ? "被驳回：" + reasons.join("\n")
      : "按审核反馈修改后重新提交";
  } else if (status === AssignmentStatus.AssignmentExpired) {
    footer = "超过截止时间未交链接，任务已失效";
  } else if (status === AssignmentStatus.AssignmentNotPayable) {
    footer = "平台审核结束，本任务不予结算";
  }

  return {
    id: task.id,
    title: "今日营销分发",
    platformText: platformText(task.platforms),
    sourceLabel,
    reissued: task.source === AssignmentSource.SourceReissued,
    status,
    statusLabel,
    tone,
    action,
    footer,
    playback: status === "playback",
    quietAction: ["待审核", "已通过", "已结算", "不予结算", "已失效"].includes(
      statusLabel,
    ),
    deadline: workDeadline(task),
    dueDate:
      status === AssignmentStatus.AssignmentExpired ? "已过" : task.due_date,
  };
}

function workDeadline(task) {
  const deadline = taskDeadline(task);
  if (!deadline) return null;

  const expiresToday =
    marketingBusinessDate(Date.parse(task.expires_at)) ===
    marketingBusinessDate();
  return {
    ...deadline,
    time: expiresToday ? "今天 " + deadline.time.slice(6) : deadline.time,
  };
}

function workTabs(cards) {
  const counts = {};
  for (const { status } of cards) counts[status] = (counts[status] || 0) + 1;
  return [
    { value: "", label: "全部", count: cards.length },
    ...Object.entries(statusDetails).map(([value, [label]]) => ({
      value,
      label,
      count: counts[value] || 0,
    })),
  ];
}

const dayMs = 24 * 60 * 60 * 1000;

function dateKey(time) {
  return new Date(time).toISOString().slice(0, 10);
}

function shortDate(date) {
  return date.slice(5).replace("-", "/");
}

function playbackWeeks(tasks, now = Date.now()) {
  const today = new Date(marketingBusinessDate(now) + "T00:00:00Z");
  const monday = today.getTime() - ((today.getUTCDay() + 6) % 7) * dayMs;
  const lastMonday = dateKey(monday - 7 * dayMs);
  const thisMonday = dateKey(monday);
  const last = {
    title:
      "上周 " +
      shortDate(lastMonday) +
      "–" +
      shortDate(dateKey(monday - dayMs)),
    rows: [],
  };
  const current = {
    title:
      "本周 " +
      shortDate(thisMonday) +
      "–" +
      shortDate(dateKey(monday + 6 * dayMs)),
    rows: [],
  };
  const older = [];
  let pendingTasks = 0;
  let overdueTasks = 0;

  for (const task of tasks) {
    const submissions = playbackPlatforms(task);
    if (!submissions.length || !task.date) continue;
    const platforms = Array.from(submissions, (submission) => ({
      key: submission.platform,
      label: platformLabels[submission.platform],
      screenshot: submission.screenshot || "",
      settled: submission.settled,
      url: submission.url,
      settlementLabel: beijingDateTime(submission.settlement_eligible_at),
    }));
    const pending = platforms.some(({ screenshot }) => !screenshot);
    const [publishedOn] = submissions
      .map(({ published_on }) => published_on)
      .filter(Boolean)
      .sort();

    const row = {
      id: task.id,
      title: task.title,
      dateLabel: publishedOn ? shortDate(publishedOn) + " 发布" : "",
      platformText: platformText(platforms.map(({ key }) => key)),
      platforms,
      pending,
      deadline: task.playback_deadline || "",
      deadlineLabel: beijingDateTime(task.playback_deadline),
      nextSettlement: beijingDateTime(task.playback_settlement_at),
      late: Date.parse(task.playback_deadline) <= now,
      canSubmit:
        !!task.settlement_frequency &&
        task.settlement_frequency !== SettlementFrequency.SettlementWeekly,
    };
    if (task.date >= thisMonday) {
      current.rows.push(row);
      if (row.canSubmit && pending) pendingTasks += 1;
    } else if (task.date >= lastMonday) {
      last.rows.push(row);
      if (pending) pendingTasks += 1;
    } else if (pending) {
      older.push(row);
      overdueTasks += 1;
    }
  }

  return {
    last,
    current,
    older,
    pendingTasks: pendingTasks + overdueTasks,
  };
}

function activePlaybackRows(weeks) {
  return [
    ...weeks.last.rows,
    ...weeks.older,
    ...weeks.current.rows.filter(({ canSubmit }) => canSubmit),
  ];
}

module.exports = {
  isPlaybackReviewed,
  workCard,
  workTabs,
  playbackWeeks,
  activePlaybackRows,
};
