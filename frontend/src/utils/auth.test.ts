import { describe, expect, it } from "vitest";
import { buildLoginHref } from "./auth";

describe("buildLoginHref", () => {
  it("builds a same-origin login URL and preserves the source page", () => {
    const href = buildLoginHref("https://brave.example.com/list?view=all#top");
    const loginUrl = new URL(href);

    expect(loginUrl.origin).toBe("https://brave.example.com");
    expect(loginUrl.pathname).toBe("/login");
    expect(loginUrl.searchParams.get("src")).toBe(
      "https://brave.example.com/list?view=all#top",
    );
    expect(href).not.toContain(":2727");
  });
});
