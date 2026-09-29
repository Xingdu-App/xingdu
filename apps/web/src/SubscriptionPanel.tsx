import SubscriptionAccess from "./SubscriptionAccess";
import { t, useLocale, localeTag } from "./i18n";
import { useEffect, useRef, useState } from "react";
import type { FormEvent } from "react";
import {
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
  SubscriptionRule,
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
  if (format === "surge") return protocol !== "vless";
  if (format === "loon") return protocol !== "tuic";
  if (format === "hysteria2_uri") return protocol === "hysteria2";
  return true;
}

const emptyInput = (): SubscriptionInput => ({
  name: "",
  format: "stash",
  node_ids: [],
  rules: [],
  final_action: "proxy",
  enabled: true,
});
const ruleTypes = [
  {
    value: "domain_suffix",
    get label() {
      return t("域名及子域名");
    },
  },
  {
    value: "domain",
    get label() {
      return t("精确域名");
    },
  },
  {
    value: "ip_cidr",
    get label() {
      return t("IP 网段");
    },
  },
];
const targets = [
  {
    value: "proxy",
    get label() {
      return t("使用节点");
    },
  },
  {
    value: "direct",
    get label() {
      return t("直接连接");
    },
  },
  {
    value: "reject",
    get label() {
      return t("拦截");
    },
  },
];

export default function SubscriptionPanel({
  nodes,
  manage,
  refreshKey,
  onNodes,
}: {
  nodes: ManagedNode[];
  manage: boolean;
  refreshKey: number;
  onNodes: () => void;
}) {
  useLocale();
  const [rows, setRows] = useState<Subscription[]>([]);
  const [loaded, setLoaded] = useState(false);
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
  function patchRule(index: number, patch: Partial<SubscriptionRule>) {
    setInput((current) => ({
      ...current,
      rules: current.rules.map((r, i) =>
        i === index ? { ...r, ...patch } : r,
      ),
    }));
  }
  return (
    <section className="panel node-panel subscription-panel">
      {manage && (
        <div className="subscription-toolbar">
          <button
            className="primary compact"
            disabled={busy || !loaded}
            onClick={() => edit(null)}
          >
            {t("＋ 创建订阅")}
          </button>
        </div>
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
                {row.rules.length}
                {t("条规则 · 默认")}
                {row.final_action === "proxy" ? t("使用节点") : t("直连")}
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
                  />
                ) : (
                  <p className="form-hint">
                    {row.link_state === "legacy"
                      ? t(
                          "旧订阅未保存可恢复链接。请重置一次链接，之后可随时查看；重置会使旧链接失效。",
                        )
                      : t("订阅链接暂时不可读取，请检查控制端加密配置。")}
                  </p>
                ))}
              {manage && (
                <div className="subscription-actions">
                  <button
                    className="secondary compact"
                    disabled={busy}
                    onClick={() => edit(row)}
                  >
                    {t("编辑订阅")}
                  </button>
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
                </div>
              )}
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
              (input.rules.length > 0 || input.final_action !== "proxy") && (
                <p className="form-error">
                  {t(
                    "分享链接不包含分流规则；请清空规则并将默认连接设为使用节点",
                  )}
                </p>
              )}
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
                      setInput((current) => ({
                        ...current,
                        node_ids: e.target.checked
                          ? [...current.node_ids, node.id]
                          : current.node_ids.filter((id) => id !== node.id),
                      }))
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
                      setInput({
                        ...input,
                        node_ids: input.node_ids.filter((v) => v !== id),
                      })
                    }
                  >
                    {t("移除选择")}
                  </button>
                </div>
              ))}
            <div className="subscription-step">
              <h3>{t("02 · 分流规则")}</h3>
              <button
                type="button"
                className="secondary compact"
                disabled={busy || input.rules.length >= 100}
                onClick={() =>
                  setInput({
                    ...input,
                    rules: [
                      ...input.rules,
                      { type: "domain_suffix", value: "", target: "direct" },
                    ],
                  })
                }
              >
                {t("＋ 添加规则")}
              </button>
            </div>
            <p className="form-hint">
              {t(
                "从上到下匹配，第一条命中生效。不添加规则时，所有流量按默认方式连接。",
              )}
            </p>
            {input.rules.map((rule, index) => (
              <div className="subscription-rule" key={index}>
                <Select
                  label={t("规则 {0} 类型", { 0: index + 1 })}
                  value={rule.type}
                  options={ruleTypes}
                  disabled={busy}
                  onChange={(value) =>
                    patchRule(index, {
                      type: value as SubscriptionRule["type"],
                    })
                  }
                />
                <input
                  aria-label={t("规则 {0} 内容", { 0: index + 1 })}
                  required
                  value={rule.value}
                  maxLength={253}
                  onChange={(e) => patchRule(index, { value: e.target.value })}
                  placeholder={
                    rule.type === "ip_cidr" ? "192.0.2.0/24" : "example.com"
                  }
                />
                <Select
                  label={t("规则 {0} 动作", { 0: index + 1 })}
                  value={rule.target}
                  options={targets}
                  disabled={busy}
                  onChange={(value) =>
                    patchRule(index, {
                      target: value as SubscriptionRule["target"],
                    })
                  }
                />
                <button
                  type="button"
                  className="icon-button"
                  aria-label={t("删除规则 {0}", { 0: index + 1 })}
                  onClick={() =>
                    setInput({
                      ...input,
                      rules: input.rules.filter((_, i) => i !== index),
                    })
                  }
                >
                  ×
                </button>
              </div>
            ))}
            <div className="subscription-default">
              <label>{t("未匹配时")}</label>
              <Select
                label={t("默认连接方式")}
                value={input.final_action}
                options={targets.slice(0, 2)}
                disabled={busy}
                onChange={(value) =>
                  setInput({
                    ...input,
                    final_action: value as "proxy" | "direct",
                  })
                }
              />
            </div>
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
                (!editor && !input.node_ids.length) ||
                input.node_ids.some((id) => {
                  const node = available.find((n) => n.id === id);
                  return node && !supportsFormat(input.format, node.protocol);
                }) ||
                (input.format === "hysteria2_uri" &&
                  (input.rules.length > 0 || input.final_action !== "proxy"))
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
