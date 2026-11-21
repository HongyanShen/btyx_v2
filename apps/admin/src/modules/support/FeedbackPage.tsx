import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  App,
  Button,
  Card,
  Form,
  Input,
  Modal,
  Table,
  Tag,
  Typography,
} from "antd";
import { get, post } from "../../api/client";
import { useSession } from "../../app/Auth";
import { Page, QueryState } from "../../components/Page";
import { FeedbackStatus } from "../../generated/enums";
import type { Feedback, User } from "../../generated/models";

const labels: Record<FeedbackStatus, string> = {
  [FeedbackStatus.FeedbackOpen]: "待处理",
  [FeedbackStatus.FeedbackReplied]: "已回复",
  [FeedbackStatus.FeedbackClosed]: "已关闭",
};

export function FeedbackPage() {
  const { canCall } = useSession();
  const canReadUsers = canCall("GET /users");
  const canReply = canCall("POST /feedback/:id/reply");
  const { message } = App.useApp();
  const cache = useQueryClient();
  const query = useQuery({
    queryKey: ["feedback"],
    queryFn: () => get<Feedback[]>("/feedback"),
  });
  const users = useQuery({
    queryKey: ["users"],
    queryFn: () => get<User[]>("/users"),
    enabled: canReadUsers,
  });
  const [selected, setSelected] = useState<Feedback | null>(null);
  const [form] = Form.useForm<{ reply: string }>();
  const names = new Map(
    (canReadUsers ? (users.data ?? []) : []).map((user) => [
      user.id,
      user.name || user.douyin || "分发员",
    ]),
  );
  const mutation = useMutation({
    mutationFn: (input: { reply: string }) =>
      post(`/feedback/${selected?.id}/reply`, input),
    onSuccess: () => {
      message.success("已回复，分发员可在小程序查看");
      setSelected(null);
      void cache.invalidateQueries({ queryKey: ["feedback"] });
    },
    onError: (error) => message.error(error.message),
  });

  return (
    <Page
      title="反馈与咨询"
      description="处理分发员在报名、任务执行和结算中的问题。"
    >
      <Card>
        <QueryState
          pending={query.isPending}
          error={query.error}
          retry={() => void query.refetch()}
        >
          <Table<Feedback>
            rowKey="id"
            dataSource={query.data}
            columns={[
              {
                title: "分发员",
                dataIndex: "user_id",
                render: (id: string) => names.get(id) ?? "分发员",
              },
              { title: "反馈内容", dataIndex: "body", ellipsis: true },
              {
                title: "状态",
                dataIndex: "status",
                render: (value: FeedbackStatus) => <Tag>{labels[value]}</Tag>,
              },
              {
                title: "提交时间",
                dataIndex: "created_at",
                render: (value: string) =>
                  new Date(value).toLocaleString("zh-CN"),
              },
              {
                title: "操作",
                render: (_, feedback) => (
                  <Button
                    type="link"
                    onClick={() => {
                      setSelected(feedback);
                      form.setFieldsValue({ reply: feedback.reply });
                    }}
                  >
                    {canReply ? "查看 / 回复" : "查看"}
                  </Button>
                ),
              },
            ]}
          />
        </QueryState>
      </Card>
      <Modal
        title={canReply ? "回复反馈" : "反馈详情"}
        open={selected !== null}
        onCancel={() => setSelected(null)}
        onOk={() => form.submit()}
        confirmLoading={mutation.isPending}
        footer={canReply ? undefined : null}
      >
        <Typography.Paragraph className="requirements">
          {selected?.body}
        </Typography.Paragraph>
        {canReply ? (
          <Form
            form={form}
            layout="vertical"
            onFinish={(values) => mutation.mutate(values)}
          >
            <Form.Item
              name="reply"
              label="回复内容"
              rules={[{ required: true }]}
            >
              <Input.TextArea rows={5} />
            </Form.Item>
          </Form>
        ) : (
          selected?.reply && (
            <Typography.Paragraph className="requirements">
              已回复　{selected.reply}
            </Typography.Paragraph>
          )
        )}
      </Modal>
    </Page>
  );
}
