import { t } from "./i18n";
import Select from "./Select";
import type { SubscriptionRule, RuleTemplateInput } from "./api";
import type { Dispatch, SetStateAction } from "react";
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

export default function RulesEditor<T extends RuleTemplateInput>({
  input,
  setInput,
  busy,
  groupOptions = [],
  hideFinal = false,
}: {
  input: T;
  setInput: Dispatch<SetStateAction<T>>;
  busy: boolean;
  groupOptions?: { value: string; label: string }[];
  hideFinal?: boolean;
}) {
  function patchRule(index: number, patch: Partial<SubscriptionRule>) {
    setInput((current) => ({
      ...current,
      rules: current.rules.map((r, i) =>
        i === index ? { ...r, ...patch } : r,
      ),
    }));
  }
  return (
    <>
      <div className="subscription-step">
        <h3>{t("分流规则")}</h3>
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
          hideFinal
            ? "自定义规则优先于模板规则集，从上到下匹配，第一条命中生效。"
            : "从上到下匹配，第一条命中生效。不添加规则时，所有流量按默认方式连接。",
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
            value={
              groupOptions.length && rule.target === "proxy"
                ? "group:proxy"
                : rule.target
            }
            options={
              groupOptions.length
                ? [...groupOptions, ...targets.slice(1)]
                : targets
            }
            disabled={busy}
            onChange={(value) =>
              patchRule(index, {
                target: value as SubscriptionRule["target"],
              })
            }
          />
          <div className="subscription-rule-actions">
            {[-1, 1].map((direction) => (
              <button
                key={direction}
                type="button"
                className="icon-button"
                disabled={
                  busy ||
                  index + direction < 0 ||
                  index + direction >= input.rules.length
                }
                aria-label={t(direction < 0 ? "上移规则 {0}" : "下移规则 {0}", {
                  0: index + 1,
                })}
                onClick={() =>
                  setInput((current) => {
                    const rules = [...current.rules];
                    [rules[index], rules[index + direction]] = [
                      rules[index + direction],
                      rules[index],
                    ];
                    return { ...current, rules };
                  })
                }
              >
                {direction < 0 ? "↑" : "↓"}
              </button>
            ))}
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
        </div>
      ))}
      {!hideFinal && (
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
      )}
    </>
  );
}
