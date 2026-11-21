const { request } = require("./request");
const { hasSession, pendingSession } = require("./session");
const { formatFen } = require("./money");
const { assignmentLabels, sourceLabels } = require("./labels");
const { AssignmentStatus } = require("./generated/enums");

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
  if (!hasSession()) {
    return { user: null, today: null, hasContent: false };
  }

  const [user, availability, assignments] = await Promise.all([
    request("/me"),
    request("/tasks/today"),
    request("/assignments"),
  ]);
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
  marketingBusinessDate,
  todaysAssignment,
  loadMarketingTasks,
  presentAssignment,
};
