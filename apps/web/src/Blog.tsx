import { useContext } from "react";
import { MarketingLocale, useMarketingText } from "./marketing-locale";
import { blogPath, blogPosts } from "./blog-posts";
import type { BlogPost } from "./blog-posts";
import "./Blog.css";

export function BlogCards({ posts = blogPosts }: { posts?: BlogPost[] }) {
  const t = useMarketingText();
  return (
    <div className="blog-grid">
      {posts.map((post) => (
        <article className="blog-card" key={post.slug}>
          <span className="site-eyebrow">{t(post.category)}</span>
          <h2>
            <a href={blogPath(post)}>{t(post.title)}</a>
          </h2>
          <p>{t(post.description)}</p>
          <span className="blog-meta">
            {t("更新于")} <time dateTime={post.updated}>{post.updated}</time>
          </span>
          <a className="site-text-link" href={blogPath(post)}>
            {t("阅读全文")} <span aria-hidden="true">↗</span>
            <span className="blog-sr-only">：{t(post.title)}</span>
          </a>
        </article>
      ))}
    </div>
  );
}
export default function Blog({ slug }: { slug?: string }) {
  const locale = useContext(MarketingLocale);
  const t = useMarketingText();
  const post = blogPosts.find((item) => item.slug === slug);
  if (!post)
    return (
      <div className="site-container blog-index">
        <section className="site-page-intro">
          <p className="site-eyebrow">XINGDU JOURNAL</p>
          <h1>
            {t("服务器管理，")}
            <em>{t("从方法开始。")}</em>
          </h1>
          <p>
            {t(
              "从 VPS 台账、Agent 接入到客户端配置，找到适合个人使用或多人协作的管理流程。",
            )}
          </p>
        </section>
        <BlogCards />
        <p className="blog-meta">
          {t("星渡产品指南 ·")} <a href="/blog/feed.xml">{t("RSS 订阅")}</a>
        </p>
      </div>
    );
  return (
    <article className="site-container blog-article" lang={locale}>
      <nav className="blog-breadcrumbs" aria-label={t("面包屑")}>
        <a href="/">{t("首页")}</a>
        <span aria-hidden="true">/</span>
        <a href="/blog">{t("博客")}</a>
        <span aria-hidden="true">/</span>
        <span aria-current="page">{t(post.title)}</span>
      </nav>
      <header className="site-page-intro">
        <p className="site-eyebrow">{t(post.category)}</p>
        <h1>{t(post.title)}</h1>
        <p>{t(post.description)}</p>
        <div className="blog-meta">
          {t("作者：")}
          <a href="/">{t("星渡 Xingdu")}</a> {t(" · 更新于")}{" "}
          <time dateTime={post.updated}>{post.updated}</time>
        </div>
      </header>
      <div className="site-document-layout">
        <nav aria-label={t("文章目录")}>
          {post.sections.map((section, i) => (
            <a key={section.id} href={`#${section.id}`}>
              <span>
                {String(i + 1).padStart(2, "0")} · {t(section.title)}
              </span>
            </a>
          ))}
        </nav>
        <div className="site-document">
          {post.sections.map((section) => (
            <section key={section.id} id={section.id}>
              <h2>{t(section.title)}</h2>
              {section.paragraphs.map((text) => (
                <p key={t(text)}>{t(text)}</p>
              ))}
              {section.checklist && (
                <ul>
                  {section.checklist.map((text) => (
                    <li key={t(text)}>{t(text)}</li>
                  ))}
                </ul>
              )}
            </section>
          ))}
          <section>
            <h2>{t("进一步阅读")}</h2>
            <ul>
              {post.references.map((reference) => (
                <li key={reference.href}>
                  <a href={reference.href}>{t(reference.title)}</a>
                </li>
              ))}
            </ul>
          </section>
          <aside className="document-note">
            <h2>{t("把流程落实到你的工作空间")}</h2>
            <p>
              {t(
                "从第一台服务器开始，验证接入、部署与客户端配置。使用前请先了解功能范围。",
              )}
            </p>
            <p>
              <a href="/app">{t("进入星渡控制台")}</a> ·{" "}
              <a href="/help">{t("查看入门帮助")}</a> ·{" "}
              <a href="/pricing">{t("比较部署方式")}</a>
            </p>
          </aside>
        </div>
      </div>
      <section className="blog-related">
        <h2>{t("继续阅读")}</h2>
        <BlogCards
          posts={post.related.flatMap((slug) =>
            blogPosts.filter((item) => item.slug === slug),
          )}
        />
      </section>
    </article>
  );
}
