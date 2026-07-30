import type { Locator, Page } from "@playwright/test";
import { BasePage } from "../base-page";

export class HomePage extends BasePage {
  readonly heading: Locator;
  readonly description: Locator;

  constructor(page: Page) {
    super(page);
    this.heading = page.getByRole("heading", { level: 1, name: "ConContexto" });
    this.description = page.getByText("Portal de Datos Económicos de España");
  }

  async goto(): Promise<void> {
    await super.goto("/");
  }
}
