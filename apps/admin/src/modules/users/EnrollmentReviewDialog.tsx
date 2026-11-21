import { useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { Alert, Button, Checkbox, Form, Input, Modal, Radio } from "antd";
import { post } from "../../api/client";
import { useSession } from "../../app/Auth";
import {
  marketingTagOptions,
  useCreateMarketingTag,
} from "../../components/MarketingTags";
import { EnrollmentAge, EnrollmentStatus } from "../../generated/enums";
import type { MarketingTag, User } from "../../generated/models";
import { AccountSearchLink } from "./AccountSearchLink";
import {
  douyinSearchURL,
  xiaohongshuSearchURL,
} from "./enrollment-profile-links";

export const enrollmentAgeLabels: Record<EnrollmentAge, string> = {
  [EnrollmentAge.EnrollmentAge18To24]: "18–24 岁",
  [EnrollmentAge.EnrollmentAge25To30]: "25–30 岁",
  [EnrollmentAge.EnrollmentAge31To40]: "31–40 岁",
  [EnrollmentAge.EnrollmentAgeOver40]: "40 岁以上",
  [EnrollmentAge.EnrollmentAgeUnknown]: "无法判断",
};

interface ReviewValues {
  recent_post?: boolean;
  new_account?: boolean;
  age_group?: EnrollmentAge;
  tags: string[];
  reason?: string;
}

export function EnrollmentReviewDialog({
  user,
  tags,
  tagsOnly = false,
  onClose,
  onSaved,
}: {
  user: User;
  tags: MarketingTag[];
  tagsOnly?: boolean;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [form] = Form.useForm<ReviewValues>();
  const [rejectionError, setRejectionError] = useState("");
  const [newTag, setNewTag] = useState("");
  const { canCall } = useSession();
  const editing = user.status === EnrollmentStatus.EnrollmentApproved;
  const reviewing = !tagsOnly && !editing;
  const douyin = douyinSearchURL(user.douyin);
  const xiaohongshu = xiaohongshuSearchURL(user.xhs);
  const canCreateTag =
    canCall("POST /marketing-tags") && canCall("GET /marketing-tags");
  const newTagName = newTag
    .trim()
    .replace(/^[#＃]+/, "")
    .trim();
  const createTag = useCreateMarketingTag(selectTag);
  const submit = useMutation({
    mutationFn: ({
      values,
      approved,
      saveTagsOnly = tagsOnly,
    }: {
      values: ReviewValues;
      approved: boolean;
      saveTagsOnly?: boolean;
    }) =>
      post(
        `/users/${user.id}/review`,
        saveTagsOnly
          ? {
              ...(user.enrollment_id
                ? { enrollment_id: user.enrollment_id }
                : {}),
              enrollment_updated_at: user.enrollment_updated_at,
              tags_only: true,
              tags: values.tags ?? [],
            }
          : {
              ...(user.enrollment_id
                ? { enrollment_id: user.enrollment_id }
                : {}),
              enrollment_updated_at: user.enrollment_updated_at,
              approved,
              portrait_only: editing,
              recent_post: values.recent_post ?? undefined,
              new_account: values.new_account ?? undefined,
              age_group: values.age_group ?? undefined,
              tags: values.tags ?? [],
              ...(!approved ? { reason: values.reason?.trim() } : {}),
            },
      ),
    onSuccess: onSaved,
  });
  const choices = marketingTagOptions(tags, user.tags ?? []).map((option) => ({
    ...option,
    label: `#${option.label.replace(/^#/, "")}`,
  }));
  const approveText = tagsOnly
    ? "保存标签"
    : editing
      ? "保存"
      : "复核通过并保存标签";
  const name = user.name || user.nickname || "报名人";
  const title = tagsOnly
    ? `编辑 ${name} 的标签`
    : editing
      ? `改 ${name} 的画像 / 标签`
      : `复核 ${name} 的报名`;
  const busy = submit.isPending || createTag.isPending;

  function selectTag(tag: MarketingTag) {
    const selected = form.getFieldsValue().tags ?? [];
    form.setFieldValue("tags", [...new Set([...selected, tag.key])]);
    setNewTag("");
  }

  function addTag() {
    if (!newTagName || busy || !canCreateTag) return;

    createTag.reset();
    const existing = tags.find((tag) => tag.name === newTagName);
    if (existing) selectTag(existing);
    else createTag.mutate(newTagName);
  }

  function reject() {
    if (busy) return;

    const values = form.getFieldsValue();
    if (!values.reason?.trim()) {
      setRejectionError("请填写驳回原因，分发员将收到此原因");
      return;
    }

    submit.mutate({ values, approved: false });
  }

  function save() {
    if (busy) return;

    if (
      editing &&
      !form.isFieldsTouched(["recent_post", "new_account", "age_group"])
    ) {
      submit.mutate({
        values: form.getFieldsValue(),
        approved: true,
        saveTagsOnly: true,
      });
      return;
    }

    form.submit();
  }

  return (
    <Modal
      className="enrollment-review-modal"
      title={title}
      open
      centered
      width={446}
      onCancel={() => !busy && onClose()}
      mask={{ closable: !busy }}
      keyboard={!busy}
      footer={[
        <Button key="cancel" onClick={onClose} disabled={busy}>
          取消
        </Button>,
        <Button
          key="save"
          type="primary"
          onClick={save}
          loading={submit.isPending && submit.variables?.approved}
          disabled={
            createTag.isPending ||
            (submit.isPending && !submit.variables?.approved)
          }
        >
          {approveText}
        </Button>,
      ]}
    >
      <Form<ReviewValues>
        form={form}
        layout="vertical"
        requiredMark={false}
        disabled={submit.isPending}
        initialValues={{
          recent_post: user.enrollment_recent_post ?? undefined,
          new_account: user.enrollment_new_account ?? undefined,
          age_group: user.enrollment_age_group ?? undefined,
          tags: user.tags ?? [],
        }}
        onFinish={(values) =>
          !busy && submit.mutate({ values, approved: true })
        }
      >
        {!tagsOnly && (
          <section className="enrollment-review-fields" aria-label="人工复核">
            <div className="enrollment-review-section-head">
              <strong>人工复核</strong>
              <span className="enrollment-muted">
                打开报名人的抖音、小红书主页看
              </span>
            </div>
            {(douyin || xiaohongshu) && (
              <div className="enrollment-review-links">
                {douyin && (
                  <AccountSearchLink value="搜索抖音号 ↗" href={douyin} />
                )}
                {xiaohongshu && (
                  <AccountSearchLink
                    value="搜索小红书号 ↗"
                    href={xiaohongshu}
                  />
                )}
              </div>
            )}
            <Form.Item
              name="recent_post"
              label="① 近 30 天发过作品"
              rules={[
                { required: true, message: "请选择近 30 天是否发过作品" },
              ]}
            >
              <Radio.Group
                className="enrollment-review-options"
                optionType="button"
                buttonStyle="solid"
                options={[
                  { label: "有", value: true },
                  { label: "没有", value: false },
                ]}
              />
            </Form.Item>
            <Form.Item
              name="new_account"
              label="② 是不是新号"
              rules={[{ required: true, message: "请选择是不是新号" }]}
            >
              <Radio.Group
                className="enrollment-review-options"
                optionType="button"
                buttonStyle="solid"
                options={[
                  { label: "不是", value: false },
                  { label: "是新号", value: true },
                ]}
              />
            </Form.Item>
            <Form.Item
              name="age_group"
              label="③ 年龄层"
              rules={[{ required: true, message: "请选择年龄层" }]}
            >
              <Radio.Group
                className="enrollment-review-options"
                optionType="button"
                buttonStyle="solid"
                options={Object.entries(enrollmentAgeLabels).map(
                  ([value, label]) => ({ value, label }),
                )}
              />
            </Form.Item>
          </section>
        )}
        <section className="enrollment-review-tags" aria-label="画像标签">
          <div className="enrollment-review-section-head">
            <strong>{tagsOnly ? "标签" : "④ 标签"}</strong>
            <span className="enrollment-muted">可多选，用于匹配营销内容。</span>
          </div>
          <Form.Item name="tags" hidden={choices.length === 0}>
            <Checkbox.Group
              className="enrollment-review-options"
              options={choices}
            />
          </Form.Item>
          {choices.length === 0 && (
            <p className="enrollment-muted">暂无可选标签</p>
          )}
          {canCreateTag && (
            <div className="enrollment-review-inline">
              <Input
                aria-label="新建画像标签"
                placeholder="没有合适的？例如 #健身"
                maxLength={25}
                autoComplete="off"
                disabled={createTag.isPending}
                value={newTag}
                onChange={(event) => {
                  setNewTag(event.target.value);
                  if (createTag.isError) createTag.reset();
                }}
                onPressEnter={(event) => {
                  event.preventDefault();
                  addTag();
                }}
              />
              <Button
                onClick={addTag}
                disabled={!newTagName}
                loading={createTag.isPending}
              >
                ＋ 加到标签库
              </Button>
            </div>
          )}
          {createTag.isError && (
            <Alert type="error" title={createTag.error.message} />
          )}
        </section>
        {reviewing && (
          <div className="enrollment-review-inline enrollment-review-reject">
            <Form.Item
              name="reason"
              help={rejectionError || undefined}
              validateStatus={rejectionError ? "error" : undefined}
            >
              <Input
                aria-label="驳回原因"
                maxLength={500}
                placeholder="不通过的原因，分发员可见"
                onChange={() => setRejectionError("")}
                onPressEnter={(event) => {
                  event.preventDefault();
                  reject();
                }}
              />
            </Form.Item>
            <Button
              type="primary"
              danger
              onClick={reject}
              loading={submit.isPending && !submit.variables?.approved}
              disabled={
                createTag.isPending ||
                (submit.isPending && submit.variables?.approved)
              }
            >
              拒绝
            </Button>
          </div>
        )}
      </Form>
      {submit.isError && <Alert type="error" title={submit.error.message} />}
    </Modal>
  );
}
