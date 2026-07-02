#!/usr/bin/env node
/**
 * Script rapide pour appeler /api/v1/custom/generate avec le template gazette.
 *
 * Usage:
 *   node scripts/gazette-generate.mjs
 *
 * Variables d'env (optionnelles):
 *   FX_API_URL    – URL de base (défaut: http://localhost:8080)
 *   FX_API_KEY    – clé API (défaut: valeur du .env.example)
 *   FX_LOGO_PATH  – chemin local du logo à envoyer en base64 (défaut: images/logo.png)
 *   FX_OUT        – fichier PDF de sortie (défaut: tmp/gazette.pdf)
 */

import { mkdir, readFile, readdir, writeFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT = resolve(__dirname, "..");

const API_URL = process.env.FX_API_URL ?? "http://localhost:8080";
const API_KEY = process.env.FX_API_KEY ?? "api-key";
const OUT_PATH = process.env.FX_OUT_PATH ?? "tmp/gazette.pdf";

async function main() {
  // 1. Lecture des fichiers template + schema + logo
  const [template, schemaRaw, logo, imageFiles] = await Promise.all([
    readFile(resolve(ROOT, "templates-custom/custom/gazette.typ"), "utf8"),
    readFile(
      resolve(ROOT, "templates-custom/custom/gazette.schema.json"),
      "utf8",
    ),
    readFile(resolve(ROOT, process.env.FX_LOGO_PATH ?? "images/logo.png")),
    // Liste toutes les images de images/ (hors logo.png)
    readdir(resolve(ROOT, "images")),
  ]);

  // Le schema doit être un objet JSON (pas une chaîne) pour l'API Go.
  const schema = JSON.parse(schemaRaw);

  // On passe l'image "à la volée" dans les données en base64. Le moteur la
  // décode et la rend EN MÉMOIRE via le helper {{bytes src}} (Typst bytes(...)),
  // sans écrire de fichier ni utiliser d'assets.
  const logoB64 = logo.toString("base64");

  // Récupère toutes les images de images/ (hors logo.png), triées par nom.
  const imagePaths = imageFiles
    .filter((f) => /\.(png|jpe?g|gif|webp)$/i.test(f) && f.endsWith("webp"))
    .sort()
    .map((f) => resolve(ROOT, "images", f));

  // Lit toutes les images en parallèle et les encode en base64.
  const imageBuffers = await Promise.all(imagePaths.map((p) => readFile(p)));
  const imagesB64 = imageBuffers.map((buf, i) => ({
    url: imagePaths[i].split("/").pop(),
    src: buf.toString("base64"),
    alt_text: `Image ${i + 1}`,
  }));

  // Répartition: article 1 -> 3 images, article 2 -> 5 images, article 3 -> 7 images
  const distribution = [3, 5, 7];
  const totalNeeded = distribution.reduce((a, b) => a + b, 0); // 15
  // Si pas assez d'images, on boucle (réutilise) pour atteindre le compte.
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
      date_evenement: `juillet 2026`,
      texte_descriptif: descriptions[idx % descriptions.length],
      style_image: "polaroid",
      images: imgs,
    };
  });

  // 2. Construction du payload conforme au schema gazette
  const payload = {
    template,
    schema,
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

  // 3. Appel POST /api/v1/custom/generate — 5 appels en parallèle pour mesurer les perfs
  const PARALLEL = 5;
  console.log(
    `→ POST ${API_URL}/api/v1/custom/generate × ${PARALLEL} (parallèle)`,
  );

  const singleCall = async (id) => {
    const t0 = performance.now();
    const res = await fetch(`${API_URL}/api/v1/custom/generate`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-API-Key": API_KEY,
      },
      body: JSON.stringify(payload),
    });
    const body = await res.json();
    const t1 = performance.now();
    const elapsed = (t1 - t0).toFixed(0);
    const ok = res.ok;
    console.log(
      `  [appel ${id}] ${ok ? "✓" : "✗"} HTTP ${res.status} — ${elapsed} ms`,
    );
    return { id, ok, status: res.status, elapsed, body, t0, t1 };
  };

  const tStart = performance.now();
  const results = await Promise.all(
    Array.from({ length: PARALLEL }, (_, i) => singleCall(i + 1)),
  );
  const tEnd = performance.now();
  const totalElapsed = (tEnd - tStart).toFixed(0);

  const successes = results.filter((r) => r.ok);
  const failures = results.filter((r) => !r.ok);

  const elapsedNums = results.map((r) => Number(r.elapsed));
  const minMs = Math.min(...elapsedNums);
  const maxMs = Math.max(...elapsedNums);
  const avgMs = (
    elapsedNums.reduce((a, b) => a + b, 0) / elapsedNums.length
  ).toFixed(0);

  console.log("");
  console.log("── Statistiques de performance ──");
  console.log(`  Appels réussis   : ${successes.length}/${PARALLEL}`);
  console.log(`  Appels échoués   : ${failures.length}/${PARALLEL}`);
  console.log(`  Temps total mur  : ${totalElapsed} ms`);
  console.log(`  Temps min / appel: ${minMs} ms`);
  console.log(`  Temps max / appel: ${maxMs} ms`);
  console.log(`  Temps moy / appel: ${avgMs} ms`);
  console.log("");

  if (failures.length > 0) {
    console.error("✗ Échecs détaillés:");
    for (const f of failures) {
      console.error(`  [appel ${f.id}] HTTP ${f.status}:`, f.body);
    }
    process.exit(1);
  }

  // 4. Décodage du PDF base64 → fichier (on prend le 1er résultat réussi)
  const first = successes[0];
  const body = first.body;
  const pdfB64 = body?.data?.pdfData;
  if (!pdfB64) {
    console.error("✗ Réponse inattendue (pas de pdfData):", body);
    process.exit(1);
  }

  const outAbs = resolve(ROOT, OUT_PATH);
  await mkdir(dirname(outAbs), { recursive: true });
  await writeFile(outAbs, Buffer.from(pdfB64, "base64"));

  console.log(
    `✓ PDF généré: ${outAbs} (${body.data.metadata?.size ?? "?"} octets)`,
  );
}

main().catch((err) => {
  console.error("✗ Erreur:", err);
  process.exit(1);
});
