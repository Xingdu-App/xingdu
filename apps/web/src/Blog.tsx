import { useMarketingText } from "./marketing-locale";
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
  const post = blogPosts.find((item) => item.slug === slug);
  if (!post)
    return (
      <div className="site-container blog-index">
        <section className="site-page-intro">
          <p className="site-eyebrow">XINGDU JOURNAL</p>
          <h1>
            服务器管理，<em>从方法开始。</em>
          </h1>
          <p>
            从 VPS 台账、Agent
            接入到客户端配置，找到适合个人使用或多人协作的管理流程。
          </p>
        </section>
        <BlogCards />
        <p className="blog-meta">
          星渡产品内容 · 以当前预览版本为基础 ·{" "}
          <a href="/blog/feed.xml">RSS 订阅</a>
        </p>
      </div>
    );
  return (
    <article className="site-container blog-article">
      <nav className="blog-breadcrumbs" aria-label="面包屑">
        <a href="/">首页</a>
        <span aria-hidden="true">/</span>
        <a href="/blog">博客</a>
        <span aria-hidden="true">/</span>
        <span aria-current="page">{post.title}</span>
      </nav>
      <header className="site-page-intro">
        <p className="site-eyebrow">{post.category}</p>
        <h1>{post.title}</h1>
        <p>{post.description}</p>
        <div className="blog-meta">
          作者：<a href="/">星渡 Xingdu</a> · 更新于{" "}
          <time dateTime={post.updated}>{post.updated}</time>
        </div>
      </header>
      <div className="site-document-layout">
        <nav aria-label="文章目录">
          {post.sections.map((section, i) => (
            <a key={section.id} href={`#${section.id}`}>
              <span>
                {String(i + 1).padStart(2, "0")} · {section.title}
              </span>
            </a>
          ))}
        </nav>
        <div className="site-document">
          {post.sections.map((section) => (
            <section key={section.id} id={section.id}>
              <h2>{section.title}</h2>
              {section.paragraphs.map((text) => (
                <p key={text}>{text}</p>
              ))}
              {section.checklist && (
                <ul>
                  {section.checklist.map((text) => (
                    <li key={text}>{text}</li>
                  ))}
                </ul>
              )}
            </section>
          ))}
          <section>
            <h2>进一步阅读</h2>
            <ul>
              {post.references.map((reference) => (
                <li key={reference.href}>
                  <a href={reference.href}>{reference.title}</a>
                </li>
              ))}
            </ul>
          </section>
          <aside className="document-note">
            <h2>把流程落实到你的工作空间</h2>
            <p>
              从第一台服务器开始，验证接入、部署与客户端配置。星渡当前为产品预览，请先了解功能范围。
            </p>
            <p>
              <a href="/app">进入星渡控制台</a> ·{" "}
              <a href="/help">查看入门帮助</a> ·{" "}
              <a href="/pricing">比较部署方式</a>
            </p>
          </aside>
        </div>
      </div>
      <section className="blog-related">
        <h2>继续阅读</h2>
        <BlogCards
          posts={post.related.flatMap((slug) =>
            blogPosts.filter((item) => item.slug === slug),
          )}
        />
      </section>
    </article>
  );
}
