use std::collections::HashMap;
use std::path::{Path, PathBuf};
use std::sync::Arc;

use chrono::{DateTime, Datelike, FixedOffset, Local, Timelike, Utc};
use typst::diag::{FileError, FileResult, Severity, SourceDiagnostic};
use typst::foundations::{Bytes, Datetime, Duration};
use typst::syntax::{FileId, RootedPath, Source, VirtualPath, VirtualRoot};
use typst::text::{Font, FontBook};
use typst::utils::LazyHash;
use typst::{Library, World, WorldExt};

use crate::fonts::FontCache;
use crate::protocol::{decode_base64_payload, CompileRequest, DiagnosticItem};

/// Normalizes a client-supplied virtual path into a canonical rooted path
/// starting with `/`.
///
/// - Backslashes (`\`) are converted to forward slashes (`/`).
/// - Empty and `.` segments are removed.
/// - `..` traversal segments pop the preceding segment and are clamped at the
///   virtual root `/` (they can never escape the virtual filesystem).
/// - Rejects embedded NUL bytes and paths that resolve to empty / root `/`.
pub fn normalize_virtual_path(raw: &str) -> Result<String, String> {
    if raw.contains('\0') {
        return Err("virtual path must not contain NUL bytes".to_owned());
    }

    let replaced = raw.replace('\\', "/");
    let mut segments: Vec<&str> = Vec::new();

    for segment in replaced.split('/') {
        match segment {
            "" | "." => {}
            ".." => {
                segments.pop();
            }
            part => segments.push(part),
        }
    }

    if segments.is_empty() {
        return Err(format!(
            "virtual path '{raw}' resolves to root directory rather than a file"
        ));
    }

    Ok(format!("/{}", segments.join("/")))
}

/// Returns `true` if the normalized virtual path has a `.typ` extension (case-insensitive).
pub fn is_typst_source_path(normalized: &str) -> bool {
    Path::new(normalized)
        .extension()
        .is_some_and(|ext| ext.eq_ignore_ascii_case("typ"))
}

fn virtual_file_id(normalized_path: &str) -> Result<FileId, String> {
    let vpath = VirtualPath::new(normalized_path)
        .map_err(|err| format!("invalid virtual path '{normalized_path}': {err:?}"))?;
    Ok(FileId::new(RootedPath::new(VirtualRoot::Project, vpath)))
}

/// Hermetic, strictly in-memory `typst::World` implementation.
///
/// Every source file, text data file (e.g. `/data.json`, `/factur-x.xml`), and
/// binary asset (e.g. logos, embedded images) lives in memory for the lifetime
/// of a single compilation request. The host filesystem is NEVER accessed
/// during compilation.
pub struct VirtualWorld {
    fonts: Arc<FontCache>,
    main_id: FileId,
    sources: HashMap<FileId, Source>,
    files: HashMap<FileId, Bytes>,
    paths: HashMap<FileId, String>,
    now: DateTime<Utc>,
}

impl VirtualWorld {
    /// Builds a hermetic `VirtualWorld` from an incoming `CompileRequest`.
    ///
    /// Returns an `Err(String)` describing any invalid virtual path or malformed
    /// Base64 payload (which maps to HTTP 400).
    pub fn from_request(fonts: Arc<FontCache>, req: CompileRequest) -> Result<Self, String> {
        let main_norm = normalize_virtual_path(&req.main_path)
            .map_err(|err| format!("invalid main_path: {err}"))?;
        let main_id = virtual_file_id(&main_norm)?;

        let estimated_files = 1 + req.virtual_files.len() + req.binary_files.len();
        let mut sources = HashMap::with_capacity(1 + req.virtual_files.len());
        let mut files = HashMap::with_capacity(estimated_files);
        let mut paths = HashMap::with_capacity(estimated_files);

        // 1. Insert UTF-8 virtual files (Typst modules, JSON data, Factur-X XML, CSV, etc.).
        for (raw_path, text) in req.virtual_files {
            let norm = normalize_virtual_path(&raw_path)
                .map_err(|err| format!("virtual_files['{raw_path}']: {err}"))?;
            let id = virtual_file_id(&norm)?;

            if is_typst_source_path(&norm) {
                sources.insert(id, Source::new(id, text.clone()));
            }
            files.insert(id, Bytes::from_string(text));
            paths.insert(id, norm);
        }

        // 2. Insert Base64-encoded binary files (logos, PNG/JPEG/SVG assets, etc.).
        for (raw_path, b64_payload) in req.binary_files {
            let norm = normalize_virtual_path(&raw_path)
                .map_err(|err| format!("binary_files['{raw_path}']: {err}"))?;
            let id = virtual_file_id(&norm)?;
            let raw_bytes = decode_base64_payload(&b64_payload)
                .map_err(|err| format!("binary_files['{raw_path}']: {err}"))?;

            files.insert(id, Bytes::new(raw_bytes));
            paths.insert(id, norm);
        }

        // 3. Insert the main Typst entry source last so `req.source` is always authoritative.
        sources.insert(main_id, Source::new(main_id, req.source.clone()));
        files.insert(main_id, Bytes::from_string(req.source));
        paths.insert(main_id, main_norm);

        Ok(Self {
            fonts,
            main_id,
            sources,
            files,
            paths,
            now: Utc::now(),
        })
    }

    /// Resolves a Typst `SourceDiagnostic` (error or warning) into a structured
    /// JSON `DiagnosticItem` with 1-based line and column coordinates.
    pub fn resolve_diagnostic(&self, diag: &SourceDiagnostic) -> DiagnosticItem {
        let severity = match diag.severity {
            Severity::Error => "error",
            Severity::Warning => "warning",
        }
        .to_owned();

        let mut file = None;
        let mut line = None;
        let mut column = None;

        if let Some(id) = diag.span.id() {
            file = Some(self.display_path(id));
            if let Some(source) = self.sources.get(&id) {
                if let Some(range) = self.range(diag.span) {
                    let lines = source.lines();
                    if let Some(line_idx) = lines.byte_to_line(range.start) {
                        line = Some(line_idx + 1);
                    }
                    if let Some(col_idx) = lines.byte_to_column(range.start) {
                        column = Some(col_idx + 1);
                    }
                }
            }
        }

        // In Typst 0.15, diagnostic hints are `Spanned<EcoString>` (`hint.v` holds the message).
        let hints = diag
            .hints
            .iter()
            .map(|hint| hint.v.to_string())
            .collect();

        DiagnosticItem {
            severity,
            message: diag.message.to_string(),
            file,
            line,
            column,
            hints,
        }
    }

    fn display_path(&self, id: FileId) -> String {
        self.paths
            .get(&id)
            .cloned()
            .unwrap_or_else(|| id.vpath().get_with_slash().to_owned())
    }

    /// Returns the request UTC timestamp as a Typst `Datetime` so PDF / PDF/A-3b
    /// metadata always includes a valid creation timestamp.
    pub fn utc_datetime(&self) -> Option<Datetime> {
        Datetime::from_ymd_hms(
            self.now.year(),
            self.now.month().try_into().ok()?,
            self.now.day().try_into().ok()?,
            self.now.hour().try_into().ok()?,
            self.now.minute().try_into().ok()?,
            self.now.second().try_into().ok()?,
        )
    }
}

impl World for VirtualWorld {
    fn library(&self) -> &LazyHash<Library> {
        self.fonts.library()
    }

    fn book(&self) -> &LazyHash<FontBook> {
        self.fonts.book()
    }

    fn main(&self) -> FileId {
        self.main_id
    }

    fn source(&self, id: FileId) -> FileResult<Source> {
        if let Some(source) = self.sources.get(&id) {
            return Ok(source.clone());
        }

        // Support importing a UTF-8 virtual file even if it does not end with `.typ`.
        if let Some(bytes) = self.files.get(&id) {
            let text = std::str::from_utf8(bytes.as_slice()).map_err(|_| FileError::InvalidUtf8)?;
            return Ok(Source::new(id, text.to_owned()));
        }

        Err(FileError::NotFound(PathBuf::from(self.display_path(id))))
    }

    fn file(&self, id: FileId) -> FileResult<Bytes> {
        self.files
            .get(&id)
            .cloned()
            .ok_or_else(|| FileError::NotFound(PathBuf::from(self.display_path(id))))
    }

    fn font(&self, index: usize) -> Option<Font> {
        self.fonts.get_font(index)
    }

    fn today(&self, offset: Option<Duration>) -> Option<Datetime> {
        match offset {
            None => {
                let local = self.now.with_timezone(&Local);
                Datetime::from_ymd(
                    local.year(),
                    local.month().try_into().ok()?,
                    local.day().try_into().ok()?,
                )
            }
            Some(duration) => {
                let offset_secs = duration.seconds().round() as i64;
                let offset_i32 = i32::try_from(offset_secs).ok()?;
                let tz = FixedOffset::east_opt(offset_i32)?;
                let dt = self.now.with_timezone(&tz);
                Datetime::from_ymd(
                    dt.year(),
                    dt.month().try_into().ok()?,
                    dt.day().try_into().ok()?,
                )
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn normalize_virtual_path_handles_leading_slashes_and_relative_segments() {
        assert_eq!(normalize_virtual_path("main.typ").unwrap(), "/main.typ");
        assert_eq!(normalize_virtual_path("/main.typ").unwrap(), "/main.typ");
        assert_eq!(
            normalize_virtual_path("///nested//dir/./file.typ").unwrap(),
            "/nested/dir/file.typ"
        );
        assert_eq!(
            normalize_virtual_path("./assets/logo.png").unwrap(),
            "/assets/logo.png"
        );
    }

    #[test]
    fn normalize_virtual_path_strips_traversal_and_backslashes() {
        assert_eq!(
            normalize_virtual_path("../../etc/passwd").unwrap(),
            "/etc/passwd"
        );
        assert_eq!(
            normalize_virtual_path("/a/b/../c/./d.typ").unwrap(),
            "/a/c/d.typ"
        );
        assert_eq!(
            normalize_virtual_path("/a/../../secret.xml").unwrap(),
            "/secret.xml"
        );
        assert_eq!(
            normalize_virtual_path(r"templates\facture\..\invoice.typ").unwrap(),
            "/templates/invoice.typ"
        );
    }

    #[test]
    fn normalize_virtual_path_rejects_empty_root_and_nul() {
        assert!(normalize_virtual_path("").is_err());
        assert!(normalize_virtual_path("/").is_err());
        assert!(normalize_virtual_path("./..").is_err());
        assert!(normalize_virtual_path("/../../").is_err());
        assert!(normalize_virtual_path("/bad\0path.typ").is_err());
    }

    #[test]
    fn typst_source_extension_detection_is_case_insensitive_and_utf8_safe() {
        assert!(is_typst_source_path("/main.typ"));
        assert!(is_typst_source_path("/templates/FACTURE.TYP"));
        assert!(is_typst_source_path("/modèle/facture-été.typ"));
        assert!(!is_typst_source_path("/data.json"));
        assert!(!is_typst_source_path("/factur-x.xml"));
        assert!(!is_typst_source_path("/é.a"));
        assert!(!is_typst_source_path("/no_extension"));
    }
}
