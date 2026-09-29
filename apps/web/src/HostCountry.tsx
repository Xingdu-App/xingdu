import { t, useLocale } from "./i18n";
import "./host-country.css";

export default function HostCountry({ code }: { code?: string }) {
  const locale = useLocale();
  if (!code || !/^[A-Z]{2}$/.test(code) || code === "ZZ") {
    return <small className="host-country is-unknown">{t("位置未知")}</small>;
  }
  const name =
    new Intl.DisplayNames([locale], { type: "region" }).of(code) || code;
  const flag = String.fromCodePoint(
    ...[...code].map((c) => 0x1f1e6 + c.charCodeAt(0) - 65),
  );
  return (
    <small className="host-country" title={`${name} · ${code}`}>
      <span className="host-country-flag" aria-hidden="true">
        {flag}
      </span>
      <span>{name}</span>
    </small>
  );
}
