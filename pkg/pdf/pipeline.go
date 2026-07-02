package pdf

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/solarpush/fx/pkg/cii"
	"github.com/solarpush/fx/pkg/invoice"
	"github.com/solarpush/fx/pkg/template"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

//go:embed templates/*.typ
var defaultTemplates embed.FS

// FacturXPipeline orchestre la génération complète d'un PDF Factur-X
type FacturXPipeline struct {
	typstBinary *TypstBinary
}

// NewFacturXPipeline crée un nouveau pipeline
func NewFacturXPipeline() (*FacturXPipeline, error) {
	typst, err := FindTypst()
	if err != nil {
		return nil, fmt.Errorf("typst non disponible: %w", err)
	}

	return &FacturXPipeline{
		typstBinary: typst,
	}, nil
}

// Generate génère un PDF Factur-X complet à partir des données de facture
func (fp *FacturXPipeline) Generate(inv *invoice.Invoice, options *GenerateOptions) ([]byte, error) {
	if options == nil {
		options = &GenerateOptions{}
	}

	// 1. Charger le template Typst
	templateContent, err := fp.loadTemplate(options.TemplatePath)
	if err != nil {
		return nil, fmt.Errorf("erreur chargement template: %w", err)
	}

	// 2. Convertir l'invoice en JSON
	jsonData, err := invoice.ToJSON(inv)
	if err != nil {
		return nil, fmt.Errorf("erreur sérialisation invoice: %w", err)
	}

	// 3. Remplir le template avec les données
	engine, err := template.New(jsonData)
	if err != nil {
		return nil, fmt.Errorf("erreur création template engine: %w", err)
	}

	filledTemplate, err := engine.Render(string(templateContent))
	if err != nil {
		return nil, fmt.Errorf("erreur rendu template: %w", err)
	}

	// 4. Générer le XML Factur-X
	xmlContent, err := cii.Generate(inv)
	if err != nil {
		return nil, fmt.Errorf("erreur génération XML: %w", err)
	}

	// 5. Créer un dossier temporaire pour l'XML
	_ = os.MkdirAll("./tmp", 0755)
	tmpDir, err := os.MkdirTemp("./tmp", "facturx-*")
	if err != nil {
		return nil, fmt.Errorf("erreur création tmpdir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	xmlFileName := "factur-x.xml"
	xmlPath := filepath.Join(tmpDir, xmlFileName)
	if err := os.WriteFile(xmlPath, xmlContent, 0644); err != nil {
		return nil, fmt.Errorf("erreur écriture xml: %w", err)
	}

	// 6. Ajouter l'attachement natif Typst
	// On utilise filepath.ToSlash et on préfixe par "/" pour que Typst cherche depuis la racine du projet (--root)
	relXmlPath := "/" + filepath.ToSlash(xmlPath)
	attachCmd := fmt.Sprintf("\n#pdf.attach(\"%s\", relationship: \"data\", mime-type: \"text/xml\", description: \"Factur-X Invoice\")\n", relXmlPath)
	filledTemplate += attachCmd

	// 7. Compiler le template en PDF avec Typst (incluant l'attachement XML)
	pdfContent, err := fp.typstBinary.CompileToPDFBytes([]byte(filledTemplate))
	if err != nil {
		return nil, fmt.Errorf("erreur compilation PDF: %w", err)
	}

	// 8. Patcher le PDF pour la conformité Factur-X (XMP custom + Nom de fichier)
	patchedPdf, err := fp.patchFacturXPDF(pdfContent, inv.Profile)
	if err != nil {
		return nil, fmt.Errorf("erreur patching PDF: %w", err)
	}

	return patchedPdf, nil
}

// GenerateToFile génère un PDF Factur-X et l'écrit dans un fichier
func (fp *FacturXPipeline) GenerateToFile(inv *invoice.Invoice, outputPath string, options *GenerateOptions) error {
	pdfData, err := fp.Generate(inv, options)
	if err != nil {
		return err
	}

	return os.WriteFile(outputPath, pdfData, 0644)
}

// CompileFile compile un fichier Typst vers PDF (PDF/A-3b, pour Factur-X)
func (fp *FacturXPipeline) CompileFile(inputPath, outputPath string) error {
	// Compiler directement le fichier d'entrée sans créer de nouveau fichier temporaire
	return fp.typstBinary.CompileFileDirect(inputPath, outputPath)
}

// CompileFilePlain compile un fichier Typst vers un PDF standard (sans contrainte
// PDF/A-3b). Destiné aux templates custom, non archivistiques : sortie plus
// légère et compilation plus rapide (pas d'intégration complète des polices).
func (fp *FacturXPipeline) CompileFilePlain(inputPath, outputPath string) error {
	return fp.typstBinary.CompileFileDirectPlain(inputPath, outputPath)
}

// loadTemplate charge le template Typst (custom ou par défaut)
func (fp *FacturXPipeline) loadTemplate(templatePath string) ([]byte, error) {
	if templatePath != "" {
		// Template personnalisé
		return os.ReadFile(templatePath)
	}

	// Template par défaut embarqué
	return defaultTemplates.ReadFile("templates/facture-template.typ")
}

// patchFacturXPDF manipule les objets PDF de bas niveau pour rendre le fichier conforme Factur-X
func (fp *FacturXPipeline) patchFacturXPDF(pdfContent []byte, profile invoice.Profile) ([]byte, error) {
	reader := bytes.NewReader(pdfContent)
	ctx, err := api.ReadContext(reader, nil)
	if err != nil {
		return nil, err
	}

	// Map profile to Factur-X ConformanceLevel for XMP
	conformanceLevel := "EN 16931"
	if profile == invoice.ProfileEXTENDED {
		conformanceLevel = "EXTENDED"
	}

	rootDict := ctx.RootDict
	metaRef, ok := rootDict["Metadata"].(types.IndirectRef)
	if ok {
		obj, _ := ctx.Dereference(metaRef)
		if stream, ok := obj.(types.StreamDict); ok {
			_ = stream.Decode()
			content := string(stream.Content)

			// Inject XMP Factur-X if not present
			if !strings.Contains(content, "fx:DocumentType") {
				schemaStr := `
        <rdf:li rdf:parseType="Resource">
          <pdfaSchema:schema>Factur-X PDFA Extension Schema</pdfaSchema:schema>
          <pdfaSchema:namespaceURI>urn:factur-x:pdfa:CrossIndustryDocument:invoice:1p0#</pdfaSchema:namespaceURI>
          <pdfaSchema:prefix>fx</pdfaSchema:prefix>
          <pdfaSchema:property>
            <rdf:Seq>
              <rdf:li rdf:parseType="Resource">
                <pdfaProperty:name>DocumentFileName</pdfaProperty:name>
                <pdfaProperty:valueType>Text</pdfaProperty:valueType>
                <pdfaProperty:category>external</pdfaProperty:category>
                <pdfaProperty:description>The name of the embedded XML document</pdfaProperty:description>
              </rdf:li>
              <rdf:li rdf:parseType="Resource">
                <pdfaProperty:name>DocumentType</pdfaProperty:name>
                <pdfaProperty:valueType>Text</pdfaProperty:valueType>
                <pdfaProperty:category>external</pdfaProperty:category>
                <pdfaProperty:description>The type of the hybrid document in capital letters, e.g. INVOICE or ORDER</pdfaProperty:description>
              </rdf:li>
              <rdf:li rdf:parseType="Resource">
                <pdfaProperty:name>Version</pdfaProperty:name>
                <pdfaProperty:valueType>Text</pdfaProperty:valueType>
                <pdfaProperty:category>external</pdfaProperty:category>
                <pdfaProperty:description>The actual version of the standard applying to the embedded XML document</pdfaProperty:description>
              </rdf:li>
              <rdf:li rdf:parseType="Resource">
                <pdfaProperty:name>ConformanceLevel</pdfaProperty:name>
                <pdfaProperty:valueType>Text</pdfaProperty:valueType>
                <pdfaProperty:category>external</pdfaProperty:category>
                <pdfaProperty:description>The conformance level of the embedded XML document</pdfaProperty:description>
              </rdf:li>
            </rdf:Seq>
          </pdfaSchema:property>
        </rdf:li>`

				insertPos := strings.Index(content, "</pdfaExtension:schemas>")
				if insertPos != -1 {
					bagEndPos := strings.LastIndex(content[:insertPos], "</rdf:Bag>")
					if bagEndPos != -1 {
						content = content[:bagEndPos] + schemaStr + "\n" + content[bagEndPos:]
					}
				} else {
					schemasBlock := `
  <rdf:Description rdf:about="" xmlns:pdfaExtension="http://www.aiim.org/pdfa/ns/extension/" xmlns:pdfaSchema="http://www.aiim.org/pdfa/ns/schema#" xmlns:pdfaProperty="http://www.aiim.org/pdfa/ns/property#">
    <pdfaExtension:schemas>
      <rdf:Bag>` + schemaStr + `
      </rdf:Bag>
    </pdfaExtension:schemas>
  </rdf:Description>`
					content = strings.Replace(content, "</rdf:RDF>", schemasBlock+"\n</rdf:RDF>", 1)
				}

				fxProps := fmt.Sprintf(`
  <rdf:Description rdf:about="" xmlns:fx="urn:factur-x:pdfa:CrossIndustryDocument:invoice:1p0#">
    <fx:DocumentType>INVOICE</fx:DocumentType>
    <fx:DocumentFileName>factur-x.xml</fx:DocumentFileName>
    <fx:Version>1.0</fx:Version>
    <fx:ConformanceLevel>%s</fx:ConformanceLevel>
  </rdf:Description>`, conformanceLevel)

				content = strings.Replace(content, "</rdf:RDF>", fxProps+"\n</rdf:RDF>", 1)
				stream.Content = []byte(content)
				stream.FilterPipeline = nil // Important: Do NOT compress XMP metadata for PDF/A
				_ = stream.Encode()         // Safely update Raw and stream Length fields
				ctx.Table[int(metaRef.ObjectNumber)].Object = stream
			}
		}
	}

	if namesObj, ok := rootDict["Names"]; ok {
		var namesDict types.Dict
		var isNamesRef bool
		if ref, ok := namesObj.(types.IndirectRef); ok {
			deref, _ := ctx.Dereference(ref)
			namesDict, _ = deref.(types.Dict)
			isNamesRef = true
		} else if d, ok := namesObj.(types.Dict); ok {
			namesDict = d
		}

		if namesDict != nil {
			if efObj, ok := namesDict["EmbeddedFiles"]; ok {
				var efTreeDict types.Dict
				var efRef types.IndirectRef
				isEfRef := false
				if ref, ok := efObj.(types.IndirectRef); ok {
					deref, _ := ctx.Dereference(ref)
					efTreeDict, _ = deref.(types.Dict)
					efRef = ref
					isEfRef = true
				} else if d, ok := efObj.(types.Dict); ok {
					efTreeDict = d
				}

				if efTreeDict != nil {
					if names, ok := efTreeDict["Names"].(types.Array); ok {
						for i := 0; i < len(names); i += 2 {
							var nameStr string
							if s, ok := names[i].(types.StringLiteral); ok {
								nameStr = s.Value()
							} else if h, ok := names[i].(types.HexLiteral); ok {
								nameStr = h.Value()
							}
							if strings.Contains(nameStr, "factur-x.xml") {
								names[i] = types.StringLiteral("factur-x.xml")
								if ref, ok := names[i+1].(types.IndirectRef); ok {
									fsObj, _ := ctx.Dereference(ref)
									if fsDict, ok := fsObj.(types.Dict); ok {
										fsDict["F"] = types.StringLiteral("factur-x.xml")
										fsDict["UF"] = types.StringLiteral("factur-x.xml")
										fsDict["AFRelationship"] = types.Name("Data")
										ctx.Table[int(ref.ObjectNumber)].Object = fsDict
									}
								}
							}
						}
						efTreeDict["Names"] = names
						if isEfRef {
							ctx.Table[int(efRef.ObjectNumber)].Object = efTreeDict
						} else {
							namesDict["EmbeddedFiles"] = efTreeDict
							if isNamesRef {
								ref := namesObj.(types.IndirectRef)
								ctx.Table[int(ref.ObjectNumber)].Object = namesDict
							} else {
								rootDict["Names"] = namesDict
							}
						}
					}
				}
			}
		}
	}

	// Désactiver l'écriture en object streams / xref stream : on produit une table
	// xref classique et des objets non compressés, comme les exemples Factur-X
	// officiels. Certains validateurs (dont FNFE) ne parcourent pas les object
	// streams et ne trouvent alors ni le /AF, ni le Filespec, ni les métadonnées.
	if ctx.Configuration != nil {
		ctx.WriteObjectStream = false
		ctx.WriteXRefStream = false
	}

	var w bytes.Buffer
	err = api.WriteContext(ctx, &w)
	if err != nil {
		return nil, err
	}

	return w.Bytes(), nil
}

// GenerateOptions options de génération
type GenerateOptions struct {
	// TemplatePath chemin vers un template Typst personnalisé
	TemplatePath string

	// OutputFormat format de sortie (pdf, facturx)
	OutputFormat string
}

// GeneratePDFOnly génère uniquement le PDF sans XML embarqué
func (fp *FacturXPipeline) GeneratePDFOnly(inv *invoice.Invoice, options *GenerateOptions) ([]byte, error) {
	if options == nil {
		options = &GenerateOptions{}
	}

	templateContent, err := fp.loadTemplate(options.TemplatePath)
	if err != nil {
		return nil, err
	}

	jsonData, err := invoice.ToJSON(inv)
	if err != nil {
		return nil, err
	}

	engine, err := template.New(jsonData)
	if err != nil {
		return nil, err
	}

	filledTemplate, err := engine.Render(string(templateContent))
	if err != nil {
		return nil, err
	}

	return fp.typstBinary.CompileToPDFBytes([]byte(filledTemplate))
}

// GenerateXMLOnly génère uniquement le XML Factur-X
func (fp *FacturXPipeline) GenerateXMLOnly(inv *invoice.Invoice) ([]byte, error) {
	return cii.Generate(inv)
}
