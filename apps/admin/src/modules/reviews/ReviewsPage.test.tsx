import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { App } from "antd";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { getPage, post } from "../../api/client";
import {
  AssignmentSource,
  AssignmentStatus,
  Platform,
  SubmissionStatus,
  Visibility,
} from "../../generated/enums";
import type { Assignment } from "../../generated/models";
import { ReviewsPage } from "./ReviewsPage";

const permissions = vi.hoisted(() => new Set<string>());

vi.mock("../../app/Auth", () => ({
  useSession: () => ({
    canCall: (endpoint: string) => permissions.has(endpoint),
  }),
}));

vi.mock("../../api/client", () => ({
  get: vi.fn(),
  getPage: vi.fn(),
  post: vi.fn(),
}));

const assignment: Assignment = {
  id: "assignment-D12",
  title: "候场时的小互动",
  user_id: "user-one",
  task_id: "task-one",
  version_id: "version-one",
  price_fen: "750",
  body: "带指定 TAG",
  files: [],
  platforms: [Platform.PlatformDouyin, Platform.PlatformXiaohongshu],
  status: AssignmentStatus.AssignmentPendingReview,
  source: AssignmentSource.SourceClaim,
  source_id: "",
  date: "2026-11-19",
  due_date: "2026-11-19",
  steps: [],
  submissions: [Platform.PlatformDouyin, Platform.PlatformXiaohongshu].map(
    (platform) => ({
      id: platform,
      platform,
      url:
        platform === Platform.PlatformDouyin
          ? "https://v.douyin.com/aaa/"
          : "https://xhslink.com/a/bbb",
      screenshot: "",
      status: SubmissionStatus.SubmissionPending,
      reason: "",
      revision: platform === Platform.PlatformDouyin ? 2 : 4,
      settled: false,
      visibility: Visibility.VisibilityUnchecked,
    }),
  ),
};

beforeEach(() => {
  vi.resetAllMocks();
  permissions.clear();
  window.matchMedia = vi.fn().mockReturnValue({
    matches: false,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  });
  vi.mocked(getPage).mockResolvedValue({ rows: [assignment], total: 25 });
});

afterEach(cleanup);

function showReviews() {
  render(
    <QueryClientProvider
      client={
        new QueryClient({
          defaultOptions: { queries: { retry: false, gcTime: 0 } },
        })
      }
    >
      <App>
        <ReviewsPage />
      </App>
    </QueryClientProvider>,
  );
}

it("满宽审核列表显示作品链接，服务端分页、筛选和刷新共享当前筛选条件", async () => {
  showReviews();
  await screen.findByRole("table", { name: "发布链接审核" });
  expect(
    screen.getAllByRole("columnheader").map((header) => header.textContent),
  ).toEqual(["ID", "类型", "任务", "分发员", "链接", "提交", "状态", "操作"]);
  expect(
    screen
      .getByRole("link", { name: "https://xhslink.com/a/bbb" })
      .getAttribute("href"),
  ).toBe("https://xhslink.com/a/bbb");
  expect(screen.queryByRole("button", { name: "驳回" })).toBeNull();
  fireEvent.click(within(screen.getByTitle("2")).getByRole("button"));
  await waitFor(() =>
    expect(getPage).toHaveBeenLastCalledWith("/assignments?page=2"),
  );
  fireEvent.change(screen.getByLabelText("状态"), {
    target: { value: "pending_review" },
  });
  await waitFor(() =>
    expect(getPage).toHaveBeenLastCalledWith(
      "/assignments?page=1&status=pending_review",
    ),
  );
  fireEvent.change(screen.getByLabelText("发布平台"), {
    target: { value: "xhs" },
  });
  await waitFor(() =>
    expect(getPage).toHaveBeenLastCalledWith(
      "/assignments?page=1&status=pending_review&platform=xhs",
    ),
  );
  await screen.findByRole("table", { name: "发布链接审核" });
  const requests = vi.mocked(getPage).mock.calls.length;
  fireEvent.click(screen.getByRole("button", { name: "刷新" }));
  await waitFor(() => expect(getPage).toHaveBeenCalledTimes(requests + 1));
  expect(getPage).toHaveBeenLastCalledWith(
    "/assignments?page=1&status=pending_review&platform=xhs",
  );
});

it("驳回须选择待审核平台并填写原因，只发送所选平台和其审核版本", async () => {
  permissions.add("POST /assignments/:id/review");
  showReviews();
  fireEvent.click(await screen.findByRole("button", { name: /^驳回$/ }));
  const dialog = await screen.findByRole("dialog", { name: "驳回发布链接" });
  const confirm = within(dialog).getByRole("button", { name: "确认驳回" });
  const reason = within(dialog).getByLabelText("驳回原因");
  expect(within(dialog).getByText(/候场时的小互动/)).toBeTruthy();
  fireEvent.click(confirm);
  expect(await within(dialog).findByText("请选择发布平台")).toBeTruthy();
  expect(post).not.toHaveBeenCalled();
  fireEvent.click(within(dialog).getByRole("checkbox", { name: "小红书" }));
  fireEvent.change(reason, { target: { value: "   " } });
  fireEvent.click(confirm);
  expect(await within(dialog).findByText("请填写驳回原因")).toBeTruthy();
  expect(post).not.toHaveBeenCalled();
  fireEvent.change(reason, { target: { value: "  缺少必带话题  " } });
  fireEvent.click(confirm);
  await waitFor(() =>
    expect(post).toHaveBeenCalledExactlyOnceWith(
      "/assignments/assignment-D12/review",
      { platform: "xhs", revision: 4, approved: false, reason: "缺少必带话题" },
    ),
  );
});

it("勾选两个平台后以同一个原因提交两个审核版本", async () => {
  permissions.add("POST /assignments/:id/review");
  showReviews();
  fireEvent.click(await screen.findByRole("button", { name: /^驳回$/ }));
  const dialog = within(
    await screen.findByRole("dialog", { name: "驳回发布链接" }),
  );
  fireEvent.click(dialog.getByRole("checkbox", { name: "抖音" }));
  fireEvent.click(dialog.getByRole("checkbox", { name: "小红书" }));
  fireEvent.change(dialog.getByLabelText("驳回原因"), {
    target: { value: "需要补齐指定话题" },
  });
  fireEvent.click(dialog.getByRole("button", { name: "确认驳回" }));
  await waitFor(() =>
    expect(post).toHaveBeenCalledExactlyOnceWith(
      "/assignments/assignment-D12/review",
      {
        submissions: [
          { platform: "dy", revision: 2 },
          { platform: "xhs", revision: 4 },
        ],
        reason: "需要补齐指定话题",
      },
    ),
  );
});
