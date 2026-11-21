import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Alert,
  App,
  Button,
  Descriptions,
  Image,
  Modal,
  Pagination,
  Select,
  Space,
  Table,
  Tag,
} from "antd";
import type { ColumnsType } from "antd/es/table";
import { get, getPage } from "../../api/client";
import { useSession } from "../../app/Auth";
import {
  marketingTagOptions,
  useMarketingTags,
} from "../../components/MarketingTags";
import { Page, QueryState } from "../../components/Page";
import { EnrollmentStatus } from "../../generated/enums";
import type { EnrollmentDocuments, User } from "../../generated/models";
import { AccountSearchLink } from "./AccountSearchLink";
import {
  enrollmentAgeLabels,
  EnrollmentReviewDialog,
} from "./EnrollmentReviewDialog";
import {
  enrollmentDocumentsPath,
  EnrollmentPhotoDialog,
} from "./EnrollmentPhotoDialog";
import { EnrollmentTagLibrary } from "./EnrollmentTagLibrary";
import {
  douyinSearchURL,
  xiaohongshuSearchURL,
} from "./enrollment-profile-links";
import {
  IdentityPhotoThumbnails,
  type IdentityPhoto,
} from "./IdentityPhotoThumbnails";
import "./enrollment-reviews.css";

const pageSize = 30;
const dateFormat = new Intl.DateTimeFormat("sv-SE", {
  timeZone: "Asia/Shanghai",
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
});

const statusDisplay = {
  [EnrollmentStatus.EnrollmentPending]: { label: "待审核", style: "pending" },
  [EnrollmentStatus.EnrollmentApproved]: { label: "已通过", style: "approved" },
  [EnrollmentStatus.EnrollmentRejected]: { label: "已拒绝", style: "rejected" },
  [EnrollmentStatus.EnrollmentNone]: { label: "未报名", style: "pending" },
};

const statusOptions = [
  EnrollmentStatus.EnrollmentPending,
  EnrollmentStatus.EnrollmentApproved,
  EnrollmentStatus.EnrollmentRejected,
].map((value) => ({ value, label: statusDisplay[value].label }));

export function EnrollmentReviewsPage() {
  const { canCall } = useSession();
  const canReview = canCall("POST /users/:id/review");
  const canViewDocuments = canCall("GET /users/:id/enrollment-documents");
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const [page, setPage] = useState(1);
  const [status, setStatus] = useState<EnrollmentStatus>();
  const [tagKey, setTagKey] = useState<string>();
  const [tagSearch, setTagSearch] = useState("");
  const params = new URLSearchParams({
    view: "enrollments",
    page: String(page),
  });
  if (status) params.set("status", status);
  if (tagKey) params.set("tag", tagKey);
  if (canViewDocuments) params.set("photos", "1");
  const query = useQuery({
    queryKey: ["enrollment-users", page, status, tagKey, canViewDocuments],
    queryFn: () => getPage<User>(`/users?${params}`),
    gcTime: canViewDocuments ? 0 : undefined,
  });
  const tags = useMarketingTags();
  const [review, setReview] = useState<{
    user: User;
    tagsOnly: boolean;
  } | null>(null);
  const [selected, setSelected] = useState<User | null>(null);
  const [photoPreview, setPhotoPreview] = useState<{
    userID: string;
    enrollmentID?: string;
    side: IdentityPhoto["side"];
  } | null>(null);
  const users = query.data?.rows ?? [];
  const selectedTagKeys = users.flatMap((user) => user.tags ?? []);
  if (tagKey) selectedTagKeys.push(tagKey);
  const tagOptions = marketingTagOptions(tags.data ?? [], selectedTagKeys);
  const searchedKey = tagSearch.trim().replace(/^#/, "");
  if (
    searchedKey &&
    !tagOptions.some(
      ({ value, label }) => value === searchedKey || label === searchedKey,
    )
  ) {
    tagOptions.push({ value: searchedKey, label: searchedKey });
  }

  const columns: ColumnsType<User> = [
    {
      title: "报名人",
      width: 154,
      render: (_, user) => (
        <>
          <Button
            type="link"
            size="small"
            className="enrollment-person"
            onClick={() => setSelected(user)}
          >
            {user.name || user.nickname || "报名人"}
          </Button>
          {user.name && user.nickname && user.name !== user.nickname && (
            <div className="enrollment-muted">{user.nickname}</div>
          )}
        </>
      ),
    },
    {
      title: "抖音号",
      width: 138,
      render: (_, user) => (
        <AccountSearchLink
          value={user.douyin}
          href={douyinSearchURL(user.douyin)}
        />
      ),
    },
    {
      title: "小红书号",
      width: 160,
      render: (_, user) => (
        <AccountSearchLink
          value={user.xhs}
          href={xiaohongshuSearchURL(user.xhs)}
        />
      ),
    },
    {
      title: "身份证照片",
      width: 152,
      render: (_, user) => {
        const photos = canViewDocuments ? user.enrollment_photo_previews : null;
        if (photos?.front_url || photos?.back_url) {
          return (
            <IdentityPhotoThumbnails
              details={photos}
              onPreview={({ side }) =>
                setPhotoPreview({
                  userID: user.id,
                  enrollmentID: user.enrollment_id,
                  side,
                })
              }
              onImageError={() => {
                if (Date.now() - query.dataUpdatedAt >= 90_000) {
                  void query.refetch({ cancelRefetch: false });
                }
              }}
            />
          );
        }

        if (photos?.unavailable) return "预览暂不可用";
        return user.enrollment_has_photos ? "已上传" : "—";
      },
    },
    {
      title: "提交",
      width: 94,
      render: (_, user) =>
        user.enrollment_submitted_at ? (
          <time dateTime={user.enrollment_submitted_at}>
            {dateFormat.format(new Date(user.enrollment_submitted_at))}
          </time>
        ) : (
          "—"
        ),
    },
    {
      title: "状态",
      width: 114,
      render: (_, user) => {
        const status = statusDisplay[user.status];
        return (
          <>
            <Tag className={`enrollment-status-${status.style}`}>
              {status.label}
            </Tag>
            {user.status === EnrollmentStatus.EnrollmentRejected &&
              user.reason && (
                <div className="enrollment-reason">{user.reason}</div>
              )}
          </>
        );
      },
    },
    {
      title: "人工复核",
      width: 128,
      render: (_, user) => {
        const lines = [
          user.enrollment_recent_post === true && "近 30 天有发作品",
          user.enrollment_recent_post === false && "近 30 天没发作品",
          user.enrollment_new_account === true && "是新号",
          user.enrollment_new_account === false && "不是新号",
          user.enrollment_age_group &&
            enrollmentAgeLabels[user.enrollment_age_group],
        ].filter(Boolean);
        return lines.length
          ? lines.map((line) => <div key={String(line)}>{line}</div>)
          : "—";
      },
    },
    {
      title: "标签",
      width: 126,
      render: (_, user) => {
        const assigned = user.tags ?? [];
        const names = marketingTagOptions(tags.data ?? [], assigned).filter(
          (tag) => assigned.includes(tag.value),
        );
        return names.length ? (
          <div className="enrollment-row-tags">
            {names.map((tag) => (
              <Tag key={tag.value} className="platform-tag">
                #{tag.label.replace(/^#/, "")}
              </Tag>
            ))}
          </div>
        ) : (
          "—"
        );
      },
    },
    {
      title: "操作",
      width: 126,
      render: (_, user) => {
        const pending = user.status === EnrollmentStatus.EnrollmentPending;
        const approved = user.status === EnrollmentStatus.EnrollmentApproved;
        if (!canReview || (!pending && !approved)) return "—";

        return (
          <>
            <Button
              size="small"
              className="enrollment-action"
              onClick={() => setReview({ user, tagsOnly: false })}
            >
              {pending ? "去复核" : "改画像 / 标签"}
            </Button>
            {pending && (user.tags?.length ?? 0) > 0 && (
              <div>
                <Button
                  type="link"
                  size="small"
                  onClick={() => setReview({ user, tagsOnly: true })}
                >
                  只改标签
                </Button>
              </div>
            )}
          </>
        );
      },
    },
  ];

  return (
    <Page
      title="营销分发 · 分发员报名审核"
      description="粉丝数不限，抖音和小红书账号均须填写。请查看两端主页，记录近 30 天发布情况、新号情况和账号画像。报名时确认《营销分发任务协议》，审核通过且已同意当前版本后可领取任务。"
    >
      <EnrollmentTagLibrary />
      <Space className="enrollment-filters" wrap>
        <Select<EnrollmentStatus>
          aria-label="按状态筛选"
          allowClear
          placeholder="全部状态"
          value={status}
          options={statusOptions}
          onChange={(value) => {
            setStatus(value);
            setPage(1);
          }}
        />
        <Select<string, { value: string; label: string }>
          aria-label="按标签筛选"
          allowClear
          showSearch={{
            onSearch: setTagSearch,
            filterOption: (term, option) =>
              (option?.label ?? "")
                .toLowerCase()
                .includes(term.trim().replace(/^#/, "").toLowerCase()),
          }}
          placeholder="全部标签"
          value={tagKey}
          options={tagOptions}
          onChange={(value) => {
            setTagKey(value);
            setPage(1);
          }}
        />
        <Button
          onClick={() => {
            if (page > 1) setPage(1);
            else void query.refetch();
          }}
          loading={query.isFetching}
        >
          刷新
        </Button>
      </Space>
      <QueryState
        pending={query.isPending}
        error={query.error}
        retry={() => void query.refetch()}
      >
        <Table<User>
          className="enrollment-table"
          rowKey={(user) => user.enrollment_id ?? user.id}
          columns={columns}
          dataSource={users}
          pagination={false}
          tableLayout="fixed"
          scroll={{ x: 1192 }}
          locale={{
            emptyText:
              status || tagKey ? "没有符合筛选条件的报名" : "暂无分发员报名",
          }}
        />
        <nav className="enrollment-pagination" aria-label="报名审核分页">
          <Pagination
            size="small"
            current={page}
            pageSize={pageSize}
            total={query.data?.total ?? 0}
            showTotal={(total) => `共 ${total} 条`}
            showSizeChanger={false}
            onChange={setPage}
          />
        </nav>
      </QueryState>
      {selected && (
        <ApplicantDetails
          user={selected}
          canViewDocuments={canViewDocuments}
          onClose={() => setSelected(null)}
        />
      )}
      {canViewDocuments && photoPreview && (
        <EnrollmentPhotoDialog
          userID={photoPreview.userID}
          enrollmentID={photoPreview.enrollmentID}
          side={photoPreview.side}
          onClose={() => setPhotoPreview(null)}
        />
      )}
      {review && (
        <EnrollmentReviewDialog
          user={review.user}
          tagsOnly={review.tagsOnly}
          tags={tags.data ?? []}
          onClose={() => setReview(null)}
          onSaved={() => {
            setReview(null);
            setPage(1);
            message.success(
              review.tagsOnly ? "已保存报名标签" : "已保存报名复核",
            );
            void queryClient.invalidateQueries({
              queryKey: ["enrollment-users"],
            });
            void queryClient.invalidateQueries({ queryKey: ["users"] });
          }}
        />
      )}
    </Page>
  );
}

function ApplicantDetails({
  user,
  canViewDocuments,
  onClose,
}: {
  user: User;
  canViewDocuments: boolean;
  onClose: () => void;
}) {
  const documents = useQuery({
    queryKey: ["enrollment-documents", user.id, user.enrollment_id],
    queryFn: () =>
      get<EnrollmentDocuments>(
        enrollmentDocumentsPath(user.id, user.enrollment_id),
      ),
    enabled: canViewDocuments,
    gcTime: 0,
    retry: false,
  });
  const details = documents.data?.details;
  const photos = [
    { label: "人像面", url: details?.front_url },
    { label: "国徽面", url: details?.back_url },
  ];

  return (
    <Modal title="报名人资料" open onCancel={onClose} footer={null} width={560}>
      <Descriptions
        column={1}
        size="small"
        items={[
          {
            key: "name",
            label: "姓名",
            children: user.name || details?.name || "—",
          },
          {
            key: "nickname",
            label: "微信昵称",
            children: user.nickname || "—",
          },
          ...(canViewDocuments
            ? [
                {
                  key: "wechat",
                  label: "微信号",
                  children: details?.wechat || "—",
                },
                {
                  key: "identity",
                  label: "身份证号",
                  children: details?.identity || "—",
                },
              ]
            : []),
          {
            key: "phone",
            label: "手机号",
            children: details?.phone || user.phone || "—",
          },
          {
            key: "dy",
            label: "抖音号",
            children: (
              <AccountSearchLink
                value={user.douyin}
                href={douyinSearchURL(user.douyin)}
              />
            ),
          },
          {
            key: "xhs",
            label: "小红书号",
            children: (
              <AccountSearchLink
                value={user.xhs}
                href={xiaohongshuSearchURL(user.xhs)}
              />
            ),
          },
        ]}
      />
      {canViewDocuments && (
        <QueryState
          pending={documents.isPending}
          error={documents.error}
          retry={() => void documents.refetch()}
        >
          <Space direction="vertical" className="enrollment-document-images">
            {documents.data?.photos_unavailable && (
              <Alert
                type="warning"
                title="暂时无法生成证件照片预览"
                action={
                  <Button onClick={() => void documents.refetch()}>重试</Button>
                }
              />
            )}
            <Space wrap>
              {photos.map(({ label, url }) => (
                <div key={label} className="enrollment-document-image">
                  {url ? (
                    <Image
                      src={url}
                      alt={`身份证${label}`}
                      width={148}
                      referrerPolicy="no-referrer"
                    />
                  ) : (
                    <span className="enrollment-muted">暂无照片</span>
                  )}
                  <div>{label}</div>
                </div>
              ))}
            </Space>
          </Space>
        </QueryState>
      )}
    </Modal>
  );
}
