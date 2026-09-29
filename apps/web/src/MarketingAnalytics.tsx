import { useEffect, useState } from "react";
import { useContext } from "react";
import { MarketingLocale } from "./marketing-locale";
import { publicPages } from "./public-pages";
import {
  analyticsAISource,
  analyticsID,
  analyticsPage,
  analyticsReferrer,
  analyticsTarget,
} from "./analytics";

const consentKey = "xingdu.analytics-consent.v1";
const id = analyticsID(import.meta.env.VITE_GA_MEASUREMENT_ID);
let started = false;
let allowed = false;
let send: (...args: unknown[]) => void = () => {};
const paths = Object.values(publicPages).map((page) => page.path);
function start() {
  const location = analyticsPage(window.location.href, paths);
  if (!id || !location || started) return;
  started = true;
  const w = window as unknown as Record<string, unknown>;
  const queue: unknown[] = [];
  w.dataLayer = queue;
  // gtag expects Arguments objects, rather than arrays, in the data layer.
  send = function () {
    queue.push(arguments);
  };
  send("consent", "default", {
    analytics_storage: "granted",
    ad_storage: "denied",
    ad_user_data: "denied",
    ad_personalization: "denied",
  });
  send("js", new Date());
  send("config", id, {
    send_page_view: false,
    allow_google_signals: false,
    allow_ad_personalization_signals: false,
    cookie_path: "/",
    cookie_domain: "none",
    page_location: location,
    page_referrer: analyticsReferrer(document.referrer),
    page_title:
      publicPages[
        Object.keys(publicPages).find(
          (key) =>
            publicPages[key as keyof typeof publicPages].path ===
            window.location.pathname,
        ) as keyof typeof publicPages
      ].title,
  });
  const aiSource = analyticsAISource(window.location.href, document.referrer);
  send("event", "page_view", aiSource ? { ai_source: aiSource } : {});
  const script = document.createElement("script");
  script.async = true;
  script.src = `https://www.googletagmanager.com/gtag/js?id=${id}`;
  document.head.appendChild(script);
  document.addEventListener("click", (event) => {
    if (!allowed || !analyticsPage(window.location.href, paths)) return;
    const anchor =
      event.target instanceof Element ? event.target.closest("a") : null;
    const target =
      anchor && analyticsTarget(anchor.href, window.location.origin);
    if (target)
      send("event", "marketing_cta_click", {
        destination: target,
        ...(aiSource ? { ai_source: aiSource } : {}),
      });
  });
}
export default function MarketingAnalytics() {
  const en = useContext(MarketingLocale) === "en";
  const [visible, setVisible] = useState(false);
  const [eligible, setEligible] = useState(false);
  useEffect(() => {
    if (!id || !analyticsPage(window.location.href, paths)) return;
    setEligible(true);
    let choice: string | null = null;
    try {
      choice = localStorage.getItem(consentKey);
    } catch {
      /* Ask again when storage is unavailable. */
    }
    allowed = choice === "accepted";
    if (allowed) start();
    setVisible(!choice);
  }, []);
  function choose(accept: boolean) {
    allowed = accept;
    try {
      localStorage.setItem(consentKey, accept ? "accepted" : "denied");
    } catch {
      /* Choice still applies to this page. */
    }
    if (accept) start();
    else if (started && id) {
      (window as unknown as Record<string, unknown>)[`ga-disable-${id}`] = true;
      for (const cookie of document.cookie.split(";")) {
        const name = cookie.trim().split("=")[0];
        if (name === "_ga" || name.startsWith("_ga_"))
          document.cookie = `${name}=; Max-Age=0; Path=/`;
      }
      // Remove the loaded tag and its automatic handlers by reloading without consent.
      window.location.reload();
    }
    setVisible(false);
  }
  if (!eligible) return null;
  return (
    <>
      <button
        className="site-analytics-settings"
        onClick={() => setVisible(true)}
      >
        {en ? "Analytics preferences" : "访问统计偏好"}
      </button>
      {visible && (
        <section
          className="site-analytics-banner"
          aria-label={en ? "Analytics preferences" : "访问统计偏好"}
        >
          <p>
            {en
              ? "Allow Google Analytics cookies to help us understand visits to our public website? Console activity is excluded. Your choice does not affect product access."
              : "允许 Google Analytics 使用 Cookie 统计官网访问吗？不统计控制台操作，你的选择不影响使用产品。"}{" "}
            <a href="/privacy">{en ? "Privacy" : "隐私说明"}</a>
          </p>
          <div>
            <button onClick={() => choose(false)}>
              {en ? "Decline" : "拒绝"}
            </button>
            <button onClick={() => choose(true)}>
              {en ? "Allow analytics" : "允许统计"}
            </button>
          </div>
        </section>
      )}
    </>
  );
}
