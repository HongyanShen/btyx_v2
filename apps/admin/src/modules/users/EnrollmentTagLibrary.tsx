import { useState, type FormEvent } from "react";
import CloseOutlined from "@ant-design/icons/CloseOutlined";
import EditOutlined from "@ant-design/icons/EditOutlined";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Alert, Button, Input, Modal, Tag, Tooltip } from "antd";
import { post } from "../../api/client";
import { useSession } from "../../app/Auth";
import {
  marketingTagsKey,
  useCreateMarketingTag,
  useMarketingTags,
} from "../../components/MarketingTags";
import type { MarketingTag } from "../../generated/models";

export function EnrollmentTagLibrary() {
  const { canCall } = useSession();
  const query = useMarketingTags();
  const queryClient = useQueryClient();
  const [newName, setNewName] = useState("");
  const [editing, setEditing] = useState<MarketingTag | null>(null);
  const [deleting, setDeleting] = useState<MarketingTag | null>(null);
  const [editedName, setEditedName] = useState("");
  const add = useCreateMarketingTag(() => setNewName(""));
  const rename = useMutation({
    mutationFn: ({ tag, name }: { tag: MarketingTag; name: string }) =>
      post(`/marketing-tags/${tag.id}/rename`, {
        name,
        previous_name: tag.name,
      }),
    onSuccess: async () => {
      setEditing(null);
      await queryClient.invalidateQueries({ queryKey: marketingTagsKey });
    },
  });
  const remove = useMutation({
    mutationFn: (tag: MarketingTag) =>
      post(`/marketing-tags/${tag.id}/delete`, { updated_at: tag.updated_at }),
    onSuccess: async () => {
      setDeleting(null);
      await queryClient.invalidateQueries({ queryKey: marketingTagsKey });
    },
  });

  if (!canCall("GET /marketing-tags")) return null;

  function addTag(event: FormEvent) {
    event.preventDefault();
    if (newName.trim() && !add.isPending) add.mutate(newName);
  }

  return (
    <section className="enrollment-tag-library" aria-label="标签库">
      <p>
        <strong>标签库</strong>
        <span>
          复核时按账号画像选择标签。营销内容也可选用这些标签，用于匹配分发员。改名后，使用该标签的分发员和内容会同步显示新名称。
        </span>
      </p>
      {query.isError ? (
        <Alert
          type="error"
          title={query.error.message}
          action={
            <Button size="small" onClick={() => void query.refetch()}>
              重试
            </Button>
          }
        />
      ) : (
        <div className="enrollment-tag-library-controls">
          {query.data?.map((tag) => (
            <span key={tag.id} className="enrollment-library-tag">
              <Tag>#{tag.name}</Tag>
              {canCall("POST /marketing-tags/:id/rename") && (
                <Tooltip title="改名" trigger={["hover", "focus"]}>
                  <Button
                    type="text"
                    size="small"
                    icon={<EditOutlined aria-hidden="true" />}
                    aria-label={`改名 ${tag.name}`}
                    onClick={() => {
                      rename.reset();
                      setEditing(tag);
                      setEditedName(tag.name);
                    }}
                  />
                </Tooltip>
              )}
              {canCall("POST /marketing-tags/:id/delete") && (
                <Tooltip title="删除" trigger={["hover", "focus"]}>
                  <Button
                    type="text"
                    size="small"
                    danger
                    icon={<CloseOutlined aria-hidden="true" />}
                    aria-label={`删除 ${tag.name}`}
                    onClick={() => {
                      remove.reset();
                      setDeleting(tag);
                    }}
                  />
                </Tooltip>
              )}
            </span>
          ))}
          {query.isPending && <span>正在加载标签</span>}
          {query.isSuccess && query.data.length === 0 && <span>暂无标签</span>}
          {canCall("POST /marketing-tags") && (
            <form className="enrollment-tag-add" onSubmit={addTag}>
              <Input
                size="small"
                aria-label="新标签"
                placeholder="新标签，例如 #健身"
                value={newName}
                onChange={(event) => setNewName(event.target.value)}
                maxLength={25}
              />
              <Button
                size="small"
                htmlType="submit"
                loading={add.isPending}
                disabled={!newName.trim()}
              >
                ＋ 添加
              </Button>
            </form>
          )}
        </div>
      )}
      {add.isError && <Alert type="error" title={add.error.message} />}
      <Modal
        title="重命名标签"
        open={editing !== null}
        okText="保存"
        okButtonProps={{ disabled: !editedName.trim() }}
        confirmLoading={rename.isPending}
        onCancel={() => !rename.isPending && setEditing(null)}
        onOk={() =>
          editing &&
          !rename.isPending &&
          rename.mutate({ tag: editing, name: editedName })
        }
      >
        {rename.isError && <Alert type="error" title={rename.error.message} />}
        <Input
          aria-label="标签名称"
          value={editedName}
          onChange={(event) => setEditedName(event.target.value)}
          maxLength={25}
        />
      </Modal>
      <Modal
        title={deleting ? `删除标签 #${deleting.name}` : "删除标签"}
        open={deleting !== null}
        okText="删除"
        okButtonProps={{ danger: true }}
        confirmLoading={remove.isPending}
        onCancel={() => !remove.isPending && setDeleting(null)}
        onOk={() => deleting && !remove.isPending && remove.mutate(deleting)}
      >
        <p>删除前请先解除该标签在分发员报名、内容池和任务上的绑定。</p>
        {remove.isError && <Alert type="error" title={remove.error.message} />}
      </Modal>
    </section>
  );
}
