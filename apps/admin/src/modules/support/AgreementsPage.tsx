import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { App, Alert, Button, Card, Form, Input, Modal, Table, Tag } from "antd";
import { get, post } from "../../api/client";
import { useSession } from "../../app/Auth";
import { Page, QueryState } from "../../components/Page";
import type { Agreement } from "../../generated/models";
import { AgreementContent, AgreementEditor } from "./AgreementEditor";

export function AgreementsPage() {
  const canPublish = useSession().canCall("POST /agreements");
  const { message } = App.useApp();
  const cache = useQueryClient();
  const query = useQuery({
    queryKey: ["agreements"],
    queryFn: () => get<Agreement[]>("/agreements"),
  });
  const [opened, setOpened] = useState(false);
  const [form] = Form.useForm<{ title: string; body_html: string }>();
  const preview = Form.useWatch("body_html", form);
  const title = Form.useWatch("title", form);
  const mutation = useMutation({
    mutationFn: (input: { title: string; body_html: string }) =>
      post("/agreements", input),
    onSuccess: () => {
      message.success("新协议已发布");
      setOpened(false);
      void cache.invalidateQueries({ queryKey: ["agreements"] });
    },
    onError: (error) => message.error(error.message),
  });

  return (
    <Page
      title="分发协议"
      description="保存各版本协议与用户同意记录。当前协议用于新领取资格检查。"
      extra={
        canPublish && (
          <Button
            type="primary"
            onClick={() => {
              form.resetFields();
              setOpened(true);
            }}
          >
            发布新协议
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
          <Table<Agreement>
            rowKey="id"
            dataSource={query.data}
            expandable={{
              expandedRowRender: (agreement) => (
                <AgreementContent
                  body={agreement.body}
                  html={agreement.body_html}
                />
              ),
            }}
            columns={[
              { title: "协议标题", dataIndex: "title" },
              {
                title: "状态",
                render: (_, agreement) => (
                  <Tag color={agreement.active ? "green" : "default"}>
                    {agreement.active ? "当前生效" : "历史版本"}
                  </Tag>
                ),
              },
              { title: "发布人", dataIndex: "creator_id" },
              {
                title: "发布时间",
                dataIndex: "created_at",
                render: (value: string) =>
                  new Date(value).toLocaleString("zh-CN"),
              },
            ]}
          />
        </QueryState>
      </Card>
      <Modal
        title="发布新协议"
        open={canPublish && opened}
        onCancel={() => setOpened(false)}
        onOk={() => form.submit()}
        confirmLoading={mutation.isPending}
        width={720}
        destroyOnHidden
      >
        <Alert
          type="warning"
          showIcon
          message="发布后，分发员需要同意新协议才能领取新任务。已领取任务继续保留原版本信息。"
          className="form-alert"
        />
        <Form
          form={form}
          layout="vertical"
          preserve={false}
          onFinish={(values) => mutation.mutate(values)}
        >
          <Form.Item name="title" label="协议标题" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item
            name="body_html"
            label="协议正文"
            rules={[{ required: true, message: "请输入协议正文" }]}
          >
            <AgreementEditor />
          </Form.Item>
          <details className="agreement-preview">
            <summary>预览小程序排版</summary>
            <article>
              <h3>{title}</h3>
              <AgreementContent html={preview} />
            </article>
          </details>
        </Form>
      </Modal>
    </Page>
  );
}
