import { lazy, Suspense, useEffect, useState } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  App as AntApp,
  Alert,
  Button,
  ConfigProvider,
  Drawer,
  Layout,
  Result,
  Space,
  Spin,
} from "antd";
import zhCN from "antd/locale/zh_CN";
import {
  BrowserRouter,
  Navigate,
  Route,
  Routes,
  useLocation,
} from "react-router-dom";
import { Login } from "../modules/auth/Login";
import { AuthProvider, useSession } from "./Auth";
import { navigation, WorkspaceNavigation } from "./WorkspaceNavigation";

const Tasks = lazy(() =>
  import("../modules/tasks/TasksPage").then((module) => ({
    default: module.TasksPage,
  })),
);
const Content = lazy(() =>
  import("../modules/content/ContentPage").then((module) => ({
    default: module.ContentPage,
  })),
);
const Users = lazy(() =>
  import("../modules/users/UsersPage").then((module) => ({
    default: module.UsersPage,
  })),
);
const EnrollmentReviews = lazy(() =>
  import("../modules/users/EnrollmentReviewsPage").then((module) => ({
    default: module.EnrollmentReviewsPage,
  })),
);
const IdentityRecords = lazy(() =>
  import("../modules/users/IdentityRecordsPage").then((module) => ({
    default: module.IdentityRecordsPage,
  })),
);
const Reviews = lazy(() =>
  import("../modules/reviews/ReviewsPage").then((module) => ({
    default: module.ReviewsPage,
  })),
);
const Settlement = lazy(() =>
  import("../modules/settlement/SettlementPage").then((module) => ({
    default: module.SettlementPage,
  })),
);
const Withdrawals = lazy(() =>
  import("../modules/finance/WithdrawalsPage").then((module) => ({
    default: module.WithdrawalsPage,
  })),
);
const Ledger = lazy(() =>
  import("../modules/finance/LedgerPage").then((module) => ({
    default: module.LedgerPage,
  })),
);
const Agreements = lazy(() =>
  import("../modules/support/AgreementsPage").then((module) => ({
    default: module.AgreementsPage,
  })),
);
const Feedback = lazy(() =>
  import("../modules/support/FeedbackPage").then((module) => ({
    default: module.FeedbackPage,
  })),
);
const Audit = lazy(() =>
  import("../modules/support/AuditPage").then((module) => ({
    default: module.AuditPage,
  })),
);
const Operators = lazy(() =>
  import("../modules/operators/OperatorsPage").then((module) => ({
    default: module.OperatorsPage,
  })),
);

const pages = {
  "/tasks": { component: Tasks, read: "GET /tasks" },
  "/contents": { component: Content, read: "GET /contents" },
  "/users": { component: EnrollmentReviews, read: "GET /users" },
  "/users/manage": {
    component: Users,
    read: "GET /users",
    permission: "/users",
  },
  "/admin/realname": {
    component: IdentityRecords,
    read: "GET /identity-verifications",
  },
  "/reviews": { component: Reviews, read: "GET /assignments" },
  "/settlements": { component: Settlement },
  "/withdrawals": { component: Withdrawals, read: "GET /withdrawals" },
  "/ledger": { component: Ledger, read: "GET /ledger" },
  "/agreements": { component: Agreements, read: "GET /agreements" },
  "/feedback": { component: Feedback, read: "GET /feedback" },
  "/audits": { component: Audit, read: "GET /audits" },
  "/operators": { component: Operators },
};

const prototypePages = new Map([
  ["#/admin/realname", "/admin/realname"],
  ["#/admin/mkt-tasks", "/tasks"],
  ["#/admin/mktapps", "/users"],
  ["#/admin/mkt-people", "/users/manage"],
  ["#/admin/mkt-works", "/reviews"],
  ["#/admin/mktpay", "/settlements"],
  ["#/admin/mkt-wd", "/withdrawals"],
]);

function Workspace() {
  const session = useSession();
  const { hash } = useLocation();
  const [navigationOpen, setNavigationOpen] = useState(false);

  useEffect(() => {
    const desktop = window.matchMedia("(min-width: 768px)");

    function closeDesktopNavigation() {
      if (desktop.matches) {
        setNavigationOpen(false);
      }
    }

    desktop.addEventListener("change", closeDesktopNavigation);

    return () => desktop.removeEventListener("change", closeDesktopNavigation);
  }, []);

  if (!session.token) {
    return <Login />;
  }

  if (!session.operator) {
    return (
      <div className="state-panel">
        {session.loading ? (
          <Spin />
        ) : (
          <Alert
            type="error"
            message={session.error?.message ?? "无法加载运营账号"}
            action={
              <Space>
                <Button onClick={session.reload}>重试</Button>
                <Button onClick={session.logout}>退出</Button>
              </Space>
            }
          />
        )}
      </div>
    );
  }

  const firstPage = navigation
    .flatMap((group) => group.children)
    .find(
      ({ key, permission, upcoming }) =>
        !upcoming && session.canPage(permission ?? key),
    );

  const prototypePage = prototypePages.get(hash);

  return (
    <Layout className="workspace">
      <Layout.Sider width={210} theme="dark" className="sidebar">
        <WorkspaceNavigation />
      </Layout.Sider>
      <Layout className="workspace-main">
        <header className="mobile-topbar">
          <Button
            type="text"
            aria-label="打开导航菜单"
            aria-expanded={navigationOpen}
            onClick={() => setNavigationOpen(true)}
          >
            <span aria-hidden="true">☰</span>
          </Button>
          <strong>白兔严选 · 运营后台</strong>
        </header>
        <Layout.Content className="workspace-content">
          <Suspense
            fallback={
              <div className="state-panel">
                <Spin />
              </div>
            }
          >
            <Routes>
              {prototypePage && (
                <Route
                  path="/"
                  element={<Navigate to={prototypePage} replace />}
                />
              )}
              {Object.entries(pages).map(([path, page]) => {
                const Component = page.component;
                const read = "read" in page ? page.read : undefined;
                const permission =
                  "permission" in page ? page.permission : path;
                let element = <Component />;
                if (!session.canPage(permission)) {
                  element = <Result status="403" title="无权访问此页面" />;
                } else if (read && !session.canCall(read)) {
                  element = (
                    <Result status="403" title="无权读取此页面的数据" />
                  );
                }

                return <Route key={path} path={path} element={element} />;
              })}
              <Route
                path="*"
                element={
                  firstPage ? (
                    <Navigate to={firstPage.key} replace />
                  ) : (
                    <Result title="当前账号尚未分配页面权限" />
                  )
                }
              />
            </Routes>
          </Suspense>
        </Layout.Content>
      </Layout>
      <Drawer
        title="导航菜单"
        placement="left"
        size={260}
        rootClassName="workspace-navigation-drawer"
        open={navigationOpen}
        onClose={() => setNavigationOpen(false)}
        destroyOnHidden
      >
        <WorkspaceNavigation onNavigate={() => setNavigationOpen(false)} />
      </Drawer>
    </Layout>
  );
}

export function Application() {
  const [client] = useState(
    () =>
      new QueryClient({
        defaultOptions: { queries: { retry: 1, staleTime: 10_000 } },
      }),
  );

  return (
    <ConfigProvider
      locale={zhCN}
      button={{ autoInsertSpace: false }}
      theme={{
        token: {
          colorPrimary: "#8b82be",
          colorLink: "#8b82be",
          colorText: "#2d2654",
          colorTextSecondary: "#888888",
          colorBgLayout: "#ffffff",
          colorBorder: "#dddddd",
          colorBorderSecondary: "#eeeeee",
          colorSuccess: "#1e9e50",
          colorError: "#dd3333",
          borderRadius: 6,
          fontSize: 13,
          controlHeight: 30,
          fontFamily:
            '"Noto Sans SC", "Noto Sans CJK SC", "PingFang SC", "Microsoft YaHei", sans-serif',
        },
        components: {
          Menu: {
            darkItemBg: "#2d2654",
            darkItemColor: "#ccccdd",
            darkGroupTitleColor: "#ffffff",
            darkItemDisabledColor: "#b7b1cc",
            darkItemSelectedBg: "#8b82be",
            darkItemSelectedColor: "#ffffff",
            darkItemHoverBg: "#3b3568",
            itemBorderRadius: 0,
            itemMarginInline: 0,
            itemMarginBlock: 0,
            itemHeight: 33,
          },
          Button: {
            controlHeight: 28,
            fontSize: 12,
            fontWeight: 700,
            paddingInline: 10,
            primaryShadow: "none",
            defaultShadow: "none",
          },
          Table: {
            headerBg: "#fafafb",
            headerColor: "#666666",
            headerBorderRadius: 0,
            headerSplitColor: "transparent",
            cellPaddingBlock: 8,
            cellPaddingInline: 10,
          },
        },
      }}
    >
      <AntApp>
        <QueryClientProvider client={client}>
          <AuthProvider>
            <BrowserRouter>
              <Workspace />
            </BrowserRouter>
          </AuthProvider>
        </QueryClientProvider>
      </AntApp>
    </ConfigProvider>
  );
}
