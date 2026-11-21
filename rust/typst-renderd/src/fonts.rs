use std::path::{Path, PathBuf};
use std::sync::OnceLock;

use tracing::{debug, warn};
use typst::foundations::Bytes;
use typst::text::{Font, FontBook};
use typst::utils::LazyHash;
use typst::{Library, LibraryExt};
use typst_kit::fonts::{self, FontPath};

/// Internal lazily-loaded font entry backing an index in `FontBook`.
enum CachedFontSlot {
    /// Already parsed in-memory font (from Typst's embedded font bundle).
    Embedded(Font),
    /// Disk-backed font face parsed lazily on first reference and cached forever.
    Disk {
        path: PathBuf,
        index: u32,
        cache: OnceLock<Option<Font>>,
    },
}

impl CachedFontSlot {
    fn from_path(font_path: FontPath) -> Self {
        Self::Disk {
            path: font_path.path,
            index: font_path.index,
            cache: OnceLock::new(),
        }
    }

    fn get(&self) -> Option<Font> {
        match self {
            Self::Embedded(font) => Some(font.clone()),
            Self::Disk { path, index, cache } => cache
                .get_or_init(|| load_font_from_disk(path, *index))
                .clone(),
        }
    }
}

fn load_font_from_disk(path: &Path, index: u32) -> Option<Font> {
    match std::fs::read(path) {
        Ok(data) => {
            let font = Font::new(Bytes::new(data), index);
            if font.is_none() {
                warn!(
                    path = %path.display(),
                    face_index = index,
                    "failed to parse font face from file"
                );
            }
            font
        }
        Err(err) => {
            warn!(
                path = %path.display(),
                face_index = index,
                error = %err,
                "failed to read font file from disk"
            );
            None
        }
    }
}

/// Process-wide, immutable font and standard-library cache shared across all
/// worker threads via `Arc<FontCache>`.
///
/// Font discovery happens strictly once at daemon startup, eliminating the
/// per-invocation font scanning cost of spawning the `typst` CLI.
pub struct FontCache {
    library: LazyHash<Library>,
    book: LazyHash<FontBook>,
    fonts: Vec<CachedFontSlot>,
}

impl FontCache {
    /// Scans configured `--font-path` directories, optional host system fonts,
    /// and Typst's embedded fallback fonts once at daemon boot.
    pub fn load(font_paths: &[PathBuf], ignore_system_fonts: bool) -> Self {
        let mut book = FontBook::new();
        let mut slots = Vec::new();

        // 1. Custom font directories first so project/brand fonts take priority.
        for dir in font_paths {
            if !dir.exists() {
                warn!(
                    path = %dir.display(),
                    "configured font path does not exist, skipping"
                );
                continue;
            }

            let before = slots.len();
            for (font_path, info) in fonts::scan(dir) {
                book.push(info);
                slots.push(CachedFontSlot::from_path(font_path));
            }
            debug!(
                path = %dir.display(),
                added = slots.len() - before,
                "scanned custom font directory"
            );
        }

        // 2. Host system fonts (unless disabled via --ignore-system-fonts).
        if !ignore_system_fonts {
            let before = slots.len();
            for (font_path, info) in fonts::system() {
                book.push(info);
                slots.push(CachedFontSlot::from_path(font_path));
            }
            debug!(added = slots.len() - before, "discovered host system fonts");
        }

        // 3. Embedded Typst fallback fonts (Libertinus Serif, New Computer Modern, DejaVu Sans Mono).
        let before = slots.len();
        for (font, info) in fonts::embedded() {
            book.push(info);
            slots.push(CachedFontSlot::Embedded(font));
        }
        debug!(
            added = slots.len() - before,
            "loaded embedded fallback fonts"
        );

        Self {
            library: LazyHash::new(Library::default()),
            book: LazyHash::new(book),
            fonts: slots,
        }
    }

    /// Returns the shared Typst standard library wrapped in `LazyHash`.
    pub fn library(&self) -> &LazyHash<Library> {
        &self.library
    }

    /// Returns the shared font metadata book wrapped in `LazyHash`.
    pub fn book(&self) -> &LazyHash<FontBook> {
        &self.book
    }

    /// Total number of font faces indexed in this cache.
    pub fn font_count(&self) -> usize {
        self.fonts.len()
    }

    /// Retrieves (lazily loading and caching if disk-backed) the font at `index`.
    pub fn get_font(&self, index: usize) -> Option<Font> {
        self.fonts.get(index)?.get()
    }
}
