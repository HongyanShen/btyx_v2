import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { App, Alert, Button, Card, Form, Input, Modal, Table, Tag } from "antd";
import { get, post } from "../../api/client";
import { useSession } from "../../app/Auth";
import { Page, QueryState } from "../../components/Page";
import { LedgerKind } from "../../generated/enums";
import type { LedgerEntry } from "../../generated/models";
import { formatFen, signedYuanToFen } from "../../shared/money";

const labels: Record<LedgerKind, string> = {
  [LedgerKind.LedgerSettlementCredit]: "结算入账",
  [LedgerKind.LedgerWithdrawalFreeze]: "提现冻结",
  [LedgerKind.LedgerWithdrawalSuccess]: "提现到账确认",
  [LedgerKind.LedgerWithdrawalRefund]: "提现退回余额",
  [LedgerKind.LedgerAdjustment]: "关联调整",
};

export function LedgerPage() {
  const canAdjust = useSession().canCall("POST /ledger/:id/adjust");
  const { message } = App.useApp();
  const cache = useQueryClient();
  const query = useQuery({
    queryKey: ["ledger"],
    queryFn: () => get<LedgerEntry[]>("/ledger"),
  });
  const [original, setOriginal] = useState<LedgerEntry | null>(null);
  const [requestKey, setRequestKey] = useState(() => crypto.randomUUID());
  const [form] = Form.useForm<{ delta_yuan: string; reason: string }>();
  const adjustment = useMutation({
    mutationFn: (input: { delta_yuan: string; reason: string }) =>
      post(
        `/ledger/${original?.id}/adjust`,
        {
          delta_fen: signedYuanToFen(input.delta_yuan),
          reason: input.reason,
        },
        requestKey,
      ),
    onSuccess: () => {
      message.success("已创建关联调整流水");
      setOriginal(null);
      void cache.invalidateQueries({ queryKey: ["ledger"] });
    },
    onError: (error) => message.error(error.message),
  });

  return (
    <Page
      title="余额流水"
      description="记录每笔结算、冻结、到账和退回。历史流水保留原貌，纠错通过关联调整完成。"
    >
      <Card>
        <QueryState
          pending={query.isPending}
          error={query.error}
          retry={() => void query.refetch()}
        >
          <Table<LedgerEntry>
            rowKey="id"
            dataSource={query.data}
            pagination={{ pageSize: 12 }}
            columns={[
              { title: "分发员编号", dataIndex: "user_id", ellipsis: true },
              {
                title: "流水类型",
                dataIndex: "kind",
                render: (kind: LedgerKind) => <Tag>{labels[kind]}</Tag>,
              },
              {
                title: "可用余额变动",
                dataIndex: "amount_fen",
                render: (amount: string) => (
                  <strong className={amount.startsWith("-") ? "" : "money"}>
                    {amount.startsWith("-") ? "" : "+"}¥{formatFen(amount)}
                  </strong>
                ),
              },
              {
                title: "说明",
                dataIndex: "reason",
                render: (value: string) => value || "—",
              },
              {
                title: "时间",
                dataIndex: "created_at",
                render: (value: string) =>
                  new Date(value).toLocaleString("zh-CN"),
              },
              {
                title: "操作",
                render: (_, row) =>
                  canAdjust && (
                    <Button
                      type="link"
                      onClick={() => {
                        setOriginal(row);
                        setRequestKey(crypto.randomUUID());
                        form.resetFields();
                      }}
                    >
                      关联调整
                    </Button>
                  ),
              },
            ]}
          />
        </QueryState>
      </Card>
      <Modal
        title="创建关联调整"
        open={canAdjust && original !== null}
        onCancel={() => setOriginal(null)}
        onOk={() => form.submit()}
        confirmLoading={adjustment.isPending}
      >
        <Alert
          type="warning"
          showIcon
          message="调整会影响可提现余额，请核对金额和原因。原流水不会被修改。"
          className="form-alert"
        />
        <Form
          form={form}
          layout="vertical"
          onValuesChange={() => setRequestKey(crypto.randomUUID())}
          onFinish={(values) => adjustment.mutate(values)}
        >
          <Form.Item
            name="delta_yuan"
            label="调整金额（元，增加填正数，扣减填负数）"
            rules={[
              { required: true },
              {
                pattern: /^-?(0|[1-9]\d*)(\.\d{1,2})?$/,
                message: "请输入最多两位小数的金额",
              },
            ]}
          >
            <Input inputMode="decimal" placeholder="例如：7.50 或 -7.50" />
          </Form.Item>
          <Form.Item
            name="reason"
            label="调整原因"
            rules={[{ required: true }]}
          >
            <Input.TextArea rows={4} />
          </Form.Item>
        </Form>
      </Modal>
    </Page>
  );
}
