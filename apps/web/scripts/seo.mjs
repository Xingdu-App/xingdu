export function siteOrigin(value = "https://xingdu.app") {
  const url = new URL(value);
  if (
    !["https:", "http:"].includes(url.protocol) ||
    url.username ||
    url.password ||
    url.pathname !== "/" ||
    url.search ||
    url.hash
  ) {
    throw new Error(
      "XINGDU_SITE_URL must be an HTTP(S) origin without credentials, path, query or fragment",
    );
  }
  return url.origin;
}
export const escapeXML = (value) =>
  String(value)
    .replaceAll("&", "&amp;")
    .replaceAll('"', "&quot;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;");
export function seoHead(meta, origin, post) {
  const url = origin + meta.path;
  const image = origin + "/xingdu-logo.png";
  const brand = {
    "@type": "Organization",
    "@id": origin + "/#organization",
    name: "星渡 Xingdu",
    url: origin + "/",
    logo: image,
    email: "info@xingdu.app",
  };
  const website = {
    "@type": "WebSite",
    "@id": origin + "/#website",
    name: "星渡 Xingdu",
    alternateName: "Xingdu",
    url: origin + "/",
    inLanguage: "zh-CN",
    publisher: { "@id": brand["@id"] },
  };
  const page = {
    "@type": post
      ? "WebPage"
      : meta.path === "/blog"
        ? "CollectionPage"
        : "WebPage",
    "@id": url + "#webpage",
    url,
    name: meta.title,
    description: meta.description,
    inLanguage: "zh-CN",
    isPartOf: { "@id": website["@id"] },
  };
  const graph = [brand, website, page];
  if (post) {
    graph.push({
      "@type": "BlogPosting",
      "@id": url + "#article",
      headline: post.title,
      description: post.description,
      dateModified: post.updated,
      author: { "@id": brand["@id"] },
      publisher: { "@id": brand["@id"] },
      mainEntityOfPage: { "@id": page["@id"] },
      inLanguage: "zh-CN",
      articleSection: post.category,
      isAccessibleForFree: true,
    });
    graph.push({
      "@type": "BreadcrumbList",
      itemListElement: [
        ["首页", origin + "/"],
        ["博客", origin + "/blog"],
        [post.title, url],
      ].map(([name, item], i) => ({
        "@type": "ListItem",
        position: i + 1,
        name,
        item,
      })),
    });
  }
  const metaTag = (name, value, property = false) =>
    `<meta ${property ? "property" : "name"}="${name}" content="${escapeXML(value)}" />`;
  return [
    `<link rel="canonical" href="${escapeXML(url)}" />`,
    `<link rel="alternate" type="application/rss+xml" title="星渡博客" href="${escapeXML(origin)}/blog/feed.xml" />`,
    metaTag("robots", "index,follow,max-image-preview:large"),
    metaTag("og:type", post ? "article" : "website", true),
    metaTag("og:locale", "zh_CN", true),
    metaTag("og:site_name", "星渡 Xingdu", true),
    metaTag("og:title", meta.title, true),
    metaTag("og:description", meta.description, true),
    metaTag("og:url", url, true),
    metaTag("og:image", image, true),
    metaTag("og:image:alt", "星渡 Xingdu 标志", true),
    metaTag("twitter:card", "summary"),
    metaTag("twitter:title", meta.title),
    metaTag("twitter:description", meta.description),
    metaTag("twitter:image", image),
    metaTag("twitter:image:alt", "星渡 Xingdu 标志"),
    ...(post
      ? [
          metaTag("article:modified_time", post.updated, true),
          metaTag("article:section", post.category, true),
        ]
      : []),
    `<script type="application/ld+json">${JSON.stringify({ "@context": "https://schema.org", "@graph": graph }).replaceAll("<", "\\u003c")}</script>`,
  ].join("\n");
}
export function sitemap(pages, origin) {
  return `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">${Object.values(
    pages,
  )
    .map(
      (page) =>
        `<url><loc>${escapeXML(origin + page.path)}</loc>${page.updated ? `<lastmod>${escapeXML(page.updated)}</lastmod>` : ""}</url>`,
    )
    .join("")}</urlset>\n`;
}
export function rss(posts, origin) {
  return `<?xml version="1.0" encoding="UTF-8"?>\n<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom"><channel><title>星渡博客</title><link>${escapeXML(origin)}/blog</link><description>个人 VPS 管理、Agent 接入与客户端配置指南</description><language>zh-CN</language><atom:link href="${escapeXML(origin)}/blog/feed.xml" rel="self" type="application/rss+xml"/>${posts.map((post) => `<item><title>${escapeXML(post.title)}</title><link>${escapeXML(origin)}/blog/${post.slug}</link><guid isPermaLink="true">${escapeXML(origin)}/blog/${post.slug}</guid><description>${escapeXML(post.description)}</description><category>${escapeXML(post.category)}</category></item>`).join("")}</channel></rss>\n`;
}
