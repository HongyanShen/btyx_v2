import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  App,
  Alert,
  Button,
  Card,
  Input,
  Modal,
  Result,
  Space,
  Statistic,
  Table,
  Typography,
} from "antd";
import { get, post } from "../../api/client";
import { useSession } from "../../app/Auth";
import { Page, QueryState } from "../../components/Page";
import type {
  SettlementBatch,
  SettlementLine,
  SettlementPreview,
  User,
} from "../../generated/models";
import { formatFen } from "../../shared/money";
import { platformLabels } from "../../shared/status";

function latestMonday(): string {
  const date = new Date();
  date.setDate(date.getDate() - ((date.getDay() + 6) % 7));

  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}-${String(date.getDate()).padStart(2, "0")}`;
}

export function SettlementPage() {
  const { canCall } = useSession();
  const canPreview = canCall("GET /settlements/preview");
  const canReadBatches = canCall("GET /settlements");
  const canReadUsers = canPreview && canCall("GET /users");
  const { message, modal } = App.useApp();
  const cache = useQueryClient();
  const [through, setThrough] = useState(latestMonday);
  const preview = useQuery({
    queryKey: ["settlement-preview", through],
    queryFn: () =>
      get<SettlementPreview>(`/settlements/preview?through=${through}`),
    enabled: canPreview && /^\d{4}-\d{2}-\d{2}$/.test(through),
  });
  const batches = useQuery({
    queryKey: ["settlements"],
    queryFn: () => get<SettlementBatch[]>("/settlements"),
    enabled: canReadBatches,
  });
  const users = useQuery({
    queryKey: ["users"],
    queryFn: () => get<User[]>("/users"),
    enabled: canReadUsers,
  });
  const names = new Map(
    (canReadUsers ? (users.data ?? []) : []).map((user) => [
      user.id,
      user.name || user.douyin || "分发员",
    ]),
  );

  const credit = useMutation({
    mutationFn: (key: string) => post("/settlements", { through }, key),
    onSuccess: () => {
      message.success("已确认入账，奖励已计入分发员可提现余额");
      void cache.invalidateQueries({ queryKey: ["settlement-preview"] });
      void cache.invalidateQueries({ queryKey: ["settlements"] });
      void cache.invalidateQueries({ queryKey: ["assignments"] });
      void cache.invalidateQueries({ queryKey: ["ledger"] });
    },
    onError: (error) => message.error(error.message),
  });

  function confirmCredit() {
    const key = crypto.randomUUID();
    modal.confirm({
      title: "确认本次结算入账？",
      content: (
        <p>
          共 {preview.data?.count ?? 0} 条平台记录，合计 ¥
          {formatFen(preview.data?.amount_fen ?? "0")}
          。确认后明细锁定，金额进入分发员可提现余额；分发员自行申请提现。
        </p>
      ),
      okText: "确认入账",
      onOk: () => credit.mutateAsync(key),
    });
  }

  return (
    <Page
      title="分发结算"
      description="每周核对平台发布与播放数据，确认入账后分发员可自行提现。"
    >
      {!canPreview && !canReadBatches && (
        <Result status="403" title="无权读取此页面的数据" />
      )}
      {canPreview && (
        <Card className="section-card">
          <div className="table-toolbar">
            <Space>
              <span>结算日期（周一）</span>
              <Input
                type="date"
                value={through}
                onChange={(event) => setThrough(event.target.value)}
              />
            </Space>
            <Button onClick={() => void preview.refetch()}>重新核对</Button>
          </div>
          <Alert
            type="info"
            showIcon
            message="只纳入审核通过、播放数据齐全且已到结算日的记录。晚交资料顺延，已入账记录不会重复计入。"
            className="form-alert"
          />
          <QueryState
            pending={preview.isPending}
            error={preview.error}
            retry={() => void preview.refetch()}
          >
            <div className="settlement-summary">
              <Statistic
                title="本次待入账"
                value={`¥${formatFen(preview.data?.amount_fen ?? "0")}`}
              />
              <Statistic
                title="可结算平台记录"
                value={preview.data?.count ?? 0}
                suffix="条"
              />
              {canCall("POST /settlements") && (
                <Button
                  type="primary"
                  size="large"
                  disabled={!preview.data?.count}
                  loading={credit.isPending}
                  onClick={confirmCredit}
                >
                  核对无误，确认入账
                </Button>
              )}
            </div>
            <Table<SettlementLine>
              rowKey="submission_id"
              dataSource={preview.data?.lines}
              pagination={{ pageSize: 10 }}
              columns={[
                {
                  title: "分发员",
                  dataIndex: "user_id",
                  render: (id: string) => names.get(id) ?? "分发员",
                },
                {
                  title: "平台",
                  dataIndex: "platform",
                  render: (platform: SettlementLine["platform"]) =>
                    platformLabels[platform],
                },
                {
                  title: "本次奖励",
                  dataIndex: "amount_fen",
                  render: (amount: string) => (
                    <strong className="money">¥{formatFen(amount)}</strong>
                  ),
                },
                { title: "可结算日期", dataIndex: "eligible_date" },
              ]}
            />
          </QueryState>
        </Card>
      )}
      {canReadBatches && (
        <Card title="已入账批次">
          <QueryState
            pending={batches.isPending}
            error={batches.error}
            retry={() => void batches.refetch()}
          >
            <Table<SettlementBatch>
              rowKey="id"
              dataSource={batches.data}
              pagination={{ pageSize: 8 }}
              columns={[
                { title: "结算日期", dataIndex: "through" },
                {
                  title: "平台记录",
                  dataIndex: "count",
                  render: (value: number) => `${value} 条`,
                },
                {
                  title: "入账金额",
                  dataIndex: "amount_fen",
                  render: (amount: string) => `¥${formatFen(amount)}`,
                },
                { title: "确认人", dataIndex: "creator_id" },
                {
                  title: "状态",
                  render: () => (
                    <Typography.Text type="success">已入账</Typography.Text>
                  ),
                },
              ]}
            />
          </QueryState>
        </Card>
      )}
    </Page>
  );
}
