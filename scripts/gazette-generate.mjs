#!/usr/bin/env node
/**
 * Script de benchmark & génération Gazette pour fX v2 (et comparaison A/B legacy).
 *
 * Usage:
 *   node scripts/gazette-generate.mjs [options]
 *
 * Options CLI:
 *   --mode=v2|legacy       Mode de rendu (défaut: v2 -> POST /api/v1/render,
 *                          legacy -> POST /api/v1/custom/generate)
 *   --parallel=N           Nombre de requêtes parallèles (défaut: 50)
 *   --format=pdf|svg|png   Format de sortie en mode v2 (défaut: pdf)
 *   --out=<chemin>         Fichier de sortie (défaut: tmp/gazette.<format>)
 *   --url=<url>            URL de base de l'API (défaut: FX_API_URL ou http://localhost:8080)
 *   --help, -h             Affiche l'aide
 *
 * Variables d'env (optionnelles):
 *   FX_API_URL    – URL de base (défaut: http://localhost:8080)
 *   FX_API_KEY    – clé API (défaut: api-key)
 *   FX_LOGO_PATH  – chemin local optionnel du logo (défaut: images/logo.png)
 *   FX_OUT        – fichier de sortie (alias: FX_OUT_PATH)
 *   FX_PARALLEL   – parallélisme par défaut (défaut: 50)
 */

import { mkdir, readFile, readdir, writeFile } from "node:fs/promises";
import { dirname, extname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT = resolve(__dirname, "..");

/**
 * Fallback inline du preset `editorial-gazette` (identique à pkg/blueprint/presets.go)
 * utilisé si GET /api/v1/studio/presets est indisponible ou désactivé.
 */
const INLINE_EDITORIAL_GAZETTE_BLUEPRINT = {
  version: "2.0",
  meta: {
    title: "La Gazette Éditoriale",
    description:
      "Gazette interne élégante avec typographie éditoriale, articles illustrés et galeries Polaroid.",
    category: "newsletter",
    locale: "fr-FR",
    modules: ["editorial"],
  },
  theme: {
    preset: "editorial-gazette",
  },
  page: {
    paper: "a4",
    flipped: false,
    margin: {
      top: "1.5cm",
      bottom: "1.6cm",
      left: "2cm",
      right: "2cm",
    },
  },
  footer: {
    left: "{{header.nom_entreprise}}",
    center: "{{header.titre_gazette}} — {{header.date_publication}}",
    showPageNumbers: true,
  },
  blocks: [
    {
      id: "gaz-header",
      type: "header",
      props: {
        eyebrow: "{{header.nom_entreprise}}",
        title: "{{header.titre_gazette}}",
        subtitle: "{{header.date_publication}}",
        alignMode: "center",
      },
    },
    {
      id: "gaz-intro",
      type: "text",
      visibleIf: "header.message_intro",
      props: {
        content: "« {{header.message_intro}} »",
        italic: true,
        muted: true,
        align: "center",
        size: "11pt",
      },
    },
    {
      id: "gaz-intro-rule",
      type: "divider",
      props: {
        style: "accent",
      },
    },
    {
      id: "gaz-articles",
      type: "repeat",
      props: {
        path: "articles",
        divider: true,
      },
      children: [
        {
          id: "gaz-article-title",
          type: "heading",
          props: {
            text: "{{titre_article}}",
            level: 1,
          },
        },
        {
          id: "gaz-article-date",
          type: "text",
          visibleIf: "date_evenement",
          props: {
            content: "{{date_evenement}}",
            italic: true,
            muted: true,
            size: "9pt",
          },
        },
        {
          id: "gaz-article-body",
          type: "text",
          props: {
            content: "{{texte_descriptif}}",
            align: "justify",
            size: "10.5pt",
          },
        },
        {
          id: "gaz-article-gallery",
          type: "gallery",
          visibleIf: "images",
          props: {
            imagesPath: "images",
            srcField: "src",
            captionField: "alt_text",
            style: "{{style_image}}",
            columns: 3,
          },
        },
      ],
    },
  ],
};

function printHelp() {
  console.log(`Usage: node scripts/gazette-generate.mjs [options]

Options:
  --mode=v2|legacy       Mode de rendu (défaut: v2)
                           v2     -> POST /api/v1/render (Blueprint + extraction binaire RAM)
                           legacy -> POST /api/v1/custom/generate (Handlebars + {{bytes src}})
  --parallel=N           Nombre d'appels en parallèle (défaut: ${process.env.FX_PARALLEL ?? 50})
  --format=pdf|svg|png   Format de sortie en mode v2 (défaut: pdf)
  --out=<path>           Chemin de sortie (défaut: tmp/gazette.<format>)
  --url=<url>            URL API fX (défaut: FX_API_URL ou http://localhost:8080)
  --help, -h             Affiche cette aide
`);
}

function parseCliArgs(argv) {
  const opts = {
    mode: "v2",
    parallel: Number(process.env.FX_PARALLEL ?? 50),
    format: "pdf",
    outPath: process.env.FX_OUT ?? process.env.FX_OUT_PATH ?? "",
    apiUrl: process.env.FX_API_URL ?? "http://localhost:8080",
    apiKey: process.env.FX_API_KEY ?? "api-key",
    help: false,
  };

  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (arg === "--help" || arg === "-h") {
      opts.help = true;
      continue;
    }

    const eqIdx = arg.indexOf("=");
    let key = arg;
    let val = undefined;
    if (eqIdx !== -1) {
      key = arg.slice(0, eqIdx);
      val = arg.slice(eqIdx + 1);
    } else if (i + 1 < argv.length && !argv[i + 1].startsWith("--")) {
      val = argv[++i];
    }

    switch (key) {
      case "--mode":
        opts.mode = String(val ?? "").toLowerCase();
        break;
      case "--parallel":
        opts.parallel = Number(val);
        break;
      case "--format":
        opts.format = String(val ?? "").toLowerCase();
        break;
      case "--out":
        opts.outPath = String(val ?? "");
        break;
      case "--url":
        opts.apiUrl = String(val ?? "");
        break;
      default:
        throw new Error(`Option inconnue: ${arg} (utilisez --help)`);
    }
  }

  if (opts.help) {
    return opts;
  }

  if (!["v2", "legacy"].includes(opts.mode)) {
    throw new Error(
      `--mode invalide: "${opts.mode}" (valeurs autorisées: v2, legacy)`,
    );
  }
  if (!Number.isInteger(opts.parallel) || opts.parallel <= 0) {
    throw new Error(
      `--parallel invalide: "${opts.parallel}" (entier positif requis)`,
    );
  }
  if (!["pdf", "svg", "png"].includes(opts.format)) {
    throw new Error(
      `--format invalide: "${opts.format}" (valeurs autorisées: pdf, svg, png)`,
    );
  }
  if (opts.mode === "legacy" && opts.format !== "pdf") {
    console.warn(
      `⚠ Le mode legacy (/api/v1/custom/generate) ne génère que du PDF ; format forcé à "pdf".`,
    );
    opts.format = "pdf";
  }

  opts.apiUrl = opts.apiUrl.replace(/\/+$/, "");
  if (!opts.outPath) {
    opts.outPath = `tmp/gazette.${opts.format}`;
  }

  return opts;
}

function percentile(sortedNums, p) {
  if (sortedNums.length === 0) return 0;
  const rank = Math.ceil(p * sortedNums.length) - 1;
  const idx = Math.min(Math.max(rank, 0), sortedNums.length - 1);
  return sortedNums[idx];
}

async function loadGazetteData() {
  const imageFiles = await readdir(resolve(ROOT, "images"));

  // Optionnellement charge le logo s'il est présent (rétro-compat FX_LOGO_PATH).
  let logoB64 = "";
  try {
    const logo = await readFile(
      resolve(ROOT, process.env.FX_LOGO_PATH ?? "images/logo.png"),
    );
    logoB64 = logo.toString("base64");
  } catch {
    // Logo optionnel
  }

  // Récupère les images WebP (ou fallback sur toute image hors logo.png), triées par nom.
  let selected = imageFiles
    .filter((f) => /\.(png|jpe?g|gif|webp)$/i.test(f) && /\.webp$/i.test(f))
    .sort();
  if (selected.length === 0) {
    selected = imageFiles
      .filter((f) => /\.(png|jpe?g|gif|webp)$/i.test(f) && f !== "logo.png")
      .sort();
  }
  if (selected.length === 0) {
    throw new Error("Aucune image trouvée dans le dossier images/");
  }

  const imagePaths = selected.map((f) => resolve(ROOT, "images", f));
  const imageBuffers = await Promise.all(imagePaths.map((p) => readFile(p)));
  const imagesB64 = imageBuffers.map((buf, i) => ({
    url: selected[i],
    src: buf.toString("base64"),
    alt_text: `Image ${i + 1}`,
  }));

  // Répartition: article 1 -> 3 images, article 2 -> 5 images, article 3 -> 7 images (15 images total)
  const distribution = [3, 5, 7];
  const totalNeeded = distribution.reduce((a, b) => a + b, 0);
  const pool = [];
  for (let i = 0; i < totalNeeded; i++) {
    pool.push(imagesB64[i % imagesB64.length]);
  }

  const descriptions = [
    "Un grand rassemblement autour de l'innovation, avec ateliers, démonstrations et rencontres de l'équipe. Les participants ont pu découvrir nos dernières avancées en matière d'énergie solaire et échanger avec nos experts sur les défis techniques de demain.",
    "Retour en images sur une journée exceptionnelle dédiée à la transition énergétique. Conférences inspirantes, moments de partage et démonstrations grand format ont rythmé cet événement mémorable, réunissant clients, partenaires et curieux autour d'une même vision.",
    "Plongée immersive au cœur de nos projets phares : des installations solaires en milieu urbain aux expérimentations en site isolé, découvrez comment SolarPush repousse les limites du possible pour bâtir un avenir énergétique plus durable et accessible à tous.",
  ];

  let cursor = 0;
  const articles = distribution.map((count, idx) => {
    const imgs = pool.slice(cursor, cursor + count);
    cursor += count;
    return {
      titre_article: `Article ${idx + 1}`,
      date_evenement: "juillet 2026",
      texte_descriptif: descriptions[idx % descriptions.length],
      style_image: "polaroid",
      images: imgs,
    };
  });

  return {
    logoB64,
    imageCount: totalNeeded,
    data: {
      header: {
        titre_gazette: "La Gazette",
        nom_entreprise: "SolarPush",
        date_publication: "JUILLET 2026",
        message_intro: "Bienvenue dans cette nouvelle édition estivale !",
      },
      articles,
    },
  };
}

async function fetchEditorialGazettePreset(apiUrl, apiKey) {
  try {
    const res = await fetch(`${apiUrl}/api/v1/studio/presets`, {
      method: "GET",
      headers: {
        Accept: "application/json",
        "X-API-Key": apiKey,
      },
    });
    if (!res.ok) {
      return null;
    }
    const json = await res.json();
    const list = Array.isArray(json?.data)
      ? json.data
      : Array.isArray(json?.data?.presets)
        ? json.data.presets
        : Array.isArray(json?.presets)
          ? json.presets
          : Array.isArray(json)
            ? json
            : [];
    const preset = list.find((p) => p?.id === "editorial-gazette");
    if (preset?.blueprint) {
      return preset;
    }
  } catch {
    // Fallback silencieux vers le Blueprint inline
  }
  return null;
}

async function buildRequestSpec(opts, gazetteData) {
  const schemaRaw = await readFile(
    resolve(ROOT, "templates-custom/custom/gazette.schema.json"),
    "utf8",
  );
  const localSchema = JSON.parse(schemaRaw);

  if (opts.mode === "legacy") {
    const template = await readFile(
      resolve(ROOT, "templates-custom/custom/gazette.typ"),
      "utf8",
    );
    return {
      endpoint: `${opts.apiUrl}/api/v1/custom/generate`,
      sourceDesc: "templates-custom/custom/gazette.typ (Handlebars {{bytes}})",
      payload: {
        template,
        schema: localSchema,
        data: gazetteData.data,
      },
    };
  }

  // Mode v2 : Blueprint editorial-gazette + extraction automatique d'images binaires en RAM
  const remotePreset = await fetchEditorialGazettePreset(
    opts.apiUrl,
    opts.apiKey,
  );
  const blueprint =
    remotePreset?.blueprint ?? INLINE_EDITORIAL_GAZETTE_BLUEPRINT;
  const schema = remotePreset?.schema ?? localSchema;
  const sourceDesc = remotePreset
    ? "preset editorial-gazette (via GET /api/v1/studio/presets)"
    : "preset editorial-gazette (Blueprint inline fallback)";

  return {
    endpoint: `${opts.apiUrl}/api/v1/render`,
    sourceDesc,
    payload: {
      blueprint,
      schema,
      data: gazetteData.data,
      format: opts.format,
      engineMode: "blueprint",
    },
  };
}

function pageOutputPath(outAbs, pageIndex, totalPages, format) {
  if (totalPages <= 1 || pageIndex === 0) {
    return outAbs;
  }
  const ext = extname(outAbs) || `.${format}`;
  const stem = outAbs.slice(0, outAbs.length - ext.length);
  return `${stem}-page-${pageIndex + 1}${ext}`;
}

async function writeRenderedArtifact(firstResult, opts) {
  const outAbs = resolve(ROOT, opts.outPath);
  await mkdir(dirname(outAbs), { recursive: true });

  // Cas 1 : Réponse binaire directe (application/pdf, image/png, image/svg+xml)
  if (firstResult.rawBuffer) {
    await writeFile(outAbs, firstResult.rawBuffer);
    return {
      files: [outAbs],
      totalBytes: firstResult.rawBuffer.length,
      pageCount: firstResult.pageCount ?? 1,
    };
  }

  // Cas 2 : Enveloppe JSON ({ success, data: { ... } })
  const d = firstResult.body?.data ?? firstResult.body ?? {};

  if (opts.format === "svg") {
    const pagesSvg = Array.isArray(d.pagesSvg)
      ? d.pagesSvg
      : Array.isArray(d.pages_svg)
        ? d.pages_svg
        : Array.isArray(d.pages)
          ? d.pages.map((p) =>
              typeof p === "string" && p.trimStart().startsWith("<")
                ? p
                : Buffer.from(String(p), "base64").toString("utf8"),
            )
          : typeof d.svg === "string"
            ? [d.svg]
            : typeof d.data === "string"
              ? [
                  d.data.trimStart().startsWith("<")
                    ? d.data
                    : Buffer.from(d.data, "base64").toString("utf8"),
                ]
              : [];

    if (pagesSvg.length === 0) {
      throw new Error(
        `Réponse SVG inattendue (pas de pagesSvg/pages): ${JSON.stringify(firstResult.body).slice(0, 300)}`,
      );
    }

    const written = [];
    let totalBytes = 0;
    for (let i = 0; i < pagesSvg.length; i++) {
      const target = pageOutputPath(outAbs, i, pagesSvg.length, "svg");
      const buf = Buffer.from(pagesSvg[i], "utf8");
      await writeFile(target, buf);
      written.push(target);
      totalBytes += buf.length;
    }
    return { files: written, totalBytes, pageCount: pagesSvg.length };
  }

  if (opts.format === "png") {
    const pagesB64 = Array.isArray(d.pagesBase64)
      ? d.pagesBase64
      : Array.isArray(d.pages_base64)
        ? d.pages_base64
        : Array.isArray(d.pages)
          ? d.pages
          : typeof d.dataBase64 === "string"
            ? [d.dataBase64]
            : typeof d.data === "string"
              ? [d.data]
              : [];

    if (pagesB64.length === 0) {
      throw new Error(
        `Réponse PNG inattendue (pas de pages/pagesBase64): ${JSON.stringify(firstResult.body).slice(0, 300)}`,
      );
    }

    const written = [];
    let totalBytes = 0;
    for (let i = 0; i < pagesB64.length; i++) {
      const target = pageOutputPath(outAbs, i, pagesB64.length, "png");
      const buf = Buffer.from(pagesB64[i], "base64");
      await writeFile(target, buf);
      written.push(target);
      totalBytes += buf.length;
    }
    return { files: written, totalBytes, pageCount: pagesB64.length };
  }

  // Format PDF (mode v2 ou legacy)
  const pdfB64 =
    d.data ?? d.dataBase64 ?? d.data_base64 ?? d.pdfData ?? d.content;
  if (typeof pdfB64 !== "string" || pdfB64.length === 0) {
    throw new Error(
      `Réponse PDF inattendue (pas de data/pdfData): ${JSON.stringify(firstResult.body).slice(0, 300)}`,
    );
  }
  const pdfBuf = Buffer.from(pdfB64, "base64");
  await writeFile(outAbs, pdfBuf);
  return {
    files: [outAbs],
    totalBytes: pdfBuf.length,
    pageCount: d.pageCount ?? d.metadata?.pageCount ?? null,
  };
}

async function main() {
  const opts = parseCliArgs(process.argv.slice(2));
  if (opts.help) {
    printHelp();
    return;
  }

  const gazetteData = await loadGazetteData();
  const spec = await buildRequestSpec(opts, gazetteData);
  const bodyJSON = JSON.stringify(spec.payload);

  console.log("── Gazette Benchmark ──");
  console.log(`  Mode      : ${opts.mode}`);
  console.log(`  Endpoint  : POST ${spec.endpoint}`);
  console.log(`  Source    : ${spec.sourceDesc}`);
  console.log(`  Format    : ${opts.format}`);
  console.log(`  Images    : ${gazetteData.imageCount} images base64 inline`);
  console.log(`  Parallèle : ${opts.parallel} requêtes`);
  console.log("");

  const singleCall = async (id) => {
    const t0 = performance.now();
    const res = await fetch(spec.endpoint, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
        "X-API-Key": opts.apiKey,
      },
      body: bodyJSON,
    });

    const contentType = (res.headers.get("content-type") ?? "").toLowerCase();
    const headerBackend = res.headers.get("x-render-backend");
    const headerDuration = res.headers.get("x-render-duration-ms");
    const headerPages = res.headers.get("x-render-pages");

    let body = null;
    let rawBuffer = null;
    if (contentType.includes("application/json")) {
      body = await res.json();
    } else {
      rawBuffer = Buffer.from(await res.arrayBuffer());
    }

    const t1 = performance.now();
    const elapsedMs = t1 - t0;
    const ok = res.ok && (body === null || body.success !== false);
    const dataObj = body?.data ?? body ?? {};
    const backend =
      headerBackend ??
      dataObj?.backend ??
      dataObj?.metadata?.backend ??
      (opts.mode === "legacy" ? "legacy-cli" : "unknown");
    const pageCount = headerPages
      ? Number(headerPages)
      : (dataObj?.pageCount ?? null);
    const compileDurationMs = headerDuration
      ? Number(headerDuration)
      : (dataObj?.metadata?.compileDurationMs ?? dataObj?.durationMs ?? null);
    const extractedImages = dataObj?.metadata?.extractedImages ?? null;

    console.log(
      `  [appel ${String(id).padStart(2, " ")}] ${ok ? "✓" : "✗"} HTTP ${res.status} — ${elapsedMs.toFixed(0)} ms (backend=${backend})`,
    );

    return {
      id,
      ok,
      status: res.status,
      elapsedMs,
      backend,
      pageCount,
      compileDurationMs,
      extractedImages,
      body,
      rawBuffer,
    };
  };

  const tStart = performance.now();
  const results = await Promise.all(
    Array.from({ length: opts.parallel }, (_, i) => singleCall(i + 1)),
  );
  const tEnd = performance.now();
  const totalElapsedMs = tEnd - tStart;

  const successes = results.filter((r) => r.ok);
  const failures = results.filter((r) => !r.ok);

  const sortedElapsed = results
    .map((r) => r.elapsedMs)
    .sort((a, b) => a - b);
  const minMs = sortedElapsed[0] ?? 0;
  const maxMs = sortedElapsed[sortedElapsed.length - 1] ?? 0;
  const avgMs =
    sortedElapsed.reduce((a, b) => a + b, 0) / Math.max(sortedElapsed.length, 1);
  const p50Ms = percentile(sortedElapsed, 0.5);
  const p95Ms = percentile(sortedElapsed, 0.95);
  const throughput =
    totalElapsedMs > 0 ? (successes.length / (totalElapsedMs / 1000)).toFixed(2) : "0.00";

  const activeBackends = [
    ...new Set(successes.map((r) => r.backend).filter(Boolean)),
  ];
  const sampleMeta = successes[0];

  console.log("");
  console.log("── Statistiques de performance ──");
  console.log(`  Mode             : ${opts.mode} (${opts.format})`);
  console.log(
    `  Backend actif    : ${activeBackends.length > 0 ? activeBackends.join(", ") : "inconnu"}`,
  );
  console.log(`  Appels réussis   : ${successes.length}/${opts.parallel}`);
  console.log(`  Appels échoués   : ${failures.length}/${opts.parallel}`);
  console.log(`  Temps total mur  : ${totalElapsedMs.toFixed(0)} ms`);
  console.log(`  Débit            : ${throughput} docs/sec`);
  console.log(`  Latence min      : ${minMs.toFixed(0)} ms`);
  console.log(`  Latence moy      : ${avgMs.toFixed(0)} ms`);
  console.log(`  Latence p50      : ${p50Ms.toFixed(0)} ms`);
  console.log(`  Latence p95      : ${p95Ms.toFixed(0)} ms`);
  console.log(`  Latence max      : ${maxMs.toFixed(0)} ms`);
  if (sampleMeta?.compileDurationMs != null) {
    console.log(
      `  Compile serveur  : ${sampleMeta.compileDurationMs} ms (1er appel)`,
    );
  }
  if (sampleMeta?.extractedImages != null) {
    console.log(
      `  Images RAM       : ${sampleMeta.extractedImages} extraites en binaire`,
    );
  }
  console.log("");

  if (failures.length > 0) {
    console.error("✗ Échecs détaillés:");
    for (const f of failures) {
      const detail =
        f.body ??
        (f.rawBuffer ? f.rawBuffer.toString("utf8").slice(0, 300) : "empty");
      console.error(`  [appel ${f.id}] HTTP ${f.status}:`, detail);
    }
    process.exit(1);
  }

  const written = await writeRenderedArtifact(successes[0], opts);
  const sizeKb = (written.totalBytes / 1024).toFixed(1);
  const pageInfo =
    written.pageCount != null ? `, ${written.pageCount} page(s)` : "";
  console.log(
    `✓ Sortie ${opts.format.toUpperCase()} générée: ${written.files.join(", ")} (${written.totalBytes} octets / ${sizeKb} Ko${pageInfo})`,
  );
}

main().catch((err) => {
  console.error("✗ Erreur:", err?.message ?? err);
  process.exit(1);
});
