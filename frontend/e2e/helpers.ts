import type { Page } from "@playwright/test";

/** Meets the backend policy: length >= 8, at least one uppercase letter, at least one digit. */
export const VALID_PASSWORD = "ValidPass123";

export function uniqueEmail(prefix = "web"): string {
  return `${prefix}-${crypto.randomUUID()}@example.com`;
}

/**
 * Registers a brand-new account through the real UI and waits for the
 * authenticated dashboard to render. Returns the email used so callers can
 * assert on it if needed. Each call uses a fresh, unique email so tests
 * never collide with data left behind by earlier runs.
 */
export async function registerNewUser(
  page: Page,
  options: { emailPrefix?: string; password?: string } = {}
): Promise<{ email: string; password: string }> {
  const email = uniqueEmail(options.emailPrefix);
  const password = options.password ?? VALID_PASSWORD;

  await page.goto("/register");
  await page.getByLabel("邮箱").fill(email);
  await page.getByLabel("密码").fill(password);
  await page.getByRole("button", { name: "注册" }).click();
  await page.getByRole("heading", { name: "今日训练" }).waitFor();

  return { email, password };
}
