const form = document.querySelector("#query-form");
const station = document.querySelector("#station");
const status = document.querySelector("#status");
const results = document.querySelector("#observations");
let releaseItems = [];
let activeDocument = true;
let queryState = "ready";
let requestNumber = 0;
let activeRequest;

/** Validate a complete default page and retain only the fields this view displays. */
function validateReleasePage(payload) {
  if (!Array.isArray(payload) || payload.length > 30)
    throw new Error("Invalid release page");
  const seen = new Set();
  return payload.map((item) => {
    if (
      !item ||
      typeof item !== "object" ||
      Array.isArray(item) ||
      typeof item.tag_name !== "string" ||
      item.tag_name.length > 128 ||
      !/^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$/.test(
        item.tag_name,
      ) ||
      seen.has(item.tag_name) ||
      item.draft !== false ||
      typeof item.prerelease !== "boolean" ||
      typeof item.published_at !== "string" ||
      !/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$/.test(item.published_at) ||
      !Number.isFinite(Date.parse(item.published_at)) ||
      new Date(item.published_at).toISOString().slice(0, 19) + "Z" !==
        item.published_at
    )
      throw new Error("Invalid release metadata");
    seen.add(item.tag_name);
    return {
      version: item.tag_name,
      publishedAt: item.published_at,
      prerelease: item.prerelease,
    };
  });
}

/** Decode bounded streamed UTF-8; a bad page never produces partial rows. */
async function readReleasePage(response) {
  const type = (response.headers.get("content-type") || "")
    .split(";")[0]
    .trim()
    .toLowerCase();
  if (
    response.status !== 200 ||
    response.redirected ||
    !response.body ||
    !["application/json", "application/vnd.github+json"].includes(type)
  )
    throw new Error("Unavailable release response");
  const reader = response.body.getReader();
  const decoder = new TextDecoder("utf-8", { fatal: true });
  let bytes = 0;
  let text = "";
  try {
    for (;;) {
      const part = await reader.read();
      if (part.done) break;
      bytes += part.value.byteLength;
      if (bytes > 1024 * 1024) throw new Error("Release page too large");
      text += decoder.decode(part.value, { stream: true });
    }
    return validateReleasePage(JSON.parse(text + decoder.decode()));
  } finally {
    void reader.cancel().catch(() => {});
    reader.releaseLock();
  }
}

/** Filters reuse the loaded page; displayed metadata is always plain text. */
function renderReleases() {
  const search = document.querySelector("#release-search").value.toLowerCase();
  const items = releaseItems.filter(
    (item) =>
      item.version.toLowerCase().includes(search) &&
      (station.value === "" ||
        (station.value === "prerelease") === item.prerelease),
  );
  const table = document.createElement("table");
  table.setAttribute("role", "table");
  table.createCaption().textContent =
    "Published KSail versions. Times are in UTC.";
  const headings = ["Version", "Published at", "Channel"];
  const header = table.createTHead().insertRow();
  for (const label of headings) {
    const cell = document.createElement("th");
    cell.scope = "col";
    cell.textContent = label;
    header.append(cell);
  }
  const body = table.createTBody();
  for (const item of items) {
    const row = body.insertRow();
    row.className = "release";
    const version = document.createElement("th");
    version.scope = "row";
    version.textContent = item.version;
    row.append(version);
    const time = document.createElement("time");
    time.dateTime = item.publishedAt;
    time.textContent = item.publishedAt.slice(0, 16).replace("T", " ");
    for (const [index, value] of [
      time,
      item.prerelease ? "Pre-release" : "Stable",
    ].entries()) {
      const cell = row.insertCell();
      cell.dataset.label = headings[index + 1];
      cell.append(value);
    }
  }
  results.replaceChildren(table);
  status.textContent = `${items.length} of ${releaseItems.length} releases`;
  if (!releaseItems.length)
    status.textContent =
      "GitHub returned no published releases. Use Refresh releases to check again.";
  else if (!items.length)
    status.textContent =
      "No releases match. Clear the version filter or choose another channel.";
}

/** Refresh and navigation revoke earlier reads before they can update the view. */
async function queryReleases() {
  const current = ++requestNumber;
  activeRequest?.abort();
  const controller = new AbortController();
  activeRequest = controller;
  releaseItems = [];
  results.replaceChildren();
  results.setAttribute("aria-busy", "true");
  status.textContent = "Loading releases…";
  queryState = "loading";
  try {
    const response = await fetch(
      "https://api.github.com/repos/devantler-tech/ksail/releases",
      {
        credentials: "omit",
        cache: "no-store",
        redirect: "error",
        referrerPolicy: "no-referrer",
        signal: AbortSignal.any([
          controller.signal,
          AbortSignal.timeout(10000),
        ]),
      },
    );
    const items = await readReleasePage(response);
    if (current !== requestNumber || !activeDocument) return;
    releaseItems = items;
    renderReleases();
    queryState = "ready";
  } catch {
    if (current !== requestNumber || !activeDocument) return;
    releaseItems = [];
    results.replaceChildren();
    status.textContent =
      "Could not read GitHub releases. Use Refresh releases to retry.";
    queryState = "error";
  } finally {
    if (current === requestNumber) results.setAttribute("aria-busy", "false");
  }
}


const filterLoadedPage = () => {
 if (queryState === "ready") renderReleases();
};
document.querySelector("#release-search").addEventListener("input", filterLoadedPage);
station.addEventListener("change", filterLoadedPage);
window.addEventListener("pagehide", () => {
 activeDocument = false;
 ++requestNumber;
 activeRequest?.abort();
 releaseItems = [];
 results.replaceChildren();
});
window.addEventListener("pageshow", (event) => {
 if (!event.persisted) return;
 activeDocument = true;
 queryReleases();
});
form.addEventListener("submit", (event) => {
 event.preventDefault();
 queryReleases();
});
queryReleases();
