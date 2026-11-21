import { Button, Tag } from "antd";
import { useSession } from "../../app/Auth";
import {
  AssignmentSource,
  AssignmentStatus,
  SubmissionStatus,
} from "../../generated/enums";
import type { Assignment, Submission, User } from "../../generated/models";
import {
  assignmentLabels,
  platformLabels,
  sourceLabels,
  submissionLabels,
} from "../../shared/status";
import { distributorName } from "../users/DistributorDialog";
import type { ReviewTarget } from "./ReviewDialog";
import {
  reviewDisplayID,
  SubmissionLink,
  SubmissionTime,
} from "./review-display";

const columns = [
  "ID",
  "类型",
  "任务",
  "分发员",
  "链接",
  "提交",
  "状态",
  "操作",
];

export function ReviewActions({
  assignment,
  submissions,
  onReview,
}: {
  assignment: Assignment;
  submissions: Submission[];
  onReview: (target: ReviewTarget) => void;
}) {
  const { canCall } = useSession();
  const pending = submissions.filter(
    (item) => item.status === SubmissionStatus.SubmissionPending,
  );
  const flaggable = submissions.filter(
    (item) =>
      item.status === SubmissionStatus.SubmissionApproved && !item.settled,
  );

  return (
    <div className="publish-review-actions">
      {pending.length > 0 && canCall("POST /assignments/:id/review") && (
        <>
          <Button
            size="small"
            type="primary"
            autoInsertSpace={false}
            onClick={() =>
              onReview({ assignment, submissions: pending, action: "approve" })
            }
          >
            通过
          </Button>
          <Button
            size="small"
            type="primary"
            danger
            autoInsertSpace={false}
            onClick={() =>
              onReview({ assignment, submissions: pending, action: "reject" })
            }
          >
            驳回
          </Button>
        </>
      )}
      {flaggable.length > 0 && canCall("POST /assignments/:id/reissue") && (
        <Button
          size="small"
          className="publish-flag-button"
          onClick={() =>
            onReview({ assignment, submissions: flaggable, action: "flag" })
          }
        >
          违规 / 限流
        </Button>
      )}
    </div>
  );
}

export function SubmissionBadge({ submission }: { submission: Submission }) {
  const { status, settled } = submission;
  const color =
    settled || status === SubmissionStatus.SubmissionPending
      ? "waiting"
      : status === SubmissionStatus.SubmissionApproved ||
          status === SubmissionStatus.SubmissionWaived
        ? "passed"
        : "rejected";

  return (
    <Tag className={`publish-review-badge is-${color}`}>
      {settled ? "已结算" : submissionLabels[status]}
    </Tag>
  );
}

function ReviewStatus({ assignment }: { assignment: Assignment }) {
  const { status, submissions } = assignment;
  const [first] = submissions;
  if (!first)
    return (
      <Tag className="publish-review-badge">{assignmentLabels[status]}</Tag>
    );

  const uniform = submissions.every(
    (item) =>
      item.status === first.status &&
      item.settled === first.settled &&
      item.reason === first.reason,
  );
  return (
    <div className="publish-review-lines">
      {uniform ? (
        <div>
          <SubmissionBadge submission={first} />
          {first.reason && (
            <div className="review-note">原因：{first.reason}</div>
          )}
        </div>
      ) : (
        submissions.map((item) => (
          <div key={item.id}>
            {submissions.length > 1 && (
              <span className="publish-review-platform-name">
                {platformLabels[item.platform]}
              </span>
            )}
            <SubmissionBadge submission={item} />
            {item.reason && (
              <div className="review-note">原因：{item.reason}</div>
            )}
          </div>
        ))
      )}
      {status === AssignmentStatus.AssignmentPendingSettlement && (
        <span className="review-note">待结算</span>
      )}
    </div>
  );
}

export function ReviewTable({
  rows,
  users,
  onOpenAssignment,
  onOpenDistributor,
  onReview,
}: {
  rows: Assignment[];
  users: ReadonlyMap<string, User>;
  onOpenAssignment: (id: string) => void;
  onOpenDistributor: (user: User) => void;
  onReview: (target: ReviewTarget) => void;
}) {
  return (
    <div
      className="marketing-table-scroll"
      tabIndex={0}
      role="region"
      aria-label="发布审核列表，可横向滚动"
    >
      <table
        className="marketing-table publish-review-table"
        aria-label="发布链接审核"
      >
        <colgroup>
          {columns.map((label) => (
            <col key={label} />
          ))}
        </colgroup>
        <thead>
          <tr>
            {columns.map((label) => (
              <th scope="col" key={label}>
                {label}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((assignment) => {
            const user = users.get(assignment.user_id);
            return (
              <tr key={assignment.id}>
                <td className="marketing-id" title={assignment.id}>
                  {reviewDisplayID(assignment.id)}
                </td>
                <td>
                  <Tag className="platform-tag publish-review-badge">
                    营销分发
                  </Tag>
                </td>
                <td>
                  <button
                    type="button"
                    className="publish-review-task"
                    onClick={() => onOpenAssignment(assignment.id)}
                  >
                    {assignment.title}
                  </button>
                  {assignment.source !== AssignmentSource.SourceClaim && (
                    <div className="review-note">
                      {sourceLabels[assignment.source]}
                    </div>
                  )}
                </td>
                <td>
                  {user ? (
                    <>
                      <button
                        type="button"
                        className="publish-review-person"
                        onClick={() => onOpenDistributor(user)}
                      >
                        {distributorName(user)}
                      </button>
                      <div className="review-note">
                        {user.nickname || "微信用户"} ·{" "}
                        {user.wechat ? `微信号 ${user.wechat}` : "微信号未填"}
                      </div>
                    </>
                  ) : (
                    <span className="review-note">分发员</span>
                  )}
                </td>
                <td>
                  {assignment.submissions.length ? (
                    assignment.submissions.map((item) => (
                      <div key={item.id} className="publish-review-link">
                        {platformLabels[item.platform]}{" "}
                        <SubmissionLink submission={item} />
                      </div>
                    ))
                  ) : (
                    <span className="review-note">尚未提交</span>
                  )}
                </td>
                <td>
                  <SubmissionTime submissions={assignment.submissions} />
                </td>
                <td>
                  <ReviewStatus assignment={assignment} />
                </td>
                <td>
                  <ReviewActions
                    assignment={assignment}
                    submissions={assignment.submissions}
                    onReview={onReview}
                  />
                </td>
              </tr>
            );
          })}
          {rows.length === 0 && (
            <tr>
              <td colSpan={columns.length} className="marketing-empty-cell">
                没有符合筛选的审核记录
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
}
