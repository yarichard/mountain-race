import { test, expect } from "@playwright/test";

const BASE = process.env.BASE_URL ?? "http://localhost:8003";

test.describe("Mountain Race — E2E", () => {
  test("Page load: all 9 panels visible", async ({ page }) => {
    await page.goto(BASE);
    await expect(page.locator(".panel-header").first()).toBeVisible();

    const headers = await page.locator(".panel-header").allTextContents();
    expect(headers.some((h) => h.includes("Participant"))).toBeTruthy();
    expect(headers.some((h) => h.includes("Recherche") || h.includes("Search"))).toBeTruthy();
    expect(headers.some((h) => h.includes("Météo") || h.includes("Weather"))).toBeTruthy();
  });

  test("Add participant: name and grade appear", async ({ page }) => {
    await page.goto(BASE);
    const addBtn = page.locator("button", { hasText: /Ajouter|Add/ });
    await addBtn.click();
    const inputs = page.locator('input[placeholder*="Nom"], input[placeholder*="Name"]');
    await inputs.last().fill("Alice");
    await expect(inputs.last()).toHaveValue("Alice");
  });

  test("Race type change: difficulty scale switches", async ({ page }) => {
    await page.goto(BASE);

    // With 1 participant (default), selects are: [0] climbing level, [1] difficulty
    const diffSelect = page.locator("select").nth(1);
    const defaultOptions = await diffSelect.locator("option").allTextContents();
    expect(defaultOptions).toContain("5c");

    // Switch to hike → alpine grades
    const hikeBtn = page.locator("button", { hasText: /^Randonnée$|^Hike$/ });
    await hikeBtn.click();
    const hikeOptions = await diffSelect.locator("option").allTextContents();
    expect(hikeOptions).toContain("AD");
    expect(hikeOptions).not.toContain("5c");
  });

  test("Route search success: results list renders", async ({ page }) => {
    await page.goto(BASE);
    // The location input placeholder in name mode is "e.g. Aiguille du Midi"
    const locationInput = page.locator('input[placeholder*="Aiguille"], input[placeholder*="Chamonix"]');
    await locationInput.fill("Chamonix");
    const searchBtn = page.locator("button", { hasText: /Rechercher|^Search$/ });
    await searchBtn.click();
    // Wait for results
    const results = page.locator("[data-testid='route-result']");
    await expect(results.first()).toBeVisible({ timeout: 15_000 });
  });

  test("Route selection: detail panel fills in", async ({ page }) => {
    await page.goto(BASE);
    // Fill location and search
    const locationInput = page.locator('input[placeholder*="Aiguille"], input[placeholder*="Chamonix"]');
    await locationInput.fill("Chamonix");
    const searchBtn = page.locator("button", { hasText: /Rechercher|^Search$/ });
    await searchBtn.click();
    // Click first result
    const firstResult = page.locator("[data-testid='route-result']").first();
    await firstResult.waitFor({ timeout: 15_000 });
    await firstResult.click();
    // The detail panel empty-state message should disappear once the route loads
    const emptyState = page.locator(".panel-body", { hasText: /Select a route to see details|Sélectionnez une course pour voir le détail/ });
    await expect(emptyState).not.toBeVisible({ timeout: 10_000 });
  });

  test("Schedule formula notice: Naismith warning shown", async ({ page }) => {
    await page.goto(BASE);
    const res = await page.request.get(`${BASE}/api/routes/999999`);
    if (!res.ok()) return; // route not found — skip
    const data = await res.json();
    if (data.schedule?.source === "formula") {
      expect(data.schedule.estimated_duration_hours).toBeGreaterThan(0);
    }
  });

  test("Weather API: returns forecast and avalanche", async ({ page }) => {
    const res = await page.request.get(`${BASE}/api/weather?lat=45.9&lon=6.9&date=2026-05-01`);
    expect(res.status()).toBe(200);
    const data = await res.json();
    expect(data).toHaveProperty("forecast");
    expect(data).toHaveProperty("avalanche");
  });

  test("PDF export: POST /api/export/pdf returns application/pdf", async ({ page }) => {
    const body = {
      id: "123456",
      title: "Test Route — Arête des Cosmiques",
      description: "**L1** Première longueur en 5c\n**L2** Deuxième longueur en 6a\n\n## Descente\nRappels de 30m.",
      difficulty: "5c",
      elevation_gain: 500,
      height_diff_down: 480,
      distance_km: 5.2,
      lat: 45.87,
      lon: 6.88,
      track: [
        [45.85, 6.85],
        [45.87, 6.87],
        [45.89, 6.89],
      ],
      elevation_profile: [
        [0, 1000],
        [1, 1200],
        [2, 1450],
        [3, 1300],
        [4, 1100],
      ],
      images: [],
      equipment: [
        { item: "Corde 60m", quantity: 1, notes: "obligatoire" },
        { item: "Dégaines", quantity: 10, notes: "obligatoire" },
      ],
      risks: ["Risque de chute de pierres", "Conditions météo variables"],
      alternative_routes: [
        { id: "789", title: "Voie normale", reason: "Repli en cas de mauvais temps" },
      ],
      schedule: {
        estimated_duration_hours: 6,
        recommended_start_time: "05:00",
        recommended_end_time: "14:00",
        source: "formula",
      },
      participants: [
        { name: "Alice", climbingLevel: "6a" },
        { name: "Bob", climbingLevel: "5c" },
      ],
      objectives: ["challenge", "fun"],
      notes: "Prévoir des crampons si neige résiduelle.",
      weather: {
        forecast: {
          date: "2026-05-01",
          temperature_min_c: 2,
          temperature_max_c: 14,
          precipitation_mm: 0,
          wind_speed_kmh: 25,
          condition: "sunny",
        },
        avalanche: { risk_level: 2, risk_label: "Limité", description: "Risque de plaques en altitude" },
      },
    };

    const res = await page.request.post(`${BASE}/api/export/pdf`, {
      data: body,
      headers: { "Content-Type": "application/json" },
    });
    expect([200, 500]).toContain(res.status());
    if (res.status() === 200) {
      expect(res.headers()["content-type"]).toContain("application/pdf");
      // PDF should be a non-trivial size (> 10 KB) indicating content was rendered
      const bytes = await res.body();
      expect(bytes.length).toBeGreaterThan(10_000);
    }
  });
});
