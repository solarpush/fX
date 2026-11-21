use std::borrow::Cow;
use std::collections::HashMap;

use base64::alphabet;
use base64::engine::general_purpose::{GeneralPurpose, GeneralPurposeConfig, STANDARD};
use base64::engine::DecodePaddingMode;
use base64::Engine as _;
use serde::{Deserialize, Serialize};

/// Crate version exposed by `GET /health`.
pub const DAEMON_VERSION: &str = env!("CARGO_PKG_VERSION");

/// Pinned Typst compiler version exposed by `GET /health`.
pub const TYPST_VERSION: &str = "0.15.1";

pub const DEFAULT_MAIN_PATH: &str = "/main.typ";
pub const DEFAULT_FORMAT: &str = "pdf";
pub const DEFAULT_PPI: f32 = 144.0;
pub const MIN_PPI: f32 = 1.0;
pub const MAX_PPI: f32 = 1200.0;

/// Base64 decoder accepting both canonically padded and unpadded standard base64.
const BASE64_DECODER: GeneralPurpose = GeneralPurpose::new(
    &alphabet::STANDARD,
    GeneralPurposeConfig::new().with_decode_padding_mode(DecodePaddingMode::Indifferent),
);

fn default_main_path() -> String {
    DEFAULT_MAIN_PATH.to_owned()
}

fn default_format() -> String {
    DEFAULT_FORMAT.to_owned()
}

fn default_ppi() -> f32 {
    DEFAULT_PPI
}

/// Supported export formats for `POST /compile`.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum OutputFormat {
    Pdf,
    PdfA3b,
    Svg,
    Png,
}

impl OutputFormat {
    /// Parses a requested output format string (case-insensitive, trimmed).
    pub fn parse(raw: &str) -> Result<Self, String> {
        match raw.trim().to_ascii_lowercase().as_str() {
            "pdf" => Ok(Self::Pdf),
            "pdf-a3b" => Ok(Self::PdfA3b),
            "svg" => Ok(Self::Svg),
            "png" => Ok(Self::Png),
            other => Err(format!(
                "unsupported format '{other}': expected one of 'pdf', 'pdf-a3b', 'svg', 'png'"
            )),
        }
    }

    /// Canonical wire identifier matching the request/response contract.
    pub const fn as_str(self) -> &'static str {
        match self {
            Self::Pdf => "pdf",
            Self::PdfA3b => "pdf-a3b",
            Self::Svg => "svg",
            Self::Png => "png",
        }
    }

    /// MIME type for the rendered artifact.
    pub const fn mime_type(self) -> &'static str {
        match self {
            Self::Pdf | Self::PdfA3b => "application/pdf",
            Self::Svg => "image/svg+xml",
            Self::Png => "image/png",
        }
    }
}

/// JSON request payload for `POST /compile`.
#[derive(Debug, Clone, Deserialize)]
pub struct CompileRequest {
    pub source: String,
    #[serde(default = "default_main_path")]
    pub main_path: String,
    #[serde(default)]
    pub virtual_files: HashMap<String, String>,
    #[serde(default)]
    pub binary_files: HashMap<String, String>,
    #[serde(default = "default_format")]
    pub format: String,
    #[serde(default = "default_ppi")]
    pub ppi: f32,
}

impl CompileRequest {
    /// Validates request parameters (format, PPI bounds, main path) before compilation.
    pub fn validate(&self) -> Result<OutputFormat, String> {
        let format = OutputFormat::parse(&self.format)?;

        if !self.ppi.is_finite() || self.ppi < MIN_PPI || self.ppi > MAX_PPI {
            return Err(format!(
                "invalid ppi {}: must be a finite value between {} and {}",
                self.ppi, MIN_PPI, MAX_PPI
            ));
        }

        if self.main_path.trim().is_empty() {
            return Err("main_path must not be empty".to_owned());
        }

        Ok(format)
    }
}

/// Structured compiler warning or error diagnostic.
#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
pub struct DiagnosticItem {
    pub severity: String,
    pub message: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub file: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub line: Option<usize>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub column: Option<usize>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub hints: Vec<String>,
}

impl DiagnosticItem {
    /// Constructs a span-less error diagnostic (for request validation or export failures).
    pub fn error(message: impl Into<String>) -> Self {
        Self {
            severity: "error".to_owned(),
            message: message.into(),
            file: None,
            line: None,
            column: None,
            hints: Vec::new(),
        }
    }
}

/// Exact layout bounding box for a tagged document block, in typographic points (pt).
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct LayoutBox {
    pub id: String,
    pub page: usize, // 1-based page number
    pub x: f64,
    pub y: f64,
    pub width: f64,
    pub height: f64,
    pub page_width: f64,
    pub page_height: f64,
}

/// JSON response payload for `POST /compile`.
#[derive(Debug, Clone, Serialize)]
pub struct CompileResponse {
    pub success: bool,
    pub format: String,
    pub mime_type: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub data_base64: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub pages_base64: Option<Vec<String>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub pages_svg: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub layout_boxes: Vec<LayoutBox>,
    pub page_count: usize,
    pub duration_ms: u64,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub warnings: Vec<DiagnosticItem>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub errors: Vec<DiagnosticItem>,
}

impl CompileResponse {
    /// Builds a successful single-artifact (PDF / PDF-A3b) response.
    pub fn success_pdf(
        format: OutputFormat,
        pdf_bytes: &[u8],
        page_count: usize,
        duration_ms: u64,
        warnings: Vec<DiagnosticItem>,
    ) -> Self {
        Self {
            success: true,
            format: format.as_str().to_owned(),
            mime_type: format.mime_type().to_owned(),
            data_base64: Some(encode_base64(pdf_bytes)),
            pages_base64: None,
            pages_svg: None,
            layout_boxes: Vec::new(),
            page_count,
            duration_ms,
            warnings,
            errors: Vec::new(),
        }
    }

    /// Builds a successful multi-page SVG response.
    pub fn success_svg(
        pages_svg: Vec<String>,
        duration_ms: u64,
        warnings: Vec<DiagnosticItem>,
    ) -> Self {
        let page_count = pages_svg.len();
        Self {
            success: true,
            format: OutputFormat::Svg.as_str().to_owned(),
            mime_type: OutputFormat::Svg.mime_type().to_owned(),
            data_base64: None,
            pages_base64: None,
            pages_svg: Some(pages_svg),
            layout_boxes: Vec::new(),
            page_count,
            duration_ms,
            warnings,
            errors: Vec::new(),
        }
    }

    /// Builds a successful multi-page PNG response.
    pub fn success_png(
        pages_base64: Vec<String>,
        duration_ms: u64,
        warnings: Vec<DiagnosticItem>,
    ) -> Self {
        let page_count = pages_base64.len();
        Self {
            success: true,
            format: OutputFormat::Png.as_str().to_owned(),
            mime_type: OutputFormat::Png.mime_type().to_owned(),
            data_base64: None,
            pages_base64: Some(pages_base64),
            pages_svg: None,
            layout_boxes: Vec::new(),
            page_count,
            duration_ms,
            warnings,
            errors: Vec::new(),
        }
    }

    /// Builds a structured failure response (HTTP 400 or HTTP 422).
    pub fn failure(
        format: impl Into<String>,
        mime_type: impl Into<String>,
        duration_ms: u64,
        warnings: Vec<DiagnosticItem>,
        errors: Vec<DiagnosticItem>,
    ) -> Self {
        Self {
            success: false,
            format: format.into(),
            mime_type: mime_type.into(),
            data_base64: None,
            pages_base64: None,
            pages_svg: None,
            layout_boxes: Vec::new(),
            page_count: 0,
            duration_ms,
            warnings,
            errors,
        }
    }

    /// Attaches extracted block layout bounding boxes to this response.
    pub fn with_layout_boxes(mut self, layout_boxes: Vec<LayoutBox>) -> Self {
        self.layout_boxes = layout_boxes;
        self
    }
}

/// JSON response payload for `GET /health`.
#[derive(Debug, Clone, Serialize)]
pub struct HealthResponse {
    pub status: &'static str,
    pub version: &'static str,
    pub typst_version: &'static str,
    pub fonts_loaded: usize,
    pub renders_total: u64,
    pub avg_ms: f64,
}

/// Encodes raw bytes using standard padded Base64.
pub fn encode_base64(bytes: &[u8]) -> String {
    STANDARD.encode(bytes)
}

/// Strips an optional `data:<mediatype>;base64,` URI prefix (case-insensitive)
/// and returns the raw Base64 payload slice.
pub fn strip_data_uri_prefix(input: &str) -> &str {
    let trimmed = input.trim();
    if trimmed.len() >= 5 && trimmed[..5].eq_ignore_ascii_case("data:") {
        if let Some(comma_idx) = trimmed.find(',') {
            let header = &trimmed[5..comma_idx];
            let has_base64_marker = header
                .split(';')
                .any(|part| part.trim().eq_ignore_ascii_case("base64"));
            if has_base64_marker {
                return &trimmed[comma_idx + 1..];
            }
        }
    }
    trimmed
}

/// Decodes a Base64 string (optionally wrapped in a `data:...;base64,` URI and/or
/// containing embedded ASCII line breaks/whitespace).
pub fn decode_base64_payload(input: &str) -> Result<Vec<u8>, String> {
    let payload = strip_data_uri_prefix(input).trim();
    let cleaned: Cow<'_, str> = if payload.bytes().any(|b| b.is_ascii_whitespace()) {
        Cow::Owned(
            payload
                .chars()
                .filter(|c| !c.is_ascii_whitespace())
                .collect(),
        )
    } else {
        Cow::Borrowed(payload)
    };

    BASE64_DECODER
        .decode(cleaned.as_bytes())
        .map_err(|err| format!("invalid base64 payload: {err}"))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn compile_request_defaults() {
        let json = r#"{"source": "= Hello"}"#;
        let req: CompileRequest = serde_json::from_str(json).expect("valid minimal request");

        assert_eq!(req.source, "= Hello");
        assert_eq!(req.main_path, "/main.typ");
        assert_eq!(req.format, "pdf");
        assert!((req.ppi - 144.0).abs() < f32::EPSILON);
        assert!(req.virtual_files.is_empty());
        assert!(req.binary_files.is_empty());
        assert_eq!(req.validate().expect("valid"), OutputFormat::Pdf);
    }

    #[test]
    fn compile_request_full_fields() {
        let json = r#"{
            "source": "= Facture",
            "main_path": "/invoices/main.typ",
            "virtual_files": {"/data.json": "{\"total\": 42}"},
            "binary_files": {"/logo.png": "data:image/png;base64,iVBORw0KGgo="},
            "format": "pdf-a3b",
            "ppi": 300.0
        }"#;
        let req: CompileRequest = serde_json::from_str(json).expect("valid full request");

        assert_eq!(req.main_path, "/invoices/main.typ");
        assert_eq!(req.virtual_files.len(), 1);
        assert_eq!(req.binary_files.len(), 1);
        assert_eq!(req.validate().expect("valid"), OutputFormat::PdfA3b);
        assert!((req.ppi - 300.0).abs() < f32::EPSILON);
    }

    #[test]
    fn format_parsing_and_mime_types() {
        assert_eq!(OutputFormat::parse("pdf").unwrap(), OutputFormat::Pdf);
        assert_eq!(OutputFormat::parse("PDF").unwrap(), OutputFormat::Pdf);
        assert_eq!(
            OutputFormat::parse("pdf-a3b").unwrap(),
            OutputFormat::PdfA3b
        );
        assert_eq!(OutputFormat::parse("svg").unwrap(), OutputFormat::Svg);
        assert_eq!(OutputFormat::parse("png").unwrap(), OutputFormat::Png);

        assert_eq!(OutputFormat::Pdf.mime_type(), "application/pdf");
        assert_eq!(OutputFormat::PdfA3b.mime_type(), "application/pdf");
        assert_eq!(OutputFormat::Svg.mime_type(), "image/svg+xml");
        assert_eq!(OutputFormat::Png.mime_type(), "image/png");

        assert!(OutputFormat::parse("docx").is_err());
        assert!(OutputFormat::parse("").is_err());
    }

    #[test]
    fn ppi_validation_rejects_out_of_bounds() {
        for bad_ppi in [0.0, -72.0, f32::NAN, f32::INFINITY, 2400.0] {
            let req = CompileRequest {
                source: "= Test".to_owned(),
                main_path: "/main.typ".to_owned(),
                virtual_files: HashMap::new(),
                binary_files: HashMap::new(),
                format: "png".to_owned(),
                ppi: bad_ppi,
            };
            assert!(
                req.validate().is_err(),
                "expected ppi {bad_ppi} to be rejected"
            );
        }
    }

    #[test]
    fn base64_plain_and_data_uri_stripping() {
        let expected = b"Hello, Factur-X!";
        let encoded = encode_base64(expected);

        assert_eq!(decode_base64_payload(&encoded).unwrap(), expected);

        let data_uri = format!("data:image/png;base64,{encoded}");
        assert_eq!(decode_base64_payload(&data_uri).unwrap(), expected);

        let upper_uri = format!("  DATA:application/octet-stream;charset=utf-8;BASE64,{encoded}  ");
        assert_eq!(decode_base64_payload(&upper_uri).unwrap(), expected);

        let multiline = format!("data:text/plain;base64,\n{}\n{}\n", &encoded[..8], &encoded[8..]);
        assert_eq!(decode_base64_payload(&multiline).unwrap(), expected);

        let unpadded = encoded.trim_end_matches('=');
        assert_eq!(decode_base64_payload(unpadded).unwrap(), expected);
    }

    #[test]
    fn base64_rejects_corrupted_input() {
        assert!(decode_base64_payload("not!valid@base64###").is_err());
        assert!(decode_base64_payload("data:image/png;base64,***").is_err());
    }

    #[test]
    fn compile_response_skips_none_and_empty_vecs() {
        let resp = CompileResponse::success_pdf(OutputFormat::Pdf, b"%PDF-1.7", 1, 12, Vec::new());
        let val = serde_json::to_value(&resp).expect("serialize");
        let obj = val.as_object().expect("json object");

        assert_eq!(obj.get("success").and_then(|v| v.as_bool()), Some(true));
        assert_eq!(obj.get("format").and_then(|v| v.as_str()), Some("pdf"));
        assert_eq!(
            obj.get("mime_type").and_then(|v| v.as_str()),
            Some("application/pdf")
        );
        assert!(obj.contains_key("data_base64"));
        assert!(!obj.contains_key("pages_base64"));
        assert!(!obj.contains_key("pages_svg"));
        assert!(!obj.contains_key("layout_boxes"));
        assert!(!obj.contains_key("warnings"));
        assert!(!obj.contains_key("errors"));
        assert_eq!(obj.get("page_count").and_then(|v| v.as_u64()), Some(1));
        assert_eq!(obj.get("duration_ms").and_then(|v| v.as_u64()), Some(12));
    }

    #[test]
    fn compile_response_with_layout_boxes_serializes_cleanly() {
        let box_item = LayoutBox {
            id: "blk-header".to_owned(),
            page: 1,
            x: 42.5,
            y: 36.0,
            width: 510.25,
            height: 84.0,
            page_width: 595.28,
            page_height: 841.89,
        };
        let resp = CompileResponse::success_svg(vec!["<svg/>".to_owned()], 5, Vec::new())
            .with_layout_boxes(vec![box_item.clone()]);

        let val = serde_json::to_value(&resp).expect("serialize");
        let boxes = val["layout_boxes"].as_array().expect("layout_boxes array");
        assert_eq!(boxes.len(), 1);
        let roundtrip: LayoutBox =
            serde_json::from_value(boxes[0].clone()).expect("deserialize LayoutBox");
        assert_eq!(roundtrip, box_item);
    }

    #[test]
    fn health_response_schema() {
        let health = HealthResponse {
            status: "ok",
            version: DAEMON_VERSION,
            typst_version: TYPST_VERSION,
            fonts_loaded: 42,
            renders_total: 123,
            avg_ms: 14.2,
        };
        let val = serde_json::to_value(&health).expect("serialize");
        assert_eq!(val["status"], "ok");
        assert_eq!(val["version"], "0.1.0");
        assert_eq!(val["typst_version"], "0.15.1");
        assert_eq!(val["fonts_loaded"], 42);
        assert_eq!(val["renders_total"], 123);
    }
}
