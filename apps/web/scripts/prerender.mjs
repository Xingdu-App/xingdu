import { build } from "vite";
import { readFile, writeFile, mkdir, rm } from "node:fs/promises";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";
const generated = resolve("node_modules/.cache/xingdu-prerender");
await build({
  build: { ssr: "src/public-entry.tsx", outDir: generated, emptyOutDir: true },
  publicDir: false,
});
const { render, publicPages } = await import(
  pathToFileURL(resolve(generated, "public-entry.js"))
);
const template = await readFile("dist/index.html", "utf8");
const siteURL = new URL(process.env.XINGDU_SITE_URL || "https://xingdu.app");
if (!["http:", "https:"].includes(siteURL.protocol)) {
  throw new Error("XINGDU_SITE_URL must use HTTP or HTTPS");
}
const origin = siteURL.origin;
const escape = (s) =>
  s
    .replaceAll("&", "&amp;")
    .replaceAll('"', "&quot;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;");
for (const [page, meta] of Object.entries(publicPages)) {
  const canonical = origin + meta.path;
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
      `<link rel="canonical" href="${escape(canonical)}" />\n<meta property="og:type" content="website" />\n<meta property="og:title" content="${escape(meta.title)}" />\n<meta property="og:description" content="${escape(meta.description)}" />\n<meta property="og:url" content="${escape(canonical)}" />\n<meta property="og:image" content="${escape(origin)}/xingdu-logo.png" />\n</head>`,
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
await writeFile(
  "dist/robots.txt",
  `User-agent: *\nAllow: /\nDisallow: /app\nDisallow: /login\nDisallow: /api/\nSitemap: ${origin}/sitemap.xml\n`,
);
await writeFile(
  "dist/sitemap.xml",
  `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">${Object.values(
    publicPages,
  )
    .map((p) => `<url><loc>${escape(origin + p.path)}</loc></url>`)
    .join("")}</urlset>\n`,
);
await rm(generated, { recursive: true, force: true });
console.log(
  "Prerendered 4 public pages, console shells, sitemap and robots.txt.",
);
