import { Button, Modal, Space, Tag } from "antd";
import type { ReactNode } from "react";
import { AttachmentPreviews } from "../../components/AttachmentPreviews";
import { AssignmentSource } from "../../generated/enums";
import type { TaskVersion, User } from "../../generated/models";
import {
  marketingBody,
  marketingDate,
  marketingDisplayID,
  MarketingProgress,
  mediaLabel,
  platformText,
  poolStatus,
} from "./MarketingPool";

const beijingTimestamp = new Intl.DateTimeFormat("zh-CN", {
  timeZone: "Asia/Shanghai",
  hourCycle: "h23",
  dateStyle: "short",
  timeStyle: "short",
});

export function distributorName(user?: User): string {
  return user?.name || user?.nickname || user?.douyin || "分发员";
}

export function MarketingDetail({
  task,
  today,
  members,
  matchingUsers,
  tagNames,
  canDownload,
  onClose,
}: {
  task?: TaskVersion;
  today: string;
  members: Map<string, User>;
  matchingUsers: User[];
  tagNames: Map<string, string>;
  canDownload: boolean;
  onClose: () => void;
}) {
  if (!task) return null;

  const body = marketingBody(task.body);
  const allocation = task.allocation;
  const assignee = allocation ? members.get(allocation.user_id) : undefined;
  const expired = task.last_expiry;
  const candidates = matchingUsers
    .map((user) => ({
      user,
      matches: task.tags?.filter((tag) => user.tags?.includes(tag)) ?? [],
    }))
    .filter(({ matches }) => matches.length > 0)
    .sort((left, right) => right.matches.length - left.matches.length);

  function row(label: string, value: ReactNode) {
    return (
      <div key={label}>
        <dt>{label}</dt>
        <dd>{value || "—"}</dd>
      </div>
    );
  }

  return (
    <Modal
      title={`${marketingDisplayID(task.task_id)} · ${task.title}`}
      width={540}
      open
      closable={false}
      onCancel={onClose}
      footer={<Button onClick={onClose}>关闭</Button>}
    >
      <dl className="marketing-detail">
        {row("艺人", task.project || "未分项目")}
        {row(
          "类型 / 平台",
          `${mediaLabel(task)} · ${platformText(task) || "—"}`,
        )}
        {row(
          "上传",
          marketingDate(new Date(task.uploaded_at ?? task.created_at)),
        )}
        {row(
          "分发语",
          <span className="marketing-detail-copy">{body.copy || "—"}</span>,
        )}
        {row(
          "TAG",
          <>
            {body.tags || "—"}{" "}
            {body.minimum && (
              <span className="marketing-note">· {body.minimum}</span>
            )}
          </>,
        )}
        {row("@账号", body.mention)}
        {row(
          "标签",
          task.tags?.length ? (
            <Space size={4} wrap>
              {task.tags.map((tag) => (
                <Tag className="platform-tag" key={tag}>
                  #{tagNames.get(tag) ?? tag}
                </Tag>
              ))}
            </Space>
          ) : (
            "—"
          ),
        )}
        {row(
          "派单",
          allocation ? (
            <>
              已分给 <strong>{distributorName(assignee)}</strong>
              {assignee?.douyin && <>（{assignee.douyin}）</>}
              {allocation.source === AssignmentSource.SourceAssigned &&
                " · 工作人员指派"}
              {" · "}
              {poolStatus(task, today)}
              <MarketingProgress allocation={allocation} />
              {!allocation.has_link && allocation.expires_at && (
                <div className="marketing-note">
                  提交截止{" "}
                  {beijingTimestamp.format(new Date(allocation.expires_at))}
                </div>
              )}
            </>
          ) : (
            <>
              <div>领取时优先匹配这些分发员</div>
              {candidates.length ? (
                candidates.map(({ user, matches }) => (
                  <div key={user.id}>
                    <strong>{distributorName(user)}</strong>
                    {user.douyin && <>（{user.douyin}）</>}
                    <span className="marketing-note">
                      {" "}
                      · 标签对上{" "}
                      {matches
                        .map((tag) => tagNames.get(tag) ?? tag)
                        .join("、")}
                    </span>
                  </div>
                ))
              ) : (
                <span className="marketing-note">无标签匹配时按队列领取</span>
              )}
            </>
          ),
        )}
        {expired &&
          row(
            "失效记录",
            <>
              {distributorName(members.get(expired.user_id))}
              {expired.source === AssignmentSource.SourceAssigned
                ? "被指派"
                : expired.source === AssignmentSource.SourceReissued
                  ? "接到重派"
                  : "领取"}
              后超时未交链接，任务已失效。
              {expired.expired_at && (
                <div className="marketing-note">
                  {beijingTimestamp.format(new Date(expired.expired_at))}
                </div>
              )}
            </>,
          )}
        {task.deadline && row("可领取截止", task.deadline)}
        {canDownload &&
          !!task.files?.length &&
          row("素材", <AttachmentPreviews ids={task.files} />)}
      </dl>
    </Modal>
  );
}
