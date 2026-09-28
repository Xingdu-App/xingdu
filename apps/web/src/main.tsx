import { StrictMode } from "react";
import { createRoot, hydrateRoot } from "react-dom/client";
import "./index.css";
import SessionGate from "./SessionGate.tsx";
import Marketing from "./Marketing.tsx";
import { publicPages } from "./public-pages";
import type { PublicPage } from "./public-pages";
import "./App.css";

// Older invitations used /#invite=; keep the fragment out of request logs.
if (
  window.location.pathname === "/" &&
  window.location.hash.startsWith("#invite=")
) {
  window.history.replaceState(null, "", "/app" + window.location.hash);
}
const pathname = window.location.pathname.replace(/\/+$/, "") || "/";
const page = (Object.keys(publicPages) as PublicPage[]).find(
  (key) => publicPages[key].path === pathname,
);
const consolePage = pathname === "/app" || pathname === "/login";
const content = page ? (
  <Marketing page={page} />
) : consolePage ? (
  <SessionGate />
) : (
  <div className="auth-page">
    <section className="auth-card">
      <p className="eyebrow">404 · XINGDU</p>
      <h1>这一站，还没有连接。</h1>
      <p className="auth-subtitle">页面不存在，回到首页继续探索。</p>
      <a className="primary auth-submit" href="/">
        返回首页
      </a>
    </section>
  </div>
);
document.title = page
  ? publicPages[page].title
  : consolePage
    ? "控制台 · 星渡 Xingdu"
    : "页面不存在 · 星渡 Xingdu";
const root = document.getElementById("root")!;
const app = <StrictMode>{content}</StrictMode>;
if (page && root.dataset.prerender === "true") hydrateRoot(root, app);
else createRoot(root).render(app);
