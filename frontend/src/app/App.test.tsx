import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";

describe("App", () => {
  beforeEach(() => {
    vi.stubGlobal(
      "fetch",
      vi.fn(
        async () =>
          new Response(
            JSON.stringify({ tokens: { access_token: "test-token", access_expires_in: 900, refresh_expires_in: 3600 } }),
            { status: 200 }
          )
      )
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("renders the fitness application identity", async () => {
    render(<App />);
    expect(screen.getByRole("navigation", { name: "主导航" })).toBeInTheDocument();
    expect(await screen.findByRole("heading", { name: "今日训练" })).toBeInTheDocument();
  });
});
