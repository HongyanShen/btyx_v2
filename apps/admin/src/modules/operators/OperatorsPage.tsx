import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Alert,
  App,
  Button,
  Checkbox,
  Form,
  Input,
  Modal,
  Switch,
  Table,
  Tag,
  Typography,
} from "antd";
import { get, post } from "../../api/client";
import { Page, QueryState } from "../../components/Page";
import type {
  Operator,
  OperatorPermissionCatalog,
} from "../../generated/models";

interface OperatorForm {
  username?: string;
  password?: string;
  pages: string[];
  endpoints: string[];
  disabled: boolean;
}

export function OperatorsPage() {
  const [editing, setEditing] = useState<Operator | "new" | null>(null);
  const operators = useQuery({
    queryKey: ["operators"],
    queryFn: () => get<Operator[]>("/operators"),
  });
  const catalog = useQuery({
    queryKey: ["operator-permissions"],
    queryFn: () => get<OperatorPermissionCatalog>("/operators/permissions"),
  });

  return (
    <Page
      title="运营账号"
      description="为每个账号配置可访问的页面和可调用的接口。"
      extra={
        <Button
          type="primary"
          disabled={!catalog.data}
          onClick={() => setEditing("new")}
        >
          新建运营账号
        </Button>
      }
    >
      {catalog.error && (
        <Alert
          type="error"
          showIcon
          message={catalog.error.message}
          action={
            <Button onClick={() => void catalog.refetch()}>重新加载权限</Button>
          }
        />
      )}
      <QueryState
        pending={operators.isPending}
        error={operators.error}
        retry={() => void operators.refetch()}
      >
        <Table<Operator>
          rowKey="id"
          dataSource={operators.data}
          scroll={{ x: 620 }}
          columns={[
            { title: "用户名", dataIndex: "username" },
            {
              title: "状态",
              render: (_, row) =>
                row.disabled ? (
                  <Tag>已停用</Tag>
                ) : (
                  <Tag color="green">已启用</Tag>
                ),
            },
            {
              title: "页面权限",
              render: (_, row) =>
                row.super_admin ? "全部" : `${row.pages.length} 项`,
            },
            {
              title: "接口权限",
              render: (_, row) =>
                row.super_admin ? "全部" : `${row.endpoints.length} 项`,
            },
            {
              title: "操作",
              render: (_, row) =>
                row.super_admin ? (
                  <Tag color="purple">超级管理员</Tag>
                ) : (
                  <Button
                    disabled={!catalog.data}
                    onClick={() => setEditing(row)}
                  >
                    编辑账号
                  </Button>
                ),
            },
          ]}
        />
      </QueryState>
      {editing && catalog.data && (
        <OperatorEditor
          editing={editing}
          catalog={catalog.data}
          onClose={() => setEditing(null)}
        />
      )}
    </Page>
  );
}

function OperatorEditor({
  editing,
  catalog,
  onClose,
}: {
  editing: Operator | "new";
  catalog: OperatorPermissionCatalog;
  onClose: () => void;
}) {
  const [form] = Form.useForm<OperatorForm>();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const cache = useQueryClient();
  const { message } = App.useApp();
  const account = editing === "new" ? undefined : editing;
  async function save(values: OperatorForm) {
    const { pages, endpoints, password, username, disabled } = values;
    setPending(true);
    setError("");

    try {
      if (account) {
        await post(`/operators/${account.id}`, {
          pages,
          endpoints,
          disabled,
          password,
        });
      } else {
        await post("/operators", { pages, endpoints, username, password });
      }

      void cache.invalidateQueries({ queryKey: ["operators"] });
      message.success(account ? "运营账号已更新" : "运营账号已创建");
      onClose();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "保存账号失败");
    } finally {
      setPending(false);
    }
  }

  return (
    <Modal
      title={account ? `编辑运营账号 · ${account.username}` : "新建运营账号"}
      open
      width={720}
      okText="保存"
      onOk={() => form.submit()}
      onCancel={onClose}
      confirmLoading={pending}
      cancelButtonProps={{ disabled: pending }}
      closable={!pending}
      keyboard={!pending}
      mask={{ closable: !pending }}
    >
      {error && (
        <Alert className="form-alert" type="error" showIcon message={error} />
      )}
      <Form
        form={form}
        layout="vertical"
        initialValues={{
          pages: catalog.pages
            .filter(({ path }) => account?.pages.includes(path))
            .map(({ path }) => path),
          endpoints: catalog.endpoints
            .filter(({ key }) => account?.endpoints.includes(key))
            .map(({ key }) => key),
          disabled: account?.disabled ?? false,
        }}
        onFinish={(values) => void save(values)}
      >
        {!account && (
          <Form.Item
            label="用户名"
            name="username"
            rules={[
              { required: true, message: "请输入用户名" },
              {
                pattern: /^[a-z0-9][a-z0-9._-]{2,63}$/i,
                message:
                  "使用 3 至 64 位字母、数字、点、下划线或连字符，并以字母或数字开头",
              },
            ]}
          >
            <Input autoComplete="off" maxLength={64} />
          </Form.Item>
        )}
        <Form.Item
          label={account ? "重置密码" : "初始密码"}
          name="password"
          extra={
            account
              ? "留空保留原密码。重置密码会使该账号的现有会话失效。"
              : "至少 12 个字符，最多 72 字节。"
          }
          rules={[{ required: !account, message: "请输入初始密码" }]}
        >
          <Input.Password autoComplete="new-password" />
        </Form.Item>
        {account && (
          <Form.Item
            label="停用账号"
            name="disabled"
            valuePropName="checked"
            extra="停用后，该账号无法登录，现有会话立即失效。"
          >
            <Switch />
          </Form.Item>
        )}
        <Form.Item label="页面权限" name="pages">
          <Checkbox.Group
            className="operator-permission-options"
            options={catalog.pages.map(({ path, label }) => ({
              label,
              value: path,
            }))}
          />
        </Form.Item>
        <Form.Item label="接口权限" name="endpoints">
          <Checkbox.Group
            className="operator-permission-options operator-endpoint-options"
            options={catalog.endpoints.map(({ key, label }) => ({
              label: (
                <>
                  {label}
                  <Typography.Text
                    className="operator-endpoint-key"
                    type="secondary"
                    code
                  >
                    {key}
                  </Typography.Text>
                </>
              ),
              value: key,
            }))}
          />
        </Form.Item>
      </Form>
    </Modal>
  );
}
