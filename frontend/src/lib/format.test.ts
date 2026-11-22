// Run with: npm test (Node's built-in test runner, no extra dependencies).
import { test } from "node:test";
import assert from "node:assert/strict";
import { formatMoney, formatQuantity } from "./format.ts";

test("formatMoney renders whole tenge without decimals", () => {
  assert.equal(formatMoney({ amount_tiyn: 275000, currency: "KZT" }), "2 750 ₸");
  assert.equal(formatMoney({ amount_tiyn: 33900000, currency: "KZT" }), "339 000 ₸");
  assert.equal(formatMoney({ amount_tiyn: 7500, currency: "KZT" }), "75 ₸");
});

test("formatMoney keeps tiyn exactly", () => {
  assert.equal(formatMoney({ amount_tiyn: 275050, currency: "KZT" }), "2 750,50 ₸");
  assert.equal(formatMoney({ amount_tiyn: 1, currency: "KZT" }), "0,01 ₸");
  // 0.1 + 0.2 style float errors must not appear.
  assert.equal(formatMoney({ amount_tiyn: 30, currency: "KZT" }), "0,30 ₸");
});

test("formatQuantity trims zeros and groups thousands", () => {
  assert.equal(formatQuantity("12.750"), "12,75");
  assert.equal(formatQuantity("20000"), "20 000");
  assert.equal(formatQuantity("0.5"), "0,5");
  assert.equal(formatQuantity("40.000"), "40");
});
