import { rulePresets, ruleTemplateSources } from "./rule-presets";
import { useEffect, useRef, useState } from "react";
import type { Dispatch, SetStateAction } from "react";
import {
  deleteRuleTemplate,
  errorMessage,
  listRuleTemplates,
  saveRuleTemplate,
} from "./api";
import type { RuleTemplate, RuleTemplateInput, SubscriptionInput } from "./api";
import { t, useLocale } from "./i18n";
import Select from "./Select";
import RulesEditor from "./RulesEditor";

export function TemplatePicker({
  input,
  onChange,
  busy,
}: {
  input: SubscriptionInput;
  onChange: Dispatch<SetStateAction<SubscriptionInput>>;
  busy: boolean;
}) {
  const [rows, setRows] = useState<RuleTemplate[]>([]);
  const [selected, setSelected] = useState("");
  const [error, setError] = useState("");
  const [confirm, setConfirm] = useState(false);
  useEffect(() => {
    const c = new AbortController();
    listRuleTemplates(c.signal)
      .then((r) => {
        if (!c.signal.aborted) setRows(r);
      })
      .catch((e) => {
        if (!c.signal.aborted) setError(errorMessage(e));
      });
    return () => c.abort();
  }, []);
  const row = [...rows, ...rulePresets].find((r) => r.id === selected);
  const incompatible =
    input.format === "hysteria2_uri" &&
    !!row &&
    (row.rules.length > 0 || row.final_action !== "proxy");
  function apply() {
    if (!row || incompatible) return;
    onChange((current) => ({
      ...current,
      rules: row.rules.map((r) => ({ ...r })),
      final_action: row.final_action,
    }));
    setConfirm(false);
    setSelected("");
  }
  return (
    <div className="template-picker">
      <label>{t("规则模板")}</label>
      <Select
        label={t("规则模板")}
        value={selected}
        options={[
          { value: "", label: t("选择规则模板") },
          ...rows.map((r) => ({ value: r.id, label: r.name })),
          ...rulePresets.map((r) => ({
            value: r.id,
            label: `${t("内置")} · ${t(r.name)}`,
            description: t(r.description),
          })),
        ]}
        disabled={busy}
        onChange={(v) => {
          setSelected(v);
          setConfirm(false);
        }}
      />
      <p className="form-hint">
        {t(
          "应用会替换当前规则和默认连接方式，保存订阅后生效。后续修改模板不会自动更新订阅。",
        )}
      </p>
      {error && (
        <p className="form-error" role="alert">
          {t(error)}
        </p>
      )}
      {row && "description" in row && (
        <p className="form-hint">{t(String(row.description))}</p>
      )}
      {row && (
        <p>
          {row.rules.length} {t("条规则 · 默认")}{" "}
          {row.final_action === "proxy" ? t("使用节点") : t("直接连接")}
        </p>
      )}
      {incompatible && (
        <p className="form-error">
          {t("此模板包含分享链接不支持的分流设置。")}
        </p>
      )}
      <button
        type="button"
        className="secondary compact"
        disabled={busy || !row || incompatible}
        onClick={() => setConfirm(true)}
      >
        {t("应用模板")}
      </button>
      {confirm && (
        <div className="delete-confirm">
          <p>{t("替换当前订阅草稿的全部规则和默认连接方式？")}</p>
          <button
            type="button"
            className="primary compact"
            disabled={busy}
            onClick={apply}
          >
            {t("确认替换")}
          </button>
          <button
            type="button"
            className="secondary compact"
            onClick={() => setConfirm(false)}
          >
            {t("取消")}
          </button>
        </div>
      )}
    </div>
  );
}

export function RuleTemplatePanel({ manage }: { manage: boolean }) {
  useLocale();
  const [rows, setRows] = useState<RuleTemplate[]>([]);
  const [version, setVersion] = useState(0);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [editor, setEditor] = useState<RuleTemplate | null | undefined>();
  const [deleting, setDeleting] = useState<string | null>(null);
  const [input, setInput] = useState<RuleTemplateInput>({
    name: "",
    rules: [],
    final_action: "proxy",
  });
  const dialog = useRef<HTMLDialogElement>(null);
  const lifecycle = useRef<AbortController | null>(null);
  useEffect(() => {
    const c = new AbortController();
    lifecycle.current = c;
    return () => c.abort();
  }, []);
  useEffect(() => {
    const c = new AbortController();
    listRuleTemplates(c.signal)
      .then((r) => {
        if (!c.signal.aborted) {
          setRows(r);
          setLoaded(true);
        }
      })
      .catch((e) => {
        if (!c.signal.aborted) setError(errorMessage(e));
      });
    return () => c.abort();
  }, [version]);
  useEffect(() => {
    if (editor !== undefined) dialog.current?.showModal();
    else dialog.current?.close();
  }, [editor]);
  function edit(row: RuleTemplate | null) {
    setError("");
    setInput(
      row
        ? {
            name: row.name,
            rules: row.rules.map((r) => ({ ...r })),
            final_action: row.final_action,
          }
        : { name: "", rules: [], final_action: "proxy" },
    );
    setEditor(row);
  }
  async function act(fn: (signal: AbortSignal) => Promise<void>) {
    const c = lifecycle.current;
    if (!c || c.signal.aborted) return;
    setBusy(true);
    setError("");
    try {
      await fn(c.signal);
      if (!c.signal.aborted) {
        setEditor(undefined);
        setDeleting(null);
        setVersion((v) => v + 1);
      }
    } catch (e) {
      if (!c.signal.aborted) setError(errorMessage(e));
    } finally {
      if (!c.signal.aborted) setBusy(false);
    }
  }
  return (
    <section className="rule-template-panel">
      <div className="subscription-step">
        <h3>{t("规则模板")}</h3>
        {manage && (
          <button
            className="secondary compact"
            disabled={busy}
            onClick={() => edit(null)}
          >
            {t("创建规则模板")}
          </button>
        )}
      </div>
      <p className="form-hint">
        {t(
          "在组织内复用分流规则。应用时复制到订阅，模板修改或删除不影响已有订阅。",
        )}
      </p>
      <div className="preset-section">
        <h4>{t("内置基础模板")}</h4>
        <p className="form-hint">
          {t(
            "星渡维护的精简方案，可直接应用或复制后编辑；不等同于社区完整规则集，不自动更新。",
          )}
        </p>
        <div className="node-grid preset-grid">
          {rulePresets.map((preset) => (
            <article className="node-card" key={preset.id}>
              <h3>{t(preset.name)}</h3>
              <p className="form-hint">{t(preset.description)}</p>
              <p>
                {preset.rules.length} {t("条规则 · 默认")}{" "}
                {preset.final_action === "proxy"
                  ? t("使用节点")
                  : t("直接连接")}
              </p>
              {manage && (
                <button
                  type="button"
                  className="secondary compact"
                  disabled={busy}
                  onClick={() => {
                    setError("");
                    setInput({
                      name: t(preset.name),
                      rules: preset.rules.map((r) => ({ ...r })),
                      final_action: preset.final_action,
                    });
                    setEditor(null);
                  }}
                >
                  {t("复制并编辑")}
                </button>
              )}
            </article>
          ))}
        </div>
        <details>
          <summary>{t("社区规则参考")}</summary>
          <p className="form-hint">
            {t(
              "以下是外部项目入口，暂不直接导入其 RULE-SET、GEOIP 或多策略组配置。订阅地址不会发送给这些项目。",
            )}
          </p>
          {ruleTemplateSources.map((source) => (
            <p key={source.name}>
              <a href={source.url} target="_blank" rel="noreferrer">
                {source.name} ↗
              </a>{" "}
              · {t(source.description)}
            </p>
          ))}
        </details>
      </div>
      {error && editor === undefined && (
        <p className="form-error" role="alert">
          {t(error)}
        </p>
      )}
      {!loaded && !error && <p>{t("正在加载…")}</p>}
      {loaded && !rows.length && (
        <p className="form-hint">{t("暂无规则模板")}</p>
      )}
      <div className="node-grid">
        {rows.map((row) => (
          <article className="node-card" key={row.id}>
            <h3>{row.name}</h3>
            <p>
              {row.rules.length} {t("条规则 · 默认")}{" "}
              {row.final_action === "proxy" ? t("使用节点") : t("直接连接")}
            </p>
            {manage && (
              <div className="subscription-actions">
                <button
                  className="secondary compact"
                  disabled={busy}
                  onClick={() => edit(row)}
                >
                  {t("编辑")}
                </button>
                <button
                  className="text-danger"
                  disabled={busy}
                  onClick={() => setDeleting(row.id)}
                >
                  {t("删除")}
                </button>
              </div>
            )}
            {deleting === row.id && (
              <div className="delete-confirm">
                <p>{t("删除此模板？已应用的订阅不受影响。")}</p>
                <button
                  className="danger"
                  disabled={busy}
                  onClick={() =>
                    void act((signal) => deleteRuleTemplate(row.id, signal))
                  }
                >
                  {t("确认删除")}
                </button>
                <button
                  className="secondary"
                  disabled={busy}
                  onClick={() => setDeleting(null)}
                >
                  {t("取消")}
                </button>
              </div>
            )}
          </article>
        ))}
      </div>
      <dialog
        ref={dialog}
        className="host-dialog machine-dialog subscription-dialog"
        aria-labelledby="rule-template-title"
        onCancel={(e) => {
          e.preventDefault();
          if (!busy) setEditor(undefined);
        }}
      >
        <div className="dialog-heading">
          <h2 id="rule-template-title">
            {editor ? t("编辑规则模板") : t("创建规则模板")}
          </h2>
          <button
            type="button"
            className="icon-button"
            aria-label={t("关闭")}
            disabled={busy}
            onClick={() => setEditor(undefined)}
          >
            ×
          </button>
        </div>
        <form
          className="machine-content"
          onSubmit={(e) => {
            e.preventDefault();
            void act(async (signal) => {
              await saveRuleTemplate(editor?.id, input, signal);
            });
          }}
        >
          {error && (
            <p className="form-error" role="alert">
              {t(error)}
            </p>
          )}
          <fieldset disabled={busy}>
            <label>
              {t("模板名称")}
              <input
                required
                maxLength={80}
                value={input.name}
                onChange={(e) => setInput({ ...input, name: e.target.value })}
              />
            </label>
            <RulesEditor input={input} setInput={setInput} busy={busy} />
          </fieldset>
          <div className="deployment-actions">
            <button className="primary" disabled={busy}>
              {busy ? t("正在保存…") : t("保存修改")}
            </button>
            <button
              type="button"
              className="secondary"
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
