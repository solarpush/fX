package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: patch <in.pdf> <out.pdf>")
		return
	}

	inFile := os.Args[1]
	outFile := os.Args[2]

	ctx, err := api.ReadContextFile(inFile)
	if err != nil {
		panic(err)
	}

	rootDict := ctx.RootDict
	metaRef, ok := rootDict["Metadata"].(types.IndirectRef)
	if ok {
		obj, _ := ctx.Dereference(metaRef)
		if stream, ok := obj.(types.StreamDict); ok {
			stream.Decode()
			content := string(stream.Content)
			
			// Inject XMP Factur-X
			if !strings.Contains(content, "fx:DocumentType") {
				xmpExt := `
  <rdf:Description rdf:about="" xmlns:fx="urn:factur-x:pdfa:CrossIndustryDocument:invoice:1p0#">
    <fx:DocumentType>INVOICE</fx:DocumentType>
    <fx:DocumentFileName>factur-x.xml</fx:DocumentFileName>
    <fx:Version>1.0</fx:Version>
    <fx:ConformanceLevel>EN 16931</fx:ConformanceLevel>
  </rdf:Description>`
  				content = strings.Replace(content, "</rdf:RDF>", xmpExt+"\n</rdf:RDF>", 1)
				stream.Content = []byte(content)
				stream.Encode()
				ctx.Table[int(metaRef.ObjectNumber)].Object = stream
			}
		}
	}

	namesDict, ok := rootDict["Names"].(types.Dict)
	if ok {
		efRef, ok := namesDict["EmbeddedFiles"].(types.IndirectRef)
		if ok {
			ef, _ := ctx.Dereference(efRef)
			if efDict, ok := ef.(types.Dict); ok {
				if names, ok := efDict["Names"].(types.Array); ok {
					for i, nameObj := range names {
						if nameStr, ok := nameObj.(types.StringLiteral); ok {
							if strings.Contains(nameStr.Value(), "factur-x.xml") {
								names[i] = types.StringLiteral("factur-x.xml")
							}
						}
						if ref, ok := nameObj.(types.IndirectRef); ok {
							fsObj, _ := ctx.Dereference(ref)
							if fsDict, ok := fsObj.(types.Dict); ok {
								fsDict["F"] = types.StringLiteral("factur-x.xml")
								fsDict["UF"] = types.StringLiteral("factur-x.xml")
								ctx.Table[int(ref.ObjectNumber)].Object = fsDict
							}
						}
					}
				}
			}
		}
	}

	var w bytes.Buffer
	err = api.WriteContext(ctx, &w)
	if err != nil {
		panic(err)
	}

	os.WriteFile(outFile, w.Bytes(), 0644)
	fmt.Println("Done")
}
