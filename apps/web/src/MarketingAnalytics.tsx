import {
  consoleAnalyticsLocation,
  interactionAction,
} from "./interaction-analytics";
import { useEffect } from "react";
import en from "./locales/en.json";
import marketingEn from "./locales/marketing-en.json";
import { publicPages } from "./public-pages";
import {
  analyticsAISource,
  analyticsID,
  analyticsPage,
  analyticsReferrer,
  analyticsClick,
  analyticsContent,
  analyticsScrollDepth,
} from "./analytics";

const id = analyticsID(import.meta.env.VITE_GA_MEASUREMENT_ID);
let started = false;
let consoleEnabled = false;
let lastConsolePage = "";
const translations = { ...en, ...marketingEn };
let send: (...args: unknown[]) => void = () => {};
const paths = Object.values(publicPages).map((page) => page.path);
function start(consoleMode = false) {
  const consoleLocation = consoleMode
    ? consoleAnalyticsLocation(window.location.href)
    : null;
  const location = consoleMode
    ? consoleLocation?.location
    : analyticsPage(window.location.href, paths);
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
    page_title: consoleMode
      ? "Xingdu console"
      : publicPages[
          Object.keys(publicPages).find(
            (key) =>
              publicPages[key as keyof typeof publicPages].path ===
              window.location.pathname,
          ) as keyof typeof publicPages
        ].title,
  });
  const script = document.createElement("script");
  script.async = true;
  script.src = `https://www.googletagmanager.com/gtag/js?id=${id}`;
  document.head.appendChild(script);
  document.addEventListener(
    "click",
    (event) => {
      const publicLocation = analyticsPage(window.location.href, paths);
      const consoleLocation = consoleEnabled
        ? consoleAnalyticsLocation(window.location.href)
        : null;
      if (!publicLocation && !consoleLocation) return;
      if (!(event.target instanceof Element)) return;
      const control = event.target.closest(
        "button, a, summary, [role=button], [role=tab], [role=option], [role=menuitem], [role=combobox], input[type=checkbox], input[type=radio], label",
      );
      // Native label activation forwards a second click to its input.
      // Record only that input click, avoiding a duplicate event.
      if (control?.tagName === "LABEL") return;
      if (!control || control.matches(":disabled, [aria-disabled=true]"))
        return;
      const kind = control.matches("a")
        ? "link"
        : control.matches("summary")
          ? "summary"
          : control.matches("input[type=checkbox]")
            ? "checkbox"
            : control.matches("input[type=radio]")
              ? "radio"
              : control.getAttribute("role") || "button";
      const action = interactionAction(
        control.getAttribute("aria-label") || control.textContent || "",
        kind,
        translations,
      );
      if (!action) return;
      const placement = control.closest("dialog")
        ? "dialog"
        : control.closest("aside, nav")
          ? "navigation"
          : control.closest("header")
            ? "header"
            : control.closest("footer")
              ? "footer"
              : "content";
      const target = control.matches("a")
        ? consoleAnalyticsLocation((control as HTMLAnchorElement).href)
        : null;
      send("event", "ui_click", {
        page_location: consoleLocation?.location || publicLocation,
        page_title: consoleLocation
          ? "Xingdu console"
          : Object.values(publicPages).find(
              (page) => page.path === window.location.pathname,
            )?.title,
        surface: consoleLocation ? "console" : "marketing",
        page_name: consoleLocation?.page || new URL(publicLocation!).pathname,
        action,
        control_type: kind,
        placement,
        ...(target &&
        new URL((control as HTMLAnchorElement).href).origin ===
          window.location.origin
          ? { destination_page: target.page }
          : {}),
      });
    },
    { capture: true },
  );
  if (consoleMode) return;
  const aiSource = analyticsAISource(window.location.href, document.referrer);
  const content = analyticsContent(window.location.pathname, paths)!;
  const emit = (name: string, params: Record<string, unknown> = {}) => {
    if (analyticsPage(window.location.href, paths) !== location) return;
    send("event", name, {
      page_location: location,
      ...content,
      ...(aiSource ? { ai_source: aiSource } : {}),
      ...params,
    });
  };
  emit("page_view");
  if (content.content_type !== "page") emit("marketing_content_view");

  document.addEventListener("click", (event) => {
    if (!analyticsPage(window.location.href, paths)) return;
    const anchor =
      event.target instanceof Element ? event.target.closest("a") : null;
    const action =
      anchor && analyticsClick(anchor.href, window.location.origin, paths);
    if (action) {
      const placement = anchor.closest("header")
        ? "header"
        : anchor.closest("footer")
          ? "footer"
          : "content";
      emit(action.name, { ...action.params, placement });
    }
  });
  const reported = new Set<number>();
  document.addEventListener(
    "scroll",
    () => {
      if (
        document.visibilityState !== "visible" ||
        analyticsPage(window.location.href, paths) !== location
      )
        return;
      const depth = analyticsScrollDepth(
        window.scrollY,
        document.documentElement.scrollHeight,
        window.innerHeight,
      );
      for (const percent of [50, 90]) {
        if (depth >= percent && !reported.has(percent)) {
          reported.add(percent);
          emit("marketing_scroll", { percent_scrolled: percent });
        }
      }
    },
    { passive: true },
  );
}
export default function MarketingAnalytics() {
  useEffect(() => {
    start();
  }, []);
  return null;
}

export function ConsoleAnalytics({ page }: { page: string }) {
  useEffect(() => {
    consoleEnabled = true;
    start(true);
    const location = consoleAnalyticsLocation(window.location.href);
    if (id && started && location && location.page !== lastConsolePage) {
      lastConsolePage = location.page;
      send("config", id, {
        send_page_view: false,
        page_location: location.location,
        page_title: "Xingdu console",
        page_referrer: analyticsReferrer(document.referrer),
      });
      send("event", "page_view", {
        page_location: location.location,
        page_title: "Xingdu console",
        surface: "console",
        page_name: location.page,
      });
    }
    return () => {
      consoleEnabled = false;
    };
  }, [page]);
  return null;
}
