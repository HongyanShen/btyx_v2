import { useMutation, useQueryClient } from "@tanstack/react-query";
import { App, Button, Input, Modal, Tag, Typography } from "antd";
import { useState } from "react";
import { post } from "../../api/client";
import { MarketingExpiryMode } from "../../generated/enums";
import type {
  MarketingSettings,
  MarketingSettingsInput,
} from "../../generated/models";
import { formatFen, yuanToFen } from "../../shared/money";

const changeTimestamp = new Intl.DateTimeFormat("zh-CN", {
  timeZone: "Asia/Shanghai",
  month: "2-digit",
  day: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
  hour12: false,
});

function formatHours(minutes: number) {
  return `${Math.floor(minutes / 60)}${minutes % 60 ? ".5" : ""}`;
}

export function MarketingControls({
  settings,
  canEdit,
}: {
  settings?: MarketingSettings;
  canEdit: boolean;
}) {
  const { modal, message } = App.useApp();
  const cache = useQueryClient();
  const [priceDraft, setPriceDraft] = useState<string>();
  const [proposedPrice, setProposedPrice] = useState<string>();
  const [expiryDraft, setExpiryDraft] = useState<string>();
  const [expiryModeDraft, setExpiryModeDraft] = useState<MarketingExpiryMode>();
  const save = useMutation({
    mutationFn: (patch: MarketingSettingsInput) =>
      post<MarketingSettings>("/marketing-settings", patch),
    onSuccess: (updated) => {
      cache.setQueryData(["marketing-settings"], updated);
      void cache.invalidateQueries({ queryKey: ["tasks"] });
      message.success("营销设置已保存");
    },
    onError: (error) => message.error(error.message),
  });
  const expiry = settings ? formatHours(settings.expiry_minutes) : "";
  const savedExpiryMode =
    settings?.expiry_mode ?? MarketingExpiryMode.MarketingExpiryFixedDuration;
  const expiryMode = expiryModeDraft ?? savedExpiryMode;
  const canSave = canEdit && !!settings && !save.isPending;

  function confirmPrice() {
    if (priceDraft === undefined || !settings || !canSave) return;
    setPriceDraft(undefined);
    try {
      const priceFen = yuanToFen(priceDraft);
      if (priceFen === "0" || BigInt(priceFen) > 50000n) {
        throw new Error("每平台单价须大于 0 元且不超过 500 元");
      }

      if (priceFen !== settings.price_fen) setProposedPrice(priceFen);
    } catch (error) {
      message.error(error instanceof Error ? error.message : "请输入有效金额");
    }
  }

  function saveExpiry() {
    if (!canSave || proposedPrice) return;

    function resetExpiryDraft() {
      setExpiryDraft(undefined);
      setExpiryModeDraft(undefined);
    }

    if (expiryMode === MarketingExpiryMode.MarketingExpiryEndOfDay) {
      save.mutate({ expiry_mode: expiryMode }, { onSuccess: resetExpiryDraft });
      return;
    }

    const value = (expiryDraft ?? expiry).trim();
    const [whole, fraction] = value.split(".");
    const minutes = Number(whole) * 60 + (fraction === "5" ? 30 : 0);
    if (
      !/^\d+(?:\.[05])?$/.test(value) ||
      !Number.isSafeInteger(minutes) ||
      minutes < 30 ||
      minutes > 24 * 60
    ) {
      message.error("固定时长须在 0.5 至 24 小时之间，以半小时为单位");
      return;
    }

    save.mutate(
      { expiry_mode: expiryMode, expiry_minutes: minutes },
      { onSuccess: resetExpiryDraft },
    );
  }

  function toggleVisibility() {
    if (!settings || !canSave || proposedPrice) return;

    const online = !settings.online;
    modal.confirm({
      title: online ? "上架营销分发？" : "下架营销分发？",
      content: online
        ? "分发员可以领取符合条件的内容，运营也可以指派。"
        : "下架后暂停新领取和手动指派，已经分配的任务仍可提交。",
      onOk: () => save.mutateAsync({ online }),
    });
  }

  return (
    <section
      className="marketing-settings"
      aria-labelledby="marketing-settings-heading"
    >
      <div className="marketing-section-heading">
        <Typography.Title id="marketing-settings-heading" level={4}>
          营销设置
        </Typography.Title>
        <Typography.Text type="secondary">运营按实际节奏设置</Typography.Text>
      </div>
      <form
        className="marketing-settings-panel"
        onSubmit={(event) => {
          event.preventDefault();
          saveExpiry();
        }}
      >
        <span className="marketing-setting-label">派单开关</span>
        <div className="marketing-setting-inline">
          {settings && (
            <>
              <Tag className={settings.online ? "task-online-tag" : undefined}>
                {settings.online ? "上架" : "下架"}
              </Tag>
              {canEdit && (
                <Button
                  size="small"
                  disabled={!canSave}
                  onClick={toggleVisibility}
                >
                  {settings.online ? "下架" : "上架"}
                </Button>
              )}
            </>
          )}
          <a
            href="https://yanxuan.baituber.cn/#/task/M1"
            target="_blank"
            rel="noreferrer"
          >
            看用户端
          </a>
          <span className="marketing-note">下架后停止新领取和指派</span>
        </div>
        <label className="marketing-setting-label" htmlFor="marketing-price">
          单价
        </label>
        <div className="marketing-setting-inline">
          <span>每个平台 ¥</span>
          <Input
            id="marketing-price"
            aria-label="每平台单价（元）"
            className="marketing-setting-input"
            inputMode="decimal"
            disabled={!canSave}
            value={
              priceDraft ?? (settings ? formatFen(settings.price_fen) : "")
            }
            onChange={(event) => setPriceDraft(event.target.value)}
            onBlur={confirmPrice}
            onPressEnter={(event) => {
              event.preventDefault();
              event.currentTarget.blur();
            }}
          />
          {settings && (
            <span className="marketing-note">
              两个平台都发 ¥{formatFen(String(BigInt(settings.price_fen) * 2n))}
              ，已领取的按领取时的单价结算
            </span>
          )}
        </div>
        <span className="marketing-setting-label">领取规则</span>
        <span>每人每天 1 条，每条只给一个人，领取时优先匹配标签。</span>
        <label
          className="marketing-setting-label"
          htmlFor="marketing-expiry-mode"
        >
          失效规则
        </label>
        <div className="marketing-setting-inline">
          <span>领取、指派或重派后</span>
          <select
            id="marketing-expiry-mode"
            className="marketing-expiry-mode"
            value={expiryMode}
            disabled={!canSave}
            onChange={(event) => {
              setExpiryModeDraft(
                event.currentTarget.value ===
                  MarketingExpiryMode.MarketingExpiryEndOfDay
                  ? MarketingExpiryMode.MarketingExpiryEndOfDay
                  : MarketingExpiryMode.MarketingExpiryFixedDuration,
              );
              setExpiryDraft(undefined);
            }}
          >
            <option value={MarketingExpiryMode.MarketingExpiryEndOfDay}>
              当天
            </option>
            <option value={MarketingExpiryMode.MarketingExpiryFixedDuration}>
              指定时间内
            </option>
          </select>
          {expiryMode === MarketingExpiryMode.MarketingExpiryFixedDuration ? (
            <>
              <Input
                aria-label="失效时长"
                className="marketing-setting-input"
                type="number"
                min="0.5"
                max="24"
                step="0.5"
                value={expiryDraft ?? expiry}
                disabled={!canSave}
                onChange={(event) => setExpiryDraft(event.target.value)}
              />
              <span>小时内未交发布链接，自动失效</span>
            </>
          ) : (
            <span>北京时间 24:00 前未交发布链接，自动失效</span>
          )}
        </div>
        <span className="marketing-setting-label">失效以后</span>
        <span>
          回到排队中，可以给其他分发员领取或指派。失效的任务仍算原分发员当天的 1
          条。
        </span>
        {canEdit && (
          <div className="marketing-setting-actions marketing-setting-inline">
            <Button
              type="primary"
              htmlType="submit"
              disabled={!canSave || !!proposedPrice}
            >
              保存失效规则
            </Button>
            {settings && (
              <span className="marketing-note">
                现在生效：
                {savedExpiryMode === MarketingExpiryMode.MarketingExpiryEndOfDay
                  ? "当天（北京时间 24:00 截止）"
                  : `指定时间内（${expiry} 小时）`}
              </span>
            )}
          </div>
        )}
        <span className="marketing-setting-label">单价修改记录</span>
        <div className="marketing-price-history">
          {settings?.price_changes?.length
            ? settings.price_changes.map((entry, index) => (
                <span key={`${entry.changed_at}-${index}`}>
                  {changeTimestamp.format(new Date(entry.changed_at))} ·{" "}
                  {entry.operator} · ¥{formatFen(entry.before_fen)} → ¥
                  {formatFen(entry.after_fen)}
                </span>
              ))
            : settings
              ? `还没改过，现在是每个平台 ¥${formatFen(settings.price_fen)}`
              : "—"}
        </div>
      </form>
      <Modal
        title="修改营销单价"
        open={proposedPrice !== undefined}
        okText="确认修改"
        confirmLoading={save.isPending}
        onOk={() =>
          proposedPrice &&
          save.mutate(
            { price_fen: proposedPrice },
            { onSuccess: () => setProposedPrice(undefined) },
          )
        }
        onCancel={() => !save.isPending && setProposedPrice(undefined)}
      >
        {settings && proposedPrice && (
          <Typography.Paragraph>
            每个平台从 ¥{formatFen(settings.price_fen)} 改为 ¥
            {formatFen(proposedPrice)}。
          </Typography.Paragraph>
        )}
        <Typography.Paragraph>
          新单价用于之后领取或指派的任务，已分配任务按原价执行。
        </Typography.Paragraph>
      </Modal>
    </section>
  );
}
