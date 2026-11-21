import { useId, useState, type FormEvent } from "react";
import { Alert, Button, Checkbox, Input, Modal, Radio } from "antd";
import type { Platform } from "../../generated/enums";
import type { Assignment, Submission, User } from "../../generated/models";
import { platformLabels } from "../../shared/status";
import { distributorName } from "../users/DistributorDialog";
import { reviewDisplayID } from "./review-display";

export interface ReviewTarget {
  assignment: Assignment;
  submissions: Submission[];
  action: "approve" | "reject" | "flag";
}

export interface ReviewDecision {
  target: ReviewTarget;
  submissions: [Submission, ...Submission[]];
  reason: string;
  keepPay: boolean;
}

const titles = {
  approve: "通过发布链接",
  reject: "驳回发布链接",
  flag: "标记违规 / 限流",
};

const confirmations = {
  approve: "确认通过",
  reject: "确认驳回",
  flag: "确认处理",
};

export function ReviewDialog({
  target,
  user,
  pending,
  error,
  onConfirm,
  onClose,
}: {
  target: ReviewTarget;
  user?: User;
  pending: boolean;
  error: Error | null;
  onConfirm: (decision: ReviewDecision) => void;
  onClose: () => void;
}) {
  const formID = useId();
  const singlePlatform = target.submissions.length === 1;
  const [platforms, setPlatforms] = useState<Platform[]>(
    singlePlatform ? [target.submissions[0].platform] : [],
  );
  const [reason, setReason] = useState("");
  const [keepPay, setKeepPay] = useState(false);
  const [attempted, setAttempted] = useState(false);
  const [submission, ...otherSubmissions] = target.submissions.filter((item) =>
    platforms.includes(item.platform),
  );
  const rejecting = target.action === "reject";
  const flagging = target.action === "flag";
  const reasonRequired = rejecting || flagging;
  const platformInvalid = attempted && !submission;
  const reasonInvalid = attempted && reasonRequired && !reason.trim();

  function confirm(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setAttempted(true);

    if (pending || !submission || (reasonRequired && !reason.trim())) {
      return;
    }

    onConfirm({
      target,
      submissions: [submission, ...otherSubmissions],
      reason: reason.trim(),
      keepPay,
    });
  }

  return (
    <Modal
      open
      centered
      closable={false}
      keyboard={!pending}
      mask={{ closable: !pending }}
      width={420}
      className="publish-review-modal"
      title={titles[target.action]}
      onCancel={pending ? undefined : onClose}
      footer={[
        <Button
          key="cancel"
          autoInsertSpace={false}
          disabled={pending}
          onClick={onClose}
        >
          取消
        </Button>,
        <Button
          key="confirm"
          type="primary"
          danger={rejecting}
          loading={pending}
          htmlType="submit"
          form={formID}
        >
          {confirmations[target.action]}
        </Button>,
      ]}
    >
      <p className="publish-review-context">
        {reviewDisplayID(target.assignment.id)} · {target.assignment.title} ·{" "}
        {user ? distributorName(user) : "分发员"}
      </p>
      <form id={formID} className="publish-review-form" onSubmit={confirm}>
        {rejecting ? (
          <div
            className="publish-review-platform"
            role="group"
            aria-labelledby={`${formID}-platform`}
            aria-describedby={
              platformInvalid ? `${formID}-platform-error` : undefined
            }
          >
            <span id={`${formID}-platform`}>发布平台</span>
            <div className="publish-review-platform-options">
              {target.submissions.map((item, index) => (
                <Checkbox
                  key={item.platform}
                  checked={platforms.includes(item.platform)}
                  autoFocus={!singlePlatform && index === 0}
                  disabled={pending || singlePlatform}
                  onChange={(event) =>
                    setPlatforms((selected) =>
                      event.target.checked
                        ? [...selected, item.platform]
                        : selected.filter((value) => value !== item.platform),
                    )
                  }
                >
                  {platformLabels[item.platform]}
                </Checkbox>
              ))}
            </div>
          </div>
        ) : (
          <div className="publish-review-platform">
            <label htmlFor={`${formID}-platform`}>发布平台</label>
            <select
              id={`${formID}-platform`}
              value={platforms[0] ?? ""}
              autoFocus={!singlePlatform}
              disabled={pending || singlePlatform}
              aria-invalid={platformInvalid}
              aria-describedby={
                platformInvalid ? `${formID}-platform-error` : undefined
              }
              onChange={(event) => {
                const selected = target.submissions.find(
                  (item) => item.platform === event.target.value,
                );
                setPlatforms(selected ? [selected.platform] : []);
              }}
            >
              <option value="" disabled>
                请选择平台
              </option>
              {target.submissions.map((item) => (
                <option key={item.platform} value={item.platform}>
                  {platformLabels[item.platform]}
                </option>
              ))}
            </select>
          </div>
        )}
        {platformInvalid && (
          <div
            role="alert"
            id={`${formID}-platform-error`}
            className="publish-review-error"
          >
            请选择发布平台
          </div>
        )}
        {flagging && (
          <>
            <Radio.Group
              aria-label="处理方式"
              value={keepPay ? "keep" : "reissue"}
              disabled={pending}
              onChange={(event) => setKeepPay(event.target.value === "keep")}
              options={[
                { value: "reissue", label: "重新派发" },
                { value: "keep", label: "素材问题，照常结算" },
              ]}
            />
            <div className="review-note">
              {keepPay
                ? "记录素材问题，所选平台照常结算。"
                : "所选平台停止结算，新任务采用派发时的版本和价格。"}
            </div>
          </>
        )}
        <Input
          autoFocus={singlePlatform}
          aria-label={rejecting ? "驳回原因" : "原因或备注"}
          aria-describedby={
            reasonInvalid ? `${formID}-reason-error` : undefined
          }
          placeholder={
            rejecting
              ? "驳回原因，分发员会在小程序里看到"
              : reasonRequired
                ? "填写处理原因"
                : "审核备注（选填）"
          }
          value={reason}
          disabled={pending}
          status={reasonInvalid ? "error" : undefined}
          onChange={(event) => setReason(event.target.value)}
        />
        {reasonInvalid && (
          <div
            role="alert"
            id={`${formID}-reason-error`}
            className="publish-review-error"
          >
            {rejecting ? "请填写驳回原因" : "请填写处理原因"}
          </div>
        )}
        {error && <Alert type="error" title={error.message} showIcon />}
      </form>
    </Modal>
  );
}
