import { useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import {
  Alert,
  App,
  Button,
  Descriptions,
  Input,
  Modal,
  Space,
  Tag,
  Typography,
} from "antd";
import { get, post } from "../../api/client";
import { QueryState } from "../../components/Page";
import type {
  DistributorBlock,
  DistributorTask,
  User,
} from "../../generated/models";
import { assignmentLabels, sourceLabels } from "../../shared/status";
import { AccountSearchLink } from "./AccountSearchLink";
import {
  douyinSearchURL,
  xiaohongshuSearchURL,
} from "./enrollment-profile-links";

const dateFormat = new Intl.DateTimeFormat("zh-CN", {
  timeZone: "Asia/Shanghai",
  dateStyle: "medium",
  timeStyle: "short",
});

export type DistributorDialogMode = "details" | "block" | "unblock";

interface DistributorDialogProps {
  user: User;
  mode: DistributorDialogMode;
  canBlock: boolean;
  tagNames: ReadonlyMap<string, string>;
  onChangeMode: (mode: DistributorDialogMode) => void;
  onClose: () => void;
  onSaved: () => void;
}

export function distributorName(user: User) {
  return user.name || user.nickname || "分发员";
}

export function DistributorTags({
  user,
  names,
}: {
  user: User;
  names: ReadonlyMap<string, string>;
}) {
  if (!user.tags?.length)
    return <Typography.Text type="secondary">未打标签</Typography.Text>;

  return (
    <Space size={[0, 4]} wrap>
      {user.tags.map((tag) => (
        <Tag key={tag}>#{names.get(tag) ?? tag}</Tag>
      ))}
    </Space>
  );
}

export function DistributorDialog({
  user,
  mode,
  canBlock,
  tagNames,
  onChangeMode,
  onClose,
  onSaved,
}: DistributorDialogProps) {
  const { message } = App.useApp();
  const [reason, setReason] = useState("");
  const [reasonMissing, setReasonMissing] = useState(false);
  const details = useQuery({
    queryKey: ["users", "distributors", user.id],
    queryFn: async () => {
      const [profile] = await get<User[]>(
        `/users?view=distributors&id=${encodeURIComponent(user.id)}`,
      );
      if (!profile?.distributor_details) {
        throw new Error("分发员详情暂不可用，请刷新后重试");
      }

      return profile;
    },
    gcTime: 0,
    staleTime: 0,
    retry: false,
  });
  const profile = details.data ?? user;
  const ready = !details.isFetching && !details.isError && !!details.data;
  const blocking = mode === "block";
  const action = blocking ? "拉黑" : "取消拉黑";
  const stale = mode !== "details" && ready && profile.blocked === blocking;
  const save = useMutation({
    mutationFn: () =>
      post(`/users/${profile.id}/block`, {
        blocked: blocking,
        reason: reason.trim(),
        updated_at: profile.distributor_details?.updated_at,
      }),
    onSuccess: () => {
      message.success(blocking ? "已拉黑" : "已取消拉黑");
      onSaved();
    },
  });

  function confirmChange() {
    if (blocking && !reason.trim()) {
      setReasonMissing(true);
      return;
    }

    save.mutate();
  }

  return (
    <Modal
      className={
        blocking
          ? "distributor-modal distributor-block-modal"
          : "distributor-modal"
      }
      open
      centered
      closable={false}
      title={
        mode === "details"
          ? "分发员详情"
          : `${action} ${distributorName(profile)}`
      }
      width={{ details: 520, block: 420, unblock: 460 }[mode]}
      onCancel={save.isPending ? undefined : onClose}
      keyboard={!save.isPending}
      mask={{ closable: !save.isPending }}
      footer={
        mode === "details" ? (
          <Space>
            {canBlock && ready && (
              <Button
                autoInsertSpace={false}
                danger={!profile.blocked}
                onClick={() =>
                  onChangeMode(profile.blocked ? "unblock" : "block")
                }
              >
                {profile.blocked ? "取消拉黑" : "拉黑"}
              </Button>
            )}
            <Button aria-label="关闭" autoInsertSpace={false} onClick={onClose}>
              关闭
            </Button>
          </Space>
        ) : (
          <Space size={blocking ? 4 : undefined}>
            <Button
              className={blocking ? "distributor-block-cancel" : undefined}
              size={blocking ? "small" : undefined}
              autoInsertSpace={false}
              disabled={save.isPending}
              onClick={onClose}
            >
              取消
            </Button>
            <Button
              className={blocking ? "distributor-block-confirm" : undefined}
              size={blocking ? "small" : undefined}
              type="primary"
              danger={blocking}
              loading={save.isPending}
              disabled={!canBlock || !ready || stale}
              onClick={confirmChange}
            >
              确认{action}
            </Button>
          </Space>
        )
      }
    >
      <QueryState
        pending={details.isPending}
        error={details.error}
        retry={() => void details.refetch()}
      >
        {mode === "details" ? (
          <DistributorProfile profile={profile} tagNames={tagNames} />
        ) : (
          <div
            className={
              blocking ? "distributor-block-body" : "distributor-dialog-body"
            }
          >
            {stale ? (
              <Alert
                type="warning"
                showIcon
                title="账号状态已更新，请关闭弹窗并刷新列表"
              />
            ) : blocking ? (
              <>
                <p className="distributor-block-impact">
                  拉黑后，这个分发员<strong>不能领任务</strong>，系统也
                  <strong>不再给他派</strong>任何任务；
                  <strong>已发布的任务照常结算</strong>。可以随时取消拉黑。
                </p>
                <TodayTasks
                  tasks={profile.distributor_details?.today_tasks ?? []}
                  warn
                />
                <div>
                  <Input
                    aria-label="拉黑原因"
                    aria-required="true"
                    aria-invalid={reasonMissing || undefined}
                    aria-describedby={
                      reasonMissing
                        ? "distributor-block-reason-error"
                        : undefined
                    }
                    placeholder="拉黑原因（必填）"
                    maxLength={300}
                    disabled={save.isPending}
                    status={reasonMissing ? "error" : undefined}
                    value={reason}
                    onChange={(event) => {
                      setReason(event.target.value);
                      setReasonMissing(false);
                    }}
                  />
                  {reasonMissing && (
                    <div
                      id="distributor-block-reason-error"
                      className="distributor-block-error"
                      role="alert"
                    >
                      请填写拉黑原因
                    </div>
                  )}
                </div>
              </>
            ) : (
              <>
                <Typography.Text>
                  取消拉黑后，符合领取条件的分发员可继续领取和接收任务。
                </Typography.Text>
                <BlockInfo info={profile.block_info} />
              </>
            )}
            {save.error && (
              <Alert type="error" showIcon title={save.error.message} />
            )}
          </div>
        )}
      </QueryState>
    </Modal>
  );
}

function DistributorProfile({
  profile,
  tagNames,
}: {
  profile: User;
  tagNames: ReadonlyMap<string, string>;
}) {
  const details = profile.distributor_details;
  return (
    <div className="distributor-dialog-body">
      <div className="distributor-profile-heading">
        <Typography.Text strong>{distributorName(profile)}</Typography.Text>
        <Tag color={profile.blocked ? "red" : "green"}>
          {profile.blocked ? "已拉黑" : "正常"}
        </Tag>
      </div>
      {profile.blocked && <BlockInfo info={profile.block_info} />}
      <section>
        <Typography.Title level={5}>实名信息</Typography.Title>
        <Descriptions
          size="small"
          column={1}
          items={[
            { key: "name", label: "实名姓名", children: profile.name || "—" },
            {
              key: "identity",
              label: "身份证号",
              children: details?.identity || "—",
            },
            { key: "phone", label: "手机号", children: details?.phone || "—" },
            {
              key: "wechat",
              label: "微信号",
              children: profile.wechat || "未填写",
            },
          ]}
        />
      </section>
      <section>
        <Typography.Title level={5}>账号</Typography.Title>
        <Descriptions
          size="small"
          column={1}
          items={[
            {
              key: "nickname",
              label: "微信昵称",
              children: profile.nickname || "—",
            },
            {
              key: "douyin",
              label: "抖音号",
              children: (
                <AccountSearchLink
                  value={profile.douyin}
                  href={douyinSearchURL(profile.douyin)}
                />
              ),
            },
            {
              key: "xhs",
              label: "小红书号",
              children: (
                <AccountSearchLink
                  value={profile.xhs}
                  href={xiaohongshuSearchURL(profile.xhs)}
                />
              ),
            },
            {
              key: "tags",
              label: "标签",
              children: <DistributorTags user={profile} names={tagNames} />,
            },
            {
              key: "submitted",
              label: "报名时间",
              children: profile.enrollment_submitted_at
                ? dateFormat.format(new Date(profile.enrollment_submitted_at))
                : "—",
            },
            {
              key: "consent",
              label: "当前协议",
              children: profile.consented ? "已同意" : "待同意",
            },
          ]}
        />
      </section>
      <TodayTasks tasks={details?.today_tasks ?? []} />
    </div>
  );
}

function BlockInfo({ info }: { info?: DistributorBlock | null }) {
  return (
    <Alert
      type="error"
      title={
        <Descriptions
          size="small"
          column={1}
          items={[
            { key: "reason", label: "拉黑原因", children: info?.reason || "—" },
            {
              key: "at",
              label: "拉黑时间",
              children: info?.at ? dateFormat.format(new Date(info.at)) : "—",
            },
            {
              key: "operator",
              label: "操作人",
              children: info?.operator || "—",
            },
          ]}
        />
      }
    />
  );
}

function TodayTasks({
  tasks,
  warn = false,
}: {
  tasks: DistributorTask[];
  warn?: boolean;
}) {
  return (
    <section className={warn ? "distributor-today-summary" : undefined}>
      <div className={warn ? "distributor-today-line" : undefined}>
        {warn ? (
          <span className="distributor-today-label">他今天的任务：</span>
        ) : (
          <Typography.Title level={5}>今天的任务</Typography.Title>
        )}
        {tasks.length ? (
          <ul className="distributor-today-tasks">
            {tasks.map((task) => (
              <li key={task.id}>
                <Typography.Text>{task.title}</Typography.Text>
                <Typography.Text type="secondary">
                  （{sourceLabels[task.source]} ·{" "}
                  {assignmentLabels[task.status]}）
                </Typography.Text>
              </li>
            ))}
          </ul>
        ) : (
          <Typography.Text type="secondary">今天还没有任务</Typography.Text>
        )}
      </div>
      {warn && tasks.some((task) => task.unpublished) && (
        <div className="distributor-today-warning">
          还有未发布的任务，建议等分发员完成后再拉黑。
        </div>
      )}
    </section>
  );
}
