import { useEffect } from "react";
import { publicPages } from "./public-pages";
import {
  analyticsAISource,
  analyticsID,
  analyticsPage,
  analyticsReferrer,
  analyticsTarget,
} from "./analytics";

const id = analyticsID(import.meta.env.VITE_GA_MEASUREMENT_ID);
let started = false;
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
    if (!analyticsPage(window.location.href, paths)) return;
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
  useEffect(() => {
    start();
  }, []);
  return null;
}
