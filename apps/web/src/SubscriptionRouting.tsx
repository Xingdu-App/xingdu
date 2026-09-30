import { t, useLocale } from "./i18n";
import Select from "./Select";
import type {
  ManagedNode,
  RoutingCatalog,
  SubscriptionInput,
  RoutingGroup,
} from "./api";
import { useState, type Dispatch, type SetStateAction } from "react";
import { applyRoutingPreset, removeRoutingGroup } from "./subscription-routing";
import "./subscription-routing.css";

type Props = {
  input: SubscriptionInput;
  setInput: Dispatch<SetStateAction<SubscriptionInput>>;
  catalog: RoutingCatalog | null;
  busy: boolean;
};
export function RoutingPresetPicker({ input, setInput, catalog, busy }: Props) {
  const english = useLocale() === "en";
  if (input.format === "hysteria2_uri" && !input.routing) return null;
  const selected = catalog?.presets.find((p) => p.id === input.routing?.preset);
  return (
    <section className="routing-section">
      <div className="subscription-step">
        <h3>{t("配置模板")}</h3>
        <span className="badge">{t("选择模板，再分配节点")}</span>
      </div>
      <p className="form-hint">
        {t(
          "模板已配置策略组和完整社区规则集。更换模板会重置组设置和规则集目标，保留已选节点和自定义规则；失效的组目标改为默认代理。",
        )}
      </p>
      <details className="routing-template-choices" open={!input.routing}>
        <summary>
          {t("选择或更换模板")}
          {selected && ` · ${english ? selected.name_en : selected.name}`}
        </summary>
        <div className="routing-presets">
          {catalog?.presets.map((preset) => (
            <button
              type="button"
              className={`routing-preset ${input.routing?.preset === preset.id ? "is-selected" : ""}`}
              key={preset.id}
              aria-pressed={input.routing?.preset === preset.id}
              disabled={busy || input.format === "hysteria2_uri"}
              onClick={() =>
                setInput((current) =>
                  applyRoutingPreset(current, preset, english),
                )
              }
            >
              <strong>
                {english ? preset.name_en : preset.name}
                {preset.recommended && (
                  <em className="routing-recommendation">{t("推荐")}</em>
                )}
              </strong>
              <span>
                {english ? preset.description_en : preset.description}
              </span>
              <small>
                {t("{0} 个策略组 · {1} 个规则集", {
                  0: preset.groups.length,
                  1: preset.bindings.length,
                })}
              </small>
            </button>
          ))}
        </div>
      </details>
      {selected && (
        <p className="form-hint">
          {english ? selected.description_en : selected.description}
        </p>
      )}
      <button
        type="button"
        className="secondary compact"
        disabled={busy || !input.routing}
        onClick={() =>
          setInput((current) => applyRoutingPreset(current, null, english))
        }
      >
        {t("仅使用自定义规则")}
      </button>
    </section>
  );
}

export function RoutingEditor({
  input,
  setInput,
  catalog,
  busy,
  nodes,
}: Props & { nodes: ManagedNode[] }) {
  const english = useLocale() === "en";
  const routing = input.routing;
  if (!routing) return null;
  const preset = catalog?.presets.find((p) => p.id === routing.preset);
  const options = [
    ...routing.groups.map((g) => ({ value: `group:${g.id}`, label: g.name })),
    { value: "direct", label: t("直接连接") },
    { value: "reject", label: t("拦截") },
  ];
  function patchGroup(id: string, patch: Partial<RoutingGroup>) {
    setInput((current) =>
      current.routing
        ? {
            ...current,
            routing: {
              ...current.routing,
              groups: current.routing.groups.map((g) =>
                g.id === id ? { ...g, ...patch } : g,
              ),
            },
          }
        : current,
    );
  }
  return (
    <section className="routing-section">
      <div className="subscription-step">
        <h3>{t("策略组与节点分配")}</h3>
        <button
          type="button"
          className="secondary compact"
          disabled={busy || routing.groups.length >= 20}
          onClick={() => {
            let index = 1;
            while (routing.groups.some((g) => g.id === `custom-${index}`))
              index++;
            setInput({
              ...input,
              routing: {
                ...routing,
                groups: [
                  ...routing.groups,
                  {
                    id: `custom-${index}`,
                    name: `${t("自定义组")} ${index}`,
                    type: "select",
                    node_ids: [],
                  },
                ],
              },
            });
          }}
        >
          {t("＋ 添加策略组")}
        </button>
      </div>
      <p className="form-hint">
        {t(
          "默认代理包含所有已选节点。同一节点可分配到多个专用组；专用组未选节点或节点全部不可用时，使用默认代理。故障转移按勾选顺序尝试，可取消后重新勾选调整顺序。",
        )}
      </p>
      <div className="routing-groups">
        {routing.groups.map((group) => (
          <article key={group.id} className="routing-group">
            <div className="routing-group-heading">
              <input
                aria-label={t("策略组名称")}
                value={group.name}
                maxLength={48}
                required
                onChange={(e) => patchGroup(group.id, { name: e.target.value })}
              />
              <Select
                label={t("策略组模式")}
                value={group.type}
                options={[
                  { value: "select", label: t("手动选择") },
                  { value: "url-test", label: t("自动测速") },
                  { value: "fallback", label: t("故障转移") },
                ]}
                disabled={busy}
                onChange={(value) =>
                  patchGroup(group.id, { type: value as RoutingGroup["type"] })
                }
              />
              {group.id !== "proxy" && (
                <button
                  type="button"
                  className="icon-button"
                  disabled={busy || !preset}
                  aria-label={t("删除策略组 {0}", { 0: group.name })}
                  onClick={() =>
                    preset &&
                    setInput((current) =>
                      removeRoutingGroup(current, group.id, preset),
                    )
                  }
                >
                  ×
                </button>
              )}
            </div>
            <label className="route-field">
              {t("策略组图标（可选）")}
              <div className="routing-icon-field">
                <GroupIconPreview key={group.icon ?? ""} url={group.icon} />
                <input
                  type="url"
                  placeholder="https://assets.example.com/icon.png"
                  aria-label={t("策略组图标（可选）")}
                  value={group.icon ?? ""}
                  disabled={busy}
                  maxLength={2048}
                  onChange={(e) =>
                    patchGroup(group.id, { icon: e.target.value || undefined })
                  }
                />
              </div>
            </label>
            {group.id === "proxy" ? (
              <p className="form-hint">
                {t("包含全部 {0} 个已选节点", { 0: input.node_ids.length })}
              </p>
            ) : (
              <div className="routing-node-selection">
                <div className="routing-node-actions">
                  <button
                    type="button"
                    className="secondary compact"
                    disabled={
                      busy ||
                      !input.node_ids.length ||
                      input.node_ids.every((id) => group.node_ids.includes(id))
                    }
                    onClick={() =>
                      patchGroup(group.id, {
                        node_ids: [
                          ...new Set([...group.node_ids, ...input.node_ids]),
                        ],
                      })
                    }
                  >
                    {t("全选")}
                  </button>
                </div>
                <div className="routing-node-chips">
                  {input.node_ids.map((id) => (
                    <label
                      key={id}
                      className={
                        group.node_ids.includes(id) ? "is-selected" : ""
                      }
                    >
                      <input
                        type="checkbox"
                        checked={group.node_ids.includes(id)}
                        onChange={(e) =>
                          patchGroup(group.id, {
                            node_ids: e.target.checked
                              ? [...group.node_ids, id]
                              : group.node_ids.filter((n) => n !== id),
                          })
                        }
                      />
                      <span>
                        {nodes.find((n) => n.id === id)?.name ??
                          t("节点已不可用")}
                        {group.type === "fallback" &&
                          group.node_ids.includes(id) && (
                            <small> #{group.node_ids.indexOf(id) + 1}</small>
                          )}
                      </span>
                    </label>
                  ))}
                  {!group.node_ids.length && (
                    <span className="routing-inherit">{t("跟随默认代理")}</span>
                  )}
                </div>
              </div>
            )}
          </article>
        ))}
      </div>
      <details className="routing-targets">
        <summary>
          {t("规则集与目标")} <span>{t("已预设，可按需调整")}</span>
        </summary>
        <p className="form-hint">
          {t(
            "自定义规则优先，其次按以下顺序匹配社区规则集，最后使用未匹配目标。",
          )}
        </p>
        {preset?.bindings.map((binding, index) => {
          const source = catalog?.sources.find((s) => s.id === binding.source);
          return (
            <div className="routing-target" key={binding.source}>
              <div>
                <strong>
                  <span className="routing-order">
                    {String(index + 1).padStart(2, "0")}
                  </span>
                  {english ? source?.name_en : source?.name}
                </strong>
                <a href={source?.url} target="_blank" rel="noreferrer">
                  ACL4SSR ↗
                </a>
              </div>
              <Select
                label={t("{0} 的规则目标", {
                  0: english ? source?.name_en : source?.name,
                })}
                value={routing.targets[binding.source] ?? binding.target}
                options={[
                  ...options,
                  { value: "disabled", label: t("不启用此规则集") },
                ]}
                disabled={busy}
                onChange={(value) =>
                  setInput({
                    ...input,
                    routing: {
                      ...routing,
                      targets: { ...routing.targets, [binding.source]: value },
                    },
                  })
                }
              />
            </div>
          );
        })}
        <p className="form-hint">
          {t(
            "规则来自 ACL4SSR，由客户端联网下载和缓存更新。首次导入需要能访问规则源；星渡不会向规则源发送你的订阅链接。模板不保证广告全覆盖或流媒体解锁。",
          )}
        </p>
        <a href={preset?.reference} target="_blank" rel="noreferrer">
          {t("查看模板参考来源")} ↗
        </a>
      </details>
      <div className="subscription-default">
        <label>{t("未匹配时")}</label>
        <Select
          label={t("默认规则目标")}
          value={routing.final}
          options={options}
          disabled={busy}
          onChange={(value) =>
            setInput({ ...input, routing: { ...routing, final: value } })
          }
        />
      </div>
    </section>
  );
}

function GroupIconPreview({ url }: { url?: string }) {
  const [failed, setFailed] = useState(false);
  let source: string | undefined;
  try {
    const parsed = new URL(url ?? "");
    if (parsed.protocol === "https:" && !parsed.username && !parsed.password) {
      source = parsed.href;
    }
  } catch {
    // An empty or incomplete URL keeps the preview placeholder.
  }
  return (
    <span className="routing-icon-preview" aria-label="Preview">
      {source && !failed ? (
        <img
          src={source}
          alt=""
          width={32}
          height={32}
          loading="lazy"
          referrerPolicy="no-referrer"
          onError={() => setFailed(true)}
        />
      ) : (
        <span>Preview</span>
      )}
    </span>
  );
}
