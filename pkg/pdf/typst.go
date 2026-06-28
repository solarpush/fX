package pdf

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

// TypstBinary gère l'exécution du binaire Typst
type TypstBinary struct {
	path string
}

// FindTypst cherche le binaire Typst dans l'ordre suivant:
// 1. @lpdjs/fx-typst package (node_modules)
// 2. Typst système (PATH)
func FindTypst() (*TypstBinary, error) {

	// 1. Chercher dans le PATH système
	if path, err := exec.LookPath("typst"); err == nil {
		return &TypstBinary{path: path}, nil
	}

	return nil, fmt.Errorf("typst non trouvé. Installez @lpdjs/fx-typst ou typst système")
}

// CompileToPDF compile un contenu Typst en PDF
func (tb *TypstBinary) CompileToPDF(typContent []byte, outputPath string) error {
	// Créer un fichier temporaire dans le répertoire du projet (pour compatibilité Snap)
	tmpBase := "./tmp/typst-compile"
	if err := os.MkdirAll(tmpBase, 0755); err != nil {
		return fmt.Errorf("erreur création dossier temporaire: %w", err)
	}
	tmpFile, err := os.CreateTemp(tmpBase, "invoice-*.typ")
	if err != nil {
		return fmt.Errorf("erreur création fichier temporaire: %w", err)
	}
	tmpName := tmpFile.Name()

	// Écrire le contenu
	if _, err := tmpFile.Write(typContent); err != nil {
		tmpFile.Close()
		os.Remove(tmpName)
		return fmt.Errorf("erreur écriture template: %w", err)
	}

	// Forcer l'écriture sur le disque
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		os.Remove(tmpName)
		return fmt.Errorf("erreur sync fichier: %w", err)
	}

	// Fermer le fichier
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("erreur fermeture fichier: %w", err)
	}

	// Construire les arguments
	args := []string{"compile"}
	if fontPath := os.Getenv("TYPST_FONT_PATHS"); fontPath != "" {
		args = append(args, "--font-path", fontPath)
	}
	// Définir la racine pour Typst (permet de résoudre les chemins commençant par /)
	root := os.Getenv("TYPST_ROOT")
	if root == "" {
		// Par défaut, le répertoire courant du projet. Ainsi #image("/images/logo.png")
		// cherchera dans ./images/logo.png
		root = "."
	}
	args = append(args, "--root", root)
	args = append(args, "--pdf-standard", "a-3b")
	args = append(args, tmpName, outputPath)

	// Compiler avec Typst (le fichier existe toujours sur le disque)
	cmd := exec.Command(tb.path, args...)
	output, err := cmd.CombinedOutput()

	// Nettoyer le fichier temporaire après la compilation
	os.Remove(tmpName)

	if err != nil {
		return fmt.Errorf("erreur compilation typst: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// CompileFileDirect compile un fichier Typst existant directement vers PDF
func (tb *TypstBinary) CompileFileDirect(inputPath, outputPath string) error {
	// Vérifier que le fichier d'entrée existe
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		log.Printf("[ERROR] Input file does not exist: %s", inputPath)
		return fmt.Errorf("input file not found: %s", inputPath)
	}

	log.Printf("[VERBOSE] Compiling directly: %s -> %s", inputPath, outputPath)
	log.Printf("[VERBOSE] Using typst binary: %s", tb.path)

	// Construire les arguments
	args := []string{"compile"}
	if fontPath := os.Getenv("TYPST_FONT_PATHS"); fontPath != "" {
		args = append(args, "--font-path", fontPath)
	}
	// Définir la racine pour Typst
	root := os.Getenv("TYPST_ROOT")
	if root == "" {
		root = "." // Par défaut, le répertoire courant du projet
	}
	args = append(args, "--root", root)
	args = append(args, "--pdf-standard", "a-3b")
	args = append(args, inputPath, outputPath)

	// Compiler directement le fichier d'entrée
	cmd := exec.Command(tb.path, args...)
	log.Printf("[VERBOSE] Running command: %s %v", tb.path, cmd.Args)

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[ERROR] Typst compilation failed: %v\nOutput: %s", err, string(output))
		return fmt.Errorf("erreur compilation typst: %w\nOutput: %s", err, string(output))
	}

	log.Printf("[VERBOSE] Compilation successful, output: %s", string(output))
	return nil
}

// CompileToPDFBytes compile et retourne le PDF en bytes
func (tb *TypstBinary) CompileToPDFBytes(typContent []byte) ([]byte, error) {
	// Utiliser le répertoire du projet au lieu de /tmp pour compatibilité Snap
	tmpBase := "./tmp/typst-pdf"
	if err := os.MkdirAll(tmpBase, 0755); err != nil {
		return nil, err
	}
	tmpDir, err := os.MkdirTemp(tmpBase, "facture-pdf-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	outputPath := filepath.Join(tmpDir, "output.pdf")

	if err := tb.CompileToPDF(typContent, outputPath); err != nil {
		return nil, err
	}

	return os.ReadFile(outputPath)
}

// Version retourne la version de Typst
func (tb *TypstBinary) Version() (string, error) {
	cmd := exec.Command(tb.path, "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}
