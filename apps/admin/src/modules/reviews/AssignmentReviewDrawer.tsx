import {
  Alert,
  App,
  Button,
  Card,
  Descriptions,
  Drawer,
  Input,
  Steps,
  Tag,
  Typography,
} from "antd";
import { post } from "../../api/client";
import { download } from "../../api/files";
import { useSession } from "../../app/Auth";
import { AssignmentSource, AssignmentStatus } from "../../generated/enums";
import type { Assignment, User } from "../../generated/models";
import { formatFen } from "../../shared/money";
import { platformLabels, sourceLabels } from "../../shared/status";
import { distributorName } from "../users/DistributorDialog";
import { SubmissionLink } from "./review-display";
import type { ReviewTarget } from "./ReviewDialog";
import { ReviewActions, SubmissionBadge } from "./ReviewTable";
import { useQueryClient } from "@tanstack/react-query";

export function AssignmentReviewDrawer({
  assignment,
  user,
  onClose,
  onReview,
}: {
  assignment: Assignment;
  user?: User;
  onClose: () => void;
  onReview: (target: ReviewTarget) => void;
}) {
  const { canCall } = useSession();
  const { message, modal } = App.useApp();
  const cache = useQueryClient();
  const canRevoke =
    canCall("POST /assignments/:id/revoke") &&
    assignment.source === AssignmentSource.SourceAssigned &&
    assignment.submissions.length === 0 &&
    assignment.status !== AssignmentStatus.AssignmentRevoked;

  function confirmRevoke() {
    let reason = "";
    modal.confirm({
      title: "撤回这次指派？",
      content: (
        <Input.TextArea
          aria-label="撤回原因"
          placeholder="撤回原因（必填）"
          onChange={(event) => {
            reason = event.target.value;
          }}
        />
      ),
      onOk: async () => {
        try {
          await post(`/assignments/${assignment.id}/revoke`, {
            reason: reason.trim(),
          });
          await Promise.all([
            cache.invalidateQueries({ queryKey: ["tasks"] }),
            cache.invalidateQueries({ queryKey: ["assignments"] }),
          ]);
          message.success("已撤回，内容与当天名额已释放");
        } catch (error) {
          if (error instanceof Error) message.error(error.message);
          throw error;
        }
      },
    });
  }

  return (
    <Drawer title={assignment.title} open size={760} onClose={onClose}>
      <Descriptions
        column={{ xs: 1, sm: 2 }}
        items={[
          {
            key: "user",
            label: "分发员",
            children: user ? distributorName(user) : "分发员",
          },
          {
            key: "source",
            label: "任务来源",
            children: sourceLabels[assignment.source],
          },
          {
            key: "price",
            label: "单平台奖励",
            children: `¥${formatFen(assignment.price_fen)}`,
          },
          { key: "due", label: "发布截止", children: assignment.due_date },
        ]}
      />
      <Steps
        size="small"
        className="task-steps"
        items={assignment.steps.map((step) => ({
          title: step.label,
          status: step.done ? "finish" : "wait",
        }))}
      />
      <Typography.Paragraph className="requirements">
        {assignment.body}
      </Typography.Paragraph>
      {assignment.platforms.map((platform) => {
        const submission = assignment.submissions.find(
          (item) => item.platform === platform,
        );
        return (
          <Card
            key={platform}
            size="small"
            title={platformLabels[platform]}
            className="platform-card"
            extra={submission && <SubmissionBadge submission={submission} />}
          >
            {!submission ? (
              <Typography.Text type="secondary">
                尚未提交发布链接
              </Typography.Text>
            ) : (
              <>
                <Typography.Paragraph>
                  <SubmissionLink
                    submission={submission}
                    label="打开发布作品"
                  />{" "}
                  <Typography.Text type="secondary">
                    · 第 {submission.revision} 次提交
                  </Typography.Text>
                </Typography.Paragraph>
                {submission.screenshot &&
                  canCall("GET /files/:id/download") && (
                    <Button
                      onClick={() =>
                        void download(submission.screenshot).catch(
                          (error: Error) => message.error(error.message),
                        )
                      }
                    >
                      查看播放量截图
                    </Button>
                  )}
                {!submission.screenshot && (
                  <Tag color="orange">待补播放数据</Tag>
                )}
                {submission.reason && (
                  <Alert
                    title={submission.reason}
                    type="warning"
                    className="form-alert"
                  />
                )}
                <div className="review-actions">
                  <ReviewActions
                    assignment={assignment}
                    submissions={[submission]}
                    onReview={onReview}
                  />
                </div>
              </>
            )}
          </Card>
        );
      })}
      {canRevoke && (
        <Button danger onClick={confirmRevoke}>
          撤回指派
        </Button>
      )}
    </Drawer>
  );
}
