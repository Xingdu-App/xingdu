import type { V2RayOptions } from "./api";
import Select from "./Select";
import { t } from "./i18n";

export const defaultV2Ray: V2RayOptions = { network: "tcp", tls: true };

export default function V2RayFields({
  value,
  onChange,
  protocol,
}: {
  value: V2RayOptions;
  onChange: (value: V2RayOptions) => void;
  protocol: string;
}) {
  const patch = (change: Partial<V2RayOptions>) =>
    onChange({ ...value, ...change });
  return (
    <div className="deployment-fields">
      <label>
        {t("部署预设")}
        <Select
          label={t("部署预设")}
          value="custom"
          options={[
            { value: "custom", label: t("当前自定义配置") },
            { value: "tcp", label: "TCP + TLS" },
            { value: "ws", label: "WebSocket + TLS" },
            { value: "grpc", label: "gRPC + TLS" },
            ...(protocol === "vless"
              ? [{ value: "reality", label: "REALITY + Vision" }]
              : []),
          ]}
          onChange={(preset) => {
            if (preset === "custom") return;
            const network =
              preset === "reality" ? "tcp" : (preset as "tcp" | "ws" | "grpc");
            onChange({
              engine: value.engine,
              network,
              tls: true,
              ...(network === "ws"
                ? { path: "/xingdu" }
                : network === "grpc"
                  ? { service_name: "xingdu" }
                  : {}),
              ...(preset === "reality"
                ? { reality: true, flow: "xtls-rprx-vision" }
                : {}),
            });
          }}
        />
      </label>
      <label>
        {t("运行时")}
        <select
          value={value.engine ?? "sing-box"}
          onChange={(e) =>
            onChange({
              network: "tcp",
              tls: value.tls,
              engine: e.target.value as "sing-box" | "xray",
            })
          }
        >
          <option value="sing-box">sing-box</option>
          <option value="xray">Xray</option>
        </select>
      </label>
      <label>
        {t("传输方式")}
        <select
          value={value.network}
          onChange={(e) => {
            const network = e.target.value as V2RayOptions["network"];
            onChange({
              engine: value.engine,
              encryption: value.encryption,
              packet_encoding: value.packet_encoding,
              fingerprint: value.fingerprint,
              reality: value.reality && network === "tcp",
              network,
              tls: value.tls,
              ...(network === "grpc"
                ? { service_name: "xingdu" }
                : network !== "tcp"
                  ? { path: "/xingdu" }
                  : {}),
            });
          }}
        >
          <option value="tcp">TCP</option>
          <option value="ws">WebSocket</option>
          <option value="grpc">gRPC</option>
          <option value="http">
            {value.engine === "xray" ? "HTTP/1.1" : "HTTP (TLS: HTTP/2)"}
          </option>
          {value.engine === "xray" && (
            <>
              <option value="httpupgrade">HTTPUpgrade</option>
              <option value="xhttp">XHTTP</option>
            </>
          )}
        </select>
      </label>
      {protocol === "vless" && (
        <label>
          {t("安全方式")}
          <Select
            label={t("安全方式")}
            value={value.reality ? "reality" : "tls"}
            options={[
              { value: "tls", label: "TLS" },
              { value: "reality", label: "REALITY" },
            ]}
            onChange={(security) =>
              onChange({
                engine: value.engine,
                network: "tcp",
                tls: true,
                ...(security === "reality"
                  ? { reality: true, flow: "xtls-rprx-vision" }
                  : {}),
              })
            }
          />
        </label>
      )}
      {protocol !== "trojan" && !value.reality && (
        <label className="check-row">
          <input
            type="checkbox"
            checked={value.tls !== false}
            onChange={(e) =>
              patch({ tls: e.target.checked, alpn: [], flow: "" })
            }
          />
          TLS
        </label>
      )}
      {!["tcp", "grpc"].includes(value.network) && (
        <>
          <label>
            {t("路径")}
            <input
              required
              maxLength={2048}
              value={value.path ?? ""}
              onChange={(e) => patch({ path: e.target.value })}
            />
          </label>
          <label>
            Host
            <input
              maxLength={253}
              value={value.host ?? ""}
              placeholder="node.example.com"
              onChange={(e) => patch({ host: e.target.value })}
            />
          </label>
        </>
      )}
      {value.network === "grpc" && (
        <label>
          gRPC service name
          <input
            required
            maxLength={128}
            value={value.service_name ?? ""}
            onChange={(e) => patch({ service_name: e.target.value })}
          />
        </label>
      )}
      {value.tls !== false &&
        !value.reality &&
        (value.network === "tcp" || value.engine === "xray") && (
          <label>
            ALPN
            <select
              value={value.alpn?.join(",") ?? ""}
              onChange={(e) =>
                patch({ alpn: e.target.value ? e.target.value.split(",") : [] })
              }
            >
              <option value="">{t("默认")}</option>
              <option value="h2">h2</option>
              <option value="http/1.1">http/1.1</option>
              {value.network === "xhttp" && <option value="h3">h3</option>}
              <option value="h2,http/1.1">h2, http/1.1</option>
            </select>
          </label>
        )}
      {protocol === "vless" &&
        ((value.network === "tcp" && value.tls !== false) ||
          (value.engine === "xray" && value.encryption)) && (
          <label className="check-row">
            <input
              type="checkbox"
              checked={value.flow === "xtls-rprx-vision"}
              onChange={(e) =>
                patch({ flow: e.target.checked ? "xtls-rprx-vision" : "" })
              }
            />
            Vision
          </label>
        )}
      {value.engine === "xray" && (
        <>
          {protocol === "vless" && !value.reality && (
            <label className="check-row">
              <input
                type="checkbox"
                checked={!!value.encryption}
                onChange={(e) =>
                  patch({ encryption: e.target.checked, flow: "" })
                }
              />
              VLESS Encryption
            </label>
          )}
          <label className="check-row">
            <input
              type="checkbox"
              checked={value.packet_encoding === "xudp"}
              onChange={(e) =>
                patch({ packet_encoding: e.target.checked ? "xudp" : "" })
              }
            />
            XUDP
          </label>
          <label>
            {t("客户端指纹")}
            <select
              value={value.fingerprint ?? ""}
              onChange={(e) => patch({ fingerprint: e.target.value })}
            >
              <option value="">{t("默认")}</option>
              <option value="chrome">Chrome</option>
              <option value="firefox">Firefox</option>
              <option value="safari">Safari</option>
              <option value="edge">Edge</option>
            </select>
          </label>
        </>
      )}
      {value.network === "xhttp" && (
        <>
          <label>
            XHTTP mode
            <select
              value={value.mode ?? "auto"}
              onChange={(e) =>
                patch({
                  mode: e.target.value as V2RayOptions["mode"],
                  download: undefined,
                })
              }
            >
              {["auto", "packet-up", "stream-up", "stream-one"].map((v) => (
                <option key={v} value={v}>
                  {v}
                </option>
              ))}
            </select>
          </label>
          <label>
            User-Agent
            <input
              maxLength={1024}
              value={value.headers?.["User-Agent"] ?? ""}
              onChange={(e) =>
                patch({
                  headers: e.target.value
                    ? { "User-Agent": e.target.value }
                    : {},
                })
              }
            />
          </label>
          {value.mode !== "stream-one" && (
            <label className="check-row">
              <input
                type="checkbox"
                checked={!!value.download}
                onChange={(e) =>
                  patch({
                    download: e.target.checked
                      ? { tls: value.tls !== false, alpn: value.alpn }
                      : undefined,
                  })
                }
              />
              {t("分离下载连接")}
            </label>
          )}
          {value.download && (
            <>
              <label className="check-row">
                <input
                  type="checkbox"
                  checked={value.download.tls}
                  onChange={(e) =>
                    patch({ download: { tls: e.target.checked } })
                  }
                />
                {t("下载连接 TLS")}
              </label>
              {value.download.tls && (
                <label>
                  {t("下载连接 ALPN")}
                  <select
                    value={value.download.alpn?.join(",") ?? ""}
                    onChange={(e) =>
                      patch({
                        download: {
                          tls: true,
                          alpn: e.target.value ? [e.target.value] : [],
                        },
                      })
                    }
                  >
                    <option value="">{t("默认")}</option>
                    <option value="http/1.1">http/1.1</option>
                    <option value="h2">h2</option>
                    <option value="h3">h3</option>
                  </select>
                </label>
              )}
            </>
          )}
        </>
      )}
    </div>
  );
}
