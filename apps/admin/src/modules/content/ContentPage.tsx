import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  App,
  Button,
  Card,
  Form,
  Input,
  Modal,
  Space,
  Table,
  Tag,
  Typography,
} from "antd";
import { get, post } from "../../api/client";
import { download } from "../../api/files";
import { useSession } from "../../app/Auth";
import { AttachmentInput } from "../../components/Attachments";
import {
  MarketingTagSelect,
  useMarketingTags,
} from "../../components/MarketingTags";
import { Page, QueryState } from "../../components/Page";
import type { Content } from "../../generated/models";

type ContentInput = Pick<
  Content,
  "title" | "body" | "project" | "files" | "tags"
>;

export function ContentPage() {
  const { canCall } = useSession();
  const tags = useMarketingTags();
  const tagNames = new Map(tags.data?.map((tag) => [tag.key, tag.name]));
  const canCreate = canCall("POST /contents");
  const canEditTags = canCall("POST /contents/:id/tags");
  const canDownload = canCall("GET /files/:id/download");
  const { message } = App.useApp();
  const cache = useQueryClient();
  const query = useQuery({
    queryKey: ["contents"],
    queryFn: () => get<Content[]>("/contents"),
  });
  const [opened, setOpened] = useState(false);
  const [editingTags, setEditingTags] = useState<Content | null>(null);
  const [draftTags, setDraftTags] = useState<string[]>([]);
  const [form] = Form.useForm<ContentInput>();
  const mutation = useMutation({
    mutationFn: (input: ContentInput) =>
      post("/contents", {
        ...input,
        project: input.project ?? "",
        files: input.files ?? [],
        tags: input.tags ?? [],
      }),
    onSuccess: () => {
      message.success("素材已加入内容池");
      setOpened(false);
      void cache.invalidateQueries({ queryKey: ["contents"] });
    },
    onError: (error) => message.error(error.message),
  });
  const saveTags = useMutation({
    mutationFn: ({ record, tags }: { record: Content; tags: string[] }) =>
      post(`/contents/${record.id}/tags`, {
        previous_tags: record.tags ?? [],
        tags,
      }),
    onSuccess: () => {
      setEditingTags(null);
      message.success("已保存素材标签");
      void cache.invalidateQueries({ queryKey: ["contents"] });
    },
    onError: (error) => message.error(error.message),
  });

  return (
    <Page
      title="素材库"
      description="查看和整理素材。要开放领取或手动指派，请在营销任务中创建内容。"
      extra={
        canCreate && (
          <Button
            type="primary"
            onClick={() => {
              form.resetFields();
              setOpened(true);
            }}
          >
            新增内容
          </Button>
        )
      }
    >
      <Card>
        <QueryState
          pending={query.isPending}
          error={query.error}
          retry={() => void query.refetch()}
        >
          <Table<Content>
            rowKey="id"
            dataSource={query.data}
            pagination={{ pageSize: 10 }}
            columns={[
              {
                title: "素材内容",
                render: (_, row) => (
                  <>
                    <strong>{row.title}</strong>
                    <Typography.Paragraph
                      type="secondary"
                      ellipsis={{ rows: 2 }}
                      className="content-excerpt"
                    >
                      {row.body}
                    </Typography.Paragraph>
                  </>
                ),
              },
              {
                title: "项目",
                dataIndex: "project",
                render: (value: string) => value || "未分项目",
              },
              {
                title: "标签",
                render: (_, row) => (
                  <Space wrap>
                    {(row.tags ?? []).map((tag) => (
                      <Tag key={tag}>{tagNames.get(tag) ?? tag}</Tag>
                    ))}
                    {canEditTags && (
                      <Button
                        type="link"
                        size="small"
                        onClick={() => {
                          setDraftTags(row.tags ?? []);
                          setEditingTags(row);
                        }}
                      >
                        编辑标签
                      </Button>
                    )}
                  </Space>
                ),
              },
              {
                title: "附件",
                render: (_, row) => (
                  <Space wrap>
                    {canDownload &&
                      (row.files ?? []).map((id, index) => (
                        <Button
                          type="link"
                          key={id}
                          onClick={() =>
                            void download(id).catch((error: Error) =>
                              message.error(error.message),
                            )
                          }
                        >
                          素材 {index + 1}
                        </Button>
                      ))}
                  </Space>
                ),
              },
              {
                title: "创建时间",
                dataIndex: "created_at",
                render: (value: string) =>
                  new Date(value).toLocaleDateString("zh-CN"),
              },
            ]}
          />
        </QueryState>
      </Card>
      <Modal
        title="新增内容"
        open={canCreate && opened}
        onCancel={() => setOpened(false)}
        onOk={() => form.submit()}
        confirmLoading={mutation.isPending}
        width={660}
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={(values) => mutation.mutate(values)}
        >
          <Form.Item name="title" label="标题" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="project" label="所属项目">
            <Input />
          </Form.Item>
          <Form.Item
            name="body"
            label="文案和发布要求"
            rules={[{ required: true }]}
          >
            <Input.TextArea rows={5} />
          </Form.Item>
          <Form.Item name="tags" label="匹配标签">
            <MarketingTagSelect />
          </Form.Item>
          <Form.Item name="files" label="图片 / 视频">
            <AttachmentInput />
          </Form.Item>
        </Form>
      </Modal>
      <Modal
        title={editingTags ? `编辑标签 · ${editingTags.title}` : "编辑标签"}
        open={editingTags !== null}
        okText="保存"
        onCancel={() => !saveTags.isPending && setEditingTags(null)}
        onOk={() =>
          editingTags &&
          !saveTags.isPending &&
          saveTags.mutate({ record: editingTags, tags: draftTags })
        }
        confirmLoading={saveTags.isPending}
      >
        <Form layout="vertical">
          <Form.Item label="匹配标签">
            <MarketingTagSelect value={draftTags} onChange={setDraftTags} />
          </Form.Item>
        </Form>
      </Modal>
    </Page>
  );
}
