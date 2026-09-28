import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import ts from "typescript";

const english = JSON.parse(
  await readFile(
    new URL("./locales/marketing-en.json", import.meta.url),
    "utf8",
  ),
);
const chinese = /[\u3400-\u9fff]/;
test("public-page copy, blog bodies and metadata have complete English translations", async () => {
  const missing = [];
  for (const name of [
    "Marketing.tsx",
    "Blog.tsx",
    "blog-posts.ts",
    "public-pages.ts",
  ]) {
    const source = await readFile(new URL(name, import.meta.url), "utf8");
    const ast = ts.createSourceFile(
      name,
      source,
      ts.ScriptTarget.Latest,
      true,
      name.endsWith("tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
    );
    function visit(node) {
      if (ts.isJsxText(node) && chinese.test(node.text)) {
        missing.push(`${name}: untranslated JSX ${node.text.trim()}`);
      }
      if (ts.isStringLiteral(node) && chinese.test(node.text)) {
        const value = english[node.text];
        if (typeof value !== "string" || !value.trim() || chinese.test(value)) {
          missing.push(`${name}: ${node.text}`);
        }
      }
      ts.forEachChild(node, visit);
    }
    visit(ast);
  }
  assert.deepEqual(missing, [], "Missing English public-page content");
});
