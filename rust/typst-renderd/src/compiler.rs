use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::Arc;
use std::time::Instant;

use tokio::sync::Semaphore;
use tracing::{debug, error};
use typst::foundations::{Dict, Label, Selector, Value};
use typst::introspection::{Introspector, MetadataElem};
use typst::layout::Em;
use typst::utils::{PicoStr, Scalar};
use typst_layout::PagedDocument;
use typst_pdf::{PdfOptions, PdfStandard, PdfStandards, Timestamp};
use typst_render::RenderOptions;
use typst_svg::SvgOptions;

use crate::fonts::FontCache;
use crate::protocol::{
    encode_base64, CompileRequest, CompileResponse, DiagnosticItem, LayoutBox, OutputFormat,
};
use crate::world::VirtualWorld;

/// Interned Typst label emitted by fX Blueprint block wrappers for exact layout extraction.
const BLOCK_BOX_LABEL: PicoStr = PicoStr::constant("fx-block-box");

/// Minimum interactive bounding-box dimension (in pt) for thin dividers/spacers.
const MIN_BOX_DIM_PT: f64 = 8.0;

/// Minimum remaining height clamp floor (in pt) when a block starts near the bottom edge of a page.
const MIN_REMAINING_HEIGHT_PT: f64 = 12.0;

/// Evict stale comemo memoization cache entries every N renders to bound long-term RSS.
const EVICT_INTERVAL: u64 = 16;

/// Maximum age (in eviction ticks) retained by `comemo::evict`.
const COMEMO_MAX_AGE: usize = 10;

/// Result classification returned to the HTTP layer so status codes map cleanly:
/// - `Success` -> HTTP 200
/// - `InvalidRequest` -> HTTP 400
/// - `CompileError` -> HTTP 422
pub enum CompileOutcome {
    Success(CompileResponse),
    InvalidRequest(CompileResponse),
    CompileError(CompileResponse),
}

/// Concurrency-bounded Typst compilation & export engine backed by `spawn_blocking`.
pub struct CompilerService {
    fonts: Arc<FontCache>,
    semaphore: Arc<Semaphore>,
    compile_counter: Arc<AtomicU64>,
}

impl CompilerService {
    pub fn new(fonts: Arc<FontCache>, workers: usize) -> Self {
        Self {
            fonts,
            semaphore: Arc::new(Semaphore::new(workers.max(1))),
            compile_counter: Arc::new(AtomicU64::new(0)),
        }
    }

    /// Number of indexed fonts available in the shared `FontCache`.
    pub fn font_count(&self) -> usize {
        self.fonts.font_count()
    }

    /// Compiles and exports a single request on Tokio's blocking worker pool,
    /// bounded to `--workers` simultaneous Typst executions.
    pub async fn compile(&self, request: CompileRequest) -> CompileOutcome {
        let format = match request.validate() {
            Ok(fmt) => fmt,
            Err(err) => {
                return CompileOutcome::InvalidRequest(CompileResponse::failure(
                    request.format,
                    "",
                    0,
                    Vec::new(),
                    vec![DiagnosticItem::error(err)],
                ));
            }
        };

        let _permit = match Arc::clone(&self.semaphore).acquire_owned().await {
            Ok(permit) => permit,
            Err(_) => {
                return CompileOutcome::CompileError(CompileResponse::failure(
                    format.as_str(),
                    format.mime_type(),
                    0,
                    Vec::new(),
                    vec![DiagnosticItem::error("compiler worker pool shut down")],
                ));
            }
        };

        let fonts = Arc::clone(&self.fonts);
        let counter = Arc::clone(&self.compile_counter);

        match tokio::task::spawn_blocking(move || {
            compile_blocking(fonts, counter, request, format)
        })
        .await
        {
            Ok(outcome) => outcome,
            Err(join_err) => {
                error!(error = %join_err, "typst worker thread panicked or was cancelled");
                CompileOutcome::CompileError(CompileResponse::failure(
                    format.as_str(),
                    format.mime_type(),
                    0,
                    Vec::new(),
                    vec![DiagnosticItem::error(format!(
                        "internal compilation worker error: {join_err}"
                    ))],
                ))
            }
        }
    }
}

fn compile_blocking(
    fonts: Arc<FontCache>,
    counter: Arc<AtomicU64>,
    request: CompileRequest,
    format: OutputFormat,
) -> CompileOutcome {
    let started = Instant::now();
    let ppi = request.ppi;

    let world = match VirtualWorld::from_request(fonts, request) {
        Ok(world) => world,
        Err(err) => {
            return CompileOutcome::InvalidRequest(CompileResponse::failure(
                format.as_str(),
                format.mime_type(),
                elapsed_ms(started),
                Vec::new(),
                vec![DiagnosticItem::error(err)],
            ));
        }
    };

    let warned = typst::compile::<PagedDocument>(&world);

    let warnings: Vec<DiagnosticItem> = warned
        .warnings
        .iter()
        .map(|diag| world.resolve_diagnostic(diag))
        .collect();

    // Periodically trim global comemo memoization tables so daemon memory stays bounded.
    let render_num = counter.fetch_add(1, Ordering::Relaxed) + 1;
    if render_num.is_multiple_of(EVICT_INTERVAL) {
        comemo::evict(COMEMO_MAX_AGE);
        debug!(renders = render_num, "evicted stale comemo cache entries");
    }

    match warned.output {
        Err(errors) => {
            let resolved_errors: Vec<DiagnosticItem> = errors
                .iter()
                .map(|diag| world.resolve_diagnostic(diag))
                .collect();
            CompileOutcome::CompileError(CompileResponse::failure(
                format.as_str(),
                format.mime_type(),
                elapsed_ms(started),
                warnings,
                resolved_errors,
            ))
        }
        Ok(document) => export_document(&world, &document, format, ppi, warnings, started),
    }
}

fn export_document(
    world: &VirtualWorld,
    document: &PagedDocument,
    format: OutputFormat,
    ppi: f32,
    warnings: Vec<DiagnosticItem>,
    started: Instant,
) -> CompileOutcome {
    let timestamp = world.utc_datetime().map(Timestamp::new_utc);
    let outcome = match format {
        OutputFormat::Pdf => {
            let options = PdfOptions {
                timestamp,
                ..PdfOptions::default()
            };
            export_pdf(world, document, format, &options, warnings, started)
        }
        OutputFormat::PdfA3b => {
            let standards = match PdfStandards::new(&[PdfStandard::A_3b]) {
                Ok(standards) => standards,
                Err(err) => {
                    return CompileOutcome::CompileError(CompileResponse::failure(
                        format.as_str(),
                        format.mime_type(),
                        elapsed_ms(started),
                        warnings,
                        vec![DiagnosticItem::error(format!(
                            "failed to configure PDF/A-3b compliance: {}",
                            err.message()
                        ))],
                    ));
                }
            };
            let options = PdfOptions {
                timestamp,
                standards,
                ..PdfOptions::default()
            };
            export_pdf(world, document, format, &options, warnings, started)
        }
        OutputFormat::Svg => {
            let svg_options = SvgOptions::default();
            let pages_svg: Vec<String> = document
                .pages()
                .iter()
                .map(|page| typst_svg::svg(page, &svg_options))
                .collect();
            CompileOutcome::Success(CompileResponse::success_svg(
                pages_svg,
                elapsed_ms(started),
                warnings,
            ))
        }
        OutputFormat::Png => {
            let pixel_per_pt = Scalar::new(f64::from(ppi) / 72.0);
            let render_options = RenderOptions {
                pixel_per_pt,
                ..RenderOptions::default()
            };
            let pages = document.pages();
            let mut pages_base64 = Vec::with_capacity(pages.len());
            for (idx, page) in pages.iter().enumerate() {
                let pixmap = typst_render::render(page, &render_options);
                match pixmap.encode_png() {
                    Ok(png_bytes) => pages_base64.push(encode_base64(&png_bytes)),
                    Err(err) => {
                        return CompileOutcome::CompileError(CompileResponse::failure(
                            format.as_str(),
                            format.mime_type(),
                            elapsed_ms(started),
                            warnings,
                            vec![DiagnosticItem::error(format!(
                                "PNG encoding failed on page {}: {err}",
                                idx + 1
                            ))],
                        ));
                    }
                }
            }
            CompileOutcome::Success(CompileResponse::success_png(
                pages_base64,
                elapsed_ms(started),
                warnings,
            ))
        }
    };

    match outcome {
        CompileOutcome::Success(resp) => {
            CompileOutcome::Success(resp.with_layout_boxes(extract_layout_boxes(document)))
        }
        other => other,
    }
}

/// Extracts exact layout bounding boxes emitted via `#metadata((id, page, x, y, w, h)) <fx-block-box>`
/// elements from a compiled Typst `PagedDocument`.
pub fn extract_layout_boxes(document: &PagedDocument) -> Vec<LayoutBox> {
    let Some(label) = Label::new(BLOCK_BOX_LABEL) else {
        return Vec::new();
    };
    let selector = Selector::Label(label);
    let matches = document.introspector().query(&selector);
    if matches.is_empty() {
        return Vec::new();
    }

    let pages = document.pages();
    let mut boxes = Vec::with_capacity(matches.len());

    for content in &matches {
        let Some(metadata) = content.to_packed::<MetadataElem>() else {
            continue;
        };
        let Value::Dict(dict) = &metadata.value else {
            continue;
        };

        let Some(Value::Str(id_str)) = dict.get("id").ok() else {
            continue;
        };
        let id = id_str.as_str();
        if id.is_empty() {
            continue;
        }

        let Some(page) = dict_page(dict, "page") else {
            continue;
        };
        let Some(page_obj) = page.checked_sub(1).and_then(|idx| pages.get(idx)) else {
            continue;
        };

        let (Some(x), Some(y), Some(raw_w), Some(raw_h)) = (
            dict_f64(dict, "x"),
            dict_f64(dict, "y"),
            dict_f64(dict, "w"),
            dict_f64(dict, "h"),
        ) else {
            continue;
        };

        let page_size = page_obj.frame.size();
        let page_width = page_size.x.to_pt();
        let page_height = page_size.y.to_pt();
        if !page_width.is_finite()
            || !page_height.is_finite()
            || page_width <= 0.0
            || page_height <= 0.0
        {
            continue;
        }

        let width = raw_w.max(MIN_BOX_DIM_PT);
        let height = raw_h
            .min((page_height - y).max(MIN_REMAINING_HEIGHT_PT))
            .max(MIN_BOX_DIM_PT);

        boxes.push(LayoutBox {
            id: id.to_owned(),
            page,
            x,
            y,
            width,
            height,
            page_width,
            page_height,
        });
    }

    boxes
}

fn dict_page(dict: &Dict, key: &str) -> Option<usize> {
    match dict.get(key).ok()? {
        Value::Int(v) => usize::try_from(*v).ok().filter(|&p| p >= 1),
        _ => None,
    }
}

fn dict_f64(dict: &Dict, key: &str) -> Option<f64> {
    let val = match dict.get(key).ok()? {
        Value::Float(v) => *v,
        Value::Int(v) => *v as f64,
        Value::Length(len) if len.em == Em::zero() => len.abs.to_pt(),
        _ => return None,
    };
    val.is_finite().then_some(val)
}

fn export_pdf(
    world: &VirtualWorld,
    document: &PagedDocument,
    format: OutputFormat,
    options: &PdfOptions,
    warnings: Vec<DiagnosticItem>,
    started: Instant,
) -> CompileOutcome {
    match typst_pdf::pdf(document, options) {
        Ok(pdf_bytes) => CompileOutcome::Success(CompileResponse::success_pdf(
            format,
            &pdf_bytes,
            document.pages().len(),
            elapsed_ms(started),
            warnings,
        )),
        Err(errors) => {
            let resolved_errors: Vec<DiagnosticItem> = errors
                .iter()
                .map(|diag| world.resolve_diagnostic(diag))
                .collect();
            CompileOutcome::CompileError(CompileResponse::failure(
                format.as_str(),
                format.mime_type(),
                elapsed_ms(started),
                warnings,
                resolved_errors,
            ))
        }
    }
}

fn elapsed_ms(started: Instant) -> u64 {
    started.elapsed().as_millis() as u64
}

#[cfg(test)]
mod tests {
    use std::collections::HashMap;
    use std::sync::OnceLock;

    use super::*;

    fn test_fonts() -> Arc<FontCache> {
        static FONTS: OnceLock<Arc<FontCache>> = OnceLock::new();
        Arc::clone(FONTS.get_or_init(|| Arc::new(FontCache::load(&[], true))))
    }

    fn make_request(source: &str, format: &str) -> CompileRequest {
        CompileRequest {
            source: source.to_owned(),
            main_path: "/main.typ".to_owned(),
            virtual_files: HashMap::new(),
            binary_files: HashMap::new(),
            format: format.to_owned(),
            ppi: 72.0,
        }
    }

    #[tokio::test]
    async fn extracts_exact_layout_boxes_across_pages_and_formats() {
        let source = r#"
#set page(width: 400pt, height: 600pt, margin: 40pt)
#let tag-block(id, body) = {
  layout(size => context {
    let pos = here().position()
    let measured = measure(width: size.width, body)
    [#metadata((
      id: id,
      page: pos.page,
      x: pos.x.pt(),
      y: pos.y.pt(),
      w: measured.width.pt(),
      h: measured.height.pt(),
    )) <fx-block-box>]
  })
  body
}

#tag-block("blk-header", block(width: 100%, height: 60pt)[Header])
#pagebreak()
#tag-block("blk-footer", block(width: 120pt, height: 30pt)[Footer])
"#;

        let service = CompilerService::new(test_fonts(), 2);

        for fmt in ["svg", "pdf", "png"] {
            let outcome = service.compile(make_request(source, fmt)).await;
            let CompileOutcome::Success(resp) = outcome else {
                panic!("expected success for format {fmt}");
            };
            assert_eq!(resp.page_count, 2);
            assert_eq!(resp.layout_boxes.len(), 2, "expected 2 boxes for {fmt}");

            let b0 = &resp.layout_boxes[0];
            assert_eq!(b0.id, "blk-header");
            assert_eq!(b0.page, 1);
            assert!((b0.x - 40.0).abs() < 0.5, "unexpected x={}", b0.x);
            assert!((b0.y - 40.0).abs() < 0.5, "unexpected y={}", b0.y);
            assert!((b0.width - 320.0).abs() < 0.5, "unexpected w={}", b0.width);
            assert!((b0.height - 60.0).abs() < 0.5, "unexpected h={}", b0.height);
            assert!((b0.page_width - 400.0).abs() < 0.01);
            assert!((b0.page_height - 600.0).abs() < 0.01);

            let b1 = &resp.layout_boxes[1];
            assert_eq!(b1.id, "blk-footer");
            assert_eq!(b1.page, 2);
            assert!((b1.x - 40.0).abs() < 0.5, "unexpected x={}", b1.x);
            assert!((b1.y - 40.0).abs() < 0.5, "unexpected y={}", b1.y);
            assert!((b1.width - 120.0).abs() < 0.5, "unexpected w={}", b1.width);
            assert!((b1.height - 30.0).abs() < 0.5, "unexpected h={}", b1.height);
            assert!((b1.page_width - 400.0).abs() < 0.01);
            assert!((b1.page_height - 600.0).abs() < 0.01);
        }
    }

    #[tokio::test]
    async fn clamps_overflowing_and_tiny_boxes_and_skips_malformed_entries() {
        let source = r#"
#set page(width: 500pt, height: 800pt, margin: 20pt)
#metadata((id: "overflow", page: 1, x: 20, y: 700, w: 460, h: 450)) <fx-block-box>
#metadata((id: "near-bottom", page: 1, x: 20.0, y: 795.0, w: 200.0, h: 50.0)) <fx-block-box>
#metadata((id: "divider", page: 1, x: 20, y: 100, w: 2, h: 1)) <fx-block-box>
#metadata("not-a-dict") <fx-block-box>
#metadata((id: "", page: 1, x: 0, y: 0, w: 10, h: 10)) <fx-block-box>
#metadata((id: "bad-page-zero", page: 0, x: 0, y: 0, w: 10, h: 10)) <fx-block-box>
#metadata((id: "bad-page-oob", page: 99, x: 0, y: 0, w: 10, h: 10)) <fx-block-box>
#metadata((id: "missing-h", page: 1, x: 0, y: 0, w: 10)) <fx-block-box>
= Heading <fx-block-box>
"#;

        let service = CompilerService::new(test_fonts(), 1);
        let CompileOutcome::Success(resp) = service.compile(make_request(source, "svg")).await
        else {
            panic!("expected compile success");
        };

        assert_eq!(resp.layout_boxes.len(), 3);

        assert_eq!(resp.layout_boxes[0].id, "overflow");
        assert!((resp.layout_boxes[0].width - 460.0).abs() < 0.01);
        assert!((resp.layout_boxes[0].height - 100.0).abs() < 0.01);

        assert_eq!(resp.layout_boxes[1].id, "near-bottom");
        assert!((resp.layout_boxes[1].height - 12.0).abs() < 0.01);

        assert_eq!(resp.layout_boxes[2].id, "divider");
        assert!((resp.layout_boxes[2].width - 8.0).abs() < 0.01);
        assert!((resp.layout_boxes[2].height - 8.0).abs() < 0.01);
    }

    #[tokio::test]
    async fn plain_document_has_empty_layout_boxes() {
        let service = CompilerService::new(test_fonts(), 1);
        let CompileOutcome::Success(resp) = service
            .compile(make_request("= Hello World", "svg"))
            .await
        else {
            panic!("expected compile success");
        };
        assert!(resp.layout_boxes.is_empty());
    }
}
