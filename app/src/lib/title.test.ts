import { describe, expect, it } from "vitest";
import { splitTitle } from "./title";

describe("splitTitle", () => {
  it("splits product and project", () => {
    expect(splitTitle("Claude Code — graphql-testing-framework")).toEqual({
      product: "Claude Code",
      project: "graphql-testing-framework",
    });
  });
  it("falls back to Claude Code when there is no separator", () => {
    expect(splitTitle("weird")).toEqual({ product: "Claude Code", project: "weird" });
  });
  it("splits on the first separator only", () => {
    expect(splitTitle("Claude Code — a — b")).toEqual({ product: "Claude Code", project: "a — b" });
  });
});
