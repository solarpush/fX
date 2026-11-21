package blueprint

import (
	"fmt"
	"strings"
)

// Supported patch operations for ApplyPatches.
const (
	OpSetTheme      = "set_theme"
	OpSetPage       = "set_page"
	OpSetHeader     = "set_header"
	OpSetFooter     = "set_footer"
	OpAddBlock      = "add_block"
	OpUpdateBlock   = "update_block"
	OpRemoveBlock   = "remove_block"
	OpMoveBlock     = "move_block"
	OpReplaceBlocks = "replace_blocks"
)

// Patch represents a single atomic edit operation against a Blueprint.
type Patch struct {
	Op        string                 `json:"op"`
	BlockID   string                 `json:"blockId,omitempty"`
	ParentID  string                 `json:"parentId,omitempty"`
	Index     *int                   `json:"index,omitempty"`
	Block     *Block                 `json:"block,omitempty"`
	Props     map[string]interface{} `json:"props,omitempty"`
	Style     *BlockStyle            `json:"style,omitempty"`
	VisibleIf *string                `json:"visibleIf,omitempty"`
	Theme     *ThemeConfig           `json:"theme,omitempty"`
	Page      *PageConfig            `json:"page,omitempty"`
	Header    *HeaderFooterConfig    `json:"header,omitempty"`
	Footer    *HeaderFooterConfig    `json:"footer,omitempty"`
	Blocks    []Block                `json:"blocks,omitempty"`
}

// Clone returns a deep copy of the Blueprint so mutations never alias original data.
func (bp *Blueprint) Clone() *Blueprint {
	if bp == nil {
		return nil
	}
	cp := *bp
	if len(bp.Meta.Modules) > 0 {
		cp.Meta.Modules = append([]string(nil), bp.Meta.Modules...)
	}
	if bp.Header != nil {
		h := *bp.Header
		cp.Header = &h
	}
	if bp.Footer != nil {
		f := *bp.Footer
		cp.Footer = &f
	}
	cp.Blocks = cloneBlocks(bp.Blocks)
	return &cp
}

func cloneBlocks(src []Block) []Block {
	if src == nil {
		return nil
	}
	out := make([]Block, len(src))
	for i, b := range src {
		out[i] = cloneBlock(b)
	}
	return out
}

func cloneBlockStyle(s *BlockStyle) *BlockStyle {
	if s == nil {
		return nil
	}
	cp := *s
	return &cp
}

func cloneBlock(b Block) Block {
	cp := Block{
		ID:        b.ID,
		Type:      b.Type,
		VisibleIf: b.VisibleIf,
		Style:     cloneBlockStyle(b.Style),
		Props:     cloneMap(b.Props),
		Children:  cloneBlocks(b.Children),
	}
	return cp
}

func cloneMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = cloneValue(v)
	}
	return out
}

func cloneValue(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		return cloneMap(val)
	case []interface{}:
		out := make([]interface{}, len(val))
		for i, item := range val {
			out[i] = cloneValue(item)
		}
		return out
	case []map[string]interface{}:
		out := make([]map[string]interface{}, len(val))
		for i, item := range val {
			out[i] = cloneMap(item)
		}
		return out
	case []string:
		out := make([]string, len(val))
		copy(out, val)
		return out
	default:
		return val
	}
}

// ApplyPatches applies a sequence of Patch operations to a deep copy of bp.
// It is pure copy-on-write: the input Blueprint is never mutated, and if any
// patch fails an error is returned with a nil result.
func ApplyPatches(bp *Blueprint, patches []Patch) (*Blueprint, error) {
	if bp == nil {
		return nil, fmt.Errorf("blueprint is nil")
	}

	next := bp.Clone()
	for i, p := range patches {
		if err := applySinglePatch(next, p); err != nil {
			return nil, fmt.Errorf("patch[%d] (%s): %w", i, p.Op, err)
		}
	}
	return next, nil
}

func applySinglePatch(bp *Blueprint, p Patch) error {
	switch p.Op {
	case OpSetTheme:
		if p.Theme == nil {
			return fmt.Errorf("theme payload is required")
		}
		if p.Theme.Preset != "" {
			bp.Theme = *p.Theme
		} else {
			mergeThemeTokens(&bp.Theme.Overrides, p.Theme.Overrides)
		}
		return nil

	case OpSetPage:
		if p.Page == nil {
			return fmt.Errorf("page payload is required")
		}
		prevPaper := bp.Page.Paper
		prevMargin := bp.Page.Margin
		bp.Page = *p.Page
		if bp.Page.Paper == "" {
			bp.Page.Paper = prevPaper
		}
		if bp.Page.Margin.Top == "" {
			bp.Page.Margin.Top = prevMargin.Top
		}
		if bp.Page.Margin.Bottom == "" {
			bp.Page.Margin.Bottom = prevMargin.Bottom
		}
		if bp.Page.Margin.Left == "" {
			bp.Page.Margin.Left = prevMargin.Left
		}
		if bp.Page.Margin.Right == "" {
			bp.Page.Margin.Right = prevMargin.Right
		}
		return nil

	case OpSetHeader:
		if p.Header == nil {
			bp.Header = nil
		} else {
			h := *p.Header
			bp.Header = &h
		}
		return nil

	case OpSetFooter:
		if p.Footer == nil {
			bp.Footer = nil
		} else {
			f := *p.Footer
			bp.Footer = &f
		}
		return nil

	case OpAddBlock:
		if p.Block == nil {
			return fmt.Errorf("block payload is required")
		}
		newBlk := cloneBlock(*p.Block)
		if strings.TrimSpace(newBlk.ID) == "" {
			return fmt.Errorf("block.id is required")
		}
		if err := checkUniqueIDsAgainstTree(bp.Blocks, newBlk); err != nil {
			return err
		}
		return insertBlock(bp, p.ParentID, p.Index, newBlk)

	case OpUpdateBlock:
		if strings.TrimSpace(p.BlockID) == "" {
			return fmt.Errorf("blockId is required")
		}
		target := findBlockByID(bp.Blocks, p.BlockID)
		if target == nil {
			return fmt.Errorf("block %q not found", p.BlockID)
		}
		if p.Block != nil {
			if p.Block.Type != "" {
				target.Type = p.Block.Type
			}
			if p.Block.VisibleIf != "" {
				target.VisibleIf = p.Block.VisibleIf
			}
			if p.Block.Props != nil {
				if target.Props == nil {
					target.Props = make(map[string]interface{})
				}
				for k, v := range p.Block.Props {
					if v == nil {
						delete(target.Props, k)
					} else {
						target.Props[k] = cloneValue(v)
					}
				}
			}
			if p.Block.Style != nil {
				applyBlockStyleUpdate(&target.Style, p.Block.Style)
			}
			if p.Block.Children != nil {
				target.Children = cloneBlocks(p.Block.Children)
			}
		}
		if p.Props != nil {
			if target.Props == nil {
				target.Props = make(map[string]interface{})
			}
			for k, v := range p.Props {
				if v == nil {
					delete(target.Props, k)
				} else {
					target.Props[k] = cloneValue(v)
				}
			}
		}
		if p.Style != nil {
			applyBlockStyleUpdate(&target.Style, p.Style)
		}
		if p.VisibleIf != nil {
			target.VisibleIf = *p.VisibleIf
		}
		return nil

	case OpRemoveBlock:
		if strings.TrimSpace(p.BlockID) == "" {
			return fmt.Errorf("blockId is required")
		}
		var removed bool
		bp.Blocks, _, removed = removeBlockByID(bp.Blocks, p.BlockID)
		if !removed {
			return fmt.Errorf("block %q not found", p.BlockID)
		}
		return nil

	case OpMoveBlock:
		if strings.TrimSpace(p.BlockID) == "" {
			return fmt.Errorf("blockId is required")
		}
		if p.ParentID == p.BlockID {
			return fmt.Errorf("cannot move block %q into itself", p.BlockID)
		}
		src := findBlockByID(bp.Blocks, p.BlockID)
		if src == nil {
			return fmt.Errorf("block %q not found", p.BlockID)
		}
		if p.ParentID != "" && findBlockByID(src.Children, p.ParentID) != nil {
			return fmt.Errorf("cannot move block %q into its own descendant %q", p.BlockID, p.ParentID)
		}
		if p.ParentID != "" && findBlockByID(bp.Blocks, p.ParentID) == nil {
			return fmt.Errorf("target parent %q not found", p.ParentID)
		}

		var moved Block
		var ok bool
		bp.Blocks, moved, ok = removeBlockByID(bp.Blocks, p.BlockID)
		if !ok {
			return fmt.Errorf("block %q not found", p.BlockID)
		}
		return insertBlock(bp, p.ParentID, p.Index, moved)

	case OpReplaceBlocks:
		bp.Blocks = cloneBlocks(p.Blocks)
		return nil

	default:
		return fmt.Errorf("unknown patch op %q", p.Op)
	}
}

func mergeThemeTokens(dst *ThemeTokens, src ThemeTokens) {
	if src.Primary != "" {
		dst.Primary = src.Primary
	}
	if src.Accent != "" {
		dst.Accent = src.Accent
	}
	if src.Bg != "" {
		dst.Bg = src.Bg
	}
	if src.Surface != "" {
		dst.Surface = src.Surface
	}
	if src.Text != "" {
		dst.Text = src.Text
	}
	if src.Muted != "" {
		dst.Muted = src.Muted
	}
	if src.Border != "" {
		dst.Border = src.Border
	}
	if src.FontHeading != "" {
		dst.FontHeading = src.FontHeading
	}
	if src.FontBody != "" {
		dst.FontBody = src.FontBody
	}
	if src.FontMono != "" {
		dst.FontMono = src.FontMono
	}
	if src.Radius != "" {
		dst.Radius = src.Radius
	}
	if src.Density != "" {
		dst.Density = src.Density
	}
}

func applyBlockStyleUpdate(dst **BlockStyle, src *BlockStyle) {
	if src == nil {
		return
	}
	if src.IsEmpty() {
		*dst = nil
		return
	}
	if *dst == nil {
		*dst = cloneBlockStyle(src)
		return
	}
	d := *dst
	if v := strings.TrimSpace(src.MarginTop); v != "" {
		d.MarginTop = v
	}
	if v := strings.TrimSpace(src.MarginBottom); v != "" {
		d.MarginBottom = v
	}
	if v := strings.TrimSpace(src.Padding); v != "" {
		d.Padding = v
	}
	if v := strings.TrimSpace(src.Bg); v != "" {
		d.Bg = v
	}
	if v := strings.TrimSpace(src.Color); v != "" {
		d.Color = v
	}
	if v := strings.TrimSpace(src.Border); v != "" {
		d.Border = v
	}
	if v := strings.TrimSpace(src.Radius); v != "" {
		d.Radius = v
	}
	if v := strings.TrimSpace(src.Align); v != "" {
		d.Align = v
	}
	if v := strings.TrimSpace(src.Width); v != "" {
		d.Width = v
	}
	if src.KeepTogether {
		d.KeepTogether = true
	}
}

func checkUniqueIDsAgainstTree(existing []Block, candidate Block) error {
	existingIDs := map[string]bool{}
	collectBlockIDs(existing, existingIDs)

	candidateIDs := map[string]bool{}
	var walk func(b Block) error
	walk = func(b Block) error {
		id := strings.TrimSpace(b.ID)
		if id != "" {
			if existingIDs[id] {
				return fmt.Errorf("block id %q already exists", id)
			}
			if candidateIDs[id] {
				return fmt.Errorf("duplicate block id %q in added subtree", id)
			}
			candidateIDs[id] = true
		}
		for _, c := range b.Children {
			if err := walk(c); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(candidate)
}

func collectBlockIDs(blocks []Block, set map[string]bool) {
	for _, b := range blocks {
		if b.ID != "" {
			set[b.ID] = true
		}
		collectBlockIDs(b.Children, set)
	}
}

func findBlockByID(blocks []Block, id string) *Block {
	for i := range blocks {
		if blocks[i].ID == id {
			return &blocks[i]
		}
		if found := findBlockByID(blocks[i].Children, id); found != nil {
			return found
		}
	}
	return nil
}

func removeBlockByID(blocks []Block, id string) ([]Block, Block, bool) {
	for i := range blocks {
		if blocks[i].ID == id {
			removed := blocks[i]
			next := make([]Block, 0, len(blocks)-1)
			next = append(next, blocks[:i]...)
			next = append(next, blocks[i+1:]...)
			return next, removed, true
		}
		if nextChildren, removed, ok := removeBlockByID(blocks[i].Children, id); ok {
			blocks[i].Children = nextChildren
			return blocks, removed, true
		}
	}
	return blocks, Block{}, false
}

func insertBlock(bp *Blueprint, parentID string, index *int, blk Block) error {
	if strings.TrimSpace(parentID) == "" {
		next, err := insertAtIndex(bp.Blocks, index, blk)
		if err != nil {
			return err
		}
		bp.Blocks = next
		return nil
	}

	parent := findBlockByID(bp.Blocks, parentID)
	if parent == nil {
		return fmt.Errorf("parent block %q not found", parentID)
	}
	next, err := insertAtIndex(parent.Children, index, blk)
	if err != nil {
		return err
	}
	parent.Children = next
	return nil
}

func insertAtIndex(slice []Block, index *int, blk Block) ([]Block, error) {
	if index == nil {
		return append(slice, blk), nil
	}
	idx := *index
	if idx < 0 || idx > len(slice) {
		return nil, fmt.Errorf("index %d out of range [0, %d]", idx, len(slice))
	}
	out := make([]Block, 0, len(slice)+1)
	out = append(out, slice[:idx]...)
	out = append(out, blk)
	out = append(out, slice[idx:]...)
	return out, nil
}
