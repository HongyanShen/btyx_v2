import { useState } from "react";
import { useInfiniteQuery, useQueryClient } from "@tanstack/react-query";
import { Alert, Button, Modal, Pagination, Table, Tag } from "antd";
import { get } from "../../api/client";
import { useSession } from "../../app/Auth";
import { Page, QueryState } from "../../components/Page";
import { AuditAction } from "../../generated/enums";
import type { IdentityRecord, IdentityRecords } from "../../generated/models";
import {
  IdentityPhotoThumbnails,
  type IdentityPhoto,
} from "./IdentityPhotoThumbnails";

function isEnrollment(action: AuditAction) {
  return (
    action === AuditAction.AuditEnrollmentIdentityVerify ||
    action === AuditAction.AuditEnrollmentIdentityFailed
  );
}

function isSuccess(action: AuditAction) {
  return (
    action === AuditAction.AuditUserIdentityVerify ||
    action === AuditAction.AuditEnrollmentIdentityVerify
  );
}

export function IdentityRecordsPage() {
  const { canCall } = useSession();
  const canViewDetails = canCall("GET /identity-verifications/documents");
  const [currentPage, setCurrentPage] = useState(1);
  const [preview, setPreview] = useState<IdentityPhoto | null>(null);
  const [previewFailed, setPreviewFailed] = useState(false);
  const queryClient = useQueryClient();
  const endpoint = canViewDetails
    ? "/identity-verifications/documents"
    : "/identity-verifications";
  const queryKey = [endpoint];
  const query = useInfiniteQuery({
    queryKey,
    gcTime: canViewDetails ? 0 : undefined,
    initialPageParam: "",
    queryFn: ({ pageParam }) =>
      get<IdentityRecords>(
        pageParam
          ? `${endpoint}?before=${encodeURIComponent(pageParam)}`
          : endpoint,
      ),
    getNextPageParam: (lastPage) => lastPage.next_before || undefined,
  });
  const pages = query.data?.pages ?? [];
  const visiblePage = Math.min(currentPage, pages.length || 1);
  const result = pages[visiblePage - 1];
  // The API supplies the next cursor instead of a total record count.
  const availablePageCount = pages.length + (query.hasNextPage ? 1 : 0);

  function refresh() {
    setCurrentPage(1);
    void queryClient.resetQueries({
      queryKey,
      exact: true,
    });
  }

  async function changePage(page: number) {
    if (pages[page - 1]) {
      setCurrentPage(page);
      return;
    }

    const next = await query.fetchNextPage({ cancelRefetch: false });
    if (!next.isFetchNextPageError && next.data?.pages[page - 1]) {
      setCurrentPage(page);
    }
  }

  return (
    <Page
      title="报名审核 · 实名认证记录"
      description={
        canViewDetails
          ? "查看每次核验留存的脱敏证件资料、照片及当次提交的联系方式。缺失字段显示「—」。"
          : "查看自动核验结果及失败原因。查看实名资料和照片需单独授权。"
      }
      extra={
        <Button
          aria-label="刷新"
          loading={query.isFetching && !query.isFetchingNextPage}
          disabled={query.isFetchingNextPage}
          onClick={refresh}
        >
          刷新
        </Button>
      }
    >
      <QueryState
        pending={query.isPending}
        error={query.isFetchNextPageError ? null : query.error}
        retry={() => void query.refetch()}
      >
        <Table<IdentityRecord>
          className="identity-records-table"
          rowKey="id"
          dataSource={result?.records}
          pagination={false}
          tableLayout="fixed"
          scroll={{ x: 1200 }}
          columns={[
            {
              title: "用户",
              width: "10%",
              render: (_, record) =>
                record.nickname || `分发员 ${record.user_id.slice(-6)}`,
            },
            {
              title: "姓名",
              width: "6%",
              render: (_, record) => record.details?.name || "—",
            },
            {
              title: "身份证号",
              width: "12%",
              render: (_, record) => record.details?.identity || "—",
            },
            {
              title: "手机号",
              width: "8.5%",
              render: (_, record) => record.details?.phone || "—",
            },
            {
              title: "微信号",
              width: "11%",
              render: (_, record) => record.details?.wechat || "—",
            },
            {
              title: "身份证照片",
              width: "15%",
              render: (_, record) => (
                <IdentityPhotoThumbnails
                  details={record.details}
                  onPreview={(photo) => {
                    setPreviewFailed(false);
                    setPreview(photo);
                  }}
                />
              ),
            },
            {
              title: "来源",
              dataIndex: "action",
              width: "8%",
              render: (action: AuditAction) =>
                isEnrollment(action) ? "分发员报名" : "独立实名",
            },
            {
              title: "提交",
              dataIndex: "created_at",
              width: "13%",
              render: (value: string) =>
                new Date(value).toLocaleString("zh-CN", {
                  timeZone: "Asia/Shanghai",
                }),
            },
            {
              title: "状态",
              dataIndex: "action",
              width: "5.5%",
              render: (action: AuditAction) =>
                isSuccess(action) ? (
                  <Tag className="identity-result-success">成功</Tag>
                ) : (
                  <Tag className="identity-result-failure">失败</Tag>
                ),
            },
            {
              title: "失败原因",
              dataIndex: "reason",
              width: "11%",
              render: (reason: string) => reason || "—",
            },
          ]}
        />
        {result?.photos_unavailable && (
          <Alert
            className="identity-records-page-error"
            type="warning"
            title="部分证件照片暂时无法预览，请稍后刷新。"
          />
        )}
        {query.isFetchNextPageError && (
          <Alert
            className="identity-records-page-error"
            type="error"
            title={query.error?.message}
          />
        )}
        {availablePageCount > 0 && (
          <nav
            className="identity-records-pagination"
            aria-label="实名认证记录分页"
            aria-busy={query.isFetchingNextPage}
          >
            <Pagination
              current={visiblePage}
              pageSize={1}
              total={availablePageCount}
              showSizeChanger={false}
              showLessItems
              disabled={query.isFetching}
              onChange={(page) => void changePage(page)}
            />
          </nav>
        )}
      </QueryState>
      <Modal
        title={preview ? `身份证${preview.side}` : undefined}
        open={canViewDetails && preview !== null}
        onCancel={() => setPreview(null)}
        closable={{ "aria-label": "关闭" }}
        footer={null}
        width={860}
        centered
        destroyOnHidden
      >
        {preview &&
          (previewFailed ? (
            <Alert
              type="warning"
              title="照片加载失败，请重新获取预览。"
              action={
                <Button
                  onClick={() => {
                    setPreview(null);
                    void query.refetch();
                  }}
                >
                  重新获取
                </Button>
              }
            />
          ) : (
            <img
              className="identity-photo-preview"
              src={preview.url}
              alt={`身份证${preview.side}大图`}
              referrerPolicy="no-referrer"
              onError={() => setPreviewFailed(true)}
            />
          ))}
      </Modal>
    </Page>
  );
}
