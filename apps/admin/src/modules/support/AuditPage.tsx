import { useQuery } from "@tanstack/react-query";
import { Card, Table, Tag } from "antd";
import { get } from "../../api/client";
import { Page, QueryState } from "../../components/Page";
import { AuditAction } from "../../generated/enums";
import type { AuditEvent } from "../../generated/models";

const labels: Record<AuditAction, string> = {
  [AuditAction.AuditEnrollmentApply]: "提交报名",
  [AuditAction.AuditEnrollmentApproved]: "报名审核通过",
  [AuditAction.AuditUserIdentityVerify]: "实名认证",
  [AuditAction.AuditUserIdentityFailed]: "实名认证失败",
  [AuditAction.AuditEnrollmentIdentityVerify]: "报名实名核验通过",
  [AuditAction.AuditEnrollmentIdentityFailed]: "报名实名核验失败",
  [AuditAction.AuditEnrollmentRejected]: "报名驳回",
  [AuditAction.AuditEnrollmentPortrait]: "更新分发员画像",
  [AuditAction.AuditMarketingTagCreate]: "添加营销标签",
  [AuditAction.AuditMarketingTagRename]: "重命名营销标签",
  [AuditAction.AuditMarketingTagDelete]: "删除营销标签",
  [AuditAction.AuditContentTags]: "修改内容池标签",
  [AuditAction.AuditUserBlock]: "调整领取资格",
  [AuditAction.AuditUserProfileUpdate]: "更新个人信息",
  [AuditAction.AuditUserPhoneVerify]: "验证微信手机号",
  [AuditAction.AuditAgreementConsent]: "同意协议",
  [AuditAction.AuditAgreementPublish]: "发布协议",
  [AuditAction.AuditTaskCreate]: "新建任务",
  [AuditAction.AuditTaskPublishVersion]: "发布任务版本",
  [AuditAction.AuditTaskVisibility]: "调整任务上下架",
  [AuditAction.AuditTaskClaim]: "领取任务",
  [AuditAction.AuditTaskAssign]: "指派任务",
  [AuditAction.AuditTaskRevoke]: "撤回指派",
  [AuditAction.AuditTaskReissue]: "重新派发",
  [AuditAction.AuditTaskExpire]: "任务自动失效",
  [AuditAction.AuditMarketingSettings]: "修改营销分发设置",
  [AuditAction.AuditSubmissionSubmit]: "提交发布记录",
  [AuditAction.AuditSubmissionApproved]: "平台审核通过",
  [AuditAction.AuditSubmissionRejected]: "平台审核驳回",
  [AuditAction.AuditSubmissionWaive]: "素材问题照常结算",
  [AuditAction.AuditSettlementCredit]: "结算入账",
  [AuditAction.AuditWithdrawalRequest]: "申请提现",
  [AuditAction.AuditLedgerAdjust]: "关联流水调整",
  [AuditAction.AuditContentCreate]: "新增内容",
  [AuditAction.AuditFeedbackReply]: "回复反馈",
  [AuditAction.AuditOperatorCreate]: "创建运营账号",
  [AuditAction.AuditOperatorUpdate]: "更新运营账号",
};

export function AuditPage() {
  const query = useQuery({
    queryKey: ["audits"],
    queryFn: () => get<AuditEvent[]>("/audits"),
  });

  return (
    <Page
      title="操作审计"
      description="关键业务操作自动留痕，记录操作人、原因和关联业务。"
    >
      <Card>
        <QueryState
          pending={query.isPending}
          error={query.error}
          retry={() => void query.refetch()}
        >
          <Table<AuditEvent>
            rowKey="id"
            dataSource={query.data}
            pagination={{ pageSize: 15 }}
            columns={[
              {
                title: "操作",
                dataIndex: "action",
                render: (action: AuditAction) => <Tag>{labels[action]}</Tag>,
              },
              {
                title: "操作人",
                dataIndex: "actor_id",
                render: (id: string, event) => {
                  if (event.actor_username) {
                    return event.actor_username;
                  }

                  return id === "admin"
                    ? "原共享管理员"
                    : `分发员 ${id.slice(-6)}`;
                },
              },
              {
                title: "原因 / 备注",
                dataIndex: "reason",
                render: (value: string) => value || "—",
              },
              { title: "关联编号", dataIndex: "reference_id", ellipsis: true },
              {
                title: "时间",
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
