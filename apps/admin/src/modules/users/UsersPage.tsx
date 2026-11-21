import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Button, Card, Input, Space, Table, Tag, Typography } from "antd";
import { get } from "../../api/client";
import { useSession } from "../../app/Auth";
import { useMarketingTags } from "../../components/MarketingTags";
import { Page, QueryState } from "../../components/Page";
import type { User } from "../../generated/models";
import { AccountSearchLink } from "./AccountSearchLink";
import {
  DistributorDialog,
  distributorName,
  DistributorTags,
  type DistributorDialogMode,
} from "./DistributorDialog";
import {
  douyinSearchURL,
  xiaohongshuSearchURL,
} from "./enrollment-profile-links";
import "./distributors.css";

const pageSize = 12;

export function UsersPage() {
  const { canCall } = useSession();
  const canBlock = canCall("POST /users/:id/block");
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: ["users", "distributors"],
    queryFn: () => get<User[]>("/users?view=distributors"),
  });
  const tags = useMarketingTags();
  const tagNames = new Map(tags.data?.map((tag) => [tag.key, tag.name]));
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);
  const [dialog, setDialog] = useState<{
    user: User;
    mode: DistributorDialogMode;
  } | null>(null);

  const keyword = search.trim().toLowerCase();
  const users = (query.data ?? []).filter((user) =>
    [user.name, user.nickname, user.phone, user.douyin, user.xhs].some(
      (value) => value.toLowerCase().includes(keyword),
    ),
  );
  const currentPage = Math.min(
    page,
    Math.max(1, Math.ceil(users.length / pageSize)),
  );

  return (
    <Page
      title="营销分发 · 分发员管理"
      description="查看已通过报名的分发员。拉黑后暂停领取和接收新任务，可随时取消拉黑。"
    >
      <Card>
        <div className="table-toolbar">
          <Input.Search
            aria-label="搜索分发员"
            placeholder="搜索姓名、手机号、抖音号或小红书号"
            value={search}
            onChange={(event) => {
              setSearch(event.target.value);
              setPage(1);
            }}
            allowClear
          />
          <Button
            aria-label="刷新"
            autoInsertSpace={false}
            loading={query.isFetching}
            onClick={() => void query.refetch()}
          >
            刷新
          </Button>
        </div>
        <QueryState
          pending={query.isPending}
          error={query.error}
          retry={() => void query.refetch()}
        >
          <Table<User>
            className="distributor-table"
            rowKey="id"
            dataSource={users}
            pagination={{
              current: currentPage,
              pageSize,
              showSizeChanger: false,
              onChange: setPage,
            }}
            locale={{
              emptyText: keyword
                ? "未找到匹配的分发员"
                : "还没有通过报名的分发员",
            }}
            scroll={{ x: 900 }}
            columns={[
              {
                title: "分发员",
                render: (_, user) => (
                  <div>
                    <Button
                      type="link"
                      autoInsertSpace={false}
                      className="distributor-name"
                      onClick={() => setDialog({ user, mode: "details" })}
                    >
                      {distributorName(user)}
                    </Button>
                    <Typography.Text
                      type="secondary"
                      className="distributor-note"
                    >
                      {user.nickname ? `${user.nickname} · ` : ""}
                      微信号 {user.wechat || "未填写"}
                    </Typography.Text>
                  </div>
                ),
              },
              {
                title: "抖音号",
                dataIndex: "douyin",
                width: 145,
                render: (value: string) => (
                  <AccountSearchLink
                    value={value}
                    href={douyinSearchURL(value)}
                  />
                ),
              },
              {
                title: "小红书号",
                dataIndex: "xhs",
                width: 145,
                render: (value: string) => (
                  <AccountSearchLink
                    value={value}
                    href={xiaohongshuSearchURL(value)}
                  />
                ),
              },
              {
                title: "手机号",
                dataIndex: "phone",
                width: 125,
                render: (value: string) => value || "—",
              },
              {
                title: "标签",
                render: (_, user) => (
                  <DistributorTags user={user} names={tagNames} />
                ),
              },
              {
                title: "状态",
                render: (_, user) => (
                  <>
                    <Tag color={user.blocked ? "red" : "green"}>
                      {user.blocked ? "已拉黑" : "正常"}
                    </Tag>
                    {user.blocked && user.block_info?.reason && (
                      <Typography.Text
                        type="secondary"
                        className="distributor-note"
                      >
                        原因：{user.block_info.reason}
                      </Typography.Text>
                    )}
                  </>
                ),
              },
              {
                title: "操作",
                width: 184,
                render: (_, user) => (
                  <Space wrap>
                    <Button
                      aria-label="详情"
                      size="small"
                      autoInsertSpace={false}
                      onClick={() => setDialog({ user, mode: "details" })}
                    >
                      详情
                    </Button>
                    {canBlock && (
                      <Button
                        aria-label={user.blocked ? "取消拉黑" : "拉黑"}
                        size="small"
                        autoInsertSpace={false}
                        danger={!user.blocked}
                        onClick={() =>
                          setDialog({
                            user,
                            mode: user.blocked ? "unblock" : "block",
                          })
                        }
                      >
                        {user.blocked ? "取消拉黑" : "拉黑"}
                      </Button>
                    )}
                  </Space>
                ),
              },
            ]}
          />
        </QueryState>
      </Card>
      {dialog && (
        <DistributorDialog
          key={`${dialog.user.id}:${dialog.mode}`}
          user={dialog.user}
          mode={dialog.mode}
          canBlock={canBlock}
          tagNames={tagNames}
          onChangeMode={(mode) => setDialog({ ...dialog, mode })}
          onClose={() => setDialog(null)}
          onSaved={() => {
            setDialog(null);
            void queryClient.invalidateQueries({ queryKey: ["users"] });
            void queryClient.invalidateQueries({
              queryKey: ["enrollment-users"],
            });
          }}
        />
      )}
    </Page>
  );
}
