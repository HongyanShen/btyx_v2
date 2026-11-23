const { request } = require("./request");
const { isCurrentSession, pendingSession, tokenKey } = require("./session");
const { formatFen } = require("./money");
const { assignmentLabels, sourceLabels } = require("./labels");
const { AssignmentStatus } = require("./generated/enums");

const taskSnapshotMaxAgeMs = 30000;
let claimedAssignment;

function rememberClaimedAssignment(assignment, token) {
  claimedAssignment = { assignment, token, time: Date.now() };
}

function takeClaimedAssignment(id) {
  const claim = claimedAssignment;
  claimedAssignment = undefined;
  if (
    claim &&
    claim.assignment.id === id &&
    Date.now() - claim.time < taskSnapshotMaxAgeMs &&
    isCurrentSession(claim.token)
  ) {
    return claim.assignment;
  }
}

function marketingBusinessDate(now = Date.now()) {
  return new Date(now + 8 * 60 * 60 * 1000).toISOString().slice(0, 10);
}

function todaysAssignment(assignments, now = Date.now()) {
  const today = marketingBusinessDate(now);
  return assignments.find(
    (assignment) =>
      assignment.date === today &&
      assignment.status !== AssignmentStatus.AssignmentRevoked,
  );
}

async function loadMarketingTasks() {
  await pendingSession();
  const token = wx.getStorageSync(tokenKey);
  if (!token) {
    return { user: null, today: null, hasContent: false };
  }

  const [user, availability, assignments] = await Promise.all([
    request("/me"),
    request("/tasks/today"),
    request("/assignments"),
  ]);
  if (!isCurrentSession(token)) throw new Error("登录状态已变化，请重新加载");

  const today = todaysAssignment(assignments);

  return {
    user,
    today: today ? presentAssignment(today) : null,
    hasContent: availability.has_content,
    priceFen: availability.price_fen,
  };
}

function presentAssignment(assignment) {
  return {
    ...assignment,
    price: formatFen(assignment.price_fen),
    statusLabel: assignmentLabels[assignment.status],
    sourceLabel: sourceLabels[assignment.source],
  };
}

module.exports = {
  taskSnapshotMaxAgeMs,
  rememberClaimedAssignment,
  takeClaimedAssignment,
  marketingBusinessDate,
  todaysAssignment,
  loadMarketingTasks,
  presentAssignment,
};
