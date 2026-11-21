import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { App, Alert, Button, Form, Modal, Select, Typography } from "antd";
import { get, post } from "../../api/client";
import { useSession } from "../../app/Auth";
import { useMarketingTags } from "../../components/MarketingTags";
import { Page, QueryState } from "../../components/Page";
import {
  AssignmentSource,
  EnrollmentStatus,
  FileMediaType,
} from "../../generated/enums";
import type {
  MarketingSettings,
  TaskInput,
  TaskVersion,
  User,
} from "../../generated/models";
import { MarketingControls } from "./MarketingControls";
import { distributorName, MarketingDetail } from "./MarketingDetail";
import {
  MarketingPoolTable,
  marketingBody,
  marketingDate,
  poolStatus,
} from "./MarketingPool";
import {
  platformChoice,
  selectedPlatforms,
  TaskFormFields,
  type TaskFormValues,
} from "./TaskForm";

function versionFormValues(version: TaskVersion): TaskFormValues {
  const body = marketingBody(version.body);
  return {
    title: version.title,
    project: version.project,
    body: body.copy || version.body,
    topics: body.copy ? body.tags : "",
    mention: body.copy ? body.mention : "",
    minimumTopics: body.copy ? (body.minimum.match(/\d+/)?.[0] ?? "") : "",
    platformChoice: platformChoice(version.platforms),
    kind:
      version.media_types?.includes(FileMediaType.FileMP4) ||
      !version.files?.length
        ? "video"
        : "images",
    tags: version.tags ?? [],
    files: version.files ?? [],
  };
}

export function TasksPage() {
  const { canCall } = useSession();
  const canEditSettings = canCall("POST /marketing-settings");
  const canAssign = canCall("POST /tasks/:id/assign") && canCall("GET /users");
  const canEdit = canCall("POST /tasks/:id/versions");
  const canPreview = canCall("GET /files/:id/download");
  const { message } = App.useApp();
  const cache = useQueryClient();
  const query = useQuery({
    queryKey: ["tasks"],
    queryFn: () => get<TaskVersion[]>("/tasks"),
    refetchInterval: 10000,
  });
  const settings = useQuery({
    queryKey: ["marketing-settings"],
    queryFn: () => get<MarketingSettings>("/marketing-settings"),
    enabled: canCall("GET /marketing-settings"),
  });
  const projects = [
    ...new Set(
      (query.data ?? []).flatMap((task) =>
        task.project ? [task.project] : [],
      ),
    ),
  ];
  const tags = useMarketingTags();
  const tagNames = new Map(tags.data?.map((tag) => [tag.key, tag.name]));
  const users = useQuery({
    queryKey: ["users"],
    queryFn: () => get<User[]>("/users"),
    enabled: canCall("GET /users"),
  });
  const members = new Map(users.data?.map((user) => [user.id, user]));
  const userNames = new Map(
    users.data?.map((user) => [
      user.id,
      [user.name || user.nickname, user.douyin].filter(Boolean).join(" · "),
    ]),
  );
  const eligibleUsers = (users.data ?? []).filter(
    (user) =>
      user.status === EnrollmentStatus.EnrollmentApproved &&
      user.consented &&
      !user.blocked,
  );
  const today = marketingDate();
  const [editing, setEditing] = useState<TaskVersion | "new" | null>(null);
  const [uploading, setUploading] = useState(false);
  const [detailID, setDetailID] = useState<string>();
  const detail = query.data?.find((row) => row.task_id === detailID);
  const [assigning, setAssigning] = useState<TaskVersion | null>(null);
  const [assignee, setAssignee] = useState<string>();
  const [assignmentKey, setAssignmentKey] = useState(() => crypto.randomUUID());
  const [form] = Form.useForm<TaskFormValues>();

  const save = useMutation({
    mutationFn: async (values: TaskFormValues) => {
      if (!editing) throw new Error("请先打开营销内容表单");

      const previous =
        editing === "new" ? undefined : versionFormValues(editing);
      const unchanged =
        previous &&
        values.body === previous.body &&
        values.topics === previous.topics &&
        values.mention === previous.mention &&
        values.minimumTopics === previous.minimumTopics;
      const input: TaskInput = {
        title: values.title,
        body:
          editing !== "new" && unchanged
            ? editing.body
            : [
                values.body,
                values.topics?.trim() && `TAG：${values.topics.trim()}`,
                values.mention?.trim() && `@账号：${values.mention.trim()}`,
                values.minimumTopics &&
                  Number(values.minimumTopics) > 0 &&
                  `话题至少带满 ${values.minimumTopics} 个（含必带 TAG）`,
              ]
                .filter(Boolean)
                .join("\n"),
        project: values.project?.trim() ?? "",
        content_id: editing === "new" ? "" : editing.content_id,
        files: values.files ?? [],
        price_fen: "",
        platforms: selectedPlatforms(values.platformChoice),
        tags: values.tags ?? [],
        deadline: editing === "new" ? "" : (editing.deadline ?? ""),
      };

      return post(
        editing === "new" ? "/tasks" : `/tasks/${editing.task_id}/versions`,
        input,
      );
    },
    onSuccess: () => {
      message.success(editing === "new" ? "已加入营销内容池" : "已保存修改");
      setEditing(null);
      void cache.invalidateQueries({ queryKey: ["tasks"] });
      void cache.invalidateQueries({ queryKey: ["contents"] });
    },
    onError: (error) => message.error(error.message),
  });

  const assign = useMutation({
    mutationFn: () =>
      post(
        `/tasks/${assigning?.task_id}/assign`,
        { user_id: assignee },
        assignmentKey,
      ),
    onSuccess: () => {
      message.success("已指派，占用分发员当天的一条名额");
      setAssigning(null);
      void cache.invalidateQueries({ queryKey: ["assignments"] });
      void cache.invalidateQueries({ queryKey: ["tasks"] });
    },
    onError: (error) => {
      message.error(error.message);
      void cache.invalidateQueries({ queryKey: ["tasks"] });
    },
  });

  function edit(version: TaskVersion | "new") {
    setEditing(version);
    form.resetFields();
    form.setFieldsValue(
      version === "new"
        ? {
            kind: "video",
            platformChoice: "both",
            project: projects[0],
            minimumTopics: "5",
            tags: [],
            files: [],
          }
        : versionFormValues(version),
    );
  }

  function canAssignContent(row: TaskVersion) {
    return (
      canAssign &&
      settings.data?.online !== false &&
      poolStatus(row, today) === "排队中"
    );
  }

  function startAssignment(row: TaskVersion) {
    setAssigning(row);
    setAssignee(undefined);
    setAssignmentKey(crypto.randomUUID());
  }

  function renderRecipient(row: TaskVersion) {
    const allocation = row.allocation;
    if (!allocation) return <span className="marketing-note">还没分出去</span>;

    const user = members.get(allocation.user_id);
    const assigned = allocation.source === AssignmentSource.SourceAssigned;
    return (
      <>
        {assigned && "已指派给 "}
        <strong>{distributorName(user)}</strong>
        {(user?.douyin || assigned) && (
          <div className="marketing-note">
            {[user?.douyin, assigned ? "工作人员指派" : null]
              .filter(Boolean)
              .join(" · ")}
          </div>
        )}
      </>
    );
  }

  function renderRowActions(row: TaskVersion) {
    return (
      <div className="marketing-row-actions">
        {canAssignContent(row) && (
          <Button
            type="primary"
            size="small"
            onClick={() => startAssignment(row)}
          >
            手动指派
          </Button>
        )}
        {canEdit && (
          <Button size="small" onClick={() => edit(row)}>
            修改
          </Button>
        )}
        <Button size="small" onClick={() => setDetailID(row.task_id)}>
          详情
        </Button>
      </div>
    );
  }

  return (
    <Page title="营销分发 · 任务管理">
      {settings.isError && (
        <Alert
          type="error"
          message="营销分发设置加载失败"
          action={
            <Button size="small" onClick={() => void settings.refetch()}>
              重试
            </Button>
          }
        />
      )}
      <MarketingControls settings={settings.data} canEdit={canEditSettings} />
      <section
        className="marketing-pool"
        aria-labelledby="marketing-pool-heading"
      >
        <div className="marketing-section-heading">
          <Typography.Title id="marketing-pool-heading" level={4}>
            营销内容
          </Typography.Title>
          <Typography.Text type="secondary">
            一条内容就是一个任务，同一条只给一个人。状态下面的小圆点显示当前进度（拿内容
            → 发布 → 交链接 → 审核 →
            结算）。前两步由分发员在小程序确认，交了发布链接后系统才有凭据。
          </Typography.Text>
        </div>
        {canCall("POST /tasks") && (
          <div className="marketing-toolbar">
            <Button
              type="primary"
              icon={<span aria-hidden="true">＋</span>}
              onClick={() => edit("new")}
            >
              上传营销内容
            </Button>
          </div>
        )}
        <QueryState
          pending={query.isPending}
          error={query.error}
          retry={() => void query.refetch()}
        >
          <MarketingPoolTable
            tasks={query.data ?? []}
            today={today}
            open={settings.data?.online !== false}
            tagNames={tagNames}
            renderRecipient={renderRecipient}
            renderActions={renderRowActions}
            canPreview={canPreview}
          />
        </QueryState>
      </section>
      <MarketingDetail
        task={detail}
        today={today}
        members={members}
        matchingUsers={eligibleUsers}
        tagNames={tagNames}
        canDownload={canPreview}
        onClose={() => setDetailID(undefined)}
      />
      <Modal
        title={
          <>
            <div>
              {editing === "new" ? "新建任务 · 营销分发" : "修改营销内容"}
            </div>
            <div className="marketing-upload-intro">
              {editing === "new"
                ? "上传后，内容进入分配队列。分发员通过统一入口领取今日任务。"
                : "保存后，新领取和指派的任务使用这一版。已分配的任务继续按领取或指派时的版本执行。"}
            </div>
          </>
        }
        className="marketing-upload-modal"
        open={
          editing !== null &&
          canCall(
            editing === "new" ? "POST /tasks" : "POST /tasks/:id/versions",
          )
        }
        width={760}
        mask={{ closable: false }}
        okText={editing === "new" ? "上传到内容池" : "保存修改"}
        onCancel={() => !save.isPending && !uploading && setEditing(null)}
        onOk={() => form.submit()}
        okButtonProps={{ disabled: uploading }}
        confirmLoading={save.isPending}
        destroyOnHidden
      >
        <Form
          className="marketing-upload-form"
          form={form}
          layout="horizontal"
          labelCol={{ flex: "134px" }}
          labelAlign="left"
          colon={false}
          onFinish={(values) =>
            !save.isPending && !uploading && save.mutate(values)
          }
        >
          <TaskFormFields
            projects={projects}
            priceFen={settings.data?.price_fen}
            uploading={uploading}
            onUploadingChange={setUploading}
          />
        </Form>
      </Modal>
      <Modal
        title={assigning ? `手动指派 · ${assigning.title}` : "手动指派"}
        open={canAssign && assigning !== null}
        onCancel={() => !assign.isPending && setAssigning(null)}
        onOk={() => !assign.isPending && assign.mutate()}
        okText="确认指派"
        okButtonProps={{ disabled: !assignee }}
        confirmLoading={assign.isPending}
      >
        <Alert
          type="info"
          message="任务将直接进入分发员的工作台，占用其当天的一条名额。提交链接前可在发布链接审核中撤回。"
          className="form-alert"
        />
        <Select
          style={{ width: "100%" }}
          placeholder="选择已通过报名并同意协议的分发员"
          loading={users.isPending}
          disabled={assign.isPending || users.isPending}
          notFoundContent={
            users.isError ? "分发员加载失败" : "暂无可选择的分发员"
          }
          value={assignee}
          onChange={(value) => {
            setAssignee(value);
            setAssignmentKey(crypto.randomUUID());
          }}
          options={eligibleUsers.map((user) => ({
            value: user.id,
            label: userNames.get(user.id) || user.id,
          }))}
        />
      </Modal>
    </Page>
  );
}
