import type { ReactNode } from "react";
import { Alert, Button, Empty, Space, Spin, Typography } from "antd";

interface PageProps {
  title: string;
  description?: string;
  extra?: ReactNode;
  children: ReactNode;
}

export function Page({ title, description, extra, children }: PageProps) {
  return (
    <section className="page">
      <div className="page-heading">
        <div>
          <Typography.Title level={2}>{title}</Typography.Title>
          {description && (
            <Typography.Text type="secondary">{description}</Typography.Text>
          )}
        </div>
        {extra && <Space wrap>{extra}</Space>}
      </div>
      {children}
    </section>
  );
}

interface QueryStateProps {
  pending: boolean;
  error: Error | null;
  empty?: boolean;
  retry: () => void;
  children: ReactNode;
}

export function QueryState({
  pending,
  error,
  empty,
  retry,
  children,
}: QueryStateProps) {
  if (pending) {
    return (
      <div className="state-panel">
        <Spin tip="正在加载" />
      </div>
    );
  }

  if (error) {
    return (
      <Alert
        type="error"
        showIcon
        message={error.message}
        action={<Button onClick={retry}>重试</Button>}
      />
    );
  }

  if (empty) {
    return (
      <div className="state-panel">
        <Empty description="暂无记录" />
      </div>
    );
  }

  return <>{children}</>;
}
