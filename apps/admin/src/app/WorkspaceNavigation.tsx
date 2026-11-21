import { Button, Menu } from "antd";
import { useLocation, useNavigate } from "react-router-dom";
import { useSession } from "./Auth";

const upcomingGroups = [
  {
    key: "upcoming-ai",
    label: "AI选品带货",
    entries: ["任务管理", "收益汇总", "发布链接审核"],
  },
  {
    key: "upcoming-creators",
    label: "达人切片带货",
    entries: ["任务管理", "达人授权申请", "发布链接审核", "收益汇总"],
  },
  {
    key: "upcoming-shoots",
    label: "素人素材拍摄",
    entries: ["拍摄任务管理", "寄样发货", "素材审核", "素材回收（点数结算）"],
  },
];

interface NavigationGroup {
  key: string;
  label: string;
  children: {
    key: string;
    label: string;
    permission?: string;
    upcoming?: boolean;
  }[];
  upcoming?: string[];
}

export const navigation: NavigationGroup[] = [
  {
    key: "marketing",
    label: "营销分发",
    children: [
      { key: "/tasks", label: "任务管理" },
      { key: "/users", label: "分发员报名审核" },
      { key: "/users/manage", label: "分发员管理", permission: "/users" },
      { key: "/reviews", label: "发布链接审核" },
      {
        key: "marketing-feishu",
        label: "飞书发布提醒（示意）",
        upcoming: true,
      },
      { key: "marketing-data", label: "数据统计", upcoming: true },
      { key: "/settlements", label: "分发结算" },
      { key: "/withdrawals", label: "提现记录" },
    ],
  },
  {
    key: "enrollment",
    label: "报名审核",
    upcoming: ["挂车账号报名审核"],
    children: [{ key: "/admin/realname", label: "实名认证记录" }],
  },
  {
    key: "general",
    label: "通用管理",
    upcoming: ["货盘", "点数规则", "点数调整记录"],
    children: [
      { key: "/ledger", label: "余额流水" },
      { key: "/agreements", label: "分发协议" },
      { key: "/feedback", label: "反馈与咨询" },
      { key: "/audits", label: "操作审计" },
      { key: "/operators", label: "运营账号" },
    ],
  },
];

function UpcomingLabel({ title }: { title: string }) {
  return (
    <span className="sidebar-upcoming">
      <span>{title}</span>
      <span className="sidebar-upcoming-badge">即将上线</span>
    </span>
  );
}

export function WorkspaceNavigation({
  onNavigate,
}: {
  onNavigate?: () => void;
}) {
  const session = useSession();
  const location = useLocation();
  const navigate = useNavigate();
  const items = [
    ...upcomingGroups.map((group) => ({
      type: "group" as const,
      key: group.key,
      label: <UpcomingLabel title={group.label} />,
      children: group.entries.map((label) => ({
        key: `${group.key}-${label}`,
        label,
        disabled: true,
      })),
    })),
    ...navigation.map((group) => {
      const canSee = ({ key, permission }: (typeof group.children)[number]) =>
        session.canPage(permission ?? key);
      const hasActive = group.children.some(
        (entry) => !entry.upcoming && canSee(entry),
      );
      const children = group.children
        .filter((entry) => (entry.upcoming ? hasActive : canSee(entry)))
        .map(({ key, label, upcoming }) => ({
          key,
          label: upcoming ? <UpcomingLabel title={label} /> : label,
          disabled: upcoming,
        }));

      return {
        type: "group" as const,
        key: group.key,
        label: group.label,
        children: [
          ...(group.upcoming ?? []).map((label) => ({
            key: `${group.key}-${label}`,
            label: <UpcomingLabel title={label} />,
            disabled: true,
          })),
          ...children,
        ],
      };
    }),
  ].filter((group) => group.children.length > 0);

  return (
    <nav className="workspace-nav" aria-label="管理后台导航">
      <div className="sidebar-brand">
        <strong>白兔严选 · 运营后台</strong>
        <span>营销分发 · 第一阶段</span>
      </div>
      <Menu
        mode="inline"
        theme="dark"
        selectedKeys={[
          location.pathname === "/contents" && session.canPage("/tasks")
            ? "/tasks"
            : location.pathname,
        ]}
        items={items}
        onClick={({ key }) => {
          navigate(key);
          onNavigate?.();
        }}
      />
      <div className="sidebar-foot">
        <span>{session.operator?.username}</span>
        <Button
          type="text"
          onClick={() => {
            onNavigate?.();
            session.logout();
          }}
        >
          退出
        </Button>
      </div>
    </nav>
  );
}
