# ADR-006 — Diagram Editor

**Status:** Accepted for Linux. macOS and Windows are not verified (see "Not verified").
**Date:** 2026-11-06

---

## Context

The Architecture Lab exam ([3.10](../phases/phase-03-architecture-lab/10-diagram-workspace.md)) needs a diagram editor. [Spec 3.9](../phases/phase-03-architecture-lab/09-diagram-editor-spike.md) fixed draw.io through its online embed (`embed.diagrams.net` in an iframe, driven by `postMessage`), the XML as the only submission, the latest XML kept in memory, and a lost connection that does not stop the clock. A throwaway prototype measured the questions of the spec in Chrome and in the Wails webview. The prototype is not in the repository.

## Decision

Use the **online embed**. Keep the **self-hosted bundle** as the fallback. The decision changes to the fallback if the embed fails to load or exchange `postMessage` in the Wails webview of a supported OS (the check that triggers it is the pre-start check below), or if a later draw.io change removes the actions this ADR relies on (`configure`, `load`, `merge`, `export`, `autosave`).

## Measurements

Environments: **Chrome on Linux** (page on `http://localhost` with the CSP `default-src 'self'; frame-src https://embed.diagrams.net; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'`) and the **Wails v2.16 webview on Linux** (`wails dev`, WebKitGTK 2.41 via `-tags webkit2_41`, no CSP set by Wails). Times are one run each; the embed was warm in the HTTP cache unless stated.

| # | Question | Result |
|---|---|---|
| 1 | Loads and runs in the webview | **Linux: yes.** The iframe loads and the `configure` → `init` → `load` handshake completes (`load` at 1.7–2.5 s on a first load, 0.5–0.8 s when the iframe is recreated). Under a restrictive CSP the only addition needed is `frame-src https://embed.diagrams.net`. **macOS and Windows: not verified** (no access to those systems). |
| 2 | Fixed configuration | URL `?embed=1&proto=json&spin=1&configure=1&ui=min&noSaveBtn=1&noExitBtn=1&saveAndExit=0&libraries=1&lang=en`. No Save or Exit buttons. The `libs` URL parameter is **ignored**; the palettes are enabled by the `configure` reply `{compressXml:false, defaultLibraries:'general;aws4;gcp2;azure2;kubernetes;er;flowchart;network'}`, which showed `AWS / ...` (Compute, Containers, Databases, Developer Tools, ...), `Azure / ...` (Compute, Containers, Databases, Integration, ...) and `GCP / ...` (Databases, Networking, Storage, Data Transfer, ...) categories. **Search works** (typing `lambda` returned AWS icons), but it queries draw.io's online index, so it needs the network. A dedicated "messaging" palette was not checked by name; messaging services appear inside the AWS, Azure and GCP categories (Integration, Application Integration). |
| 2b | Read-only view (for [3.13](../phases/phase-03-architecture-lab/13-evaluation-and-history-screen.md)) | Chrome only. Adding `chrome=0` to the URL renders the diagram with no menus, sidebars or toolbars, and it cannot be edited: with the typical example loaded, selecting a component and pressing Delete, and dragging another, left the export at 24 cells with no `autosave` event. The same actions without `chrome=0` deleted the component (23 cells, one `autosave`). The Wails webview was not run in this mode. |
| 3 | Events and latency | An edit made by hand (a shape inserted from the palette) produced an `autosave` event with the full XML within the 1.7 s my measurement could resolve; a programmatic `merge` produced it in 67–88 ms in the webview. The **pull** `{action:'export', format:'xml', xml:1}` answers with an `export` event carrying the full `xml` in **10–13 ms**. So an exam that ends at the deadline does not depend on the last `autosave` having arrived: the screen asks for the XML at submit time (the exact edit delay of `autosave` for a hand edit was not measured below 1.7 s). |
| 4 | Detecting loss and recovery | The embed has no heartbeat event, but the `export` pull is a probe: an editor that is gone does not answer (a removed iframe never answered within 1.2 s). **Recovery works with the diagram intact**: a new iframe, the same handshake, `load` with the XML kept in memory, then `export` returned the same number of cells (5 of 5 in the webview, 3 of 3 in Chrome). Once loaded, the editor runs client side: only search and lazily loaded resources need the network, so a dropped connection after load does not stop drawing. A real network drop in the middle of a session was **not** reproduced; the removed-iframe simulation stands in for "the editor stopped responding". |
| 5 | XML size | Three diagrams emitted by the editor (see below): 1,485 bytes (7 cells), 7,821 bytes (24 cells, AWS icons, a VPC container, HTML labels), 49,565 bytes (231 cells, 12 grouped domains). About 215 bytes per cell, so the 2,000-cell cap of 3.11 is about 430 KB, far under the 2 MiB limit. The large example is synthetic in content, but its XML comes from the editor. |
| 6 | The XML is enough for the evaluator | Yes. A Python extraction from the XML alone (no styles, no coordinates) listed every component by label, every edge with source label, target label and edge label, and the grouping (`parent` ids; 6 services inside the VPC container in the typical example). No edge was dangling and no component was unlabeled. HTML labels are kept as HTML (`CloudFront<br><b>CDN</b>`), so 3.11 must convert them to text. A component with an icon (`resIcon` in `style`) but no label would be invisible to a summary that ignores styles; the prompt of 3.11 should tell the user to label components, and the summary may keep the shape name as a hint. |
| 7 | What the embed sends | **Partially verified.** From the page, the only host seen was `embed.diagrams.net`, and with the cache cold and no network the editor did not load at all, so loading it needs the internet. Requests made inside the iframe (fonts, search index, analytics) are not visible to the page and were not captured, so whether diagram content leaves the app is **not verified**. The embed does not receive the diagram except through the `load` and `merge` messages the app sends; whether draw.io itself transmits it was not inspected. |
| 8 | Fallback cost | **Not measured by us.** From a web search only (not checked against the source): jgraph publishes the `jgraph/drawio` Docker image, built with `ant war` and served by Tomcat, and one third-party guide reports about 41 MB compressed; `?offline=1` disables the cloud storage integrations; updating means taking a newer release. Self-hosting would mean shipping the draw.io web app with Athena and updating it with each release. No spec covers building it. |

### Examples

Real outputs of the editor, used by tests in 3.10 and 3.11 (all within the 2 MiB limit, uncompressed because of `compressXml:false`):

- [`ADR-006-examples/small.drawio.xml`](ADR-006-examples/small.drawio.xml): 3 components, 2 edges.
- [`ADR-006-examples/typical.drawio.xml`](ADR-006-examples/typical.drawio.xml): 11 components, 11 labeled edges, a VPC container with 6 children, HTML labels.
- [`ADR-006-examples/large.drawio.xml`](ADR-006-examples/large.drawio.xml): 121 components, 108 edges, 12 containers.

## Integration rules for 3.10

**Configuration.** URL as in question 2; reply to `configure` with `{action:'configure', config:{compressXml:false, defaultLibraries:'general;aws4;gcp2;azure2;kubernetes;er;flowchart;network'}}`.

**Messages.** Accept a message only when `event.origin === 'https://embed.diagrams.net'` and `event.source` is the iframe's window; send with that exact target origin.

| From the editor | The screen does |
|---|---|
| `configure` | Send the configuration. |
| `init` | Send `{action:'load', xml:<empty mxGraphModel>, autosave:1}`. |
| `load` | The editor is ready: call `onReady`. |
| `autosave` | Keep `xml` in memory as the latest diagram and call `onChange(xml)`. |
| `export` | Resolve the pending `getXml()` with `xml`. |

**Pre-start check (3.9 decision 6).** Mount the iframe and wait for `load`. There is no iframe error event when the network is down, so the only failure signal is the absence of `load`: treat no `load` within 15 s as failure (warm loads took under 2.5 s). Recommended value; the offline run waited 30 s and received nothing. Note that a cached embed can load while the machine is offline, in which case the check passes and only search and new resources fail; this is acceptable.

**Reading the XML at submit and at the deadline.** `getXml()` sends the `export` pull and waits up to 1.5 s for the answer. If it answers, submit that XML; if not, submit the latest `autosave` XML held in memory. The wrapper therefore exposes `getXml()` in addition to the callbacks of 3.10.

**Read-only view (3.13).** Same handshake with `chrome=0` added to the URL, `load` with the stored XML and no `autosave`.

**Connection loss (3.9 decision 5).** While the exam runs, probe with the `export` pull every 5 s with a 1.5 s timeout. After two missed answers call `onConnectionLost`. Recover by recreating the iframe, repeating the handshake with `load` carrying the latest XML in memory, and calling `onConnectionRestored` when the `load` ack arrives. The clock is never touched.

## Not verified

1. macOS and Windows webviews (WebKit and WebView2). The check of the decision is the same on both; run the pre-start check there before the first release for those systems.
2. A real network drop in the middle of a session (only a removed iframe was simulated).
3. What the iframe requests besides the editor itself, and whether diagram content leaves the app.
4. The delay of `autosave` after a hand edit below 1.7 s. Not needed by the integration, because submitting uses the `export` pull.
5. Fallback size and update process, beyond the web-search figures in question 8.
6. A palette called "messaging" by name; the cloud palettes cover it.
7. The read-only view (`chrome=0`) in the Wails webview and without network.
