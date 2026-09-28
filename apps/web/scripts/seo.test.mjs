import assert from "node:assert/strict";
import { readFile, access } from "node:fs/promises";
import { test } from "node:test";
import { siteOrigin, seoHead, rss } from "./seo.mjs";

const origin = siteOrigin(process.env.XINGDU_SITE_URL);
const read = (path) => readFile(`dist/${path}`, "utf8");
const site = await read("sitemap.xml");
const urls = [...site.matchAll(/<loc>([^<]+)<\/loc>/g)].map(
  (match) => new URL(match[1]),
);

test("site origin rejects paths, credentials and non-web schemes", () => {
  for (const value of [
    "javascript:alert(1)",
    "https://example.com/blog",
    "https://user:secret@example.com",
    "https://example.com/?q=1",
    "https://example.com/#home",
  ])
    assert.throws(() => siteOrigin(value));
  assert.equal(
    siteOrigin("https://example.com:8443/"),
    "https://example.com:8443",
  );
});

test("metadata and feed escape content without creating extra script tags", () => {
  const title = '</script><script>alert("x")</script>&';
  const html = seoHead({ path: "/", title, description: title }, origin);
  assert.equal((html.match(/<script /g) || []).length, 1);
  assert.equal((html.match(/<\/script>/g) || []).length, 1);
  assert.match(
    rss(
      [{ slug: "test", title, description: title, category: "A & B" }],
      origin,
    ),
    /&lt;\/script&gt;/,
  );
});

test("every sitemap URL has a complete unique public HTML page and working local links", async () => {
  assert.ok(urls.length >= 12);
  assert.equal(new Set(urls.map((url) => url.href)).size, urls.length);
  const titles = new Set();
  const descriptions = new Set();
  for (const url of urls) {
    assert.equal(url.origin, origin);
    assert.ok(!url.pathname.startsWith("/app") && url.pathname !== "/login");
    const html = await read(
      url.pathname === "/"
        ? "index.html"
        : `${url.pathname.slice(1)}/index.html`,
    );
    assert.match(html, /data-prerender="true"/);
    assert.equal((html.match(/<h1[ >]/g) || []).length, 1, url.href);
    const title = html.match(/<title>([^<]+)<\/title>/)?.[1];
    const description = html.match(/name="description" content="([^"]+)"/)?.[1];
    assert.ok(title && !titles.has(title), url.href);
    titles.add(title);
    assert.ok(description && !descriptions.has(description), url.href);
    descriptions.add(description);
    assert.equal(html.match(/rel="canonical" href="([^"]+)"/)?.[1], url.href);
    assert.match(html, /name="twitter:card"/);
    assert.doesNotMatch(html, /content="noindex/);
    const schema = JSON.parse(
      html.match(/<script type="application\/ld\+json">(.*?)<\/script>/s)[1],
    );
    const article = schema["@graph"].find(
      (item) => item["@type"] === "BlogPosting",
    );
    if (url.pathname.startsWith("/blog/")) {
      assert.ok(article, url.href);
      assert.match(html, /aria-label="文章目录"/);
      assert.ok(
        html.toLowerCase().includes(`datetime="${article.dateModified}"`),
      );
      assert.ok(
        schema["@graph"].some((item) => item["@type"] === "BreadcrumbList"),
      );
      assert.match(html, /继续阅读/);
    } else assert.equal(article, undefined);
    for (const [, href] of html.matchAll(/href="([^"&]+)"/g)) {
      if (href.startsWith("#"))
        assert.ok(html.includes(`id="${href.slice(1)}"`), `${url.href}${href}`);
      if (!href.startsWith("/") || href.startsWith("//")) continue;
      const target = new URL(href, origin);
      if (target.pathname.startsWith("/app") || target.pathname === "/login")
        continue;
      await access(
        `dist${target.pathname === "/" ? "/index.html" : target.pathname.includes(".") ? target.pathname : `${target.pathname}/index.html`}`,
      );
    }
  }
});

test("RSS contains article URLs; console and not-found shells stay noindex", async () => {
  const feed = await read("blog/feed.xml");
  for (const url of urls.filter((url) => url.pathname.startsWith("/blog/")))
    assert.ok(feed.includes(url.href));
  for (const path of ["app/index.html", "login/index.html", "404.html"]) {
    const html = await read(path);
    assert.match(html, /name="robots" content="noindex,nofollow"/);
    assert.doesNotMatch(html, /BlogPosting/);
  }
  assert.ok(
    (await read("robots.txt")).includes(`Sitemap: ${origin}/sitemap.xml`),
  );
});
