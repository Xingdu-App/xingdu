import { t, useLocale, localeTag } from "./i18n";
import Select from "./Select";
import { agentVersionAtLeast } from "./agent-version";
import { useEffect, useRef, useState } from "react";
import {
  errorMessage,
  forgetCredential,
  getMachine,
  inspectSSH,
  installSSH,
  upgradeSSH,
  upgradeAgent,
  issueEnrollment,
  revokeMachine,
} from "./api";
import type { Enrollment, Host, MachineState } from "./api";
const jobStates: Record<string, string> = {
  get queued() {
    return t("等待安装");
  },
  get running() {
    return t("正在安装");
  },
  get installed() {
    return t("安装命令完成");
  },
  get failed() {
    return t("安装失败");
  },
  get cancelled() {
    return t("已取消");
  },
};
const failures: Record<string, string> = {
  get awaiting_heartbeat() {
    return t("新版本已启动，正在等待心跳确认。");
  },
  get agent_upgraded() {
    return t("已收到目标版本的 Agent 心跳，升级完成。");
  },
  get upgrade_release_changed() {
    return t("控制端升级包已变化，请重新提交升级任务。");
  },
  get upgrade_failed_check_vps() {
    return t(
      "升级未完成，请检查 SSH 权限、原安装与控制端地址。启动失败时会尝试恢复旧版本。",
    );
  },
  get self_update_failed() {
    return t("Agent 自更新失败，请检查升级服务或使用 SSH 重试。");
  },
  get upgrade_unconfirmed() {
    return t(
      "升级命令已执行，但尚未确认新版本心跳。请检查机器状态，不要重复安装。",
    );
  },
  get service_installed() {
    return t("服务已安装，请以下方心跳状态为准。");
  },
  get host_key_changed() {
    return t("SSH 指纹发生变化，未发送登录凭据。");
  },
  get ssh_target_blocked() {
    return t("目标被网络访问策略阻止。");
  },
  get ssh_auth_or_handshake_failed() {
    return t("SSH 认证或握手失败。");
  },
  get ssh_unreachable() {
    return t("无法连接 SSH。");
  },
  get dns_failed() {
    return t("域名解析失败。");
  },
  get unsupported_platform() {
    return t("当前仅支持 Linux amd64 / arm64。");
  },
  get agent_artifact_unavailable() {
    return t("控制端缺少 Agent 构建产物。");
  },
  get install_failed_check_vps() {
    return t("请在 VPS 检查 systemd、sudo 权限、已有安装及控制端连通性。");
  },
  get target_changed() {
    return t("连接地址已变更，任务已停止。");
  },
  get authorization_revoked() {
    return t("提交者的组织管理权限已撤销。");
  },
  get interrupted_or_expired() {
    return t("任务中断或过期，未自动重试。");
  },
  get revoked() {
    return t("已撤销接入。");
  },
  get credential_unavailable() {
    return t("无法解密任务凭据，请检查部署密钥。");
  },
};
export default function MachineDialog({
  host,
  manage,
  onClose,
  onChanged,
}: {
  host: Host;
  manage: boolean;
  onClose: () => void;
  onChanged: () => void;
}) {
  useLocale();
  const dialog = useRef<HTMLDialogElement>(null);
  const [state, setState] = useState<MachineState | null>(null),
    [error, setError] = useState(""),
    [notice, setNotice] = useState(""),
    [busy, setBusy] = useState(false);
  const [path, setPath] = useState("manual"),
    [mode, setMode] = useState("manage"),
    [confirmManage, setConfirmManage] = useState(false),
    [enrollment, setEnrollment] = useState<Enrollment | null>(null);
  const [method, setMethod] = useState("pem"),
    [password, setPassword] = useState(""),
    [privateKey, setPrivateKey] = useState(""),
    [passphrase, setPassphrase] = useState(""),
    [filename, setFilename] = useState("");
  const [fingerprint, setFingerprint] = useState(""),
    [confirmFingerprint, setConfirmFingerprint] = useState(false),
    [retain, setRetain] = useState(false),
    [useSaved, setUseSaved] = useState(false),
    [confirmAction, setConfirmAction] = useState<"revoke" | "forget" | null>(
      null,
    );
  async function refresh() {
    setState(await getMachine(host.id));
  }
  useEffect(() => {
    const d = dialog.current;
    d?.showModal();
    let active = true;
    const poll = () =>
      getMachine(host.id)
        .then((v) => {
          if (active) setState(v);
        })
        .catch((e) => {
          if (active) setError(errorMessage(e));
        });
    void poll();
    const timer = window.setInterval(poll, 5000);
    return () => {
      active = false;
      window.clearInterval(timer);
      d?.close();
    };
  }, [host.id]);
  async function act(f: () => Promise<void>) {
    setBusy(true);
    setError("");
    setNotice("");
    try {
      await f();
      await refresh();
      onChanged();
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  }
  const privileged = path === "upgrade" || mode !== "manage" || confirmManage;
  const upgrading = state?.jobs.some(
    (j) => j.action === "upgrade" && ["queued", "running"].includes(j.state),
  );
  const latest = state?.latest_agent_version;
  const canUpgrade = Boolean(
    latest &&
    state?.agent &&
    !state.agent.revoked_at &&
    agentVersionAtLeast(latest, state.agent.metrics.version) &&
    !agentVersionAtLeast(state.agent.metrics.version, latest),
  );
  const metrics = state?.agent?.metrics;
  return (
    <dialog
      ref={dialog}
      className="host-dialog machine-dialog"
      aria-labelledby="machine-title"
      onCancel={(e) => {
        e.preventDefault();
        if (!busy) onClose();
      }}
    >
      <div className="dialog-heading">
        <div>
          <p className="eyebrow">MACHINE ACCESS</p>
          <h2 id="machine-title">
            {host.name}
            {t("· 机器接入")}
          </h2>
        </div>
        <button
          className="icon-button"
          aria-label={t("关闭机器接入")}
          disabled={busy}
          onClick={onClose}
        >
          ×
        </button>
      </div>
      <div className="machine-content">
        <p className="form-hint">
          {host.ssh_user}@{host.address}:{host.ssh_port}
        </p>
        {error && (
          <p role="alert" className="form-error">
            {t(error)}
          </p>
        )}
        {notice && (
          <p role="status" className="notice">
            {t(notice)}
          </p>
        )}
        <section className="machine-summary">
          <strong>
            {state?.agent
              ? state.agent.revoked_at
                ? t("机器授权已撤销")
                : state.agent.mode === "monitor"
                  ? t("已注册 · 探针模式")
                  : t("已注册 · 托管模式")
              : t("尚未注册 Agent")}
          </strong>
          {metrics?.hostname && (
            <>
              <p>
                {metrics.hostname} · {metrics.os} / {metrics.arch} ·{" "}
                {metrics.version}
              </p>
              <p>
                {metrics.cpus}
                {t("核 · 内存")}{" "}
                {Math.round(metrics.memory_total_bytes / 1024 / 1024)}
                {t("MiB · 可用")}
                {Math.round(metrics.memory_available_bytes / 1024 / 1024)}{" "}
                {t("MiB · 负载")}
                {metrics.load_1.toFixed(2)}
              </p>
              <p>
                {t("运行")}
                {Math.floor(metrics.uptime_seconds / 3600)}
                {t("小时")}
              </p>
            </>
          )}
          <p>
            {t(
              "在线状态以服务器列表中的最近心跳为准，超过 90 秒未上报会显示离线。",
            )}
          </p>
          {manage && latest && state?.agent && !state.agent.revoked_at && (
            <div className="machine-job">
              <strong>{t("Agent 升级")}</strong>
              <p>
                {t("当前版本")}：{metrics?.version || "—"} · {t("可用版本")}：
                {latest}
              </p>
              <p className="form-hint">
                {t(
                  state.agent.self_update
                    ? "Agent 已支持自更新，无需 SSH 凭据。点击后下载并校验升级包，保留配置和节点，新版本心跳确认后完成。"
                    : "升级服务尚未连接。旧版 Agent 需先通过 SSH 升级一次；已有升级服务时请检查其运行状态。",
                )}
              </p>
              <button
                className="primary"
                disabled={busy || upgrading || !canUpgrade}
                onClick={() => {
                  if (state.agent?.self_update) {
                    void act(async () => {
                      await upgradeAgent(host.id);
                      setNotice(t("升级任务已提交，正在等待新版本上线。"));
                    });
                  } else if (state.credential) {
                    void act(async () => {
                      await upgradeSSH(host.id, { use_saved: true });
                      setNotice(t("升级任务已提交，正在等待新版本上线。"));
                    });
                  } else {
                    setPath("upgrade");
                    setMode(state.agent!.mode);
                    setUseSaved(false);
                    setNotice(
                      t("请提供 SSH 凭据并核实主机指纹，然后开始升级。"),
                    );
                  }
                }}
              >
                {upgrading
                  ? t("正在升级…")
                  : canUpgrade
                    ? t("升级 Agent")
                    : agentVersionAtLeast(metrics?.version, latest)
                      ? t("已是当前或更新版本")
                      : t("无法判断当前版本")}
              </button>
              {canUpgrade && (state.credential || state.agent.self_update) && (
                <button
                  className="secondary"
                  disabled={busy || upgrading}
                  onClick={() => {
                    setPath("upgrade");
                    setMode(state.agent!.mode);
                    setUseSaved(false);
                    setFingerprint("");
                    setConfirmFingerprint(false);
                    setNotice(
                      t("请提供 SSH 凭据并核实主机指纹，然后开始升级。"),
                    );
                  }}
                >
                  {t("使用 SSH 升级")}
                </button>
              )}
            </div>
          )}
        </section>
        {manage && (
          <>
            <div className="mode-options">
              <button
                className={path === "manual" ? "primary" : "secondary"}
                disabled={busy}
                onClick={() => setPath("manual")}
              >
                {t("主动安装 Agent")}
              </button>
              <button
                className={path === "ssh" ? "primary" : "secondary"}
                disabled={busy}
                onClick={() => setPath("ssh")}
              >
                {t("SSH 自动安装")}
              </button>
            </div>
            <label htmlFor="agent-mode">{t("运行权限")}</label>
            <Select
              label={t("运行权限")}
              id="agent-mode"
              value={mode}
              disabled={busy || path === "upgrade"}
              onChange={(value) => {
                setMode(value);
                setConfirmManage(false);
                setEnrollment(null);
              }}
              options={[
                {
                  value: "manage",
                  label: t("托管模式"),
                  description: t("以 root 运行 · 支持授权的协议部署"),
                },
                {
                  value: "monitor",
                  label: t("探针模式"),
                  description: t("专用低权限用户 · 只采集状态"),
                },
              ]}
            />
            {mode === "manage" && path !== "upgrade" && (
              <label className="check-row">
                <input
                  type="checkbox"
                  checked={confirmManage}
                  onChange={(e) => setConfirmManage(e.target.checked)}
                  disabled={busy}
                />
                {t(
                  "我授权在此 VPS 上以 root 运行 Agent，用于状态采集及明确授权的协议安装与卸载；不提供任意远程命令。",
                )}
              </label>
            )}
            {path === "manual" ? (
              <section>
                <p className="form-hint">
                  {t(
                    "在 VPS 终端执行安装命令，按提示输入注册令牌。无需向星渡提供 SSH 密码或私钥。支持 Linux systemd，安装不会覆盖已有 Agent。",
                  )}
                </p>
                <button
                  className="primary"
                  disabled={busy || !privileged}
                  onClick={() =>
                    void act(async () => {
                      setEnrollment(
                        await issueEnrollment(host.id, mode, confirmManage),
                      );
                    })
                  }
                >
                  {t("生成一次性安装令牌")}
                </button>
                {enrollment && (
                  <div className="enrollment-result">
                    <label>{t("安装命令")}</label>
                    <textarea
                      readOnly
                      rows={4}
                      value={enrollment.command}
                      onFocus={(e) => e.target.select()}
                    />
                    <label>
                      {t("注册令牌（仅显示这一次，不放入命令参数）")}
                    </label>
                    <input
                      readOnly
                      value={enrollment.token}
                      onFocus={(e) => e.target.select()}
                    />
                    <p>
                      {t("有效期至")}{" "}
                      {new Date(enrollment.expires_at).toLocaleString(
                        localeTag(),
                      )}
                      {t("。新令牌接入后，旧 Agent 身份会失效。")}
                    </p>
                    {!enrollment.origin.startsWith("https://") && (
                      <p className="form-error">
                        {t(
                          "当前控制端是本地地址，仅适合本机验证。远程 VPS 需要可访问的 HTTPS 控制端地址。",
                        )}
                      </p>
                    )}
                  </div>
                )}
              </section>
            ) : (
              <section>
                <p className="form-hint">
                  {t(
                    "使用已登记的 SSH 用户。安装需要 root 或免密 sudo；不会修改 SSH 配置或防火墙。先通过云厂商控制台或已有可信连接核对指纹。",
                  )}
                </p>
                <button
                  className="secondary"
                  disabled={busy}
                  onClick={() =>
                    void act(async () => {
                      const r = await inspectSSH(host.id);
                      setFingerprint(r.fingerprint);
                      setConfirmFingerprint(false);
                    })
                  }
                >
                  {t("获取 SSH 主机指纹（不发送凭据）")}
                </button>
                <label htmlFor="ssh-fingerprint">
                  {t("已核实的 SHA256 主机指纹")}
                </label>
                <input
                  id="ssh-fingerprint"
                  value={fingerprint}
                  onChange={(e) => {
                    setFingerprint(e.target.value);
                    setConfirmFingerprint(false);
                  }}
                  placeholder="SHA256:…"
                  disabled={busy}
                />
                <label className="check-row">
                  <input
                    type="checkbox"
                    checked={confirmFingerprint}
                    onChange={(e) => setConfirmFingerprint(e.target.checked)}
                    disabled={busy}
                  />
                  {t("我已通过可信来源核对该指纹")}
                </label>
                {state?.credential && (
                  <label className="check-row">
                    <input
                      type="checkbox"
                      checked={useSaved}
                      disabled={busy}
                      onChange={(e) => {
                        setUseSaved(e.target.checked);
                        if (e.target.checked) {
                          setFingerprint(state.credential!.fingerprint);
                          setConfirmFingerprint(false);
                        }
                      }}
                    />
                    {t("使用加密保存的")}{" "}
                    {state.credential.method === "pem" ? t("私钥") : t("密码")}
                    （
                    {new Date(state.credential.saved_at).toLocaleString(
                      localeTag(),
                    )}
                    ）
                  </label>
                )}
                {!useSaved && (
                  <>
                    <label htmlFor="ssh-method">{t("认证方式")}</label>
                    <Select
                      label={t("认证方式")}
                      id="ssh-method"
                      value={method}
                      disabled={busy}
                      onChange={(value) => {
                        setMethod(value);
                        setPassword("");
                        setPrivateKey("");
                        setPassphrase("");
                        setFilename("");
                      }}
                      options={[
                        { value: "pem", label: t("PEM / OpenSSH 私钥") },
                        { value: "password", label: t("SSH 密码") },
                      ]}
                    />
                    {method === "password" ? (
                      <>
                        <label htmlFor="ssh-password">{t("SSH 密码")}</label>
                        <input
                          id="ssh-password"
                          type="password"
                          autoComplete="off"
                          value={password}
                          disabled={busy}
                          onChange={(e) => setPassword(e.target.value)}
                        />
                      </>
                    ) : (
                      <>
                        <label htmlFor="ssh-key">
                          {t("私钥文件（最大 24 KiB）")}
                        </label>
                        <input
                          id="ssh-key"
                          type="file"
                          accept=".pem,.key"
                          disabled={busy}
                          onChange={async (e) => {
                            const f = e.target.files?.[0];
                            if (!f) return;
                            if (f.size > 24 * 1024) {
                              setError(t("私钥文件过大"));
                              return;
                            }
                            setPrivateKey(await f.text());
                            setFilename(f.name);
                          }}
                        />
                        {filename && (
                          <small>
                            {t("已选择：")}
                            {filename}
                            {t("，内容不会回显")}
                          </small>
                        )}
                        <label htmlFor="ssh-passphrase">
                          {t("私钥口令（加密私钥必填）")}
                        </label>
                        <input
                          id="ssh-passphrase"
                          type="password"
                          autoComplete="off"
                          value={passphrase}
                          disabled={busy}
                          onChange={(e) => setPassphrase(e.target.value)}
                        />
                      </>
                    )}
                    <label className="check-row">
                      <input
                        type="checkbox"
                        checked={retain}
                        onChange={(e) => setRetain(e.target.checked)}
                        disabled={busy}
                      />
                      {t("长期加密保存凭据，供此组织管理员再次使用")}
                    </label>
                    <p className="form-hint">
                      {t(
                        "不勾选时，凭据仅加密暂存于本次任务，结束或过期后清除。已保存的凭据不会因取消勾选而自动删除，可在下方单独删除。",
                      )}
                    </p>
                  </>
                )}
                <button
                  className="primary"
                  disabled={
                    busy ||
                    upgrading ||
                    !privileged ||
                    !confirmFingerprint ||
                    (!useSaved &&
                      (method === "password" ? !password : !privateKey))
                  }
                  onClick={() =>
                    void act(async () => {
                      try {
                        await (path === "upgrade" ? upgradeSSH : installSSH)(
                          host.id,
                          {
                            method,
                            password,
                            private_key: privateKey,
                            passphrase,
                            fingerprint,
                            mode,
                            retain,
                            use_saved: useSaved,
                            confirm_manage: confirmManage,
                            confirm_fingerprint: confirmFingerprint,
                          },
                        );
                        setNotice(
                          path === "upgrade"
                            ? t("升级任务已提交，正在等待新版本上线。")
                            : t(
                                "安装任务已提交。凭据不会回显，进度每 5 秒更新。",
                              ),
                        );
                      } finally {
                        setPassword("");
                        setPrivateKey("");
                        setPassphrase("");
                        setFilename("");
                      }
                    })
                  }
                >
                  {path === "upgrade" ? t("开始升级") : t("开始 SSH 安装")}
                </button>
              </section>
            )}
            <h3>{t("安装与升级记录")}</h3>
            {!state?.jobs.length && (
              <p className="form-hint">{t("暂无 SSH 安装任务。")}</p>
            )}
            {state?.jobs.map((j) => (
              <article className="machine-job" key={j.id}>
                <strong>
                  {j.action === "upgrade"
                    ? j.state === "installed" && j.result === "agent_upgraded"
                      ? t("升级完成")
                      : j.state === "failed"
                        ? t("升级失败")
                        : j.state === "queued"
                          ? t("等待升级")
                          : j.state === "running"
                            ? t("正在升级…")
                            : (jobStates[j.state] ?? j.state)
                    : (jobStates[j.state] ?? j.state)}{" "}
                  · {j.action === "upgrade" && `${j.target_version} · `}
                  {j.mode === "manage" ? t("托管") : t("探针")}
                </strong>
                <small>
                  {new Date(j.created_at).toLocaleString(localeTag())}
                </small>
                {j.result && (
                  <p>
                    {failures[j.result] ??
                      t("任务未完成，请检查服务端与 VPS 状态。")}
                  </p>
                )}
              </article>
            ))}
            <div className="mode-options">
              {state?.credential && (
                <button
                  className="secondary"
                  disabled={busy}
                  onClick={() => setConfirmAction("forget")}
                >
                  {t("删除保存的 SSH 凭据")}
                </button>
              )}
              <button
                className="text-danger"
                disabled={busy}
                onClick={() => setConfirmAction("revoke")}
              >
                {t("撤销机器接入")}
              </button>
            </div>
            {confirmAction && (
              <div className="delete-confirm">
                <p>
                  {confirmAction === "forget"
                    ? t(
                        "删除长期保存的凭据？已提交的任务仍会继续使用其临时副本。",
                      )
                    : t(
                        "撤销 Agent、所有未使用的安装令牌并取消安装任务？已在 VPS 上发出的操作无法自动回滚；此操作不会停止或卸载已部署的协议服务。需要终止服务时，请先在「协议部署」中卸载并确认完成。",
                      )}
                </p>
                <button
                  className="danger"
                  disabled={busy}
                  onClick={() =>
                    void act(async () => {
                      if (confirmAction === "forget") {
                        await forgetCredential(host.id);
                        setUseSaved(false);
                      } else {
                        await revokeMachine(host.id);
                        setEnrollment(null);
                      }
                      setConfirmAction(null);
                    })
                  }
                >
                  {t("确认")}
                </button>
                <button
                  className="secondary"
                  disabled={busy}
                  onClick={() => setConfirmAction(null)}
                >
                  {t("取消")}
                </button>
              </div>
            )}
          </>
        )}
      </div>
    </dialog>
  );
}
