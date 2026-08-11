import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { App } from "./App";

describe("App", () => {
  it("renders the fitness application identity", () => {
    render(<App />);
    expect(screen.getByRole("heading", { name: "今日训练" })).toBeInTheDocument();
    expect(screen.getByRole("navigation", { name: "主导航" })).toBeInTheDocument();
  });
});
