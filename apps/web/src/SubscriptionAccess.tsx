import { useState } from "react";
import { QRCodeSVG } from "qrcode.react";
import { t } from "./i18n";
import {
  copySubscriptionURL,
  subscriptionImportURL,
} from "./subscription-access";

export default function SubscriptionAccess({
  path,
  name,
  format,
  enabled,
}: {
  path: string;
  name: string;
  format: string;
  enabled: boolean;
}) {
  const [qr, setQR] = useState(false);
  const [notice, setNotice] = useState("");
  const url = new URL(path, window.location.origin);
  url.searchParams.set("format", format);
  const value = url.href;
  const importURL = subscriptionImportURL(format, value);
  return (
    <div className="subscription-access">
      <label>
        <span>{t("订阅地址")}</span>
        <input
          type="text"
          readOnly
          value={value}
          autoComplete="off"
          spellCheck={false}
          aria-label={t("订阅地址")}
          onFocus={(e) => e.currentTarget.select()}
        />
      </label>
      <div className="subscription-actions">
        <button
          className="secondary compact"
          onClick={async () => {
            const copied = await copySubscriptionURL(value);
            setNotice(
              copied
                ? t("订阅链接已复制。")
                : t("复制受限，请选中上方地址手动复制。"),
            );
          }}
        >
          {t("复制链接")}
        </button>
        <button
          className="secondary compact"
          disabled={!enabled}
          aria-expanded={qr}
          onClick={() => setQR((v) => !v)}
        >
          {qr ? t("收起二维码") : t("二维码")}
        </button>
        {importURL && enabled && (
          <a
            className="button secondary compact"
            href={importURL}
            rel="noreferrer"
            onClick={() =>
              setNotice(
                t(
                  "已尝试打开客户端；若没有响应，请先安装客户端，或复制地址手动添加。",
                ),
              )
            }
          >
            {t("导入 {0}", { 0: format === "stash" ? "Stash" : "Surge" })}
          </a>
        )}
      </div>
      {!enabled && (
        <p className="form-hint">{t("订阅已停用，此地址暂时无法获取配置。")}</p>
      )}
      {qr && enabled && (
        <div className="subscription-qr">
          <QRCodeSVG
            value={value}
            size={200}
            marginSize={4}
            level="M"
            title={t("{0} 的订阅二维码", { 0: name })}
          />
          <p>{t("请使用客户端扫描订阅二维码，或复制地址手动添加。")}</p>
        </div>
      )}
      {notice && (
        <p role="status" className="form-hint">
          {notice}
        </p>
      )}
    </div>
  );
}
