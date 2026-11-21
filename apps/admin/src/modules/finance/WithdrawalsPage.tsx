import { useQuery } from "@tanstack/react-query";
import { Alert, Button, Card, Table, Tag } from "antd";
import { get } from "../../api/client";
import { Page, QueryState } from "../../components/Page";
import { WithdrawalStatus } from "../../generated/enums";
import type { Withdrawal } from "../../generated/models";
import { formatFen } from "../../shared/money";
import { withdrawalLabels } from "../../shared/status";

export function WithdrawalsPage() {
  const query = useQuery({
    queryKey: ["withdrawals"],
    queryFn: () => get<Withdrawal[]>("/withdrawals"),
    refetchInterval: 15_000,
  });
  const blocked = query.data?.some(
    (record) => record.status === WithdrawalStatus.WithdrawalBlockedConfig,
  );

  return (
    <Page
      title="提现记录"
      description="分发员按余额申请提现，转账结果异步更新。"
      extra={<Button onClick={() => void query.refetch()}>刷新结果</Button>}
    >
      {blocked && (
        <Alert
          type="warning"
          showIcon
          message="微信转账通道尚未完成配置。相关申请处于等待处理状态，金额仍保留在冻结余额中。"
          className="form-alert"
        />
      )}
      <Card>
        <QueryState
          pending={query.isPending}
          error={query.error}
          retry={() => void query.refetch()}
        >
          <Table<Withdrawal>
            rowKey="id"
            dataSource={query.data}
            pagination={{ pageSize: 12 }}
            columns={[
              { title: "申请编号", dataIndex: "id", ellipsis: true },
              { title: "分发员编号", dataIndex: "user_id", ellipsis: true },
              {
                title: "申请金额",
                dataIndex: "amount_fen",
                render: (amount: string) => (
                  <strong className="money">¥{formatFen(amount)}</strong>
                ),
              },
              { title: "收款方式", render: () => "微信零钱" },
              {
                title: "处理状态",
                dataIndex: "status",
                render: (status: WithdrawalStatus) => (
                  <Tag
                    color={
                      status === WithdrawalStatus.WithdrawalSuccess
                        ? "green"
                        : "gold"
                    }
                  >
                    {withdrawalLabels[status]}
                  </Tag>
                ),
              },
              {
                title: "说明",
                dataIndex: "reason",
                render: (reason: string) => reason || "—",
              },
              {
                title: "申请时间",
                dataIndex: "created_at",
                render: (value: string) =>
                  new Date(value).toLocaleString("zh-CN"),
              },
            ]}
          />
        </QueryState>
      </Card>
    </Page>
  );
}
