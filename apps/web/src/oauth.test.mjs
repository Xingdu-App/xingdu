import assert from "node:assert/strict";
import test from "node:test";
import {
  oauthFeedback,
  oauthAuthorizationURL,
  oauthErrors,
  canUnlinkIdentity,
} from "./oauth.ts";

test("callback only exposes fixed messages and cleans sensitive query parameters", () => {
  const result = oauthFeedback(
    new URL(
      "https://xingdu.example/app/security?oauth_error=account_exists&code=SECRET&state=SECRET&error_description=SECRET&page=security#invite=keep",
    ),
  );
  assert.equal(result.message, oauthErrors.account_exists);
  assert.equal(result.error, true);
  assert.equal(result.cleanPath, "/app/security?page=security#invite=keep");
  assert.equal(result.consumed, true);
  for (const value of [
    "<script>SECRET</script>",
    "__proto__",
    "constructor",
    "",
  ]) {
    const url = new URL("https://xingdu.example/login");
    url.searchParams.set("oauth_error", value);
    const feedback = oauthFeedback(url);
    assert.equal(feedback.message, oauthErrors.unavailable);
    assert.equal(typeof feedback.message, "string");
    assert.equal(feedback.cleanPath, "/login");
  }
});

test("linked success and ordinary routes retain only intended routing state", () => {
  const linked = oauthFeedback(
    new URL("https://xingdu.example/app/security?oauth=linked"),
  );
  assert.equal(linked.message, "登录方式已绑定。");
  assert.equal(linked.error, false);
  assert.equal(linked.cleanPath, "/app/security");
  const ordinary = oauthFeedback(new URL("https://xingdu.example/app/hosts"));
  assert.equal(ordinary.message, "");
  assert.equal(ordinary.consumed, false);
});

test("authorize URL allows only expected HTTPS provider endpoints", () => {
  assert.equal(
    oauthAuthorizationURL(
      "google",
      "https://accounts.google.com/o/oauth2/v2/auth?state=fixture",
    ),
    "https://accounts.google.com/o/oauth2/v2/auth?state=fixture",
  );
  assert.equal(
    oauthAuthorizationURL(
      "github",
      "https://github.com/login/oauth/authorize?state=fixture",
    ),
    "https://github.com/login/oauth/authorize?state=fixture",
  );
  for (const value of [
    "javascript:alert(1)",
    "//evil.example",
    "not a URL",
    "http://accounts.google.com/o/oauth2/v2/auth",
    "https://accounts.google.com.evil.example/o/oauth2/v2/auth",
    "https://accounts.google.com/other",
    "https://user:secret@accounts.google.com/o/oauth2/v2/auth",
    "https://accounts.google.com:444/o/oauth2/v2/auth",
    "https://accounts.google.com/o/oauth2/v2/auth#secret",
    "https://github.com/login/oauth/authorize",
  ]) {
    assert.throws(() => oauthAuthorizationURL("google", value), {
      message: "第三方登录地址无效，请联系部署管理员。",
    });
  }
});

test("unlink requires another currently available method, not merely another identity", () => {
  const both = [{ provider: "google" }, { provider: "github" }];
  assert.equal(
    canUnlinkIdentity("google", false, [{ provider: "google" }], {
      google: true,
      github: false,
    }),
    false,
  );
  assert.equal(
    canUnlinkIdentity("google", false, both, { google: true, github: false }),
    false,
  );
  assert.equal(
    canUnlinkIdentity("google", false, both, { google: true, github: true }),
    true,
  );
  assert.equal(
    canUnlinkIdentity("google", false, both, { google: false, github: true }),
    true,
  );
  assert.equal(
    canUnlinkIdentity("google", true, both, { google: false, github: false }),
    true,
  );
});
