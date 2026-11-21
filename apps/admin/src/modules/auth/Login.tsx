import { useState } from "react";
import { Alert, Button, Form, Input, Typography } from "antd";
import { post } from "../../api/client";
import { useSession } from "../../app/Auth";
import type { LoginResult } from "../../generated/models";

export function Login() {
  const session = useSession();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");

  async function submit(values: { username: string; password: string }) {
    setPending(true);
    setError("");

    try {
      const result = await post<LoginResult>("/auth/login", values);
      session.login(result.token);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "登录失败");
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="login-screen">
      <section className="login-brand">
        <div className="brand-mark">兔</div>
        <span className="eyebrow">BAITU SELECT</span>
        <Typography.Title>
          让好内容
          <br />
          找到合适的人。
        </Typography.Title>
        <p>营销分发 · 内容协作 · 清晰结算</p>
        <div className="login-note">白兔严选 / 运营工作空间</div>
      </section>
      <section className="login-form">
        <div>
          <span className="eyebrow">运营后台</span>
          <Typography.Title level={2}>欢迎回来</Typography.Title>
          <Typography.Paragraph type="secondary">
            登录后管理内容、审核发布与核对结算。
          </Typography.Paragraph>
          {error && (
            <Alert
              type="error"
              showIcon
              message={error}
              className="form-alert"
            />
          )}
          <Form layout="vertical" onFinish={submit} requiredMark={false}>
            <Form.Item
              label="用户名"
              name="username"
              rules={[{ required: true, message: "请输入用户名" }]}
            >
              <Input size="large" autoComplete="username" />
            </Form.Item>
            <Form.Item
              label="密码"
              name="password"
              rules={[{ required: true, message: "请输入密码" }]}
            >
              <Input.Password size="large" autoComplete="current-password" />
            </Form.Item>
            <Button
              type="primary"
              htmlType="submit"
              size="large"
              block
              loading={pending}
            >
              进入运营后台
            </Button>
          </Form>
        </div>
      </section>
    </div>
  );
}
