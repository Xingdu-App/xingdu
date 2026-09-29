import {
  siteOrigin,
  escapeXML as escape,
  seoHead,
  sitemap,
  rss,
  robots,
} from "./seo.mjs";
import { build } from "vite";
import { readFile, writeFile, mkdir, rm } from "node:fs/promises";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";
const generated = resolve("node_modules/.cache/xingdu-prerender");
await build({
  build: { ssr: "src/public-entry.tsx", outDir: generated, emptyOutDir: true },
  publicDir: false,
});
const { render, publicPages, blogPosts } = await import(
  pathToFileURL(resolve(generated, "public-entry.js"))
);
const template = await readFile("dist/index.html", "utf8");
const origin = siteOrigin(process.env.XINGDU_SITE_URL);
for (const [page, meta] of Object.entries(publicPages)) {
  let html = template
    .replace(/<title>.*?<\/title>/s, `<title>${escape(meta.title)}</title>`)
    .replace(
      /<meta\s+name="description"[^>]*>/,
      `<meta name="description" content="${escape(meta.description)}" />`,
    )
    .replace(
      '<div id="root"></div>',
      `<div id="root" data-prerender="true">${render(page)}</div>`,
    )
    .replace(
      "</head>",
      seoHead(
        meta,
        origin,
        blogPosts.find((post) => post.slug === meta.articleSlug),
      ) + "\n</head>",
    );
  const directory = meta.path === "/" ? "dist" : "dist" + meta.path;
  await mkdir(directory, { recursive: true });
  await writeFile(directory + "/index.html", html);
}
// Console pages never serve the marketing prerender or expose authenticated data.
for (const path of ["app", "login"]) {
  await mkdir("dist/" + path, { recursive: true });
  await writeFile(
    "dist/" + path + "/index.html",
    template
      .replace(
        "<title>星渡 Xingdu</title>",
        "<title>控制台 · 星渡 Xingdu</title>",
      )
      .replace(
        "</head>",
        '<meta name="robots" content="noindex,nofollow" /></head>',
      ),
  );
}
await writeFile(
  "dist/404.html",
  template.replace(
    "</head>",
    '<meta name="robots" content="noindex,nofollow" /></head>',
  ),
);
await writeFile("dist/robots.txt", robots(origin));
await writeFile("dist/sitemap.xml", sitemap(publicPages, origin));
await writeFile("dist/blog/feed.xml", rss(blogPosts, origin));
await rm(generated, { recursive: true, force: true });
console.log(
  `Prerendered ${Object.keys(publicPages).length} public pages, console shells, sitemap and robots.txt.`,
);
