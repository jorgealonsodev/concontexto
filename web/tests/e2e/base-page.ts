import type { Page } from "@playwright/test";

/** Parent class for all page objects (playwright skill convention). */
export class BasePage {
  constructor(protected page: Page) {}

  async goto(path: string): Promise<void> {
    await this.page.goto(path);
  }
}
