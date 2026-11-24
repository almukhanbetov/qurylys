import { test } from "node:test";
import assert from "node:assert/strict";
import { safeNext } from "./redirect.ts";

test("safeNext keeps local paths and rejects other sites", () => {
  assert.equal(safeNext("/account/company?x=1"), "/account/company?x=1");
  for (const bad of [undefined, "", "https://evil.example", "//evil.example", "/\\evil.example", "/\t/evil.example", "javascript:1", ["/a"]]) {
    assert.equal(safeNext(bad), "/account");
  }
});
