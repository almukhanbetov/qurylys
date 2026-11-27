import { test } from "node:test";
import assert from "node:assert/strict";
import { lineTotal, moneyInput, parseMoney, parseQuantity } from "./quantity.ts";

// Same cases as backend/internal/quantity/quantity_test.go and TestMoneyRuleInSQL.
test("lineTotal rounds half up once and exactly", () => {
  const cases: [number, string, number | null][] = [
    [150000, "12", 1800000], [115000, "12", 1380000], [115000, "60", 6900000], [8437, "500", 4218500],
    [83333, "0.6", 50000], [1, "0.5", 1], [1, "1", 1], [1, "0.499", 0], [3, "0.5", 2],
    [9007199254740991, "1", 9007199254740991], [9007199254740991, "2", null],
  ];
  for (const [price, qty, want] of cases) assert.equal(lineTotal(price, qty), want, `${price} x ${qty}`);
});

test("money input is exact", () => {
  for (const [raw, want] of [["1500", 150000], ["1 150,5", 115050], ["84.37", 8437], ["0,01", 1]] as const) {
    assert.equal(parseMoney(raw), want);
  }
  for (const raw of ["", "0", "-1", "1.234", "1e3", "abc", "1,2,3", "99999999999999"]) assert.equal(parseMoney(raw), null, raw);
  assert.equal(moneyInput(115050), "1150,50");
  assert.equal(moneyInput(150000), "1500");
  assert.equal(moneyInput(1), "0,01");
  assert.equal(moneyInput(null), "");
});

test("quantity input is exact", () => {
  assert.equal(parseQuantity("0,6"), "0.6");
  assert.equal(parseQuantity("1 500.5"), "1500.5");
  for (const raw of ["", "-1", "1.2345", "1e3", ".5", "x"]) assert.equal(parseQuantity(raw), null, raw);
});
