import { useState } from "react";
import { ReloadOutlined } from "@ant-design/icons";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Alert, App, Button, Pagination } from "antd";
import { get, getPage, post } from "../../api/client";
import { useSession } from "../../app/Auth";
import { useMarketingTags } from "../../components/MarketingTags";
import { Page, QueryState } from "../../components/Page";
import { AssignmentStatus, Platform } from "../../generated/enums";
import type { Assignment, User } from "../../generated/models";
import { assignmentLabels, platformLabels } from "../../shared/status";
import { DistributorDialog } from "../users/DistributorDialog";
import "../users/distributors.css";
import { AssignmentReviewDrawer } from "./AssignmentReviewDrawer";
import {
  ReviewDialog,
  type ReviewDecision,
  type ReviewTarget,
} from "./ReviewDialog";
import { ReviewTable } from "./ReviewTable";
import "./reviews.css";

const pageSize = 12;

export function ReviewsPage() {
  const { canCall } = useSession();
  const canReadUsers = canCall("GET /users");
  const { message } = App.useApp();
  const cache = useQueryClient();
  const [page, setPage] = useState(1);
  const [status, setStatus] = useState("");
  const [platform, setPlatform] = useState("");
  const [selectedID, setSelectedID] = useState<string | null>(null);
  const [distributor, setDistributor] = useState<User | null>(null);
  const [review, setReview] = useState<ReviewTarget | null>(null);
  const params = new URLSearchParams({ page: String(page) });
  if (status) params.set("status", status);
  if (platform) params.set("platform", platform);

  const query = useQuery({
    queryKey: ["assignments", "reviews", page, status, platform],
    queryFn: () => getPage<Assignment>(`/assignments?${params}`),
  });
  const users = useQuery({
    queryKey: ["users", "distributors"],
    queryFn: () => get<User[]>("/users?view=distributors"),
    enabled: canReadUsers,
  });
  const tags = useMarketingTags();
  const usersByID = new Map(
    (canReadUsers ? (users.data ?? []) : []).map((user) => [user.id, user]),
  );
  const selected = query.data?.rows.find((row) => row.id === selectedID);
  const total = query.data?.total ?? 0;
  const lastPage = Math.max(1, Math.ceil(total / pageSize));
  if (query.data && page > lastPage) setPage(lastPage);

  const mutation = useMutation({
    mutationFn: ({ target, submissions, reason, keepPay }: ReviewDecision) => {
      const path = `/assignments/${target.assignment.id}`;
      const [submission] = submissions;
      if (target.action === "flag") {
        return post(`${path}/reissue`, {
          platform: submission.platform,
          reason,
          keep_pay: keepPay,
        });
      }

      if (target.action === "reject" && submissions.length > 1) {
        return post(`${path}/review`, {
          submissions: submissions.map(({ platform, revision }) => ({
            platform,
            revision,
          })),
          reason,
        });
      }

      return post(`${path}/review`, {
        platform: submission.platform,
        revision: submission.revision,
        approved: target.action === "approve",
        reason,
      });
    },
    onSuccess: async () => {
      await cache.invalidateQueries({ queryKey: ["assignments"] });
      setReview(null);
      message.success("处理结果已保存");
    },
    onError: () => {
      void cache.invalidateQueries({ queryKey: ["assignments"] });
    },
  });

  function openReview(target: ReviewTarget) {
    mutation.reset();
    setReview(target);
  }

  function refresh() {
    void query.refetch();
    if (canReadUsers) void users.refetch();
  }

  return (
    <Page
      title="营销分发 · 发布链接审核"
      description="核对各平台的发布内容、分发语和必带 TAG。驳回原因会在分发员的小程序中显示。"
    >
      <div
        className="marketing-filters review-filters"
        role="group"
        aria-label="审核筛选"
      >
        <div className="marketing-filter-field">
          <label htmlFor="publish-review-status">状态</label>
          <select
            id="publish-review-status"
            value={status}
            onChange={(event) => {
              setStatus(event.target.value);
              setPage(1);
              setSelectedID(null);
            }}
          >
            <option value="">全部状态</option>
            {Object.values(AssignmentStatus).map((value) => (
              <option key={value} value={value}>
                {assignmentLabels[value]}
              </option>
            ))}
          </select>
        </div>
        <div className="marketing-filter-field">
          <label htmlFor="publish-review-filter-platform">发布平台</label>
          <select
            id="publish-review-filter-platform"
            value={platform}
            onChange={(event) => {
              setPlatform(event.target.value);
              setPage(1);
              setSelectedID(null);
            }}
          >
            <option value="">全部平台</option>
            {Object.values(Platform).map((value) => (
              <option key={value} value={value}>
                {platformLabels[value]}
              </option>
            ))}
          </select>
        </div>
        <Button
          aria-label="刷新"
          icon={<ReloadOutlined />}
          loading={query.isFetching || (canReadUsers && users.isFetching)}
          onClick={refresh}
        >
          刷新
        </Button>
      </div>
      {canReadUsers && users.isError && (
        <Alert type="warning" title="分发员信息加载失败，请刷新重试" showIcon />
      )}
      <QueryState pending={query.isPending} error={query.error} retry={refresh}>
        <ReviewTable
          rows={query.data?.rows ?? []}
          users={usersByID}
          onOpenAssignment={setSelectedID}
          onOpenDistributor={setDistributor}
          onReview={openReview}
        />
        <Pagination
          className="review-pagination"
          current={page}
          total={total}
          pageSize={pageSize}
          showSizeChanger={false}
          showTotal={(count) => `共 ${count} 条`}
          onChange={(nextPage) => {
            setSelectedID(null);
            setPage(nextPage);
          }}
        />
      </QueryState>
      {selected && (
        <AssignmentReviewDrawer
          assignment={selected}
          user={usersByID.get(selected.user_id)}
          onClose={() => setSelectedID(null)}
          onReview={openReview}
        />
      )}
      {canReadUsers && distributor && (
        <DistributorDialog
          user={distributor}
          mode="details"
          canBlock={false}
          tagNames={new Map(tags.data?.map((tag) => [tag.key, tag.name]))}
          onChangeMode={() => {}}
          onClose={() => setDistributor(null)}
          onSaved={() => setDistributor(null)}
        />
      )}
      {review &&
        canCall(
          review.action === "flag"
            ? "POST /assignments/:id/reissue"
            : "POST /assignments/:id/review",
        ) && (
          <ReviewDialog
            target={review}
            user={usersByID.get(review.assignment.user_id)}
            pending={mutation.isPending}
            error={mutation.error}
            onConfirm={mutation.mutate}
            onClose={() => setReview(null)}
          />
        )}
    </Page>
  );
}
