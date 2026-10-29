/*
 * Browser E2E for memory-settings account isolation. Run from repository root:
 * pnpm --dir web dev --host 127.0.0.1 --port 5178 --strictPort
 * PWCLI="$HOME/.codex/skills/playwright/scripts/playwright_cli.sh"
 * mkdir -p output/playwright/pr131-memory
 * "$PWCLI" -s=ink-memory-lifecycle open http://127.0.0.1:5178/login --headed
 * "$PWCLI" -s=ink-memory-lifecycle run-code --filename web/scripts/memory-lifecycle-e2e.js --raw > output/playwright/pr131-memory/results.json
 * "$PWCLI" -s=ink-memory-lifecycle close
 * All API calls use browser route fixtures. No API, database, or real credentials.
 * Failure scenarios are declared before execution and included in the artifact.
 */
// Playwright CLI evaluates this standalone function; its helpers must remain self-contained.
/* eslint-disable unicorn/consistent-function-scoping -- CLI evaluates one self-contained function. */
// eslint-disable-next-line no-unused-vars -- Invoked by Playwright CLI --filename.
async function memoryLifecycleE2E(page) {
  await page.setViewportSize({ width: 1440, height: 1000 });
  const base = new URL(page.url()).origin;
  const failureScenarios = [
    "Pending old save keeps Save disabled after logout/login",
    "Old success overwrites newly loaded settings",
    "Old failure leaks its error into new account",
    "Old settings or errors survive when new settings GET fails",
    "Old completion clears a fresh account pending save",
  ];
  const results = [];
  const unexpected = [];
  const requests = [];
  const holds = [];
  let holdNext = false;
  let failNextSave = false;
  let failSettingsRead = false;
  let currentUser;
  let sequence = 0;
  const memory = (name) => ({
    enabled: true,
    zepBaseUrl: `https://${name}.example.invalid`,
    keyConfigured: true,
    available: true,
  });
  let settings = memory("initial");
  const devices = [
    {
      id: "fixture-printer",
      name: "Fixture printer",
      status: "connected",
      note: "No physical printer",
    },
  ];
  const workspace = {
    revision: 1,
    devices,
    conversations: [],
    activeConversationId: "",
    printJobs: [],
    schedules: [],
    sources: [],
    preferences: {
      loginProtectionEnabled: false,
      sendConfirmationEnabled: false,
      tutorialTabEnabled: false,
      theme: "light",
      defaultDeviceId: devices[0].id,
      locale: "en-US",
    },
    serviceBinding: { providerName: null, modelName: "Ink AI", bound: false },
  };
  const check = (ok, message) => {
    if (!ok) throw Error(message);
  };
  const routeHandler = async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    const method = request.method();
    requests.push({ method, path });
    const reply = (body, status = 200) =>
      route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });
    if (path === "/api/v1/auth/login") {
      const { email } = request.postDataJSON();
      currentUser = { id: email, email: `${email}@example.invalid`, name: email, role: "admin" };
      sequence++;
      return reply({
        user: currentUser,
        accessToken: `fixture-access-${sequence}`,
        refreshToken: `fixture-refresh-${sequence}`,
        expiresIn: 3600,
      });
    }
    if (path === "/api/v1/auth/me") return reply({ user: currentUser });
    if (path === "/api/v1/auth/logout") return route.fulfill({ status: 204 });
    if (path === "/api/v1/workspace")
      return reply(method === "PUT" ? request.postDataJSON() : workspace);
    if (path === "/api/v1/ai/config")
      return reply({
        bound: false,
        providerName: "",
        providerType: "openai",
        baseUrl: "",
        model: "Ink AI",
        keyConfigured: false,
      });
    if (path === "/api/v1/printers") return reply({ devices });
    if (path === "/api/v1/print-jobs") return reply({ printJobs: [], nextCursor: null });
    if (path === "/api/v1/print-jobs/status")
      return reply({
        printJobs: [],
        counts: { pending: 0, queued: 0, completed: 0, failed: 0, cancelled: 0, todayCompleted: 0 },
        latestJobId: null,
      });
    if (path === "/api/v1/memory/settings")
      return failSettingsRead
        ? reply({ code: "fixture_read_failure", message: "Fixture unavailable" }, 503)
        : reply(settings);
    if (path === "/api/v1/admin/memory/settings" && method === "PUT") {
      if (holdNext) {
        holdNext = false;
        await new Promise((resolve) =>
          holds.push({
            release: async (body, status = 200) => {
              await reply(body, status);
              resolve();
            },
          }),
        );
        return;
      }
      if (failNextSave) {
        failNextSave = false;
        return reply({ code: "fixture_save_failure", message: "Old account save failed" }, 400);
      }
      settings = { ...settings, ...request.postDataJSON() };
      return reply(settings);
    }
    unexpected.push({ method, path });
    return reply(
      { code: "unexpected_fixture_request", message: `Unexpected ${method} ${path}` },
      500,
    );
  };
  await page.unroute("**/api/**");
  await page.route("**/api/**", routeHandler);
  await page.evaluate(() => {
    localStorage.clear();
    sessionStorage.clear();
  });
  await page.goto(`${base}/login`);
  const login = async (name) => {
    await page.locator("#email").fill(name);
    await page.locator("#password").fill("fixture-password-only");
    await page.locator('button[type="submit"]').click();
    await page.locator('a[href="/settings"]').first().click();
    await page.getByRole("button", { name: "Configure memory", exact: true }).waitFor();
  };
  const open = () => page.getByRole("button", { name: "Configure memory", exact: true }).click();
  const dialog = () => page.getByRole("dialog", { name: "Memory system config" });
  const save = () => dialog().getByRole("button", { name: "Save config", exact: true });
  const cancel = () => dialog().getByRole("button", { name: "Cancel", exact: true }).click();
  const logout = async () => {
    await page.getByRole("button", { name: "Log out", exact: true }).first().click();
    await page.locator('a[href^="/login"]:visible').first().click();
    await page.locator("#email").waitFor();
  };
  const heldSave = async () => {
    const index = holds.length;
    holdNext = true;
    await save().click();
    await dialog().getByRole("button", { name: "Saving...", exact: true }).waitFor();
    for (let i = 0; i < 100 && holds.length === index; i++) await page.waitForTimeout(20);
    check(holds.length > index, "save request did not reach fixture");
    return holds[index];
  };
  const artifact = async (name) => {
    const path = `output/playwright/pr131-memory/${name}.png`;
    await page.screenshot({ path, fullPage: true, animations: "disabled" });
    return path;
  };
  try {
    await login("admin-a");
    await open();
    const oldSuccess = await heldSave();
    await cancel();
    await logout();
    settings = memory("fresh-b");
    await login("admin-b");
    await open();
    check(await save().isEnabled(), "Save remains disabled after login while old save is pending");
    check(
      (await dialog().locator('input[type="url"]').inputValue()) === settings.zepBaseUrl,
      "fresh settings were not loaded",
    );
    results.push({
      scenario: failureScenarios[0],
      passed: true,
      screenshot: await artifact("relogin-save-enabled"),
    });
    const freshPending = await heldSave();
    await oldSuccess.release(memory("stale-a"));
    await page.waitForTimeout(150);
    check(
      await dialog().getByRole("button", { name: "Saving...", exact: true }).isDisabled(),
      "old completion unlocked fresh pending save",
    );
    check(
      (await dialog().locator('input[type="url"]').inputValue()) === settings.zepBaseUrl,
      "old response overwrote fresh settings",
    );
    results.push({ scenario: failureScenarios[1], passed: true });
    results.push({
      scenario: failureScenarios[4],
      passed: true,
      screenshot: await artifact("fresh-save-still-pending"),
    });
    await freshPending.release(memory("fresh-b-saved"));
    await dialog().waitFor({ state: "hidden" });
    await open();
    const oldFailure = await heldSave();
    await cancel();
    await logout();
    settings = memory("fresh-c");
    await login("admin-c");
    await open();
    await oldFailure.release(
      { code: "fixture_old_failure", message: "STALE OLD ACCOUNT FAILURE" },
      400,
    );
    await page.waitForTimeout(150);
    check(await save().isEnabled(), "Save disabled after stale failure");
    check(
      !(await page.locator("body").innerText()).includes("STALE OLD ACCOUNT FAILURE"),
      "stale save error leaked",
    );
    results.push({
      scenario: failureScenarios[2],
      passed: true,
      screenshot: await artifact("stale-error-ignored"),
    });
    failNextSave = true;
    await save().click();
    await dialog().getByText("Old account save failed", { exact: true }).waitFor();
    await cancel();
    await logout();
    failSettingsRead = true;
    await login("admin-d");
    check(
      !(await page.locator("body").innerText()).includes("Old account save failed"),
      "old error remains after login",
    );
    await open();
    check(await save().isEnabled(), "Save disabled with unavailable settings GET");
    check(
      (await dialog().locator("button[aria-pressed]").getAttribute("aria-pressed")) === "false",
      "old enabled setting leaked",
    );
    await dialog().locator("button[aria-pressed]").click();
    check((await dialog().locator('input[type="url"]').inputValue()) === "", "old URL leaked");
    check(
      (await dialog().locator('input[type="password"]').getAttribute("placeholder")) ===
        "Enter Zep API Key",
      "old keyConfigured flag leaked",
    );
    results.push({
      scenario: failureScenarios[3],
      passed: true,
      screenshot: await artifact("failed-load-clean-settings"),
    });
    check(unexpected.length === 0, JSON.stringify(unexpected));
    await cancel();
    await logout();
    return {
      passed: true,
      mode: "real browser + Vue app + intercepted API fixtures",
      failureScenarios,
      results,
      requests,
      unexpected,
      cleanedUp: "Logged out; fixtures only, no development API/database used",
    };
  } catch (error) {
    const screenshot = await artifact("failure");
    return {
      passed: false,
      error: String(error),
      screenshot,
      failureScenarios,
      results,
      requests,
      unexpected,
    };
  }
}
