import ActionMenu from "./ActionMenu";
import { createPortal } from "react-dom";
import { RoutingPresetPicker, RoutingEditor } from "./SubscriptionRouting";
import {
  selectSubscriptionNodes,
  validRoutingNames,
  applyRoutingPreset,
} from "./subscription-routing";
import { RuleTemplatePanel, TemplatePicker } from "./RuleTemplates";
import RulesEditor from "./RulesEditor";
import SubscriptionAccess from "./SubscriptionAccess";
import { t, useLocale, localeTag } from "./i18n";
import { useEffect, useRef, useState } from "react";
import type { FormEvent } from "react";
import {
  loadRoutingCatalog,
  createSubscription,
  deleteSubscription,
  errorMessage,
  listSubscriptions,
  rotateSubscription,
  updateSubscription,
} from "./api";
import type {
  ManagedNode,
  Subscription,
  SubscriptionInput,
  RoutingCatalog,
} from "./api";
import Select from "./Select";
import { isShadowsocks, protocolNames } from "./api";

const formatLabels: Record<string, string> = {
  stash: "Stash",
  mihomo: "Mihomo / Clash.Meta",
  surge: "Surge",
  loon: "Loon · 系统 CA",
  hysteria2_uri: "Hysteria 2 · 分享链接",
};
function supportsFormat(format: string, protocol: string) {
  if (["anytls", "http"].includes(protocol))
    return ["stash", "mihomo"].includes(format);
  if (isShadowsocks(protocol))
    return ["stash", "mihomo", "surge"].includes(format);
  if (["socks", "mixed"].includes(protocol))
    return ["stash", "mihomo", "surge"].includes(format);
  if (["hysteria", "shadowtls"].includes(protocol))
    return ["stash", "mihomo"].includes(format);
  if (protocol === "snell") return ["stash", "surge"].includes(format);
  if (protocol === "snell6") return format === "surge";
  if (format === "surge")
    return ["trojan", "vmess", "hysteria2", "tuic"].includes(protocol);
  if (format === "loon")
    return ["trojan", "vless", "vmess", "hysteria2"].includes(protocol);
  if (format === "hysteria2_uri") return protocol === "hysteria2";
  return (
    ["stash", "mihomo"].includes(format) &&
    ["trojan", "vless", "vmess", "hysteria2", "tuic"].includes(protocol)
  );
}

const emptyInput = (): SubscriptionInput => ({
  name: "",
  format: "stash",
  node_ids: [],
  rules: [],
  final_action: "proxy",
  enabled: true,
});
export default function SubscriptionPanel({
  actionTarget,
  nodes,
  manage,
  refreshKey,
  onNodes,
}: {
  actionTarget: HTMLElement | null;
  nodes: ManagedNode[];
  manage: boolean;
  refreshKey: number;
  onNodes: () => void;
}) {
  const locale = useLocale();
  const [rows, setRows] = useState<Subscription[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [catalog, setCatalog] = useState<RoutingCatalog | null>(null);
  const [catalogError, setCatalogError] = useState("");
  useEffect(() => {
    const controller = new AbortController();
    loadRoutingCatalog(controller.signal)
      .then((value) => {
        if (!controller.signal.aborted) {
          setCatalog(value);
          setCatalogError("");
        }
      })
      .catch((e) => {
        if (!controller.signal.aborted) setCatalogError(errorMessage(e));
      });
    return () => controller.abort();
  }, [refreshKey]);
  const [version, setVersion] = useState(0);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [editor, setEditor] = useState<Subscription | null | undefined>();
  const [input, setInput] = useState<SubscriptionInput>(emptyInput);
  const [confirmation, setConfirmation] = useState<{
    row: Subscription;
    action: "delete" | "rotate";
  } | null>(null);
  const lifecycle = useRef<AbortController | null>(null);
  const dialog = useRef<HTMLDialogElement>(null);
  const available = nodes.filter(
    (n) =>
      (n.action === "deploy" && n.state === "succeeded") ||
      (n.action === "restart" &&
        ["queued", "running", "failed", "interrupted"].includes(n.state)),
  );
  useEffect(() => {
    const controller = new AbortController();
    lifecycle.current = controller;
    return () => controller.abort();
  }, []);
  useEffect(() => {
    const controller = new AbortController();
    listSubscriptions(controller.signal)
      .then((data) => {
        if (!controller.signal.aborted) {
          setRows(data);
          setLoaded(true);
        }
      })
      .catch((e: unknown) => {
        if (!controller.signal.aborted) {
          setLoaded(false);
          setError(errorMessage(e));
        }
      });
    return () => controller.abort();
  }, [refreshKey, version]);
  useEffect(() => {
    if (editor !== undefined) dialog.current?.showModal();
    else dialog.current?.close();
  }, [editor]);
  function edit(row: Subscription | null) {
    setInput(
      row
        ? {
            name: row.name,
            format: row.format,
            node_ids: [...row.node_ids],
            rules: row.rules.map((r) => ({ ...r })),
            final_action: row.final_action,
            enabled: row.enabled,
            routing: row.routing ? structuredClone(row.routing) : null,
          }
        : emptyInput(),
    );
    setError("");
    setEditor(row);
  }
  async function act(fn: (signal: AbortSignal) => Promise<void>) {
    const controller = lifecycle.current;
    if (!controller || controller.signal.aborted) return;
    setBusy(true);
    setError("");
    setNotice("");
    try {
      await fn(controller.signal);
      if (!controller.signal.aborted) setVersion((v) => v + 1);
    } catch (e) {
      if (!controller.signal.aborted) setError(errorMessage(e));
    } finally {
      if (!controller.signal.aborted) setBusy(false);
    }
  }
  function submit(e: FormEvent) {
    e.preventDefault();
    void act(async (signal) => {
      if (editor) await updateSubscription(editor.id, input, signal);
      else {
        await createSubscription(input, signal);
      }
      if (!signal.aborted) {
        setEditor(undefined);
        setNotice(
          editor
            ? t("订阅已更新，原链接会输出最新配置。")
            : t("订阅已创建，可随时查看和复制链接。"),
        );
      }
    });
  }
  const editAction = (row: Subscription) => (
    <button
      className="secondary compact"
      disabled={busy}
      onClick={() => edit(row)}
    >
      {t("编辑订阅")}
    </button>
  );
  const secondaryActions = (row: Subscription) => (
    <>
      <button
        className="secondary compact"
        disabled={busy}
        onClick={() =>
          void act(async (signal) => {
            await updateSubscription(
              row.id,
              {
                name: row.name,
                format: row.format,
                node_ids: row.node_ids,
                rules: row.rules,
                final_action: row.final_action,
                enabled: !row.enabled,
                routing: row.routing,
              },
              signal,
            );
            if (!signal.aborted) {
              setNotice(
                row.enabled
                  ? t("订阅已停用，链接不再输出配置。")
                  : t("订阅已启用。"),
              );
            }
          })
        }
      >
        {row.enabled ? t("停用") : t("启用")}
      </button>
      <button
        className="secondary compact"
        disabled={busy}
        onClick={() => setConfirmation({ row, action: "rotate" })}
      >
        {t("重置链接")}
      </button>
      <button
        className="text-danger"
        disabled={busy}
        onClick={() => setConfirmation({ row, action: "delete" })}
      >
        {t("删除")}
      </button>
    </>
  );
  return (
    <section className="panel node-panel subscription-panel">
      {manage &&
        actionTarget &&
        createPortal(
          <button
            className="primary compact"
            disabled={busy || !loaded}
            onClick={() => edit(null)}
          >
            {t("＋ 创建订阅")}
          </button>,
          actionTarget,
        )}
      {error && editor === undefined && (
        <p className="form-error" role="alert">
          {t(error)}
        </p>
      )}
      {notice && (
        <p className="notice" role="status">
          {t(notice)}
        </p>
      )}
      {!loaded ? (
        <div className="deployment-empty">
          {error ? t("订阅列表暂时不可用，请刷新重试。") : t("正在获取订阅…")}
        </div>
      ) : rows.length === 0 ? (
        <div className="empty-state">
          <span className="pending-star" aria-hidden="true">
            ▧
          </span>
          <h3>{t("你的第一个订阅")}</h3>
          <p>{t("将节点与分流规则放进一个订阅，在兼容的客户端中导入链接。")}</p>
          {!available.length && (
            <button className="secondary" onClick={onNodes}>
              {t("先查看节点 →")}
            </button>
          )}
        </div>
      ) : (
        <div className="node-grid subscription-list">
          {rows.map((row) => (
            <article className="node-card subscription-card" key={row.id}>
              <div className="node-card-top">
                <span className="node-protocol">
                  {t(formatLabels[row.format] ?? row.format)}
                </span>
                <span
                  className={`deployment-state deployment-${row.enabled ? "succeeded" : "cancelled"}`}
                >
                  {row.enabled ? t("已启用") : t("已停用")}
                </span>
              </div>
              <h3>{row.name}</h3>
              <p className="subscription-meta">
                {row.node_ids.length}
                {t("个已选节点 ·")}
                {row.routing ? (
                  t("{0} 个策略组", { 0: row.routing.groups.length })
                ) : (
                  <>
                    {row.rules.length}
                    {t("条规则 · 默认")}
                    {row.final_action === "proxy" ? t("使用节点") : t("直连")}
                  </>
                )}
              </p>
              <div className="host-tags">
                {row.node_ids.map((id) => (
                  <span className="badge" key={id}>
                    {nodes.find((n) => n.id === id)?.name ?? t("节点已不可用")}
                  </span>
                ))}
              </div>
              <p className="subscription-updated">
                {t("更新于")}
                {new Date(row.updated_at).toLocaleString(localeTag())}
              </p>
              {manage &&
                (row.subscription_path ? (
                  <SubscriptionAccess
                    key={row.subscription_path + row.format}
                    path={row.subscription_path}
                    name={row.name}
                    format={row.format}
                    enabled={row.enabled}
                    busy={busy}
                    primaryActions={editAction(row)}
                    moreActions={secondaryActions(row)}
                  />
                ) : (
                  <>
                    <p className="form-hint">
                      {row.link_state === "legacy"
                        ? t(
                            "旧订阅未保存可恢复链接。请重置一次链接，之后可随时查看；重置会使旧链接失效。",
                          )
                        : t("订阅链接暂时不可读取，请检查控制端加密配置。")}
                    </p>
                    <div className="subscription-actions">
                      {editAction(row)}
                      <ActionMenu disabled={busy}>
                        {secondaryActions(row)}
                      </ActionMenu>
                    </div>
                  </>
                ))}
              {confirmation?.row.id === row.id && (
                <div className="delete-confirm">
                  <p>
                    {confirmation.action === "delete"
                      ? t("删除此订阅后，链接将失效，节点本身不受影响。")
                      : t("重置后旧链接立即失效，客户端需要换用新链接。")}
                  </p>
                  <button
                    className="danger"
                    disabled={busy}
                    onClick={() =>
                      void act(async (signal) => {
                        if (confirmation.action === "delete") {
                          await deleteSubscription(row.id, signal);
                          if (!signal.aborted) {
                            setNotice(t("订阅已删除。"));
                          }
                        } else {
                          await rotateSubscription(row.id, signal);
                          if (!signal.aborted) {
                            setNotice(t("新链接已生成，旧链接已失效。"));
                          }
                        }
                        if (!signal.aborted) setConfirmation(null);
                      })
                    }
                  >
                    {t("确认")}
                    {confirmation.action === "delete" ? t("删除") : t("重置")}
                  </button>
                  <button
                    className="secondary"
                    disabled={busy}
                    onClick={() => setConfirmation(null)}
                  >
                    {t("取消")}
                  </button>
                </div>
              )}
            </article>
          ))}
        </div>
      )}
      {catalog && (
        <section className="configuration-template-library">
          <h3>{t("配置模板")}</h3>
          <p className="form-hint">
            {t(
              "选择完整分流方案，分配节点后即可生成订阅。规则集由客户端自动更新。",
            )}
          </p>
          <div className="routing-presets">
            {catalog.presets.map((preset) => (
              <button
                key={preset.id}
                type="button"
                className="routing-preset"
                disabled={!manage || busy}
                onClick={() => {
                  setInput(
                    applyRoutingPreset(emptyInput(), preset, locale === "en"),
                  );
                  setError("");
                  setEditor(null);
                }}
              >
                <strong>
                  {locale === "en" ? preset.name_en : preset.name}
                  {preset.recommended && (
                    <em className="routing-recommendation">{t("推荐")}</em>
                  )}
                </strong>
                <span>
                  {locale === "en" ? preset.description_en : preset.description}
                </span>
                <small>
                  {t("{0} 个策略组 · {1} 个规则集", {
                    0: preset.groups.length,
                    1: preset.bindings.length,
                  })}
                </small>
                <b>{t("使用此模板")} →</b>
              </button>
            ))}
          </div>
        </section>
      )}
      <RuleTemplatePanel manage={manage} />
      <p className="node-footnote">
        {t(
          "支持 Stash、Mihomo、Surge、Loon 和 Hysteria 2 分享链接。Shadowrocket 的安全导入尚未验证。停用或重置链接不会撤回已下载的节点凭据。",
        )}
      </p>
      <dialog
        ref={dialog}
        className="host-dialog machine-dialog subscription-dialog"
        aria-labelledby="subscription-title"
        onCancel={(e) => {
          e.preventDefault();
          if (!busy) setEditor(undefined);
        }}
      >
        <div className="dialog-heading">
          <div>
            <p className="eyebrow">SUBSCRIPTION</p>
            <h2 id="subscription-title">
              {editor ? t("编辑订阅") : t("创建订阅")}
            </h2>
          </div>
          <button
            className="icon-button"
            type="button"
            aria-label={t("关闭订阅编辑")}
            disabled={busy}
            onClick={() => setEditor(undefined)}
          >
            ×
          </button>
        </div>
        <form className="machine-content" onSubmit={submit}>
          {error && (
            <p className="form-error" role="alert">
              {t(error)}
            </p>
          )}
          <fieldset disabled={busy}>
            <label htmlFor="subscription-name">
              {t("订阅名称")}
              <input
                id="subscription-name"
                required
                maxLength={80}
                value={input.name}
                onChange={(e) => setInput({ ...input, name: e.target.value })}
                placeholder={t("例如：日常使用")}
              />
            </label>
            <div className="subscription-default">
              <label htmlFor="subscription-format">{t("客户端格式")}</label>
              <Select
                id="subscription-format"
                label={t("客户端格式")}
                value={input.format}
                disabled={busy}
                options={[
                  { value: "stash", label: "Stash" },
                  { value: "mihomo", label: "Mihomo / Clash.Meta" },
                  {
                    value: "surge",
                    label: "Surge",
                    description: t("不支持 VLESS"),
                  },
                  {
                    value: "loon",
                    label: t("Loon · 系统 CA"),
                    description: t("需要公共 CA 证书，不支持 TUIC"),
                  },
                  {
                    value: "hysteria2_uri",
                    label: t("Hysteria 2 · 分享链接"),
                    description: t("仅节点链接，不含规则"),
                  },
                ]}
                onChange={(value) =>
                  setInput({
                    ...input,
                    format: value as SubscriptionInput["format"],
                  })
                }
              />
            </div>
            <p className="form-hint">
              {t(
                "不同格式的协议支持不同，不兼容节点不会被静默删除。分享链接不包含分流规则，客户端 App 的实际导入和联网仍需验证。",
              )}
            </p>
            {input.format === "loon" && (
              <p className="form-hint">
                {t(
                  "选择 Loon 表示使用系统 CA 验证，而非固定叶证书。节点必须提供受信任的完整证书链；自签名或私有 CA 节点会被拒绝导出。",
                )}
              </p>
            )}
            {input.node_ids.some((id) => {
              const node = available.find((n) => n.id === id);
              return node && !supportsFormat(input.format, node.protocol);
            }) && (
              <p className="form-error">
                {t("已选节点包含不兼容协议，请取消选择或更换客户端格式。")}
              </p>
            )}
            {input.format === "hysteria2_uri" &&
              (input.rules.length > 0 ||
                input.final_action !== "proxy" ||
                !!input.routing) && (
                <p className="form-error">
                  {t(
                    "分享链接不包含分流规则；请先切回完整配置格式移除模板，清空规则并将默认连接设为使用节点",
                  )}
                </p>
              )}
            {catalogError && (
              <p className="form-error">
                {t("配置模板加载失败，请刷新页面重试。")}
              </p>
            )}
            {!catalog && !catalogError && (
              <p className="form-hint">{t("正在加载配置模板…")}</p>
            )}
            <RoutingPresetPicker
              input={input}
              setInput={setInput}
              catalog={catalog}
              busy={busy}
            />
            <div className="subscription-step">
              <h3>{t("01 · 选择节点")}</h3>
              <span className="badge">
                {t("已选")}
                {input.node_ids.length}
              </span>
            </div>
            <div className="subscription-node-options">
              {available.map((node) => (
                <label className="subscription-node-option" key={node.id}>
                  <input
                    type="checkbox"
                    checked={input.node_ids.includes(node.id)}
                    disabled={
                      !supportsFormat(input.format, node.protocol) &&
                      !input.node_ids.includes(node.id)
                    }
                    onChange={(e) =>
                      setInput((current) =>
                        selectSubscriptionNodes(
                          current,
                          e.target.checked
                            ? [...current.node_ids, node.id]
                            : current.node_ids.filter((id) => id !== node.id),
                        ),
                      )
                    }
                  />
                  <span>
                    <strong>{node.name}</strong>
                    <small>
                      {protocolNames[node.protocol]} · {node.host_name}
                      {!supportsFormat(input.format, node.protocol) &&
                        ` · ${t("此格式不支持")}`}
                    </small>
                  </span>
                </label>
              ))}
            </div>
            {!available.length && (
              <p className="form-hint">{t("还没有可选节点，请先部署节点。")}</p>
            )}
            {input.node_ids
              .filter((id) => !available.some((n) => n.id === id))
              .map((id) => (
                <div className="subscription-missing" key={id}>
                  {t("已选节点不可用")}
                  <button
                    type="button"
                    className="text-danger"
                    onClick={() =>
                      setInput(
                        selectSubscriptionNodes(
                          input,
                          input.node_ids.filter((v) => v !== id),
                        ),
                      )
                    }
                  >
                    {t("移除选择")}
                  </button>
                </div>
              ))}
            <RoutingEditor
              input={input}
              setInput={setInput}
              catalog={catalog}
              busy={busy}
              nodes={nodes}
            />
            {editor !== undefined && !input.routing && (
              <TemplatePicker input={input} onChange={setInput} busy={busy} />
            )}
            <RulesEditor
              input={input}
              setInput={setInput}
              busy={busy}
              hideFinal={!!input.routing}
              groupOptions={input.routing?.groups.map((g) => ({
                value: `group:${g.id}`,
                label: g.name,
              }))}
            />
            {!validRoutingNames(input) && (
              <p className="form-error">
                {t(
                  "策略组名称必须唯一，不能使用 DIRECT、REJECT 或逗号等配置分隔符。",
                )}
              </p>
            )}
            <label className="check-row">
              <input
                type="checkbox"
                checked={input.enabled}
                onChange={(e) =>
                  setInput({ ...input, enabled: e.target.checked })
                }
              />
              {t("启用订阅链接")}
            </label>
          </fieldset>
          <div className="deployment-actions">
            <button
              className="primary"
              disabled={
                busy ||
                !validRoutingNames(input) ||
                (!editor && !input.node_ids.length) ||
                input.node_ids.some((id) => {
                  const node = available.find((n) => n.id === id);
                  return node && !supportsFormat(input.format, node.protocol);
                }) ||
                (input.format === "hysteria2_uri" &&
                  (input.rules.length > 0 ||
                    input.final_action !== "proxy" ||
                    !!input.routing))
              }
            >
              {busy
                ? t("正在保存…")
                : editor
                  ? t("保存修改")
                  : t("创建并生成链接")}
            </button>
            <button
              className="secondary"
              type="button"
              disabled={busy}
              onClick={() => setEditor(undefined)}
            >
              {t("取消")}
            </button>
          </div>
        </form>
      </dialog>
    </section>
  );
}
